package models

type LocalRunner struct {
	ID          string
	Path        string
	NeedCompile bool
}

type RunnerConfig struct {
	ID          string
	BuildScript string
	RunScript   string
}
