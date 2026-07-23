package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestLegacyZWTPrefix_realGit_does_not_adopt_or_mutate_a_legacy_worktree_at_the_new_target_path(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}
	legacyBranch := strings.Replace(string(expectedBranch(fixture.project, pullRequest)), "zwm/", "zwt/", 1)
	legacyPath := string(expectedPath(fixture.project, pullRequest))
	runGit(t, fixture.primary, "branch", legacyBranch)
	runGit(t, fixture.primary, "worktree", "add", "--quiet", legacyPath, legacyBranch)
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
	require.Equal(t, legacyBranch, symbolicHead(t, legacyPath))
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "123", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
	}, githubInvocations(t, fixture.ghRecord))
}

func TestLegacyZWTPrefix_realGit_ignores_legacy_local_prefix_collision_without_mutating_the_legacy_ref(t *testing.T) {
	// Given
	fixture := newRealPullRequestFixture(t)
	service := realPullRequestService(fixture)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}
	legacyBranch := strings.Replace(string(expectedBranch(fixture.project, pullRequest)), "zwm/", "zwt/", 1)
	runGit(t, fixture.primary, "branch", legacyBranch)
	legacyBefore := runGit(t, fixture.primary, "rev-parse", legacyBranch)

	// When
	result, err := service.Checkout(context.Background(), app.PullRequestInput{Project: fixture.project, Selector: github.PullRequestSelector("123")})

	// Then
	require.NoError(t, err)
	require.Equal(t, app.PullRequestCreated, result.Action)
	require.Equal(t, legacyBefore, runGit(t, fixture.primary, "rev-parse", legacyBranch))
	require.Equal(t, []githubInvocation{
		{Directory: fixture.source, Arguments: []string{"pr", "view", "123", "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}},
		{Directory: string(result.Worktree), Arguments: []string{"pr", "checkout", "123", "--branch", string(result.Branch)}},
	}, githubInvocations(t, fixture.ghRecord))
}
