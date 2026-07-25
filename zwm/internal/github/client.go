package github

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

func (client Client) ResolvePullRequest(ctx context.Context, directory Directory, selector PullRequestSelector) (PullRequest, error) {
	output, err := client.run(ctx, directory, "pr", "view", string(selector), "--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv")
	if err != nil {
		return PullRequest{}, err
	}
	return parsePullRequest(output.Stdout)
}

func (client Client) CheckoutPullRequest(ctx context.Context, request CheckoutRequest) error {
	arguments := []string{"pr", "checkout", string(request.Selector), "--branch", string(request.Branch)}
	if request.Force {
		arguments = append(arguments, "--force")
	}
	_, err := client.run(ctx, request.Directory, arguments...)
	return err
}

// ListOpenPullRequests returns open pull requests as "<number>\t<title>" lines.
// It is intended for shell completion and reports no diagnostics beyond the
// returned error.
func (client Client) ListOpenPullRequests(ctx context.Context, directory Directory) ([]PullRequestSummary, error) {
	output, err := client.run(ctx, directory, "pr", "list", "--state", "open", "--json", "number,title", "--jq", ".[] | [.number, .title] | @tsv")
	if err != nil {
		return nil, err
	}
	var summaries []PullRequestSummary
	for _, line := range strings.Split(string(output.Stdout), "\n") {
		if line == "" {
			continue
		}
		number, title, found := strings.Cut(line, "\t")
		if !found || !validNumber(number) {
			continue
		}
		summaries = append(summaries, PullRequestSummary{Number: PullRequestNumber(number), Title: title})
	}
	return summaries, nil
}

func parsePullRequest(output []byte) (PullRequest, error) {
	metadata := strings.TrimSuffix(string(output), "\n")
	if metadata == "" || strings.Contains(metadata, "\n") || strings.Count(metadata, "\t") != 1 {
		return PullRequest{}, &MetadataError{Output: append([]byte(nil), output...)}
	}
	number, headRefName, _ := strings.Cut(metadata, "\t")
	if !validNumber(number) || headRefName == "" {
		return PullRequest{}, &MetadataError{Output: append([]byte(nil), output...)}
	}
	return PullRequest{Number: PullRequestNumber(number), HeadRefName: HeadRefName(headRefName)}, nil
}

func validNumber(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
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
