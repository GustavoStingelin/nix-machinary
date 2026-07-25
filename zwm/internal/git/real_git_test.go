package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/stretchr/testify/require"
)

type repositorySnapshot struct {
	Content   []byte
	Refs      []byte
	Status    []byte
	Worktrees []byte
}

func TestClient_validation_does_not_mutate_real_Git_repository_when_targets_are_rejected(t *testing.T) {
	// Given
	repository := newRepository(t)
	commit := strings.TrimSpace(string(runGit(t, repository, "rev-parse", "HEAD")))
	runGit(t, repository, "branch", "local-only")
	runGit(t, repository, "update-ref", "refs/remotes/origin/remote-only", commit)
	runGit(t, repository, "tag", "-m", "tag fixture", "tag-only")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\ndirty\n"), 0o600))
	before := snapshotRepository(t, repository)
	client := git.NewClient(git.Config{})

	// When
	branchError := client.ValidateBranch(context.Background(), git.Directory(repository), git.Branch("invalid..branch"))
	commitError := func() error {
		_, err := client.ResolveCommit(context.Background(), git.Directory(repository), git.Commitish("missing-start-point"))
		return err
	}()
	localExists, localError := client.LocalBranchExists(context.Background(), git.Directory(repository), git.Branch("local-only"))
	remoteExists, remoteError := client.LocalBranchExists(context.Background(), git.Directory(repository), git.Branch("remote-only"))
	tagExists, tagError := client.LocalBranchExists(context.Background(), git.Directory(repository), git.Branch("tag-only"))
	worktrees, worktreeError := client.ListWorktrees(context.Background(), git.Directory(repository))
	after := snapshotRepository(t, repository)

	// Then
	require.ErrorIs(t, branchError, git.ErrInvalidBranch)
	require.ErrorIs(t, commitError, git.ErrInvalidCommitish)
	require.NoError(t, localError)
	require.True(t, localExists)
	require.NoError(t, remoteError)
	require.False(t, remoteExists)
	require.NoError(t, tagError)
	require.False(t, tagExists)
	require.NoError(t, worktreeError)
	require.NotEmpty(t, worktrees)
	require.Equal(t, before, after)
}

func TestClient_ListLocalBranches_returns_local_heads_without_mutating_the_repository(t *testing.T) {
	// Given
	repository := newRepository(t)
	commit := strings.TrimSpace(string(runGit(t, repository, "rev-parse", "HEAD")))
	runGit(t, repository, "branch", "feature/one")
	runGit(t, repository, "branch", "bugfix/two")
	runGit(t, repository, "update-ref", "refs/remotes/origin/remote-only", commit)
	runGit(t, repository, "tag", "-m", "tag fixture", "tag-only")
	defaultBranch := git.Branch(strings.TrimSpace(string(runGit(t, repository, "rev-parse", "--abbrev-ref", "HEAD"))))
	before := snapshotRepository(t, repository)
	client := git.NewClient(git.Config{})

	// When
	branches, err := client.ListLocalBranches(context.Background(), git.Directory(repository))
	after := snapshotRepository(t, repository)

	// Then
	require.NoError(t, err)
	require.ElementsMatch(t, []git.Branch{defaultBranch, "feature/one", "bugfix/two"}, branches)
	require.Equal(t, before, after)
}

func newRepository(t *testing.T) string {
	t.Helper()

	repository := filepath.Join(t.TempDir(), "repository")
	require.NoError(t, os.MkdirAll(repository, 0o755))
	runGit(t, repository, "init", "--quiet")
	runGit(t, repository, "config", "user.email", "zwm-test@example.invalid")
	runGit(t, repository, "config", "user.name", "ZWM Test")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\n"), 0o600))
	runGit(t, repository, "add", "tracked.txt")
	runGit(t, repository, "commit", "--quiet", "-m", "initial")
	return repository
}

func snapshotRepository(t *testing.T, repository string) repositorySnapshot {
	t.Helper()

	return repositorySnapshot{
		Content:   readFile(t, filepath.Join(repository, "tracked.txt")),
		Refs:      runGit(t, repository, "for-each-ref", "--format=%(refname):%(objectname)"),
		Status:    runGit(t, repository, "status", "--porcelain=v1"),
		Worktrees: runGit(t, repository, "worktree", "list", "--porcelain", "-z"),
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func runGit(t *testing.T, directory string, arguments ...string) []byte {
	t.Helper()

	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(arguments, " "), output)
	return output
}
