package command

import (
	"context"
	"os"
	"sort"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/tui"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

// recentLimit caps the recent list. It is a way back into a tab you had open,
// not an archive: past a dozen rows it stops being faster than typing `wco`.
const recentLimit = 12

// Recent lists the managed worktrees zwm has created, most recently touched
// first, so a tab that has since been closed can be reopened from the dashboard.
//
// The list comes from Git rather than from a history file zwm would have to
// write, which means it works retroactively — every worktree already on disk is
// offered, including ones created long before this existed. Git is also the only
// source that knows each worktree's branch: the directory name is a flattened
// display of it (worktree.ManagedDisplay turns "itests/accounts" into
// "itests-accounts"), so the path alone cannot say what to check out.
//
// Failure to read one project is not failure to list the rest: a project whose
// Git call fails is skipped, because a dashboard section is worth showing
// partially filled.
func (source tuiSource) Recent(ctx context.Context) ([]tui.RecentView, error) {
	views := make([]tui.RecentView, 0)
	for _, name := range project.ListNames(project.Directory(source.home)) {
		resolution, err := source.projects.Resolve(ctx, project.Request{
			Home:    project.Directory(source.home),
			Project: project.Value(name),
			// Resolve by name, so the dashboard's own working directory — whichever
			// pane it was opened from — cannot influence which project this is.
			WorkingDirectory: project.Directory(source.home),
		})
		if err != nil {
			continue
		}
		raw, err := source.git.ListWorktrees(ctx, git.Directory(resolution.ProjectRoot))
		if err != nil {
			continue
		}
		records, err := worktree.ParsePorcelainZ(raw)
		if err != nil {
			continue
		}
		views = append(views, recentViews(resolution, records, modifiedAt)...)
	}
	sortRecent(views)
	if len(views) > recentLimit {
		views = views[:recentLimit]
	}
	return views, nil
}

// recentViews maps one project's worktree records to dashboard rows, keeping only
// the managed ones. touched reads a worktree's modification time; it is a
// parameter so the mapping is testable without a filesystem.
func recentViews(resolution project.Resolution, records []worktree.Record, touched func(string) time.Time) []tui.RecentView {
	views := make([]tui.RecentView, 0, len(records))
	for _, record := range records {
		// Only branch checkouts under this project's managed root: that excludes the
		// primary worktree (whose tab is the plain project name and which `o`
		// reopens), detached and bare entries, and any worktree the user made
		// elsewhere by hand.
		if record.State != worktree.HeadBranch || record.Prunable {
			continue
		}
		if !underManagedRoot(string(record.Path), string(resolution.ManagedRoot)) {
			continue
		}
		branch := localBranch(record.Branch)
		if branch == "" {
			continue
		}
		views = append(views, recentView(string(resolution.Key), branch, string(record.Path), touched(string(record.Path))))
	}
	return views
}

// recentView builds one row, reproducing the tab title its command would give it
// so the dashboard can tell whether that tab is already open.
//
// A pull-request worktree is the interesting case: `wpr` checks out a branch
// named "zwm/pr-<n>-<hash>" but titles the tab "<key>:pr-<n>", and reopening it
// has to go back through `wpr` — a plain `wco` of that branch would work but
// would title the tab after the raw branch, leaving two names for one worktree.
func recentView(key, branch, path string, touched time.Time) tui.RecentView {
	if number, ok := managedPRNumber(branch); ok {
		return tui.RecentView{
			Project:       key,
			Title:         key + ":pr-" + number,
			Worktree:      path,
			TouchedAt:     touched,
			PullRequest:   number,
			IsPullRequest: true,
		}
	}
	return tui.RecentView{
		Project:   key,
		Branch:    branch,
		Title:     key + ":" + branch,
		Worktree:  path,
		TouchedAt: touched,
	}
}

// sortRecent orders rows most recently touched first, with the title as the
// tie-break so the list is stable when timestamps match (or are all zero because
// every stat failed).
func sortRecent(views []tui.RecentView) {
	sort.SliceStable(views, func(left, right int) bool {
		if !views[left].TouchedAt.Equal(views[right].TouchedAt) {
			return views[left].TouchedAt.After(views[right].TouchedAt)
		}
		return views[left].Title < views[right].Title
	})
}

// underManagedRoot reports whether path sits inside root. Both come from the same
// resolver and Git, so a plain prefix test on a separator-terminated root is
// enough; no cleaning is attempted, because a path that needs it is not a path
// this produced.
func underManagedRoot(path, root string) bool {
	if root == "" || len(path) <= len(root) {
		return false
	}
	return path[:len(root)] == root && path[len(root)] == os.PathSeparator
}

// localBranch strips the ref prefix Git reports. Records are validated as local
// refs during parsing, so anything else is skipped rather than guessed at.
func localBranch(ref worktree.Ref) string {
	const prefix = "refs/heads/"
	value := string(ref)
	if len(value) <= len(prefix) || value[:len(prefix)] != prefix {
		return ""
	}
	return value[len(prefix):]
}

// managedPRNumber recovers the pull-request number from a worktree branch zwm
// created for one; the second result is false for ordinary branches.
func managedPRNumber(branch string) (string, bool) {
	if match := managedPRBranch.FindStringSubmatch(branch); match != nil {
		return match[1], true
	}
	return "", false
}

// modifiedAt is the recency signal: the worktree directory's own modification
// time. It moves when the checkout is created and when its top level changes,
// and — usefully — not when an agent edits a nested file, so the ordering
// reflects working on a branch rather than any write anywhere beneath it. An
// unreadable path sorts last rather than failing the row.
func modifiedAt(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
