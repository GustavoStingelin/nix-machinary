package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

func TestResolveProject_rejectsNonWorktreesAndInvalidSymlinks(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string) string
	}{
		{
			name: "non repository directory",
			setup: func(t *testing.T, root string) string {
				path := filepath.Join(root, "not-a-repository")
				makeDirectory(t, path)
				return path
			},
		},
		{
			name: "missing directory",
			setup: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing")
			},
		},
		{
			name: "symlink loop",
			setup: func(t *testing.T, root string) string {
				path := filepath.Join(root, "loop")
				if err := os.Symlink(path, path); err != nil {
					t.Fatalf("make symlink loop: %v", err)
				}
				return path
			},
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T, root string) string {
				path := filepath.Join(root, "dangling")
				if err := os.Symlink(filepath.Join(root, "missing"), path); err != nil {
					t.Fatalf("make dangling symlink: %v", err)
				}
				return path
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			home := filepath.Join(t.TempDir(), "home")
			makeDirectory(t, filepath.Join(home, "code"))
			candidate := test.setup(t, t.TempDir())
			resolver := NewResolver(realRepository{})

			// When
			_, err := resolver.Resolve(context.Background(), Request{
				Home:             Directory(home),
				Project:          Value(candidate),
				WorkingDirectory: Directory(t.TempDir()),
			})

			// Then
			if !errors.Is(err, errs.ErrProject) {
				t.Fatalf("error = %v, want project class", err)
			}
		})
	}
}

func TestResolveProject_rejectsEveryManagedRootOverlapWithoutChangingRepositories(t *testing.T) {
	tests := []struct {
		name        string
		projectPath func(string) string
	}{
		{name: "managed root", projectPath: func(home string) string { return filepath.Join(home, "code", ".worktrees") }},
		{name: "managed child", projectPath: func(home string) string { return filepath.Join(home, "code", ".worktrees", "nested") }},
		{name: "managed ancestor", projectPath: func(home string) string { return filepath.Join(home, "code") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			home := filepath.Join(t.TempDir(), "home")
			projectRoot := test.projectPath(home)
			initializeRepository(t, projectRoot)
			before := snapshotSource(t, projectRoot)
			resolver := NewResolver(realRepository{})

			// When
			_, err := resolver.Resolve(context.Background(), Request{
				Home:             Directory(home),
				Project:          Value(projectRoot),
				WorkingDirectory: Directory(t.TempDir()),
			})

			// Then
			if !errors.Is(err, errs.ErrProject) {
				t.Fatalf("error = %v, want project class", err)
			}
			assertSourceUnchanged(t, before, projectRoot)
		})
	}
}
