package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The palette lets the dashboard run zwm's other commands without leaving it:
// `o` opens a project, `w` checks out a branch worktree (wco, with new-branch
// support), and `p` checks out a pull request (wpr). Each is a filterable picker;
// the branch/PR flows first pick a project, then the branch/PR within it.

type uiMode int

const (
	modeTree uiMode = iota
	modePicker
)

type pickerKind int

const (
	pickOpenProject pickerKind = iota
	pickBranchProject
	pickBranch
	pickPRProject
	pickPR
)

type pickerItem struct {
	label string
	value string
	isNew bool // pickBranch: the synthetic "create branch <filter>" entry (wco -b)
}

type picker struct {
	title    string
	kind     pickerKind
	project  string // context for pickBranch / pickPR
	items    []pickerItem
	loading  bool
	filter   string
	cursor   int
	offset   int
	allowNew bool // pickBranch: offer creating a new branch from the filter text
}

// --- messages ---

type pickerItemsMsg struct {
	gen   int
	items []pickerItem
}

type commandDoneMsg struct{ err error }

// --- opening pickers & loading their items ---

func (m *model) openPicker(kind pickerKind, title, project string, allowNew bool) tea.Cmd {
	m.mode = modePicker
	m.pickerGen++
	m.pick = picker{title: title, kind: kind, project: project, allowNew: allowNew, loading: true}
	m.status = ""
	return m.loadPickerItemsCmd(kind, project, m.pickerGen)
}

func (m *model) loadPickerItemsCmd(kind pickerKind, project string, gen int) tea.Cmd {
	return func() tea.Msg {
		var items []pickerItem
		switch kind {
		case pickOpenProject, pickBranchProject, pickPRProject:
			for _, name := range m.commander.Projects(m.ctx) {
				items = append(items, pickerItem{label: name, value: name})
			}
		case pickBranch:
			for _, branch := range m.commander.Branches(m.ctx, project) {
				items = append(items, pickerItem{label: branch, value: branch})
			}
		case pickPR:
			// Completer entries are "<selector>:<description>"; jump on the selector.
			for _, entry := range m.commander.PullRequests(m.ctx, project) {
				selector, _, _ := strings.Cut(entry, ":")
				items = append(items, pickerItem{label: entry, value: selector})
			}
		}
		return pickerItemsMsg{gen: gen, items: items}
	}
}

// --- key handling ---

func (m *model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.mode = modeTree
		m.status = ""
		return m, nil
	case tea.KeyUp:
		m.pick.moveCursor(-1)
		return m, nil
	case tea.KeyDown:
		m.pick.moveCursor(1)
		return m, nil
	case tea.KeyEnter:
		return m, m.selectPickerItem(false)
	case tea.KeyCtrlF:
		// Force applies only to the pull-request flow (wpr --force).
		if m.pick.kind == pickPR {
			return m, m.selectPickerItem(true)
		}
		return m, nil
	case tea.KeyBackspace:
		if len(m.pick.filter) > 0 {
			m.pick.filter = m.pick.filter[:len(m.pick.filter)-1]
			m.pick.cursor = 0
		}
		return m, nil
	case tea.KeyRunes:
		m.pick.filter += string(msg.Runes)
		m.pick.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m *model) selectPickerItem(force bool) tea.Cmd {
	items := m.pick.visibleItems()
	if m.pick.cursor < 0 || m.pick.cursor >= len(items) {
		return nil
	}
	item := items[m.pick.cursor]
	project := m.pick.project

	switch m.pick.kind {
	case pickOpenProject:
		return m.runCommandCmd(func(ctx context.Context) error { return m.commander.Open(ctx, item.value) })
	case pickBranchProject:
		return m.openPicker(pickBranch, "wco: branch in "+item.value, item.value, true)
	case pickBranch:
		if item.isNew {
			return m.runCommandCmd(func(ctx context.Context) error { return m.commander.CheckoutNew(ctx, project, item.value) })
		}
		return m.runCommandCmd(func(ctx context.Context) error { return m.commander.CheckoutExisting(ctx, project, item.value) })
	case pickPRProject:
		return m.openPicker(pickPR, "wpr: pull request in "+item.value, item.value, false)
	case pickPR:
		return m.runCommandCmd(func(ctx context.Context) error { return m.commander.PullRequest(ctx, project, item.value, force) })
	}
	return nil
}

// runCommandCmd runs a zwm command (which creates/focuses the Zellij tab itself)
// and, on success, quits so the dashboard's floating pane closes onto it.
func (m *model) runCommandCmd(run func(context.Context) error) tea.Cmd {
	return func() tea.Msg { return commandDoneMsg{err: run(m.ctx)} }
}

// --- filtering ---

// visibleItems returns the filter-matched items, prepending a create-new-branch
// entry when the flow allows it and the filter matches no existing branch.
func (p picker) visibleItems() []pickerItem {
	if p.filter == "" && !p.allowNew {
		return p.items
	}
	lower := strings.ToLower(p.filter)
	matched := make([]pickerItem, 0, len(p.items))
	exact := false
	for _, item := range p.items {
		if p.filter == "" || strings.Contains(strings.ToLower(item.label), lower) {
			matched = append(matched, item)
		}
		if item.label == p.filter {
			exact = true
		}
	}
	if p.allowNew && p.filter != "" && !exact {
		create := pickerItem{label: "＋ create branch " + p.filter, value: p.filter, isNew: true}
		matched = append([]pickerItem{create}, matched...)
	}
	return matched
}

func (p *picker) moveCursor(delta int) {
	count := len(p.visibleItems())
	if count == 0 {
		p.cursor = 0
		return
	}
	p.cursor = clamp(p.cursor+delta, 0, count-1)
}

// --- view ---

func (m *model) pickerView() string {
	var out strings.Builder
	out.WriteString(titleStyle.Render(m.pick.title))
	out.WriteByte('\n')
	out.WriteString(dimStyle.Render("filter: ") + m.pick.filter + dimStyle.Render("▏"))
	out.WriteByte('\n')

	if m.pick.loading {
		out.WriteString(dimStyle.Render("  loading…"))
		out.WriteByte('\n')
		out.WriteString(m.pickerFooter())
		return out.String()
	}

	items := m.pick.visibleItems()
	if len(items) == 0 {
		out.WriteString(dimStyle.Render("  (no matches)"))
		out.WriteByte('\n')
		out.WriteString(m.pickerFooter())
		return out.String()
	}

	bodyHeight := max(m.height-4, 1)
	m.pick.offset = scrollOffset(m.pick.offset, m.pick.cursor, bodyHeight, len(items))
	end := clamp(m.pick.offset+bodyHeight, 0, len(items))
	for index := m.pick.offset; index < end; index++ {
		label := items[index].label
		if items[index].isNew {
			label = doneStyle.Render(label)
		}
		out.WriteString(gutter(index == m.pick.cursor) + label)
		out.WriteByte('\n')
	}
	out.WriteString(m.pickerFooter())
	return out.String()
}

func (m *model) pickerFooter() string {
	if m.status != "" {
		return errorStyle.Render(m.status)
	}
	hint := "type to filter · ↑/↓ move · enter select · esc back"
	if m.pick.kind == pickPR {
		hint = "type to filter · ↑/↓ move · enter checkout · ctrl+f force · esc back"
	}
	return footerStyle.Render(hint)
}

// scrollOffset keeps cursor within [offset, offset+height) with minimal movement.
func scrollOffset(offset, cursor, height, total int) int {
	if total <= height {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	return clamp(offset, 0, total-height)
}
