package main

import (
	"context"
	"fmt"
	"log"

	"github.com/SornchaiTheDev/go-grader/services"
)

func main() {
	ctx := context.Background()

	boxID := 0

	isolateService := services.NewIsolateService(ctx, boxID)
	defer isolateService.Cleanup()

	fmt.Println("Starting worker...")
	err := isolateService.Init()
	if err != nil {
		log.Fatal(err)
	}

	code := `#include <stdio.h>
	int main() {
	    printf("Hello, World!");
	}
	`

	err = isolateService.CreateFile("main.c", code)
	if err != nil {
		log.Fatal(err)
	}

	boxPath := fmt.Sprintf("/var/local/lib/isolate/%d/box", boxID)
	err = isolateService.Compile([]string{"cd", boxPath, "&&", "pwd"})
	if err != nil {
		log.Fatal(err)
	}

	err = isolateService.Run("./program", []string{}, nil)
	if err != nil {
		log.Fatal(err)
	}
}
