package models

import "github.com/CSKU-Lab/go-grader/constants/execution"

type GradeResult struct {
	Status          execution.Status
	Error           string
	TestCaseResults []TestCaseResult
}
