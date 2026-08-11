package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// refreshInterval paces the session tree and the agents panel. Every tick that
// needs a session's tabs costs one `zellij action query-tab-names` client, and
// that call is far from free on the other side: measured at ~250ms of Zellij
// server CPU on a ten-tab session, spent in the very plugin threads that also
// deliver the zwm-attn pipes. Polling every session every two seconds was enough
// to keep those threads permanently behind, and Zellij drops a CLI pipe that
// misses a 1s deadline — i.e. lost attention glyphs. So the tick is deliberately
// slow, and refreshDataCmd asks for tabs only where they are needed.
const refreshInterval = 5 * time.Second

// reviewInterval paces the review queue independently of the session tree. It
// costs a GitHub search (plus a staleness probe per checked-out pull request), so
// it refreshes far less often than the local-state tick.
const reviewInterval = 2 * time.Minute

// --- messages ---

type sessionsLoadedMsg struct{ sessions []SessionView }

type reviewsLoadedMsg struct{ reviews []ReviewView }

type reviewTickMsg struct{}

// browsedMsg reports a finished browser launch. Unlike the checkout commands it
// does not quit the dashboard: opening a pull request is something you may want
// to do to several rows in a row.
type browsedMsg struct {
	number string
	err    error
}

type sessionDataMsg struct {
	session string
	// tabs is nil both when the session has none and when the refresh skipped the
	// tab query; tabsLoaded tells the two apart, so a skipped query never blanks
	// the tabs already on screen.
	tabs       []TabView
	tabsLoaded bool
	agents     []AgentView
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
	tabs     []TabView
	agents   []AgentView
}

type selKind int

const (
	selSession selKind = iota
	selTab
	selAgent
	selReview
)

// selection points at a navigable row: a session header, a tab within one, an
// entry in the top agents panel, or a pull request in the review queue.
type selection struct {
	kind    selKind
	session int
	tab     int
	agent   int
	review  int
}

// agentEntry is one running agent in the top triage panel, flattened across all
// sessions and ordered by how much it wants attention.
type agentEntry struct {
	session  string
	tabTitle string
	paneID   string
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
	reviews  []ReviewView
	rows     []selection
	cursor   int
	offset   int

	// reviewsLoaded distinguishes "no review requests" from "not fetched yet", so
	// the section can say which.
	reviewsLoaded bool

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
	return tea.Batch(m.loadSessionsCmd(), m.loadReviewsCmd(), tickCmd(), reviewTickCmd())
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

// loadSessionDataCmd reloads one session. withTabs asks for the tab query, the
// expensive half; the agents come from the on-disk record store either way, and
// the tabs — when fetched — double as the live set that retires records whose tab
// has closed, so one query serves both.
func (m *model) loadSessionDataCmd(session string, withTabs bool) tea.Cmd {
	return func() tea.Msg {
		var tabs []TabView
		if withTabs {
			queried, err := m.source.Tabs(m.ctx, session)
			if err != nil {
				return errMsg{err}
			}
			tabs = queried
		}
		agents, err := m.source.Agents(m.ctx, session, tabs)
		if err != nil {
			return errMsg{err}
		}
		return sessionDataMsg{session: session, tabs: tabs, tabsLoaded: withTabs, agents: agents}
	}
}

func (m *model) jumpCmd(target JumpTarget) tea.Cmd {
	return func() tea.Msg {
		if err := m.jumper.JumpTo(m.ctx, target); err != nil {
			return errMsg{err}
		}
		return jumpDoneMsg{}
	}
}

// loadReviewsCmd fetches the review queue. A failure leaves the previous list in
// place and surfaces the error in the status line rather than blanking the
// section, because `gh` failing (offline, rate limited, not authenticated) says
// nothing about whether those review requests still exist.
func (m *model) loadReviewsCmd() tea.Cmd {
	return func() tea.Msg {
		reviews, err := m.source.Reviews(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return reviewsLoadedMsg{reviews}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func reviewTickCmd() tea.Cmd {
	return tea.Tick(reviewInterval, func(time.Time) tea.Msg { return reviewTickMsg{} })
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
	case reviewsLoadedMsg:
		m.reviews = msg.reviews
		sortReviews(m.reviews)
		m.reviewsLoaded = true
		m.rebuildRows()
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.loadSessionsCmd(), tickCmd())
	case reviewTickMsg:
		return m, tea.Batch(m.loadReviewsCmd(), reviewTickCmd())
	case browsedMsg:
		// Stay in the dashboard either way, so the queue is still there to work
		// through once the browser has the pull request.
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = fmt.Sprintf("opened #%s in your browser", msg.number)
		}
		return m, nil
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
	case "tab":
		m.cycleSection(1)
		return m, nil
	case "shift+tab":
		m.cycleSection(-1)
		return m, nil
	case "r":
		m.status = ""
		return m, tea.Batch(m.loadSessionsCmd(), m.loadReviewsCmd())
	case "enter":
		return m, m.activate()
	case "ctrl+f":
		// Force applies to review rows only: reset a stale worktree to the pull
		// request's latest remote state (wpr --force).
		return m, m.activateReview(true, false)
	case "a":
		// Hand the pull request under the cursor to a review agent.
		return m, m.activateReview(false, true)
	case "b":
		return m, m.browseReview()
	case "o":
		return m, m.openPicker(pickOpenProject, "o: open a project", "", false)
	case "w":
		return m, m.branchPickerForSelection()
	case "p":
		return m, m.prPickerForSelection()
	}
	return m, nil
}

// cycleSection moves the cursor to the first row of the next (or previous)
// non-empty section, so Tab hops between the agents panel, the review queue, and
// the session tree without scrolling through them.
func (m *model) cycleSection(delta int) {
	order := []selKind{selAgent, selReview, selSession}
	starts := make([]int, 0, len(order))
	for _, kind := range order {
		if index, ok := m.firstRowOfKind(kind); ok {
			starts = append(starts, index)
		}
	}
	if len(starts) == 0 {
		return
	}
	m.status = ""
	// Land on the section after (before) the one holding the cursor. Sections are
	// contiguous, so the current section is the last start at or below the cursor.
	current := 0
	for index, start := range starts {
		if start <= m.cursor {
			current = index
		}
	}
	next := ((current+delta)%len(starts) + len(starts)) % len(starts)
	m.cursor = starts[next]
}

func (m *model) firstRowOfKind(kind selKind) (int, bool) {
	for index, row := range m.rows {
		if row.kind == kind {
			return index, true
		}
	}
	return 0, false
}

// browseReview opens the pull request under the cursor on GitHub. It needs no
// local checkout, so unlike the other review actions it also works on rows whose
// repository is not cloned — often exactly the ones you want to look at in a
// browser rather than check out.
func (m *model) browseReview() tea.Cmd {
	row, ok := m.currentRow()
	if !ok || row.kind != selReview {
		return nil
	}
	review := m.reviews[row.review]
	m.status = ""
	return func() tea.Msg {
		err := m.commander.BrowsePullRequest(m.ctx, review.Repository, review.Number)
		return browsedMsg{number: review.Number, err: err}
	}
}

// activateReview runs the review row under the cursor. force resets a stale
// worktree; agent additionally starts a review agent in the resulting tab. Both
// are no-ops on any other row kind, so the shared ctrl+f / a bindings stay inert
// elsewhere.
func (m *model) activateReview(force, agent bool) tea.Cmd {
	row, ok := m.currentRow()
	if !ok || row.kind != selReview {
		return nil
	}
	review := m.reviews[row.review]
	if review.Project == "" {
		m.status = fmt.Sprintf("%s has no checkout under ~/code — clone it first", review.Repository)
		return nil
	}
	if agent {
		return m.runCommandCmd(func(ctx context.Context) error {
			return m.commander.ReviewPullRequest(ctx, review.Project, review.Repository, review.Number, force || review.Stale)
		})
	}
	return m.runCommandCmd(func(ctx context.Context) error {
		return m.commander.PullRequest(ctx, review.Project, review.Number, force)
	})
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
	// Always re-query, even with tabs already on hand: a collapsed session is not
	// refreshed, so what is held may be minutes old, and opening it is exactly the
	// moment the list has to be true. One query on a keypress is not worth caching
	// against.
	return m.loadSessionDataCmd(session.name, true)
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
		// Carry the agent's pane so the jump lands on the agent itself, not just
		// on the tab hosting it.
		return m.jumpTo(JumpTarget{Session: entry.session, Tab: entry.tabTitle, PaneID: entry.paneID})
	case selReview:
		// Plain open/reuse. Enter never discards local commits; ctrl+f is the
		// explicit opt-in for resetting a stale worktree.
		return m.activateReview(false, false)
	case selSession:
		return m.toggle()
	case selTab:
		session := m.sessions[row.session]
		return m.jumpTo(JumpTarget{Session: session.name, Tab: session.tabs[row.tab].Title})
	}
	return nil
}

// jumpTo focuses a target, guarding the two cases the Zellij 0.43.1 CLI can't
// do: an unknown tab, and a tab in another session.
func (m *model) jumpTo(target JumpTarget) tea.Cmd {
	if target.Tab == "" {
		m.status = "this agent's tab is unknown — cannot jump"
		return nil
	}
	if target.Session != m.current {
		m.status = fmt.Sprintf("cross-session jump to %q is not supported yet", target.Session)
		return nil
	}
	return m.jumpCmd(target)
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
			if msg.tabsLoaded {
				m.sessions[i].tabs = msg.tabs
			}
			m.sessions[i].agents = msg.agents
			m.buildAgents()
			m.rebuildRows()
			return
		}
	}
}

// refreshDataCmd reloads every non-exited session so the agents panel and the
// tree show live state (new attention, closed tabs) on each tick. All sessions
// load, not just expanded ones, because the panel spans them all — but the tab
// query, the one call that costs the Zellij server real work (see
// refreshInterval), is asked for only where its answer is used: a session whose
// tabs are on screen, or one holding agent records that may need retiring. A
// collapsed session with no records needs neither, and a record appearing in it
// still arrives from the store on the next tick, which then queries it too.
func (m *model) refreshDataCmd() tea.Cmd {
	var cmds []tea.Cmd
	for _, session := range m.sessions {
		if !session.exited {
			withTabs := session.expanded || len(session.agents) > 0
			cmds = append(cmds, m.loadSessionDataCmd(session.name, withTabs))
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
				paneID:   agent.PaneID,
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

// sortReviews groups the queue by repository, and within a repository puts the
// longest-waiting pull request first.
//
// GitHub's search returns "best-match" relevance for a query with no search
// terms, which is not an order worth showing, so the queue imposes its own.
// Ascending number is exactly creation order within one repository — numbers are
// handed out monotonically — so it needs no timestamp to mean "waiting longest".
func sortReviews(reviews []ReviewView) {
	sort.SliceStable(reviews, func(left, right int) bool {
		first, second := reviews[left], reviews[right]
		// Owners differ in casing across repositories, so fold it for grouping.
		if a, b := strings.ToLower(first.Repository), strings.ToLower(second.Repository); a != b {
			return a < b
		}
		// Numeric, so #286 sorts before #1083 rather than lexicographically after.
		return reviewNumber(first.Number) < reviewNumber(second.Number)
	})
}

// reviewNumber parses a pull request number for ordering. The GitHub client only
// emits digit-only numbers, so a parse failure sorts first rather than panicking.
func reviewNumber(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return number
}

// rebuildRows recomputes the navigable rows: the agents panel first (so it has
// priority for the cursor), then the review queue, then session headers and the
// tab rows of expanded sessions. Keeps the cursor in range.
func (m *model) rebuildRows() {
	rows := make([]selection, 0, len(m.agents)+len(m.reviews)+len(m.sessions))
	for agentIndex := range m.agents {
		rows = append(rows, selection{kind: selAgent, agent: agentIndex})
	}
	for reviewIndex := range m.reviews {
		rows = append(rows, selection{kind: selReview, review: reviewIndex})
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
