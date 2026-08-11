package command

import (
	"context"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/agentstate"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/tui"
	"github.com/stretchr/testify/require"
)

func TestTuiSourceAgents_forgets_agents_whose_tab_closed(t *testing.T) {
	store := agentstate.NewStore(t.TempDir())
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "5", TabTitle: "btcwallet:feature", State: agentstate.Working}))
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "9", TabTitle: "btcwallet:closed", State: agentstate.Done}))
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "3", TabTitle: "", State: agentstate.Waiting}))

	source := tuiSource{store: store}
	liveTabs := []tui.TabView{{Title: "btcwallet"}, {Title: "btcwallet:feature"}}

	agents, err := source.Agents(context.Background(), "bitcoin", liveTabs)
	require.NoError(t, err)

	// Live tab kept, unknown-tab (empty) kept, closed tab dropped.
	titles := make([]string, 0, len(agents))
	for _, agent := range agents {
		titles = append(titles, agent.TabTitle)
	}
	require.ElementsMatch(t, []string{"btcwallet:feature", ""}, titles)

	// The stale record was also forgotten on disk, so it can't resurface.
	remaining, err := store.Load()
	require.NoError(t, err)
	require.Len(t, remaining, 2)
}

// An empty tab list is what both a failed query and a skipped one look like, and
// neither is evidence that the tab is gone.
func TestTuiSourceAgents_keeps_records_without_a_tab_list(t *testing.T) {
	store := agentstate.NewStore(t.TempDir())
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "9", TabTitle: "btcwallet:closed", State: agentstate.Done}))

	source := tuiSource{store: store}

	agents, err := source.Agents(context.Background(), "bitcoin", nil)
	require.NoError(t, err)
	require.Len(t, agents, 1)

	remaining, err := store.Load()
	require.NoError(t, err)
	require.Len(t, remaining, 1)
}
