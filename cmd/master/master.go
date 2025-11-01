package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
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

	ctx := context.Background()
	env := configs.NewEnv(logger)

	rb, err := queue.NewRabbitMQ(logger, env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}
	defer rb.Close()

	execution := models.Execution{
		Files: []models.File{
			{
				Name:    "main.py",
				Content: `print("Hello",input())`,
			},
		},
		RunnerID: "python_3_11_2",
		TaskID:   "1",
	}

	message, err := json.Marshal(&execution)
	if err != nil {
		logger.Fatalw("Cannot parse execution struct to json", "error", err)
	}

	for range 1 {
		err = rb.Publish(ctx, "grading", message)
		if err != nil {
			logger.Fatalw("Cannot publish message to the execution queue", "error", err)
		}

		logger.Info("Publish message to the queue successfully!")
	}
}
