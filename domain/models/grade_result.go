package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type TestCaseResult struct {
	ID       string
	Status   execution.Status
	Message  string
	WallTime float32
	Memory   int32
	StdOut   string
	StdErr   string
}

type GradeResult struct {
	ID              string
	Status          execution.Status
	TestCaseResults []TestCaseResult
	AvgWallTime     float32
	AvgMemory       int32
}

type StoredGradeResult struct {
	ID              string
	Status          execution.Status
	TestCaseResults []StoredTestCaseResult
	AvgWallTime     float32
	AvgMemory       int32
}

type StoredTestCaseResult struct {
	ID       string
	Status   execution.Status
	Output   string
	Message  string
	WallTime float32
	Memory   int32
}
