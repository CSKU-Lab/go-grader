package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/SornchaiTheDev/go-grader/constants"
	"github.com/SornchaiTheDev/go-grader/models"
)

type LanguageConfigService struct {
	languages []models.LanguageConfig
}

func NewLanguageConfigService() *LanguageConfigService {
	languages := getLanguagesConfig()
	return &LanguageConfigService{languages: languages}
}

func (s *LanguageConfigService) Get(id string, boxID int) *models.LanguageConfig {
	for _, language := range s.languages {
		if language.ID == id {
			return &models.LanguageConfig{
				Name:          language.Name,
				Version:       language.Version,
				SandboxFiles:  language.SandboxFiles,
				CompileScript: replaceSandboxPath(language.CompileScript, boxID),
				RunScript:     language.RunScript,
			}
		}
	}
	return nil
}

func replaceSandboxPath(script []string, boxID int) []string {
	var substituted []string
	boxPath := fmt.Sprintf(constants.BOX_PATH, boxID)
	for _, word := range script {
		if strings.Contains(word, "$sandbox_path") {
			substituted = append(substituted, strings.ReplaceAll(word, "$sandbox_path", boxPath))
		} else {
			substituted = append(substituted, word)
		}
	}

	return substituted
}

func getLanguagesConfig() []models.LanguageConfig {
	data, err := os.ReadFile(constants.LANGUAGES_CONFIG_FILE)
	if err != nil {
		log.Fatal("Error loading languages config file")
	}

	var config models.LanguagesConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("Error parsing languages config file")
	}

	return config.Languages
}
