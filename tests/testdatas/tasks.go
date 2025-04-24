package testdatas

import "github.com/CSKU-Lab/go-grader/domain/models"

var Tasks = []models.Task{
	{
		ID: "hello_x",
		TestCases: []models.TestCase{
			{
				ID:     "hello_1",
				Input:  "World",
				Output: "Hello World\n",
			},
			{
				ID:     "hello_2",
				Input:  "CSKU",
				Output: "Hello CSKU\n",
			},
		},
	},
	{
		ID: "count",
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
