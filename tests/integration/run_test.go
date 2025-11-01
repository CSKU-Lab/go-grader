package integration

import (
	"testing"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
)

func TestRunPassed(t *testing.T) {
	runnerService, cleanup := initTest(t)
	runner := runnerService.NewExecutor()
	defer runner.Cleanup()
	defer cleanup()

	runner.SetRunner("python_test")
	runner.SetFiles([]models.File{
		{
			Name:    "main.py",
			Content: `print("Hello World")`,
		},
	})
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	t.Log(result)
}

func TestRunWithInput(t *testing.T) {
	runnerService, cleanup := initTest(t)
	runner := runnerService.NewExecutor()
	defer runner.Cleanup()
	defer cleanup()

	runner.SetRunner("python_test")
	runner.SetFiles([]models.File{
		{
			Name:    "main.py",
			Content: `print(input())`,
		},
	})

	runner.SetInput("Hello World")
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	expected := "Hello World\n"
	got := result.StdOut

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
	}
}

func TestRunCompileFailed(t *testing.T) {
	runnerService, cleanup := initTest(t)
	runner := runnerService.NewExecutor()
	defer runner.Cleanup()
	defer cleanup()

	runner.SetRunner("cpp_test")
	runner.SetFiles([]models.File{
		{
			Name:    "main.cpp",
			Content: "#include <iostream",
		},
	})

	result, err := runner.Run()
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
	runnerService, cleanup := initTest(t)
	runner := runnerService.NewExecutor()
	defer runner.Cleanup()
	defer cleanup()

	runner.SetRunner("python_test")
	runner.SetFiles([]models.File{
		{
			Name:    "main.py",
			Content: `print("Hello World"`,
		},
	})
	result, err := runner.Run()
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}

	expected := execution.RUN_FAILED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected %s, Got %s", expected, got)
	}
}
