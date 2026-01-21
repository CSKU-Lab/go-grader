package models

type TestCase struct {
	ID     string
	Input  string
	Output string
}

type TestCaseGroup struct {
	ID        string
	TestCases []TestCase
	Score     int32
}
