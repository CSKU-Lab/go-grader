package models

type LanguageConfig struct {
	Name          string   `json:"name"`
	ID            string   `json:"id"`
	Version       string   `json:"version"`
	SandboxFiles  []string `json:"sandbox_files"`
	CompileScript []string `json:"compile_script"`
	RunScript     []string `json:"run_script"`
}

type LanguagesConfig struct {
	Languages []LanguageConfig `json:"languages"`
}
