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
	testCaseGroups []models.TestCaseGroup
	logger         *zap.SugaredLogger
}

type Executor interface {
	Cleanup() error
	SetRunner(ID string) error
	SetFiles(files []models.File) error
	SetInput(input string) error
	SetLimits(limits *models.Limit)
	SetTestCaseGroups(testCases []models.TestCaseGroup)
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

func (r *executor) SetTestCaseGroups(testCaseGroups []models.TestCaseGroup) {
	if r.comparePath == "" {
		r.logger.Fatal("You need to set compare ID before setting test cases")
	}
	r.testCaseGroups = testCaseGroups
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

		// this mean compilation failed
		if result != nil {
			return &models.GradeResult{
				Status: result.Status,
			}, nil
		}
	}

	totalWallTime := float32(0)
	totalMemory := int32(0)
	gradedStatus := execution.RUN_PASSED
	testCaseGroupResults := make([]models.TestCaseGroupResult, 0, len(r.testCaseGroups))
	for _, group := range r.testCaseGroups {
		groupResults, metadata, err := r.generateTestCaseResults(group.TestCases)
		if err != nil {
			return nil, fmt.Errorf("failed to generate test case results: %w", err)
		}

		score := int32(0)
		if !metadata.isFailed {
			score = group.Score
		}

		testCaseGroupResults = append(testCaseGroupResults, models.TestCaseGroupResult{
			ID:      group.ID,
			Results: groupResults,
			Score:   score,
		})

		totalWallTime += metadata.totalWallTime
		totalMemory += metadata.totalMemory

		if metadata.isFailed {
			gradedStatus = execution.RUN_FAILED
		}
	}

	tcsCount := 0
	for _, group := range r.testCaseGroups {
		tcsCount += len(group.TestCases)
	}

	return &models.GradeResult{
		Status:               gradedStatus,
		TestCaseGroupResults: testCaseGroupResults,
		AvgWallTime:          totalWallTime / float32(tcsCount),
		AvgMemory:            totalMemory / int32(tcsCount),
	}, nil
}

type resultMetadata struct {
	totalWallTime float32
	totalMemory   int32
	isFailed      bool
}

func (r *executor) generateTestCaseResults(tcs []models.TestCase) ([]models.TestCaseResult, *resultMetadata, error) {
	metadata := &resultMetadata{}
	testCaseResults := make([]models.TestCaseResult, 0, len(tcs))
	for _, tc := range tcs {
		if err := r.SetInput(tc.Input); err != nil {
			return nil, nil, fmt.Errorf("failed to set input: %w", err)
		}

		result, err := r.run()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to run: %w", err)
		}

		metadata.totalWallTime += result.Metadata.WallTime
		metadata.totalMemory += result.Metadata.Memory

		output := result.StdOut
		if result.StdErr != "" {
			output = result.StdErr
		}

		testCaseResult := models.TestCaseResult{
			ID:       tc.ID,
			Status:   execution.RUN_FAILED,
			Output:   output,
			WallTime: result.Metadata.WallTime,
			Memory:   result.Metadata.Memory,
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

			metadata.isFailed = true
		}

		err = r.instance.CreateFile("output", result.StdOut, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create output file: %w", err)
		}

		compareResult, err := r.compare(tc.Output)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compare: %w", err)
		}

		if compareResult == "" {
			testCaseResult.Status = execution.RUN_PASSED
		} else {
			metadata.isFailed = true
		}

		testCaseResults = append(testCaseResults, testCaseResult)

		r.hasInput = false
	}

	return testCaseResults, metadata, nil
}
