package command

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

type serviceTestEnvironment map[zellij.EnvironmentVariable]string

func (environment serviceTestEnvironment) Lookup(variable zellij.EnvironmentVariable) (string, bool) {
	value, present := environment[variable]
	return value, present
}

type serviceTestRunner struct {
	availability map[zellij.CommandName]error
}

func (runner serviceTestRunner) Available(ctx context.Context, command zellij.CommandName) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return runner.availability[command]
}

func (serviceTestRunner) Run(context.Context, zellij.Command) (zellij.Output, error) {
	return zellij.Output{}, nil
}

type serviceTestRepository struct {
	worktreeRoot     project.Directory
	primaryRoot      project.Directory
	worktreeRootErr  error
	primaryRootErr   error
	worktreeRequests []project.Directory
	primaryRequests  []project.Directory
}

func (repository *serviceTestRepository) WorktreeRoot(_ context.Context, directory project.Directory) (project.Directory, error) {
	repository.worktreeRequests = append(repository.worktreeRequests, directory)
	return repository.worktreeRoot, repository.worktreeRootErr
}

func (repository *serviceTestRepository) PrimaryWorktreeRoot(_ context.Context, directory project.Directory) (project.Directory, error) {
	repository.primaryRequests = append(repository.primaryRequests, directory)
	return repository.primaryRoot, repository.primaryRootErr
}

type recordingTabLauncher struct {
	inputs []zellij.Input
	result zellij.Result
	err    error
}

func (launcher *recordingTabLauncher) Launch(_ context.Context, input zellij.Input) (zellij.Result, error) {
	launcher.inputs = append(launcher.inputs, input)
	return launcher.result, launcher.err
}

func newOpenProjectTestService(home string, repository project.Repository, tabs app.TabLauncher) Service {
	environment := serviceTestEnvironment{
		zellij.EnvironmentHome:   home,
		zellij.EnvironmentZellij: "test-session",
	}
	return Service{
		env:       environment,
		preflight: zellij.Config{Runner: serviceTestRunner{}, Environment: environment},
		projects:  project.NewResolver(repository),
		tabs:      tabs,
	}
}

func canonicalTestDirectory(t *testing.T, directory string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(directory)
	require.NoError(t, err)
	return canonical
}
