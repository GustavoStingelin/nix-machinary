package command

import (
	"context"
	"os"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/agentstate"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/tui"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

// agentStateTTL bounds how long a done/stale agent record lingers once its pane
// is gone; Zellij 0.43.1 emits no pane-close event, so time is the backstop.
const agentStateTTL = 24 * time.Hour

// NewSystemTUI wires the dashboard against the real Zellij binary, the on-disk
// agent-state store, and the same completer/service the CLI subcommands use.
func NewSystemTUI() cli.TUIRunner {
	config := zellij.Config{Runner: zellij.SystemRunner{}, Environment: zellij.SystemEnvironment{}}
	store := agentstate.NewStore(agentstate.Dir(os.LookupEnv))
	current, _ := zellij.CurrentSession(config)
	return tui.NewRunner(
		tuiSource{config: config, store: store},
		tuiJumper{config: config, current: current},
		tuiCommander{completer: NewSystemCompleter(), service: NewSystemService()},
		current,
	)
}

// tuiCommander adapts the shell completer and application service to the
// tui.Commander port, so the dashboard runs the exact same project commands as
// the wco/o/wpr subcommands (which create/focus the tab themselves).
type tuiCommander struct {
	completer cli.Completer
	service   cli.Service
}

func (commander tuiCommander) Projects(ctx context.Context) []string {
	return commander.completer.Projects(ctx)
}

func (commander tuiCommander) Branches(ctx context.Context, project string) []string {
	return commander.completer.Branches(ctx, cli.ProjectNameOrPath(project))
}

func (commander tuiCommander) PullRequests(ctx context.Context, project string) []string {
	return commander.completer.PullRequests(ctx, cli.ProjectNameOrPath(project))
}

func (commander tuiCommander) Open(ctx context.Context, project string) error {
	return commander.run(ctx, project, cli.OpenProject{})
}

func (commander tuiCommander) CheckoutExisting(ctx context.Context, project, branch string) error {
	return commander.run(ctx, project, cli.CheckoutExisting{Branch: cli.BranchName(branch)})
}

func (commander tuiCommander) CheckoutNew(ctx context.Context, project, branch string) error {
	return commander.run(ctx, project, cli.CheckoutNew{Branch: cli.BranchName(branch)})
}

func (commander tuiCommander) PullRequest(ctx context.Context, project, selector string, force bool) error {
	return commander.run(ctx, project, cli.PullRequest{Selector: cli.PullRequestSelector(selector), Force: force})
}

func (commander tuiCommander) run(ctx context.Context, project string, action cli.Action) error {
	_, err := commander.service.Execute(ctx, cli.Invocation{
		Project: cli.ProjectNameOrPath(project),
		Action:  action,
	})
	return err
}

// tuiSource adapts the Zellij inventory commands and the agent-state store to
// the tui.Source port, converting adapter types into the TUI's view structs.
type tuiSource struct {
	config zellij.Config
	store  *agentstate.Store
}

func (source tuiSource) Sessions(ctx context.Context) ([]tui.SessionView, error) {
	sessions, err := zellij.ListSessions(ctx, source.config)
	if err != nil {
		return nil, err
	}
	current, _ := zellij.CurrentSession(source.config)

	names := make([]string, 0, len(sessions))
	views := make([]tui.SessionView, 0, len(sessions))
	for _, session := range sessions {
		names = append(names, session.Name)
		views = append(views, tui.SessionView{
			Name:    session.Name,
			Current: session.Name == current,
			Exited:  session.Exited,
		})
	}
	// Drop records for sessions that no longer exist (and time-expire the rest)
	// so the dashboard doesn't show ghosts.
	source.store.Prune(names, agentStateTTL)
	return views, nil
}

func (source tuiSource) Tabs(ctx context.Context, session string) ([]tui.TabView, error) {
	tabs, err := zellij.QueryTabNames(ctx, source.config, session)
	if err != nil {
		return nil, err
	}
	views := make([]tui.TabView, 0, len(tabs))
	for _, tab := range tabs {
		views = append(views, tui.TabView{Title: tab.Title, NeedsAttention: tab.NeedsAttention})
	}
	return views, nil
}

func (source tuiSource) Agents(ctx context.Context, session string) ([]tui.AgentView, error) {
	records, err := source.store.Load()
	if err != nil {
		return nil, err
	}
	// An agent whose tab has closed is dead. Reconcile against the session's live
	// tabs and forget those records so a finished agent doesn't linger (and can't
	// resurface if the same worktree tab is reopened later). Only reconcile when
	// the tab query succeeds with a non-empty list, so a transient failure never
	// deletes live state.
	liveTabs, reconcile := source.liveTabTitles(ctx, session)

	views := make([]tui.AgentView, 0)
	for _, record := range records {
		if record.Session != session {
			continue
		}
		if reconcile && record.TabTitle != "" {
			if _, live := liveTabs[record.TabTitle]; !live {
				_ = source.store.Delete(record.Session, record.PaneID)
				continue
			}
		}
		views = append(views, tui.AgentView{
			Agent:    record.Agent,
			PaneID:   record.PaneID,
			TabTitle: record.TabTitle,
			State:    string(record.State),
		})
	}
	return views, nil
}

// liveTabTitles returns the set of current tab titles for a session. The second
// result is false when the query failed or returned nothing, signalling callers
// not to treat records as stale on unreliable data.
func (source tuiSource) liveTabTitles(ctx context.Context, session string) (map[string]struct{}, bool) {
	tabs, err := zellij.QueryTabNames(ctx, source.config, session)
	if err != nil || len(tabs) == 0 {
		return nil, false
	}
	titles := make(map[string]struct{}, len(tabs))
	for _, tab := range tabs {
		titles[tab.Title] = struct{}{}
	}
	return titles, true
}

// tuiJumper focuses a tab — and, when the target names one, the pane inside it —
// in the current session. Cross-session switching is not available from the
// Zellij 0.43.1 CLI while attached, so it is refused for now.
type tuiJumper struct {
	config  zellij.Config
	current string
}

func (jumper tuiJumper) JumpTo(ctx context.Context, target tui.JumpTarget) error {
	if target.Session != jumper.current {
		return errs.New(errs.External, "cross-session jump is not supported yet")
	}
	if _, err := zellij.GoToTab(ctx, jumper.config, target.Tab); err != nil {
		return err
	}
	if target.PaneID == "" {
		return nil
	}
	// Land on the agent's own pane within the tab. The CLI has no pane-focus
	// action, so this goes through the zwm-attn plugin, and it is best-effort:
	// the tab is already focused, so a missing plugin or a pane that has since
	// closed must not turn a good jump into an error.
	focusCtx, cancel := context.WithTimeout(ctx, attnPipeTimeout)
	defer cancel()
	_, _ = zellij.FocusPane(focusCtx, jumper.config, target.PaneID)
	return nil
}
