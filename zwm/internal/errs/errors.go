// Package errs defines the command's typed failure contract.
package errs

import "errors"

// Class identifies a stable CLI failure class.
type Class string

const (
	// Usage identifies invalid command input.
	Usage Class = "usage"
	// Project identifies project-resolution failures.
	Project Class = "project"
	// Preflight identifies missing command or session requirements.
	Preflight Class = "preflight"
	// External identifies external command failures.
	External Class = "external"
)

var (
	// ErrUsage matches failures classified as Usage.
	ErrUsage error = classError{class: Usage}
	// ErrProject matches failures classified as Project.
	ErrProject error = classError{class: Project}
	// ErrPreflight matches failures classified as Preflight.
	ErrPreflight error = classError{class: Preflight}
	// ErrExternal matches failures classified as External.
	ErrExternal error = classError{class: External}
)

// Error carries a stable failure class while retaining an optional cause.
type Error struct {
	Class   Class
	Message string
	cause   error
}

// New creates a classified error without a cause.
func New(class Class, message string) *Error {
	return &Error{Class: class, Message: message}
}

// Wrap creates a classified error that retains its cause for errors.Is and errors.As.
func Wrap(class Class, message string, cause error) *Error {
	return &Error{Class: class, Message: message, cause: cause}
}

func (errorValue *Error) Error() string {
	return errorValue.Message
}

func (errorValue *Error) Unwrap() []error {
	classCause := classError{class: errorValue.Class}
	if errorValue.cause == nil {
		return []error{classCause}
	}
	return []error{errorValue.cause, classCause}
}

// ClassOf returns the stable class for an error, defaulting unknown failures to External.
func ClassOf(err error) Class {
	var classified *Error
	if errors.As(err, &classified) {
		return classified.Class
	}

	switch {
	case errors.Is(err, ErrUsage):
		return Usage
	case errors.Is(err, ErrProject):
		return Project
	case errors.Is(err, ErrPreflight):
		return Preflight
	default:
		return External
	}
}

// ExitCode maps a classified failure to the documented process exit status.
func ExitCode(err error) int {
	switch ClassOf(err) {
	case Usage:
		return 64
	case Project:
		return 65
	case Preflight:
		return 69
	case External:
		return 1
	default:
		return 1
	}
}

type classError struct {
	class Class
}

func (errorValue classError) Error() string {
	return string(errorValue.class)
}
