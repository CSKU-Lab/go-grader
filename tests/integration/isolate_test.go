package integration_test

import (
	"log"
	"testing"

	"github.com/CSKU-Lab/go-grader/setup"
	"github.com/CSKU-Lab/go-grader/tests/testdatas"
)

func TestIsolate(t *testing.T) {
	setup.Init(testdatas.Languages, testdatas.Compares)
	log.Println("Setup config successfully")
}
