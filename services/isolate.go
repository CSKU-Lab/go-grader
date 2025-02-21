package services

import (
	"context"
	"os/exec"
)

type isolateService struct {
	ctx context.Context
}

func NewIsolateService(ctx context.Context) *isolateService {
	return &isolateService{ctx: ctx}
}

func (s *isolateService) Init() error {
	cmd := exec.CommandContext(s.ctx, "isolate", "--init")
	_, err := cmd.Output()
	return err
}

func (s *isolateService) Cleanup() error {
	cmd := exec.CommandContext(s.ctx, "isolate", "--cleanup")
	_, err := cmd.Output()
	return err
}

func (s *isolateService) Run() (string, error) {
	args := []string{
		"--run",
		"--",
		"/usr/bin/python3",
		"-c",
		"print('Hello, World!')",
	}

	cmd := exec.CommandContext(s.ctx, "isolate", args...)

	stdout, err := cmd.Output()

	return string(stdout), err
}
