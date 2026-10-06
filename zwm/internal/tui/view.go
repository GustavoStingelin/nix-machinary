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
	if m.mode == modeConfirm {
		return m.confirmView()
	}
	if !m.ready {
		return "\n  loading sessions…\n"
	}

	// The pane bar and the footer always cost a row each, and the spare keeps the
	// body clear of the terminal's own last line.
	chrome := 3
	prompt, promptShown := m.filterPrompt()
	if promptShown {
		chrome++
	}
	body := m.window(m.displayLines(), chrome)

	var out strings.Builder
	out.WriteString(m.paneBar())
	out.WriteByte('\n')
	if promptShown {
		out.WriteString(prompt)
		out.WriteByte('\n')
	}
	for _, line := range body {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteString(m.footer())
	return out.String()
}

// paneBar names the panes and marks the active one, so the keys that switch
// them are visible rather than something you have to know.
func (m *model) paneBar() string {
	labels := []string{"1 dashboard", "2 worktrees", "3 my PRs"}
	rendered := make([]string, 0, len(labels))
	for index, label := range labels {
		if pane(index) == m.activePane {
			rendered = append(rendered, titleStyle.Render(label))
			continue
		}
		rendered = append(rendered, dimStyle.Render(label))
	}
	bar := titleStyle.Render("zwm") + "  " + strings.Join(rendered, dimStyle.Render(" │ "))
	if m.activePane != paneWorktrees || !m.worktreesLoaded {
		return bar
	}
	// The count is of what is on screen, so a filter's effect is visible even
	// when the matches all fit without scrolling.
	count := fmt.Sprintf("[%d]", len(m.rows))
	if m.filter.text != "" {
		count = fmt.Sprintf("[%d/%d]", len(m.rows), len(m.worktrees))
	}
	return bar + "  " + dimStyle.Render(count)
}

// displayLines flattens the active pane into rendered rows, tracking which map
// back to navigable selections so the cursor and scrolling can be resolved.
func (m *model) displayLines() []displayLine {
	switch m.activePane {
	case paneWorktrees:
		return m.worktreeLines()
	case paneMine:
		return m.mineLines()
	}
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

// worktreeLines renders the worktrees pane, grouped under a heading per project
// so a machine with several checkouts reads as a list of projects rather than
// one flat run of similar-looking paths.
func (m *model) worktreeLines() []displayLine {
	lines := make([]displayLine, 0, len(m.worktrees)+4)
	if !m.worktreesLoaded {
		return append(lines, displayLine{text: dimStyle.Render("  loading…"), row: -1})
	}
	if len(m.worktrees) == 0 {
		return append(lines, displayLine{text: dimStyle.Render("  (no worktrees)"), row: -1})
	}
	if len(m.rows) == 0 {
		return append(lines, displayLine{text: dimStyle.Render("  (no matches)"), row: -1})
	}
	// Rows, not m.worktrees: a filter leaves gaps in the latter, and the headings
	// must follow what survives it.
	project := ""
	for row, selected := range m.rows {
		entry := m.worktrees[selected.worktree]
		if entry.Project != project {
			project = entry.Project
			lines = append(lines, displayLine{text: dimStyle.Render("─── " + project + " ───"), row: -1})
		}
		open := m.tabIsOpenInCurrentSession(entry.Title)
		lines = append(lines, displayLine{text: renderWorktree(entry, open, m.now(), m.cursor == row), row: row})
	}
	return lines
}

// mineLines renders the "my PRs" pane, grouped under a heading per repository
// like the worktrees pane is per project.
func (m *model) mineLines() []displayLine {
	header := titleStyle.Render("my pull requests")
	if m.mineRefreshing {
		header += " " + workingStyle.Render(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
	}
	lines := []displayLine{{text: header, row: -1}}
	if !m.mineLoaded {
		return append(lines, displayLine{text: dimStyle.Render("  loading…"), row: -1})
	}
	if len(m.mine) == 0 {
		return append(lines, displayLine{text: dimStyle.Render("  (no open pull requests)"), row: -1})
	}
	width := reviewNumberWidth(m.mine)
	repository := ""
	for row, selected := range m.rows {
		pullRequest := m.mine[selected.mine]
		if pullRequest.Repository != repository {
			repository = pullRequest.Repository
			lines = append(lines, displayLine{text: dimStyle.Render("─── " + repository + " ───"), row: -1})
		}
		lines = append(lines, displayLine{text: m.renderMine(pullRequest, width, m.cursor == row), row: row})
	}
	return lines
}

// renderMine renders one of the user's pull requests: its branches, then where
// Enter will take it — an open tab, an existing worktree, a local branch, or
// (with no badge) a fresh wpr checkout.
func (m *model) renderMine(pullRequest ReviewView, numberWidth int, selected bool) string {
	line := gutter(selected) + fmt.Sprintf("#%-*s", numberWidth, pullRequest.Number)
	if pullRequest.Project == "" {
		return line + dimStyle.Render(reviewBranches(pullRequest)+"  "+pullRequest.Title+"  (not cloned)")
	}
	line += dimStyle.Render(reviewBranches(pullRequest))
	switch entry, ok := m.mineWorktree(pullRequest); {
	case ok && m.tabIsOpenInCurrentSession(entry.Title):
		line += "  " + doneStyle.Render("open")
	case ok:
		line += "  " + doneStyle.Render("worktree")
	case pullRequest.LocalBranch:
		line += "  " + dimStyle.Render("branch")
	}
	return line + "  " + pullRequest.Title
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
// window scrolls lines so the cursor stays visible. chrome is how many rows the
// pane bar, the filter prompt and the footer take, which the body cannot use.
func (m *model) window(lines []displayLine, chrome int) []string {
	bodyHeight := max(m.height-chrome, 1)
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

// renderWorktree renders one worktree row: the tab it would restore, a badge for
// the kinds that are not an ordinary managed checkout, and either an "open"
// badge — because Enter then jumps instead of checking anything out — or how
// long ago it was touched.
func renderWorktree(entry WorktreeView, open bool, now time.Time, selected bool) string {
	line := gutter(selected) + entry.Title
	if badge := worktreeBadge(entry.Kind); badge != "" {
		line += "  " + dimStyle.Render(badge)
	}
	if open {
		return line + "  " + doneStyle.Render("open")
	}
	return line + "  " + dimStyle.Render(worktreeAge(entry.TouchedAt, now))
}

// worktreeBadge names the kinds worth calling out: the repository root, which
// cannot be deleted, and a worktree zwm did not create, which no wco/wpr
// reopens. A plain managed checkout is the default and carries none.
func worktreeBadge(kind WorktreeKind) string {
	switch kind {
	case WorktreePrimary:
		return "root"
	case WorktreeExternal:
		return "external"
	}
	return ""
}

// worktreeAge renders a coarse age: the list only needs to convey "yesterday" from
// "last month", and a zero time means the worktree could not be stat'd at all.
func worktreeAge(touched, now time.Time) string {
	if touched.IsZero() {
		return "?"
	}
	elapsed := now.Sub(touched)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
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
	if m.activePane == paneMine {
		return footerStyle.Render("↑/↓ move · enter work on it · b browser · [ ] pane · r refresh · q quit")
	}
	if m.activePane == paneWorktrees {
		verb := "enter open"
		if row, ok := m.currentRow(); ok && m.tabIsOpenInCurrentSession(m.worktrees[row.worktree].Title) {
			verb = "enter jump"
		}
		if m.filter.active {
			return footerStyle.Render("type to filter · ↑/↓ move · enter accept · esc clear")
		}
		return footerStyle.Render("↑/↓ move · " + verb + " · / filter · d delete · [ ] pane · r refresh · q quit")
	}
	// The review queue rebinds enter and adds two keys, so the hint follows the
	// cursor rather than listing every binding at once.
	if row, ok := m.currentRow(); ok {
		switch row.kind {
		case selReview:
			return footerStyle.Render("↑/↓ move · tab section · enter checkout · ctrl+f force · a review agent · b browser · r refresh · q quit")
		}
	}
	return footerStyle.Render("↑/↓ move · tab section · enter jump · o open · w wco · p wpr · [ ] pane · r refresh · q quit")
}

func gutter(selected bool) string {
	if selected {
		return titleStyle.Render("❯ ")
	}
	return "  "
}
