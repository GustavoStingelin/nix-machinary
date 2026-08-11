package command

import (
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/stretchr/testify/require"
)

func TestReviewPrompt_uses_the_pull_requests_own_base_not_the_default_branch(t *testing.T) {
	// A stacked pull request: #1305 merges into itests/watch-only-create, so the
	// review range is that branch — diffing against master would present every
	// commit from the branches underneath as part of this review.
	refs := github.PullRequestRefs{
		Number:      "1305",
		BaseRefName: "itests/watch-only-create",
		HeadRefName: "task-watch-policy",
		HeadOid:     "a08a506b935de6056782ef5d4d98f488dbb3f1c5",
	}

	prompt := reviewPrompt("btcsuite/btcwallet", "1305", refs)

	require.Contains(t, prompt, "#1305")
	require.Contains(t, prompt, "btcsuite/btcwallet")
	require.Contains(t, prompt, "itests/watch-only-create...task-watch-policy")
	require.Contains(t, prompt, "origin/itests/watch-only-create...HEAD")
	require.NotContains(t, prompt, "master")
}

func TestManagedPullRequestPath_matches_the_wpr_worktree_location(t *testing.T) {
	path := managedPullRequestPath("/Users/head", "btcwallet", "1303")

	require.Equal(t, filepath.Join("/Users/head", "code", ".wt", "btcwallet", "pr-1303"), string(path))
}
