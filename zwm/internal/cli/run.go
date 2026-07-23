package cli

import (
	"context"
	"io"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

// Config supplies the process boundary dependencies for Run.
type Config struct {
	Arguments []string
	Service   Service
	Stdout    io.Writer
	Stderr    io.Writer
}

// Run parses raw arguments, delegates approved requests, and returns a process exit code.
func Run(ctx context.Context, config Config) int {
	parsed, err := parse(config.Arguments)
	if err != nil {
		return writeFailure(config.Stderr, err)
	}
	if parsed.help {
		if _, err := io.WriteString(config.Stdout, HelpText); err != nil {
			return errs.ExitCode(errs.Wrap(errs.External, "write help", err))
		}
		return 0
	}

	var result Result
	if err := newCommand(config, parsed.invocation, &result).Run(ctx, parsed.frameworkArgs); err != nil {
		return writeFailure(config.Stderr, err)
	}
	return writeResult(config.Stdout, result)
}
