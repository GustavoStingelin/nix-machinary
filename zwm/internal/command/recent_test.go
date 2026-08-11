package command

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/tui"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/stretchr/testify/require"
)

// resolutionFor builds the project identity the resolver would produce for a
// project rooted under a fake code root.
func resolutionFor(root string) project.Resolution {
	return project.Resolution{
		Key:         project.Key("btcwallet"),
		ProjectRoot: project.Directory(filepath.Join(root, "btcwallet")),
		ManagedRoot: project.Directory(filepath.Join(root, ".wt", "btcwallet")),
	}
}

func branchRecord(path, branch string) worktree.Record {
	return worktree.Record{
		Path:   worktree.Path(path),
		Head:   worktree.OID("0123456789abcdef0123456789abcdef01234567"),
		Branch: worktree.LocalRef(worktree.Branch(branch)),
		State:  worktree.HeadBranch,
	}
}

// fixedTouch answers with a per-path time, and the zero time for anything else,
// standing in for a stat that failed.
func fixedTouch(times map[string]time.Time) func(string) time.Time {
	return func(path string) time.Time { return times[path] }
}

func TestRecentViews_keeps_managed_worktrees_and_titles_them_like_wco(t *testing.T) {
	root := "/code"
	resolution := resolutionFor(root)
	managed := filepath.Join(root, ".wt", "btcwallet", "itests-accounts")

	views := recentViews(resolution, []worktree.Record{
		// The primary worktree: its tab is the bare project name, reopened by `o`.
		branchRecord(filepath.Join(root, "btcwallet"), "main"),
		branchRecord(managed, "itests/accounts"),
	}, fixedTouch(nil))

	require.Len(t, views, 1, "only the managed worktree belongs in the list")
	require.Equal(t, "btcwallet:itests/accounts", views[0].Title)
	require.Equal(t, "itests/accounts", views[0].Branch, "the branch, not the flattened directory name")
	require.Equal(t, managed, views[0].Worktree)
	require.False(t, views[0].IsPullRequest)
}

// The directory name is a flattened display of the branch, so it cannot be used
// to recover it — this is why the list comes from Git rather than a readdir.
func TestRecentViews_recovers_a_branch_the_directory_name_cannot_express(t *testing.T) {
	root := "/code"
	managed := filepath.Join(root, ".wt", "btcwallet", worktree.ManagedDisplay("itests/accounts"))
	require.Equal(t, "itests-accounts", filepath.Base(managed))

	views := recentViews(resolutionFor(root), []worktree.Record{branchRecord(managed, "itests/accounts")}, fixedTouch(nil))

	require.Equal(t, "itests/accounts", views[0].Branch)
}

func TestRecentViews_maps_a_pull_request_worktree_to_its_number_and_tab(t *testing.T) {
	root := "/code"
	managed := filepath.Join(root, ".wt", "btcwallet", "zwm-pr-1313-abc123")

	views := recentViews(resolutionFor(root), []worktree.Record{
		branchRecord(managed, "zwm/pr-1313-abc123def"),
	}, fixedTouch(nil))

	require.Len(t, views, 1)
	require.True(t, views[0].IsPullRequest)
	require.Equal(t, "1313", views[0].PullRequest)
	require.Equal(t, "btcwallet:pr-1313", views[0].Title, "the tab title wpr gives it, not the raw branch")
	require.Empty(t, views[0].Branch, "a pull request row reopens by number, not by branch")
}

func TestRecentViews_skips_detached_bare_and_prunable_worktrees(t *testing.T) {
	root := "/code"
	managedRoot := filepath.Join(root, ".wt", "btcwallet")

	detached := worktree.Record{Path: worktree.Path(filepath.Join(managedRoot, "detached")), State: worktree.HeadDetached}
	bare := worktree.Record{Path: worktree.Path(filepath.Join(managedRoot, "bare")), State: worktree.HeadBare}
	prunable := branchRecord(filepath.Join(managedRoot, "gone"), "gone")
	prunable.Prunable = true

	views := recentViews(resolutionFor(root), []worktree.Record{detached, bare, prunable}, fixedTouch(nil))

	require.Empty(t, views)
}

// A worktree elsewhere on disk is the user's business, not zwm's: it has no
// managed tab title and reopening it is not a thing this dashboard can do.
func TestRecentViews_ignores_worktrees_outside_the_managed_root(t *testing.T) {
	root := "/code"
	outside := branchRecord(filepath.Join(root, "elsewhere", "feature"), "feature")
	// A sibling directory sharing the managed root's prefix must not count either.
	sibling := branchRecord(filepath.Join(root, ".wt", "btcwallet-scratch", "feature"), "feature")

	views := recentViews(resolutionFor(root), []worktree.Record{outside, sibling}, fixedTouch(nil))

	require.Empty(t, views)
}

func TestSortRecent_orders_most_recently_touched_first(t *testing.T) {
	now := time.Now()
	list := []tui.RecentView{
		{Title: "p:unknown-b"},
		{Title: "p:lastweek", TouchedAt: now.Add(-7 * 24 * time.Hour)},
		{Title: "p:unknown-a"},
		{Title: "p:yesterday", TouchedAt: now.Add(-24 * time.Hour)},
	}

	sortRecent(list)

	titles := make([]string, 0, len(list))
	for _, view := range list {
		titles = append(titles, view.Title)
	}
	require.Equal(t, []string{"p:yesterday", "p:lastweek", "p:unknown-a", "p:unknown-b"}, titles,
		"newest first, unreadable timestamps last, ties broken by title")
}
