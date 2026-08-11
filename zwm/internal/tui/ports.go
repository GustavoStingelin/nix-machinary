// Package tui renders the `zwm tui` session dashboard: open Zellij sessions,
// their tabs, and the three-way attention state of the coding agents inside
// them, with Enter to jump to a tab (or straight to an agent's pane). It
// depends only on the consumer-owned ports
// below, so it never imports the Zellij or agent-state adapters directly.
package tui

import (
	"context"
	"time"
)

// Agent attention states, matching agentstate's vocabulary. Kept as plain
// strings so this package stays free of other internal dependencies.
const (
	StateWorking = "working"
	StateWaiting = "waiting"
	StateDone    = "done"
)

// SessionView is a Zellij session. Exited marks a dead session kept only for
// resurrection.
type SessionView struct {
	Name    string
	Current bool
	Exited  bool
}

// TabView is a tab within a session. NeedsAttention reflects the zwm-attn tab
// glyph (binary, self-clearing on focus).
type TabView struct {
	Title          string
	NeedsAttention bool
}

// AgentView is one coding agent (one pane) and its three-way attention state.
// TabTitle is the tab the agent runs in, so the dashboard can nest it under that
// tab; empty means the tab is unknown and the agent renders at session level.
type AgentView struct {
	Agent    string
	PaneID   string
	TabTitle string
	State    string
}

// ReviewView is one open pull request awaiting the user's review.
//
// Project is the local project under the code root that the pull request's
// repository maps to, empty when there is no local checkout — those rows still
// show (a review request you can't act on locally is still a review request) but
// cannot be opened. Worktree is the managed worktree path once the pull request
// has been checked out, and Stale means that worktree's HEAD no longer matches
// the pull request's head commit, so opening it would show outdated code.
// Base is the branch the pull request merges into, which for a stacked pull
// request is the branch below it rather than the repository's default branch.
// It is shown because it decides what a review is actually diffing.
type ReviewView struct {
	Number     string
	Repository string
	Project    string
	Title      string
	Author     string
	Base       string
	Head       string
	Worktree   string
	Stale      bool
}

// Source supplies the dashboard's data. Each call may shell out to Zellij, gh,
// or git, or read the agent-state store, so the model invokes them from tea.Cmd
// goroutines.
type Source interface {
	Sessions(ctx context.Context) ([]SessionView, error)
	Tabs(ctx context.Context, session string) ([]TabView, error)
	// Agents lists the session's agent records. liveTabs carries the session's
	// tabs when the caller has just queried them, so records whose tab has since
	// closed can be forgotten; an empty list means "no reliable tab list here",
	// which skips that reconciliation rather than deleting live state. Taking the
	// tabs as an argument keeps the caller's one tab query serving both, instead
	// of costing the Zellij server a second one per refresh.
	Agents(ctx context.Context, session string, liveTabs []TabView) ([]AgentView, error)
	// Reviews lists pull requests awaiting the user's review, in any order — the
	// model sorts them. It is the slowest source (a GitHub search plus a
	// per-pull-request ref lookup), so the model loads it on its own schedule
	// rather than every tick, and persists the result for CachedReviews.
	Reviews(ctx context.Context) ([]ReviewView, error)
	// CachedReviews returns the last persisted queue and when it was fetched,
	// touching no network so the section has rows to draw immediately. ok is
	// false when no usable cache exists.
	CachedReviews(ctx context.Context) (reviews []ReviewView, fetchedAt time.Time, ok bool)
}

// JumpTarget is where Enter should land: a tab, plus the pane inside it when
// known. Agent rows carry the agent's own pane so the jump lands on it; tab
// rows leave PaneID empty and stop at the tab.
type JumpTarget struct {
	Session string
	Tab     string
	PaneID  string
}

// Jumper focuses a jump target. Same-session jumps use go-to-tab-name and then
// focus the pane; cross-session jumps are not yet supported and return an error
// the model surfaces as a hint.
type Jumper interface {
	JumpTo(ctx context.Context, target JumpTarget) error
}

// Commander runs zwm's project commands from inside the dashboard. The listing
// methods are best-effort (empty on failure, matching the shell completer); the
// action methods create/focus the Zellij tab themselves, so the dashboard just
// quits onto the result.
type Commander interface {
	Projects(ctx context.Context) []string
	Branches(ctx context.Context, project string) []string
	PullRequests(ctx context.Context, project string) []string
	Open(ctx context.Context, project string) error
	CheckoutExisting(ctx context.Context, project, branch string) error
	CheckoutNew(ctx context.Context, project, branch string) error
	// PullRequest checks out a PR worktree; force resets an existing managed
	// worktree to the PR's latest remote state (wpr --force).
	PullRequest(ctx context.Context, project, selector string, force bool) error
	// ReviewPullRequest checks out the PR worktree exactly as PullRequest does and
	// then starts a review agent in that tab, seeded with a prompt naming the pull
	// request and its real base branch. repository is "owner/name", needed because
	// the review queue spans repositories.
	ReviewPullRequest(ctx context.Context, project, repository, selector string, force bool) error
	// BrowsePullRequest opens a pull request on GitHub in the user's browser. It
	// needs no local checkout, so it works for every row in the review queue.
	BrowsePullRequest(ctx context.Context, repository, selector string) error
}
