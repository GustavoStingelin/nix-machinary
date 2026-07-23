package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

func (client Client) ValidateBranch(ctx context.Context, directory Directory, branch Branch) error {
	_, err := client.run(ctx, directory, "check-ref-format", "--branch", string(branch))
	if err == nil {
		return nil
	}
	if commandExited(err) {
		return &InvalidBranchError{Branch: branch, Cause: commandFailure(err)}
	}
	return err
}

func (client Client) ResolveCommit(ctx context.Context, directory Directory, commitish Commitish) (Commit, error) {
	output, err := client.run(ctx, directory, "rev-parse", "--verify", "--quiet", "--end-of-options", string(commitish)+"^{commit}")
	if err != nil {
		if commandExited(err) {
			return "", &InvalidCommitishError{Commitish: commitish, Cause: commandFailure(err)}
		}
		return "", err
	}
	return Commit(string(bytes.TrimSuffix(output.Stdout, []byte("\n")))), nil
}

func (client Client) LocalBranchExists(ctx context.Context, directory Directory, branch Branch) (bool, error) {
	_, err := client.run(ctx, directory, "show-ref", "--verify", "--quiet", "refs/heads/"+string(branch))
	if err == nil {
		return true, nil
	}
	if commandExitCode(err, 1) {
		return false, nil
	}
	return false, err
}

func (client Client) ListWorktrees(ctx context.Context, directory Directory) ([]byte, error) {
	output, err := client.run(ctx, directory, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return output.Stdout, nil
}

type commandOutput struct {
	Stdout []byte
	Stderr []byte
}

func (client Client) run(ctx context.Context, directory Directory, arguments ...string) (commandOutput, error) {
	command := exec.CommandContext(ctx, client.executable, arguments...)
	command.Dir = string(directory)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	output := commandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err == nil {
		return output, nil
	}
	cause := err
	if contextError := ctx.Err(); contextError != nil {
		cause = errors.Join(contextError, err)
	}
	return output, &CommandError{
		Arguments: append([]string(nil), arguments...),
		Directory: string(directory),
		Stdout:    output.Stdout,
		Stderr:    output.Stderr,
		Cause:     cause,
	}
}

func commandExitCode(err error, expected int) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.ExitCode() == expected
}

func commandExited(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError)
}

func commandFailure(err error) *CommandError {
	var commandError *CommandError
	if errors.As(err, &commandError) {
		return commandError
	}
	return nil
}
