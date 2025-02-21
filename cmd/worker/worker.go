package main

import (
	"context"
	"fmt"
	"log"

	"github.com/SornchaiTheDev/go-grader/models"
	"github.com/SornchaiTheDev/go-grader/services"
)

func main() {
	ctx := context.Background()
	isolateService := services.NewIsolateService(ctx)
	defer isolateService.Cleanup()

	fmt.Println("Starting worker...")
	err := isolateService.Init()
	if err != nil {
		log.Fatal(err)
	}

	err = isolateService.Run(&models.Limit{
		WallTime: 0.5,
	})
	if err != nil {
		log.Fatal(err)
	}
}
