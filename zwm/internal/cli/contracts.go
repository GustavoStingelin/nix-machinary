// Package cli owns the raw command-line boundary and application service port.
package cli

import "context"

// ProjectNameOrPath identifies the optional project selected at the CLI boundary.
type ProjectNameOrPath string

// BranchName identifies a branch argument before Git validation.
type BranchName string

// StartPoint identifies an optional new-branch start point before Git validation.
type StartPoint string

// PullRequestSelector identifies a pull request selector before GitHub validation.
type PullRequestSelector string

// Invocation is a parsed, syntactically valid CLI request.
type Invocation struct {
	Project ProjectNameOrPath
	Action  Action
}

type Result struct {
	Worktree        string
	DisplayIdentity string
	TabAction       string
	TabTitle        string
	TabWorktree     string
}

// Action is one of the requests accepted by the zwm CLI.
type Action interface {
	action()
}

// CheckoutExisting requests checkout of an existing local branch.
type CheckoutExisting struct {
	Branch BranchName
}

func (CheckoutExisting) action() {}

// CheckoutNew requests creation of a new branch, optionally from StartPoint.
type CheckoutNew struct {
	Branch     BranchName
	StartPoint StartPoint
}

func (CheckoutNew) action() {}

// PullRequest requests checkout of a pull request selector.
type PullRequest struct {
	Selector PullRequestSelector
}

func (PullRequest) action() {}

// Service is the CLI-owned application seam for a parsed request.
type Service interface {
	Execute(context.Context, Invocation) (Result, error)
}
