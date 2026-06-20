package models

type GradeExecution struct {
	ID              string `json:"id"`
	Files           []File `json:"files"`
	TaskID          string `json:"task_id"`
	RunnerID        string `json:"runner_id"`
	CompareScriptID string `json:"compare_script_id,omitempty"`
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

type CompareTestExecution struct {
	Files       []File `json:"files"`
	BuildScript string `json:"build_script"`
	RunScript   string `json:"run_script"`
	RunName     string `json:"run_name"`
	SolOutput   string `json:"sol_output"`
	Output      string `json:"output"`
}
