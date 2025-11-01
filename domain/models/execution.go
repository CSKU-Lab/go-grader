package models

type GradeExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	RunnerID string `json:"runner_id"`
	TaskID   string `json:"task_id"`
}

type RunExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	Input    string `json:"input"`
	RunnerID string `json:"runner_id"`
}
