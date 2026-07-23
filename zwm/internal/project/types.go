// Package project resolves canonical repository identity without mutating Git state.
package project

import "context"

// Directory identifies a filesystem directory.
type Directory string

// Key identifies a collision-safe project namespace.
type Key string

// Value is the raw project name-or-path supplied by the caller.
type Value string

// Request supplies the explicit environment required to resolve a project.
type Request struct {
	Home             Directory
	Project          Value
	WorkingDirectory Directory
}

// Resolution is the canonical project identity used by later worktree operations.
type Resolution struct {
	InvocationWorktree Directory
	Key                Key
	ManagedRoot        Directory
	ProjectRoot        Directory
}

// Repository provides the read-only Git facts required for project normalization.
type Repository interface {
	WorktreeRoot(context.Context, Directory) (Directory, error)
	PrimaryWorktreeRoot(context.Context, Directory) (Directory, error)
}

// Resolver resolves project requests through its repository seam.
type Resolver struct {
	repository Repository
}

// NewResolver creates a resolver backed by the supplied read-only repository seam.
func NewResolver(repository Repository) Resolver {
	return Resolver{repository: repository}
}
