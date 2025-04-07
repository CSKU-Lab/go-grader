package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
)

func main() {
	ctx := context.Background()

	isolateService := services.NewIsolateService(ctx)
	compileService := services.NewCompileService(ctx)
	languageService := services.NewLanguageConfigService()
	runnerService := services.NewRunnerService(isolateService, compileService, languageService)

	rb, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}

	rb.Consume(ctx, "execution", func(message []byte) {
		execution := &models.Execution{}

		err := json.Unmarshal(message, execution)
		if err != nil {
			log.Fatalln("Cannot unmarshal message")
		}
		stdOut, stdErr, metadata, err := runnerService.Run(execution)
		if err != nil {
			log.Fatalln("Error from runner ", err)
		}

		log.Println(stdOut, stdErr, metadata)
	})
}
