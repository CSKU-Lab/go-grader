package services

import (
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
)

type CompareService struct {
	compares []models.LocalCompare
}

func NewCompareService() *CompareService {
	return &CompareService{
		compares: getCompares(),
	}
}

func getCompares() []models.LocalCompare {
	data, err := os.ReadFile(constants.COMPARE_LIST_PATH)
	if err != nil {
		log.Fatalf("Cannot read compares.json: %s", err)
	}

	var l models.LocalCompareList
	err = json.Unmarshal(data, &l)
	if err != nil {
		log.Fatalln("Cannot unmarshal compares.json: ", err)
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
