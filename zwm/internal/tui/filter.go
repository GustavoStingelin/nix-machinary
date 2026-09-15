package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// `/` filters the worktrees pane, k9s-style: the prompt opens at the top, the
// list narrows on every keystroke, and the arrows keep working while you type so
// you can steer straight onto a row. Enter keeps the filter and hands the keys
// back to the pane; Esc clears it.
//
// It is not a mode of its own (the pane still renders and navigates normally),
// only a claim on the letter keys, which is why it is a flag rather than a
// uiMode.
type filter struct {
	active bool
	text   string
}

// matches reports whether a worktree should be shown. The path is searched
// alongside the title because the two carry different information — "1313" finds
// a pull request by number, "itests" finds it by directory — and case is ignored
// because branch names are not typed carefully.
func (f filter) matches(entry WorktreeView) bool {
	if f.text == "" {
		return true
	}
	needle := strings.ToLower(f.text)
	for _, field := range []string{entry.Title, entry.Branch, entry.Worktree, entry.Project} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// beginFilter opens the prompt. It is a worktrees-pane key: the dashboard's
// sections are short and live, and narrowing them would hide the very state the
// pane exists to report.
func (m *model) beginFilter() tea.Cmd {
	if m.activePane != paneWorktrees {
		return nil
	}
	m.filter.active = true
	m.status = ""
	return nil
}

// handleFilterKey consumes keys while the prompt is open. Navigation and the
// two exits are passed through; everything else is text, which is why `d` types
// a `d` here rather than deleting anything.
func (m *model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.clearFilter()
		return m, nil
	case tea.KeyEnter:
		// Keep the filter, stop capturing keys: the point of filtering is to act on
		// what is left.
		m.filter.active = false
		return m, nil
	case tea.KeyUp:
		m.moveCursor(-1)
		return m, nil
	case tea.KeyDown:
		m.moveCursor(1)
		return m, nil
	case tea.KeyBackspace:
		if m.filter.text != "" {
			m.filter.text = m.filter.text[:len(m.filter.text)-1]
			m.applyFilter()
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.filter.text += string(msg.Runes)
		m.applyFilter()
		return m, nil
	}
	return m, nil
}

// clearFilter drops the filter and the prompt together: an empty filter left
// showing an empty prompt would claim the letter keys for nothing.
func (m *model) clearFilter() {
	m.filter = filter{}
	m.applyFilter()
}

// applyFilter rebuilds the rows and puts the cursor back at the top, because the
// row the old index pointed at is rarely the row still under it.
func (m *model) applyFilter() {
	m.cursor = 0
	m.offset = 0
	m.rebuildRows()
}

// filterPrompt is the k9s-style input line, shown only while the prompt is open
// or a filter is in force — a pane with nothing filtered carries no chrome.
func (m *model) filterPrompt() (string, bool) {
	if m.activePane != paneWorktrees || (!m.filter.active && m.filter.text == "") {
		return "", false
	}
	line := titleStyle.Render("/") + m.filter.text
	if m.filter.active {
		return line + dimStyle.Render("▏"), true
	}
	// Not capturing keys any more, so say what gets the filter back off again.
	return line + dimStyle.Render("  (esc to clear)"), true
}
