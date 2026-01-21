package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type RunResult struct {
	ID       string
	Status   execution.Status
	Output   string
	WallTime float32
	Memory   int32
}
