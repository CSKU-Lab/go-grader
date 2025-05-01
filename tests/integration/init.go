package integration

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func initTest() services.ExecutorService {
	setup.Init(testdatas.Runners, testdatas.Compares)
	isolateService := services.NewIsolateService(context.Background())
	runnerService := services.NewRunnerService()
	compareService := services.NewCompareService()
	executorService := services.NewExecutorService(isolateService, runnerService, compareService)

	return executorService
}
