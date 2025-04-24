package integration

import (
	"log"
	"testing"

	"github.com/CSKU-Lab/go-grader/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/internal/setup"
)

func TestRunPassed(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

	runner.SetLanguage("python_test")
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

	log.Println(result)
}

func TestRunWithInput(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

	runner.SetLanguage("python_test")
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
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

	runner.SetLanguage("cpp_test")
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
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

	runner.SetLanguage("python_test")
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
