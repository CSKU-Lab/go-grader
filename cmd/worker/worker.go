package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/SornchaiTheDev/go-grader/services"
)

var wg sync.WaitGroup

func main() {
	ctx := context.Background()

	isolateService := services.NewIsolateService(ctx, 2)
	compileService := services.NewCompileService(ctx)

	wg.Add(1)
	go runner(isolateService, compileService)
	wg.Add(1)
	go runner(isolateService, compileService)
	wg.Add(1)
	go runner(isolateService, compileService)
	wg.Add(1)
	go runner(isolateService, compileService)

	wg.Wait()
	log.Println("All done")
}

func runner(isolateService *services.IsolateService, compileService *services.CompileService) {
	defer wg.Done()

	instance := isolateService.New()
	defer instance.Cleanup()

	code := `#include <stdio.h>
	int fibo(int n) {
		if(n <= 1) {
			return n;
		}
		return fibo(n-1) + fibo(n-2);
	}
	int main() {
		printf("%d",fibo(40));
	}
	`

	err := instance.CreateFile("main.c", code)
	if err != nil {
		log.Fatal(err)
	}

	boxPath := fmt.Sprintf("/var/local/lib/isolate/%d/box", instance.ID())
	err = compileService.Compile([]string{"gcc", boxPath + "/main.c", "-o", boxPath + "/program"})
	if err != nil {
		log.Fatal("Error on compile: ", err)
	}

	err = instance.CreateFile("input", "1")
	if err != nil {
		log.Fatal(err)
	}

	err = instance.Run("./program", []string{}, nil)
	if err != nil {
		log.Fatal(err)
	}

	output, err := instance.GetOutput()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Output: ", output)

	metadata, err := instance.GetMetadata()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Metadata: ", metadata)

}
