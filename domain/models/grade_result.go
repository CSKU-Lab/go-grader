package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type TestCaseResult struct {
	ID       string           `json:"id"`
	Status   execution.Status `json:"status"`
	Input    string           `json:"input"`
	Output   string           `json:"output"`
	Message  string           `json:"message"`
	WallTime float32          `json:"wall_time"`
	Memory   int32            `json:"memory"`
}

type TestCaseGroupResult struct {
	ID      string           `json:"id"`
	Score   int32            `json:"score"`
	Results []TestCaseResult `json:"results"`
}

type GradeResult struct {
	Status               execution.Status      `json:"status"`
	TestCaseGroupResults []TestCaseGroupResult `json:"test_case_group_results"`
	AvgWallTime          float32               `json:"avg_wall_time"`
	AvgMemory            int32                 `json:"avg_memory"`
}
