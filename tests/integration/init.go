package integration

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func initTest() services.RunnerService {
	setup.Init(testdatas.Languages, testdatas.Compares)
	isolateService := services.NewIsolateService(context.Background())
	languageService := services.NewLanguageService()
	compareService := services.NewCompareService()
	runnerService := services.NewRunnerService(isolateService, languageService, compareService)

	return runnerService
}
