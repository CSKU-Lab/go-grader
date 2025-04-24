package integration

import (
	"sync"
	"testing"

	"github.com/CSKU-Lab/go-grader/constants/execution"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func TestGradeCompileError(t *testing.T) {
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

	result, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	expected := execution.COMPILE_FAILED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
	}
}

func TestGradePassed(t *testing.T) {
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

	result, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	expected := execution.RUN_PASSED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
	}
}

func TestGradeFailed(t *testing.T) {
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

	result, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	if result.Status == execution.COMPILE_FAILED {
		t.Fatalf("Compiled error")
	}

	expected := execution.RUN_FAILED
	got := result.Status

	if got != expected {
		t.Fatalf("Expected: %s, Got: %s", expected, got)
	}
}

func TestGradeMultipleRunners(t *testing.T) {
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

func TestGradeWithLimits(t *testing.T) {
	runnerService := initTest()
	runner := runnerService.NewRunner()
	defer runner.Cleanup()
	defer setup.Cleanup()

	err := runner.SetLanguage("python_test")
	if err != nil {
		t.Fatalf("Cannot set language: %s", err)
	}

	err = runner.SetFiles([]models.File{
		{
			Name: "main.py",
			Content: `import time
time.sleep(50)`,
		},
	})
	if err != nil {
		t.Fatalf("Cannot set files: %s", err)
	}

	runner.SetCompareID("claude_3.7")
	runner.SetTestCases(testdatas.Tasks[0].TestCases)
	runner.SetLimits(&models.Limit{
		CPUTime:      1,
		WallTime:     6,
		CPUExtraTime: 2,
	})

	result, err := runner.Grade()
	if err != nil {
		t.Fatal(err)
	}

	for _, testcase := range result.TestCaseResults {
		expected := execution.TIME_LIMIT_EXCEEDED
		got := testcase.Status

		if got != expected {
			t.Fatalf("Expected: %s, Got: %s", expected, got)
		}
	}
}
