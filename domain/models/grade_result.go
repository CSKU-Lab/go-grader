package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type TestCaseResult struct {
	ID       string
	Status   execution.Status
	Input    string
	Output   string
	Message  string
	WallTime float32
	Memory   int32
}

type TestCaseGroupResult struct {
	ID      string
	Score   int32
	Results []TestCaseResult
}

type GradeResult struct {
	Status               execution.Status
	TestCaseGroupResults []TestCaseGroupResult
	AvgWallTime          float32
	AvgMemory            int32
}
