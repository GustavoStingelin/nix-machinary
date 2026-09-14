package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

func TestRemoveWorktree_realGit_deletes_the_checkout_and_keeps_the_branch(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})
	branch := git.Branch("itests/accounts")
	task5Git(t, repository, "branch", string(branch))
	created, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project, Branch: branch})
	require.NoError(t, err)

	result, err := service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{
		Project:  project,
		Worktree: created.Worktree,
	})
	require.NoError(t, err)
	require.Equal(t, branch, result.Branch)
	require.False(t, result.BranchDeleted)

	require.NoDirExists(t, string(created.Worktree))
	require.NotContains(t, string(task5Git(t, repository, "worktree", "list", "--porcelain")), string(created.Worktree))
	require.Contains(t, task5Branches(t, repository), string(branch))
}

func TestRemoveWorktree_realGit_deletes_an_unmerged_branch_when_asked(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})
	branch := git.Branch("zwm/pr-1313-ab12cd34")
	created, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project, Branch: branch})
	require.NoError(t, err)
	// An unmerged commit is the normal case for a pull-request branch, and the
	// one a plain `git branch -d` would refuse.
	task5WriteAndCommit(t, string(created.Worktree), "pr.txt", "pull request work\n")

	result, err := service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{
		Project:      project,
		Worktree:     created.Worktree,
		DeleteBranch: true,
	})
	require.NoError(t, err)
	require.True(t, result.BranchDeleted)

	require.NoDirExists(t, string(created.Worktree))
	require.NotContains(t, task5Branches(t, repository), string(branch))
}

// Git's own refusal is the only guard against discarding work that was never
// committed, so the removal must stay unforced.
func TestRemoveWorktree_realGit_refuses_a_worktree_with_uncommitted_changes(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})
	branch := git.Branch("dirty")
	created, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project, Branch: branch})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(string(created.Worktree), "tracked.txt"), []byte("uncommitted\n"), 0o600))

	_, err = service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{Project: project, Worktree: created.Worktree})
	require.Error(t, err)
	require.Equal(t, errs.External, errs.ClassOf(err))
	require.DirExists(t, string(created.Worktree))
	require.Contains(t, task5Branches(t, repository), string(branch))
}

// A worktree made by hand outside the managed root is exactly the kind that
// accumulates, so it is deletable like any other.
func TestRemoveWorktree_realGit_deletes_a_worktree_made_outside_the_managed_root(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})
	elsewhere := task5Worktree(t, repository, "by-hand")

	result, err := service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{
		Project:  project,
		Worktree: worktree.Path(elsewhere),
	})
	require.NoError(t, err)
	require.Equal(t, git.Branch("by-hand"), result.Branch)
	require.NoDirExists(t, elsewhere)
	require.Contains(t, task5Branches(t, repository), "by-hand")
}

// Deleting the primary worktree would take the repository with it.
func TestRemoveWorktree_realGit_refuses_the_primary_worktree(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})

	_, err := service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{
		Project:  project,
		Worktree: worktree.Path(repository),
	})
	require.Error(t, err)
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	require.DirExists(t, repository)
}

// A path Git does not know about is not a worktree at all, so it is refused
// rather than deleted off the filesystem.
func TestRemoveWorktree_realGit_refuses_an_unregistered_path(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})
	stray := filepath.Join(string(project.ManagedRoot), "stray")
	require.NoError(t, os.MkdirAll(stray, 0o755))

	_, err := service.RemoveWorktree(context.Background(), app.RemoveWorktreeInput{
		Project:  project,
		Worktree: worktree.Path(stray),
	})
	require.Error(t, err)
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	require.DirExists(t, stray)
}

// task5Worktree adds a linked worktree outside the managed root and returns the
// path Git itself reports for it, which on macOS is the symlink-resolved one.
func task5Worktree(t *testing.T, repository, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), branch)
	task5Git(t, repository, "worktree", "add", "--quiet", "-b", branch, path)
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

func task5Branches(t *testing.T, repository string) []string {
	t.Helper()
	raw := task5Git(t, repository, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	return strings.Fields(string(raw))
}
