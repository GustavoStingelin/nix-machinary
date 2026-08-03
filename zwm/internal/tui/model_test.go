package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	sessions []SessionView
	tabs     map[string][]TabView
	agents   map[string][]AgentView
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

type jumpCall struct{ session, tab string }

type fakeJumper struct {
	calls []jumpCall
	err   error
}

func (jumper *fakeJumper) JumpTo(_ context.Context, session, tab string) error {
	jumper.calls = append(jumper.calls, jumpCall{session, tab})
	return jumper.err
}

type commandCall struct {
	op, project, arg string
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

func loaded(t *testing.T) (*model, *fakeJumper) {
	t.Helper()
	m, jumper := newTestModel()
	send(t, m, sessionsLoadedMsg{sessions: m.source.(fakeSource).sessions})
	return m, jumper
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
	lines := strings.Split(m.View(), "\n")

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
	require.Greater(t, opencode, worktreeTab)
	require.Equal(t, worktreeTab+1, opencode)
	// claude (unknown tab) renders at session level, above the first tab row.
	require.Less(t, claude, firstTab)
}

func TestModel_enter_on_a_current_session_tab_jumps(t *testing.T) {
	m, jumper := loaded(t)
	send(t, m, key("down")) // cursor bitcoin -> first tab (btcwallet)

	_, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	require.IsType(t, jumpDoneMsg{}, cmd())
	require.Equal(t, []jumpCall{{session: "bitcoin", tab: "btcwallet"}}, jumper.calls)
}

func TestModel_enter_on_a_remote_session_tab_shows_a_hint_and_does_not_jump(t *testing.T) {
	m, jumper := loaded(t)

	// rows: bitcoin, editor, agent, nix, stale. Walk to nix and expand it.
	send(t, m, key("down"))
	send(t, m, key("down"))
	send(t, m, key("down")) // on nix
	send(t, m, key("right"))
	send(t, m, key("down")) // on nix's shell tab
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

	send(t, m, key("down")) // cursor onto the "btcwallet" tab
	send(t, m, key("w"))    // scoped to btcwallet, no project prompt
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

	send(t, m, key("down")) // onto the "btcwallet" tab
	send(t, m, key("w"))
	typeString(t, m, "main")
	send(t, m, key("enter"))

	require.Equal(t, []commandCall{{op: "wco", project: "btcwallet", arg: "main"}}, commander.calls)
}

func TestPalette_wpr_scopes_to_the_tab_project(t *testing.T) {
	m, _ := loaded(t)
	commander := m.commander.(*fakeCommander)

	send(t, m, key("down")) // onto the "btcwallet" tab
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

	send(t, m, key("down")) // onto the "btcwallet" tab
	send(t, m, key("p"))    // PR picker

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF}) // force-checkout the selected PR
	require.NotNil(t, cmd)
	require.IsType(t, commandDoneMsg{}, cmd())

	require.Equal(t, []commandCall{{op: "wpr", project: "btcwallet", arg: "123", force: true}}, commander.calls)
}

func TestPalette_w_on_a_session_header_falls_back_to_the_project_picker(t *testing.T) {
	m, _ := loaded(t)

	// Cursor is on the bitcoin session header, which is a workspace, not a
	// project, so `w` opens the project picker rather than guessing.
	send(t, m, key("w"))
	require.Equal(t, modePicker, m.mode)
	require.Equal(t, pickBranchProject, m.pick.kind)
}

func TestSelectedProject_derives_from_a_tab_title_only(t *testing.T) {
	m, _ := loaded(t)

	// A session header is a workspace, not a project.
	_, ok := m.selectedProject()
	require.False(t, ok)

	// A "<project>:<branch>" tab yields its project prefix.
	send(t, m, key("down")) // onto "btcwallet"
	send(t, m, key("down")) // onto "btcwallet:itests/very-first-itests"
	project, ok := m.selectedProject()
	require.True(t, ok)
	require.Equal(t, "btcwallet", project)
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

	// Walk to the exited session (last row) and try to expand it.
	for range m.rows {
		send(t, m, key("down"))
	}
	row, ok := m.currentRow()
	require.True(t, ok)
	require.True(t, m.sessions[row.session].exited)

	send(t, m, key("right"))
	require.False(t, m.sessions[row.session].expanded)
	require.Contains(t, strings.ToLower(m.status), "exited")
}
