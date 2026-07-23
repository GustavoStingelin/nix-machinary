package github

import "errors"

var (
	ErrCommand           = errors.New("github command failed")
	ErrMalformedMetadata = errors.New("malformed pull request metadata")
)

type CommandError struct {
	Arguments []string
	Directory string
	Stdout    []byte
	Stderr    []byte
	Cause     error
}

func (errorValue *CommandError) Error() string {
	return "github command failed"
}

func (errorValue *CommandError) Unwrap() error {
	return errorValue.Cause
}

func (errorValue *CommandError) Is(target error) bool {
	return target == ErrCommand
}

type MetadataError struct {
	Output []byte
}

func (errorValue *MetadataError) Error() string {
	return "malformed pull request metadata"
}

func (errorValue *MetadataError) Is(target error) bool {
	return target == ErrMalformedMetadata
}
