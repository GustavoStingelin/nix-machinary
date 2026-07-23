package app

import (
	"context"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

type BranchGit interface {
	ValidateBranch(context.Context, git.Directory, git.Branch) error
	ResolveCommit(context.Context, git.Directory, git.Commitish) (git.Commit, error)
	LocalBranchExists(context.Context, git.Directory, git.Branch) (bool, error)
	ListWorktrees(context.Context, git.Directory) ([]byte, error)
	AddExistingWorktree(context.Context, git.Directory, git.WorktreePath, git.Branch) error
	AddNewWorktree(context.Context, git.Directory, git.Branch, git.WorktreePath, git.Commit) error
}

type TabLauncher interface {
	Launch(context.Context, zellij.Input) (zellij.Result, error)
}

type TabLauncherFunc func(context.Context, zellij.Input) (zellij.Result, error)

func (launch TabLauncherFunc) Launch(ctx context.Context, input zellij.Input) (zellij.Result, error) {
	return launch(ctx, input)
}

type BranchService struct {
	git  BranchGit
	tabs TabLauncher
}

func NewBranchService(branchGit BranchGit, tabs TabLauncher) BranchService {
	return BranchService{git: branchGit, tabs: tabs}
}

type CheckoutExistingInput struct {
	Project project.Resolution
	Branch  git.Branch
}

type CheckoutNewInput struct {
	Project    project.Resolution
	Branch     git.Branch
	StartPoint git.Commitish
}

type CheckoutResult struct {
	Worktree        worktree.Path
	DisplayIdentity string
	TabAction       zellij.Action
	TabTitle        zellij.TabTitle
	TabWorktree     zellij.WorktreePath
}
