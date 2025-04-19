package models

type Execution struct {
	Files      []File `json:"files"`
	LanguageID string `json:"language_id"`
	TaskID     string `json:"task_id"`
}
