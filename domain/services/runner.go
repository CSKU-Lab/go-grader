package services

import (
	"errors"
	"os"
	"slices"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"go.uber.org/zap"
)

type RunnerService struct {
	runners []models.LocalRunner
	logger  *zap.SugaredLogger
}

func NewRunnerService(logger *zap.SugaredLogger) *RunnerService {
	runners, err := getRunners()
	if err != nil {
		logger.Fatalf("Cannot get runners: %s", err)
	}

	return &RunnerService{
		runners: runners,
		logger:  logger,
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

	var runners []models.LocalRunner

	for _, file := range entries {
		needCompile, err := isNeedCompile(constants.RUNNER_DIR + "/" + file.Name())
		if err != nil {
			return nil, err
		}

		runners = append(runners, models.LocalRunner{
			ID:          file.Name(),
			Path:        constants.RUNNER_DIR + "/" + file.Name(),
			NeedCompile: needCompile,
		})
	}

	return runners, nil
}

func (l *RunnerService) GetByID(ID string) (*models.LocalRunner, error) {
	for _, runner := range l.runners {
		if runner.ID == ID {
			return &runner, nil
		}
	}

	return nil, errors.New("runner not found")
}
