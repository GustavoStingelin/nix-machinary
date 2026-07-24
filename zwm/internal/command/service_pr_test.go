package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
	"github.com/stretchr/testify/require"
)

func TestServiceExecute_uses_concise_pull_request_tab_title_when_head_ref_is_long(t *testing.T) {
	// Given
	home := canonicalTestDirectory(t, t.TempDir())
	projectRoot := filepath.Join(home, "code", "loop")
	require.NoError(t, os.MkdirAll(projectRoot, 0o755))
	t.Chdir(projectRoot)

	repository := &serviceTestRepository{
		worktreeRoot: project.Directory(projectRoot),
		primaryRoot:  project.Directory(projectRoot),
	}
	pullRequest := github.PullRequest{
		Number:      github.PullRequestNumber("1185"),
		HeadRefName: github.HeadRefName("dependabot/go_modules/swapserverrpc/google.golang.org/grpc-1.82.1"),
	}
	git := &commandPullRequestGit{
		records: []worktree.Record{{
			Path:   worktree.Path(projectRoot),
			Head:   worktree.OID("0123456789abcdef0123456789abcdef01234567"),
			Branch: worktree.LocalRef("main"),
			State:  worktree.HeadBranch,
		}},
		head: worktree.OID("0123456789abcdef0123456789abcdef01234567"),
	}
	gateway := &commandPullRequestGateway{pullRequest: pullRequest}
	gateway.onCheckout = func(branch github.Branch) {
		git.register(git.addCalls[len(git.addCalls)-1], worktree.Branch(branch))
	}
	tabs := &commandPRTabLauncher{results: []zellij.Result{
		{Action: zellij.Created, Title: zellij.TabTitle("loop:pr-1185")},
		{Action: zellij.Focused, Title: zellij.TabTitle("loop:pr-1185")},
	}}
	service := Service{
		env: serviceTestEnvironment{
			zellij.EnvironmentHome:   home,
			zellij.EnvironmentZellij: "test-session",
		},
		preflight: zellij.Config{Runner: serviceTestRunner{}, Environment: serviceTestEnvironment{
			zellij.EnvironmentHome:   home,
			zellij.EnvironmentZellij: "test-session",
		}},
		projects:     project.NewResolver(repository),
		pullRequests: app.NewPullRequestService(git, gateway),
		tabs:         tabs,
	}

	// When
	created, createErr := service.Execute(context.Background(), cli.Invocation{Action: cli.PullRequest{Selector: cli.PullRequestSelector("1185")}})
	reused, reuseErr := service.Execute(context.Background(), cli.Invocation{Action: cli.PullRequest{Selector: cli.PullRequestSelector("1185")}})

	// Then
	require.NoError(t, createErr)
	require.NoError(t, reuseErr)
	require.Equal(t, filepath.Join(home, "code", ".wt", "loop", "pr-1185"), string(git.addCalls[0]))
	require.Equal(t, []zellij.Input{
		{Title: zellij.TabTitle("loop:pr-1185"), Cwd: zellij.Directory(git.addCalls[0])},
		{Title: zellij.TabTitle("loop:pr-1185"), Cwd: zellij.Directory(git.addCalls[0])},
	}, tabs.inputs)
	require.Equal(t, cli.WorktreeResult{
		Worktree:        string(git.addCalls[0]),
		DisplayIdentity: "pr-1185-dependabot/go_modules/swapserverrpc/google.golang.org/grpc-1.82.1",
		TabAction:       "created",
		TabTitle:        "loop:pr-1185",
		TabWorktree:     "",
	}, created)
	require.Equal(t, cli.WorktreeResult{
		Worktree:        string(git.addCalls[0]),
		DisplayIdentity: "pr-1185-dependabot/go_modules/swapserverrpc/google.golang.org/grpc-1.82.1",
		TabAction:       "focused",
		TabTitle:        "loop:pr-1185",
		TabWorktree:     "",
	}, reused)
}

type commandPullRequestGit struct {
	records  []worktree.Record
	head     worktree.OID
	addCalls []worktree.Path
}

func (git *commandPullRequestGit) LocalBranchExists(_ context.Context, _ project.Directory, branch worktree.Branch) (bool, error) {
	for _, record := range git.records {
		if record.State == worktree.HeadBranch && record.Branch == worktree.LocalRef(branch) {
			return true, nil
		}
	}
	return false, nil
}

func (git *commandPullRequestGit) ListWorktrees(context.Context, project.Directory) ([]worktree.Record, error) {
	return append([]worktree.Record(nil), git.records...), nil
}

func (git *commandPullRequestGit) ResolveHead(context.Context, project.Directory) (worktree.OID, error) {
	return git.head, nil
}

func (git *commandPullRequestGit) AddDetached(_ context.Context, request app.DetachedWorktreeRequest) error {
	git.addCalls = append(git.addCalls, request.Path)
	git.records = append(git.records, worktree.Record{Path: request.Path, Head: git.head, State: worktree.HeadDetached})
	return nil
}

func (git *commandPullRequestGit) register(path worktree.Path, branch worktree.Branch) {
	for index := range git.records {
		if git.records[index].Path == path {
			git.records[index].Branch = worktree.LocalRef(branch)
			git.records[index].State = worktree.HeadBranch
		}
	}
}

type commandPullRequestGateway struct {
	pullRequest github.PullRequest
	onCheckout  func(github.Branch)
}

type commandPRTabLauncher struct {
	inputs  []zellij.Input
	results []zellij.Result
}

func (launcher *commandPRTabLauncher) Launch(_ context.Context, input zellij.Input) (zellij.Result, error) {
	launcher.inputs = append(launcher.inputs, input)
	result := launcher.results[0]
	launcher.results = launcher.results[1:]
	return result, nil
}

func (gateway *commandPullRequestGateway) ResolvePullRequest(context.Context, github.Directory, github.PullRequestSelector) (github.PullRequest, error) {
	return gateway.pullRequest, nil
}

func (gateway *commandPullRequestGateway) CheckoutPullRequest(_ context.Context, request github.CheckoutRequest) error {
	if gateway.onCheckout != nil {
		gateway.onCheckout(request.Branch)
	}
	return nil
}
