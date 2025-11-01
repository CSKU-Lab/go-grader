package repositories

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/models"
)

type Result interface {
	CreateRunResult(ctx context.Context, id string, result *models.RunResult) error
	GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error)
	CreateGradedResult(ctx context.Context, id string, result *models.GradedResult) error
	GetGradedResultByID(ctx context.Context, id string) (*models.StoredGradedResult, error)
}
