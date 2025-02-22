package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/SornchaiTheDev/go-grader/responses"
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

	code := `
	package main
	func main() {
		println("Hello, World!")
	}
	`
	lang := "Go"
	config := langConfigService.Get(lang, instance.ID())

	for _, file := range config.SandboxFiles {
		err := instance.CreateFile(file, code)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(config.CompileScript) != 0 {
		err := compileService.Compile(config.CompileScript)
		if err != nil {
			log.Fatal("Error on compile: ", err)
		}
	}

	// // If has input
	// err = instance.CreateFile("input", "1")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	err := instance.Run(config.RunScript, nil, false)
	if err != nil {
		log.Println(err)
	}

	stdOut, err := instance.GetOutput()
	if err != nil {
		log.Fatal(err)
	}

	stdErr, err := instance.GetError()
	if err != nil {
		log.Fatal(err)
	}

	metadata, err := instance.GetMetadata()
	if err != nil {
		log.Fatal(err)
	}

	result := responses.Result{
		SandboxMetadata: metadata,
		Output:          stdOut,
		Error:           stdErr,
	}

	resultStr, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Result:", string(resultStr))
}
