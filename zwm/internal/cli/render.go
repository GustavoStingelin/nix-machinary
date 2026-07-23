package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

func writeResult(writer io.Writer, result Result) int {
	_, err := fmt.Fprintf(writer, "worktree_path=%s\ndisplay_identity=%s\ntab_action=%s\ntab_title=%s\ntab_worktree=%s\n",
		result.Worktree,
		result.DisplayIdentity,
		result.TabAction,
		result.TabTitle,
		result.TabWorktree,
	)
	if err != nil {
		return errs.ExitCode(errs.Wrap(errs.External, "write checkout result", err))
	}
	return 0
}

func writeFailure(writer io.Writer, err error) int {
	if writeErr := writeExternalStderr(writer, err); writeErr != nil {
		return errs.ExitCode(errs.Wrap(errs.External, "write external error output", writeErr))
	}
	if writeErr := writeRecovery(writer, err); writeErr != nil {
		return errs.ExitCode(errs.Wrap(errs.External, "write recovery output", writeErr))
	}
	if _, writeErr := fmt.Fprintf(writer, "zwm: %s: %s\n", errs.ClassOf(err), err); writeErr != nil {
		return errs.ExitCode(errs.Wrap(errs.External, "write error output", writeErr))
	}
	return errs.ExitCode(err)
}

func writeExternalStderr(writer io.Writer, err error) error {
	var gitFailure *git.CommandError
	if errors.As(err, &gitFailure) {
		_, writeErr := writer.Write(gitFailure.Stderr)
		return writeErr
	}
	var githubFailure *github.CommandError
	if errors.As(err, &githubFailure) {
		_, writeErr := writer.Write(githubFailure.Stderr)
		return writeErr
	}
	var zellijFailure *zellij.CommandFailure
	if errors.As(err, &zellijFailure) {
		_, writeErr := io.WriteString(writer, zellijFailure.Output.Stderr)
		return writeErr
	}
	return nil
}

func writeRecovery(writer io.Writer, err error) error {
	var pullRequestFailure *app.PullRequestError
	if !errors.As(err, &pullRequestFailure) || pullRequestFailure.Recovery == nil {
		return nil
	}
	_, writeErr := fmt.Fprintf(writer, "zwm: recovery: detached managed pull-request worktree remains at %s; inspect it and complete or remove it manually when safe\n", pullRequestFailure.Recovery.DetachedWorktree)
	return writeErr
}
