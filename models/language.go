package models

type LanguageConfig struct {
	ID          string
	Files       []string
	BuildScript string
	RunScript   string
}
