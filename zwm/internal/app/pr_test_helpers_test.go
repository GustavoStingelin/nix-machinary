package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

type fakePullRequestGit struct {
	branchExists bool
	records      []worktree.Record
	head         worktree.OID
	err          error
	addCalls     []worktree.Path
}

func (git *fakePullRequestGit) LocalBranchExists(_ context.Context, _ project.Directory, _ worktree.Branch) (bool, error) {
	return git.branchExists, git.err
}

func (git *fakePullRequestGit) ListWorktrees(_ context.Context, _ project.Directory) ([]worktree.Record, error) {
	return append([]worktree.Record(nil), git.records...), git.err
}

func (git *fakePullRequestGit) ResolveHead(_ context.Context, _ project.Directory) (worktree.OID, error) {
	return git.head, git.err
}

func (git *fakePullRequestGit) AddDetached(_ context.Context, request app.DetachedWorktreeRequest) error {
	git.addCalls = append(git.addCalls, request.Path)
	git.records = append(git.records, worktree.Record{Path: request.Path, Head: git.head, State: worktree.HeadDetached})
	return git.err
}

func (git *fakePullRequestGit) register(path worktree.Path, branch worktree.Branch) {
	git.branchExists = true
	for index := range git.records {
		if git.records[index].Path == path {
			git.records[index].Branch = worktree.LocalRef(branch)
			git.records[index].State = worktree.HeadBranch
		}
	}
}

type fakePullRequestGateway struct {
	pullRequest      github.PullRequest
	resolveError     error
	checkoutError    error
	resolveSelectors []github.PullRequestSelector
	checkoutBranches []github.Branch
	checkoutForces   []bool
	onCheckout       func(github.Branch)
}

func (gateway *fakePullRequestGateway) ResolvePullRequest(_ context.Context, _ github.Directory, selector github.PullRequestSelector) (github.PullRequest, error) {
	gateway.resolveSelectors = append(gateway.resolveSelectors, selector)
	return gateway.pullRequest, gateway.resolveError
}

func (gateway *fakePullRequestGateway) CheckoutPullRequest(_ context.Context, request github.CheckoutRequest) error {
	gateway.checkoutBranches = append(gateway.checkoutBranches, request.Branch)
	gateway.checkoutForces = append(gateway.checkoutForces, request.Force)
	if gateway.onCheckout != nil {
		gateway.onCheckout(request.Branch)
	}
	return gateway.checkoutError
}

func pullRequestProject(t *testing.T) project.Resolution {
	t.Helper()

	root := t.TempDir()
	managedRoot := filepath.Join(root, ".wt", "project")
	return project.Resolution{
		InvocationWorktree: project.Directory(filepath.Join(root, "source")),
		ProjectRoot:        project.Directory(filepath.Join(root, "project")),
		ManagedRoot:        project.Directory(managedRoot),
	}
}

func initialRecords(projectResolution project.Resolution) []worktree.Record {
	return []worktree.Record{{
		Path:   worktree.Path(projectResolution.ProjectRoot),
		Head:   worktree.OID("0123456789abcdef0123456789abcdef01234567"),
		Branch: worktree.LocalRef("main"),
		State:  worktree.HeadBranch,
	}}
}

func expectedBranch(projectResolution project.Resolution, pullRequest github.PullRequest) worktree.Branch {
	identity := "project:" + decimalLength(string(projectResolution.ProjectRoot)) + ":" + string(projectResolution.ProjectRoot) + "\n" +
		"number:" + decimalLength(string(pullRequest.Number)) + ":" + string(pullRequest.Number) + "\n" +
		"head:" + decimalLength(string(pullRequest.HeadRefName)) + ":" + string(pullRequest.HeadRefName) + "\n"
	sum := sha256.Sum256([]byte(identity))
	return worktree.Branch("zwm/pr-" + string(pullRequest.Number) + "-" + hex.EncodeToString(sum[:])[:8])
}

func expectedPath(projectResolution project.Resolution, pullRequest github.PullRequest) worktree.Path {
	identity := "pr-" + string(pullRequest.Number)
	return worktree.ManagedWorktreePath(worktree.Path(projectResolution.ManagedRoot), identity)
}

func decimalLength(value string) string {
	return strconv.Itoa(len(value))
}

func ensureManagedRoot(t *testing.T, projectResolution project.Resolution) {
	t.Helper()
	require.NoError(t, os.MkdirAll(string(projectResolution.ManagedRoot), 0o700))
}
