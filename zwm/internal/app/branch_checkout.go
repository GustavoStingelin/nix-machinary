package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

func (service BranchService) CheckoutExisting(ctx context.Context, input CheckoutExistingInput) (CheckoutResult, error) {
	if err := service.validateBranch(ctx, input.Project, input.Branch); err != nil {
		return CheckoutResult{}, err
	}
	exists, err := service.git.LocalBranchExists(ctx, git.Directory(input.Project.ProjectRoot), input.Branch)
	if err != nil {
		return CheckoutResult{}, errs.Wrap(errs.External, "check local branch", err)
	}
	if !exists {
		return CheckoutResult{}, errs.New(errs.Usage, fmt.Sprintf("local branch '%s' does not exist", input.Branch))
	}

	path, validation, err := service.target(ctx, input.Project, input.Branch)
	if err != nil {
		return CheckoutResult{}, err
	}
	if _, err := validation.AcceptedPath(); err != nil {
		return CheckoutResult{}, targetUnavailable(input.Branch, err)
	}
	if validation.Registration == worktree.RegistrationAvailable {
		if err := os.MkdirAll(string(input.Project.ManagedRoot), 0o755); err != nil {
			return CheckoutResult{}, errs.Wrap(errs.External, "create managed worktree root", err)
		}
		if err := service.git.AddExistingWorktree(ctx, git.Directory(input.Project.ProjectRoot), git.WorktreePath(path), input.Branch); err != nil {
			return CheckoutResult{}, errs.Wrap(errs.External, "add existing branch worktree", err)
		}
	}
	return service.launch(ctx, input.Project, input.Branch, path)
}

func (service BranchService) CheckoutNew(ctx context.Context, input CheckoutNewInput) (CheckoutResult, error) {
	if err := service.validateBranch(ctx, input.Project, input.Branch); err != nil {
		return CheckoutResult{}, err
	}
	startPoint := input.StartPoint
	if startPoint == "" {
		startPoint = "HEAD"
	}
	commit, err := service.git.ResolveCommit(ctx, git.Directory(input.Project.InvocationWorktree), startPoint)
	if err != nil {
		if errors.Is(err, git.ErrInvalidCommitish) {
			return CheckoutResult{}, errs.Wrap(errs.Usage, fmt.Sprintf("invalid start-point '%s'", startPoint), err)
		}
		return CheckoutResult{}, errs.Wrap(errs.External, "resolve start-point", err)
	}

	exists, err := service.git.LocalBranchExists(ctx, git.Directory(input.Project.ProjectRoot), input.Branch)
	if err != nil {
		return CheckoutResult{}, errs.Wrap(errs.External, "check local branch", err)
	}
	path, validation, err := service.target(ctx, input.Project, input.Branch)
	if err != nil {
		return CheckoutResult{}, err
	}
	if exists {
		if validation.Registration == worktree.RegistrationReusable && validation.Branch == worktree.BranchManaged {
			return service.launch(ctx, input.Project, input.Branch, path)
		}
		return CheckoutResult{}, errs.New(errs.Usage, fmt.Sprintf("local branch '%s' already exists", input.Branch))
	}
	if validation.Registration != worktree.RegistrationAvailable || validation.Branch != worktree.BranchUnregistered {
		return CheckoutResult{}, targetUnavailable(input.Branch, &worktree.InvalidTargetError{Validation: validation})
	}
	if err := os.MkdirAll(string(input.Project.ManagedRoot), 0o755); err != nil {
		return CheckoutResult{}, errs.Wrap(errs.External, "create managed worktree root", err)
	}
	if err := service.git.AddNewWorktree(ctx, git.Directory(input.Project.ProjectRoot), input.Branch, git.WorktreePath(path), commit); err != nil {
		return CheckoutResult{}, errs.Wrap(errs.External, "add new branch worktree", err)
	}
	return service.launch(ctx, input.Project, input.Branch, path)
}

func (service BranchService) validateBranch(ctx context.Context, resolution project.Resolution, branch git.Branch) error {
	err := service.git.ValidateBranch(ctx, git.Directory(resolution.ProjectRoot), branch)
	if err == nil {
		return nil
	}
	if errors.Is(err, git.ErrInvalidBranch) {
		return errs.Wrap(errs.Usage, fmt.Sprintf("invalid branch '%s'", branch), err)
	}
	return errs.Wrap(errs.External, "validate branch", err)
}

func (service BranchService) target(ctx context.Context, resolution project.Resolution, branch git.Branch) (worktree.Path, worktree.Validation, error) {
	path := worktree.ManagedWorktreePath(worktree.Path(resolution.ManagedRoot), string(branch))
	raw, err := service.git.ListWorktrees(ctx, git.Directory(resolution.ProjectRoot))
	if err != nil {
		return "", worktree.Validation{}, errs.Wrap(errs.External, "list Git worktrees", err)
	}
	records, err := worktree.ParsePorcelainZ(raw)
	if err != nil {
		return "", worktree.Validation{}, errs.Wrap(errs.External, "parse Git worktrees", err)
	}
	occupied, err := branchPathOccupied(path)
	if err != nil {
		return "", worktree.Validation{}, errs.Wrap(errs.External, "inspect managed worktree path", err)
	}
	return path, worktree.ValidateTarget(worktree.TargetInput{
		Branch:       worktree.Branch(branch),
		ManagedPath:  path,
		Records:      records,
		PathOccupied: occupied,
	}), nil
}

func (service BranchService) launch(ctx context.Context, resolution project.Resolution, branch git.Branch, path worktree.Path) (CheckoutResult, error) {
	title := zellij.TabTitle(string(resolution.Key) + ":" + string(branch))
	result, err := service.tabs.Launch(ctx, zellij.Input{Title: title, Worktree: zellij.WorktreePath(path)})
	if err != nil {
		return CheckoutResult{}, err
	}
	return CheckoutResult{
		Worktree:        path,
		DisplayIdentity: string(branch),
		TabAction:       result.Action,
		TabTitle:        title,
		TabWorktree:     zellij.WorktreePath(path),
	}, nil
}

func branchPathOccupied(path worktree.Path) (bool, error) {
	_, err := os.Lstat(string(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func targetUnavailable(branch git.Branch, cause error) error {
	return errs.Wrap(errs.Usage, fmt.Sprintf("branch '%s' is not available for a managed worktree", branch), cause)
}
