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

type CheckoutRequest struct {
	Directory Directory
	Selector  PullRequestSelector
	Branch    Branch
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
