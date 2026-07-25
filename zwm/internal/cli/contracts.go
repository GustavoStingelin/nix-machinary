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

type Result interface {
	result()
}

type WorktreeResult struct {
	Worktree        string
	DisplayIdentity string
	TabAction       string
	TabTitle        string
	TabWorktree     string
}

func (WorktreeResult) result() {}

type OpenProjectResult struct {
	ProjectRoot string
	TabAction   string
	TabTitle    string
	TabCwd      string
}

func (OpenProjectResult) result() {}

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

type OpenProject struct{}

func (OpenProject) action() {}

// Service is the CLI-owned application seam for a parsed request.
type Service interface {
	Execute(context.Context, Invocation) (Result, error)
}

// Completer supplies shell-completion candidates for the interactive commands.
// Every method is best-effort: implementations return an empty slice rather than
// an error so a failed lookup never disrupts the shell.
type Completer interface {
	// Branches returns local branch names for the selected project.
	Branches(ctx context.Context, project ProjectNameOrPath) []string
	// Projects returns selectable project names.
	Projects(ctx context.Context) []string
	// PullRequests returns open pull request candidates for the selected
	// project, each formatted as a "<selector>:<description>" completion entry.
	PullRequests(ctx context.Context, project ProjectNameOrPath) []string
}
