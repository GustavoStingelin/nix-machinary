package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

func TestServiceExecute_opens_bare_project_name_at_primary_root(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	projectRoot := filepath.Join(home, "code", "project-name")
	require.NoError(t, os.MkdirAll(projectRoot, 0o755))
	repository := &serviceTestRepository{
		worktreeRoot: project.Directory(projectRoot),
		primaryRoot:  project.Directory(projectRoot),
	}
	tabs := &recordingTabLauncher{result: zellij.Result{
		Action: zellij.Created,
		Title:  "project-name",
		Cwd:    zellij.Directory(projectRoot),
	}}
	service := newOpenProjectTestService(home, repository, tabs)

	// When
	result, err := service.Execute(context.Background(), cli.Invocation{
		Project: "project-name",
		Action:  cli.OpenProject{},
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, cli.OpenProjectResult{
		ProjectRoot: projectRoot,
		TabAction:   "created",
		TabTitle:    "project-name",
		TabCwd:      projectRoot,
	}, result)
	require.Equal(t, []zellij.Input{{
		Title: "project-name",
		Cwd:   zellij.Directory(projectRoot),
	}}, tabs.inputs)
	require.Equal(t, []project.Directory{project.Directory(projectRoot)}, repository.worktreeRequests)
	require.Equal(t, []project.Directory{project.Directory(projectRoot)}, repository.primaryRequests)
}

func TestServiceExecute_opens_linked_worktree_selection_at_primary_root(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	projectRoot := filepath.Join(home, "code", "canonical-project")
	linkedWorktree := filepath.Join(home, "linked", "feature-worktree")
	require.NoError(t, os.MkdirAll(projectRoot, 0o755))
	require.NoError(t, os.MkdirAll(linkedWorktree, 0o755))
	repository := &serviceTestRepository{
		worktreeRoot: project.Directory(linkedWorktree),
		primaryRoot:  project.Directory(projectRoot),
	}
	tabs := &recordingTabLauncher{result: zellij.Result{
		Action: zellij.Focused,
		Title:  "canonical-project",
		Cwd:    zellij.Directory(projectRoot),
	}}
	service := newOpenProjectTestService(home, repository, tabs)

	// When
	result, err := service.Execute(context.Background(), cli.Invocation{
		Project: cli.ProjectNameOrPath(linkedWorktree),
		Action:  cli.OpenProject{},
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, cli.OpenProjectResult{
		ProjectRoot: projectRoot,
		TabAction:   "focused",
		TabTitle:    "canonical-project",
		TabCwd:      projectRoot,
	}, result)
	require.Equal(t, []zellij.Input{{
		Title: "canonical-project",
		Cwd:   zellij.Directory(projectRoot),
	}}, tabs.inputs)
	require.Equal(t, []project.Directory{project.Directory(linkedWorktree)}, repository.worktreeRequests)
	require.Equal(t, []project.Directory{project.Directory(linkedWorktree)}, repository.primaryRequests)
}
