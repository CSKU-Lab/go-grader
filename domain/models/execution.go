package models

type GradeExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	TaskID   string `json:"task_id"`
	RunnerID string `json:"runner_id"`
}

type RunExecution struct {
	ID       string `json:"id"`
	Files    []File `json:"files"`
	Input    string `json:"input"`
	RunnerID string `json:"runner_id"`
	Limit    *Limit `json:"limit"`
}

type RunnerTestExecution struct {
	InitialFiles []File `json:"initial_files"`
	Input        string `json:"input"`
	RunScript    string `json:"run_script"`
	BuildScript  string `json:"build_script"`
}
