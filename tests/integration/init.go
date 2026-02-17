package integration

import (
	"testing"

	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func initTest(t *testing.T) (services.ExecutorService, func()) {
	t.Helper()

	logger, cleanup, err := logging.New("test")
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	setup.Init(logger, testdatas.Runners, testdatas.Compares)
	isolateService := services.NewIsolateService(logger, 2, 2)
	runnerService := services.NewRunnerService(logger)
	compareService := services.NewCompareService(logger)
	executorService := services.NewExecutorService(logger, isolateService, runnerService, compareService)

	return executorService, func() {
		setup.Cleanup(logger)
		if err := cleanup(); err != nil {
			t.Logf("logger cleanup error: %v", err)
		}
	}
}
