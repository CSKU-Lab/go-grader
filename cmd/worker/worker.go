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

	boxID := 0

	isolateService := services.NewIsolateService(ctx, boxID)
	defer isolateService.Cleanup()

	compileService := services.NewCompileService(ctx)

	fmt.Println("Starting worker...")
	err := isolateService.Init()
	if err != nil {
		log.Fatal(err)
	}

	code := `#include <stdio.h>
	int main() {
		int i;
		scanf("%d", &i);
		printf("%d\\n", i * 2);
	}
	`

	err = isolateService.CreateFile("main.c", code)
	if err != nil {
		log.Fatal(err)
	}

	boxPath := fmt.Sprintf("/var/local/lib/isolate/%d/box", boxID)
	err = compileService.Compile([]string{"gcc", boxPath + "/main.c", "-o", boxPath + "/program"})
	if err != nil {
		log.Fatal("Error on compile: ", err)
	}

	err = isolateService.CreateFile("input", "1")
	if err != nil {
		log.Fatal(err)
	}

	err = isolateService.Run("./program", []string{}, &models.Limit{
		WallTime: 0.5,
	})
	if err != nil {
		log.Fatal(err)
	}

	output, err := isolateService.GetOutput()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Output: ", output)

	metadata, err := isolateService.GetMetadata()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Metadata: ", metadata)

}
