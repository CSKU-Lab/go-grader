package models

type GradeExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	TaskID   string `json:"task_id"`
}

type RunExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	Input    string `json:"input"`
	RunnerID string `json:"runner_id"`
}
