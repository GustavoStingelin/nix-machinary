package zellij

import (
	"context"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

type prerequisite string

const (
	prerequisiteGit     prerequisite = "git"
	prerequisiteGH      prerequisite = "gh"
	prerequisiteZellij  prerequisite = "zellij"
	prerequisiteSession prerequisite = "session"
	prerequisiteHome    prerequisite = "home"
)

type PreflightFailure struct {
	prerequisite prerequisite
	cause        error
}

func (failure *PreflightFailure) Error() string {
	switch failure.prerequisite {
	case prerequisiteGit, prerequisiteGH, prerequisiteZellij:
		return "required command '" + string(failure.prerequisite) + "' is not available"
	case prerequisiteSession:
		return "must be run inside an existing Zellij session"
	case prerequisiteHome:
		return "HOME is not available"
	default:
		return "preflight requirement is not available"
	}
}

func (failure *PreflightFailure) Unwrap() error {
	return failure.cause
}

func Preflight(ctx context.Context, config Config) error {
	for _, requirement := range []struct {
		command      CommandName
		prerequisite prerequisite
	}{
		{command: CommandGit, prerequisite: prerequisiteGit},
		{command: CommandGH, prerequisite: prerequisiteGH},
		{command: CommandZellij, prerequisite: prerequisiteZellij},
	} {
		if err := config.Runner.Available(ctx, requirement.command); err != nil {
			failure := &PreflightFailure{prerequisite: requirement.prerequisite, cause: err}
			return errs.Wrap(errs.Preflight, failure.Error(), failure)
		}
	}

	if session, present := config.Environment.Lookup(EnvironmentZellij); !present || session == "" {
		failure := &PreflightFailure{prerequisite: prerequisiteSession}
		return errs.Wrap(errs.Preflight, failure.Error(), failure)
	}

	return nil
}
