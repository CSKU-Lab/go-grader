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

type IsolateService struct {
	ctx    context.Context
	boxIds chan int
}

func NewIsolateService(ctx context.Context, maxInstance int) *IsolateService {
	boxIds := make(chan int, maxInstance)
	for i := range maxInstance {
		boxIds <- i
	}

	return &IsolateService{ctx: ctx, boxIds: boxIds}
}

type isolateInstance struct {
	ctx          context.Context
	boxID        int
	boxPath      string
	metadataPath string
	boxIds       chan int
}

func (s *IsolateService) New() *isolateInstance {
	boxID := <-s.boxIds
	boxPath := fmt.Sprintf("/var/local/lib/isolate/%d/box", boxID)
	metadataPath := fmt.Sprintf("/tmp/box_%d_metadata", boxID)

	instance := isolateInstance{
		ctx:          s.ctx,
		boxID:        boxID,
		boxPath:      boxPath,
		metadataPath: metadataPath,
		boxIds:       s.boxIds,
	}
	instance.init()

	return &instance
}

func (i *isolateInstance) log(format string, args ...any) {
	log.Printf("[Instance:%d] :: %s", i.boxID, fmt.Sprintf(format, args...))
}

func (s *isolateInstance) execute(args ...string) error {
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

func (i *isolateInstance) init() error {
	i.log("Initializing sandbox...")
	err := i.execute("--init")
	return err
}

func (i *isolateInstance) ID() int {
	return i.boxID
}

func (i *isolateInstance) Cleanup() error {
	i.log("Cleaning up sandbox...")
	err := i.execute("--cleanup")
	if err == nil {
		i.boxIds <- i.boxID
	}
	return err
}

func (i *isolateInstance) CreateFile(name string, content string) error {
	i.log("Creating file %s...", name)
	filePath := fmt.Sprintf("%s/%s", i.boxPath, name)
	cmd := exec.CommandContext(i.ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", content, filePath))
	err := cmd.Run()
	return err
}

func (i *isolateInstance) Run(programPath string, runnerCmd []string, limit *models.Limit) error {
	i.log("Running program...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--stdin=input",
		"--meta=" + i.metadataPath,
		"--stdout=output.txt",
		"--run",
		"--",
		programPath,
	}

	args = append(args[:4], append(runnerCmd, args[4:]...)...)
	args = append(_limits, args...)

	err := i.execute(args...)

	return err
}

func (i *isolateInstance) GetOutput() (string, error) {
	i.log("Getting output...")
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.CommandContext(i.ctx, "cat", fmt.Sprintf("%s/output.txt", i.boxPath))
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stdErr.String())
	}

	return stdOut.String(), nil
}

func (i *isolateInstance) GetMetadata() (string, error) {
	i.log("Getting Metadata...")
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.CommandContext(i.ctx, "cat", i.metadataPath)
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
