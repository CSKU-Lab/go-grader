package testdatas

import "github.com/CSKU-Lab/go-grader/models"

var Languages = []models.LanguageConfig{
	{
		ID:          "c_98",
		BuildScript: "#!/bin/bash\n\ngcc main.c -o main",
		RunScript:   "#!/bin/bash\n\n./main",
		Files:       []string{"main.c"},
	},
}
