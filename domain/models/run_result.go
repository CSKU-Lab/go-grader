package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type RunResult struct {
	Status   execution.Status
	StdOut   string
	StdErr   string
	WallTime float32
	Memory   int32
}
