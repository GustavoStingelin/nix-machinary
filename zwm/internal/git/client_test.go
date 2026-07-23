package git_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/stretchr/testify/require"
)

type invocation struct {
	Arguments []string
	Directory string
}

func TestClient_ValidateBranch_uses_exact_argv_and_cwd_when_branch_is_valid(t *testing.T) {
	// Given
	client, recordPath := helperClient(t)
	directory := t.TempDir()

	// When
	err := client.ValidateBranch(context.Background(), git.Directory(directory), git.Branch("feature/topic"))

	// Then
	require.NoError(t, err)
	require.Equal(t, invocation{Arguments: []string{"check-ref-format", "--branch", "feature/topic"}, Directory: directory}, readInvocation(t, recordPath))
}

func TestClient_ValidateBranch_returns_typed_error_when_Git_rejects_branch(t *testing.T) {
	// Given
	client, _ := helperClient(t)
	t.Setenv("GIT_HELPER_EXIT", "1")

	// When
	err := client.ValidateBranch(context.Background(), git.Directory(t.TempDir()), git.Branch("invalid..branch"))

	// Then
	var branchError *git.InvalidBranchError
	require.ErrorAs(t, err, &branchError)
	require.ErrorIs(t, err, git.ErrInvalidBranch)
}

func TestClient_ResolveCommit_uses_commit_peeling_and_returns_stdout_value(t *testing.T) {
	// Given
	client, recordPath := helperClient(t)
	directory := t.TempDir()

	// When
	commit, err := client.ResolveCommit(context.Background(), git.Directory(directory), git.Commitish("origin/main"))

	// Then
	require.NoError(t, err)
	require.Equal(t, git.Commit("0123456789abcdef0123456789abcdef01234567"), commit)
	require.Equal(t, invocation{Arguments: []string{"rev-parse", "--verify", "--quiet", "--end-of-options", "origin/main^{commit}"}, Directory: directory}, readInvocation(t, recordPath))
}

func TestClient_LocalBranchExists_uses_exact_local_ref_without_remote_or_tag_guessing(t *testing.T) {
	// Given
	client, recordPath := helperClient(t)
	directory := t.TempDir()

	// When
	exists, err := client.LocalBranchExists(context.Background(), git.Directory(directory), git.Branch("topic"))

	// Then
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, invocation{Arguments: []string{"show-ref", "--verify", "--quiet", "refs/heads/topic"}, Directory: directory}, readInvocation(t, recordPath))
}

func TestClient_ListWorktrees_preserves_NUL_delimited_stdout(t *testing.T) {
	// Given
	client, _ := helperClient(t)
	raw := "worktree /tmp/linked\x00HEAD 0123456789abcdef0123456789abcdef01234567\x00detached\x00\x00"
	t.Setenv("GIT_HELPER_OUTPUT_BASE64", base64.StdEncoding.EncodeToString([]byte(raw)))

	// When
	got, err := client.ListWorktrees(context.Background(), git.Directory(t.TempDir()))

	// Then
	require.NoError(t, err)
	require.Equal(t, []byte(raw), got)
}

func TestClient_captures_stdout_stderr_and_exit_error_when_command_fails(t *testing.T) {
	// Given
	client, _ := helperClient(t)
	t.Setenv("GIT_HELPER_EXIT", "42")

	// When
	_, err := client.ListWorktrees(context.Background(), git.Directory(t.TempDir()))

	// Then
	var commandError *git.CommandError
	require.ErrorAs(t, err, &commandError)
	require.Equal(t, []byte("stdout before failure"), commandError.Stdout)
	require.Equal(t, []byte("stderr before failure"), commandError.Stderr)
	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError)
}

func TestClient_stops_hung_command_when_context_is_cancelled_and_can_resume(t *testing.T) {
	// Given
	client, _ := helperClient(t)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	defer signal.Stop(signals)
	t.Setenv("GIT_HELPER_HANG", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	directory := git.Directory(t.TempDir())
	watchdog := time.NewTimer(30 * time.Second)
	defer watchdog.Stop()

	// When
	go func() {
		_, err := client.ListWorktrees(ctx, directory)
		result <- err
	}()

	select {
	case <-signals:
		cancel()
	case <-watchdog.C:
		t.Fatal("helper process did not start")
	}

	var err error
	select {
	case err = <-result:
	case <-watchdog.C:
		t.Fatal("cancelled helper process did not terminate")
	}

	// Then
	require.ErrorIs(t, err, context.Canceled)
	t.Setenv("GIT_HELPER_HANG", "")
	_, err = client.ListWorktrees(context.Background(), git.Directory(t.TempDir()))
	require.NoError(t, err)
}

func helperClient(t *testing.T) (git.Client, string) {
	t.Helper()

	helper := filepath.Join(t.TempDir(), "git-helper")
	build := exec.Command("go", "build", "-o", helper, "./testdata/git-helper")
	build.Dir = "."
	build.Env = os.Environ()
	require.NoError(t, build.Run())
	recordPath := filepath.Join(t.TempDir(), "invocation.json")
	t.Setenv("GIT_HELPER_RECORD", recordPath)

	return git.NewClient(git.Config{Executable: helper}), recordPath
}

func readInvocation(t *testing.T, recordPath string) invocation {
	t.Helper()

	raw, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	var got invocation
	require.NoError(t, json.Unmarshal(raw, &got))
	return got
}

func TestClient_does_not_mask_invalid_commit_as_command_failure(t *testing.T) {
	// Given
	client, _ := helperClient(t)
	t.Setenv("GIT_HELPER_EXIT", "1")

	// When
	_, err := client.ResolveCommit(context.Background(), git.Directory(t.TempDir()), git.Commitish("missing"))

	// Then
	var commitishError *git.InvalidCommitishError
	require.ErrorAs(t, err, &commitishError)
	require.True(t, errors.Is(err, git.ErrInvalidCommitish))
}

func TestClient_AddWorktree_uses_exact_branch_creation_argv_and_cwd(t *testing.T) {
	tests := []struct {
		name      string
		invoke    func(git.Client, git.Directory) error
		arguments []string
	}{
		{
			name: "existing branch",
			invoke: func(client git.Client, directory git.Directory) error {
				return client.AddExistingWorktree(context.Background(), directory, git.WorktreePath("/managed/feature"), git.Branch("feature/topic"))
			},
			arguments: []string{"worktree", "add", "/managed/feature", "feature/topic"},
		},
		{
			name: "new branch",
			invoke: func(client git.Client, directory git.Directory) error {
				return client.AddNewWorktree(context.Background(), directory, git.Branch("new/topic"), git.WorktreePath("/managed/new"), git.Commit("0123456789abcdef0123456789abcdef01234567"))
			},
			arguments: []string{"worktree", "add", "-b", "new/topic", "/managed/new", "0123456789abcdef0123456789abcdef01234567"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, recordPath := helperClient(t)
			directory := git.Directory(t.TempDir())
			require.NoError(t, test.invoke(client, directory))
			require.Equal(t, invocation{Arguments: test.arguments, Directory: string(directory)}, readInvocation(t, recordPath))
		})
	}
}
