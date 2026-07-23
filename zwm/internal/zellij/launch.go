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

	if hasExactTabName(tabNames.Stdout, input.Title) {
		command := Command{
			Name: CommandZellij,
			Args: []string{"action", "go-to-tab-name", string(input.Title)},
		}
		output, focusErr := config.Runner.Run(ctx, command)
		if focusErr != nil {
			return Result{}, externalFailure("focus Zellij tab", command, output, focusErr)
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

func hasExactTabName(tabNames string, title TabTitle) bool {
	for tabName := range strings.SplitSeq(tabNames, "\n") {
		if tabName == string(title) {
			return true
		}
	}
	return false
}

func externalFailure(message string, command Command, output Output, cause error) error {
	failure := &CommandFailure{Command: command, Output: output, cause: cause}
	return errs.Wrap(errs.External, message, failure)
}
