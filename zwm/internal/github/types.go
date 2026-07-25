package github

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
