package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/messaging"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/adapters/sqlx"
	"github.com/CSKU-Lab/go-grader/internal/generators"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"go.uber.org/zap"
)

func storeRunResult(ctx context.Context, logger *zap.SugaredLogger, wg *sync.WaitGroup, service services.ResultService, q messaging.Queue) {
	defer wg.Done()
	err := q.Consume(ctx, "run_results", func(msg []byte) {
		result := &models.RunResult{}
		err := json.Unmarshal(msg, result)
		if err != nil {
			logger.Errorln("Cannot unmarshal run result message", "error", err)
		}
		err = service.CreateRunResult(ctx, result.ID, result)
		if err != nil {
			logger.Errorln("Cannot store run result to the database", "error", err)
		}
	})
	if err != nil {
		logger.Fatalw("Cannot consume run result messages", "error", err)
	}
}

func seedQueue(logger *zap.SugaredLogger, q messaging.Queue) {
	err := q.Declare("grade")
	if err != nil {
		logger.Fatalw("Cannot declare grading queue", "error", err)
	}

	err = q.Declare("run")
	if err != nil {
		logger.Fatalw("Cannot declare grading queue", "error", err)
	}

	err = q.Declare("run_results")
	if err != nil {
		logger.Fatalw("Cannot declare run_results queue", "error", err)
	}

	err = q.Declare("grade_results")
	if err != nil {
		logger.Fatalw("Cannot declare grade_results queue", "error", err)
	}
}

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

	ctx := context.Background()
	env := configs.NewEnv(logger)

	rb, err := queue.NewRabbitMQ(logger, env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}
	defer rb.Close()

	seedQueue(logger, rb)

	db := configs.NewDB(logger, os.Getenv("DATABASE_URL"))
	resultRepo := sqlx.NewSQLXInstance(db)
	resultService := services.NewResultService(resultRepo)

	go func() {
		for range 100 {
			id := generators.UUID()
			execution := models.Execution{
				ID: id,
				Files: []models.File{
					{
						Name:    "main.py",
						Content: `print("This output is in Postgresql database")`,
					},
				},
				RunnerID: "python_3_11_2",
				TaskID:   "1",
			}

			message, err := json.Marshal(&execution)
			if err != nil {
				logger.Fatalw("Cannot parse execution struct to json", "error", err)
			}

			err = rb.Publish(ctx, "run", message)
			if err != nil {
				logger.Fatalw("Cannot publish message to the execution queue", "error", err)
			}
			logger.Info("Publish message to the queue successfully!")
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go storeRunResult(context.Background(), logger, &wg, resultService, rb)

	wg.Wait()
}
