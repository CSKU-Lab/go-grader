package testdatas

import "github.com/CSKU-Lab/go-grader/models"

var Tasks = []models.Task{
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
	{
		ID: "count",
		LanguageIDs: []string{
			"c_98",
			"cpp_test",
			"python_test",
		},
		TestCases: []models.TestCase{
			{
				Input:  "4",
				Output: "1\n2\n3\n4\n",
			},
			{
				Input:  "1",
				Output: "1\n",
			},
		},
	},
}
