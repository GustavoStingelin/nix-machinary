package app

import (
	"bytes"
	"context"
	"errors"
	"os/exec"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

type SystemPullRequestGit struct {
	client     git.Client
	executable string
}

func NewSystemPullRequestGit(executable string) SystemPullRequestGit {
	if executable == "" {
		executable = "git"
	}
	return SystemPullRequestGit{client: git.NewClient(git.Config{Executable: executable}), executable: executable}
}

func (systemGit SystemPullRequestGit) LocalBranchExists(ctx context.Context, directory project.Directory, branch worktree.Branch) (bool, error) {
	return systemGit.client.LocalBranchExists(ctx, git.Directory(directory), git.Branch(branch))
}

func (systemGit SystemPullRequestGit) ListWorktrees(ctx context.Context, directory project.Directory) ([]worktree.Record, error) {
	raw, err := systemGit.client.ListWorktrees(ctx, git.Directory(directory))
	if err != nil {
		return nil, err
	}
	return worktree.ParsePorcelainZ(raw)
}

func (systemGit SystemPullRequestGit) ResolveHead(ctx context.Context, directory project.Directory) (worktree.OID, error) {
	commit, err := systemGit.client.ResolveCommit(ctx, git.Directory(directory), git.Commitish("HEAD"))
	if err != nil {
		return "", err
	}
	return worktree.OID(commit), nil
}

func (systemGit SystemPullRequestGit) AddDetached(ctx context.Context, request DetachedWorktreeRequest) error {
	arguments := []string{"-C", string(request.Repository), "worktree", "add", "--detach", string(request.Path), string(request.Start)}
	command := exec.CommandContext(ctx, systemGit.executable, arguments...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return nil
	}
	cause := err
	if contextError := ctx.Err(); contextError != nil {
		cause = errors.Join(contextError, err)
	}
	return &git.CommandError{
		Arguments: arguments,
		Directory: string(request.Repository),
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		Cause:     cause,
	}
}
