package services

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/repositories"
)

type ResultService interface {
	CreateRunResult(ctx context.Context, id string, result *models.RunResult) error
	CreateGradeResult(ctx context.Context, id string, result *models.GradedResult) error
	GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error)
	GetGradedResultByID(ctx context.Context, id string) (*models.StoredGradedResult, error)
}

type resultService struct {
	repo repositories.Result
}

func NewResultService(repo repositories.Result) ResultService {
	return &resultService{
		repo: repo,
	}
}

func (rs *resultService) CreateRunResult(ctx context.Context, id string, result *models.RunResult) error {
	return rs.repo.CreateRunResult(ctx, id, result)
}

func (rs *resultService) CreateGradeResult(ctx context.Context, id string, result *models.GradedResult) error {
	return rs.repo.CreateGradedResult(ctx, id, result)
}

func (rs *resultService) GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error) {
	return rs.repo.GetRunResultByID(ctx, id)
}

func (rs *resultService) GetGradedResultByID(ctx context.Context, id string) (*models.StoredGradedResult, error) {
	return rs.repo.GetGradedResultByID(ctx, id)
}
