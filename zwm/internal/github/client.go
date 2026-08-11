package github

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"slices"
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

// ListOpenPullRequests returns open pull requests as number, title, and author
// handle. It is intended for shell completion and reports no diagnostics beyond
// the returned error.
func (client Client) ListOpenPullRequests(ctx context.Context, directory Directory) ([]PullRequestSummary, error) {
	output, err := client.run(ctx, directory, "pr", "list", "--state", "open", "--json", "number,title,author", "--jq", ".[] | [.number, .title, .author.login] | @tsv")
	if err != nil {
		return nil, err
	}
	var summaries []PullRequestSummary
	for _, line := range strings.Split(string(output.Stdout), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || !validNumber(fields[0]) {
			continue
		}
		summaries = append(summaries, PullRequestSummary{Number: PullRequestNumber(fields[0]), Title: fields[1], Author: fields[2]})
	}
	return summaries, nil
}

// reviewRequestLimit caps the review queue. The search is one call regardless of
// size, but an unbounded list would make the dashboard section unreadable.
const reviewRequestLimit = "50"

// ListReviewRequests returns open pull requests that request the authenticated
// user's review, across every repository. It is a single search call and carries
// no branch detail: `gh search prs` cannot return baseRefName/headRefName, so
// callers needing refs follow up with ViewPullRequestRefs per pull request.
func (client Client) ListReviewRequests(ctx context.Context, directory Directory) ([]ReviewRequest, error) {
	output, err := client.run(ctx, directory,
		"search", "prs", "--review-requested", "@me", "--state", "open",
		"--limit", reviewRequestLimit,
		"--json", "number,title,author,repository",
		"--jq", ".[] | [.number, .repository.nameWithOwner, .author.login, .title] | @tsv")
	if err != nil {
		return nil, err
	}
	var requests []ReviewRequest
	for line := range strings.SplitSeq(string(output.Stdout), "\n") {
		if line == "" {
			continue
		}
		// Title is last and unsplit: it is the only field that can contain a tab.
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 || !validNumber(fields[0]) || fields[1] == "" {
			continue
		}
		requests = append(requests, ReviewRequest{
			Number:     PullRequestNumber(fields[0]),
			Repository: fields[1],
			Author:     fields[2],
			Title:      fields[3],
		})
	}
	return requests, nil
}

// ViewPullRequestRefs reads one pull request's base/head branches and head
// commit. repository is "owner/name", which lets this work from any directory —
// the review queue spans repositories, so it cannot rely on the cwd's remote.
func (client Client) ViewPullRequestRefs(ctx context.Context, directory Directory, repository string, number PullRequestNumber) (PullRequestRefs, error) {
	output, err := client.run(ctx, directory, "pr", "view", string(number),
		"--repo", repository,
		"--json", "number,baseRefName,headRefName,headRefOid",
		"--jq", "[.number, .baseRefName, .headRefName, .headRefOid] | @tsv")
	if err != nil {
		return PullRequestRefs{}, err
	}
	return parsePullRequestRefs(output.Stdout)
}

// BrowsePullRequest opens a pull request in the user's browser. It goes through
// `gh` rather than an OS opener so there is no per-platform launcher here, and it
// takes the repository explicitly so it works for a pull request whose repository
// has no local checkout at all.
func (client Client) BrowsePullRequest(ctx context.Context, directory Directory, repository string, number PullRequestNumber) error {
	_, err := client.run(ctx, directory, "pr", "view", string(number), "--repo", repository, "--web")
	return err
}

func parsePullRequestRefs(output []byte) (PullRequestRefs, error) {
	metadata := strings.TrimSuffix(string(output), "\n")
	fields := strings.Split(metadata, "\t")
	if len(fields) != 4 || !validNumber(fields[0]) {
		return PullRequestRefs{}, &MetadataError{Output: append([]byte(nil), output...)}
	}
	if slices.Contains(fields[1:], "") {
		return PullRequestRefs{}, &MetadataError{Output: append([]byte(nil), output...)}
	}
	return PullRequestRefs{
		Number:      PullRequestNumber(fields[0]),
		BaseRefName: fields[1],
		HeadRefName: fields[2],
		HeadOid:     fields[3],
	}, nil
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
