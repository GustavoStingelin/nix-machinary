package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingTUI struct {
	runs int
	err  error
}

func (tui *recordingTUI) Run(context.Context) error {
	tui.runs++
	return tui.err
}

type attnCall struct{ signal, agent string }

type recordingAttn struct {
	calls []attnCall
	err   error
}

func (attn *recordingAttn) Record(_ context.Context, signal, agent string) error {
	attn.calls = append(attn.calls, attnCall{signal, agent})
	return attn.err
}

func runDispatch(t *testing.T, config Config, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	config.Arguments = args
	config.Stdout = &stdout
	config.Stderr = &stderr
	if config.Service == nil {
		config.Service = &recordingService{}
	}
	exitCode := Run(context.Background(), config)
	return exitCode, stdout.String(), stderr.String()
}

func TestBareZwm_launches_the_tui(t *testing.T) {
	tui := &recordingTUI{}

	exitCode, stdout, stderr := runDispatch(t, Config{TUI: tui})

	require.Equal(t, 0, exitCode)
	require.Equal(t, 1, tui.runs)
	require.Empty(t, stdout, "the TUI must not touch the key=value stdout contract")
	require.Empty(t, stderr)
}

func TestTuiSubcommand_launches_the_tui(t *testing.T) {
	tui := &recordingTUI{}

	exitCode, _, _ := runDispatch(t, Config{TUI: tui}, "tui")

	require.Equal(t, 0, exitCode)
	require.Equal(t, 1, tui.runs)
}

func TestBareZwm_without_a_tui_keeps_the_usage_error(t *testing.T) {
	exitCode, _, stderr := runDispatch(t, Config{})

	require.Equal(t, 64, exitCode)
	require.Equal(t, "zwm: usage: missing subcommand\n", stderr)
}

func TestAttn_forwards_the_state_and_agent(t *testing.T) {
	attn := &recordingAttn{}

	exitCode, stdout, stderr := runDispatch(t, Config{Attn: attn}, "attn", "waiting", "--agent", "claude")

	require.Equal(t, 0, exitCode)
	require.Equal(t, []attnCall{{signal: "waiting", agent: "claude"}}, attn.calls)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

func TestAttn_requires_a_state(t *testing.T) {
	attn := &recordingAttn{}

	exitCode, _, stderr := runDispatch(t, Config{Attn: attn}, "attn")

	require.Equal(t, 64, exitCode)
	require.Equal(t, "zwm: usage: attn requires a state\n", stderr)
	require.Empty(t, attn.calls)
}
