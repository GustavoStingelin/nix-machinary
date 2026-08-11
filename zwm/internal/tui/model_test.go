package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	sessions  []SessionView
	tabs      map[string][]TabView
	agents    map[string][]AgentView
	reviews   []ReviewView
	reviewErr error
	recents   []RecentView
	recentErr error
	cached    []ReviewView
	cachedAt  time.Time
	cachedOK  bool
	// tabCalls, when set, records every session whose tabs were queried, in
	// order. It is a pointer so the value receivers below can still append.
	tabCalls *[]string
}

func (source fakeSource) Reviews(context.Context) ([]ReviewView, error) {
	return source.reviews, source.reviewErr
}

func (source fakeSource) CachedReviews(context.Context) ([]ReviewView, time.Time, bool) {
	return source.cached, source.cachedAt, source.cachedOK
}

func (source fakeSource) Sessions(context.Context) ([]SessionView, error) {
	return source.sessions, nil
}

func (source fakeSource) Tabs(_ context.Context, session string) ([]TabView, error) {
	if source.tabCalls != nil {
		*source.tabCalls = append(*source.tabCalls, session)
	}
	return source.tabs[session], nil
}

// Agents mirrors the real source: it answers from its own records and only uses
// liveTabs to drop records whose tab is gone, so a call without tabs returns
// everything it holds.
func (source fakeSource) Agents(_ context.Context, session string, liveTabs []TabView) ([]AgentView, error) {
	if len(liveTabs) == 0 {
		return source.agents[session], nil
	}
	live := make(map[string]struct{}, len(liveTabs))
	for _, tab := range liveTabs {
		live[tab.Title] = struct{}{}
	}
	kept := make([]AgentView, 0, len(source.agents[session]))
	for _, agent := range source.agents[session] {
		if _, ok := live[agent.TabTitle]; ok || agent.TabTitle == "" {
			kept = append(kept, agent)
		}
	}
	return kept, nil
}

// Recent answers from a fixed list; the model decides what Enter does with each
// row, which is what the tests below exercise.
func (source fakeSource) Recent(context.Context) ([]RecentView, error) {
	return source.recents, source.recentErr
}

type jumpCall struct{ session, tab, paneID string }

type fakeJumper struct {
	calls []jumpCall
	err   error
}

func (jumper *fakeJumper) JumpTo(_ context.Context, target JumpTarget) error {
	jumper.calls = append(jumper.calls, jumpCall{target.Session, target.Tab, target.PaneID})
	return jumper.err
}

type commandCall struct {
	op, project, arg string
	repository       string
	force            bool
}

type fakeCommander struct {
	projects []string
	branches map[string][]string
	prs      map[string][]string
	calls    []commandCall
	err      error
}

func (commander *fakeCommander) Projects(context.Context) []string { return commander.projects }
func (commander *fakeCommander) Branches(_ context.Context, project string) []string {
	return commander.branches[project]
}
func (commander *fakeCommander) PullRequests(_ context.Context, project string) []string {
	return commander.prs[project]
}
func (commander *fakeCommander) Open(_ context.Context, project string) error {
	commander.calls = append(commander.calls, commandCall{op: "open", project: project})
	return commander.err
}
func (commander *fakeCommander) CheckoutExisting(_ context.Context, project, branch string) error {
	commander.calls = append(commander.calls, commandCall{op: "wco", project: project, arg: branch})
	return commander.err
}
func (commander *fakeCommander) CheckoutNew(_ context.Context, project, branch string) error {
	commander.calls = append(commander.calls, commandCall{op: "wco-new", project: project, arg: branch})
	return commander.err
}
func (commander *fakeCommander) PullRequest(_ context.Context, project, selector string, force bool) error {
	commander.calls = append(commander.calls, commandCall{op: "wpr", project: project, arg: selector, force: force})
	return commander.err
}
func (commander *fakeCommander) ReviewPullRequest(_ context.Context, project, repository, selector string, force bool) error {
	commander.calls = append(commander.calls, commandCall{
		op: "review", project: project, repository: repository, arg: selector, force: force,
	})
	return commander.err
}
func (commander *fakeCommander) BrowsePullRequest(_ context.Context, repository, selector string) error {
	commander.calls = append(commander.calls, commandCall{op: "browse", repository: repository, arg: selector})
	return commander.err
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send routes a message through Update and drains any single follow-up command
// so the model reaches a settled state, mirroring what the tea runtime does.
func send(t *testing.T, m *model, msg tea.Msg) {
	t.Helper()
	_, cmd := m.Update(msg)
	if cmd == nil {
		return
	}
	if next := cmd(); next != nil {
		m.Update(next)
	}
}

func newTestModel() (*model, *fakeJumper) {
	source := fakeSource{
		// A "bitcoin" workspace session holding project tabs (btcwallet root +
		// a worktree tab), plus an unordered open session and an exited one.
		sessions: []SessionView{
			{Name: "nix", Current: false},
			{Name: "bitcoin", Current: true},
			{Name: "stale", Exited: true},
		},
		tabs: map[string][]TabView{
			"bitcoin": {
				{Title: "btcwallet"},
				{Title: "btcwallet:itests/very-first-itests", NeedsAttention: true},
			},
			"nix": {{Title: "nix-machinary"}},
		},
		agents: map[string][]AgentView{
			"bitcoin": {
				// opencode runs in the worktree tab; claude's tab is unknown.
				{Agent: "opencode", PaneID: "5", TabTitle: "btcwallet:itests/very-first-itests", State: StateWorking},
				{Agent: "claude", PaneID: "3", TabTitle: "", State: StateWaiting},
			},
			"nix": {{Agent: "opencode", PaneID: "1", TabTitle: "nix-machinary", State: StateWorking}},
		},
	}
	jumper := &fakeJumper{}
	commander := &fakeCommander{
		projects: []string{"bitcoin", "nix-machinary", "btcwallet"},
		branches: map[string][]string{"btcwallet": {"main", "itests/very-first-itests"}},
		prs:      map[string][]string{"btcwallet": {"123:fix the thing", "124:another"}},
	}
	return newModel(context.Background(), source, jumper, commander, "bitcoin"), jumper
}

// runBatch executes a command and every command it batches, feeding each
// resulting message back through Update. send stops at the first follow-up,
// which is enough for a single command but drops a tea.Batch on the floor.
func runBatch(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			runBatch(t, m, sub)
		}
		return
	}
	m.Update(msg)
}

func loadData(t *testing.T, m *model, session string) {
	t.Helper()
	send(t, m, m.loadSessionDataCmd(session, true)())
}

func loaded(t *testing.T) (*model, *fakeJumper) {
	t.Helper()
	m, jumper := newTestModel()
	send(t, m, sessionsLoadedMsg{sessions: m.source.(fakeSource).sessions})
	// The runtime batches data loads for all non-exited sessions; drive them
	// explicitly so the agents panel is populated.
	loadData(t, m, "bitcoin")
	loadData(t, m, "nix")
	return m, jumper
}

func focusSession(t *testing.T, m *model, name string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selSession && m.sessions[row.session].name == name {
			m.cursor = i
			return
		}
	}
	t.Fatalf("session %q not among rows", name)
}

func focusTab(t *testing.T, m *model, title string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selTab && m.sessions[row.session].tabs[row.tab].Title == title {
			m.cursor = i
			return
		}
	}
	t.Fatalf("tab %q not among rows (expand its session first)", title)
}

func focusAgent(t *testing.T, m *model, label, tabTitle string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selAgent && m.agents[row.agent].label == label && m.agents[row.agent].tabTitle == tabTitle {
			m.cursor = i
			return
		}
	}
	t.Fatalf("agent %q@%q not among rows", label, tabTitle)
}

func TestModel_orders_current_first_then_open_then_exited(t *testing.T) {
	m, _ := loaded(t)
	require.Equal(t, "bitcoin", m.sessions[0].name)
	require.True(t, m.sessions[0].current)
	require.Equal(t, "nix", m.sessions[1].name)
	require.Equal(t, "stale", m.sessions[2].name)
	require.True(t, m.sessions[2].exited)
}

func TestModel_auto_expands_the_current_session_on_load(t *testing.T) {
	m, _ := loaded(t)

	// Current session is expanded and its data is loaded without any keypress.
	require.True(t, m.sessions[0].expanded)
	view := m.View()
	require.Contains(t, view, "(current)")
	require.Contains(t, view, "btcwallet")       // tab
	require.Contains(t, view, "opencode")        // agent
	require.Contains(t, view, "working")         // three-way state
	require.Contains(t, view, "waiting for you") // claude, session-level
	require.Contains(t, view, "(exited)")        // stale session shown, display-only
}

// The tab query is the dashboard's only expensive call (it costs the Zellij
// server real CPU in the threads that also carry the attention pipes), so a
// refresh must ask for it session by session, and only where the answer is used.
func TestRefresh_queries_tabs_only_for_expanded_sessions_and_sessions_with_agents(t *testing.T) {
	var queried []string
	source := fakeSource{
		sessions: []SessionView{
			{Name: "bitcoin", Current: true},
			{Name: "nix"},
			{Name: "idle"},
			{Name: "stale", Exited: true},
		},
		tabs: map[string][]TabView{
			"bitcoin": {{Title: "btcwallet"}},
			"nix":     {{Title: "nix-machinary"}},
			"idle":    {{Title: "scratch"}},
		},
		agents: map[string][]AgentView{
			"nix": {{Agent: "opencode", PaneID: "1", TabTitle: "nix-machinary", State: StateWorking}},
		},
		tabCalls: &queried,
	}
	m := newModel(context.Background(), source, &fakeJumper{}, &fakeCommander{}, "bitcoin")

	// First refresh: only the auto-expanded current session has tabs on screen,
	// and no agent records are known yet — those arrive from the store with this
	// very refresh, without a tab query of their own.
	_, cmd := m.Update(sessionsLoadedMsg{sessions: source.sessions})
	runBatch(t, m, cmd)
	require.Equal(t, []string{"bitcoin"}, queried)
	require.Len(t, m.sessions[1].agents, 1, "nix's agent came from the record store")

	// Second refresh: nix now holds a record, so its tabs are needed to retire it
	// should the tab be gone. "idle" is collapsed and has none, and "stale" has
	// exited — neither is ever queried.
	queried = nil
	runBatch(t, m, m.refreshDataCmd())
	require.ElementsMatch(t, []string{"bitcoin", "nix"}, queried)
}

func TestExpand_requeries_the_tabs_of_a_collapsed_session(t *testing.T) {
	var queried []string
	tabs := map[string][]TabView{
		"bitcoin": {{Title: "btcwallet"}},
		"nix":     {{Title: "nix-machinary"}},
	}
	source := fakeSource{
		sessions: []SessionView{{Name: "bitcoin", Current: true}, {Name: "nix"}},
		tabs:     tabs,
		tabCalls: &queried,
	}
	m := newModel(context.Background(), source, &fakeJumper{}, &fakeCommander{}, "bitcoin")
	_, cmd := m.Update(sessionsLoadedMsg{sessions: source.sessions})
	runBatch(t, m, cmd)

	// Open and close "nix" so its tabs are on hand, then let them go stale: while
	// collapsed it is skipped by every refresh, so re-opening it is the moment its
	// tabs have to be true again — held tabs are no reason to skip the query.
	focusSession(t, m, "nix")
	send(t, m, key("right"))
	send(t, m, key("left"))
	focusSession(t, m, "nix")
	tabs["nix"] = []TabView{{Title: "nix-machinary:zjstatus"}}
	queried = nil
	send(t, m, key("right"))

	require.Equal(t, []string{"nix"}, queried)
	require.Contains(t, m.View(), "nix-machinary:zjstatus")
}

func TestModel_places_agent_under_its_tab_and_unknown_at_session_level(t *testing.T) {
	m, _ := loaded(t)
	// Look only at the sessions section, below the panel separator, so the
	// panel's own copies of the agents don't confuse the ordering check.
	_, sessions, found := strings.Cut(m.View(), "sessions ───")
	require.True(t, found)
	lines := strings.Split(sessions, "\n")

	indexOf := func(needle string) int {
		for i, line := range lines {
			if strings.Contains(line, needle) {
				return i
			}
		}
		return -1
	}

	worktreeTab := indexOf("very-first-itests")
	opencode := indexOf("opencode")
	claude := indexOf("claude")
	firstTab := indexOf("btcwallet")

	// opencode renders on the line immediately after its worktree tab.
	require.Equal(t, worktreeTab+1, opencode)
	// claude (unknown tab) renders at session level, above the first tab row.
	require.Less(t, claude, firstTab)
}

func TestModel_enter_on_a_current_session_tab_jumps(t *testing.T) {
	m, jumper := loaded(t)
	focusTab(t, m, "btcwallet")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, jumpDoneMsg{}, cmd())
	require.Equal(t, []jumpCall{{session: "bitcoin", tab: "btcwallet"}}, jumper.calls)
}

func TestModel_enter_on_a_remote_session_tab_shows_a_hint_and_does_not_jump(t *testing.T) {
	m, jumper := loaded(t)

	focusSession(t, m, "nix")
	send(t, m, key("right")) // expand nix
	focusTab(t, m, "nix-machinary")
	send(t, m, key("enter"))

	require.Empty(t, jumper.calls)
	require.Contains(t, strings.ToLower(m.status), "not supported")
}

func typeString(t *testing.T, m *model, s string) {
	t.Helper()
	for _, r := range s {
		send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestPalette_open_project_filters_and_runs_open(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	send(t, m, key("o")) // open the project picker (items load via the follow-up cmd)
	require.Equal(t, modePicker, m.mode)
	typeString(t, m, "btc") // narrows to "btcwallet"

	items := m.pick.visibleItems()
	require.Len(t, items, 1)
	require.Equal(t, "btcwallet", items[0].value)

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())
	require.Equal(t, []commandCall{{op: "open", project: "btcwallet"}}, commander.calls)
}

func TestPalette_wco_scopes_to_the_tab_project_and_creates_a_new_branch(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	focusTab(t, m, "btcwallet")
	send(t, m, key("w")) // scoped to btcwallet, no project prompt
	require.Equal(t, pickBranch, m.pick.kind)
	require.Equal(t, "btcwallet", m.pick.project)

	// A filter that matches no existing branch offers a create entry at the top.
	typeString(t, m, "feature/new")
	items := m.pick.visibleItems()
	require.True(t, items[0].isNew)

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())
	require.Equal(t, []commandCall{{op: "wco-new", project: "btcwallet", arg: "feature/new"}}, commander.calls)
}

func TestPalette_wco_checks_out_an_existing_branch_in_the_tab_project(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	focusTab(t, m, "btcwallet")
	send(t, m, key("w"))
	typeString(t, m, "main")
	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wco", project: "btcwallet", arg: "main"}}, commander.calls)
}

func TestPalette_wpr_scopes_to_the_tab_project(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	focusTab(t, m, "btcwallet")
	send(t, m, key("p"))
	require.Equal(t, pickPR, m.pick.kind)
	require.Equal(t, "btcwallet", m.pick.project)
	// First PR entry "123:fix the thing" -> selector "123".
	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "123"}}, commander.calls)
}

func TestPalette_wpr_ctrl_f_forces_the_checkout(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	focusTab(t, m, "btcwallet")
	send(t, m, key("p")) // PR picker

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF}) // force-checkout the selected PR
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "123", force: true}}, commander.calls)
}

func TestPalette_w_on_a_session_header_falls_back_to_the_project_picker(t *testing.T) {
	m, _ := loaded(t)

	// The bitcoin session header is a workspace, not a project, so `w` opens the
	// project picker rather than guessing.
	focusSession(t, m, "bitcoin")
	send(t, m, key("w"))
	require.Equal(t, modePicker, m.mode)
	require.Equal(t, pickBranchProject, m.pick.kind)
}

func TestSelectedProject_derives_from_a_tab_title_only(t *testing.T) {
	m, _ := loaded(t)

	// A session header is a workspace, not a project.
	focusSession(t, m, "bitcoin")
	_, ok := m.selectedProject()
	require.False(t, ok)

	// A "<project>:<branch>" tab yields its project prefix.
	focusTab(t, m, "btcwallet:itests/very-first-itests")
	project, ok := m.selectedProject()
	require.True(t, ok)
	require.Equal(t, "btcwallet", project)
}

func TestModel_agents_panel_orders_by_waiting_then_working_then_done(t *testing.T) {
	m, _ := loaded(t)

	require.NotEmpty(t, m.agents)
	require.Equal(t, StateWaiting, m.agents[0].state)
	require.Equal(t, "claude", m.agents[0].label)
	for i := 1; i < len(m.agents); i++ {
		require.LessOrEqual(t, panelRank(m.agents[i-1].state), panelRank(m.agents[i].state))
	}
}

func TestModel_agents_panel_has_cursor_priority_on_load(t *testing.T) {
	m, _ := loaded(t)
	row, ok := m.currentRow()
	require.True(t, ok)
	require.Equal(t, selAgent, row.kind)
}

func TestModel_enter_on_a_panel_agent_jumps_to_its_pane_in_the_current_session(t *testing.T) {
	m, jumper := loaded(t)
	focusAgent(t, m, "opencode", "btcwallet:itests/very-first-itests")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, jumpDoneMsg{}, cmd())
	// The agent's pane rides along, so the jump lands on the agent, not just on
	// the tab hosting it.
	require.Equal(t, []jumpCall{{
		session: "bitcoin",
		tab:     "btcwallet:itests/very-first-itests",
		paneID:  "5",
	}}, jumper.calls)
}

func TestModel_enter_on_a_panel_agent_in_another_session_hints(t *testing.T) {
	m, jumper := loaded(t)
	focusAgent(t, m, "opencode", "nix-machinary") // nix is not the current session

	send(t, m, key("enter"))
	require.Empty(t, jumper.calls)
	require.Contains(t, strings.ToLower(m.status), "not supported")
}

func TestModel_enter_on_a_panel_agent_with_unknown_tab_hints(t *testing.T) {
	m, jumper := loaded(t)
	focusAgent(t, m, "claude", "") // unknown tab

	send(t, m, key("enter"))
	require.Empty(t, jumper.calls)
	require.Contains(t, strings.ToLower(m.status), "unknown")
}

func TestPalette_esc_returns_to_the_tree(t *testing.T) {
	m, _ := loaded(t)
	send(t, m, key("o"))
	require.Equal(t, modePicker, m.mode)
	send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.Equal(t, modeTree, m.mode)
}

func TestModel_exited_session_cannot_be_expanded(t *testing.T) {
	m, _ := loaded(t)

	focusSession(t, m, "stale")
	row, ok := m.currentRow()
	require.True(t, ok)
	require.True(t, m.sessions[row.session].exited)

	send(t, m, key("right"))
	require.False(t, m.sessions[row.session].expanded)
	require.Contains(t, strings.ToLower(m.status), "exited")
}

// --- review queue ---

func reviewFixtures() []ReviewView {
	return []ReviewView{
		// Stacked onto another branch, checked out locally and behind the remote.
		{Number: "1305", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: define watch-only and script policy", Author: "yyforyongyu",
			Base: "itests/watch-only-create", Head: "task-watch-policy",
			Worktree: "/wt/btcwallet/pr-1305", Stale: true},
		// Cloned locally but not checked out as a worktree yet.
		{Number: "3630", Repository: "lightninglabs/lightning-infra", Project: "lightning-infra",
			Title: "lumosd: raise CPU limits", Author: "Roasbeef", Base: "main", Head: "cpu-limits"},
		// Review requested on a repository with no local clone at all. Refs are
		// still fetched for these, so the branches show even though nothing local
		// can be opened.
		{Number: "77", Repository: "someone/not-cloned", Title: "a change", Author: "nobody",
			Base: "trunk", Head: "some-fix"},
	}
}

func withReviews(t *testing.T) (*model, *fakeCommander) {
	t.Helper()
	m, _ := loaded(t)
	send(t, m, reviewsLoadedMsg{reviews: reviewFixtures()})
	return m, m.commander.(*fakeCommander)
}

func focusReview(t *testing.T, m *model, number string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selReview && m.reviews[row.review].Number == number {
			m.cursor = i
			return
		}
	}
	t.Fatalf("review %s is not a navigable row", number)
}

func TestReviewQueue_renders_base_branch_and_local_state(t *testing.T) {
	m, _ := withReviews(t)

	view := m.View()
	require.Contains(t, view, "review queue")
	// The base branch is shown because it decides what the review actually diffs.
	require.Contains(t, view, "itests/watch-only-create")
	require.Contains(t, view, "stale")
	require.Contains(t, view, "(not cloned)")
}

func TestReviewQueue_enter_checks_out_without_force(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "1305")

	send(t, m, m.activate()())

	// Enter must never discard local commits, even on a stale worktree.
	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1305"}}, commander.calls)
}

func TestReviewQueue_force_key_resets_the_worktree(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "1305")

	send(t, m, m.activateReview(true, false)())

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1305", force: true}}, commander.calls)
}

func TestReviewQueue_agent_key_passes_the_repository_and_forces_when_stale(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "1305")

	send(t, m, m.activateReview(false, true)())

	// The repository travels with the request (the queue spans repositories), and a
	// stale worktree is reset first so the agent never reviews outdated code.
	require.Equal(t, []commandCall{{
		op: "review", project: "btcwallet", repository: "btcsuite/btcwallet", arg: "1305", force: true,
	}}, commander.calls)
}

func TestReviewQueue_agent_key_does_not_force_a_fresh_checkout(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "3630")

	send(t, m, m.activateReview(false, true)())

	require.Equal(t, []commandCall{{
		op: "review", project: "lightning-infra", repository: "lightninglabs/lightning-infra", arg: "3630",
	}}, commander.calls)
}

func TestReviewQueue_refuses_a_repository_with_no_local_checkout(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "77")

	cmd := m.activate()

	require.Nil(t, cmd)
	require.Empty(t, commander.calls)
	require.Contains(t, strings.ToLower(m.status), "no checkout")
}

func TestReviewQueue_tab_cycles_between_sections(t *testing.T) {
	m, _ := withReviews(t)
	m.cursor = 0
	require.Equal(t, selAgent, m.rows[m.cursor].kind)

	m.cycleSection(1)
	require.Equal(t, selReview, m.rows[m.cursor].kind)
	m.cycleSection(1)
	require.Equal(t, selSession, m.rows[m.cursor].kind)
	m.cycleSection(1)
	require.Equal(t, selAgent, m.rows[m.cursor].kind, "wraps back to the first section")
	m.cycleSection(-1)
	require.Equal(t, selSession, m.rows[m.cursor].kind, "shift+tab goes backwards")
}

func TestReviewQueue_empty_queue_is_distinguished_from_not_yet_loaded(t *testing.T) {
	m, _ := loaded(t)
	require.Contains(t, m.View(), "loading…")

	send(t, m, reviewsLoadedMsg{reviews: nil})

	require.Contains(t, m.View(), "nothing waiting on you")
}

func TestReviewQueue_shows_head_and_base_branches(t *testing.T) {
	m, _ := withReviews(t)

	view := m.View()
	// Both ends of the range, so it is obvious what the review spans — for a
	// stacked pull request the base is the branch below it, not master.
	require.Contains(t, view, "task-watch-policy → itests/watch-only-create")
	require.Contains(t, view, "cpu-limits → main")
}

func TestReviewBranches_degrades_when_refs_are_unavailable(t *testing.T) {
	require.Equal(t, "  head → base", reviewBranches(ReviewView{Head: "head", Base: "base"}))
	require.Equal(t, "  → base", reviewBranches(ReviewView{Base: "base"}))
	require.Equal(t, "  head → ?", reviewBranches(ReviewView{Head: "head"}))
	require.Equal(t, "", reviewBranches(ReviewView{}))
}

func TestReviewQueue_browse_key_opens_the_pull_request_and_stays_open(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "1305")

	send(t, m, m.browseReview()())

	require.Equal(t, []commandCall{{op: "browse", repository: "btcsuite/btcwallet", arg: "1305"}}, commander.calls)
	// Opening a browser must not tear down the dashboard: the queue is still
	// there to work through.
	require.Equal(t, modeTree, m.mode)
	require.Contains(t, m.status, "#1305")
}

func TestReviewQueue_browse_works_without_a_local_checkout(t *testing.T) {
	m, commander := withReviews(t)
	focusReview(t, m, "77")

	send(t, m, m.browseReview()())

	// Unlike checkout, browsing needs nothing local — these are often exactly the
	// rows you want to look at in a browser.
	require.Equal(t, []commandCall{{op: "browse", repository: "someone/not-cloned", arg: "77"}}, commander.calls)
	require.NotContains(t, m.status, "no checkout")
}

func TestReviewQueue_browse_reports_failure_without_quitting(t *testing.T) {
	m, commander := withReviews(t)
	commander.err = errBrowse
	focusReview(t, m, "1305")

	send(t, m, m.browseReview()())

	require.Equal(t, modeTree, m.mode)
	require.Contains(t, m.status, "no browser")
}

func TestReviewQueue_browse_is_inert_outside_the_review_section(t *testing.T) {
	m, commander := withReviews(t)
	focusSession(t, m, "bitcoin")

	require.Nil(t, m.browseReview())
	require.Empty(t, commander.calls)
}

var errBrowse = errors.New("no browser available")

func TestReviewQueue_shows_branches_even_when_the_repository_is_not_cloned(t *testing.T) {
	m, _ := withReviews(t)

	view := m.View()
	require.Contains(t, view, "some-fix → trunk")
	require.Contains(t, view, "(not cloned)")
}

func TestSortReviews_groups_by_repository_then_longest_waiting_first(t *testing.T) {
	reviews := []ReviewView{
		{Number: "1313", Repository: "btcsuite/btcwallet"},
		{Number: "3545", Repository: "lightninglabs/lightning-infra"},
		{Number: "286", Repository: "btcsuite/btcd"},
		{Number: "1083", Repository: "btcsuite/btcwallet"},
		{Number: "3709", Repository: "lightninglabs/lightning-infra"},
		{Number: "285", Repository: "btcsuite/btcd"},
	}

	sortReviews(reviews)

	// Repositories grouped alphabetically; within each, the oldest pull request
	// (lowest number, since numbers are monotonic per repository) comes first.
	got := make([]string, 0, len(reviews))
	for _, review := range reviews {
		got = append(got, review.Repository+"#"+review.Number)
	}
	require.Equal(t, []string{
		"btcsuite/btcd#285", "btcsuite/btcd#286",
		"btcsuite/btcwallet#1083", "btcsuite/btcwallet#1313",
		"lightninglabs/lightning-infra#3545", "lightninglabs/lightning-infra#3709",
	}, got)
}

func TestSortReviews_compares_numbers_numerically_not_lexicographically(t *testing.T) {
	reviews := []ReviewView{
		{Number: "1083", Repository: "same/repo"},
		{Number: "286", Repository: "same/repo"},
	}

	sortReviews(reviews)

	// Lexicographically "1083" < "286"; the queue must use the numeric order.
	require.Equal(t, "286", reviews[0].Number)
	require.Equal(t, "1083", reviews[1].Number)
}

func TestSortReviews_folds_owner_casing_when_grouping(t *testing.T) {
	reviews := []ReviewView{
		{Number: "2", Repository: "Owner/repo"},
		{Number: "9", Repository: "other/repo"},
		{Number: "1", Repository: "owner/repo"},
	}

	sortReviews(reviews)

	// "Owner/repo" and "owner/repo" are the same repository and must stay adjacent
	// and in number order, rather than being split by uppercase sorting ahead of
	// every lowercase name. ("other" precedes "owner", so #9 leads.)
	got := make([]string, 0, len(reviews))
	for _, review := range reviews {
		got = append(got, review.Repository+"#"+review.Number)
	}
	require.Equal(t, []string{"other/repo#9", "owner/repo#1", "Owner/repo#2"}, got)
}

func TestReviewQueue_applies_the_ordering_on_load(t *testing.T) {
	m, _ := loaded(t)
	send(t, m, reviewsLoadedMsg{reviews: []ReviewView{
		{Number: "3630", Repository: "lightninglabs/lightning-infra", Project: "lightning-infra"},
		{Number: "1305", Repository: "btcsuite/btcwallet", Project: "btcwallet"},
		{Number: "1083", Repository: "btcsuite/btcwallet", Project: "btcwallet"},
	}})

	require.Equal(t, []string{"1083", "1305", "3630"},
		[]string{m.reviews[0].Number, m.reviews[1].Number, m.reviews[2].Number})
}

// --- cached queue and refresh indicator ---

func cachedFixture() []ReviewView {
	return []ReviewView{
		{Number: "999", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "from the cache", Base: "master", Head: "old"},
	}
}

func TestReviewQueue_cache_fills_the_section_before_the_fetch_lands(t *testing.T) {
	m, _ := loaded(t)
	fetchedAt := time.Now().Add(-3 * time.Minute)

	send(t, m, cachedReviewsMsg{reviews: cachedFixture(), fetchedAt: fetchedAt, ok: true})

	// Rows are on screen without any network call having returned.
	require.True(t, m.reviewsLoaded)
	require.True(t, m.reviewsFromCache)
	require.Len(t, m.reviews, 1)
	require.NotContains(t, m.View(), "loading…")
	require.Contains(t, m.View(), "from the cache")
}

func TestReviewQueue_header_shows_the_cache_age_and_drops_it_once_fresh(t *testing.T) {
	m, _ := loaded(t)
	m.now = func() time.Time { return time.Unix(2_000, 0) }
	send(t, m, cachedReviewsMsg{
		reviews: cachedFixture(), fetchedAt: time.Unix(2_000, 0).Add(-2 * time.Hour), ok: true,
	})

	// Stale rows must not be presented as current, especially when gh is failing.
	require.Contains(t, m.View(), "cached 2h ago")

	send(t, m, reviewsLoadedMsg{reviews: reviewFixtures()})

	require.NotContains(t, m.View(), "cached")
	require.False(t, m.reviewsFromCache)
}

func TestReviewQueue_fetch_result_replaces_the_cached_rows(t *testing.T) {
	m, _ := loaded(t)
	send(t, m, cachedReviewsMsg{reviews: cachedFixture(), fetchedAt: time.Now(), ok: true})

	send(t, m, reviewsLoadedMsg{reviews: reviewFixtures()})

	require.Len(t, m.reviews, len(reviewFixtures()))
	require.NotContains(t, m.View(), "from the cache")
}

func TestReviewQueue_a_late_cache_read_never_clobbers_a_landed_fetch(t *testing.T) {
	m, _ := loaded(t)
	send(t, m, reviewsLoadedMsg{reviews: reviewFixtures()})

	// The disk read finishing second must not undo fresher rows.
	send(t, m, cachedReviewsMsg{reviews: cachedFixture(), fetchedAt: time.Now(), ok: true})

	require.Len(t, m.reviews, len(reviewFixtures()))
	require.False(t, m.reviewsFromCache)
	require.NotContains(t, m.View(), "from the cache")
}

func TestReviewQueue_absent_cache_leaves_the_section_loading(t *testing.T) {
	m, _ := loaded(t)

	send(t, m, cachedReviewsMsg{ok: false})

	require.False(t, m.reviewsLoaded)
	require.Contains(t, m.View(), "loading…")
}

func TestReviewQueue_spinner_turns_while_refreshing_and_stops_after(t *testing.T) {
	m, _ := loaded(t)
	require.NotNil(t, m.beginReviewRefresh())
	require.True(t, m.refreshing)
	require.Contains(t, m.View(), spinnerFrames[0])

	// The tick keeps rescheduling itself only while a fetch is in flight.
	_, cmd := m.Update(spinnerTickMsg{})
	require.NotNil(t, cmd)
	require.Equal(t, 1, m.spinnerFrame)

	send(t, m, reviewsLoadedMsg{reviews: reviewFixtures()})
	require.False(t, m.refreshing)

	_, cmd = m.Update(spinnerTickMsg{})
	require.Nil(t, cmd, "the spinner loop must end when the fetch does")
}

func TestReviewQueue_refresh_is_not_started_twice_while_one_is_in_flight(t *testing.T) {
	m, _ := loaded(t)
	require.NotNil(t, m.beginReviewRefresh())

	// A manual r landing during the periodic refresh must not stack a second
	// fetch or a second spinner loop.
	require.Nil(t, m.beginReviewRefresh())
}

func TestReviewQueue_failed_fetch_stops_the_spinner_and_keeps_the_rows(t *testing.T) {
	m, _ := loaded(t)
	send(t, m, cachedReviewsMsg{reviews: cachedFixture(), fetchedAt: time.Now(), ok: true})
	m.beginReviewRefresh()

	send(t, m, reviewsFailedMsg{err: errBrowse})

	// Being offline says nothing about whether those review requests still exist,
	// so the cached rows stay — but the spinner must not turn forever.
	require.False(t, m.refreshing)
	require.Len(t, m.reviews, 1)
	require.Contains(t, m.status, "no browser")
	_, cmd := m.Update(spinnerTickMsg{})
	require.Nil(t, cmd)
}

func TestHumanAge_reads_the_way_a_person_would_say_it(t *testing.T) {
	require.Equal(t, "just now", humanAge(20*time.Second))
	require.Equal(t, "5m ago", humanAge(5*time.Minute))
	require.Equal(t, "3h ago", humanAge(3*time.Hour))
	require.Equal(t, "2d ago", humanAge(50*time.Hour))
}

// --- recent tabs ---

// recentModel builds a dashboard whose current session ("bitcoin") has one open
// tab, plus three managed worktrees: one whose tab is that open one, one closed
// branch worktree, and one closed pull-request worktree.
func recentModel(t *testing.T) (*model, *fakeJumper, *fakeCommander) {
	t.Helper()
	source := fakeSource{
		sessions: []SessionView{{Name: "bitcoin", Current: true}},
		tabs:     map[string][]TabView{"bitcoin": {{Title: "btcwallet:live"}}},
		recents: []RecentView{
			{Project: "btcwallet", Branch: "live", Title: "btcwallet:live", Worktree: "/wt/live"},
			{Project: "btcwallet", Branch: "itests/accounts", Title: "btcwallet:itests/accounts", Worktree: "/wt/itests-accounts"},
			{Project: "btcwallet", PullRequest: "1313", IsPullRequest: true, Title: "btcwallet:pr-1313", Worktree: "/wt/pr-1313"},
		},
	}
	jumper := &fakeJumper{}
	commander := &fakeCommander{}
	m := newModel(context.Background(), source, jumper, commander, "bitcoin")
	_, cmd := m.Update(sessionsLoadedMsg{sessions: source.sessions})
	runBatch(t, m, cmd)
	send(t, m, recentsLoadedMsg{recents: source.recents})
	return m, jumper, commander
}

func focusRecent(t *testing.T, m *model, title string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selRecent && m.recents[row.recent].Title == title {
			m.cursor = i
			return
		}
	}
	t.Fatalf("recent row %q not among rows", title)
}

func TestRecent_lists_managed_worktrees_with_their_age(t *testing.T) {
	m, _, _ := recentModel(t)
	m.recents[1].TouchedAt = m.now().Add(-50 * time.Hour)
	m.rebuildRows()

	view := m.View()
	require.Contains(t, view, "recent tabs")
	require.Contains(t, view, "btcwallet:itests/accounts")
	require.Contains(t, view, "2d ago")
	// The worktree whose tab is open says so instead of an age, because Enter
	// jumps to it rather than checking anything out.
	require.Contains(t, view, "open")
}

func TestRecent_enter_jumps_when_the_tab_is_already_open(t *testing.T) {
	m, jumper, commander := recentModel(t)
	focusRecent(t, m, "btcwallet:live")

	send(t, m, key("enter"))

	require.Equal(t, []jumpCall{{session: "bitcoin", tab: "btcwallet:live"}}, jumper.calls)
	require.Empty(t, commander.calls, "an open tab must not be checked out again")
}

func TestRecent_enter_reopens_a_closed_branch_worktree_with_wco(t *testing.T) {
	m, jumper, commander := recentModel(t)
	focusRecent(t, m, "btcwallet:itests/accounts")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Empty(t, jumper.calls)
	require.Equal(t, []commandCall{{op: "wco", project: "btcwallet", arg: "itests/accounts"}}, commander.calls)
}

// A pull-request worktree's branch is "zwm/pr-<n>-<hash>" while its tab is
// "<project>:pr-<n>", so reopening has to go back through wpr — a wco of that
// branch would title the tab after the raw branch instead.
func TestRecent_enter_reopens_a_pull_request_worktree_with_wpr_and_never_forces(t *testing.T) {
	m, _, commander := recentModel(t)
	focusRecent(t, m, "btcwallet:pr-1313")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1313", force: false}}, commander.calls)
}

func TestRecent_section_is_absent_when_there_are_no_managed_worktrees(t *testing.T) {
	m, _, _ := recentModel(t)
	m.recents = nil
	m.rebuildRows()

	require.NotContains(t, m.View(), "recent tabs")
	for _, row := range m.rows {
		require.NotEqual(t, selRecent, row.kind)
	}
}

func TestRecent_tab_cycles_into_the_section(t *testing.T) {
	m, _, _ := recentModel(t)
	// From the tree, Tab must reach the recent section rather than skipping it.
	found := false
	for range 4 {
		m.cycleSection(1)
		if row, ok := m.currentRow(); ok && row.kind == selRecent {
			found = true
			break
		}
	}
	require.True(t, found, "tab never landed on the recent section")
}
