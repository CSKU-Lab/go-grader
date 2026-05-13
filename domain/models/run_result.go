package models

import "github.com/CSKU-Lab/go-grader/domain/constants/execution"

type RunResult struct {
	ID            string           `json:"id"`
	Status        execution.Status `json:"status"`
	Output        string           `json:"output"`
	WallTime      float32          `json:"wall_time"`
	Memory        int32            `json:"memory"`
	ExitCode      int              `json:"exit_code"`
	CompareResult string           `json:"compare_result"`
}
