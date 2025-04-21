package testdatas

import "github.com/CSKU-Lab/go-grader/models"

var tasks = []models.Task{
	{
		ID: "hello_x",
		LanguageIDs: []string{
			"c_98",
			"cpp_test",
		},
		TestCases: []models.TestCase{
			{
				Input:  "World",
				Output: "Hello World\n",
			},
			{
				Input:  "CSKU",
				Output: "Hello CSKU\n",
			},
		},
	},
}
