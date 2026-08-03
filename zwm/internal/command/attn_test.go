package command

import (
	"context"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/agentstate"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	commands []zellij.Command
}

func (runner *recordingRunner) Available(context.Context, zellij.CommandName) error {
	return nil
}

func (runner *recordingRunner) Run(_ context.Context, command zellij.Command) (zellij.Output, error) {
	runner.commands = append(runner.commands, command)
	return zellij.Output{}, nil
}

func newTestAttnRecorder(t *testing.T, env serviceTestEnvironment) (attnRecorder, *recordingRunner, *agentstate.Store) {
	t.Helper()
	runner := &recordingRunner{}
	store := agentstate.NewStore(t.TempDir())
	return attnRecorder{env: env, runner: runner, store: store}, runner, store
}

func TestAttnRecord_is_a_no_op_outside_zellij(t *testing.T) {
	// Given an environment with no ZELLIJ session
	recorder, runner, store := newTestAttnRecorder(t, serviceTestEnvironment{
		zellij.EnvironmentZellijSessionName: "bitcoin",
		zellij.EnvironmentZellijPaneID:      "3",
	})

	// When
	err := recorder.Record(context.Background(), "waiting", "claude")

	// Then no record is written and no glyph pipe is fired
	require.NoError(t, err)
	require.Empty(t, runner.commands)
	records, loadErr := store.Load()
	require.NoError(t, loadErr)
	require.Empty(t, records)
}

func TestAttnRecord_persists_state_and_fires_glyph_pipe(t *testing.T) {
	// Given a live session with pane identity
	recorder, runner, store := newTestAttnRecorder(t, serviceTestEnvironment{
		zellij.EnvironmentZellij:            "0",
		zellij.EnvironmentZellijSessionName: "bitcoin",
		zellij.EnvironmentZellijPaneID:      "3",
	})

	// When a finished signal arrives
	err := recorder.Record(context.Background(), "finished", "claude")

	// Then it is persisted as the canonical "done" state...
	require.NoError(t, err)
	records, loadErr := store.Load()
	require.NoError(t, loadErr)
	require.Len(t, records, 1)
	require.Equal(t, "bitcoin", records[0].Session)
	require.Equal(t, "3", records[0].PaneID)
	require.Equal(t, "claude", records[0].Agent)
	require.Equal(t, agentstate.Done, records[0].State)

	// ...and the tab glyph pipe is fired for this pane
	require.Len(t, runner.commands, 1)
	require.Equal(t, zellij.CommandZellij, runner.commands[0].Name)
	require.Equal(t, []string{
		"pipe", "--plugin", "zwm-attn", "--name", "zwm-attn",
		"--args", "pane_id=3,event=done",
	}, runner.commands[0].Args)
}

func TestManagedPRSuffix_maps_pr_branches_and_ignores_others(t *testing.T) {
	suffix, ok := managedPRSuffix("zwm/pr-42-abcd1234")
	require.True(t, ok)
	require.Equal(t, "pr-42", suffix)

	_, ok = managedPRSuffix("itests/very-first-itests")
	require.False(t, ok)

	_, ok = managedPRSuffix("main")
	require.False(t, ok)
}

func TestAttnRecord_rejects_an_unknown_state(t *testing.T) {
	// Given a live session
	recorder, runner, _ := newTestAttnRecorder(t, serviceTestEnvironment{
		zellij.EnvironmentZellij:            "0",
		zellij.EnvironmentZellijSessionName: "bitcoin",
		zellij.EnvironmentZellijPaneID:      "3",
	})

	// When an unrecognized signal arrives
	err := recorder.Record(context.Background(), "napping", "claude")

	// Then it is a usage error and nothing is fired
	require.ErrorIs(t, err, errs.ErrUsage)
	require.Empty(t, runner.commands)
}
