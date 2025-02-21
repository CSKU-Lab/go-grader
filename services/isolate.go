package services

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"

	"github.com/SornchaiTheDev/go-grader/models"
)

type isolateService struct {
	ctx context.Context
}

func NewIsolateService(ctx context.Context) *isolateService {
	return &isolateService{ctx: ctx}
}

func (s *isolateService) execute(args ...string) ([]byte, error) {
	cmd := exec.CommandContext(s.ctx, "isolate", args...)
	return cmd.Output()
}

func (s *isolateService) Init() error {
	_, err := s.execute("--init")
	return err
}

func (s *isolateService) Cleanup() error {
	_, err := s.execute("--cleanup")
	return err
}

func (s *isolateService) Run(limit *models.Limit) (string, error) {
	_limits := getLimitArgs(limit)

	args := []string{
		"--run",
		"--",
		"/usr/bin/python3",
		"-c",
		"import time; time.sleep(0.5)",
	}

	args = append(_limits, args...)

	stdout, err := s.execute(args...)

	return string(stdout), err
}

func getLimitArgs(limit *models.Limit) []string {
	v := reflect.ValueOf(limit).Elem()
	t := v.Type()

	args := make([]string, 0)

	for i := range v.NumField() {
		field := v.Field(i)
		arg := t.Field(i).Tag.Get("arg")

		if field.Kind() == reflect.Int {
			if field.Int() == 0 {
				continue
			}
			args = append(args, fmt.Sprintf("%s=%d", arg, field.Int()))
		} else if field.Kind() == reflect.Bool {
			if !field.Bool() {
				continue
			}
			args = append(args, fmt.Sprintf("%s", arg))
		}
	}

	return args
}
