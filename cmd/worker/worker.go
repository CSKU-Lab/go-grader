package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CSKU-Lab/go-grader/configs"
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

	taskGRPC, closeTask := initTaskServerClient(logger, env.GetTaskServerURL())
	defer closeTask()

	runnerRes, err := configGRPC.GetRunners(ctx, &configPB.GetRunnersRequest{})
	if err != nil {
		logger.Fatalw("Cannot get runners from gRPC server", "error", err)
	}

	compareRes, err := configGRPC.GetCompares(ctx, &emptypb.Empty{})
	if err != nil {
		logger.Fatalw("Cannot get compares from gRPC server", "error", err)
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

	go func() {
		err := q.Consume(ctx, "run", func(message []byte) {
			execution := &models.RunExecution{}

			err := json.Unmarshal(message, execution)
			if err != nil {
				logger.Fatalw("Cannot unmarshal message", "error", err)
			}

			executor := executorService.NewExecutor()
			defer executor.Cleanup()

			if err := executor.SetRunner(execution.RunnerID); err != nil {
				logger.Fatalw("Cannot set runner", "error", err)
			}

			if err := executor.SetFiles(execution.Files); err != nil {
				logger.Fatalw("Cannot set files", "error", err)
			}

			if err := executor.SetInput(execution.Input); err != nil {
				logger.Fatalw("Cannot set input", "error", err)
			}

			result, err := executor.Run()
			if err != nil {
				logger.Fatalw("Error from runner", "error", err)
			}

			result.ID = execution.ID

			bytesResult, err := json.Marshal(result)
			if err != nil {
				logger.Errorw("Cannot marshal run result", "error", err)
			}

			err = q.Publish(ctx, "run_results", bytesResult)
			if err != nil {
				logger.Errorw("Cannot publish run result to the queue", "error", err)
			}

			logger.Infow("Runner finished", "result", result)
		})
		if err != nil {
			logger.Fatalw("Cannot consume message from the queue", "error", err)
		}
	}()

	go q.Consume(ctx, "grade", func(message []byte) {
		execution := &models.GradeExecution{}

		err := json.Unmarshal(message, execution)
		if err != nil {
			logger.Fatalw("Cannot unmarshal message", "error", err)
		}

		task, err := taskGRPC.GetTask(ctx, &taskPB.GetTaskRequest{
			Id: execution.TaskID,
		})
		if err != nil {
			logger.Fatalw("Cannot get task from gRPC server", "error", err)
		}

		executor := executorService.NewExecutor()
		if err := executor.SetRunner(task.GetRunnerId()); err != nil {
			logger.Fatalw("Cannot set runner", "error", err)
		}
		if err := executor.SetFiles(execution.Files); err != nil {
			logger.Fatalw("Cannot set files", "error", err)
		}
		executor.SetCompareID(task.GetCompareId())
		executor.SetTestCases(func() []models.TestCase {
			var testCases []models.TestCase
			for _, testcase := range task.Testcases {
				testCases = append(testCases, models.TestCase{
					ID:     testcase.Id,
					Input:  testcase.Input,
					Output: testcase.Output,
				})
			}
			return testCases
		}())

		result, err := executor.Grade()
		if err != nil {
			logger.Fatalw("Error from runner", "error", err)
		}

		logger.Infow("Grading finished", "result", result)

		result.ID = execution.ID

		bytesResult, err := json.Marshal(result)
		if err != nil {
			logger.Errorw("Cannot marshal grade result", "error", err)
		}

		err = q.Publish(ctx, "grade_results", bytesResult)
		if err != nil {
			logger.Errorw("Cannot publish grade result to the queue", "error", err)
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
