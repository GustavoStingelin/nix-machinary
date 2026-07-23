package app_test

import (
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/mocks"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/mock"
)

const testOID = "0123456789abcdef0123456789abcdef01234567"

type branchProject struct {
	project.Resolution
}

func newBranchProject(t *testing.T) branchProject {
	t.Helper()
	root := t.TempDir()
	return branchProject{Resolution: project.Resolution{
		InvocationWorktree: project.Directory(root),
		Key:                "project-key",
		ManagedRoot:        project.Directory(filepath.Join(root, "managed")),
		ProjectRoot:        project.Directory(root),
	}}
}

func managedPath(project branchProject, branch git.Branch) string {
	return string(worktree.ManagedWorktreePath(worktree.Path(project.ManagedRoot), string(branch)))
}

func checkoutResult(project branchProject, branch git.Branch, path string, action zellij.Action) app.CheckoutResult {
	title := zellij.TabTitle(string(project.Key) + ":" + string(branch))
	return app.CheckoutResult{
		Worktree:        worktree.Path(path),
		DisplayIdentity: string(branch),
		TabAction:       action,
		TabTitle:        title,
		TabWorktree:     zellij.WorktreePath(path),
	}
}

func expectExistingPreparation(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch, records []byte) {
	mock.InOrder(
		branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(nil).Once(),
		branchGit.EXPECT().LocalBranchExists(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(true, nil).Once(),
		branchGit.EXPECT().ListWorktrees(mock.Anything, git.Directory(project.ProjectRoot)).Return(records, nil).Once(),
	)
}

func expectNewPreparation(branchGit *mocks.MockBranchGit, project branchProject, branch git.Branch, startPoint git.Commitish, commit git.Commit, exists bool, records []byte) {
	mock.InOrder(
		branchGit.EXPECT().ValidateBranch(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(nil).Once(),
		branchGit.EXPECT().ResolveCommit(mock.Anything, git.Directory(project.InvocationWorktree), startPoint).Return(commit, nil).Once(),
		branchGit.EXPECT().LocalBranchExists(mock.Anything, git.Directory(project.ProjectRoot), branch).Return(exists, nil).Once(),
		branchGit.EXPECT().ListWorktrees(mock.Anything, git.Directory(project.ProjectRoot)).Return(records, nil).Once(),
	)
}

func expectTabLaunch(tabs *mocks.MockTabLauncher, project branchProject, branch git.Branch, path string, action zellij.Action) {
	title := zellij.TabTitle(string(project.Key) + ":" + string(branch))
	tabs.EXPECT().Launch(mock.Anything, zellij.Input{Title: title, Worktree: zellij.WorktreePath(path)}).Return(zellij.Result{Action: action, Title: title, Worktree: zellij.WorktreePath(path)}, nil).Once()
}

func availableWorktrees(project branchProject) []byte {
	return primaryWorktrees(project, "main")
}

func primaryWorktrees(project branchProject, branch git.Branch) []byte {
	return porcelainRecord(string(project.ProjectRoot), string(branch), false)
}

func managedWorktrees(project branchProject, branch git.Branch, path string, prunable bool) []byte {
	return append(primaryWorktrees(project, "main"), porcelainRecord(path, string(branch), prunable)...)
}

func unmanagedWorktrees(project branchProject, branch git.Branch) []byte {
	return append(primaryWorktrees(project, "main"), porcelainRecord(filepath.Join(string(project.ProjectRoot), "unmanaged"), string(branch), false)...)
}

func porcelainRecord(path string, branch string, prunable bool) []byte {
	record := []byte("worktree " + path + "\x00HEAD " + testOID + "\x00branch refs/heads/" + branch + "\x00")
	if prunable {
		record = append(record, []byte("prunable stale\x00")...)
	}
	return append(record, 0)
}
