package models

import "github.com/CSKU-Lab/go-grader/constants/execution"

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
	Status          execution.Status
	Error           string
	TestCaseResults []TestCaseResult
}
