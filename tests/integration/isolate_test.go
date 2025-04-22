package integration_test

import (
	"context"
	"testing"

	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
	"github.com/CSKU-Lab/go-grader/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func TestIsolate(t *testing.T) {
	setup.Init(testdatas.Languages, testdatas.Compares)

	isolateService := services.NewIsolateService(context.Background())
	languageService := services.NewLanguageService()
	compareService := services.NewCompareService()
	runnerService := services.NewRunnerService(isolateService, languageService, compareService)

	runner := runnerService.NewRunner()
	defer runner.Cleanup()

	err := runner.SetLanguage("cpp_test")
	if err != nil {
		t.Fatalf("Cannot set language: %s", err)
	}

	err = runner.SetFiles([]models.File{
		{
			Name: "main.cpp",
			Content: `#include <iostream>
				using namespace std;
				int main() {
					string name;
					cin >> name;
					cout << "Hello " << name << endl;
					return 0;
				}`,
		},
	})
	if err != nil {
		t.Fatalf("Cannot set files: %s", err)
	}

	runner.SetCompareID("default")
	runner.SetTestCases(testdatas.Tasks[0].TestCases)

	_, err = runner.Run()
	if err != nil {
		t.Fatalf("Cannot run: %s", err)
	}
}
