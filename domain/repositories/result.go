package repositories

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/models"
)

type Result interface {
	CreateRunResult(ctx context.Context, id string, result *models.RunResult) error
	GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error)
	CreateGradeResult(ctx context.Context, id string, result *models.GradeResult) error
	GetGradeResultByID(ctx context.Context, id string) (*models.StoredGradeResult, error)
}
