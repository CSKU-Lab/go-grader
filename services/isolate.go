package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"

	"github.com/CSKU-Lab/go-grader/constants"
	"github.com/CSKU-Lab/go-grader/models"
)

type IsolateService struct {
	ctx    context.Context
	boxIds chan int
}

func NewIsolateService(ctx context.Context) *IsolateService {
	boxIds := make(chan int, constants.MAX_QUEUES)
	for i := range constants.MAX_QUEUES {
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

func (s *IsolateService) NewInstance() *isolateInstance {
	boxID := <-s.boxIds
	metadataPath := fmt.Sprintf("/tmp/box_%d_metadata", boxID)

	instance := isolateInstance{
		ctx:          s.ctx,
		boxID:        boxID,
		boxPath:      fmt.Sprintf(constants.BOX_PATH, boxID),
		metadataPath: metadataPath,
		boxIds:       s.boxIds,
	}
	instance.init()

	return &instance
}

func (i *isolateInstance) log(format string, args ...any) {
	log.Printf("[Instance:%d] :: %s", i.boxID, fmt.Sprintf(format, args...))
}

func (s *isolateInstance) execute(args ...string) (*bytes.Buffer, error) {
	boxID := fmt.Sprintf("--box-id=%d", s.boxID)
	cmd := exec.CommandContext(s.ctx, "isolate", append([]string{boxID}, args...)...)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	cmd.Run()

	return &stdOut, nil
}

func (i *isolateInstance) init() error {
	i.log("Initializing sandbox...")
	_, err := i.execute("--init")
	return err
}

func (i *isolateInstance) ID() int {
	return i.boxID
}

func (i *isolateInstance) Cleanup() error {
	i.log("Cleaning up sandbox...")
	_, err := i.execute("--cleanup")
	if err == nil {
		i.boxIds <- i.boxID
	}
	return err
}

func (i *isolateInstance) CreateFile(name string, content string) error {
	i.log("Creating file %s...", name)
	filePath := fmt.Sprintf("%s/%s", i.boxPath, name)
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (i *isolateInstance) Run(runnerCmd []string, limit *models.Limit, hasInput bool) error {
	i.log("Running program...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--meta=" + i.metadataPath,
		"--processes=100",
		"--stdout=stdout",
		"--stderr=stderr",
		"--run",
		"--",
	}

	if hasInput {
		args = append([]string{"--stdin=input"}, args...)
	}

	args = append(_limits, args...)
	args = append(args, runnerCmd...)

	_, err := i.execute(args...)

	return err
}

func (i *isolateInstance) GetOutput() (string, error) {
	i.log("Getting stdout...")
	return i.catFile("stdout")
}

func (i *isolateInstance) GetError() (string, error) {
	i.log("Getting stderror...")
	return i.catFile("stderr")
}

func (i *isolateInstance) catFile(fileName string) (string, error) {
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.CommandContext(i.ctx, "cat", fmt.Sprintf("%s/%s", i.boxPath, fileName))
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", errors.New(stdErr.String())
	}

	return stdOut.String(), nil
}

func (i *isolateInstance) getMetadata() (string, error) {
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

func (i *isolateInstance) GetMetadata() (*models.Metadata, error) {
	metadata, err := i.getMetadata()
	if err != nil {
		return nil, err
	}

	return models.ParseMetadata(metadata)
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

		switch field.Kind() {
		case reflect.Int:
			if field.Int() == 0 {
				continue
			}
			args = append(args, fmt.Sprintf("%s=%d", arg, field.Int()))
		case reflect.Float32:
			if field.Float() == 0 {
				continue
			}
			args = append(args, fmt.Sprintf("%s=%f", arg, field.Float()))
		case reflect.Bool:
			if !field.Bool() {
				continue
			}
			args = append(args, fmt.Sprintf("%s", arg))
		}
	}

	return args
}
