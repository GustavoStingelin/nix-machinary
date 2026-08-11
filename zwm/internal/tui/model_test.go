package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	sessions  []SessionView
	tabs      map[string][]TabView
	agents    map[string][]AgentView
	reviews   []ReviewView
	reviewErr error
}

func (source fakeSource) Reviews(context.Context) ([]ReviewView, error) {
	return source.reviews, source.reviewErr
}

func (source fakeSource) Sessions(context.Context) ([]SessionView, error) {
	return source.sessions, nil
}

func (source fakeSource) Tabs(_ context.Context, session string) ([]TabView, error) {
	return source.tabs[session], nil
}

func (source fakeSource) Agents(_ context.Context, session string) ([]AgentView, error) {
	return source.agents[session], nil
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

func loadData(t *testing.T, m *model, session string) {
	t.Helper()
	send(t, m, m.loadSessionDataCmd(session)())
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
