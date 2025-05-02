package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
)

func main() {
	ctx := context.Background()

	rb, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
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
		log.Fatalln("Cannot parse execution struct to json")
	}

	for range 1 {
		err = rb.Publish(ctx, "grading", message)
		if err != nil {
			log.Fatalln("Cannot publish message to the execution queue")
		}

		log.Println("Publish message to the queue successfully!")
	}
}
