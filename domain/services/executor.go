package services

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"go.uber.org/zap"
)

type executorService struct {
	isolateService *IsolateService
	runnerService  *RunnerService
	compareService *CompareService
	logger         *zap.SugaredLogger
}

type ExecutorService interface {
	NewExecutor() Executor
}

func NewExecutorService(logger *zap.SugaredLogger, isolateService *IsolateService, runnerService *RunnerService, compareService *CompareService) ExecutorService {
	return &executorService{
		isolateService: isolateService,
		runnerService:  runnerService,
		compareService: compareService,
		logger:         logger,
	}
}

type Result struct {
	StdOut   string
	StdErr   string
	Metadata *models.Metadata
}

type executor struct {
	instance       *IsolateInstance
	runnerService  *RunnerService
	compareService *CompareService
	lang           *models.LocalRunner
	comparePath    string
	limits         *models.Limit
	hasInput       bool
	testcases      []models.TestCase
	logger         *zap.SugaredLogger
}

type Executor interface {
	Cleanup() error
	SetRunner(ID string) error
	SetFiles(files []models.File) error
	SetInput(input string) error
	SetLimits(limits *models.Limit)
	SetTestCases(testCases []models.TestCase)
	SetCompareID(ID string)
	Run() (*models.RunResult, error)
	Grade() (*models.GradeResult, error)
}

func (r *executorService) NewExecutor() Executor {
	return &executor{
		instance:       r.isolateService.NewInstance(),
		runnerService:  r.runnerService,
		compareService: r.compareService,
		logger:         r.logger,
	}
}

func (r *executor) Cleanup() error {
	return r.instance.Cleanup()
}

func (r *executor) SetRunner(ID string) error {
	language, err := r.runnerService.GetByID(ID)
	if err != nil {
		return err
	}

	r.lang = language
	return nil
}

func (r *executor) SetFiles(files []models.File) error {
	for _, file := range files {
		if err := r.instance.CreateFile(file.Name, file.Content, 0655); err != nil {
			return err
		}
	}
	return nil
}

func (r *executor) SetInput(input string) error {
	r.hasInput = true
	return r.instance.CreateFile("input", input, 0644)
}

func (r *executor) compile() (*models.RunResult, error) {
	err := r.instance.CompileUsing(r.lang.Path)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr, err := r.instance.GetError()
			if err != nil {
				return nil, err
			}

			return &models.RunResult{
				Status: execution.COMPILE_FAILED,
				StdOut: "",
				StdErr: stderr,
			}, nil
		}
		return nil, err
	}

	return nil, nil
}

func (r *executor) run() (*Result, error) {
	err := r.instance.Run(r.lang.Path, r.limits, r.hasInput)
	if err != nil {
		return nil, err
	}

	stdOut, err := r.instance.GetOutput()
	if err != nil {
		return nil, err
	}

	stdErr, err := r.instance.GetError()
	if err != nil {
		return nil, err
	}

	metadata, err := r.instance.GetMetadata()
	if err != nil {
		return nil, err
	}

	return &Result{
		StdOut:   stdOut,
		StdErr:   stdErr,
		Metadata: metadata,
	}, nil
}

func (r *executor) SetLimits(limits *models.Limit) {
	r.limits = limits
}

func (r *executor) SetTestCases(testCases []models.TestCase) {
	if r.comparePath == "" {
		r.logger.Fatal("You need to set compare ID before setting test cases")
	}
	r.testcases = testCases
}

func (r *executor) SetCompareID(ID string) {
	compare, err := r.compareService.GetByID(ID)
	if err != nil {
		r.logger.Fatalw("Cannot get compare", "error", err)
	}

	r.comparePath = compare.Path
}

func (r *executor) compare(solOutput string) (string, error) {
	err := r.instance.CreateFile("sol_output", solOutput, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create sol_output file: %w", err)
	}

	err = r.instance.Run(r.comparePath, nil, false)
	if err != nil {
		return "", fmt.Errorf("failed to run compare: %w", err)
	}

	result, err := r.instance.GetCompareResult()
	if err != nil {
		return "", fmt.Errorf("failed to get compare result: %w", err)
	}

	return result, nil
}

func (r *executor) Run() (*models.RunResult, error) {
	if r.lang.NeedCompile {
		result, err := r.compile()
		if err != nil {
			return nil, err
		}

		if result != nil {
			return result, nil
		}
	}

	result, err := r.run()
	if err != nil {
		return nil, fmt.Errorf("failed to run: %w", err)
	}

	runResult := &models.RunResult{
		WallTime: result.Metadata.WallTime,
		Memory:   result.Metadata.Memory,
		StdOut:   result.StdOut,
		StdErr:   result.StdErr,
	}
	if result.StdErr != "" {
		runResult.Status = execution.RUN_FAILED
		return runResult, nil
	}

	r.hasInput = false
	runResult.Status = execution.RUN_PASSED
	return runResult, nil
}

func (r *executor) Grade() (*models.GradeResult, error) {
	if r.lang.NeedCompile {
		result, err := r.compile()
		if err != nil {
			return nil, err
		}

		if result != nil {
			return &models.GradeResult{
				Status: result.Status,
				Error:  result.StdErr,
			}, nil
		}
	}

	gradedStatus := execution.RUN_PASSED
	var testCaseResults []models.TestCaseResult
	for _, testCase := range r.testcases {
		if err := r.SetInput(testCase.Input); err != nil {
			return nil, fmt.Errorf("failed to set input: %w", err)
		}

		testCaseResult := models.TestCaseResult{
			ID:     testCase.ID,
			Status: execution.RUN_FAILED,
		}

		result, err := r.run()
		if err != nil {
			return nil, fmt.Errorf("failed to run: %w", err)
		}

		testCaseResult.StdOut = result.StdOut
		testCaseResult.StdErr = result.StdErr
		testCaseResult.WallTime = result.Metadata.WallTime
		testCaseResult.Memory = result.Metadata.Memory

		isFailed := false

		if result.StdErr != "" {
			testCaseResult.Message = result.StdErr
			isFailed = true
		}

		if result.Metadata.FailedStatus != "" {
			testCaseResult.Message = result.Metadata.FailedMessage
			switch result.Metadata.FailedStatus {
			case "TO":
				testCaseResult.Status = execution.TIME_LIMIT_EXCEEDED
			case "RE":
				testCaseResult.Status = execution.RUNTIME_ERROR
			case "SG":
				testCaseResult.Status = execution.SIGNAL_ERROR
			case "XX":
				testCaseResult.Status = execution.GRADER_ERROR
			}

			if r.limits.Memory != 0 && result.Metadata.Memory > r.limits.Memory {
				testCaseResult.Status = execution.MEMORY_LIMIT_EXCEEDED
			}

			isFailed = true
		}

		err = r.instance.CreateFile("output", result.StdOut, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}

		compareResult, err := r.compare(testCase.Output)
		if err != nil {
			return nil, fmt.Errorf("failed to compare: %w", err)
		}

		if compareResult == "" {
			testCaseResult.Status = execution.RUN_PASSED
		} else {
			isFailed = true
		}

		if isFailed {
			if gradedStatus != execution.RUN_FAILED {
				gradedStatus = execution.RUN_FAILED
			}
		}

		testCaseResults = append(testCaseResults, testCaseResult)

		r.hasInput = false
	}
	return &models.GradeResult{
		Status:          gradedStatus,
		TestCaseResults: testCaseResults,
	}, nil
}
