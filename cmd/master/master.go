package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/domain/models"
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
				Name: "main.c",
				Content: `#include<stdio.h>
				int main() {
					printf("Hello World");
					return 0;
				}`,
			},
		},
		LanguageID: "C_99",
	}

	message, err := json.Marshal(&execution)
	if err != nil {
		log.Fatalln("Cannot parse execution struct to json")
	}

	for {
		for range 10 {
			err = rb.Publish(ctx, "execution", message)
			if err != nil {
				log.Fatalln("Cannot publish message to the execution queue")
			}

			log.Println("Publish message to the queue successfully!")
		}
		time.Sleep(time.Second * 2)
	}
}
