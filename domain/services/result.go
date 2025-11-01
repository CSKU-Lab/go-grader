package services

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/repositories"
)

type ResultService interface {
	CreateRunResult(ctx context.Context, id string, result *models.RunResult) error
	CreateGradeResult(ctx context.Context, id string, result *models.GradeResult) error
	GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error)
	GetGradeResultByID(ctx context.Context, id string) (*models.StoredGradeResult, error)
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

func (rs *resultService) CreateGradeResult(ctx context.Context, id string, result *models.GradeResult) error {
	return rs.repo.CreateGradeResult(ctx, id, result)
}

func (rs *resultService) GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error) {
	return rs.repo.GetRunResultByID(ctx, id)
}

func (rs *resultService) GetGradeResultByID(ctx context.Context, id string) (*models.StoredGradeResult, error) {
	return rs.repo.GetGradeResultByID(ctx, id)
}
