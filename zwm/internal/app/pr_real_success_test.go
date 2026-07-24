package app_test

import (
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestCheckoutPR_realGit_creates_and_reuses_for_branch_url_and_number_selectors(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}
	before := snapshot(t, fixture)

	// When
	created, createError := service.Checkout(contextForTest(t), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("feature/pr-ready")})

	// Then
	require.NoError(t, createError)
	require.Equal(t, app.PullRequestCreated, created.Action)
	require.Equal(t, expectedBranch(fixture.project, pullRequest), created.Branch)
	require.Equal(t, expectedPath(fixture.project, pullRequest), created.Worktree)
	require.Equal(t, filepath.Join(string(fixture.project.ManagedRoot), "pr-123"), string(created.Worktree))
	require.Equal(t, "pr-123-feature/pr-ready", created.Display)
	require.Equal(t, string(created.Branch), symbolicHead(t, string(created.Worktree)))
	require.Equal(t, fixture.prCommit, string(runGit(t, string(created.Worktree), "rev-parse", "HEAD"))[:40])
	after := snapshot(t, fixture)
	require.Equal(t, before.sourceFile, after.sourceFile)
	require.Equal(t, before.status, after.status)
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "feature/pr-ready", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
		{Directory: string(created.Worktree), Arguments: []string{"pr", "checkout", "feature/pr-ready", "--branch", string(created.Branch)}},
	}, githubInvocations(t, fixture.ghRecord))

	clearGitHubInvocations(t, fixture.ghRecord)
	byURL, urlError := service.Checkout(contextForTest(t), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("https://github.com/example/project/pull/123")})
	byNumber, numberError := service.Checkout(contextForTest(t), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("123")})

	require.NoError(t, urlError)
	require.NoError(t, numberError)
	require.Equal(t, app.PullRequestReused, byURL.Action)
	require.Equal(t, app.PullRequestReused, byNumber.Action)
	require.Equal(t, created.Worktree, byURL.Worktree)
	require.Equal(t, created.Worktree, byNumber.Worktree)
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "https://github.com/example/project/pull/123", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
		{Directory: fixture.source, Arguments: []string{"pr", "view", "123", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}
