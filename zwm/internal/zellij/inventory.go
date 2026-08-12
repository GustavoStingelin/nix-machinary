package zellij

import (
	"context"
	"strings"
)

// The markers the zwm-attn plugin keeps at the front of a tab name to carry the
// attention state of the agents inside it. Each is two columns wide, so a state
// change never reflows the tab bar; clearedMarker is the blank a marker the user
// has seen retires to, which means a tab an agent has run in keeps a two-column
// indent forever. All four must stay byte-for-byte identical to the constants in
// zellij-plugins/zwm-tabmark/src/lib.rs, which is where the rules that maintain
// them live.
const (
	workingGlyph   = "◐ "
	attentionGlyph = "● "
	doneGlyph      = "✓ "
	clearedMarker  = "  "
)

// Session is a Zellij session. Exited marks a dead session that survives only
// for resurrection (Zellij's "EXITED - attach to resurrect").
type Session struct {
	Name   string
	Exited bool
}

// Tab is a tab within a session. NeedsAttention reflects the zwm-attn glyph on
// the tab name (binary and self-clearing on focus); Title is the name with the
// glyph stripped.
type Tab struct {
	Title          string
	NeedsAttention bool
}

// ListSessions returns the sessions via `zellij list-sessions --no-formatting`,
// which prints one line per session as "<name> [Created ...] (<status>)" with no
// ANSI. The status distinguishes exited sessions so the dashboard can order them
// last. It works whether or not the caller is attached.
func ListSessions(ctx context.Context, config Config) ([]Session, error) {
	command := Command{Name: CommandZellij, Args: []string{"list-sessions", "--no-formatting"}}
	output, err := config.Runner.Run(ctx, command)
	if err != nil {
		return nil, externalFailure("list Zellij sessions", command, output, err)
	}
	return parseSessions(output.Stdout), nil
}

// QueryTabNames returns the tabs of any session by name via
// `zellij --session <name> action query-tab-names`, which routes to that session
// even when the caller is attached elsewhere.
func QueryTabNames(ctx context.Context, config Config, session string) ([]Tab, error) {
	command := Command{Name: CommandZellij, Args: []string{"--session", session, "action", "query-tab-names"}}
	output, err := config.Runner.Run(ctx, command)
	if err != nil {
		return nil, externalFailure("query Zellij tab names", command, output, err)
	}
	return parseTabs(output.Stdout), nil
}

// FocusTabTitle focuses the tab with the given title in the current session.
//
// The title is not always the tab's name: zwm-attn keeps a marker at the front
// of the name, and Zellij matches go-to-tab-name against the raw name
// (screen.rs compares `t.name == name`), so passing the marker-stripped title
// finds nothing and silently does nothing — which is what the dashboard's Enter
// did for every tab an agent had ever run in. Recover the raw name first. If the
// query fails the title is sent as-is, which is no worse than not trying.
func FocusTabTitle(ctx context.Context, config Config, title string) (Output, error) {
	name := title
	if output, err := config.Runner.Run(ctx, tabNamesCommand()); err == nil {
		if raw, found := findTabName(output.Stdout, TabTitle(title)); found {
			name = raw
		}
	}
	return GoToTab(ctx, config, name)
}

// GoToTab focuses a tab by its raw name — markers included — in the current
// session. Callers holding a title rather than a name want FocusTabTitle.
// Cross-session switching is not possible from the Zellij 0.43.1 CLI while
// attached, so this only works for the caller's own session.
func GoToTab(ctx context.Context, config Config, tab string) (Output, error) {
	command := Command{Name: CommandZellij, Args: []string{"action", "go-to-tab-name", tab}}
	output, err := config.Runner.Run(ctx, command)
	if err != nil {
		return output, externalFailure("focus Zellij tab", command, output, err)
	}
	return output, nil
}

// CurrentSession reports the session the caller is attached to, from
// ZELLIJ_SESSION_NAME.
func CurrentSession(config Config) (string, bool) {
	name, present := config.Environment.Lookup(EnvironmentZellijSessionName)
	if !present || name == "" {
		return "", false
	}
	return name, true
}

func parseSessions(stdout string) []Session {
	sessions := make([]Session, 0)
	for line := range strings.SplitSeq(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		name, _, _ := strings.Cut(trimmed, " ")
		if name == "" {
			continue
		}
		sessions = append(sessions, Session{Name: name, Exited: strings.Contains(trimmed, "EXITED")})
	}
	return sessions
}

func parseTabs(stdout string) []Tab {
	tabs := make([]Tab, 0)
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		tabs = append(tabs, parseTab(line))
	}
	return tabs
}

func parseTab(line string) Tab {
	// Every marker has to be recognised, not just the ones that mean something
	// here: the title is what Launch matches an existing tab on and what a jump
	// resolves against, so a marker left in it would open a second tab carrying
	// the same title, or jump nowhere.
	for _, marker := range []string{attentionGlyph, doneGlyph, workingGlyph, clearedMarker} {
		if stripped, ok := strings.CutPrefix(line, marker); ok {
			// Waiting and done are the states that want the user; a working agent
			// is just busy, and the dashboard shows its own spinner for it.
			needsAttention := marker == attentionGlyph || marker == doneGlyph
			return Tab{Title: stripped, NeedsAttention: needsAttention}
		}
	}
	return Tab{Title: line}
}
