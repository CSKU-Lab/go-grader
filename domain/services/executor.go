package services

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type executorService struct {
	isolateService *IsolateService
	runnerService  *RunnerService
	compareService *CompareService
	logger         *zap.SugaredLogger
}

type ExecutorService interface {
	NewExecutor() ExecutorBuilder
}

func NewExecutorService(logger *zap.SugaredLogger, isolateService *IsolateService, runnerService *RunnerService, compareService *CompareService) ExecutorService {
	return &executorService{
		isolateService: isolateService,
		runnerService:  runnerService,
		compareService: compareService,
		logger:         logger,
	}
}

type ExecutorBuilder interface {
	RunnerID(ID string) ExecutorBuilder
	Files(files []models.File) ExecutorBuilder
	Input(input string) ExecutorBuilder
	Limits(limits *models.Limit) ExecutorBuilder
	TestCaseGroups(testCases []models.TestCaseGroup) ExecutorBuilder
	CompareID(ID string) ExecutorBuilder
	Build() (*executor, execution.Status)
}

type executor struct {
	logger         *zap.SugaredLogger
	compare        *models.LocalCompare
	runner         *models.LocalRunner
	isolateService *IsolateService

	limits         *models.Limit
	files          []models.File
	testCaseGroups []models.TestCaseGroup
	input          string
}

type executorBuilder struct {
	runnerID       string
	compareID      string
	files          []models.File
	input          string
	limits         *models.Limit
	testcaseGroups []models.TestCaseGroup

	runnerService  *RunnerService
	compareService *CompareService
	logger         *zap.SugaredLogger
	isolateService *IsolateService
}

func (r *executorService) NewExecutor() ExecutorBuilder {
	return &executorBuilder{
		runnerService:  r.runnerService,
		compareService: r.compareService,
		logger:         r.logger,
		isolateService: r.isolateService,
	}
}

func (r *executorBuilder) RunnerID(ID string) ExecutorBuilder {
	r.runnerID = ID
	return r
}

func (r *executorBuilder) Files(files []models.File) ExecutorBuilder {
	r.files = files
	return r
}

func (r *executorBuilder) Input(input string) ExecutorBuilder {
	r.input = input
	return r
}

func (r *executorBuilder) CompareID(compareID string) ExecutorBuilder {
	r.compareID = compareID
	return r
}

func (r *executorBuilder) Limits(limits *models.Limit) ExecutorBuilder {
	r.limits = limits
	return r
}

func (r *executorBuilder) TestCaseGroups(testCaseGroups []models.TestCaseGroup) ExecutorBuilder {
	r.testcaseGroups = testCaseGroups
	return r
}

func (r *executorBuilder) Build() (*executor, execution.Status) {
	if r.files == nil {
		return nil, execution.FILE_NOT_FOUND
	}

	exec := &executor{
		logger:         r.logger,
		limits:         r.limits,
		files:          r.files,
		testCaseGroups: r.testcaseGroups,
		input:          r.input,
		isolateService: r.isolateService,
	}

	runner, err := r.runnerService.GetByID(r.runnerID)
	if err != nil {
		return nil, execution.GRADER_ERROR
	}

	exec.runner = runner

	if r.compareID != "" {
		compare, err := r.compareService.GetByID(r.compareID)
		if err != nil {
			return nil, execution.GRADER_ERROR
		}
		exec.compare = compare
	}

	return exec, execution.BUILD_PASSED
}

func (r *executor) Run() (*models.RunResult, error) {
	instance := r.isolateService.NewRunInstance()
	defer func() {
		if err := instance.Cleanup(); err != nil {
			r.logger.Fatalw("Cleanup error", "error", err.Error())
		}
	}()

	for _, file := range r.files {
		err := instance.CreateFile(file.Name, file.Content, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", file.Name, err)
		}
	}

	if r.runner.NeedCompile {
		output, err := instance.CompileUsing(r.runner.Path)
		if err != nil {
			return &models.RunResult{
				Status: execution.COMPILE_FAILED,
				Output: output,
			}, nil
		}
	}

	output, err := instance.Run(r.runner.Path, r.input, r.limits)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.logger.Infow("Run error", "error", err.Error(), "output", output)
			return &models.RunResult{
				Status: execution.RUN_FAILED,
				Output: output,
			}, nil
		}
		return nil, err
	}

	metadata, err := instance.GetMetadata()
	if err != nil {
		return nil, err
	}

	runResult := &models.RunResult{
		WallTime: metadata.WallTime,
		Memory:   metadata.Memory,
		Output:   output,
		Status:   execution.RUN_PASSED,
	}

	return runResult, nil
}

func (r *executor) Grade() (*models.GradeResult, error) {
	if r.compare == nil {
		return &models.GradeResult{
			Status: execution.GRADER_ERROR,
		}, nil
	}

	var totalScore int32
	var totalWallTime float32
	var totalMemory int32
	var totalTestCases int32
	status := execution.RUN_PASSED

	testCaseGroupResults := make([]models.TestCaseGroupResult, 0, len(r.testCaseGroups))
	var mu sync.Mutex
	var eg errgroup.Group
	eg.SetLimit(30)
	for _, tcg := range r.testCaseGroups {
		eg.Go(func() error {
			tcgRunner := &testcaseGroupRunner{
				logger:         r.logger,
				id:             tcg.ID,
				tcs:            tcg.TestCases,
				score:          tcg.Score,
				files:          r.files,
				limits:         r.limits,
				runner:         r.runner,
				compare:        r.compare,
				isolateService: r.isolateService,
			}

			result, metadata, err := tcgRunner.Result()
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()

			if metadata.SomeTestCaseFailed && status != execution.RUN_FAILED {
				status = execution.RUN_FAILED
			}

			testCaseGroupResults = append(testCaseGroupResults, *result)
			totalScore += result.Score
			totalWallTime += metadata.WallTime
			totalMemory += metadata.Memory
			totalTestCases += int32(len(result.Results))
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &models.GradeResult{
		Status:               status,
		TestCaseGroupResults: testCaseGroupResults,
		AvgWallTime:          totalWallTime / float32(totalTestCases),
		AvgMemory:            totalMemory / totalTestCases,
		Score:                totalScore,
	}, nil
}

type runMetadata struct {
	WallTime           float32
	Memory             int32
	SomeTestCaseFailed bool
}

type testcaseGroupRunner struct {
	logger         *zap.SugaredLogger
	id             string
	score          int32
	tcs            []models.TestCase
	files          []models.File
	limits         *models.Limit
	runner         *models.LocalRunner
	compare        *models.LocalCompare
	isolateService *IsolateService
}

func (t *testcaseGroupRunner) Result() (*models.TestCaseGroupResult, *runMetadata, error) {
	var testcaseResults []models.TestCaseResult
	isSomeTestCaseFailed := false
	var totalWallTime float32
	var totalMemory int32

	var mu sync.Mutex
	var eg errgroup.Group
	eg.SetLimit(30)
	for _, tc := range t.tcs {
		eg.Go(func() error {
			result, err := t.GetTestCaseResult(&tc)
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()

			if !isSomeTestCaseFailed && result.Status != execution.RUN_PASSED {
				isSomeTestCaseFailed = true
			}

			testcaseResults = append(testcaseResults, *result)

			totalWallTime += result.WallTime
			totalMemory += result.Memory
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	score := t.score
	if isSomeTestCaseFailed {
		score = 0
	}

	return &models.TestCaseGroupResult{
			ID:      t.id,
			Score:   score,
			Results: testcaseResults,
		}, &runMetadata{
			WallTime:           totalWallTime,
			Memory:             totalMemory,
			SomeTestCaseFailed: isSomeTestCaseFailed,
		}, nil
}

func (t *testcaseGroupRunner) GetTestCaseResult(tc *models.TestCase) (*models.TestCaseResult, error) {
	output, metadata, err := t.RunTestCase(tc)
	if err != nil {
		return nil, err
	}

	status := metadata.status
	message := output
	if status == execution.RUN_PASSED {
		compareResult, err := t.CompareOutput(output, tc.Output)
		if err != nil {
			return nil, err
		}
		t.logger.Infow("Compare result", "test_case_id", tc.ID, "compare_result", compareResult)

		if compareResult != "" {
			status = execution.RUN_FAILED
		}

		message = compareResult
	}

	return &models.TestCaseResult{
		ID:       tc.ID,
		Status:   status,
		Input:    tc.Input,
		Output:   output,
		Message:  message,
		WallTime: metadata.walltime,
		Memory:   metadata.memory,
	}, nil
}

type testcaseMetadata struct {
	status   execution.Status
	walltime float32
	memory   int32
}

func (t *testcaseGroupRunner) RunTestCase(tc *models.TestCase) (output string, tcMet *testcaseMetadata, err error) {
	instance := t.isolateService.NewGradeInstance()
	defer func() {
		if _err := instance.Cleanup(); _err != nil {
			err = errors.Join(err, fmt.Errorf("cleanup error: %w", _err))
		}
	}()

	for _, file := range t.files {
		err := instance.CreateFile(file.Name, file.Content, 0644)
		if err != nil {
			return "create file error", &testcaseMetadata{
				status: execution.GRADER_ERROR,
			}, nil
		}
	}

	if t.runner.NeedCompile {
		output, err := instance.CompileUsing(t.runner.Path)
		if err != nil {
			return output, &testcaseMetadata{
				status: execution.COMPILE_FAILED,
			}, nil
		}
	}

	output, err = instance.Run(t.runner.Path, tc.Input, t.limits)
	if err != nil {
		return output,
			&testcaseMetadata{
				status: execution.GRADER_ERROR,
			},
			nil
	}

	metadata, err := instance.GetMetadata()
	if err != nil {
		return "", &testcaseMetadata{
			status: execution.GRADER_ERROR,
		}, err
	}

	status := execution.RUN_PASSED
	if metadata.FailedStatus != "" {
		switch metadata.FailedStatus {
		case "TO":
			status = execution.TIME_LIMIT_EXCEEDED
		case "RE":
			status = execution.RUNTIME_ERROR
		case "SG":
			status = execution.SIGNAL_ERROR
		case "XX":
			status = execution.GRADER_ERROR
		}

		if t.limits.Memory != 0 && metadata.Memory > t.limits.Memory {
			status = execution.MEMORY_LIMIT_EXCEEDED
		}

	}

	return output, &testcaseMetadata{
		status:   status,
		walltime: metadata.WallTime,
		memory:   metadata.Memory,
	}, nil
}

func (t *testcaseGroupRunner) CompareOutput(output, expected string) (compareResult string, err error) {
	instance := t.isolateService.NewGradeInstance()
	defer func() {
		if _err := instance.Cleanup(); _err != nil {
			err = errors.Join(err, fmt.Errorf("cleanup error: %w", _err))
		}
	}()

	err = instance.CreateFile("output", output, 0644)
	if err != nil {
		return "", err
	}

	err = instance.CreateFile("sol_output", expected, 0644)
	if err != nil {
		return "", err
	}

	output, err = instance.Run(t.compare.Path, "", nil)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() != 1 {
				return "", err
			}
		}
	}

	return instance.GetCompareResult()
}
