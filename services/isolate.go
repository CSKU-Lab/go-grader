package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"

	"github.com/SornchaiTheDev/go-grader/models"
)

type isolateService struct {
	ctx   context.Context
	boxID string
}

func NewIsolateService(ctx context.Context, boxID int) *isolateService {
	_boxID := fmt.Sprintf("--box-id=%d", boxID)

	return &isolateService{ctx: ctx, boxID: _boxID}
}

func (s *isolateService) execute(args ...string) error {
	cmd := exec.CommandContext(s.ctx, "isolate", append([]string{s.boxID}, args...)...)
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return errors.New(stdErr.String())
	}
	return nil
}

func (s *isolateService) Init() error {
	err := s.execute("--init")
	return err
}

func (s *isolateService) Cleanup() error {
	err := s.execute("--cleanup")
	return err
}

func (s *isolateService) Copy(path string) error {
	cmd := exec.CommandContext(s.ctx, "cp", "-r", path, "/tmp/isolate/0/box/")
	err := cmd.Run()
	return err
}

func (s *isolateService) Run(limit *models.Limit) error {
	_limits := getLimitArgs(limit)

	args := []string{
		"--run",
		"--",
		"/usr/bin/python3",
		"-c",
		"import time; time.sleep(1)",
	}

	args = append(_limits, args...)

	err := s.execute(args...)

	return err
}

func getLimitArgs(limit *models.Limit) []string {
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
