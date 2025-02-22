package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"reflect"

	"github.com/SornchaiTheDev/go-grader/models"
)

type isolateService struct {
	ctx          context.Context
	boxID        int
	boxPath      string
	metadataPath string
}

func NewIsolateService(ctx context.Context, boxID int) *isolateService {
	boxPath := fmt.Sprintf("/var/local/lib/isolate/%d/box", boxID)
	metadataPath := fmt.Sprintf("/tmp/metadata")

	return &isolateService{ctx: ctx, boxID: boxID, boxPath: boxPath, metadataPath: metadataPath}
}

func (s *isolateService) execute(args ...string) error {
	boxID := fmt.Sprintf("--box-id=%d", s.boxID)
	cmd := exec.CommandContext(s.ctx, "isolate", append([]string{boxID}, args...)...)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return errors.New(err.Error() + " : " + stdErr.String())
	}
	return nil
}

func (s *isolateService) Init() error {
	log.Printf("Initializing sandbox (ID %d)...", s.boxID)
	err := s.execute("--init")
	return err
}

func (s *isolateService) Cleanup() error {
	log.Printf("Cleaning up sandbox (ID %d)...", s.boxID)
	err := s.execute("--cleanup")
	return err
}

func (s *isolateService) CreateFile(name string, content string) error {
	log.Printf("Creating file %s...", name)
	filePath := fmt.Sprintf("%s/%s", s.boxPath, name)
	cmd := exec.CommandContext(s.ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", content, filePath))
	err := cmd.Run()
	return err
}

func (s *isolateService) Run(programPath string, runnerCmd []string, limit *models.Limit) error {
	log.Println("Running program...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--stdin=input",
		"--meta=" + s.metadataPath,
		"--stdout=output.txt",
		"--run",
		"--",
		programPath,
	}

	args = append(args[:4], append(runnerCmd, args[4:]...)...)
	args = append(_limits, args...)

	err := s.execute(args...)

	return err
}

func (s *isolateService) GetOutput() (string, error) {
	log.Println("Getting output...")
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.CommandContext(s.ctx, "cat", fmt.Sprintf("%s/output.txt", s.boxPath))
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stdErr.String())
	}

	return stdOut.String(), nil
}

func (s *isolateService) GetMetadata() (string, error) {
	log.Println("Getting Metadata...")
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.CommandContext(s.ctx, "cat", s.metadataPath)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stdErr.String())
	}

	return stdOut.String(), nil
}

func getLimitArgs(limit *models.Limit) []string {
	if limit == nil {
		return []string{}
	}

	v := reflect.ValueOf(limit).Elem()
	t := v.Type()

	args := make([]string, 0)

	for i := range v.NumField() {
		field := v.Field(i)
		arg := t.Field(i).Tag.Get("arg")

		if field.Kind() == reflect.Float32 {
			if field.Float() == 0 {
				continue
			}
			args = append(args, fmt.Sprintf("%s=%f", arg, field.Float()))
		} else if field.Kind() == reflect.Bool {
			if !field.Bool() {
				continue
			}
			args = append(args, fmt.Sprintf("%s", arg))
		}
	}

	return args
}
