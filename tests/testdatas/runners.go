package testdatas

import "github.com/CSKU-Lab/go-grader/domain/models"

var Runners = []models.RunnerConfig{
	{
		ID:          "c_98",
		BuildScript: "#!/bin/bash\n\ngcc main.c -o main",
		RunScript:   "#!/bin/bash\n\n./main",
	},
	{
		ID:          "cpp_test",
		BuildScript: "#!/bin/bash\n\n g++ main.cpp -o main",
		RunScript:   "#!/bin/bash\n\n./main",
	},
	{
		ID:        "python_test",
		RunScript: "#!/bin/bash\n\npython3 main.py",
	},
}
