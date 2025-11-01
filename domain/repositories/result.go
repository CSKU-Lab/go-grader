package repositories

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/models"
)

type Result interface {
	CreateRunResult(ctx context.Context, id string, result *models.RunResult) error
	CreateGradeResult(ctx context.Context, id string, result *models.GradeResult) error
	GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error)
	GetGradeResultByID(ctx context.Context, id string) (*models.GradeResult, error)
}
