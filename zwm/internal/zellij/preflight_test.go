package zellij

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

func TestPreflight_confirms_commands_then_session_when_requirements_are_available(t *testing.T) {
	// Given
	log := &callLog{}
	runner := &fakeRunner{log: log}
	environment := fakeEnvironment{
		log:    log,
		values: map[EnvironmentVariable]string{EnvironmentZellij: "session-1"},
	}

	// When
	err := Preflight(context.Background(), Config{Runner: runner, Environment: environment})

	// Then
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	assertEqual(t, []string{
		"available:git",
		"available:gh",
		"available:zellij",
		"env:ZELLIJ",
	}, log.calls)
	assertEqual(t, []Command(nil), runner.commands)
}

func TestPreflight_returns_typed_preflight_error_before_session_or_command_execution_when_command_is_unavailable(t *testing.T) {
	tests := []struct {
		name      string
		command   CommandName
		wantCalls []string
	}{
		{name: "git", command: CommandGit, wantCalls: []string{"available:git"}},
		{name: "gh", command: CommandGH, wantCalls: []string{"available:git", "available:gh"}},
		{name: "zellij", command: CommandZellij, wantCalls: []string{"available:git", "available:gh", "available:zellij"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			log := &callLog{}
			cause := errors.New(string(test.command) + " unavailable")
			runner := &fakeRunner{
				availability: map[CommandName]error{test.command: cause},
				log:          log,
			}
			environment := fakeEnvironment{
				log:    log,
				values: map[EnvironmentVariable]string{EnvironmentZellij: "session-1"},
			}

			// When
			err := Preflight(context.Background(), Config{Runner: runner, Environment: environment})

			// Then
			if !errors.Is(err, errs.ErrPreflight) {
				t.Fatalf("expected preflight class, got %v", err)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("expected cause %v, got %v", cause, err)
			}
			var failure *PreflightFailure
			if !errors.As(err, &failure) {
				t.Fatalf("expected PreflightFailure, got %T", err)
			}
			assertEqual(t, "required command '"+string(test.command)+"' is not available", failure.Error())
			assertEqual(t, test.wantCalls, log.calls)
			assertEqual(t, []Command(nil), runner.commands)
		})
	}
}

func TestPreflight_returns_typed_preflight_error_without_query_when_zellij_session_is_empty(t *testing.T) {
	// Given
	log := &callLog{}
	runner := &fakeRunner{log: log}
	environment := fakeEnvironment{
		log:    log,
		values: map[EnvironmentVariable]string{EnvironmentZellij: ""},
	}

	// When
	err := Preflight(context.Background(), Config{Runner: runner, Environment: environment})

	// Then
	if !errors.Is(err, errs.ErrPreflight) {
		t.Fatalf("expected preflight class, got %v", err)
	}
	var failure *PreflightFailure
	if !errors.As(err, &failure) {
		t.Fatalf("expected PreflightFailure, got %T", err)
	}
	assertEqual(t, "must be run inside an existing Zellij session", failure.Error())
	assertEqual(t, []string{
		"available:git",
		"available:gh",
		"available:zellij",
		"env:ZELLIJ",
	}, log.calls)
	assertEqual(t, []Command(nil), runner.commands)
}
