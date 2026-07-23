package app

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

func (service PullRequestService) Checkout(ctx context.Context, input PullRequestInput) (PullRequestResult, error) {
	pullRequest, err := service.github.ResolvePullRequest(ctx, github.Directory(input.Project.InvocationWorktree), input.Selector)
	if err != nil {
		return PullRequestResult{}, pullRequestError(errs.Usage, "could not resolve pull request '"+string(input.Selector)+"'", err, "")
	}

	display := "pr-" + string(pullRequest.Number) + "-" + string(pullRequest.HeadRefName)
	branch := pullRequestBranch(input.Project, pullRequest)
	managedPath := worktree.ManagedWorktreePath(worktree.Path(input.Project.ManagedRoot), display)
	branchExists, err := service.git.LocalBranchExists(ctx, input.Project.ProjectRoot, branch)
	if err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not inspect local pull request branch", err, "")
	}
	records, err := service.git.ListWorktrees(ctx, input.Project.ProjectRoot)
	if err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not inspect managed worktrees", err, "")
	}
	occupied, err := pullRequestPathOccupied(managedPath)
	if err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not inspect managed worktree path", err, "")
	}
	validation := worktree.ValidateTarget(worktree.TargetInput{
		Branch:       branch,
		ManagedPath:  managedPath,
		Records:      records,
		PathOccupied: occupied,
	})
	if branchExists {
		if validation.Registration == worktree.RegistrationReusable && validation.Branch == worktree.BranchManaged {
			return PullRequestResult{Action: PullRequestReused, Branch: branch, Display: display, Worktree: managedPath}, nil
		}
		return PullRequestResult{}, pullRequestError(errs.Usage, "local branch '"+string(branch)+"' already exists", nil, "")
	}
	if validation.Registration == worktree.RegistrationDetached {
		return PullRequestResult{}, pullRequestError(errs.External, "detached managed pull-request worktree requires recovery", nil, managedPath)
	}
	if _, acceptedError := validation.AcceptedPath(); acceptedError != nil {
		return PullRequestResult{}, pullRequestError(errs.Usage, "pull request '"+string(input.Selector)+"' is not available for a managed worktree", acceptedError, "")
	}

	start, err := service.git.ResolveHead(ctx, input.Project.InvocationWorktree)
	if err != nil {
		return PullRequestResult{}, pullRequestError(errs.Project, "could not resolve source HEAD for pull request '"+string(input.Selector)+"'", err, "")
	}
	if err := os.MkdirAll(string(input.Project.ManagedRoot), 0o700); err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not create managed worktree root", err, "")
	}
	if err := service.git.AddDetached(ctx, DetachedWorktreeRequest{Repository: input.Project.ProjectRoot, Path: managedPath, Start: start}); err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not create detached pull-request worktree", err, "")
	}
	if err := service.github.CheckoutPullRequest(ctx, github.CheckoutRequest{
		Directory: github.Directory(managedPath),
		Selector:  input.Selector,
		Branch:    github.Branch(branch),
	}); err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "could not checkout pull request", err, managedPath)
	}
	if err := service.verifyRegistration(ctx, input.Project.ProjectRoot, branch, managedPath); err != nil {
		return PullRequestResult{}, pullRequestError(errs.External, "GitHub CLI did not register expected pull-request branch", err, managedPath)
	}
	return PullRequestResult{Action: PullRequestCreated, Branch: branch, Display: display, Worktree: managedPath}, nil
}

func (service PullRequestService) verifyRegistration(ctx context.Context, repositoryPath project.Directory, branch worktree.Branch, managedPath worktree.Path) error {
	records, err := service.git.ListWorktrees(ctx, repositoryPath)
	if err != nil {
		return err
	}
	occupied, err := pullRequestPathOccupied(managedPath)
	if err != nil {
		return err
	}
	validation := worktree.ValidateTarget(worktree.TargetInput{
		Branch:       branch,
		ManagedPath:  managedPath,
		Records:      records,
		PathOccupied: occupied,
	})
	if validation.Registration != worktree.RegistrationReusable || validation.Branch != worktree.BranchManaged {
		return &worktree.InvalidTargetError{Validation: validation}
	}
	return nil
}

func pullRequestBranch(projectResolution project.Resolution, pullRequest github.PullRequest) worktree.Branch {
	projectPath := string(projectResolution.ProjectRoot)
	number := string(pullRequest.Number)
	headRefName := string(pullRequest.HeadRefName)
	identity := "project:" + strconv.Itoa(len(projectPath)) + ":" + projectPath + "\n" +
		"number:" + strconv.Itoa(len(number)) + ":" + number + "\n" +
		"head:" + strconv.Itoa(len(headRefName)) + ":" + headRefName + "\n"
	return worktree.Branch("zwm/pr-" + number + "-" + worktree.IdentityHash(identity)[:8])
}

func pullRequestError(class errs.Class, message string, cause error, detachedPath worktree.Path) *PullRequestError {
	var recovery *PullRequestRecovery
	if detachedPath != "" {
		recovery = &PullRequestRecovery{DetachedWorktree: detachedPath}
	}
	return &PullRequestError{Class: class, Message: message, Cause: cause, Recovery: recovery}
}

func pullRequestPathOccupied(path worktree.Path) (bool, error) {
	_, err := os.Lstat(string(path))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
