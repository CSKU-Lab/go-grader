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
	t.Log("Setup complete")

	isolateService := services.NewIsolateService(context.Background())
	languageService := services.NewLanguageService()
	runnerService := services.NewRunnerService(isolateService, languageService)

	result, err := runnerService.Run(
		"cpp_test",
		[]models.File{
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
		},
	)
	if err != nil {
		t.Fatalf("Error from runner %s", err)
	}

	t.Logf("StdOut: %s", result.StdOut)

}
