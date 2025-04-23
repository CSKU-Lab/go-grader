package models

type TestCase struct {
	ID     string
	Input  string
	Output string
}

type TestCaseResult struct {
	ID      string
	Status  string
	Output  string
	Message string
}
