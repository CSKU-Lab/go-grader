package models

type Execution struct {
	Files    []File `json:"files"`
	RunnerID string `json:"runner_id"`
	TaskID   string `json:"task_id"`
}
