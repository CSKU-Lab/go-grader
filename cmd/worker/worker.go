package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	configPB "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	taskPB "github.com/CSKU-Lab/go-grader/genproto/task/v1"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	logger, loggerCleanup, err := logging.New(os.Getenv("ENV"))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := loggerCleanup(); err != nil {
			logger.Warnw("failed to flush logger", "error", err)
		}
	}()

	env := configs.NewEnv(logger)

	ctx, cancel := context.WithCancel(context.Background())

	configGRPC, closeConfig := initConfigServerClient(logger, env.GetConfigServerURL())
	defer closeConfig()

	// taskGRPC, closeTask := initTaskServerClient(logger, env.GetTaskServerURL())
	// defer closeTask()

	var runnerRes *configPB.GetRunnersResponse
	for i := range 5 {
		runnerRes, err = configGRPC.GetRunners(ctx, &configPB.GetRunnersRequest{})
		if err == nil {
			break
		}
		logger.Warnw("Cannot get runners from gRPC server, retrying", "error", err, "attempt", i+1)
		time.Sleep(time.Duration((2*i)+1) * time.Second)
	}
	if err != nil {
		logger.Fatalw("Cannot get runners from gRPC server after retries", "error", err)
	}

	var compareRes *configPB.GetComparesResponse
	for i := range 5 {
		compareRes, err = configGRPC.GetCompares(ctx, &emptypb.Empty{})
		if err == nil {
			break
		}
		logger.Warnw("Cannot get compares from gRPC server, retrying", "error", err, "attempt", i+1)
		time.Sleep(time.Duration((2*i)+1) * time.Second)
	}
	if err != nil {
		logger.Fatalw("Cannot get compares from gRPC server after retries", "error", err)
	}

	runners := runnerPbToModel(runnerRes.Runners)
	compares := comparePbToModel(compareRes.Compares)

	setup.Init(logger, runners, compares)

	q, err := queue.NewRabbitMQ(logger, env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}

	runnerService := services.NewRunnerService(logger)
	compareService := services.NewCompareService(logger)

	isolateService := services.NewIsolateService(ctx, logger)
	executorService := services.NewExecutorService(logger, isolateService, runnerService, compareService)

	logger.Info("Worker is ready to start working 🤖...")

	var wg sync.WaitGroup

	wg.Go(func() {
		defer wg.Done()
		err := q.Consume(ctx, "run", constants.MAX_QUEUES, func(message []byte) error {
			payload := &models.RunExecution{}

			err := json.Unmarshal(message, payload)
			if err != nil {
				logger.Errorw("Cannot unmarshal message", "error", err)
				return err
			}

			executor := executorService.NewExecutor()
			defer executor.Cleanup()

			if err := executor.SetRunner(payload.RunnerID); err != nil {
				logger.Errorw("Cannot set runner", "error", err)
				return err
			}

			if err := executor.SetFiles(payload.Files); err != nil {
				logger.Errorw("Cannot set files", "error", err)
				return err
			}

			if err := executor.SetInput(payload.Input); err != nil {
				logger.Errorw("Cannot set input", "error", err)
				return err
			}

			bytesResult, err := json.Marshal(models.RunResult{
				ID:     payload.ID,
				Status: execution.RUNNING,
			})
			if err != nil {
				logger.Errorw("Cannot marshal run result", "error", err)
			}

			err = q.PublishToTopic(ctx, "topic.run_results", "result."+payload.ID, payload.ID, bytesResult)
			if err != nil {
				logger.Errorw("Cannot publish run result to the queue", "error", err)
			}

			result, err := executor.Run()
			if err != nil {
				logger.Errorw("Error from runner", "error", err)
				return err
			}

			result.ID = payload.ID

			bytesResult, err = json.Marshal(result)
			if err != nil {
				logger.Errorw("Cannot marshal run result", "error", err)
			}

			err = q.PublishToTopic(ctx, "topic.run_results", "result."+payload.ID, payload.ID, bytesResult)
			if err != nil {
				logger.Errorw("Cannot publish run result to the queue", "error", err)
			}

			logger.Infow("Runner finished", "result", result)
			return nil
		})
		if err != nil {
			logger.Errorw("Cannot consume message from the run queue", "error", err)
		}
	})

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	logger.Infof("Receive %s signal from OS, going to shutdown...", sig)
	timer := time.AfterFunc(10*time.Second, func() {
		logger.Warn("Server couldn't stop grafully in time. Doing force stop.")
	})
	defer timer.Stop()
	cancel()

	wg.Wait()

	q.Close()
	logger.Info("RabbitMQ connection is closed.")
	closeConfig()
	logger.Info("gRPC connection is closed.")
	logger.Info("Successfully gracefully shutdown the server :D")
}

func initConfigServerClient(logger *zap.SugaredLogger, clientAddr string) (client configPB.ConfigServiceClient, close func()) {
	conn, err := grpc.NewClient(clientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	c := configPB.NewConfigServiceClient(conn)

	return c, func() {
		conn.Close()
	}
}

func initTaskServerClient(logger *zap.SugaredLogger, clientAddr string) (client taskPB.TaskServiceClient, close func()) {
	conn, err := grpc.NewClient(clientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	c := taskPB.NewTaskServiceClient(conn)

	return c, func() {
		conn.Close()
	}
}

func runnerPbToModel(languages []*configPB.Runner) []models.RunnerConfig {
	_runners := make([]models.RunnerConfig, 0, 10)
	for _, lang := range languages {
		_runners = append(_runners, models.RunnerConfig{
			ID:          lang.GetId(),
			BuildScript: lang.GetBuildScript(),
			RunScript:   lang.GetRunScript(),
		})
	}
	return _runners
}

func comparePbToModel(compares []*configPB.CompareResponse) []models.CompareConfig {
	_compares := make([]models.CompareConfig, 0, 10)
	for _, compare := range compares {
		files := make([]models.File, 0, 10)
		for _, file := range compare.GetFiles() {
			files = append(files, models.File{
				Name:    file.GetName(),
				Content: file.GetContent(),
			})
		}

		_compares = append(_compares, models.CompareConfig{
			ID:          compare.GetId(),
			Files:       files,
			BuildScript: compare.GetBuildScript(),
			RunScript:   compare.GetRunScript(),
			RunName:     compare.GetRunName(),
		})
	}
	return _compares
}
