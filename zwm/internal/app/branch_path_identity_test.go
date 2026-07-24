package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/mocks"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckoutExisting_uses_short_branch_leaf_without_hash_or_output_drift(t *testing.T) {
	// Given
	project := newBranchProject(t)
	branch := git.Branch("mau")
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
	require.Equal(t, filepath.Join(string(project.ManagedRoot), "mau"), path)
	require.Equal(t, checkoutResult(project, branch, path, zellij.Created), result)
}
