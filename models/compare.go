package models

type CompareConfig struct {
	ID          string
	Files       []File
	BuildScript string
	RunScript   string
	RunName     string
}
