package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func TestGradeCompileError(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("cpp_test").
		Files([]models.File{
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
		}).
		CompareID("claude_3.7").
		TestCaseGroups([]models.TestCaseGroup{
			{
				ID:        "group_1",
				TestCases: testdatas.Tasks[0].TestCases,
				Score:     100,
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Grade(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Grade() returns only RUN_PASSED or RUN_FAILED at top level
	// Check individual test cases for specific failure reasons
	if result.Status != execution.RUN_FAILED {
		t.Fatalf("Expected top-level status: RUN_FAILED, Got: %s", result.Status)
	}

	// Verify that individual test cases have COMPILE_FAILED
	foundCompileError := false
	for _, tcGroup := range result.TestCaseGroupResults {
		for _, testcase := range tcGroup.Results {
			if testcase.Status == execution.COMPILE_FAILED {
				foundCompileError = true
				break
			}
		}
		if foundCompileError {
			break
		}
	}

	if !foundCompileError {
		t.Fatalf("Expected to find COMPILE_FAILED in individual test cases")
	}
}

func TestGradePassed(t *testing.T) {
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("cpp_test").
		Files([]models.File{
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
		}).
		CompareID("claude_3.7").
		TestCaseGroups([]models.TestCaseGroup{
			{
				ID:        "group_1",
				TestCases: testdatas.Tasks[0].TestCases,
				Score:     100,
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Grade(context.Background())
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
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("cpp_test").
		Files([]models.File{
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
		}).
		CompareID("claude_3.7").
		TestCaseGroups([]models.TestCaseGroup{
			{
				ID:        "group_1",
				TestCases: testdatas.Tasks[0].TestCases,
				Score:     100,
			},
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Grade(context.Background())
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
	executorService, cleanup := initTest(t)
	defer cleanup()

	var wg sync.WaitGroup
	errChan := make(chan error)

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			executor, status := executorService.NewExecutor().
				RunnerID("cpp_test").
				Files([]models.File{
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
				}).
				CompareID("claude_3.7").
				TestCaseGroups([]models.TestCaseGroup{
					{
						ID:        "group_1",
						TestCases: testdatas.Tasks[0].TestCases,
						Score:     100,
					},
				}).
				Build()

			if status != execution.BUILD_PASSED {
				errChan <- errors.New("build failed: " + string(status))
				return
			}

			_, err := executor.Grade(context.Background())
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
	executorService, cleanup := initTest(t)
	defer cleanup()

	executor, status := executorService.NewExecutor().
		RunnerID("python_test").
		Files([]models.File{
			{
				Name: "main.py",
				Content: `import time
time.sleep(50)`,
			},
		}).
		CompareID("claude_3.7").
		TestCaseGroups([]models.TestCaseGroup{
			{
				ID:        "group_1",
				TestCases: testdatas.Tasks[0].TestCases,
				Score:     100,
			},
		}).
		Limits(&models.Limit{
			CPUTime:      1,
			WallTime:     6,
			CPUExtraTime: 2,
		}).
		Build()

	if status != execution.BUILD_PASSED {
		t.Fatalf("Build failed: %s", status)
	}

	result, err := executor.Grade(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Grade() returns only RUN_PASSED or RUN_FAILED at top level
	if result.Status != execution.RUN_FAILED {
		t.Fatalf("Expected top-level status: RUN_FAILED, Got: %s", result.Status)
	}

	// Check individual test cases for TIME_LIMIT_EXCEEDED
	for _, tcGroup := range result.TestCaseGroupResults {
		for _, testcase := range tcGroup.Results {
			expected := execution.TIME_LIMIT_EXCEEDED
			got := testcase.Status

			if got != expected {
				t.Fatalf("Expected: %s, Got: %s", expected, got)
			}
		}
	}
}
