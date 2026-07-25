package github_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestClient_ResolvePullRequest_uses_exact_view_argv_and_invocation_worktree(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	t.Setenv("GH_VIEW_STDOUT", "123\tfeature/pr-ready\n")
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	pullRequest, err := client.ResolvePullRequest(context.Background(), github.Directory(directory), github.PullRequestSelector("feature/pr-ready"))

	// Then
	require.NoError(t, err)
	require.Equal(t, github.PullRequest{Number: github.PullRequestNumber("123"), HeadRefName: github.HeadRefName("feature/pr-ready")}, pullRequest)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{
			"pr",
			"view",
			"feature/pr-ready",
			"--json",
			"number,headRefName",
			"--jq",
			"[.number, .headRefName] | @tsv",
		},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_ResolvePullRequest_rejects_malformed_metadata_before_any_mutation(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "empty output", output: ""},
		{name: "empty number", output: "\tfeature/topic\n"},
		{name: "zero number", output: "0\tfeature/topic\n"},
		{name: "empty head", output: "123\t\n"},
		{name: "multiple tabs", output: "123\tfeature\textra\n"},
		{name: "multiple records", output: "123\tfeature\n124\tother\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			helper, _ := fakeGH(t)
			t.Setenv("GH_VIEW_STDOUT", test.output)
			client := github.NewClient(github.Config{Executable: helper})

			// When
			_, err := client.ResolvePullRequest(context.Background(), github.Directory(t.TempDir()), github.PullRequestSelector("123"))

			// Then
			require.ErrorIs(t, err, github.ErrMalformedMetadata)
		})
	}
}

func TestClient_ResolvePullRequest_retains_raw_stderr_when_the_command_fails(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_VIEW_EXIT", "4")
	t.Setenv("GH_VIEW_STDOUT", "stdout before failure")
	t.Setenv("GH_VIEW_STDERR", "fake gh: pull request selector missing was not found")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	_, err := client.ResolvePullRequest(context.Background(), github.Directory(t.TempDir()), github.PullRequestSelector("missing"))

	// Then
	var commandError *github.CommandError
	require.ErrorAs(t, err, &commandError)
	require.Equal(t, []byte("stdout before failure"), commandError.Stdout)
	require.Equal(t, []byte("fake gh: pull request selector missing was not found"), commandError.Stderr)
	var exitError *exec.ExitError
	require.True(t, errors.As(err, &exitError))
}

func TestClient_CheckoutPullRequest_uses_exact_checkout_argv_and_worktree(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	err := client.CheckoutPullRequest(context.Background(), github.CheckoutRequest{
		Directory: github.Directory(directory),
		Selector:  github.PullRequestSelector("https://github.com/example/project/pull/123"),
		Branch:    github.Branch("zwm/pr-123-deadbeef"),
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{
			"pr",
			"checkout",
			"https://github.com/example/project/pull/123",
			"--branch",
			"zwm/pr-123-deadbeef",
		},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_CheckoutPullRequest_retains_raw_stderr_when_the_command_fails(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_CHECKOUT_EXIT", "75")
	t.Setenv("GH_CHECKOUT_STDERR", "fake gh: injected checkout failure for 456")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	err := client.CheckoutPullRequest(context.Background(), github.CheckoutRequest{
		Directory: github.Directory(t.TempDir()),
		Selector:  github.PullRequestSelector("456"),
		Branch:    github.Branch("zwm/pr-456-deadbeef"),
	})

	// Then
	var commandError *github.CommandError
	require.ErrorAs(t, err, &commandError)
	require.Equal(t, []byte("fake gh: injected checkout failure for 456"), commandError.Stderr)
}

func TestClient_ListOpenPullRequests_uses_exact_list_argv_and_parses_summaries(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	t.Setenv("GH_LIST_STDOUT", "12\tFix the bug\n34\tAdd the thing\n")
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	summaries, err := client.ListOpenPullRequests(context.Background(), github.Directory(directory))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.PullRequestSummary{
		{Number: github.PullRequestNumber("12"), Title: "Fix the bug"},
		{Number: github.PullRequestNumber("34"), Title: "Add the thing"},
	}, summaries)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{"pr", "list", "--state", "open", "--json", "number,title", "--jq", ".[] | [.number, .title] | @tsv"},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_ListOpenPullRequests_skips_malformed_lines(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_LIST_STDOUT", "12\tGood\nnot-a-number\ttitle\n\n34\tAlso good\n")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	summaries, err := client.ListOpenPullRequests(context.Background(), github.Directory(t.TempDir()))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.PullRequestSummary{
		{Number: github.PullRequestNumber("12"), Title: "Good"},
		{Number: github.PullRequestNumber("34"), Title: "Also good"},
	}, summaries)
}
