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
	sessions    []SessionView
	tabs        map[string][]TabView
	agents      map[string][]AgentView
	reviews     []ReviewView
	reviewErr   error
	worktrees   []WorktreeView
	worktreeErr error
	mine        []ReviewView
	cached      []ReviewView
	cachedAt    time.Time
	cachedOK    bool
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

func (source fakeSource) MyPullRequests(context.Context) ([]ReviewView, error) {
	return source.mine, nil
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
func (source fakeSource) Worktrees(context.Context) ([]WorktreeView, error) {
	return source.worktrees, source.worktreeErr
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
	// branch marks a worktree removal that was asked to delete the branch too.
	branch bool
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
func (commander *fakeCommander) OpenWorktree(_ context.Context, worktree, title string) error {
	commander.calls = append(commander.calls, commandCall{op: "open-worktree", project: title, arg: worktree})
	return commander.err
}
func (commander *fakeCommander) RemoveWorktree(_ context.Context, project, worktree string, deleteBranch bool) error {
	commander.calls = append(commander.calls, commandCall{op: "rm", project: project, arg: worktree, branch: deleteBranch})
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
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
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

// --- worktrees ---

// worktreeModel builds a dashboard whose current session ("bitcoin") has one
// open tab, plus one worktree of every kind: a managed branch worktree whose tab
// is that open one, a closed managed branch worktree, a closed pull-request
// worktree, a repository root, and one made by hand outside the managed root.
func worktreeModel(t *testing.T) (*model, *fakeJumper, *fakeCommander) {
	t.Helper()
	source := fakeSource{
		sessions: []SessionView{{Name: "bitcoin", Current: true}},
		tabs:     map[string][]TabView{"bitcoin": {{Title: "btcwallet:live"}}},
		worktrees: []WorktreeView{
			{Project: "btcwallet", Branch: "live", Title: "btcwallet:live", Worktree: "/wt/live"},
			{Project: "btcwallet", Branch: "itests/accounts", Title: "btcwallet:itests/accounts", Worktree: "/wt/itests-accounts"},
			{Project: "btcwallet", Branch: "zwm/pr-1313-ab12cd34", Kind: WorktreePullRequest, PullRequest: "1313", Title: "btcwallet:pr-1313", Worktree: "/wt/pr-1313"},
			{Project: "btcwallet", Kind: WorktreePrimary, Title: "btcwallet", Worktree: "/code/btcwallet"},
			{Project: "lnd", Branch: "feature", Kind: WorktreeExternal, Title: "lnd:feature", Worktree: "/tmp/by-hand"},
		},
	}
	jumper := &fakeJumper{}
	commander := &fakeCommander{}
	m := newModel(context.Background(), source, jumper, commander, "bitcoin")
	_, cmd := m.Update(sessionsLoadedMsg{sessions: source.sessions})
	runBatch(t, m, cmd)
	send(t, m, worktreesLoadedMsg{worktrees: source.worktrees})
	// Worktrees live in their own pane, which is where every test below acts.
	m.switchPane(paneWorktrees)
	return m, jumper, commander
}

func focusWorktree(t *testing.T, m *model, title string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selWorktree && m.worktrees[row.worktree].Title == title {
			m.cursor = i
			return
		}
	}
	t.Fatalf("worktree row %q not among rows", title)
}

func TestWorktrees_lists_every_worktree_with_its_age(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.worktrees[1].TouchedAt = m.now().Add(-50 * time.Hour)
	m.rebuildRows()

	view := m.View()
	require.Contains(t, view, "btcwallet:itests/accounts")
	require.Contains(t, view, "2d ago")
	// The worktree whose tab is open says so instead of an age, because Enter
	// jumps to it rather than checking anything out.
	require.Contains(t, view, "open")
	// The two kinds no wco reopens are called out, and rows are grouped by project.
	require.Contains(t, view, "root")
	require.Contains(t, view, "external")
	require.Contains(t, view, "─── lnd ───")
}

func TestWorktrees_enter_jumps_when_the_tab_is_already_open(t *testing.T) {
	m, jumper, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:live")

	send(t, m, key("enter"))

	require.Equal(t, []jumpCall{{session: "bitcoin", tab: "btcwallet:live"}}, jumper.calls)
	require.Empty(t, commander.calls, "an open tab must not be checked out again")
}

func TestWorktrees_enter_reopens_a_closed_branch_worktree_with_wco(t *testing.T) {
	m, jumper, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:itests/accounts")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Empty(t, jumper.calls)
	require.Equal(t, []commandCall{{op: "wco", project: "btcwallet", arg: "itests/accounts"}}, commander.calls)
}

// A pull-request worktree's branch is "zwm/pr-<n>-<hash>" while its tab is
// "<project>:pr-<n>", so reopening has to go back through wpr — a wco of that
// branch would title the tab after the raw branch instead.
func TestWorktrees_enter_reopens_a_pull_request_worktree_with_wpr_and_never_forces(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:pr-1313")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1313", force: false}}, commander.calls)
}

// The repository root is a project, not a branch worktree, so it reopens the way
// `o` does.
func TestWorktrees_enter_reopens_the_primary_worktree_as_the_project(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	cmd()

	require.Equal(t, []commandCall{{op: "open", project: "btcwallet"}}, commander.calls)
}

// No wco/wpr describes a worktree zwm did not create — wco would refuse its
// branch as already checked out — so it opens at its own path instead.
func TestWorktrees_enter_opens_an_external_worktree_at_its_own_path(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "lnd:feature")

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	cmd()

	require.Equal(t, []commandCall{{op: "open-worktree", project: "lnd:feature", arg: "/tmp/by-hand"}}, commander.calls)
}

func TestWorktrees_pane_says_so_when_there_are_none(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.worktrees = nil
	m.rebuildRows()

	require.Contains(t, m.View(), "(no worktrees)")
	require.Empty(t, m.rows)
}

// --- panes ---

func TestPanes_worktrees_are_absent_from_the_dashboard_pane(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.switchPane(paneDashboard)

	require.NotContains(t, m.View(), "btcwallet:itests/accounts")
	for _, row := range m.rows {
		require.NotEqual(t, selWorktree, row.kind)
	}
}

func TestPanes_bracket_keys_cycle_between_the_panes(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.switchPane(paneDashboard)

	send(t, m, key("]"))
	require.Equal(t, paneWorktrees, m.activePane)

	send(t, m, key("]"))
	require.Equal(t, paneMine, m.activePane)

	send(t, m, key("]"))
	require.Equal(t, paneDashboard, m.activePane, "] wraps after the last pane")

	send(t, m, key("["))
	require.Equal(t, paneMine, m.activePane)
}

func TestPanes_number_keys_select_a_pane_directly(t *testing.T) {
	m, _, _ := worktreeModel(t)

	send(t, m, key("1"))
	require.Equal(t, paneDashboard, m.activePane)

	send(t, m, key("2"))
	require.Equal(t, paneWorktrees, m.activePane)

	send(t, m, key("3"))
	require.Equal(t, paneMine, m.activePane)
}

// The panes list unrelated things, so a carried-over index would land somewhere
// arbitrary.
func TestPanes_switching_resets_the_cursor(t *testing.T) {
	m, _, _ := worktreeModel(t)
	focusWorktree(t, m, "lnd:feature")
	require.NotZero(t, m.cursor)

	m.switchPane(paneDashboard)

	require.Zero(t, m.cursor)
}

func TestPanes_the_bar_marks_the_active_pane(t *testing.T) {
	m, _, _ := worktreeModel(t)

	require.Contains(t, m.View(), "1 dashboard")
	require.Contains(t, m.View(), "2 worktrees")
}

// --- deleting a worktree ---

func TestWorktreeDelete_d_asks_before_deleting_and_names_the_worktree_and_branch(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:pr-1313")

	send(t, m, key("d"))

	view := m.View()
	require.Contains(t, view, "delete worktree")
	require.Contains(t, view, "/wt/pr-1313")
	require.Contains(t, view, "zwm/pr-1313-ab12cd34")
	require.Empty(t, commander.calls, "the prompt must not delete anything on its own")
}

func TestWorktreeDelete_y_removes_the_worktree_and_keeps_the_branch(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:itests/accounts")
	send(t, m, key("d"))

	_, cmd := m.Update(key("y"))
	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, worktreeRemovedMsg{}, msg)
	m.Update(msg)

	require.Equal(t, []commandCall{{op: "rm", project: "btcwallet", arg: "/wt/itests-accounts"}}, commander.calls)
	require.Equal(t, modeTree, m.mode)
	require.Contains(t, m.status, "deleted btcwallet:itests/accounts")
}

func TestWorktreeDelete_b_also_deletes_the_branch(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:pr-1313")
	send(t, m, key("d"))

	_, cmd := m.Update(key("b"))
	require.NotNil(t, cmd)
	m.Update(cmd())

	require.Equal(t, []commandCall{{op: "rm", project: "btcwallet", arg: "/wt/pr-1313", branch: true}}, commander.calls)
	require.Contains(t, m.status, "branch zwm/pr-1313-ab12cd34")
}

// Anything but the two deliberate keys backs out, which is the safe default for
// a prompt whose other answers delete.
func TestWorktreeDelete_any_other_key_cancels(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:itests/accounts")
	send(t, m, key("d"))

	send(t, m, key("n"))

	require.Equal(t, modeTree, m.mode)
	require.Empty(t, commander.calls)
}

// The worktree directory is the working directory of whatever runs in that tab,
// and Git would delete it out from under a live shell.
func TestWorktreeDelete_refuses_a_worktree_whose_tab_is_open(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:live")

	send(t, m, key("d"))

	require.Equal(t, modeTree, m.mode)
	require.Empty(t, commander.calls)
	require.Contains(t, m.status, "close the tab first")
}

// A branch deletion can fail after the worktree is already gone, so the list has
// to be reloaded even when the command reports an error.
func TestWorktreeDelete_reports_a_failure_and_still_reloads_the_list(t *testing.T) {
	m, _, commander := worktreeModel(t)
	commander.err = errors.New("git worktree remove: contains modified files")
	focusWorktree(t, m, "btcwallet:itests/accounts")
	send(t, m, key("d"))

	_, cmd := m.Update(key("y"))
	_, reload := m.Update(cmd())

	require.Contains(t, m.status, "contains modified files")
	require.NotNil(t, reload, "the worktree list must be reloaded after a failed delete")
	require.IsType(t, worktreesLoadedMsg{}, reload())
}

func TestWorktreeDelete_footer_advertises_the_key(t *testing.T) {
	m, _, _ := worktreeModel(t)
	focusWorktree(t, m, "btcwallet:itests/accounts")

	require.Contains(t, m.View(), "d delete")
}

// The repository root is refused before the prompt: deleting it would take every
// other worktree's backing store with it.
func TestWorktreeDelete_refuses_the_primary_worktree(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "btcwallet")

	send(t, m, key("d"))

	require.Equal(t, modeTree, m.mode)
	require.Empty(t, commander.calls)
	require.Contains(t, m.status, "cannot be deleted")
}

// A worktree zwm did not create is exactly the kind that accumulates, so it is
// deletable like any other.
func TestWorktreeDelete_deletes_an_external_worktree(t *testing.T) {
	m, _, commander := worktreeModel(t)
	focusWorktree(t, m, "lnd:feature")
	send(t, m, key("d"))

	_, cmd := m.Update(key("y"))
	require.NotNil(t, cmd)
	m.Update(cmd())

	require.Equal(t, []commandCall{{op: "rm", project: "lnd", arg: "/tmp/by-hand"}}, commander.calls)
}

// --- filtering ---

// typeFilter opens the prompt and types text into it, one keystroke at a time,
// which is also what exercises the narrowing on every keystroke.
func typeFilter(t *testing.T, m *model, text string) {
	t.Helper()
	send(t, m, key("/"))
	for _, character := range text {
		send(t, m, key(string(character)))
	}
}

func worktreeTitles(m *model) []string {
	titles := make([]string, 0, len(m.rows))
	for _, row := range m.rows {
		titles = append(titles, m.worktrees[row.worktree].Title)
	}
	return titles
}

func TestFilter_narrows_the_list_as_you_type(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "itests")

	require.Equal(t, []string{"btcwallet:itests/accounts"}, worktreeTitles(m))
	require.Contains(t, m.View(), "btcwallet:itests/accounts")
	require.NotContains(t, m.View(), "btcwallet:pr-1313")
}

// The path carries information the title does not, which is the point of
// searching both.
func TestFilter_matches_the_worktree_path_as_well_as_the_title(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "by-hand")

	require.Equal(t, []string{"lnd:feature"}, worktreeTitles(m))
}

func TestFilter_finds_a_pull_request_by_number(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "1313")

	require.Equal(t, []string{"btcwallet:pr-1313"}, worktreeTitles(m))
}

func TestFilter_ignores_case(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "ITESTS")

	require.Equal(t, []string{"btcwallet:itests/accounts"}, worktreeTitles(m))
}

func TestFilter_says_so_when_nothing_matches(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "nothing-matches-this")

	require.Empty(t, m.rows)
	require.Contains(t, m.View(), "(no matches)")
}

// The count is of what is on screen, so the filter's effect shows even when the
// matches would have fit unfiltered.
func TestFilter_bar_counts_matches_against_the_total(t *testing.T) {
	m, _, _ := worktreeModel(t)
	require.Contains(t, m.View(), "[5]")

	typeFilter(t, m, "itests")

	require.Contains(t, m.View(), "[1/5]")
}

func TestFilter_backspace_widens_the_list_again(t *testing.T) {
	m, _, _ := worktreeModel(t)
	typeFilter(t, m, "itestsX")
	require.Empty(t, m.rows)

	send(t, m, key("backspace"))

	require.Equal(t, []string{"btcwallet:itests/accounts"}, worktreeTitles(m))
}

// Enter keeps the filter and hands the keys back, because the point of
// filtering is to act on what is left.
func TestFilter_enter_keeps_the_filter_and_releases_the_keys(t *testing.T) {
	m, _, commander := worktreeModel(t)
	typeFilter(t, m, "itests")

	send(t, m, key("enter"))
	require.False(t, m.filter.active)
	require.Equal(t, "itests", m.filter.text)
	require.Equal(t, []string{"btcwallet:itests/accounts"}, worktreeTitles(m))

	// `d` deletes again rather than typing a `d`.
	send(t, m, key("d"))
	require.Equal(t, modeConfirm, m.mode)
	require.Empty(t, commander.calls)
}

func TestFilter_esc_clears_the_filter(t *testing.T) {
	m, _, _ := worktreeModel(t)
	typeFilter(t, m, "itests")

	send(t, m, key("esc"))

	require.False(t, m.filter.active)
	require.Empty(t, m.filter.text)
	require.Len(t, m.rows, 5)
}

// Esc after Enter still clears, so the key that dismisses the prompt also undoes
// what the prompt did.
func TestFilter_esc_clears_an_accepted_filter_before_it_quits(t *testing.T) {
	m, _, _ := worktreeModel(t)
	typeFilter(t, m, "itests")
	send(t, m, key("enter"))

	_, cmd := m.Update(key("esc"))

	require.Nil(t, cmd, "the first esc clears rather than quitting")
	require.Empty(t, m.filter.text)
	require.Len(t, m.rows, 5)
}

// While the prompt is open every letter is text, so the action keys cannot fire
// behind it.
func TestFilter_action_keys_are_text_while_the_prompt_is_open(t *testing.T) {
	m, _, commander := worktreeModel(t)

	typeFilter(t, m, "d")

	require.Equal(t, "d", m.filter.text)
	require.Equal(t, modeTree, m.mode)
	require.Empty(t, commander.calls)
}

// The row indices behind a filtered list are not its positions, so deleting has
// to reach the entry actually under the cursor.
func TestFilter_delete_acts_on_the_filtered_row(t *testing.T) {
	m, _, commander := worktreeModel(t)
	typeFilter(t, m, "1313")
	send(t, m, key("enter"))
	require.Zero(t, m.cursor)

	send(t, m, key("d"))
	_, cmd := m.Update(key("y"))
	require.NotNil(t, cmd)
	m.Update(cmd())

	require.Equal(t, []commandCall{{op: "rm", project: "btcwallet", arg: "/wt/pr-1313"}}, commander.calls)
}

// Headings come from the rows that survive the filter, not from the full list.
func TestFilter_project_headings_follow_the_matches(t *testing.T) {
	m, _, _ := worktreeModel(t)

	typeFilter(t, m, "by-hand")

	view := m.View()
	require.Contains(t, view, "─── lnd ───")
	require.NotContains(t, view, "─── btcwallet ───")
}

func TestFilter_is_dropped_when_the_pane_changes(t *testing.T) {
	m, _, _ := worktreeModel(t)
	typeFilter(t, m, "itests")

	m.switchPane(paneDashboard)
	m.switchPane(paneWorktrees)

	require.Empty(t, m.filter.text)
	require.Len(t, m.rows, 5)
}

// The dashboard's sections are short and live; narrowing them would hide the
// state the pane exists to report.
func TestFilter_is_a_worktrees_pane_key_only(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.switchPane(paneDashboard)

	send(t, m, key("/"))

	require.False(t, m.filter.active)
	require.NotContains(t, m.View(), "/▏")
}

// --- my pull requests ---

func mineFixtures() []ReviewView {
	return []ReviewView{
		// Its branch is already checked out in a worktree whose tab is open.
		{Number: "1400", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: live work", Base: "master", Head: "live", LocalBranch: true},
		// Local branch, but no worktree on it yet.
		{Number: "1401", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: parked", Base: "master", Head: "parked", LocalBranch: true},
		// Opened from another machine: nothing local at all.
		{Number: "1402", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: elsewhere", Base: "master", Head: "elsewhere"},
		// Already checked out earlier through wpr.
		{Number: "1313", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: via wpr", Base: "master", Head: "via-wpr"},
		{Number: "9", Repository: "someone/not-cloned", Title: "far away", Base: "main", Head: "x"},
	}
}

func mineModel(t *testing.T) (*model, *fakeJumper, *fakeCommander) {
	t.Helper()
	m, jumper, commander := worktreeModel(t)
	send(t, m, myPullRequestsLoadedMsg{pullRequests: mineFixtures()})
	m.switchPane(paneMine)
	return m, jumper, commander
}

func focusMine(t *testing.T, m *model, number string) {
	t.Helper()
	for i, row := range m.rows {
		if row.kind == selMine && m.mine[row.mine].Number == number {
			m.cursor = i
			return
		}
	}
	t.Fatalf("pull request %s is not a navigable row", number)
}

func TestMine_pane_lists_only_the_users_pull_requests_with_where_enter_goes(t *testing.T) {
	m, _, _ := mineModel(t)

	view := m.View()
	require.Contains(t, view, "my pull requests")
	require.Contains(t, view, "─── btcsuite/btcwallet ───")
	require.Contains(t, view, "parked → master")
	require.Contains(t, view, "open")
	require.Contains(t, view, "branch")
	require.Contains(t, view, "(not cloned)")
	for _, row := range m.rows {
		require.Equal(t, selMine, row.kind)
	}
}

func TestMine_enter_jumps_to_the_open_tab_of_its_branch(t *testing.T) {
	m, jumper, commander := mineModel(t)
	focusMine(t, m, "1400")

	send(t, m, key("enter"))

	require.Equal(t, []jumpCall{{session: "bitcoin", tab: "btcwallet:live"}}, jumper.calls)
	require.Empty(t, commander.calls)
}

// Your own pull request is a branch you push to, so it reopens through wco and
// keeps its name; wpr would rename it to zwm/pr-<n>-<hash>.
func TestMine_enter_reopens_a_local_branch_with_wco(t *testing.T) {
	m, _, commander := mineModel(t)
	focusMine(t, m, "1401")

	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wco", project: "btcwallet", arg: "parked"}}, commander.calls)
}

func TestMine_enter_falls_back_to_wpr_without_a_local_branch(t *testing.T) {
	m, _, commander := mineModel(t)
	focusMine(t, m, "1402")

	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1402"}}, commander.calls)
}

func TestMine_enter_reuses_an_existing_wpr_worktree(t *testing.T) {
	m, _, commander := mineModel(t)
	focusMine(t, m, "1313")

	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "1313"}}, commander.calls)
}

func TestMine_enter_refuses_a_repository_with_no_local_checkout(t *testing.T) {
	m, _, commander := mineModel(t)
	focusMine(t, m, "9")

	require.Nil(t, m.activate())
	require.Empty(t, commander.calls)
	require.Contains(t, m.status, "no checkout")
}

func TestMine_browse_key_opens_the_pull_request(t *testing.T) {
	m, _, commander := mineModel(t)
	focusMine(t, m, "9")

	send(t, m, key("b"))

	require.Equal(t, []commandCall{{op: "browse", repository: "someone/not-cloned", arg: "9"}}, commander.calls)
}

func TestMine_empty_list_is_distinguished_from_not_yet_loaded(t *testing.T) {
	m, _, _ := worktreeModel(t)
	m.switchPane(paneMine)
	require.Contains(t, m.View(), "loading…")

	send(t, m, myPullRequestsLoadedMsg{})

	require.Contains(t, m.View(), "(no open pull requests)")
}

func TestMine_failed_fetch_keeps_the_rows_and_reports(t *testing.T) {
	m, _, _ := mineModel(t)
	m.mineRefreshing = true

	send(t, m, myPullRequestsFailedMsg{err: errors.New("gh: offline")})

	require.False(t, m.mineRefreshing)
	require.Len(t, m.mine, len(mineFixtures()))
	require.Contains(t, m.status, "offline")
}
