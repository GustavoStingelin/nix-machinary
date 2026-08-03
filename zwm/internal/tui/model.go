package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const refreshInterval = 2 * time.Second

// --- messages ---

type sessionsLoadedMsg struct{ sessions []SessionView }

type sessionDataMsg struct {
	session string
	tabs    []TabView
	agents  []AgentView
}

type tickMsg struct{}

type jumpDoneMsg struct{}

type errMsg struct{ err error }

// --- model ---

type sessionState struct {
	name     string
	current  bool
	exited   bool
	expanded bool
	loaded   bool
	tabs     []TabView
	agents   []AgentView
}

type selKind int

const (
	selSession selKind = iota
	selTab
	selAgent
)

// selection points at a navigable row: a session header, a tab within one, or an
// entry in the top agents panel.
type selection struct {
	kind    selKind
	session int
	tab     int
	agent   int
}

// agentEntry is one running agent in the top triage panel, flattened across all
// sessions and ordered by how much it wants attention.
type agentEntry struct {
	session  string
	tabTitle string
	label    string
	state    string
}

type model struct {
	ctx       context.Context
	source    Source
	jumper    Jumper
	commander Commander
	current   string

	sessions []sessionState
	agents   []agentEntry
	rows     []selection
	cursor   int
	offset   int

	mode      uiMode
	pick      picker
	pickerGen int

	width  int
	height int
	status string
	ready  bool
}

func newModel(ctx context.Context, source Source, jumper Jumper, commander Commander, current string) *model {
	return &model{ctx: ctx, source: source, jumper: jumper, commander: commander, current: current, height: 24, width: 80}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.loadSessionsCmd(), tickCmd())
}

// --- commands ---

func (m *model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.source.Sessions(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return sessionsLoadedMsg{sessions}
	}
}

func (m *model) loadSessionDataCmd(session string) tea.Cmd {
	return func() tea.Msg {
		tabs, err := m.source.Tabs(m.ctx, session)
		if err != nil {
			return errMsg{err}
		}
		agents, err := m.source.Agents(m.ctx, session)
		if err != nil {
			return errMsg{err}
		}
		return sessionDataMsg{session: session, tabs: tabs, agents: agents}
	}
}

func (m *model) jumpCmd(session, tab string) tea.Cmd {
	return func() tea.Msg {
		if err := m.jumper.JumpTo(m.ctx, session, tab); err != nil {
			return errMsg{err}
		}
		return jumpDoneMsg{}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// --- update ---

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case sessionsLoadedMsg:
		firstLoad := !m.ready
		m.mergeSessions(msg.sessions)
		m.ready = true
		if firstLoad {
			m.expandCurrentInitially()
		}
		return m, m.refreshDataCmd()
	case sessionDataMsg:
		m.applySessionData(msg)
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.loadSessionsCmd(), tickCmd())
	case jumpDoneMsg:
		return m, tea.Quit
	case pickerItemsMsg:
		if msg.gen == m.pickerGen && m.mode == modePicker {
			m.pick.items = msg.items
			m.pick.loading = false
			m.pick.cursor = 0
		}
		return m, nil
	case commandDoneMsg:
		if msg.err != nil {
			m.mode = modeTree
			m.status = msg.err.Error()
			return m, nil
		}
		return m, tea.Quit
	case errMsg:
		m.status = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
		if m.mode == modePicker {
			return m.handlePickerKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
		return m, nil
	case "down", "j":
		m.moveCursor(1)
		return m, nil
	case "left", "h":
		return m, m.collapse()
	case "right", "l":
		return m, m.expand()
	case " ":
		return m, m.toggle()
	case "r":
		m.status = ""
		return m, m.loadSessionsCmd()
	case "enter":
		return m, m.activate()
	case "o":
		return m, m.openPicker(pickOpenProject, "o: open a project", "", false)
	case "w":
		return m, m.branchPickerForSelection()
	case "p":
		return m, m.prPickerForSelection()
	}
	return m, nil
}

// selectedProject infers the project from the tab under the cursor: zwm names
// tabs "<project>", "<project>:<branch>", or "<project>:pr-<n>", so the prefix is
// the project. A session is a workspace that can hold tabs from several projects,
// so a session header yields no project and the caller falls back to a picker.
func (m *model) selectedProject() (string, bool) {
	row, ok := m.currentRow()
	if !ok {
		return "", false
	}
	var title string
	switch row.kind {
	case selTab:
		title = m.sessions[row.session].tabs[row.tab].Title
	case selAgent:
		title = m.agents[row.agent].tabTitle
	default:
		return "", false
	}
	project, _, _ := strings.Cut(title, ":")
	if project == "" {
		return "", false
	}
	return project, true
}

// branchPickerForSelection opens wco's branch picker scoped to the project under
// the cursor, falling back to the project picker only when nothing is selected.
func (m *model) branchPickerForSelection() tea.Cmd {
	if project, ok := m.selectedProject(); ok {
		return m.openPicker(pickBranch, "wco: branch in "+project, project, true)
	}
	return m.openPicker(pickBranchProject, "wco: pick a project", "", false)
}

// prPickerForSelection opens wpr's pull-request picker scoped to the project
// under the cursor, falling back to the project picker when nothing is selected.
func (m *model) prPickerForSelection() tea.Cmd {
	if project, ok := m.selectedProject(); ok {
		return m.openPicker(pickPR, "wpr: pull request in "+project, project, false)
	}
	return m.openPicker(pickPRProject, "wpr: pick a project", "", false)
}

func (m *model) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, len(m.rows)-1)
	m.status = ""
}

func (m *model) currentRow() (selection, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return selection{}, false
	}
	return m.rows[m.cursor], true
}

func (m *model) expand() tea.Cmd {
	row, ok := m.currentRow()
	if !ok || row.kind != selSession {
		return nil
	}
	session := &m.sessions[row.session]
	if session.exited {
		m.status = fmt.Sprintf("%q is exited — nothing to open", session.name)
		return nil
	}
	if session.expanded {
		return nil
	}
	session.expanded = true
	m.rebuildRows()
	if !session.loaded {
		return m.loadSessionDataCmd(session.name)
	}
	return nil
}

func (m *model) collapse() tea.Cmd {
	row, ok := m.currentRow()
	if !ok {
		return nil
	}
	switch row.kind {
	case selTab:
		// Collapsing from a tab row returns focus to its session header.
		m.sessions[row.session].expanded = false
		m.rebuildRows()
		m.selectSession(row.session)
	case selSession:
		if m.sessions[row.session].expanded {
			m.sessions[row.session].expanded = false
			m.rebuildRows()
		}
	}
	return nil
}

func (m *model) toggle() tea.Cmd {
	row, ok := m.currentRow()
	if !ok || row.kind != selSession {
		return nil
	}
	if m.sessions[row.session].expanded {
		return m.collapse()
	}
	return m.expand()
}

func (m *model) activate() tea.Cmd {
	row, ok := m.currentRow()
	if !ok {
		return nil
	}
	switch row.kind {
	case selAgent:
		entry := m.agents[row.agent]
		return m.jumpTo(entry.session, entry.tabTitle)
	case selSession:
		return m.toggle()
	case selTab:
		session := m.sessions[row.session]
		return m.jumpTo(session.name, session.tabs[row.tab].Title)
	}
	return nil
}

// jumpTo focuses a tab, guarding the two cases the Zellij 0.43.1 CLI can't do:
// an unknown tab, and a tab in another session.
func (m *model) jumpTo(session, tab string) tea.Cmd {
	if tab == "" {
		m.status = "this agent's tab is unknown — cannot jump"
		return nil
	}
	if session != m.current {
		m.status = fmt.Sprintf("cross-session jump to %q is not supported yet", session)
		return nil
	}
	return m.jumpCmd(session, tab)
}

// mergeSessions reconciles a fresh session list into the model, preserving the
// expanded/loaded/data state of sessions that still exist so a periodic refresh
// never collapses the user's view.
func (m *model) mergeSessions(fresh []SessionView) {
	previous := make(map[string]sessionState, len(m.sessions))
	for _, session := range m.sessions {
		previous[session.name] = session
	}
	next := make([]sessionState, 0, len(fresh))
	for _, view := range fresh {
		state := sessionState{name: view.Name, current: view.Current, exited: view.Exited}
		if prior, ok := previous[view.Name]; ok {
			state.expanded = prior.expanded
			state.loaded = prior.loaded
			state.tabs = prior.tabs
			state.agents = prior.agents
		}
		next = append(next, state)
	}
	// Order for easy navigation: current session first, then other running
	// sessions, then exited ones. Stable so creation order holds within a group.
	sort.SliceStable(next, func(i, j int) bool {
		return sessionRank(next[i]) < sessionRank(next[j])
	})
	m.sessions = next
	m.buildAgents()
	m.rebuildRows()
}

func sessionRank(session sessionState) int {
	switch {
	case session.current:
		return 0
	case session.exited:
		return 2
	default:
		return 1
	}
}

// expandCurrentInitially opens the current session so its tabs show on load. The
// cursor stays at the top, where the agents panel takes priority.
func (m *model) expandCurrentInitially() {
	for i := range m.sessions {
		if m.sessions[i].current {
			m.sessions[i].expanded = true
			m.rebuildRows()
			return
		}
	}
}

func (m *model) applySessionData(msg sessionDataMsg) {
	for i := range m.sessions {
		if m.sessions[i].name == msg.session {
			m.sessions[i].tabs = msg.tabs
			m.sessions[i].agents = msg.agents
			m.sessions[i].loaded = true
			m.buildAgents()
			m.rebuildRows()
			return
		}
	}
}

// refreshDataCmd reloads tabs+agents for every non-exited session so the agents
// panel and the tree show live state (new attention, closed tabs) on each tick.
// All sessions load, not just expanded ones, because the panel spans them all.
func (m *model) refreshDataCmd() tea.Cmd {
	var cmds []tea.Cmd
	for _, session := range m.sessions {
		if !session.exited {
			cmds = append(cmds, m.loadSessionDataCmd(session.name))
		}
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	default:
		return tea.Batch(cmds...)
	}
}

// buildAgents flattens every non-exited session's agents into the triage panel,
// ordered by attention: waiting for you, then working, then done.
func (m *model) buildAgents() {
	entries := make([]agentEntry, 0)
	for _, session := range m.sessions {
		if session.exited {
			continue
		}
		for _, agent := range session.agents {
			label := agent.Agent
			if label == "" {
				label = "pane " + agent.PaneID
			}
			entries = append(entries, agentEntry{
				session:  session.name,
				tabTitle: agent.TabTitle,
				label:    label,
				state:    agent.State,
			})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if ri, rj := panelRank(entries[i].state), panelRank(entries[j].state); ri != rj {
			return ri < rj
		}
		return entries[i].tabTitle < entries[j].tabTitle
	})
	m.agents = entries
}

// panelRank orders the agents panel: waiting for you first, then working, then
// done (finished but unreviewed).
func panelRank(state string) int {
	switch state {
	case StateWaiting:
		return 0
	case StateWorking:
		return 1
	case StateDone:
		return 2
	default:
		return 3
	}
}

// rebuildRows recomputes the navigable rows: the agents panel first (so it has
// priority for the cursor), then session headers and the tab rows of expanded
// sessions. Keeps the cursor in range.
func (m *model) rebuildRows() {
	rows := make([]selection, 0, len(m.agents)+len(m.sessions))
	for agentIndex := range m.agents {
		rows = append(rows, selection{kind: selAgent, agent: agentIndex})
	}
	for sessionIndex, session := range m.sessions {
		rows = append(rows, selection{kind: selSession, session: sessionIndex})
		if session.expanded {
			for tabIndex := range session.tabs {
				rows = append(rows, selection{kind: selTab, session: sessionIndex, tab: tabIndex})
			}
		}
	}
	m.rows = rows
	if m.cursor >= len(rows) {
		m.cursor = max(0, len(rows)-1)
	}
}

func (m *model) selectSession(sessionIndex int) {
	for i, row := range m.rows {
		if row.kind == selSession && row.session == sessionIndex {
			m.cursor = i
			return
		}
	}
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
