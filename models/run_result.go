package models

import "github.com/CSKU-Lab/go-grader/constants/execution"

type RunResult struct {
	Status execution.Status
	StdOut string
	StdErr string
}
