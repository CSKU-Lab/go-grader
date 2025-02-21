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

	fmt.Println("Startng worker...")
	err := isolateService.Init()
	if err != nil {
		log.Fatal(err)
	}

	output, err := isolateService.Run(&models.Limit{
		WallTime: 1,
	})
	if err != nil {
		log.Fatal(err)
	}

	print("Stdout: ", output)
}
