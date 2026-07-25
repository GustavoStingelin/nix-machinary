package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/command"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(ctx, cli.Config{
		Arguments: os.Args[1:],
		Service:   command.NewSystemService(),
		Completer: command.NewSystemCompleter(),
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}))
}
