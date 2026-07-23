package git

import "errors"

var (
	ErrCommand          = errors.New("git command failed")
	ErrInvalidBranch    = errors.New("invalid git branch")
	ErrInvalidCommitish = errors.New("invalid git commit-ish")
)

type CommandError struct {
	Arguments []string
	Directory string
	Stdout    []byte
	Stderr    []byte
	Cause     error
}

func (errorValue *CommandError) Error() string {
	return "git command failed"
}

func (errorValue *CommandError) Unwrap() error {
	return errorValue.Cause
}

func (errorValue *CommandError) Is(target error) bool {
	return target == ErrCommand
}

type InvalidBranchError struct {
	Branch Branch
	Cause  *CommandError
}

func (errorValue *InvalidBranchError) Error() string {
	return "invalid git branch"
}

func (errorValue *InvalidBranchError) Unwrap() []error {
	return []error{ErrInvalidBranch, errorValue.Cause}
}

type InvalidCommitishError struct {
	Commitish Commitish
	Cause     *CommandError
}

func (errorValue *InvalidCommitishError) Error() string {
	return "invalid git commit-ish"
}

func (errorValue *InvalidCommitishError) Unwrap() []error {
	return []error{ErrInvalidCommitish, errorValue.Cause}
}
