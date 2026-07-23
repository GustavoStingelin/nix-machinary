package zellij

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type callLog struct {
	calls []string
}

type runResponse struct {
	output Output
	err    error
}

type fakeRunner struct {
	availability map[CommandName]error
	commands     []Command
	log          *callLog
	responses    []runResponse
}

func (runner *fakeRunner) Available(_ context.Context, command CommandName) error {
	runner.log.calls = append(runner.log.calls, "available:"+string(command))
	return runner.availability[command]
}

func (runner *fakeRunner) Run(_ context.Context, command Command) (Output, error) {
	runner.log.calls = append(runner.log.calls, "run:"+string(command.Name))
	runner.commands = append(runner.commands, command)
	if len(runner.responses) == 0 {
		return Output{}, errors.New("unexpected command")
	}

	response := runner.responses[0]
	runner.responses = runner.responses[1:]
	return response.output, response.err
}

type fakeEnvironment struct {
	log    *callLog
	values map[EnvironmentVariable]string
}

func (environment fakeEnvironment) Lookup(variable EnvironmentVariable) (string, bool) {
	environment.log.calls = append(environment.log.calls, "env:"+string(variable))
	value, ok := environment.values[variable]
	return value, ok
}

func assertEqual[T any](testingHandle *testing.T, want T, got T) {
	testingHandle.Helper()
	if !reflect.DeepEqual(want, got) {
		testingHandle.Fatalf("want %#v, got %#v", want, got)
	}
}
