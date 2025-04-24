package models

type LocalLanguage struct {
	ID          string
	Path        string
	NeedCompile bool
}

type LanguageConfig struct {
	ID          string
	Files       []string
	BuildScript string
	RunScript   string
}
