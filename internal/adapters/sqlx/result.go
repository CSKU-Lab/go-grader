package sqlx

import (
	"context"

	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/repositories"
	"github.com/jmoiron/sqlx"
)

type sqlxInstance struct {
	db *sqlx.DB
}

type runResultRow struct {
	ID       string  `db:"id"`
	Status   string  `db:"status"`
	Output   string  `db:"output"`
	WallTime float32 `db:"wall_time"`
	Memory   int32   `db:"memory"`
}

type gradedResultRow struct {
	ID          string  `db:"id"`
	Status      string  `db:"status"`
	AvgWallTime float32 `db:"avg_wall_time"`
	AvgMemory   int32   `db:"avg_memory"`
}

type testCaseResultRow struct {
	TestCaseID    string  `db:"test_case_id"`
	GradeResultID string  `db:"grade_result_id"`
	Status        string  `db:"status"`
	Output        string  `db:"output"`
	Message       string  `db:"message"`
	WallTime      float32 `db:"wall_time"`
	Memory        int32   `db:"memory"`
}

func NewSQLXInstance(db *sqlx.DB) repositories.Result {
	return &sqlxInstance{db: db}
}

func (s *sqlxInstance) CreateRunResult(ctx context.Context, id string, result *models.RunResult) error {
	query := `INSERT INTO run_results (id, status, output, wall_time, memory) VALUES ($1, $2, $3, $4, $5)`

	output := result.StdOut
	if output == "" {
		output = result.StdErr
	}

	_, err := s.db.ExecContext(ctx, query, id, result.Status, output, result.WallTime, result.Memory)
	if err != nil {
		return err
	}

	return nil
}

func (s *sqlxInstance) CreateGradeResult(ctx context.Context, id string, result *models.GradeResult) error {
	gradeResultQuery := `INSERT INTO grade_results (id, status, avg_wall_time, avg_memory) VALUES ($1, $2, $3, $4)`

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, gradeResultQuery, id, result.Status, result.AvgWallTime, result.AvgMemory)
	if err != nil {
		return err
	}

	testCaseResultQuery := `INSERT INTO test_case_results (test_case_id, grade_result_id, status, output, message, wall_time, memory) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for _, testCaseResult := range result.TestCaseResults {
		output := testCaseResult.StdOut
		if output == "" {
			output = testCaseResult.StdErr
		}

		_, err = tx.ExecContext(ctx, testCaseResultQuery, testCaseResult.ID, id, testCaseResult.Status, output, testCaseResult.Message, testCaseResult.WallTime, testCaseResult.Memory)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqlxInstance) GetRunResultByID(ctx context.Context, id string) (*models.StoredRunResult, error) {
	query := `SELECT id, status, output, wall_time, memory FROM run_results WHERE id = $1`

	var runResult runResultRow
	err := s.db.GetContext(ctx, &runResult, query, id)
	if err != nil {
		return nil, err
	}

	return &models.StoredRunResult{
		ID:       runResult.ID,
		Status:   execution.Status(runResult.Status),
		Output:   runResult.Output,
		WallTime: runResult.WallTime,
		Memory:   runResult.Memory,
	}, nil
}

func (s *sqlxInstance) GetGradeResultByID(ctx context.Context, id string) (*models.StoredGradeResult, error) {
	gradeResultQuery := `SELECT id, status, avg_wall_time, avg_memory FROM grade_results WHERE id = $1`

	var gradeResult gradedResultRow
	err := s.db.GetContext(ctx, &gradeResult, gradeResultQuery, id)
	if err != nil {
		return nil, err
	}

	var testCaseResults []models.StoredTestCaseResult

	testCaseQuery := `SELECT test_case_id, grade_result_id, status, output, message, wall_time, memory FROM test_case_results WHERE grade_result_id = $1`

	var testCaseResultRows []testCaseResultRow
	err = s.db.SelectContext(ctx, &testCaseResultRows, testCaseQuery, id)
	if err != nil {
		return nil, err
	}

	for _, row := range testCaseResultRows {
		testCaseResults = append(testCaseResults, models.StoredTestCaseResult{
			ID:       row.TestCaseID,
			Status:   execution.Status(row.Status),
			Output:   row.Output,
			Message:  row.Message,
			WallTime: row.WallTime,
			Memory:   row.Memory,
		})
	}

	return &models.StoredGradeResult{
		ID:              gradeResult.ID,
		Status:          execution.Status(gradeResult.Status),
		AvgWallTime:     gradeResult.AvgWallTime,
		AvgMemory:       gradeResult.AvgMemory,
		TestCaseResults: testCaseResults,
	}, nil

}
