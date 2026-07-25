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

// Run parses raw arguments with urfave/cli, delegates approved requests, and
// returns a process exit code.
func Run(ctx context.Context, config Config) int {
	var result Result
	arguments := append([]string{"zwm"}, config.Arguments...)
	if err := newCommand(config, &result).Run(ctx, arguments); err != nil {
		return writeFailure(config.Stderr, err)
	}
	if err := writeResult(config.Stdout, result); err != nil {
		return writeFailure(config.Stderr, err)
	}
	return 0
}

func usageError(message string) error {
	return errs.New(errs.Usage, message)
}
