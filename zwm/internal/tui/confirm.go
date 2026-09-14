package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// `d` in the worktrees pane deletes the worktree under the cursor. It is the
// only destructive thing the dashboard does, so it goes through a confirmation
// step that names the worktree and offers the branch as a second, separate
// answer: `y` removes the checkout Git can recreate, `b` additionally removes
// the branch, which for an unmerged one is the last copy of its commits.

type confirmation struct {
	title    string
	project  string
	worktree string
	branch   string
}

// worktreeRemovedMsg reports a finished deletion. Like browsedMsg — and unlike
// the checkout commands — it does not quit: pruning worktrees is something you
// do to several rows in a row.
type worktreeRemovedMsg struct {
	title         string
	branchDeleted bool
	err           error
}

// confirmRemoveWorktree opens the confirmation for the row under the cursor.
//
// Two rows are refused before the prompt rather than after it. The primary
// worktree is the repository, and deleting it would take every other worktree's
// backing store with it — the service refuses it too, but there is no reason to
// offer it. And a worktree whose tab is still open in this session is some live
// shell's working directory, which Git would happily delete out from under it.
func (m *model) confirmRemoveWorktree() tea.Cmd {
	row, ok := m.currentRow()
	if !ok || row.kind != selWorktree {
		return nil
	}
	entry := m.worktrees[row.worktree]
	if entry.Kind == WorktreePrimary {
		m.status = entry.Title + " is the repository itself — it cannot be deleted"
		return nil
	}
	if m.tabIsOpenInCurrentSession(entry.Title) {
		m.status = entry.Title + " is open in this session — close the tab first"
		return nil
	}
	m.mode = modeConfirm
	m.status = ""
	m.confirm = confirmation{
		title:    entry.Title,
		project:  entry.Project,
		worktree: entry.Worktree,
		branch:   entry.Branch,
	}
	return nil
}

// handleConfirmKey answers the confirmation. Only the three deliberate keys act;
// anything else cancels, which is the right default for a prompt that deletes.
func (m *model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch msg.String() {
	case "y", "enter":
		return m, m.removeWorktreeCmd(false)
	case "b":
		return m, m.removeWorktreeCmd(true)
	}
	m.mode = modeTree
	m.status = ""
	return m, nil
}

func (m *model) removeWorktreeCmd(deleteBranch bool) tea.Cmd {
	target := m.confirm
	return func() tea.Msg {
		err := m.commander.RemoveWorktree(m.ctx, target.project, target.worktree, deleteBranch)
		return worktreeRemovedMsg{title: target.title, branchDeleted: deleteBranch && err == nil, err: err}
	}
}

// removalStatus describes the outcome. A failure still reloads the list at the
// call site, because deleting the branch can fail after the worktree is already
// gone and the rows on screen would otherwise be wrong.
func removalStatus(msg worktreeRemovedMsg, branch string) string {
	if msg.err != nil {
		return msg.err.Error()
	}
	if msg.branchDeleted {
		return fmt.Sprintf("deleted %s and branch %s", msg.title, branch)
	}
	return "deleted " + msg.title
}

func (m *model) confirmView() string {
	lines := []string{
		titleStyle.Render("delete worktree"),
		"",
		"  " + m.confirm.title,
		"  " + dimStyle.Render(m.confirm.worktree),
		"  " + dimStyle.Render("branch ") + m.confirm.branch,
		"",
		footerStyle.Render("y delete worktree · b delete worktree + branch · any other key cancels"),
	}
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
