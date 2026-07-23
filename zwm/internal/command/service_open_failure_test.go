package command

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

func TestServiceExecute_returns_preflight_failure_without_home_before_open_resolution(t *testing.T) {
	// Given
	environment := serviceTestEnvironment{zellij.EnvironmentZellij: "test-session"}
	repository := &serviceTestRepository{}
	tabs := &recordingTabLauncher{}
	service := Service{
		env:       environment,
		preflight: zellij.Config{Runner: serviceTestRunner{}, Environment: environment},
		projects:  project.NewResolver(repository),
		tabs:      tabs,
	}

	// When
	result, err := service.Execute(context.Background(), cli.Invocation{Project: "project", Action: cli.OpenProject{}})

	// Then
	require.Nil(t, result)
	require.ErrorIs(t, err, errs.ErrPreflight)
	require.EqualError(t, err, "HOME is not available")
	require.Empty(t, repository.worktreeRequests)
	require.Empty(t, repository.primaryRequests)
	require.Empty(t, tabs.inputs)
}

func TestServiceExecute_returns_project_failure_for_non_worktree_before_open_launch(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	repository := &serviceTestRepository{}
	tabs := &recordingTabLauncher{}
	service := newOpenProjectTestService(home, repository, tabs)

	// When
	result, err := service.Execute(context.Background(), cli.Invocation{Project: "missing-project", Action: cli.OpenProject{}})

	// Then
	require.Nil(t, result)
	require.ErrorIs(t, err, errs.ErrProject)
	require.Empty(t, repository.worktreeRequests)
	require.Empty(t, repository.primaryRequests)
	require.Empty(t, tabs.inputs)
}

func TestServiceExecute_returns_project_failure_for_managed_root_overlap_before_open_launch(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	projectRoot := filepath.Join(home, "code", ".worktrees", "nested-project")
	require.NoError(t, os.MkdirAll(projectRoot, 0o755))
	repository := &serviceTestRepository{
		worktreeRoot: project.Directory(projectRoot),
		primaryRoot:  project.Directory(projectRoot),
	}
	tabs := &recordingTabLauncher{}
	service := newOpenProjectTestService(home, repository, tabs)

	// When
	result, err := service.Execute(context.Background(), cli.Invocation{
		Project: cli.ProjectNameOrPath(projectRoot),
		Action:  cli.OpenProject{},
	})

	// Then
	require.Nil(t, result)
	require.ErrorIs(t, err, errs.ErrProject)
	require.Empty(t, tabs.inputs)
}

func TestServiceExecute_propagates_open_tab_failure(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{name: "create", cause: errors.New("create Zellij tab")},
		{name: "focus", cause: errors.New("focus Zellij tab")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			home := canonicalTestDirectory(t, t.TempDir())
			projectRoot := filepath.Join(home, "code", "project-name")
			require.NoError(t, os.MkdirAll(projectRoot, 0o755))
			repository := &serviceTestRepository{
				worktreeRoot: project.Directory(projectRoot),
				primaryRoot:  project.Directory(projectRoot),
			}
			tabs := &recordingTabLauncher{err: test.cause}
			service := newOpenProjectTestService(home, repository, tabs)

			// When
			result, err := service.Execute(context.Background(), cli.Invocation{Project: "project-name", Action: cli.OpenProject{}})

			// Then
			require.Nil(t, result)
			require.ErrorIs(t, err, test.cause)
			require.Equal(t, []zellij.Input{{Title: "project-name", Cwd: zellij.Directory(projectRoot)}}, tabs.inputs)
		})
	}
}

func TestServiceExecute_propagates_context_cancellation_before_open_resolution(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	environment := serviceTestEnvironment{
		zellij.EnvironmentHome:   home,
		zellij.EnvironmentZellij: "test-session",
	}
	repository := &serviceTestRepository{}
	tabs := &recordingTabLauncher{}
	service := Service{
		env:       environment,
		preflight: zellij.Config{Runner: serviceTestRunner{}, Environment: environment},
		projects:  project.NewResolver(repository),
		tabs:      tabs,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	result, err := service.Execute(ctx, cli.Invocation{Project: "project-name", Action: cli.OpenProject{}})

	// Then
	require.Nil(t, result)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errs.ErrPreflight)
	require.Empty(t, repository.worktreeRequests)
	require.Empty(t, repository.primaryRequests)
	require.Empty(t, tabs.inputs)
}
