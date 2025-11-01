package services

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"go.uber.org/zap"
)

type CompareService struct {
	compares []models.LocalCompare
	logger   *zap.SugaredLogger
}

func NewCompareService(logger *zap.SugaredLogger) *CompareService {
	return &CompareService{
		compares: getCompares(logger),
		logger:   logger,
	}
}

func getCompares(logger *zap.SugaredLogger) []models.LocalCompare {
	data, err := os.ReadFile(constants.COMPARE_LIST_PATH)
	if err != nil {
		logger.Fatalf("Cannot read compares.json: %s", err)
	}

	var l models.LocalCompareList
	err = json.Unmarshal(data, &l)
	if err != nil {
		logger.Fatalw("Cannot unmarshal compares.json", "error", err)
	}

	return l.Compares
}

func (c *CompareService) GetByID(ID string) (*models.LocalCompare, error) {
	for _, compare := range c.compares {
		if compare.ID == ID {
			return &compare, nil
		}
	}

	return nil, errors.New("Compare not found")
}
