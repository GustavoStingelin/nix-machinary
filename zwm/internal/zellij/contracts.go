package zellij

import (
	"context"
	"fmt"
)

type CommandName string

const (
	CommandGit    CommandName = "git"
	CommandGH     CommandName = "gh"
	CommandZellij CommandName = "zellij"
)

type EnvironmentVariable string

const (
	EnvironmentHome   EnvironmentVariable = "HOME"
	EnvironmentZellij EnvironmentVariable = "ZELLIJ"
)

type Config struct {
	Runner      Runner
	Environment Environment
}

type Runner interface {
	Available(context.Context, CommandName) error
	Run(context.Context, Command) (Output, error)
}

type Environment interface {
	Lookup(EnvironmentVariable) (string, bool)
}

type Command struct {
	Name CommandName
	Args []string
	Dir  string
}

type Output struct {
	Stdout string
	Stderr string
}

type TabTitle string

type Directory string

type Input struct {
	Title TabTitle
	Cwd   Directory
}

type Action string

const (
	Created Action = "created"
	Focused Action = "focused"
)

type Result struct {
	Action Action
	Title  TabTitle
	Cwd    Directory
	Output Output
}

type CommandFailure struct {
	Command Command
	Output  Output
	cause   error
}

func (failure *CommandFailure) Error() string {
	return fmt.Sprintf("%s %v: %v", failure.Command.Name, failure.Command.Args, failure.cause)
}

func (failure *CommandFailure) Unwrap() error {
	return failure.cause
}
