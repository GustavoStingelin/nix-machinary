package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

func TestPullRequestService_returns_usage_failure_before_git_mutation_when_metadata_resolution_fails(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	git := &fakePullRequestGit{head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	metadataFailure := &github.CommandError{Stderr: []byte("fake gh: pull request selector missing was not found")}
	gateway := &fakePullRequestGateway{resolveError: metadataFailure}
	service := app.NewPullRequestService(git, gateway)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("missing")})

	// Then
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	require.Empty(t, git.addCalls)
	require.Empty(t, gateway.checkoutBranches)
	var commandError *github.CommandError
	require.True(t, errors.As(err, &commandError))
	require.Equal(t, metadataFailure.Stderr, commandError.Stderr)
}

func TestCheckoutPR_creates_then_reuses_the_deterministic_managed_worktree_for_branch_url_and_number_selectors(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	ensureManagedRoot(t, projectResolution)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}
	git := &fakePullRequestGit{records: initialRecords(projectResolution), head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest}
	gateway.onCheckout = func(branch github.Branch) {
		git.register(git.addCalls[len(git.addCalls)-1], worktree.Branch(branch))
	}
	service := app.NewPullRequestService(git, gateway)

	// When
	created, createError := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("feature/pr-ready")})
	byURL, urlError := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("https://github.com/example/project/pull/123")})
	byNumber, numberError := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("123")})

	// Then
	require.NoError(t, createError)
	require.NoError(t, urlError)
	require.NoError(t, numberError)
	require.Equal(t, app.PullRequestCreated, created.Action)
	require.Equal(t, app.PullRequestReused, byURL.Action)
	require.Equal(t, app.PullRequestReused, byNumber.Action)
	require.Equal(t, github.PullRequestNumber("123"), created.Number)
	require.Equal(t, github.PullRequestNumber("123"), byURL.Number)
	require.Equal(t, github.PullRequestNumber("123"), byNumber.Number)
	require.Equal(t, worktree.Branch(expectedBranch(projectResolution, pullRequest)), created.Branch)
	require.Equal(t, expectedPath(projectResolution, pullRequest), created.Worktree)
	require.Equal(t, "pr-123-feature/pr-ready", created.Display)
	require.Equal(t, []worktree.Path{expectedPath(projectResolution, pullRequest)}, git.addCalls)
	require.Equal(t, []github.Branch{github.Branch(expectedBranch(projectResolution, pullRequest))}, gateway.checkoutBranches)
	require.Equal(t, []github.PullRequestSelector{"feature/pr-ready", "https://github.com/example/project/pull/123", "123"}, gateway.resolveSelectors)
}

func TestPullRequestService_preserves_unambiguous_identity_for_fork_metadata(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	ensureManagedRoot(t, projectResolution)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("77"), HeadRefName: github.HeadRefName("fork/topic")}
	git := &fakePullRequestGit{records: initialRecords(projectResolution), head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest}
	gateway.onCheckout = func(branch github.Branch) {
		git.register(git.addCalls[len(git.addCalls)-1], worktree.Branch(branch))
	}
	service := app.NewPullRequestService(git, gateway)

	// When
	result, err := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("fork/topic")})

	// Then
	require.NoError(t, err)
	require.Equal(t, github.PullRequestNumber("77"), result.Number)
	require.Equal(t, "pr-77-fork/topic", result.Display)
	require.Equal(t, expectedBranch(projectResolution, pullRequest), result.Branch)
	require.Equal(t, expectedPath(projectResolution, pullRequest), result.Worktree)
}

func TestPullRequestService_rejects_a_new_namespace_collision_without_add_or_checkout(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("124"), HeadRefName: github.HeadRefName("feature/collision")}
	git := &fakePullRequestGit{branchExists: true, records: initialRecords(projectResolution)}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest}
	service := app.NewPullRequestService(git, gateway)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("124")})

	// Then
	require.Equal(t, errs.Usage, errs.ClassOf(err))
	require.Empty(t, git.addCalls)
	require.Empty(t, gateway.checkoutBranches)
}

func TestCheckoutPR_preserves_checkout_stderr_and_detached_leftover_before_recovery(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	ensureManagedRoot(t, projectResolution)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("456"), HeadRefName: github.HeadRefName("feature/failure")}
	git := &fakePullRequestGit{records: initialRecords(projectResolution), head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	checkoutFailure := &github.CommandError{Stderr: []byte("fake gh: injected checkout failure for 456")}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest, checkoutError: checkoutFailure}
	service := app.NewPullRequestService(git, gateway)
	input := app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("456")}

	// When
	_, firstError := service.Checkout(context.Background(), input)
	git.addCalls = nil
	_, repeatError := service.Checkout(context.Background(), input)

	// Then
	var firstFailure *app.PullRequestError
	require.ErrorAs(t, firstError, &firstFailure)
	require.Equal(t, expectedPath(projectResolution, pullRequest), firstFailure.Recovery.DetachedWorktree)
	var commandError *github.CommandError
	require.True(t, errors.As(firstError, &commandError))
	require.Equal(t, checkoutFailure.Stderr, commandError.Stderr)
	var repeatFailure *app.PullRequestError
	require.ErrorAs(t, repeatError, &repeatFailure)
	require.Equal(t, expectedPath(projectResolution, pullRequest), repeatFailure.Recovery.DetachedWorktree)
	require.Empty(t, git.addCalls)
	require.Len(t, gateway.checkoutBranches, 1)
}

func TestCheckoutPR_preserves_detached_worktree_when_checkout_claims_success_without_registering_branch(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	ensureManagedRoot(t, projectResolution)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("321"), HeadRefName: github.HeadRefName("feature/false-success")}
	git := &fakePullRequestGit{records: initialRecords(projectResolution), head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest}
	service := app.NewPullRequestService(git, gateway)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("321")})

	// Then
	var failure *app.PullRequestError
	require.ErrorAs(t, err, &failure)
	require.Equal(t, expectedPath(projectResolution, pullRequest), failure.Recovery.DetachedWorktree)
	require.Len(t, git.addCalls, 1)
	require.Len(t, gateway.checkoutBranches, 1)
}

func TestCheckoutPR_returns_external_recovery_failure_when_success_loses_exact_registration(t *testing.T) {
	// Given
	projectResolution := pullRequestProject(t)
	ensureManagedRoot(t, projectResolution)
	pullRequest := github.PullRequest{Number: github.PullRequestNumber("900"), HeadRefName: github.HeadRefName("feature/disappears")}
	git := &fakePullRequestGit{records: initialRecords(projectResolution), head: worktree.OID("0123456789abcdef0123456789abcdef01234567")}
	gateway := &fakePullRequestGateway{pullRequest: pullRequest}
	gateway.onCheckout = func(github.Branch) {
		git.records = initialRecords(projectResolution)
	}
	service := app.NewPullRequestService(git, gateway)

	// When
	_, err := service.Checkout(context.Background(), app.PullRequestInput{Project: projectResolution, Selector: github.PullRequestSelector("900")})

	// Then
	require.Error(t, err)
	require.Equal(t, errs.External, errs.ClassOf(err))
	var failure *app.PullRequestError
	require.ErrorAs(t, err, &failure)
	require.NotNil(t, failure.Recovery)
	require.Equal(t, expectedPath(projectResolution, pullRequest), failure.Recovery.DetachedWorktree)
	var invalidTarget *worktree.InvalidTargetError
	require.ErrorAs(t, err, &invalidTarget)
	require.Equal(t, worktree.RegistrationAvailable, invalidTarget.Validation.Registration)
	require.Equal(t, worktree.BranchUnregistered, invalidTarget.Validation.Branch)
	require.Len(t, git.addCalls, 1)
	require.Len(t, gateway.checkoutBranches, 1)
}
