package tui

import (
	"context"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Runner renders the dashboard on an alternate screen and satisfies
// cli.TUIRunner. It renders to stderr, never stdout, so the CLI's key=value
// stdout contract stays clean even if a caller captures stdout.
type Runner struct {
	source    Source
	jumper    Jumper
	commander Commander
	current   string
	output    io.Writer
}

// NewRunner wires the data source, the tab jumper, the command runner, and the
// caller's current session (empty when launched outside Zellij).
func NewRunner(source Source, jumper Jumper, commander Commander, current string) *Runner {
	return &Runner{source: source, jumper: jumper, commander: commander, current: current, output: os.Stderr}
}

func (runner *Runner) Run(ctx context.Context) error {
	program := tea.NewProgram(
		newModel(ctx, runner.source, runner.jumper, runner.commander, runner.current),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithOutput(runner.output),
	)
	_, err := program.Run()
	return err
}
