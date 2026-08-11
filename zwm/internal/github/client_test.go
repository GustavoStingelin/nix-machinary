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

func TestClient_CheckoutPullRequest_appends_force_flag_when_force_is_requested(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	err := client.CheckoutPullRequest(context.Background(), github.CheckoutRequest{
		Directory: github.Directory(directory),
		Selector:  github.PullRequestSelector("123"),
		Branch:    github.Branch("zwm/pr-123-deadbeef"),
		Force:     true,
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{
			"pr",
			"checkout",
			"123",
			"--branch",
			"zwm/pr-123-deadbeef",
			"--force",
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
	t.Setenv("GH_LIST_STDOUT", "12\tFix the bug\talice\n34\tAdd the thing\tbob\n")
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	summaries, err := client.ListOpenPullRequests(context.Background(), github.Directory(directory))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.PullRequestSummary{
		{Number: github.PullRequestNumber("12"), Title: "Fix the bug", Author: "alice"},
		{Number: github.PullRequestNumber("34"), Title: "Add the thing", Author: "bob"},
	}, summaries)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{"pr", "list", "--state", "open", "--json", "number,title,author", "--jq", ".[] | [.number, .title, .author.login] | @tsv"},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_ListOpenPullRequests_skips_lines_without_number_title_and_author(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_LIST_STDOUT", "12\tGood\talice\nnot-a-number\ttitle\tbob\n34\tMissing author\n\n56\tAlso good\tcarol\n")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	summaries, err := client.ListOpenPullRequests(context.Background(), github.Directory(t.TempDir()))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.PullRequestSummary{
		{Number: github.PullRequestNumber("12"), Title: "Good", Author: "alice"},
		{Number: github.PullRequestNumber("56"), Title: "Also good", Author: "carol"},
	}, summaries)
}

func TestClient_ListReviewRequests_uses_exact_search_argv_and_parses_rows(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	t.Setenv("GH_SEARCH_STDOUT", "1305\tbtcsuite/btcwallet\tyyforyongyu\twallet: define watch-only and script policy\n"+
		"3630\tlightninglabs/lightning-infra\tRoasbeef\tlumosd: raise CPU limits\n")
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	requests, err := client.ListReviewRequests(context.Background(), github.Directory(directory))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.ReviewRequest{
		{Number: "1305", Repository: "btcsuite/btcwallet", Author: "yyforyongyu", Title: "wallet: define watch-only and script policy"},
		{Number: "3630", Repository: "lightninglabs/lightning-infra", Author: "Roasbeef", Title: "lumosd: raise CPU limits"},
	}, requests)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{
			"search", "prs", "--review-requested", "@me", "--state", "open",
			"--limit", "50",
			"--json", "number,title,author,repository",
			"--jq", ".[] | [.number, .repository.nameWithOwner, .author.login, .title] | @tsv",
		},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_ListReviewRequests_keeps_titles_containing_tabs_intact(t *testing.T) {
	// Given a title with a tab: it is the last field, so it must not be split.
	helper, _ := fakeGH(t)
	t.Setenv("GH_SEARCH_STDOUT", "12\towner/repo\tauthor\ttitle\twith tab\n")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	requests, err := client.ListReviewRequests(context.Background(), github.Directory(t.TempDir()))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.ReviewRequest{
		{Number: "12", Repository: "owner/repo", Author: "author", Title: "title\twith tab"},
	}, requests)
}

func TestClient_ListReviewRequests_skips_rows_with_an_invalid_number_or_repository(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_SEARCH_STDOUT", "abc\towner/repo\ta\tbad number\n"+
		"0123\towner/repo\ta\tleading zero\n"+
		"9\t\ta\tno repository\n"+
		"7\towner/repo\ta\tkeeps this one\n")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	requests, err := client.ListReviewRequests(context.Background(), github.Directory(t.TempDir()))

	// Then
	require.NoError(t, err)
	require.Equal(t, []github.ReviewRequest{
		{Number: "7", Repository: "owner/repo", Author: "a", Title: "keeps this one"},
	}, requests)
}

func TestClient_ViewPullRequestRefs_targets_the_named_repository_and_returns_the_real_base(t *testing.T) {
	// Given a stacked pull request: its base is the branch below it, not master.
	helper, recordPath := fakeGH(t)
	t.Setenv("GH_VIEW_STDOUT", "1305\titests/watch-only-create\ttask-watch-policy\ta08a506b935de6056782ef5d4d98f488dbb3f1c5\n")
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	refs, err := client.ViewPullRequestRefs(context.Background(), github.Directory(directory), "btcsuite/btcwallet", "1305")

	// Then
	require.NoError(t, err)
	require.Equal(t, github.PullRequestRefs{
		Number:      "1305",
		BaseRefName: "itests/watch-only-create",
		HeadRefName: "task-watch-policy",
		HeadOid:     "a08a506b935de6056782ef5d4d98f488dbb3f1c5",
	}, refs)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{
			"pr", "view", "1305",
			"--repo", "btcsuite/btcwallet",
			"--json", "number,baseRefName,headRefName,headRefOid",
			"--jq", "[.number, .baseRefName, .headRefName, .headRefOid] | @tsv",
		},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_ViewPullRequestRefs_rejects_incomplete_metadata(t *testing.T) {
	for name, stdout := range map[string]string{
		"missing field": "1305\tbase\thead\n",
		"empty base":    "1305\t\thead\toid\n",
		"empty oid":     "1305\tbase\thead\t\n",
		"bad number":    "x\tbase\thead\toid\n",
		"no output":     "",
	} {
		t.Run(name, func(t *testing.T) {
			helper, _ := fakeGH(t)
			t.Setenv("GH_VIEW_STDOUT", stdout)
			client := github.NewClient(github.Config{Executable: helper})

			_, err := client.ViewPullRequestRefs(context.Background(), github.Directory(t.TempDir()), "owner/repo", "1305")

			// A half-read pull request must not produce a review prompt with a
			// wrong or empty base branch.
			require.Error(t, err)
		})
	}
}

func TestReviewRequest_RepositoryName_extracts_the_project_directory_name(t *testing.T) {
	require.Equal(t, "btcwallet", github.ReviewRequest{Repository: "btcsuite/btcwallet"}.RepositoryName())
	require.Equal(t, "bare", github.ReviewRequest{Repository: "bare"}.RepositoryName())
}

func TestClient_BrowsePullRequest_uses_exact_web_argv_for_the_named_repository(t *testing.T) {
	// Given
	helper, recordPath := fakeGH(t)
	directory := t.TempDir()
	client := github.NewClient(github.Config{Executable: helper})

	// When
	err := client.BrowsePullRequest(context.Background(), github.Directory(directory), "someone/not-cloned", "77")

	// Then --repo carries the repository, so this works with no local checkout.
	require.NoError(t, err)
	require.Equal(t, invocation{
		Directory: directory,
		Arguments: []string{"pr", "view", "77", "--repo", "someone/not-cloned", "--web"},
	}, readInvocations(t, recordPath)[0])
}

func TestClient_BrowsePullRequest_reports_a_failing_browser_launch(t *testing.T) {
	// Given
	helper, _ := fakeGH(t)
	t.Setenv("GH_VIEW_EXIT", "1")
	t.Setenv("GH_VIEW_STDERR", "failed to open browser\n")
	client := github.NewClient(github.Config{Executable: helper})

	// When
	err := client.BrowsePullRequest(context.Background(), github.Directory(t.TempDir()), "owner/repo", "1")

	// Then
	require.Error(t, err)
}
