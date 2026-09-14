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

// primaryRecord is the entry Git always lists first. The porcelain output
// carries no other marker for the primary worktree, so position is how the
// mapping recognises it.
func primaryRecord(root string) worktree.Record {
	return branchRecord(filepath.Join(root, "btcwallet"), "main")
}

func TestWorktreeViews_titles_a_managed_worktree_like_wco(t *testing.T) {
	root := "/code"
	managed := filepath.Join(root, ".wt", "btcwallet", "itests-accounts")

	views := worktreeViews(resolutionFor(root), []worktree.Record{
		primaryRecord(root),
		branchRecord(managed, "itests/accounts"),
	}, fixedTouch(nil))

	require.Len(t, views, 2)
	require.Equal(t, "btcwallet:itests/accounts", views[1].Title)
	require.Equal(t, "itests/accounts", views[1].Branch, "the branch, not the flattened directory name")
	require.Equal(t, managed, views[1].Worktree)
	require.Equal(t, tui.WorktreeManaged, views[1].Kind)
}

// The primary worktree is listed too, but it opens by project rather than by
// branch — and its branch changes under you, so carrying one would be a lie.
func TestWorktreeViews_lists_the_primary_worktree_as_the_project_itself(t *testing.T) {
	root := "/code"

	views := worktreeViews(resolutionFor(root), []worktree.Record{primaryRecord(root)}, fixedTouch(nil))

	require.Len(t, views, 1)
	require.Equal(t, tui.WorktreePrimary, views[0].Kind)
	require.Equal(t, "btcwallet", views[0].Title)
	require.Equal(t, filepath.Join(root, "btcwallet"), views[0].Worktree)
	require.Empty(t, views[0].Branch)
}

// The directory name is a flattened display of the branch, so it cannot be used
// to recover it — this is why the list comes from Git rather than a readdir.
func TestWorktreeViews_recovers_a_branch_the_directory_name_cannot_express(t *testing.T) {
	root := "/code"
	managed := filepath.Join(root, ".wt", "btcwallet", worktree.ManagedDisplay("itests/accounts"))
	require.Equal(t, "itests-accounts", filepath.Base(managed))

	views := worktreeViews(resolutionFor(root), []worktree.Record{
		primaryRecord(root),
		branchRecord(managed, "itests/accounts"),
	}, fixedTouch(nil))

	require.Equal(t, "itests/accounts", views[1].Branch)
}

func TestWorktreeViews_maps_a_pull_request_worktree_to_its_number_and_tab(t *testing.T) {
	root := "/code"
	managed := filepath.Join(root, ".wt", "btcwallet", "zwm-pr-1313-abc123")

	views := worktreeViews(resolutionFor(root), []worktree.Record{
		primaryRecord(root),
		branchRecord(managed, "zwm/pr-1313-abc123def"),
	}, fixedTouch(nil))

	require.Len(t, views, 2)
	require.Equal(t, tui.WorktreePullRequest, views[1].Kind)
	require.Equal(t, "1313", views[1].PullRequest)
	require.Equal(t, "btcwallet:pr-1313", views[1].Title, "the tab title wpr gives it, not the raw branch")
	// Reopening goes by number, but deleting needs the real branch, so the row
	// carries both.
	require.Equal(t, "zwm/pr-1313-abc123def", views[1].Branch)
}

// A linked worktree with no branch has no tab title to restore and no command
// that reopens it, and a prunable one is already gone.
func TestWorktreeViews_skips_detached_bare_and_prunable_worktrees(t *testing.T) {
	root := "/code"
	managedRoot := filepath.Join(root, ".wt", "btcwallet")

	detached := worktree.Record{Path: worktree.Path(filepath.Join(managedRoot, "detached")), State: worktree.HeadDetached}
	bare := worktree.Record{Path: worktree.Path(filepath.Join(managedRoot, "bare")), State: worktree.HeadBare}
	prunable := branchRecord(filepath.Join(managedRoot, "gone"), "gone")
	prunable.Prunable = true

	views := worktreeViews(resolutionFor(root), []worktree.Record{
		primaryRecord(root), detached, bare, prunable,
	}, fixedTouch(nil))

	require.Len(t, views, 1, "only the primary survives")
	require.Equal(t, tui.WorktreePrimary, views[0].Kind)
}

// Worktrees made outside the managed root are listed — they are the ones that
// accumulate unnoticed — but marked external, because no wco/wpr reopens them.
func TestWorktreeViews_marks_worktrees_outside_the_managed_root_as_external(t *testing.T) {
	root := "/code"
	outside := branchRecord(filepath.Join(root, "elsewhere", "feature"), "feature")
	// A sibling directory sharing the managed root's prefix is not inside it.
	sibling := branchRecord(filepath.Join(root, ".wt", "btcwallet-scratch", "scratch"), "scratch")

	views := worktreeViews(resolutionFor(root), []worktree.Record{primaryRecord(root), outside, sibling}, fixedTouch(nil))

	require.Len(t, views, 3)
	require.Equal(t, tui.WorktreeExternal, views[1].Kind)
	require.Equal(t, "btcwallet:feature", views[1].Title)
	require.Equal(t, tui.WorktreeExternal, views[2].Kind)
}

// A pull-request-shaped branch checked out somewhere zwm did not put it is not
// a wpr worktree, and `wpr` would not reopen it where it is.
func TestWorktreeViews_does_not_treat_an_external_pr_branch_as_a_pull_request(t *testing.T) {
	root := "/code"
	outside := branchRecord(filepath.Join(root, "elsewhere", "pr"), "zwm/pr-1313-abc123def")

	views := worktreeViews(resolutionFor(root), []worktree.Record{primaryRecord(root), outside}, fixedTouch(nil))

	require.Equal(t, tui.WorktreeExternal, views[1].Kind)
	require.Empty(t, views[1].PullRequest)
}

// The pane renders a heading per project, so project is the primary key: a
// global recency order would interleave them.
func TestSortWorktrees_groups_by_project_then_root_then_recency(t *testing.T) {
	now := time.Now()
	list := []tui.WorktreeView{
		{Project: "lnd", Title: "lnd:unknown-b"},
		{Project: "btcwallet", Title: "btcwallet:lastweek", TouchedAt: now.Add(-7 * 24 * time.Hour)},
		{Project: "lnd", Title: "lnd:unknown-a"},
		{Project: "btcwallet", Title: "btcwallet:yesterday", TouchedAt: now.Add(-24 * time.Hour)},
		// The root sorts to the top of its project however long ago it was touched.
		{Project: "btcwallet", Title: "btcwallet", Kind: tui.WorktreePrimary, TouchedAt: now.Add(-30 * 24 * time.Hour)},
	}

	sortWorktrees(list)

	titles := make([]string, 0, len(list))
	for _, view := range list {
		titles = append(titles, view.Title)
	}
	require.Equal(t, []string{
		"btcwallet", "btcwallet:yesterday", "btcwallet:lastweek",
		"lnd:unknown-a", "lnd:unknown-b",
	}, titles, "by project, root first, then newest first, unreadable timestamps last, ties by title")
}
