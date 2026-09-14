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

// worktreeLimit is a guard against a pathological machine, not a working-set
// cap. The pane is now the only place a worktree zwm did not create is visible
// at all, so silently hiding one would be worse than a long scroll.
const worktreeLimit = 200

// Worktrees lists every Git worktree of every project under the code root, most
// recently touched first, so any of them can be reopened or deleted from the
// dashboard.
//
// It reports all of them, not only the ones under the managed root, because the
// worktrees that most need pruning are exactly the ones zwm did not create and
// therefore does not otherwise show. What differs between them is only how they
// open, which is what tui.WorktreeKind records.
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
func (source tuiSource) Worktrees(ctx context.Context) ([]tui.WorktreeView, error) {
	views := make([]tui.WorktreeView, 0)
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
		views = append(views, worktreeViews(resolution, records, modifiedAt)...)
	}
	sortWorktrees(views)
	if len(views) > worktreeLimit {
		views = views[:worktreeLimit]
	}
	return views, nil
}

// worktreeViews maps one project's worktree records to dashboard rows. touched
// reads a worktree's modification time; it is a parameter so the mapping is
// testable without a filesystem.
//
// Git lists the primary worktree first, which is how it is recognised — the
// porcelain output carries no other marker for it.
func worktreeViews(resolution project.Resolution, records []worktree.Record, touched func(string) time.Time) []tui.WorktreeView {
	views := make([]tui.WorktreeView, 0, len(records))
	for index, record := range records {
		if record.Prunable {
			continue
		}
		path := string(record.Path)
		if index == 0 {
			// The primary worktree opens by project, not by branch, so its branch is
			// deliberately left unset — it is also the one that changes under you.
			views = append(views, tui.WorktreeView{
				Project:   string(resolution.Key),
				Kind:      tui.WorktreePrimary,
				Title:     string(resolution.Key),
				Worktree:  path,
				TouchedAt: touched(path),
			})
			continue
		}
		// A detached or bare linked worktree has no branch to name a tab after and
		// no command that reopens it, so it is skipped rather than shown unusable.
		if record.State != worktree.HeadBranch {
			continue
		}
		branch, ok := worktree.LocalBranch(record.Branch)
		if !ok {
			continue
		}
		managed := worktree.UnderManagedRoot(record.Path, worktree.Path(resolution.ManagedRoot))
		views = append(views, worktreeView(string(resolution.Key), string(branch), path, managed, touched(path)))
	}
	return views
}

// worktreeView builds one linked-worktree row, reproducing the tab title its
// command would give it so the dashboard can tell whether that tab is already
// open.
//
// A pull-request worktree is the interesting case: `wpr` checks out a branch
// named "zwm/pr-<n>-<hash>" but titles the tab "<key>:pr-<n>", and reopening it
// has to go back through `wpr` — a plain `wco` of that branch would work but
// would title the tab after the raw branch, leaving two names for one worktree.
// A worktree outside the managed root has no such command at all and is opened
// at its own path, but it is still titled the same way so a tab already open on
// it is still recognised.
func worktreeView(key, branch, path string, managed bool, touched time.Time) tui.WorktreeView {
	if number, ok := managedPRNumber(branch); ok && managed {
		return tui.WorktreeView{
			Project:     key,
			Kind:        tui.WorktreePullRequest,
			Branch:      branch,
			Title:       key + ":pr-" + number,
			Worktree:    path,
			TouchedAt:   touched,
			PullRequest: number,
		}
	}
	kind := tui.WorktreeExternal
	if managed {
		kind = tui.WorktreeManaged
	}
	return tui.WorktreeView{
		Project:   key,
		Kind:      kind,
		Branch:    branch,
		Title:     key + ":" + branch,
		Worktree:  path,
		TouchedAt: touched,
	}
}

// sortWorktrees groups rows by project and orders each group with the
// repository root first and the rest most recently touched first, the title
// breaking ties so the list is stable when timestamps match (or are all zero
// because every stat failed).
//
// Project is the primary key rather than recency because the pane renders a
// heading per project: a global recency order would interleave them and print
// the same project under several headings.
func sortWorktrees(views []tui.WorktreeView) {
	sort.SliceStable(views, func(left, right int) bool {
		first, second := views[left], views[right]
		if first.Project != second.Project {
			return first.Project < second.Project
		}
		if (first.Kind == tui.WorktreePrimary) != (second.Kind == tui.WorktreePrimary) {
			return first.Kind == tui.WorktreePrimary
		}
		if !first.TouchedAt.Equal(second.TouchedAt) {
			return first.TouchedAt.After(second.TouchedAt)
		}
		return first.Title < second.Title
	})
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
