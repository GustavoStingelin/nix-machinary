// Package tui renders the `zwm tui` session dashboard: open Zellij sessions,
// their tabs, and the three-way attention state of the coding agents inside
// them, with Enter to jump to a tab (or straight to an agent's pane). It
// depends only on the consumer-owned ports
// below, so it never imports the Zellij or agent-state adapters directly.
package tui

import "context"

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

// Source supplies the dashboard's data. Each call may shell out to Zellij or
// read the agent-state store, so the model invokes them from tea.Cmd goroutines.
type Source interface {
	Sessions(ctx context.Context) ([]SessionView, error)
	Tabs(ctx context.Context, session string) ([]TabView, error)
	Agents(ctx context.Context, session string) ([]AgentView, error)
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
}
