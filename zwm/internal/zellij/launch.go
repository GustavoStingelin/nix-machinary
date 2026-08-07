package zellij

import (
	"context"
	"strings"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

const worktreeLayoutSuffix = "/.config/zellij/layouts/worktree.kdl"

func Launch(ctx context.Context, config Config, input Input) (Result, error) {
	if err := Preflight(ctx, config); err != nil {
		return Result{}, err
	}

	tabNames, err := config.Runner.Run(ctx, Command{
		Name: CommandZellij,
		Args: []string{"action", "query-tab-names"},
	})
	if err != nil {
		return Result{}, externalFailure("query Zellij tab names", Command{
			Name: CommandZellij,
			Args: []string{"action", "query-tab-names"},
		}, tabNames, err)
	}

	if name, found := findTabName(tabNames.Stdout, input.Title); found {
		output, focusErr := GoToTab(ctx, config, name)
		if focusErr != nil {
			return Result{}, focusErr
		}
		return Result{Action: Focused, Title: input.Title, Cwd: input.Cwd, Output: output}, nil
	}

	home, present := config.Environment.Lookup(EnvironmentHome)
	if !present || home == "" {
		failure := &PreflightFailure{prerequisite: prerequisiteHome}
		return Result{}, errs.Wrap(errs.Preflight, failure.Error(), failure)
	}
	command := Command{
		Name: CommandZellij,
		Args: []string{
			"action", "new-tab",
			"--layout", home + worktreeLayoutSuffix,
			"--name", string(input.Title),
			"--cwd", string(input.Cwd),
		},
	}
	output, createErr := config.Runner.Run(ctx, command)
	if createErr != nil {
		return Result{}, externalFailure("create Zellij tab", command, output, createErr)
	}
	return Result{Action: Created, Title: input.Title, Cwd: input.Cwd, Output: output}, nil
}

// findTabName locates an existing tab by title and returns the tab's current
// name, which is what Zellij matches on. The two differ while zwm-attn marks
// the tab: matching the raw name would miss a marked tab and create a second
// tab with the same title, so the comparison uses the glyph-stripped title.
func findTabName(tabNames string, title TabTitle) (string, bool) {
	for tabName := range strings.SplitSeq(tabNames, "\n") {
		tabName = strings.TrimRight(tabName, "\r")
		if tabName == "" {
			continue
		}
		if parseTab(tabName).Title == string(title) {
			return tabName, true
		}
	}
	return "", false
}

func externalFailure(message string, command Command, output Output, cause error) error {
	failure := &CommandFailure{Command: command, Output: output, cause: cause}
	return errs.Wrap(errs.External, message, failure)
}
