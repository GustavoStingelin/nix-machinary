package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	urfave "github.com/urfave/cli/v3"
)

const (
	projectFlagName = "project"
	newBranchFlag   = "b"
)

// newCommand builds the urfave/cli command tree. Flag and subcommand parsing is
// owned by urfave; each Action performs only the semantic validation urfave
// cannot express (argument arity and emptiness) before delegating to the service.
func newCommand(config Config, result *Result) *urfave.Command {
	root := &urfave.Command{
		Name:                  "zwm",
		Usage:                 "Zellij worktree manager",
		HideHelpCommand:       true,
		EnableShellCompletion: true,
		ShellComplete:         rootShellComplete(config.Completer),
		Writer:                config.Stdout,
		ErrWriter:             config.Stderr,
		OnUsageError:          onUsageError,
		ExitErrHandler:        func(context.Context, *urfave.Command, error) {},
		Flags: []urfave.Flag{
			&urfave.StringFlag{
				Name:    projectFlagName,
				Aliases: []string{"C"},
				Usage:   "select a project before the subcommand",
			},
		},
		// Reached only when the first argument matches no subcommand.
		Action: func(_ context.Context, cmd *urfave.Command) error {
			if cmd.NArg() == 0 {
				return usageError("missing subcommand")
			}
			return usageError("unknown subcommand '" + cmd.Args().First() + "'")
		},
		Commands: []*urfave.Command{
			checkoutCommand(config, result),
			openCommand(config, result),
			pullRequestCommand(config, result),
		},
	}

	for _, command := range root.Commands {
		command.OnUsageError = onUsageError
	}
	return root
}

func checkoutCommand(config Config, result *Result) *urfave.Command {
	return &urfave.Command{
		Name:      "wco",
		Usage:     "check out a branch in a worktree",
		ArgsUsage: "<branch> | -b <new-branch> [<start-point>]",
		HideHelp:  true,
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: newBranchFlag, Usage: "create a new branch"},
		},
		ShellComplete: func(ctx context.Context, cmd *urfave.Command) {
			if config.Completer == nil {
				return
			}
			// With -b the first positional is a new branch name (no candidates);
			// its optional start-point and the plain existing-branch argument both
			// complete from local branches.
			startPointPosition := cmd.Bool(newBranchFlag) && cmd.NArg() == 1
			existingPosition := !cmd.Bool(newBranchFlag) && cmd.NArg() == 0
			if startPointPosition || existingPosition {
				printCandidates(cmd.Root().Writer, config.Completer.Branches(ctx, project(cmd)))
			}
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			action, err := checkoutAction(cmd)
			if err != nil {
				return err
			}
			return execute(ctx, config, result, project(cmd), action)
		},
	}
}

func checkoutAction(cmd *urfave.Command) (Action, error) {
	arguments := cmd.Args().Slice()
	if cmd.Bool(newBranchFlag) {
		if len(arguments) == 0 || arguments[0] == "" {
			return nil, usageError("wco -b requires a new branch")
		}
		if len(arguments) > 2 {
			return nil, usageError("wco -b accepts a new branch and optional start-point")
		}
		action := CheckoutNew{Branch: BranchName(arguments[0])}
		if len(arguments) == 2 {
			if arguments[1] == "" {
				return nil, usageError("wco -b requires a non-empty start-point when provided")
			}
			action.StartPoint = StartPoint(arguments[1])
		}
		return action, nil
	}

	if len(arguments) == 0 || arguments[0] == "" {
		return nil, usageError("wco requires an existing local branch")
	}
	// Guard against option-like branches urfave forwards as positionals
	// (e.g. "-123"), which must never reach the git subprocess as a flag.
	if strings.HasPrefix(arguments[0], "-") {
		return nil, usageError("unknown wco option '" + arguments[0] + "'")
	}
	if len(arguments) != 1 {
		return nil, usageError("wco accepts exactly one existing local branch")
	}
	return CheckoutExisting{Branch: BranchName(arguments[0])}, nil
}

func openCommand(config Config, result *Result) *urfave.Command {
	return &urfave.Command{
		Name:      "o",
		Usage:     "open a project",
		ArgsUsage: "<name-or-path>",
		HideHelp:  true,
		ShellComplete: func(ctx context.Context, cmd *urfave.Command) {
			if config.Completer != nil && cmd.NArg() == 0 {
				printCandidates(cmd.Root().Writer, config.Completer.Projects(ctx))
			}
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			if cmd.IsSet(projectFlagName) {
				return usageError("o does not accept -C/--project")
			}
			arguments := cmd.Args().Slice()
			if len(arguments) == 0 || arguments[0] == "" {
				return usageError("o requires a project name or path")
			}
			if len(arguments) != 1 {
				return usageError("o accepts exactly one project name or path")
			}
			return execute(ctx, config, result, ProjectNameOrPath(arguments[0]), OpenProject{})
		},
	}
}

func pullRequestCommand(config Config, result *Result) *urfave.Command {
	return &urfave.Command{
		Name:      "wpr",
		Usage:     "check out a pull request in a worktree",
		ArgsUsage: "<number|url|branch>",
		HideHelp:  true,
		ShellComplete: func(ctx context.Context, cmd *urfave.Command) {
			if config.Completer != nil && cmd.NArg() == 0 {
				printCandidates(cmd.Root().Writer, config.Completer.PullRequests(ctx, project(cmd)))
			}
		},
		Action: func(ctx context.Context, cmd *urfave.Command) error {
			arguments := cmd.Args().Slice()
			if len(arguments) == 0 || arguments[0] == "" {
				return usageError("wpr requires a pull request selector")
			}
			if len(arguments) != 1 {
				return usageError("wpr accepts exactly one pull request selector")
			}
			// Guard against option-like selectors urfave forwards as positionals
			// (e.g. "-123"), which must never reach the gh subprocess as a flag.
			if strings.HasPrefix(arguments[0], "-") {
				return usageError("invalid pull request selector '" + arguments[0] + "'")
			}
			return execute(ctx, config, result, project(cmd), PullRequest{Selector: PullRequestSelector(arguments[0])})
		},
	}
}

func project(cmd *urfave.Command) ProjectNameOrPath {
	return ProjectNameOrPath(cmd.String(projectFlagName))
}

func execute(ctx context.Context, config Config, result *Result, selectedProject ProjectNameOrPath, action Action) error {
	serviceResult, err := config.Service.Execute(ctx, Invocation{Project: selectedProject, Action: action})
	if err != nil {
		return err
	}
	*result = serviceResult
	return nil
}

// rootShellComplete completes a project name when the value of -C/--project is
// being typed, and otherwise falls back to urfave's default subcommand and flag
// completion.
func rootShellComplete(completer Completer) urfave.ShellCompleteFunc {
	return func(ctx context.Context, cmd *urfave.Command) {
		if completer != nil && completingProjectFlagValue() {
			printCandidates(cmd.Root().Writer, completer.Projects(ctx))
			return
		}
		urfave.DefaultCompleteWithFlags(ctx, cmd)
	}
}

// completingProjectFlagValue reports whether the token immediately before the
// completion sentinel is the project flag, meaning the shell is completing its
// value. The root command's completion runs before flags are bound, so os.Args
// is the only view of what the user has typed.
func completingProjectFlagValue() bool {
	arguments := os.Args
	if len(arguments) < 2 {
		return false
	}
	previous := arguments[len(arguments)-2]
	return previous == "-C" || previous == "--project"
}

func printCandidates(writer io.Writer, candidates []string) {
	for _, candidate := range candidates {
		fmt.Fprintln(writer, candidate)
	}
}

// onUsageError normalizes urfave flag-parsing failures into the zwm usage class
// so they render as "zwm: usage: <message>" with exit code 64.
func onUsageError(_ context.Context, _ *urfave.Command, err error, _ bool) error {
	return usageError(err.Error())
}
