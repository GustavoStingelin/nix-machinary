package zellij

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type SystemRunner struct{}

func (SystemRunner) Available(ctx context.Context, command CommandName) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := exec.LookPath(string(command))
	return err
}

func (SystemRunner) Run(ctx context.Context, command Command) (Output, error) {
	process := exec.CommandContext(ctx, string(command.Name), command.Args...)
	process.Dir = command.Dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	err := process.Run()
	return Output{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

type SystemEnvironment struct{}

func (SystemEnvironment) Lookup(variable EnvironmentVariable) (string, bool) {
	return os.LookupEnv(string(variable))
}
