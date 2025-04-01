package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/SornchaiTheDev/go-grader/infrastructure/queue"
	"github.com/SornchaiTheDev/go-grader/models"
)

func main() {
	ctx := context.Background()

	rb, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}
	defer rb.Close()

	execution := models.Execution{
		Code:       `import time
time.sleep(2
print("Hello, World!")`,
		LanguageID: "python_3.8",
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
