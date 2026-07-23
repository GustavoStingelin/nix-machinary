package git

import (
	"bytes"
	"context"
	"errors"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

var ErrNoWorktree = errors.New("no Git worktree found")

func (client Client) WorktreeRoot(ctx context.Context, directory Directory) (Directory, error) {
	output, err := client.run(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return Directory(bytes.TrimSuffix(output.Stdout, []byte("\n"))), nil
}

func (client Client) PrimaryWorktreeRoot(ctx context.Context, directory Directory) (Directory, error) {
	raw, err := client.ListWorktrees(ctx, directory)
	if err != nil {
		return "", err
	}
	records, err := worktree.ParsePorcelainZ(raw)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", ErrNoWorktree
	}
	return Directory(records[0].Path), nil
}
