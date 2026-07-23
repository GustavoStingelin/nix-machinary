package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

func TestCheckoutExisting_realGit_creates_and_reuses_managed_slash_branch_without_changing_dirty_source(t *testing.T) {
	repository := task5NewRepository(t)
	branch := git.Branch("feature/with-slash")
	task5Git(t, repository, "branch", string(branch))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("initial\ndirty\n"), 0o600))
	before := task5SnapshotSource(t, repository)
	project := task5Project(t, repository, repository)
	tabs := &task5Tabs{}
	service := app.NewBranchService(git.NewClient(git.Config{}), tabs)
	path := string(worktree.ManagedWorktreePath(worktree.Path(project.ManagedRoot), string(branch)))

	created, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project, Branch: branch})
	require.NoError(t, err)
	require.Equal(t, zellij.Created, created.TabAction)
	require.Equal(t, path, string(created.Worktree))
	require.Equal(t, string(branch), strings.TrimSpace(string(task5Git(t, path, "symbolic-ref", "--short", "HEAD"))))
	task5RequireSourceContentAndStatus(t, before, repository)
	afterCreate := task5SnapshotSource(t, repository)

	reused, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project, Branch: branch})
	require.NoError(t, err)
	require.Equal(t, zellij.Focused, reused.TabAction)
	require.Equal(t, afterCreate, task5SnapshotSource(t, repository))
}

func TestCheckoutNew_realGit_uses_linked_source_head_and_explicit_commitishes(t *testing.T) {
	repository := task5NewRepository(t)
	task5Git(t, repository, "branch", "-M", "master")
	masterCommit := strings.TrimSpace(string(task5Git(t, repository, "rev-parse", "master")))
	source := filepath.Join(t.TempDir(), "source")
	task5Git(t, repository, "worktree", "add", "--quiet", "-b", "source/current", source, "master")
	task5WriteAndCommit(t, source, "source.txt", "source branch\n")
	sourceCommit := strings.TrimSpace(string(task5Git(t, source, "rev-parse", "HEAD")))
	remote := filepath.Join(t.TempDir(), "remote")
	task5Git(t, repository, "worktree", "add", "--quiet", "-b", "fixture/origin-main", remote, "master")
	task5WriteAndCommit(t, remote, "remote.txt", "remote branch\n")
	remoteCommit := strings.TrimSpace(string(task5Git(t, remote, "rev-parse", "HEAD")))
	task5Git(t, repository, "update-ref", "refs/remotes/origin/main", remoteCommit)
	require.NoError(t, os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("initial\ndirty\n"), 0o600))
	before := task5SnapshotSource(t, source)
	project := task5Project(t, repository, source)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})

	tests := []struct {
		branch     git.Branch
		startPoint git.Commitish
		wantCommit string
	}{
		{branch: "new/current-head", wantCommit: sourceCommit},
		{branch: "new/from-master", startPoint: "master", wantCommit: masterCommit},
		{branch: "new/from-origin-main", startPoint: "origin/main", wantCommit: remoteCommit},
	}
	for _, test := range tests {
		t.Run(string(test.branch), func(t *testing.T) {
			result, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project, Branch: test.branch, StartPoint: test.startPoint})
			require.NoError(t, err)
			require.Equal(t, test.wantCommit, strings.TrimSpace(string(task5Git(t, repository, "rev-parse", string(test.branch)))))
			require.Equal(t, string(test.branch), strings.TrimSpace(string(task5Git(t, string(result.Worktree), "symbolic-ref", "--short", "HEAD"))))
		})
	}
	task5RequireSourceContentAndStatus(t, before, source)
}

func TestCheckoutNew_realGit_uses_distinct_managed_paths_when_displays_collide(t *testing.T) {
	repository := task5NewRepository(t)
	project := task5Project(t, repository, repository)
	service := app.NewBranchService(git.NewClient(git.Config{}), &task5Tabs{})

	first, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project, Branch: "collision/a"})
	require.NoError(t, err)
	second, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project, Branch: "collision-a"})
	require.NoError(t, err)
	require.NotEqual(t, first.Worktree, second.Worktree)
	require.Equal(t, worktree.ManagedDisplay("collision/a"), worktree.ManagedDisplay("collision-a"))
}
