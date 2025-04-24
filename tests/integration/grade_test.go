package integration

import (
	"sync"
	"testing"
	"time"

	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func TestIsolate_GradeCompileError(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

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
					cout << "Hello " << name << endl
					return 0;
				}`,
		},
	})
	if err != nil {
		t.Fatalf("Cannot set files: %s", err)
	}

	runner.SetCompareID("claude_3.7")
	runner.SetTestCases(testdatas.Tasks[0].TestCases)

	results, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Hour)
	for _, result := range results {
		if result.Status == "FAILED" {
			t.Fatal("Some test case failed.")
		}
	}
}

func TestIsolate_GradePassed(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

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

	runner.SetCompareID("claude_3.7")
	runner.SetTestCases(testdatas.Tasks[0].TestCases)

	results, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	for _, result := range results {
		if result.Status == "FAILED" {
			t.Fatal("Some test case failed.")
		}
	}
}

func TestIsolate_GradeFailed(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

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
					cout << "Hello " << "eiei" << endl;
					return 0;
				}`,
		},
	})
	if err != nil {
		t.Fatalf("Cannot set files: %s", err)
	}

	runner.SetCompareID("claude_3.7")
	runner.SetTestCases(testdatas.Tasks[0].TestCases)

	results, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	failedCount := 0
	for _, result := range results {
		if result.Status == "FAILED" {
			failedCount++
		}
	}

	if failedCount == 0 {
		t.Fatal("All test cases passed.")
	}
}

func TestIsolate_GradeMultipleRunners(t *testing.T) {
	runnerService := initTest()
	defer setup.Cleanup()

	var wg sync.WaitGroup
	errChan := make(chan error)

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := runnerService.NewRunner()
			defer runner.Cleanup()
			runner.SetLanguage("cpp_test")
			runner.SetFiles([]models.File{
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
			runner.SetCompareID("claude_3.7")
			runner.SetTestCases(testdatas.Tasks[0].TestCases)
			_, err := runner.Grade()
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Error in one of the runners: %s", err)
		}
	default:
	}

}
