package command

import (
	"context"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/agentstate"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

type tabNamesRunner struct{ stdout string }

func (tabNamesRunner) Available(context.Context, zellij.CommandName) error { return nil }
func (runner tabNamesRunner) Run(context.Context, zellij.Command) (zellij.Output, error) {
	return zellij.Output{Stdout: runner.stdout}, nil
}

func TestTuiSourceAgents_forgets_agents_whose_tab_closed(t *testing.T) {
	store := agentstate.NewStore(t.TempDir())
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "5", TabTitle: "btcwallet:feature", State: agentstate.Working}))
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "9", TabTitle: "btcwallet:closed", State: agentstate.Done}))
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "3", TabTitle: "", State: agentstate.Waiting}))

	source := tuiSource{
		config: zellij.Config{Runner: tabNamesRunner{stdout: "btcwallet\nbtcwallet:feature\n"}},
		store:  store,
	}

	agents, err := source.Agents(context.Background(), "bitcoin")
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

func TestTuiSourceAgents_keeps_records_when_the_tab_query_is_unreliable(t *testing.T) {
	store := agentstate.NewStore(t.TempDir())
	require.NoError(t, store.Write(agentstate.Record{Session: "bitcoin", PaneID: "9", TabTitle: "btcwallet:closed", State: agentstate.Done}))

	// Empty tab list => cannot trust it => reconcile must be skipped.
	source := tuiSource{
		config: zellij.Config{Runner: tabNamesRunner{stdout: ""}},
		store:  store,
	}

	agents, err := source.Agents(context.Background(), "bitcoin")
	require.NoError(t, err)
	require.Len(t, agents, 1)

	remaining, err := store.Load()
	require.NoError(t, err)
	require.Len(t, remaining, 1)
}
