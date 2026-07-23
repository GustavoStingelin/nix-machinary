package worktree_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

type realRepositorySnapshot struct {
	Content   []byte
	Refs      []byte
	Status    []byte
	Worktrees []byte
}

func TestValidateTarget_classifies_real_Git_registrations_without_mutating_source(t *testing.T) {
	// Given
	repository := newRealRepository(t)
	managedRoot := filepath.Join(t.TempDir(), "managed root")
	require.NoError(t, os.MkdirAll(managedRoot, 0o755))
	canonicalManagedRoot, err := filepath.EvalSymlinks(managedRoot)
	require.NoError(t, err)
	managedRootPath := worktree.Path(canonicalManagedRoot)
	client := git.NewClient(git.Config{})

	primaryRecords := listRecords(t, client, repository)
	primaryBranch := worktree.Branch(strings.TrimPrefix(string(primaryRecords[0].Branch), "refs/heads/"))
	managedBranch := worktree.Branch("managed/topic")
	managedPath := worktree.ManagedWorktreePath(managedRootPath, string(managedBranch))
	runRealGit(t, repository, "branch", string(managedBranch))
	runRealGit(t, repository, "worktree", "add", "--quiet", string(managedPath), string(managedBranch))

	unmanagedBranch := worktree.Branch("unmanaged/topic")
	unmanagedPath := filepath.Join(t.TempDir(), "unmanaged linked\nworktree")
	runRealGit(t, repository, "branch", string(unmanagedBranch))
	runRealGit(t, repository, "worktree", "add", "--quiet", unmanagedPath, string(unmanagedBranch))

	mismatchBranch := worktree.Branch("mismatch/expected")
	mismatchPath := worktree.ManagedWorktreePath(managedRootPath, string(mismatchBranch))
	runRealGit(t, repository, "branch", "mismatch/other")
	runRealGit(t, repository, "worktree", "add", "--quiet", string(mismatchPath), "mismatch/other")

	detachedBranch := worktree.Branch("detached/topic")
	detachedPath := worktree.ManagedWorktreePath(managedRootPath, string(detachedBranch))
	runRealGit(t, repository, "worktree", "add", "--quiet", "--detach", string(detachedPath), "HEAD")

	occupiedBranch := worktree.Branch("occupied/topic")
	occupiedPath := worktree.ManagedWorktreePath(managedRootPath, string(occupiedBranch))
	require.NoError(t, os.MkdirAll(string(occupiedPath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\ndirty\n"), 0o600))
	before := snapshotRealRepository(t, repository)
	records := listRecords(t, client, repository)

	// When
	primary := worktree.ValidateTarget(worktree.TargetInput{
		Branch: primaryBranch, ManagedPath: worktree.ManagedWorktreePath(managedRootPath, string(primaryBranch)), Records: records,
	})
	managed := worktree.ValidateTarget(worktree.TargetInput{Branch: managedBranch, ManagedPath: managedPath, Records: records})
	unmanaged := worktree.ValidateTarget(worktree.TargetInput{
		Branch: unmanagedBranch, ManagedPath: worktree.ManagedWorktreePath(managedRootPath, string(unmanagedBranch)), Records: records,
	})
	mismatched := worktree.ValidateTarget(worktree.TargetInput{Branch: mismatchBranch, ManagedPath: mismatchPath, Records: records})
	detached := worktree.ValidateTarget(worktree.TargetInput{Branch: detachedBranch, ManagedPath: detachedPath, Records: records})
	occupied := worktree.ValidateTarget(worktree.TargetInput{
		Branch: occupiedBranch, ManagedPath: occupiedPath, Records: records, PathOccupied: true,
	})
	after := snapshotRealRepository(t, repository)

	// Then
	require.Equal(t, worktree.RegistrationAvailable, primary.Registration)
	require.Equal(t, worktree.BranchPrimary, primary.Branch)
	require.Equal(t, worktree.RegistrationReusable, managed.Registration)
	require.Equal(t, worktree.BranchManaged, managed.Branch)
	require.Equal(t, worktree.RegistrationAvailable, unmanaged.Registration)
	require.Equal(t, worktree.BranchUnmanaged, unmanaged.Branch)
	require.Equal(t, worktree.RegistrationMismatched, mismatched.Registration)
	require.Equal(t, worktree.BranchUnregistered, mismatched.Branch)
	require.Equal(t, worktree.RegistrationDetached, detached.Registration)
	require.Equal(t, worktree.BranchUnregistered, detached.Branch)
	require.Equal(t, worktree.RegistrationOccupied, occupied.Registration)
	require.Equal(t, worktree.BranchUnregistered, occupied.Branch)
	require.Equal(t, before, after)
}

func TestValidateTarget_rejects_prunable_real_Git_registration_without_mutating_source(t *testing.T) {
	// Given
	repository := newRealRepository(t)
	managedRoot := filepath.Join(t.TempDir(), "managed root")
	require.NoError(t, os.MkdirAll(managedRoot, 0o755))
	canonicalManagedRoot, err := filepath.EvalSymlinks(managedRoot)
	require.NoError(t, err)
	branch := worktree.Branch("stale/topic")
	managedPath := worktree.ManagedWorktreePath(worktree.Path(canonicalManagedRoot), string(branch))
	runRealGit(t, repository, "branch", string(branch))
	runRealGit(t, repository, "worktree", "add", "--quiet", string(managedPath), string(branch))
	require.NoError(t, os.RemoveAll(string(managedPath)))
	runRealGit(t, repository, "config", "gc.worktreePruneExpire", "now")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\ndirty\n"), 0o600))
	client := git.NewClient(git.Config{})
	records := listRecords(t, client, repository)
	before := snapshotRealRepository(t, repository)
	require.True(t, bytes.Contains(before.Worktrees, []byte("prunable ")))

	// When
	validation := worktree.ValidateTarget(worktree.TargetInput{Branch: branch, ManagedPath: managedPath, Records: records})
	_, validationError := validation.AcceptedPath()
	after := snapshotRealRepository(t, repository)

	// Then
	require.Equal(t, worktree.RegistrationInvalid, validation.Registration)
	require.Equal(t, worktree.BranchManaged, validation.Branch)
	require.ErrorIs(t, validationError, worktree.ErrInvalidTarget)
	require.Equal(t, before, after)
}

func newRealRepository(t *testing.T) string {
	t.Helper()

	repository := filepath.Join(t.TempDir(), "repository")
	require.NoError(t, os.MkdirAll(repository, 0o755))
	runRealGit(t, repository, "init", "--quiet")
	runRealGit(t, repository, "config", "user.email", "zwm-test@example.invalid")
	runRealGit(t, repository, "config", "user.name", "ZWM Test")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\n"), 0o600))
	runRealGit(t, repository, "add", "tracked.txt")
	runRealGit(t, repository, "commit", "--quiet", "-m", "initial")
	return repository
}

func listRecords(t *testing.T, client git.Client, repository string) []worktree.Record {
	t.Helper()

	raw, err := client.ListWorktrees(context.Background(), git.Directory(repository))
	require.NoError(t, err)
	records, err := worktree.ParsePorcelainZ(raw)
	require.NoError(t, err)
	return records
}

func snapshotRealRepository(t *testing.T, repository string) realRepositorySnapshot {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repository, "tracked.txt"))
	require.NoError(t, err)
	return realRepositorySnapshot{
		Content:   content,
		Refs:      runRealGit(t, repository, "for-each-ref", "--format=%(refname):%(objectname)"),
		Status:    runRealGit(t, repository, "status", "--porcelain=v1"),
		Worktrees: runRealGit(t, repository, "worktree", "list", "--porcelain", "-z"),
	}
}

func runRealGit(t *testing.T, directory string, arguments ...string) []byte {
	t.Helper()

	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(arguments, " "), output)
	return output
}
