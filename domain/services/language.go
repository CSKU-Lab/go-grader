package services

import (
	"errors"
	"log"
	"os"
	"slices"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
)

type RunnerService struct {
	runners []models.LocalRunner
}

func NewRunnerService() *RunnerService {
	runners, err := getRunners()
	if err != nil {
		log.Fatalf("Cannot get runners: %s", err)
	}

	return &RunnerService{
		runners: runners,
	}

}

func isNeedCompile(runnerPath string) (bool, error) {
	entries, err := os.ReadDir(runnerPath)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(entries, func(entry os.DirEntry) bool {
		return entry.Name() == "build_script.sh"
	}), nil

}

func getRunners() ([]models.LocalRunner, error) {
	entries, err := os.ReadDir(constants.RUNNER_DIR)
	if err != nil {
		return nil, err
	}

	var languages []models.LocalRunner

	for _, file := range entries {
		needCompile, err := isNeedCompile(constants.RUNNER_DIR + "/" + file.Name())
		if err != nil {
			return nil, err
		}

		languages = append(languages, models.LocalRunner{
			ID:          file.Name(),
			Path:        constants.RUNNER_DIR + "/" + file.Name(),
			NeedCompile: needCompile,
		})
	}

	return languages, nil
}

func (l *RunnerService) GetByID(ID string) (*models.LocalRunner, error) {
	for _, runner := range l.runners {
		if runner.ID == ID {
			return &runner, nil
		}
	}

	return nil, errors.New("runner not found")
}
