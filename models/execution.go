package models

type Execution struct {
	Code       string `json:"code"`
	LanguageID string `json:"language_id"`
	TaskID     string `json:"task_id"`
}
