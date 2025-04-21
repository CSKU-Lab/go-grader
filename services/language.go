package services

import (
	"errors"
	"log"
	"os"
	"slices"

	"github.com/CSKU-Lab/go-grader/constants"
)

type LanguageService struct {
	languages []Language
}

func NewLanguageService() *LanguageService {
	languages, err := getLanguages()
	if err != nil {
		log.Fatalf("Cannot get languages: %s", err)
	}

	return &LanguageService{
		languages: languages,
	}

}

type Language struct {
	ID          string
	Path        string
	NeedCompile bool
}

func isNeedCompile(langPath string) (bool, error) {
	entries, err := os.ReadDir(langPath)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(entries, func(entry os.DirEntry) bool {
		return entry.Name() == "build_script.sh"
	}), nil

}

func getLanguages() ([]Language, error) {
	LANGDIR := constants.CONFIG_DIR + "/languages"

	entries, err := os.ReadDir(LANGDIR)
	if err != nil {
		return nil, err
	}

	var languages []Language

	for _, file := range entries {
		needCompile, err := isNeedCompile(LANGDIR + "/" + file.Name())
		if err != nil {
			return nil, err
		}

		languages = append(languages, Language{
			ID:          file.Name(),
			Path:        LANGDIR + "/" + file.Name(),
			NeedCompile: needCompile,
		})
	}

	return languages, nil
}

func (l *LanguageService) GetByID(ID string) (*Language, error) {
	for _, lang := range l.languages {
		if lang.ID == ID {
			return &lang, nil
		}
	}

	return nil, errors.New("language not found")
}
