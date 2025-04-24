package services

import (
	"errors"
	"fmt"
	"log"
	"os/exec"

	"github.com/CSKU-Lab/go-grader/constants/execution"
	"github.com/CSKU-Lab/go-grader/models"
)

type runnerService struct {
	isolateService  *IsolateService
	languageService *LanguageService
	compareService  *CompareService
}

type RunnerService interface {
	NewRunner() Runner
}

func NewRunnerService(isolateService *IsolateService, languageService *LanguageService, compareService *CompareService) RunnerService {
	return &runnerService{
		isolateService:  isolateService,
		languageService: languageService,
		compareService:  compareService,
	}
}

type Result struct {
	StdOut   string
	StdErr   string
	Metadata *models.Metadata
}

type runner struct {
	instance        *IsolateInstance
	languageService *LanguageService
	compareService  *CompareService
	lang            *models.LocalLanguage
	comparePath     string
	limits          *models.Limit
	hasInput        bool
	testcases       []models.TestCase
}

type Runner interface {
	Cleanup() error
	SetLanguage(ID string) error
	SetFiles(files []models.File) error
	SetInput(input string) error
	SetLimits(limits *models.Limit)
	SetTestCases(testCases []models.TestCase)
	SetCompareID(ID string)
	Run() (*models.RunResult, error)
	Grade() (*models.GradeResult, error)
}

func (r *runnerService) NewRunner() Runner {
	return &runner{
		instance:        r.isolateService.NewInstance(),
		languageService: r.languageService,
		compareService:  r.compareService,
	}
}

func (r *runner) Cleanup() error {
	return r.instance.Cleanup()
}

func (r *runner) SetLanguage(ID string) error {
	language, err := r.languageService.GetByID(ID)
	if err != nil {
		return err
	}

	r.lang = language
	return nil
}

func (r *runner) SetFiles(files []models.File) error {
	for _, file := range files {
		if err := r.instance.CreateFile(file.Name, file.Content, 0655); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) SetInput(input string) error {
	r.hasInput = true
	return r.instance.CreateFile("input", input, 0644)
}

func (r *runner) compile() (*models.RunResult, error) {
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

func (r *runner) run() (*Result, error) {
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

func (r *runner) SetLimits(limits *models.Limit) {
	r.limits = limits
}

func (r *runner) SetTestCases(testCases []models.TestCase) {
	if r.comparePath == "" {
		log.Fatalln("You need to set compare ID before setting test cases")
	}
	r.testcases = testCases
}

func (r *runner) SetCompareID(ID string) {
	compare, err := r.compareService.GetByID(ID)
	if err != nil {
		log.Fatalln("Cannot get compare: ", err)
	}

	r.comparePath = compare.Path
}

func (r *runner) compare(solOutput string) (string, error) {
	err := r.instance.CreateFile("sol_output", solOutput, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create sol_output file: %w", err)
	}

	err = r.instance.Run(r.comparePath, nil, false)
	if err != nil {
		return "", fmt.Errorf("failed to run compare: %w", err)
	}

	//TODO: Test if compare exit with code 1

	result, err := r.instance.GetCompareResult()
	if err != nil {
		return "", fmt.Errorf("failed to get compare result: %w", err)
	}

	return result, nil
}

func (r *runner) Run() (*models.RunResult, error) {
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
	if result.StdErr != "" {
		return &models.RunResult{
			Status: execution.RUN_FAILED,
			StdOut: result.StdOut,
			StdErr: result.StdErr,
		}, nil
	}

	r.hasInput = false
	return &models.RunResult{
		Status: execution.RUN_PASSED,
		StdOut: result.StdOut,
		StdErr: result.StdErr,
	}, nil
}

func (r *runner) Grade() (*models.GradeResult, error) {
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

	var testCaseResults []models.TestCaseResult
	for _, testCase := range r.testcases {
		if err := r.SetInput(testCase.Input); err != nil {
			return nil, fmt.Errorf("failed to set input: %w", err)
		}

		result, err := r.run()
		if err != nil {
			return nil, fmt.Errorf("failed to run: %w", err)
		}

		err = r.instance.CreateFile("output", result.StdOut, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}

		compareResult, err := r.compare(testCase.Output)
		if err != nil {
			return nil, fmt.Errorf("failed to compare: %w", err)
		}

		status := execution.RUN_FAILED
		if compareResult == "" {
			status = execution.RUN_PASSED
		}

		testCaseResult := models.TestCaseResult{
			ID:             testCase.ID,
			Status:         status,
			CompareMessage: compareResult,
		}

		testCaseResults = append(testCaseResults, testCaseResult)

		r.hasInput = false
	}
	return &models.GradeResult{
		Status:          execution.RUN_PASSED,
		TestCaseResults: testCaseResults,
	}, nil
}
