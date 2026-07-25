package app

import (
	"context"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

type PullRequestGit interface {
	LocalBranchExists(context.Context, project.Directory, worktree.Branch) (bool, error)
	ListWorktrees(context.Context, project.Directory) ([]worktree.Record, error)
	ResolveHead(context.Context, project.Directory) (worktree.OID, error)
	AddDetached(context.Context, DetachedWorktreeRequest) error
}

type PullRequestGateway interface {
	ResolvePullRequest(context.Context, github.Directory, github.PullRequestSelector) (github.PullRequest, error)
	CheckoutPullRequest(context.Context, github.CheckoutRequest) error
}

type DetachedWorktreeRequest struct {
	Repository project.Directory
	Path       worktree.Path
	Start      worktree.OID
}

type PullRequestService struct {
	git    PullRequestGit
	github PullRequestGateway
}

func NewPullRequestService(pullRequestGit PullRequestGit, pullRequestGateway PullRequestGateway) PullRequestService {
	return PullRequestService{git: pullRequestGit, github: pullRequestGateway}
}

type PullRequestInput struct {
	Project  project.Resolution
	Selector github.PullRequestSelector
	// Force resets an already-checked-out managed pull-request branch to the
	// latest remote state instead of reusing the local version as-is.
	Force bool
}

type PullRequestAction string

const (
	PullRequestCreated PullRequestAction = "created"
	PullRequestReused  PullRequestAction = "reused"
)

type PullRequestResult struct {
	Action   PullRequestAction
	Branch   worktree.Branch
	Display  string
	Number   github.PullRequestNumber
	Worktree worktree.Path
}
