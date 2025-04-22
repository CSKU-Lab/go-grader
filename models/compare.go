package models

type LocalCompare struct {
	ID      string `json:"id"`
	RunName string `json:"run_name"`
	Path    string `json:"path"`
}

type LocalCompareList struct {
	Compares []LocalCompare `json:"compares"`
}

type CompareConfig struct {
	ID          string
	Files       []File
	BuildScript string
	RunScript   string
	RunName     string
}
