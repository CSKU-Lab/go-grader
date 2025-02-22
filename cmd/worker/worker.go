package main

import (
	"context"
	"log"
	"sync"

	"github.com/SornchaiTheDev/go-grader/models"
	"github.com/SornchaiTheDev/go-grader/services"
)

var wg sync.WaitGroup

func main() {
	ctx := context.Background()

	isolateService := services.NewIsolateService(ctx, 1)
	compileService := services.NewCompileService(ctx)
	languageService := services.NewLanguageConfigService()

	wg.Add(1)
	go runner(isolateService, compileService, languageService)
	// wg.Add(1)
	// go runner(isolateService, compileService, languageService)
	// wg.Add(1)
	// go runner(isolateService, compileService, languageService)
	// wg.Add(1)
	// go runner(isolateService, compileService, languageService)

	wg.Wait()
	log.Println("All done")
}

func runner(isolateService *services.IsolateService, compileService *services.CompileService, langConfigService *services.LanguageConfigService) {
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
	lang := "C"
	config := langConfigService.Get(lang, instance.ID())

	for _, file := range config.SandboxFiles {
		err := instance.CreateFile(file, code)
		if err != nil {
			log.Fatal(err)
		}
	}

	err := compileService.Compile(config.CompileScript)
	if err != nil {
		log.Fatal("Error on compile: ", err)
	}

	// // If has input
	// err = instance.CreateFile("input", "1")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	err = instance.Run(config.RunScript, &models.Limit{
		WallTime: 0.2,
	}, false)
	if err != nil {
		log.Println(err)
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
