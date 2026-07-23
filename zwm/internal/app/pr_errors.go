package app

import (
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

type PullRequestRecovery struct {
	DetachedWorktree worktree.Path
}

type PullRequestError struct {
	Class    errs.Class
	Message  string
	Cause    error
	Recovery *PullRequestRecovery
}

func (errorValue *PullRequestError) Error() string {
	return errorValue.Message
}

func (errorValue *PullRequestError) Unwrap() []error {
	classification := errs.New(errorValue.Class, "")
	if errorValue.Cause == nil {
		return []error{classification}
	}
	return []error{errorValue.Cause, classification}
}
