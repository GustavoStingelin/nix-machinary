package github

import "strings"

type Directory string

type PullRequestSelector string

type PullRequestNumber string

type HeadRefName string

type Branch string

type PullRequest struct {
	Number      PullRequestNumber
	HeadRefName HeadRefName
}

// PullRequestSummary describes an open pull request for shell completion.
type PullRequestSummary struct {
	Number PullRequestNumber
	Title  string
	Author string
}

// ReviewRequest is an open pull request awaiting the authenticated user's
// review. It spans every repository the search can see, so Repository is
// "owner/name" and may name a repo with no local checkout.
type ReviewRequest struct {
	Number     PullRequestNumber
	Repository string
	Title      string
	Author     string
}

// RepositoryName is the bare repo name ("btcwallet" from "btcsuite/btcwallet"),
// which is what a project directory under the code root is named.
func (request ReviewRequest) RepositoryName() string {
	_, name, found := strings.Cut(request.Repository, "/")
	if !found {
		return request.Repository
	}
	return name
}

// PullRequestRefs carries the branch detail a review needs. BaseRefName is the
// branch the pull request actually merges into — for a stacked pull request that
// is the branch below it, not the repository's default branch — so it is the
// only correct left side of a review diff range. HeadOid is the head commit,
// used to tell whether a local worktree has fallen behind.
type PullRequestRefs struct {
	Number      PullRequestNumber
	BaseRefName string
	HeadRefName string
	HeadOid     string
}

type CheckoutRequest struct {
	Directory Directory
	Selector  PullRequestSelector
	Branch    Branch
	// Force resets the existing local branch to the latest state of the pull
	// request, discarding local commits (gh pr checkout --force).
	Force bool
}

type Config struct {
	Executable string
}

type Client struct {
	executable string
}

func NewClient(config Config) Client {
	executable := config.Executable
	if executable == "" {
		executable = "gh"
	}
	return Client{executable: executable}
}
