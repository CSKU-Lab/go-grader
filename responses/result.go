package responses

import "github.com/SornchaiTheDev/go-grader/models"

type Result struct {
	SandboxMetadata *models.Metadata `json:"sandbox_metadata"`
	Output          string           `json:"output"`
	Error           string           `json:"error"`
}
