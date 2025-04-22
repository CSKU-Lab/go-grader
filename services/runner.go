package services

import (
	"log"

	"github.com/CSKU-Lab/go-grader/models"
)

type runnerService struct {
	isolateService  *IsolateService
	languageService *LanguageService
	compareService  *CompareService
}

func NewRunnerService(isolateService *IsolateService, languageService *LanguageService, compareService *CompareService) *runnerService {
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

func (r *runnerService) NewRunner() *runner {
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

func (r *runner) run() (*Result, error) {
	if r.lang.NeedCompile {
		err := r.instance.CompileUsing(r.lang.Path)
		if err != nil {
			return nil, err
		}
	}

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

func (r *runner) compare(solOutout string) error {
	err := r.instance.CreateFile("sol_output", solOutout, 0644)
	if err != nil {
		return err
	}

	err = r.instance.Run(r.comparePath, nil, false)
	if err != nil {
		return err
	}
	return nil
}

func (r *runner) Run() (*Result, error) {
	result, err := r.run()
	if err != nil {
		return nil, err
	}

	if r.testcases != nil {
		for _, testCase := range r.testcases {
			if err := r.compare(testCase.Output); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
