package services

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os/exec"
)

type CompileService struct {
	ctx context.Context
}

func NewCompileService(ctx context.Context) *CompileService {
	return &CompileService{ctx: ctx}
}

func (s *CompileService) Compile(script []string) error {
	log.Println("Compiling code...")
	cmd := exec.CommandContext(s.ctx, script[0], script[1:]...)
	var stdErr bytes.Buffer
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return errors.New(err.Error() + " : " + stdErr.String())
	}

	return nil
}
