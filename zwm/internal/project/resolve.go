package project

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

// Resolve canonicalizes a project selection and derives its managed worktree root.
func (resolver Resolver) Resolve(ctx context.Context, request Request) (Resolution, error) {
	candidate, err := canonicalExistingDirectory(projectPath(request))
	if err != nil {
		return Resolution{}, notWorktreeError(request.Project, err)
	}

	invocationRoot, err := resolver.repository.WorktreeRoot(ctx, candidate)
	if err != nil {
		return Resolution{}, notWorktreeError(request.Project, err)
	}
	invocationWorktree, err := canonicalExistingDirectory(string(invocationRoot))
	if err != nil {
		return Resolution{}, notWorktreeError(request.Project, err)
	}

	primaryRoot, err := resolver.repository.PrimaryWorktreeRoot(ctx, invocationWorktree)
	if err != nil {
		return Resolution{}, primaryWorktreeError(err)
	}
	projectRoot, err := canonicalExistingDirectory(string(primaryRoot))
	if err != nil {
		return Resolution{}, primaryWorktreeError(err)
	}

	home, err := canonicalExistingDirectory(string(request.Home))
	if err != nil {
		return Resolution{}, errs.Wrap(errs.Project, fmt.Sprintf("home directory '%s' is not available", request.Home), err)
	}
	codeRoot := canonicalCodeRoot(home)
	managedRootsRoot := filepath.Join(codeRoot, ".wt")
	if pathsOverlap(string(projectRoot), managedRootsRoot) {
		return Resolution{}, errs.New(errs.Project, fmt.Sprintf("project '%s' conflicts with managed worktree root '%s'", projectRoot, managedRootsRoot))
	}

	key, err := deriveKey(projectRoot, codeRoot)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{
		InvocationWorktree: invocationWorktree,
		Key:                key,
		ManagedRoot:        Directory(filepath.Join(managedRootsRoot, string(key))),
		ProjectRoot:        projectRoot,
	}, nil
}

func notWorktreeError(value Value, cause error) error {
	if value == "" {
		return errs.Wrap(errs.Project, "current directory is not a Git worktree", cause)
	}
	return errs.Wrap(errs.Project, fmt.Sprintf("project '%s' is not a Git worktree", value), cause)
}

func primaryWorktreeError(cause error) error {
	return errs.Wrap(errs.Project, "could not determine the primary Git worktree", cause)
}

func projectPath(request Request) string {
	value := string(request.Project)
	if value == "" {
		return string(request.WorkingDirectory)
	}
	if value == "~" {
		return string(request.Home)
	}
	if len(value) >= 2 && value[0] == '~' && value[1] == '/' {
		return filepath.Join(string(request.Home), value[2:])
	}
	if isPathLike(value) {
		if filepath.IsAbs(value) {
			return value
		}
		return filepath.Join(string(request.WorkingDirectory), value)
	}
	return filepath.Join(string(request.Home), "code", value)
}

func isPathLike(value string) bool {
	return value == "." || value == ".." || value == "~" || strings.HasPrefix(value, "~/") || strings.Contains(value, "/")
}
