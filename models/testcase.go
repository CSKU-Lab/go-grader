package models

import "github.com/CSKU-Lab/go-grader/constants/execution"

type TestCase struct {
	ID     string
	Input  string
	Output string
}

type TestCaseResult struct {
	ID      string
	Status  execution.Status
	Message string
	CPUTime float32
	Memory  int32
}
