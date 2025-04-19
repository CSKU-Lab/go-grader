package services

import (
	"github.com/CSKU-Lab/go-grader/models"
)

type runnerService struct {
	isolateService *IsolateService
}

func NewRunnerService(isolateService *IsolateService) *runnerService {
	return &runnerService{
		isolateService: isolateService,
	}
}

func (r *runnerService) Run(e *models.Execution) (string, string, *models.Metadata, error) {
	instance := r.isolateService.NewInstance()
	defer instance.Cleanup()

	// for _, file := range config.SandboxFiles {
	// 	err := instance.CreateFile(file, e.Code)
	// 	if err != nil {
	// 		return "", "", nil, err
	// 	}
	// }
	//
	// if len(config.CompileScript) != 0 {
	// 	err := instance.Compile()
	// 	if err != nil {
	// 		return "", "", nil, errors.New(fmt.Sprintf("Error on compile: %s", err))
	// 	}
	// }
	//
	// // If has input
	// err = instance.CreateFile("input", "1")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// err := instance.Run(config.RunScript, nil, false)
	// if err != nil {
	// 	return "", "", nil, err
	// }

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
