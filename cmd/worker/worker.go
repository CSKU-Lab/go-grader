package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	cskuotel "github.com/CSKU-Lab/otel"
	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants/broadcast"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	configPB "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	taskPB "github.com/CSKU-Lab/go-grader/genproto/task/v1"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"github.com/CSKU-Lab/queue"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	otelShutdown, err := cskuotel.Init(context.Background())
	if err != nil {
		logger.Warnw("tracing unavailable", "error", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(shutdownCtx); err != nil {
				logger.Warnw("tracer shutdown error", "error", err)
			}
		}()
	}

	env := configs.NewEnv(logger)
	ctx, cancel := context.WithCancel(context.Background())

	configGRPC, closeConfig := initConfigServerClient(logger, env.GetConfigServerURL())
	defer closeConfig()

	taskGRPC, closeTask := initTaskServerClient(logger, env.GetTaskServerURL())
	defer closeTask()

	var runnerRes *configPB.GetAllRunnersResponse
	for i := range 5 {
		runnerRes, err = configGRPC.GetAllRunners(ctx, &configPB.GetAllRunnersRequest{
			IncludeScripts: true,
		})
		if err == nil {
			break
		}
		logger.Warnw("Cannot get runners from gRPC server, retrying", "error", err, "attempt", i+1)
		time.Sleep(time.Duration((2*i)+1) * time.Second)
	}
	if err != nil {
		logger.Fatalw("Cannot get runners from gRPC server after retries", "error", err)
	}

	var compareRes *configPB.GetAllComparesResponse
	for i := range 5 {
		compareRes, err = configGRPC.GetAllCompares(ctx, &configPB.GetAllComparesRequest{
			IncludeFiles:   true,
			IncludeScripts: true,
		})
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

	q, err := queue.NewRabbitMQ(env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}

	_, err = q.CreateQueue(ctx, "run", &queue.QueueOptions{
		Durable: true,
	})
	if err != nil {
		logger.Fatalw("Cannot create 'run' queue", "error", err)
	}
	_, err = q.CreateQueue(ctx, "grade", &queue.QueueOptions{
		Durable: true,
	})
	if err != nil {
		logger.Fatalw("Cannot create 'run' queue", "error", err)
	}
	_, err = q.CreateQueue(ctx, "runner_test", &queue.QueueOptions{
		Durable: true,
	})
	if err != nil {
		logger.Fatalw("Cannot create 'runner_test' queue", "error", err)
	}
	_, err = q.CreateQueue(ctx, "compare_test", &queue.QueueOptions{
		Durable: true,
	})
	if err != nil {
		logger.Fatalw("Cannot create 'compare_test' queue", "error", err)
	}
	_, err = q.CreateQueue(ctx, "broadcast", &queue.QueueOptions{
		Durable: true,
	})
	if err != nil {
		logger.Fatalw("Cannot create 'broadcast' queue", "error", err)
	}

	setup.Init(logger, runners, compares)

	isolateService := services.NewIsolateService(logger, env.GetRunQueueAmount(), env.GetGradeQueueAmount())
	runnerService := services.NewRunnerService(logger)
	compareService := services.NewCompareService(logger)
	executorService := services.NewExecutorService(
		logger,
		isolateService,
		runnerService,
		compareService,
	)

	executorHolder := services.NewExecutorHolder(&executorService)

	logger.Info("Worker is ready to start working 🤖...")

	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			err := q.Consume(ctx, "broadcast", 1, true, func(d *queue.Derivery, exit chan struct{}) error {
				var action broadcast.Action

				if err := json.Unmarshal(d.Body, &action); err != nil {
					logger.Errorw("Cannot unmarshal broadcast message", "error", err)
					return err
				}

				switch action {
				case broadcast.REFETCH_CONFIG:
					logger.Info("Received broadcast: refetch config")

					runnerRes, err := configGRPC.GetAllRunners(ctx, &configPB.GetAllRunnersRequest{
						IncludeScripts: true,
					})
					if err != nil {
						logger.Errorw("Failed to refetch runners", "error", err)
						return err
					}

					compareRes, err := configGRPC.GetAllCompares(ctx, &configPB.GetAllComparesRequest{
						IncludeScripts: true,
						IncludeFiles:   true,
					})
					if err != nil {
						logger.Errorw("Failed to refetch compares", "error", err)
						return err
					}

					setup.Cleanup(logger)

					runners := runnerPbToModel(runnerRes.Runners)
					compares := comparePbToModel(compareRes.Compares)

					setup.Init(logger, runners, compares)

					newRunnerService := services.NewRunnerService(logger)
					newCompareService := services.NewCompareService(logger)

					newExecutor := services.NewExecutorService(
						logger,
						isolateService,
						newRunnerService,
						newCompareService,
					)

					executorHolder.Swap(&newExecutor)

					logger.Info("Config successfully reloaded 🚀")
				default:
					logger.Warnw("Unknown broadcast message", "type", action)
				}

				return nil
			})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Errorw("broadcast consumer error, restarting", "error", err)
			} else {
				logger.Warn("broadcast consumer exited unexpectedly, restarting")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	})

	wg.Go(func() {
		for {
			err := q.Consume(ctx, "run", env.GetRunQueueAmount(), true, func(derivery *queue.Derivery, exit chan struct{}) error {
				traceCtx := cskuotel.ExtractTraceContext(derivery.Headers)
				traceCtx, span := otel.Tracer("go-grader/worker").Start(traceCtx, "worker.run")
				defer span.End()

				payload := &models.RunExecution{}

				err := json.Unmarshal(derivery.Body, payload)
				if err != nil {
					logger.Errorw("Cannot unmarshal message", "error", err)
					return err
				}

				logger.Infow("Received run request", "payload", payload.RunnerID)

				exService := *executorHolder.Get()
				executor, err := exService.NewExecutor().
					RunnerID(payload.RunnerID).
					Files(payload.Files).
					Input(payload.Input).
					Limits(payload.Limit).
					Build()

				if err != nil {
					bytesResult, err := json.Marshal(models.RunResult{
						ID:     payload.ID,
						Status: execution.GRADER_ERROR,
					})
					if err != nil {
						logger.Errorw("Cannot marshal run result", "error", err)
					}

					err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						Body:          bytesResult,
						CorrelationID: payload.ID,
					})
					if err != nil {
						logger.Errorw("Cannot publish run result to the queue", "error", err)
					}
					return nil
				}

				bytesResult, err := json.Marshal(models.RunResult{
					ID:     payload.ID,
					Status: execution.RUNNING,
				})
				if err != nil {
					logger.Errorw("Cannot marshal run result", "error", err)
				}

				err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
					Body:          bytesResult,
					CorrelationID: payload.ID,
				})
				if err != nil {
					logger.Errorw("Cannot publish run result to the queue", "error", err)
				}

				result, err := executor.Run(ctx)
				if err != nil {
					logger.Errorw("Error from runner", "error", err)
					errResult, _ := json.Marshal(models.RunResult{
						ID:     payload.ID,
						Status: execution.GRADER_ERROR,
					})
					if pubErr := q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						Body:          errResult,
						CorrelationID: payload.ID,
					}); pubErr != nil {
						logger.Errorw("Cannot publish grader error result", "error", pubErr)
					}
					return err
				}

				result.ID = payload.ID

				bytesResult, err = json.Marshal(result)
				if err != nil {
					logger.Errorw("Cannot marshal run result", "error", err)
				}

				err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
					CorrelationID: payload.ID,
					Body:          bytesResult,
				})
				if err != nil {
					logger.Errorw("Cannot publish run result to the queue", "error", err)
				}

				logger.Infow("Runner finished", "result", result)
				return nil
			})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Errorw("run consumer error, restarting", "error", err)
			} else {
				logger.Warn("run consumer exited unexpectedly, restarting")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	})

	wg.Go(func() {
		for {
			err := q.Consume(ctx, "grade", env.GetGradeQueueAmount(), true, func(derivery *queue.Derivery, exit chan struct{}) error {
				traceCtx := cskuotel.ExtractTraceContext(derivery.Headers)
				traceCtx, span := otel.Tracer("go-grader/worker").Start(traceCtx, "worker.grade")
				defer span.End()

				payload := &models.GradeExecution{}

				err := json.Unmarshal(derivery.Body, payload)
				if err != nil {
					logger.Errorw("Cannot unmarshal message", "error", err)
					return err
				}

				task, err := taskGRPC.GetTask(traceCtx, &taskPB.GetTaskRequest{Id: payload.TaskID})
				if err != nil {
					logger.Errorw("Cannot get task from gRPC server", "error", err)
					return err
				}

				var limit *models.Limit
				if task.Limit != nil {
					limit = &models.Limit{
						CPUTime:      task.Limit.CpuTime,
						CPUExtraTime: task.Limit.CpuExtraTime,
						Memory:       task.Limit.Memory,
						WallTime:     task.Limit.WallTime,
						Stack:        task.Limit.Stack,
						MaxOpenFiles: task.Limit.MaxOpenFiles,
						MaxFileSize:  task.Limit.MaxFileSize,
						NetworkAllow: task.Limit.NetworkAllow,
					}
				}

				exService := *executorHolder.Get()
				executor, err := exService.NewExecutor().
					RunnerID(payload.RunnerID).
					Files(payload.Files).
					TestCaseGroups(testcaseGroupsPbToModel(task.GetTestCaseGroups())).
					Limits(limit).
					CompareID(task.GetCompareScriptId()).
					Build()
				if err != nil {
					bytesResult, err := json.Marshal(models.RunResult{
						ID:     payload.ID,
						Status: execution.GRADER_ERROR,
					})
					if err != nil {
						logger.Errorw("Cannot marshal run result", "error", err)
					}

					err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						Body:          bytesResult,
						CorrelationID: payload.ID,
					})
					if err != nil {
						logger.Errorw("Cannot publish run result to the queue", "error", err)
					}
					return nil
				}

				bytesResult, err := json.Marshal(models.RunResult{
					ID:     payload.ID,
					Status: execution.RUNNING,
				})
				if err != nil {
					logger.Errorw("Cannot marshal run result", "error", err)
				}

				err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
					Body:          bytesResult,
					CorrelationID: payload.ID,
				})
				if err != nil {
					logger.Errorw("Cannot publish run result to the queue", "error", err)
				}

				result, err := executor.Grade(ctx)
				if err != nil {
					logger.Errorw("Error from runner", "error", err)
					errResult, _ := json.Marshal(models.RunResult{
						ID:     payload.ID,
						Status: execution.GRADER_ERROR,
					})
					if pubErr := q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						Body:          errResult,
						CorrelationID: payload.ID,
					}); pubErr != nil {
						logger.Errorw("Cannot publish grader error result", "error", pubErr)
					}
					return err
				}

				bytesResult, err = json.Marshal(result)
				if err != nil {
					logger.Errorw("Cannot marshal run result", "error", err)
				}

				err = q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
					CorrelationID: payload.ID,
					Body:          bytesResult,
				})
				if err != nil {
					logger.Errorw("Cannot publish run result to the queue", "error", err)
				}

				logger.Info("Runner finished")
				return nil
			})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Errorw("grade consumer error, restarting", "error", err)
			} else {
				logger.Warn("grade consumer exited unexpectedly, restarting")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	})

	wg.Go(func() {
		for {
			err := q.Consume(ctx, "runner_test", 1, true, func(derivery *queue.Derivery, exit chan struct{}) error {
				result := models.RunResult{
					ID: derivery.CorrelationID,
				}

				publish := func() error {
					resultBytes, err := json.Marshal(result)
					if err != nil {
						return err
					}
					return q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						CorrelationID: derivery.CorrelationID,
						Body:          []byte(resultBytes),
					})
				}

				payload := &models.RunnerTestExecution{}
				err := json.Unmarshal(derivery.Body, payload)
				if err != nil {
					return err
				}

				instance := isolateService.NewRunInstance()
				instance.Init(ctx)
				defer instance.Cleanup()

				result.Status = execution.RUNNING
				if err = publish(); err != nil {
					return err
				}

				for _, file := range payload.InitialFiles {
					err = instance.CreateFile(file.Name, file.Content, 0600)
					if err != nil {
						result.Status = execution.GRADER_ERROR
						result.Output = err.Error()
						if err = publish(); err != nil {
							return err
						}
					}
				}

				if payload.BuildScript != "" {
					err = instance.CreateFile("build_script.sh", payload.BuildScript, 0700)
					if err != nil {
						result.Status = execution.GRADER_ERROR
						result.Output = err.Error()
						if err = publish(); err != nil {
							return err
						}
					}

					output, err := instance.Compile(ctx)
					if err != nil {
						result.Status = execution.COMPILE_FAILED
						result.Output = output
						if err = publish(); err != nil {
							return err
						}
					}
				}

				err = instance.CreateFile("run_script.sh", payload.RunScript, 0700)
				if err != nil {
					result.Status = execution.GRADER_ERROR
					result.Output = err.Error()
					if err = publish(); err != nil {
						return err
					}
				}

				output, err := instance.Run(ctx, "run_script.sh", payload.Input, nil)
				if err != nil {
					result.Status = execution.RUNTIME_ERROR
					result.Output = output
					if err = publish(); err != nil {
						return err
					}
				}

				metadata, err := instance.GetMetadata()
				if err != nil {
					result.Status = execution.GRADER_ERROR
					result.Output = err.Error()
					if err = publish(); err != nil {
						return err
					}
				}

				result.Output = output
				result.Status = execution.RUN_PASSED
				result.WallTime = metadata.WallTime
				result.Memory = metadata.Memory

				return publish()
			})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Errorw("runner_test consumer error, restarting", "error", err)
			} else {
				logger.Warn("runner_test consumer exited unexpectedly, restarting")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	})

	wg.Go(func() {
		for {
			err := q.Consume(ctx, "compare_test", 1, true, func(derivery *queue.Derivery, exit chan struct{}) error {
				result := models.RunResult{
					ID: derivery.CorrelationID,
				}

				publish := func() error {
					resultBytes, err := json.Marshal(result)
					if err != nil {
						return err
					}
					return q.Publish(ctx, "", derivery.ReplyTo, &queue.Derivery{
						CorrelationID: derivery.CorrelationID,
						Body:          resultBytes,
					})
				}

				payload := &models.CompareTestExecution{}
				err := json.Unmarshal(derivery.Body, payload)
				if err != nil {
					return err
				}

				if payload.RunName == "" {
					result.Status = execution.GRADER_ERROR
					result.Output = "run_name is required: set a Run Name in the compare script settings"
					return publish()
				}

				if payload.RunScript == "" {
					result.Status = execution.GRADER_ERROR
					result.Output = "run_script.sh content is required"
					return publish()
				}

				instance := isolateService.NewRunInstance()
				instance.Init(ctx)
				defer instance.Cleanup()

				result.Status = execution.RUNNING
				if err = publish(); err != nil {
					return err
				}

				for _, file := range payload.Files {
					err = instance.CreateFile(file.Name, file.Content, 0644)
					if err != nil {
						result.Status = execution.GRADER_ERROR
						result.Output = err.Error()
						return publish()
					}
				}

				err = instance.CreateFile("sol_output", payload.SolOutput, 0644)
				if err != nil {
					result.Status = execution.GRADER_ERROR
					result.Output = err.Error()
					return publish()
				}

				err = instance.CreateFile("output", payload.Output, 0644)
				if err != nil {
					result.Status = execution.GRADER_ERROR
					result.Output = err.Error()
					return publish()
				}

				if payload.BuildScript != "" {
					err = instance.CreateFile("build_script.sh", payload.BuildScript, 0700)
					if err != nil {
						result.Status = execution.GRADER_ERROR
						result.Output = err.Error()
						return publish()
					}

					output, err := instance.Compile(ctx)
					if err != nil {
						result.Status = execution.COMPILE_FAILED
						result.Output = output
						return publish()
					}
				}

				preamble := fmt.Sprintf(
					"#!/bin/bash\n"+
						"export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n"+
						"export RUN_NAME=/box/%s\n"+
						"export OUTPUT=/box/output\n"+
						"export SOL_OUTPUT=/box/sol_output\n",
					payload.RunName,
				)
				err = instance.CreateFile("run_script.sh", preamble+payload.RunScript, 0700)
				if err != nil {
					result.Status = execution.GRADER_ERROR
					result.Output = err.Error()
					return publish()
				}

				output, exitCode, err := instance.RunCompare(ctx)
				result.ExitCode = exitCode
				if err != nil {
					result.Status = execution.RUNTIME_ERROR
					result.Output = output
					return publish()
				}

				compareResult, _ := instance.GetCompareResult()
				result.Output = output
				result.CompareResult = compareResult
				result.Status = execution.RUN_PASSED
				return publish()
			})
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Errorw("compare_test consumer error, restarting", "error", err)
			} else {
				logger.Warn("compare_test consumer exited unexpectedly, restarting")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
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
	conn, err := grpc.NewClient(clientAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		logger.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	c := configPB.NewConfigServiceClient(conn)

	return c, func() {
		conn.Close()
	}
}

func initTaskServerClient(logger *zap.SugaredLogger, clientAddr string) (client taskPB.TaskServiceClient, close func()) {
	conn, err := grpc.NewClient(clientAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		logger.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	c := taskPB.NewTaskServiceClient(conn)

	return c, func() {
		conn.Close()
	}
}

func runnerPbToModel(languages []*configPB.RunnerResponse) []models.RunnerConfig {
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

func testcaseGroupsPbToModel(tcs []*taskPB.TestCaseGroup) []models.TestCaseGroup {
	testcases := make([]models.TestCaseGroup, 0, len(tcs))
	for _, tc := range tcs {
		testcase := models.TestCaseGroup{
			ID:        tc.GetId(),
			Score:     tc.GetScore(),
			TestCases: make([]models.TestCase, 0, len(tc.GetTestCases())),
		}

		for _, t := range tc.GetTestCases() {
			testcase.TestCases = append(testcase.TestCases, models.TestCase{
				ID:     t.GetId(),
				Input:  t.GetInput(),
				Output: t.GetOutput(),
			})
		}

		testcases = append(testcases, testcase)
	}
	return testcases
}
