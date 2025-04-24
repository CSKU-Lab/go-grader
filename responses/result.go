package responses

import "github.com/CSKU-Lab/go-grader/domain/models"

type Result struct {
	SandboxMetadata *models.Metadata `json:"sandbox_metadata"`
	Output          string           `json:"output"`
	Error           string           `json:"error"`
}
