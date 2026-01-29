package services

import (
	"errors"
	"fmt"
	"os/exec"

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

	isolateService *IsolateService
	runnerService  *RunnerService
	compareService *CompareService
	logger         *zap.SugaredLogger
}

func (r *executorService) NewExecutor() ExecutorBuilder {
	return &executorBuilder{
		runnerService:  r.runnerService,
		isolateService: r.isolateService,
		compareService: r.compareService,
		logger:         r.logger,
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
	instance := r.isolateService.NewInstance()

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
			r.logger.Errorw("Run error", "error", err.Error(), "output", output)
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

	err = instance.Cleanup()
	if err != nil {
		return nil, err
	}

	runResult := &models.RunResult{
		WallTime: metadata.WallTime,
		Memory:   metadata.Memory,
		Output:   output,
	}

	runResult.Status = execution.RUN_PASSED
	return runResult, nil
}

func (r *executor) Grade() (*models.GradeResult, error) {
	if r.compare == nil {
		return nil, errors.New("compare must be provided for grading")
	}

	if r.runner.NeedCompile {
		instance := r.isolateService.NewInstance()
		_, err := instance.CompileUsing(r.runner.Path)
		if err != nil {
			return &models.GradeResult{
				Status: execution.COMPILE_FAILED,
			}, nil
		}

		err = instance.Cleanup()
		if err != nil {
			return nil, err
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
	resultMetadata := &resultMetadata{}
	testCaseResults := make([]models.TestCaseResult, 0, len(tcs))
	var eg errgroup.Group
	for _, tc := range tcs {
		eg.Go(func() error {
			instance := r.isolateService.NewInstance()
			testCaseResult := models.TestCaseResult{
				ID:     tc.ID,
				Status: execution.RUN_FAILED,
			}

			for _, file := range r.files {
				err := instance.CreateFile(file.Name, file.Content, 0644)
				if err != nil {
					return err
				}
			}

			output, err := instance.Run(r.runner.Path, tc.Input, r.limits)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					testCaseResult.Output = output
					testCaseResult.Status = execution.RUN_FAILED
				} else {
					return err
				}
			}

			testCaseResult.Output = output

			metadata, err := instance.GetMetadata()
			if err != nil {
				return err
			}

			testCaseResult.WallTime = metadata.WallTime
			testCaseResult.Memory = metadata.Memory

			err = instance.Cleanup()
			if err != nil {
				r.logger.Errorw("Cleanup error", "error", err.Error())
			}

			resultMetadata.totalWallTime += metadata.WallTime
			resultMetadata.totalMemory += metadata.Memory

			if metadata.FailedStatus != "" {
				testCaseResult.Message = metadata.FailedMessage
				switch metadata.FailedStatus {
				case "TO":
					testCaseResult.Status = execution.TIME_LIMIT_EXCEEDED
				case "RE":
					testCaseResult.Status = execution.RUNTIME_ERROR
				case "SG":
					testCaseResult.Status = execution.SIGNAL_ERROR
				case "XX":
					testCaseResult.Status = execution.GRADER_ERROR
				}

				if r.limits.Memory != 0 && metadata.Memory > r.limits.Memory {
					testCaseResult.Status = execution.MEMORY_LIMIT_EXCEEDED
				}

				resultMetadata.isFailed = true
			}

			if resultMetadata.isFailed {
				testCaseResults = append(testCaseResults, testCaseResult)
				return nil
			}

			instance = r.isolateService.NewInstance()

			err = instance.CreateFile("output", output, 0644)
			if err != nil {
				return err
			}

			err = instance.CreateFile("sol_output", tc.Output, 0644)
			if err != nil {
				return err
			}

			output, err = instance.Run(r.compare.Path, "", nil)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					if exitErr.ExitCode() != 1 {
						return err
					}
				}
			}

			compareResult, err := instance.GetCompareResult()
			if err != nil {
				return err
			}

			err = instance.Cleanup()
			if err != nil {
				r.logger.Errorw("Cleanup error", "error", err.Error())
			}

			if compareResult == "" {
				testCaseResult.Status = execution.RUN_PASSED
			} else {

				testCaseResult.Message = compareResult
				resultMetadata.isFailed = true
			}

			testCaseResults = append(testCaseResults, testCaseResult)
			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, nil, err
	}

	return testCaseResults, resultMetadata, nil
}
