package command

import (
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestPullRequestCandidates_orders_by_descending_number_with_aligned_author_column(t *testing.T) {
	// Given
	summaries := []github.PullRequestSummary{
		{Number: github.PullRequestNumber("12"), Title: "Fix the bug", Author: "alice"},
		{Number: github.PullRequestNumber("340"), Title: "Add the thing", Author: "bob"},
		{Number: github.PullRequestNumber("98"), Title: "Tidy up", Author: "carol"},
	}

	// When
	candidates := pullRequestCandidates(summaries)

	// Then: newest (highest number) first, author handles padded to a column.
	require.Equal(t, []string{
		"340:@bob    Add the thing",
		"98:@carol  Tidy up",
		"12:@alice  Fix the bug",
	}, candidates)
}

func TestPullRequestCandidates_marks_a_missing_author_handle(t *testing.T) {
	// Given
	summaries := []github.PullRequestSummary{
		{Number: github.PullRequestNumber("7"), Title: "No author", Author: ""},
	}

	// When
	candidates := pullRequestCandidates(summaries)

	// Then
	require.Equal(t, []string{"7:@?  No author"}, candidates)
}

func TestPullRequestCandidates_returns_nothing_for_no_pull_requests(t *testing.T) {
	require.Empty(t, pullRequestCandidates(nil))
}
