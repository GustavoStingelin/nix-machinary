package app_test

import (
	"context"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestPullRequestService_realGit_rejects_new_namespace_collision_without_mutation(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	t.Setenv("GH_VIEW_STDOUT", "124\tfeature/collision\n")
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("124"), HeadRefName: github.HeadRefName("feature/collision")}
	runGit(t, fixture.primary, "branch", string(expectedBranch(fixture.project, pullRequest)))
	before := snapshot(t, fixture)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("124")})

	// Then
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	after := snapshot(t, fixture)
	require.Equal(t, before.refs, after.refs)
	require.Equal(t, before.worktrees, after.worktrees)
	require.Equal(t, before.sourceFile, after.sourceFile)
	require.Equal(t, before.status, after.status)
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "124", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}

func TestPullRequestService_realGit_rejects_malformed_metadata_before_mutation(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	t.Setenv("GH_VIEW_STDOUT", "789\tfeature/malformed\nunexpected\n")
	before := snapshot(t, fixture)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("789")})

	// Then
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	after := snapshot(t, fixture)
	require.Equal(t, before.refs, after.refs)
	require.Equal(t, before.worktrees, after.worktrees)
	require.Equal(t, before.sourceFile, after.sourceFile)
	require.Equal(t, before.status, after.status)
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "789", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}
