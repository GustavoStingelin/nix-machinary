package app_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

type task5SourceSnapshot struct {
	content []byte
	status  []byte
}

type task5Tabs struct {
	titles map[zellij.TabTitle]struct{}
}

func (tabs *task5Tabs) Launch(_ context.Context, input zellij.Input) (zellij.Result, error) {
	if tabs.titles == nil {
		tabs.titles = make(map[zellij.TabTitle]struct{})
	}
	if _, found := tabs.titles[input.Title]; found {
		return zellij.Result{Action: zellij.Focused, Title: input.Title, Cwd: input.Cwd}, nil
	}
	tabs.titles[input.Title] = struct{}{}
	return zellij.Result{Action: zellij.Created, Title: input.Title, Cwd: input.Cwd}, nil
}

func task5NewRepository(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repository")
	require.NoError(t, os.MkdirAll(repository, 0o755))
	task5Git(t, repository, "init", "--quiet")
	task5Git(t, repository, "config", "user.email", "zwm-test@example.invalid")
	task5Git(t, repository, "config", "user.name", "ZWM Test")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\n"), 0o600))
	task5Git(t, repository, "add", "tracked.txt")
	task5Git(t, repository, "commit", "--quiet", "-m", "initial")
	canonical, err := filepath.EvalSymlinks(repository)
	require.NoError(t, err)
	return canonical
}

func task5Project(t *testing.T, root string, invocation string) project.Resolution {
	t.Helper()
	managedRoot := filepath.Join(t.TempDir(), "managed")
	canonicalManagedRoot, err := filepath.EvalSymlinks(filepath.Dir(managedRoot))
	require.NoError(t, err)
	return project.Resolution{
		InvocationWorktree: project.Directory(invocation),
		Key:                "project-key",
		ManagedRoot:        project.Directory(filepath.Join(canonicalManagedRoot, filepath.Base(managedRoot))),
		ProjectRoot:        project.Directory(root),
	}
}

func task5WriteAndCommit(t *testing.T, directory string, name string, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600))
	task5Git(t, directory, "add", name)
	task5Git(t, directory, "commit", "--quiet", "-m", name)
}

func task5SnapshotSource(t *testing.T, directory string) task5SourceSnapshot {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(directory, "tracked.txt"))
	require.NoError(t, err)
	return task5SourceSnapshot{content: content, status: task5Git(t, directory, "status", "--porcelain=v1")}
}

func task5RequireSourceContentAndStatus(t *testing.T, before task5SourceSnapshot, directory string) {
	t.Helper()
	after := task5SnapshotSource(t, directory)
	require.True(t, bytes.Equal(before.content, after.content))
	require.True(t, bytes.Equal(before.status, after.status))
}

func task5Git(t *testing.T, directory string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(arguments, " "), output)
	return output
}
