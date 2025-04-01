package services

import (
	"errors"
	"fmt"

	"github.com/SornchaiTheDev/go-grader/models"
)

type runnerService struct {
	isolateService    *IsolateService
	compileService    *CompileService
	langConfigService *LanguageConfigService
}

func NewRunnerService(isolateService *IsolateService, compileService *CompileService, langConfigService *LanguageConfigService) *runnerService {
	return &runnerService{
		isolateService:    isolateService,
		compileService:    compileService,
		langConfigService: langConfigService,
	}
}

func (r *runnerService) Run(e *models.Execution) (string, string, *models.Metadata, error) {
	instance := r.isolateService.NewInstance()
	// defer instance.Cleanup()

	config := r.langConfigService.Get(e.LanguageID, instance.ID())

	for _, file := range config.SandboxFiles {
		err := instance.CreateFile(file, e.Code)
		if err != nil {
			return "", "", nil, err
		}
	}

	if len(config.CompileScript) != 0 {
		err := r.compileService.Compile(config.CompileScript)
		if err != nil {
			return "", "", nil, errors.New(fmt.Sprintf("Error on compile: %s", err))
		}
	}

	// // If has input
	// err = instance.CreateFile("input", "1")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	err := instance.Run(config.RunScript, nil, false)
	if err != nil {
		return "", "", nil, err
	}

	stdOut, err := instance.GetOutput()
	if err != nil {
		return "", "", nil, err
	}

	stdErr, err := instance.GetError()
	if err != nil {
		return "", "", nil, err
	}

	metadata, err := instance.GetMetadata()
	if err != nil {
		return "", "", nil, err
	}

	return stdOut, stdErr, metadata, nil
}
