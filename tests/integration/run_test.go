package integration

import (
	"log"
	"testing"

	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/setup"
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
	log.Println(result.StdOut)

	expected := "Hello World\n"
	got := result.StdOut

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
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

	log.Println(result)
}
