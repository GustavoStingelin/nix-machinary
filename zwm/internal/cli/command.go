package cli

import (
	"context"

	urfave "github.com/urfave/cli/v3"
)

func newCommand(config Config, invocation Invocation, result *Result) *urfave.Command {
	execute := func(ctx context.Context, _ *urfave.Command) error {
		serviceResult, err := config.Service.Execute(ctx, invocation)
		if err != nil {
			return err
		}
		*result = serviceResult
		return nil
	}

	return &urfave.Command{
		Name:                   "zwm",
		UsageText:              "zwm [-C|--project <name-or-path>] {co|pr}",
		HideHelp:               true,
		HideHelpCommand:        true,
		UseShortOptionHandling: false,
		Writer:                 config.Stdout,
		ErrWriter:              config.Stderr,
		ExitErrHandler: func(_ context.Context, _ *urfave.Command, _ error) {
		},
		Commands: []*urfave.Command{
			{
				Name:            "co",
				HideHelp:        true,
				HideHelpCommand: true,
				SkipFlagParsing: true,
				Action:          execute,
			},
			{
				Name:            "pr",
				HideHelp:        true,
				HideHelpCommand: true,
				SkipFlagParsing: true,
				Action:          execute,
			},
		},
	}
}
