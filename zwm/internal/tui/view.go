package tui

import (
	"fmt"
	"strings"
	"time"
)

type displayLine struct {
	text string
	row  int // index into m.rows, or -1 for non-navigable info lines
}

func (m *model) View() string {
	if m.mode == modePicker {
		return m.pickerView()
	}
	if !m.ready {
		return "\n  loading sessions…\n"
	}

	lines := m.displayLines()
	body := m.window(lines)

	var out strings.Builder
	out.WriteString(titleStyle.Render("zwm"))
	out.WriteByte('\n')
	for _, line := range body {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteString(m.footer())
	return out.String()
}

// displayLines flattens the session tree into rendered rows, tracking which map
// back to navigable selections so the cursor and scrolling can be resolved.
func (m *model) displayLines() []displayLine {
	lines := make([]displayLine, 0)
	row := 0

	// Agents panel: all running agents across sessions, ordered by attention, so
	// jumping between the ones that need you is one section at the top.
	if len(m.agents) > 0 {
		lines = append(lines, displayLine{text: titleStyle.Render("agents"), row: -1})
		for _, entry := range m.agents {
			selected := m.cursor == row
			lines = append(lines, displayLine{text: renderAgentEntry(entry, m.current, selected), row: row})
			row++
		}
		lines = append(lines, displayLine{text: dimStyle.Render("───"), row: -1})
	}

	// Review queue: pull requests waiting on you, one Tab away from the agents
	// panel. Rendered before the tree because it is a to-do list, not state.
	lines = append(lines, displayLine{text: m.reviewHeader(), row: -1})
	switch {
	case !m.reviewsLoaded:
		lines = append(lines, displayLine{text: dimStyle.Render("  loading…"), row: -1})
	case len(m.reviews) == 0:
		lines = append(lines, displayLine{text: dimStyle.Render("  (nothing waiting on you)"), row: -1})
	default:
		width := reviewNumberWidth(m.reviews)
		for _, review := range m.reviews {
			selected := m.cursor == row
			lines = append(lines, displayLine{text: renderReview(review, width, selected), row: row})
			row++
		}
	}
	lines = append(lines, displayLine{text: dimStyle.Render("─── sessions ───"), row: -1})

	for _, session := range m.sessions {
		selected := m.cursor == row
		lines = append(lines, displayLine{text: m.renderSession(session, selected), row: row})
		row++
		if !session.expanded {
			continue
		}
		// Agents whose tab is unknown (or no longer open) render at session level.
		for _, agent := range session.agents {
			if !agentMatchesAnyTab(agent, session.tabs) {
				lines = append(lines, displayLine{text: renderAgent(agent, "      "), row: -1})
			}
		}
		for _, tab := range session.tabs {
			selected := m.cursor == row
			lines = append(lines, displayLine{text: renderTab(tab, selected), row: row})
			row++
			// Nest each agent under the tab it runs in.
			for _, agent := range session.agents {
				if agent.TabTitle != "" && agent.TabTitle == tab.Title {
					lines = append(lines, displayLine{text: renderAgent(agent, "          "), row: -1})
				}
			}
		}
		if len(session.tabs) == 0 {
			lines = append(lines, displayLine{text: dimStyle.Render("      (no tabs)"), row: -1})
		}
	}
	return lines
}

func agentMatchesAnyTab(agent AgentView, tabs []TabView) bool {
	if agent.TabTitle == "" {
		return false
	}
	for _, tab := range tabs {
		if tab.Title == agent.TabTitle {
			return true
		}
	}
	return false
}

// window scrolls the body so the cursor line stays visible, mutating the stored
// offset. bodyHeight leaves room for the title and footer.
func (m *model) window(lines []displayLine) []string {
	bodyHeight := max(m.height-3, 1)
	if len(lines) <= bodyHeight {
		m.offset = 0
	} else {
		cursorLine := 0
		for i, line := range lines {
			if line.row == m.cursor {
				cursorLine = i
				break
			}
		}
		if cursorLine < m.offset {
			m.offset = cursorLine
		}
		if cursorLine >= m.offset+bodyHeight {
			m.offset = cursorLine - bodyHeight + 1
		}
		m.offset = clamp(m.offset, 0, len(lines)-bodyHeight)
	}

	end := clamp(m.offset+bodyHeight, 0, len(lines))
	rendered := make([]string, 0, end-m.offset)
	for _, line := range lines[m.offset:end] {
		rendered = append(rendered, line.text)
	}
	return rendered
}

func (m *model) renderSession(session sessionState, selected bool) string {
	prefix := "▸ "
	switch {
	case session.exited:
		prefix = "· " // no expand affordance: exited sessions are display-only
	case session.expanded:
		prefix = "▾ "
	}
	line := gutter(selected) + prefix + titleStyle.Render(session.name)
	switch {
	case session.exited:
		line += dimStyle.Render(" (exited)")
	case session.current:
		line += dimStyle.Render(" (current)")
	}
	if badge := rollupBadge(session.agents); badge != "" && !session.exited {
		line += "  " + badge
	}
	return line
}

func renderAgent(agent AgentView, indent string) string {
	style, phrase := stateStyle(agent.State)
	label := agent.Agent
	if label == "" {
		label = "pane " + agent.PaneID
	}
	return indent + style.Render(stateGlyph(agent.State)+" "+label+"  "+phrase)
}

// renderAgentEntry renders a triage-panel row: the agent's state and label, then
// where it lives (its tab, prefixed with the session when it isn't the current
// one, so cross-session agents are legible even though jumping to them is not yet
// supported).
func renderAgentEntry(entry agentEntry, current string, selected bool) string {
	style, phrase := stateStyle(entry.state)
	location := entry.tabTitle
	if location == "" {
		location = "?"
	}
	if entry.session != current {
		location = entry.session + " · " + location
	}
	return gutter(selected) +
		style.Render(stateGlyph(entry.state)+" "+entry.label+"  "+phrase) +
		"  " + dimStyle.Render(location)
}

func renderTab(tab TabView, selected bool) string {
	marker := "  "
	if tab.NeedsAttention {
		marker = waitingStyle.Render("● ")
	}
	return gutter(selected) + "  " + marker + tab.Title
}

// spinnerFrames is the refresh indicator, reusing the half-circle already used
// for a working agent so "something is running" reads the same everywhere.
var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// reviewHeader titles the section and says what state it is in: a turning circle
// while a fetch is running, and — until the first fetch of this run lands — how
// old the cached rows below it are. Showing the age matters when gh is failing:
// without it a days-old queue would look current.
func (m *model) reviewHeader() string {
	header := titleStyle.Render("review queue")
	if m.refreshing {
		header += " " + workingStyle.Render(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
	}
	if m.reviewsFromCache && !m.reviewsFetchedAt.IsZero() {
		header += dimStyle.Render("  cached " + humanAge(m.now().Sub(m.reviewsFetchedAt)))
	}
	return header
}

// humanAge renders a duration the way a person would say it.
func humanAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

// reviewNumberWidth pads pull request numbers into a column so repositories and
// titles line up across rows.
func reviewNumberWidth(reviews []ReviewView) int {
	width := 0
	for _, review := range reviews {
		if len(review.Number) > width {
			width = len(review.Number)
		}
	}
	return width
}

// reviewBranches renders "head → base", the two branches the review spans. Base
// is the branch the pull request actually merges into, so for a stacked pull
// request this reads e.g. "task-watch-policy → itests/watch-only-create" rather
// than implying master. Empty when the refs could not be read.
func reviewBranches(review ReviewView) string {
	switch {
	case review.Head != "" && review.Base != "":
		return "  " + review.Head + " → " + review.Base
	case review.Base != "":
		return "  → " + review.Base
	case review.Head != "":
		return "  " + review.Head + " → ?"
	default:
		return ""
	}
}

// renderReview renders one review-queue row: the pull request, the branches it
// spans, and its local state. A pull request whose repository has no checkout under
// the code root is dimmed, because Enter cannot open it.
func renderReview(review ReviewView, numberWidth int, selected bool) string {
	line := gutter(selected) + fmt.Sprintf("#%-*s ", numberWidth, review.Number)

	repo := review.Repository
	if review.Project == "" {
		// No local checkout: say so where the status badges go, and dim the row.
		// `b` still works on this row, so the branches are still worth showing.
		return line + dimStyle.Render(repo+reviewBranches(review)+"  "+review.Title+"  (not cloned)")
	}
	line += repo + dimStyle.Render(reviewBranches(review))
	switch {
	case review.Stale:
		line += "  " + waitingStyle.Render("stale")
	case review.Worktree != "":
		line += "  " + doneStyle.Render("local")
	}
	if review.Author != "" {
		line += "  " + dimStyle.Render("@"+review.Author)
	}
	return line + "  " + review.Title
}

// rollupBadge summarizes a session's agents with the state that most wants the
// user's attention.
func rollupBadge(agents []AgentView) string {
	best := ""
	for _, agent := range agents {
		if urgency(agent.State) > urgency(best) {
			best = agent.State
		}
	}
	if best == "" {
		return ""
	}
	style, phrase := stateStyle(best)
	return style.Render(stateGlyph(best) + " " + phrase)
}

func (m *model) footer() string {
	if m.status != "" {
		return errorStyle.Render(m.status)
	}
	// The review queue rebinds enter and adds two keys, so the hint follows the
	// cursor rather than listing every binding at once.
	if row, ok := m.currentRow(); ok && row.kind == selReview {
		return footerStyle.Render("↑/↓ move · tab section · enter checkout · ctrl+f force · a review agent · b browser · r refresh · q quit")
	}
	return footerStyle.Render("↑/↓ move · tab section · enter jump · o open · w wco · p wpr · r refresh · q quit")
}

func gutter(selected bool) string {
	if selected {
		return titleStyle.Render("❯ ")
	}
	return "  "
}
