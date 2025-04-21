package services

import (
	"github.com/CSKU-Lab/go-grader/models"
)

type runnerService struct {
	isolateService  *IsolateService
	languageService *LanguageService
}

func NewRunnerService(isolateService *IsolateService, languageService *LanguageService) *runnerService {
	return &runnerService{
		isolateService:  isolateService,
		languageService: languageService,
	}
}

type Result struct {
	StdOut   string
	StdErr   string
	Metadata *models.Metadata
}

func (r *runnerService) Run(langID string, files []models.File) (*Result, error) {
	instance := r.isolateService.NewInstance()
	defer instance.Cleanup()

	for _, file := range files {
		if err := instance.CreateFile(file.Name, file.Content); err != nil {
			return nil, err
		}
	}

	language, err := r.languageService.GetByID(langID)
	if err != nil {
		return nil, err
	}

	if language.NeedCompile {
		err := instance.CompileUsing(language.Path)
		if err != nil {
			return nil, err
		}
	}

	err = instance.Run(language.Path, nil, false)
	if err != nil {
		return nil, err
	}

	stdOut, err := instance.GetOutput()
	if err != nil {
		return nil, err
	}

	stdErr, err := instance.GetError()
	if err != nil {
		return nil, err
	}

	metadata, err := instance.GetMetadata()
	if err != nil {
		return nil, err
	}

	return &Result{
		StdOut:   stdOut,
		StdErr:   stdErr,
		Metadata: metadata,
	}, nil
}
