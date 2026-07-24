package app_test

import (
	"context"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestPullRequestService_realGit_rejects_mismatched_registered_target_without_mutation(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}
	mismatchedBranch := "unrelated/pr-123"
	mismatchedPath := string(expectedPath(fixture.project, pullRequest))
	runGit(t, fixture.primary, "branch", mismatchedBranch)
	runGit(t, fixture.primary, "worktree", "add", "--quiet", mismatchedPath, mismatchedBranch)
	before := snapshot(t, fixture)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("123")})

	// Then
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	after := snapshot(t, fixture)
	require.Equal(t, before.refs, after.refs)
	require.Equal(t, before.worktrees, after.worktrees)
	require.Equal(t, before.sourceFile, after.sourceFile)
	require.Equal(t, before.status, after.status)
	require.Equal(t, mismatchedBranch, symbolicHead(t, mismatchedPath))
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "123", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}
