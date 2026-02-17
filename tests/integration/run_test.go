package integration

import (
	"context"
	"testing"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
)

func TestRunPassed(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("python_test").
		Files([]models.File{
			{
				Name:    "main.py",
				Content: `print("Hello World")`,
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Run(context.Background())
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	t.Log(result)
}

func TestRunWithInput(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("python_test").
		Files([]models.File{
			{
				Name:    "main.py",
				Content: `print(input())`,
			},
		}).
		Input("Hello World").
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Run(context.Background())
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	expected := "Hello World\n"
	got := result.Output

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
	}
}

func TestRunCompileFailed(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("cpp_test").
		Files([]models.File{
			{
				Name:    "main.cpp",
				Content: "#include <iostream",
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Run(context.Background())
	if err != nil {
		t.Fatalf("Cannot run : %s", err)
	}

	expected := execution.COMPILE_FAILED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected %s, Got %s", expected, got)
	}
}

func TestRunFailed(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("python_test").
		Files([]models.File{
			{
				Name:    "main.py",
				Content: `print("Hello World"`,
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Run(context.Background())
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	expected := execution.RUN_FAILED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected %s, Got %s", expected, got)
	}
}
