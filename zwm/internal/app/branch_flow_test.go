package app_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/mocks"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckoutExisting_creates_managed_worktree_for_exact_local_slash_branch(t *testing.T) {
	// Given
	project := newBranchProject(t)
	branch := git.Branch("feature/with-slash")
	path := managedPath(project, branch)
	branchGit := mocks.NewMockBranchGit(t)
	tabs := mocks.NewMockTabLauncher(t)
	expectExistingPreparation(branchGit, project, branch, availableWorktrees(project))
	branchGit.EXPECT().AddExistingWorktree(mock.Anything, git.Directory(project.ProjectRoot), git.WorktreePath(path), branch).Return(nil).Once()
	expectTabLaunch(tabs, project, branch, path, zellij.Created)
	service := app.NewBranchService(branchGit, tabs)

	// When
	result, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project.Resolution, Branch: branch})

	// Then
	require.NoError(t, err)
	require.Equal(t, checkoutResult(project, branch, path, zellij.Created), result)
}

func TestCheckoutExisting_reuses_only_exact_managed_registration(t *testing.T) {
	// Given
	project := newBranchProject(t)
	branch := git.Branch("feature/with-slash")
	path := managedPath(project, branch)
	branchGit := mocks.NewMockBranchGit(t)
	tabs := mocks.NewMockTabLauncher(t)
	expectExistingPreparation(branchGit, project, branch, managedWorktrees(project, branch, path, false))
	expectTabLaunch(tabs, project, branch, path, zellij.Focused)
	service := app.NewBranchService(branchGit, tabs)

	// When
	result, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project.Resolution, Branch: branch})

	// Then
	require.NoError(t, err)
	require.Equal(t, checkoutResult(project, branch, path, zellij.Focused), result)
}

func TestCheckoutExisting_rejects_invalid_missing_or_unavailable_targets(t *testing.T) {
	tests := []struct {
		name        string
		branch      git.Branch
		prepare     func(*mocks.MockBranchGit, branchProject, git.Branch)
		preparePath func(string)
		wantMessage string
		wantError   error
	}{
		{
			name:        "invalid branch",
			branch:      "invalid..branch",
			wantMessage: "invalid branch 'invalid..branch'",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(&git.InvalidBranchError{Branch: branch, Cause: &git.CommandError{Cause: errors.New("invalid branch")}}).Once()
			},
		},
		{
			name:        "missing local branch",
			branch:      "missing/topic",
			wantMessage: "local branch 'missing/topic' does not exist",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(nil).Once()
				branchGit.EXPECT().LocalBranchExists(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(false, nil).Once()
			},
		},
		{
			name:        "remote-only branch",
			branch:      "remote-only/topic",
			wantMessage: "local branch 'remote-only/topic' does not exist",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(nil).Once()
				branchGit.EXPECT().LocalBranchExists(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(false, nil).Once()
			},
		},
		{
			name:        "primary branch",
			branch:      "main",
			wantMessage: "branch 'main' is not available for a managed worktree",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				expectExistingPreparation(branchGit, project, branch, primaryWorktrees(project, branch))
			},
		},
		{
			name:        "unmanaged linked branch",
			branch:      "unmanaged/topic",
			wantMessage: "branch 'unmanaged/topic' is not available for a managed worktree",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				expectExistingPreparation(branchGit, project, branch, unmanagedWorktrees(project, branch))
			},
		},
		{
			name:        "occupied managed path",
			branch:      "occupied/topic",
			wantMessage: "branch 'occupied/topic' is not available for a managed worktree",
			wantError:   errs.ErrUsage,
			preparePath: func(path string) {
				require.NoError(t, os.MkdirAll(path, 0o755))
			},
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				expectExistingPreparation(branchGit, project, branch, availableWorktrees(project))
			},
		},
		{
			name:        "prunable registration",
			branch:      "stale/topic",
			wantMessage: "branch 'stale/topic' is not available for a managed worktree",
			wantError:   errs.ErrUsage,
			prepare: func(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch) {
				path := managedPath(project, branch)
				expectExistingPreparation(branchGit, project, branch, managedWorktrees(project, branch, path, true))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newBranchProject(t)
			path := managedPath(project, test.branch)
			if test.preparePath != nil {
				test.preparePath(path)
			}
			branchGit := mocks.NewMockBranchGit(t)
			tabs := mocks.NewMockTabLauncher(t)
			test.prepare(branchGit, project, test.branch)
			service := app.NewBranchService(branchGit, tabs)

			// When
			_, err := service.CheckoutExisting(context.Background(), app.CheckoutExistingInput{Project: project.Resolution, Branch: test.branch})

			// Then
			require.ErrorIs(t, err, test.wantError)
			require.Equal(t, 64, errs.ExitCode(err))
			require.Equal(t, test.wantMessage, err.Error())
		})
	}
}

func TestCheckoutNew_rejects_invalid_start_point_before_target_or_tab_operations(t *testing.T) {
	project := newBranchProject(t)
	branch := git.Branch("new/invalid-start")
	startPoint := git.Commitish("missing-start-point")
	branchGit := mocks.NewMockBranchGit(t)
	tabs := mocks.NewMockTabLauncher(t)
	branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(nil).Once()
	branchGit.EXPECT().ResolveCommit(mock.Anything, git.Directory(project.InvocationWorktree), startPoint).Return("", &git.InvalidCommitishError{Commitish: startPoint, Cause: &git.CommandError{Cause: errors.New("invalid commit-ish")}}).Once()
	service := app.NewBranchService(branchGit, tabs)

	_, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project.Resolution, Branch: branch, StartPoint: startPoint})

	require.ErrorIs(t, err, errs.ErrUsage)
	require.Equal(t, 64, errs.ExitCode(err))
	require.Equal(t, "invalid start-point 'missing-start-point'", err.Error())
}

func TestCheckoutNew_creates_from_invocation_head_and_preserves_result_fields(t *testing.T) {
	// Given
	project := newBranchProject(t)
	branch := git.Branch("new/current-head")
	path := managedPath(project, branch)
	commit := git.Commit("0123456789abcdef0123456789abcdef01234567")
	branchGit := mocks.NewMockBranchGit(t)
	tabs := mocks.NewMockTabLauncher(t)
	expectNewPreparation(branchGit, project, branch, "HEAD", commit, false, availableWorktrees(project))
	branchGit.EXPECT().AddNewWorktree(mock.Anything, git.Directory(project.ProjectRoot), branch, git.WorktreePath(path), commit).Return(nil).Once()
	expectTabLaunch(tabs, project, branch, path, zellij.Created)
	service := app.NewBranchService(branchGit, tabs)

	// When
	result, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project.Resolution, Branch: branch})

	// Then
	require.NoError(t, err)
	require.Equal(t, checkoutResult(project, branch, path, zellij.Created), result)
}

func TestCheckoutNew_resolves_explicit_commitish_verbatim_from_invocation_worktree(t *testing.T) {
	for _, startPoint := range []git.Commitish{"master", "origin/main"} {
		t.Run(string(startPoint), func(t *testing.T) {
			// Given
			project := newBranchProject(t)
			branch := git.Branch("new/from-" + string(startPoint))
			path := managedPath(project, branch)
			commit := git.Commit("0123456789abcdef0123456789abcdef01234567")
			branchGit := mocks.NewMockBranchGit(t)
			tabs := mocks.NewMockTabLauncher(t)
			expectNewPreparation(branchGit, project, branch, startPoint, commit, false, availableWorktrees(project))
			branchGit.EXPECT().AddNewWorktree(mock.Anything, git.Directory(project.ProjectRoot), branch, git.WorktreePath(path), commit).Return(nil).Once()
			expectTabLaunch(tabs, project, branch, path, zellij.Created)
			service := app.NewBranchService(branchGit, tabs)

			// When
			_, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project.Resolution, Branch: branch, StartPoint: startPoint})

			// Then
			require.NoError(t, err)
		})
	}
}

func TestCheckoutNew_reuses_exact_registration_and_rejects_collisions(t *testing.T) {
	tests := []struct {
		name        string
		worktrees   []byte
		wantMessage string
		wantResult  bool
	}{
		{name: "exact reusable registration", wantResult: true},
		{name: "unregistered local collision", wantMessage: "local branch 'existing/collision' already exists"},
		{name: "mismatched registration", wantMessage: "local branch 'existing/collision' already exists"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newBranchProject(t)
			branch := git.Branch("existing/collision")
			path := managedPath(project, branch)
			if test.name == "exact reusable registration" {
				test.worktrees = managedWorktrees(project, branch, path, false)
			}
			if test.name == "unregistered local collision" {
				test.worktrees = availableWorktrees(project)
			}
			if test.name == "mismatched registration" {
				test.worktrees = managedWorktrees(project, "other/topic", path, false)
			}
			branchGit := mocks.NewMockBranchGit(t)
			tabs := mocks.NewMockTabLauncher(t)
			commit := git.Commit("0123456789abcdef0123456789abcdef01234567")
			expectNewPreparation(branchGit, project, branch, "HEAD", commit, true, test.worktrees)
			if test.wantResult {
				expectTabLaunch(tabs, project, branch, path, zellij.Focused)
			}
			service := app.NewBranchService(branchGit, tabs)

			// When
			result, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project.Resolution, Branch: branch})

			// Then
			if test.wantResult {
				require.NoError(t, err)
				require.Equal(t, checkoutResult(project, branch, path, zellij.Focused), result)
				return
			}
			require.ErrorIs(t, err, errs.ErrUsage)
			require.Equal(t, test.wantMessage, err.Error())
		})
	}
}

func TestCheckoutNew_retains_add_failure_and_never_launches_tab(t *testing.T) {
	// Given
	project := newBranchProject(t)
	branch := git.Branch("new/add-failure")
	path := managedPath(project, branch)
	commit := git.Commit("0123456789abcdef0123456789abcdef01234567")
	addFailure := &git.CommandError{Arguments: []string{"worktree", "add", "-b", string(branch), path, string(commit)}, Directory: string(project.ProjectRoot), Stderr: []byte("external add failure\n"), Cause: errors.New("exit status 1")}
	branchGit := mocks.NewMockBranchGit(t)
	tabs := mocks.NewMockTabLauncher(t)
	expectNewPreparation(branchGit, project, branch, "HEAD", commit, false, availableWorktrees(project))
	branchGit.EXPECT().AddNewWorktree(mock.Anything, git.Directory(project.ProjectRoot), branch, git.WorktreePath(path), commit).Return(addFailure).Once()
	service := app.NewBranchService(branchGit, tabs)

	// When
	_, err := service.CheckoutNew(context.Background(), app.CheckoutNewInput{Project: project.Resolution, Branch: branch})

	// Then
	require.ErrorIs(t, err, errs.ErrExternal)
	require.Equal(t, 1, errs.ExitCode(err))
	var commandError *git.CommandError
	require.ErrorAs(t, err, &commandError)
	require.Equal(t, []byte("external add failure\n"), commandError.Stderr)
}
