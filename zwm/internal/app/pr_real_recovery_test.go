package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestCheckoutPR_realGit_preserves_detached_worktree_and_raw_checkout_stderr_for_recovery(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	t.Setenv("GH_CHECKOUT_EXIT", "75")
	t.Setenv("GH_CHECKOUT_STDERR", "fake gh: injected checkout failure for 456")
	t.Setenv("GH_VIEW_STDOUT", "456\tfeature/failure\n")
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("456"), HeadRefName: github.HeadRefName("feature/failure")}
	before := snapshot(t, fixture)
	input := app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("456")}

	// When
	_, firstError := service.Checkout(context.Background(), input)
	afterFailure := snapshot(t, fixture)
	clearGitHubInvocations(t, fixture.ghRecord)
	_, repeatError := service.Checkout(context.Background(), input)
	afterRepeat := snapshot(t, fixture)

	// Then
	var firstFailure *app.PullRequestError
	require.ErrorAs(t, firstError, &firstFailure)
	require.Equal(t, expectedPath(fixture.project, pullRequest), firstFailure.Recovery.DetachedWorktree)
	var commandError *github.CommandError
	require.True(t, errors.As(firstError, &commandError))
	require.Equal(t, []byte("fake gh: injected checkout failure for 456"), commandError.Stderr)
	require.Equal(t, before.refs, afterFailure.refs)
	require.NotEqual(t, before.worktrees, afterFailure.worktrees)
	require.Equal(t, before.sourceFile, afterFailure.sourceFile)
	require.Equal(t, before.status, afterFailure.status)
	require.DirExists(t, string(firstFailure.Recovery.DetachedWorktree))
	detachedHead(t, string(firstFailure.Recovery.DetachedWorktree))

	var repeatFailure *app.PullRequestError
	require.ErrorAs(t, repeatError, &repeatFailure)
	require.Equal(t, firstFailure.Recovery, repeatFailure.Recovery)
	require.Equal(t, afterFailure.refs, afterRepeat.refs)
	require.Equal(t, afterFailure.worktrees, afterRepeat.worktrees)
	require.Equal(t, afterFailure.sourceFile, afterRepeat.sourceFile)
	require.Equal(t, afterFailure.status, afterRepeat.status)
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "456", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}

func TestCheckoutPR_realGit_rejects_false_checkout_success_without_mutating_refs(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	t.Setenv("GH_CHECKOUT_MODE", "false-success")
	t.Setenv("GH_VIEW_STDOUT", "321\tfeature/false-success\n")
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("321"), HeadRefName: github.HeadRefName("feature/false-success")}
	before := snapshot(t, fixture)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("321")})

	// Then
	var failure *app.PullRequestError
	require.ErrorAs(t, err, &failure)
	require.Equal(t, expectedPath(fixture.project, pullRequest), failure.Recovery.DetachedWorktree)
	after := snapshot(t, fixture)
	require.Equal(t, before.refs, after.refs)
	require.NotEqual(t, before.worktrees, after.worktrees)
	require.Equal(t, before.sourceFile, after.sourceFile)
	require.Equal(t, before.status, after.status)
	require.DirExists(t, string(failure.Recovery.DetachedWorktree))
	detachedHead(t, string(failure.Recovery.DetachedWorktree))
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "321", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
		{Directory: string(failure.Recovery.DetachedWorktree), Arguments: []string{"pr", "checkout", "321", "--branch", string(expectedBranch(fixture.project, pullRequest))}},
	}, githubInvocations(t, fixture.ghRecord))
}
