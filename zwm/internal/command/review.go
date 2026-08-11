package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/tui"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

// reviewRefWorkers bounds the concurrent `gh pr view` calls used to fill in each
// queued pull request's base/head. The queue is small, but one sequential call
// per pull request is the difference between the section appearing at once and
// trickling in over several seconds.
const reviewRefWorkers = 6

// reviewAgent is the command the `a` binding hands a pull request to. opencode's
// interactive TUI accepts a seed prompt, which is what makes a one-key review
// launch possible; `opencode run` would finish and exit instead.
const reviewAgent = "opencode"

// reviewSource answers the dashboard's review queue: which pull requests want
// the user's review, which of them have a local checkout, and which of those
// checkouts have gone stale.
type reviewSource struct {
	github github.Client
	git    git.Client
	env    zellij.Environment
}

func newReviewSource() reviewSource {
	return reviewSource{
		github: github.NewClient(github.Config{}),
		git:    git.NewClient(git.Config{}),
		env:    zellij.SystemEnvironment{},
	}
}

// Reviews lists open pull requests awaiting the user's review. It is one GitHub
// search plus one concurrent `gh pr view` per result for the branch detail, and
// one `git rev-parse` for each pull request that already has a managed worktree.
func (source reviewSource) Reviews(ctx context.Context) ([]tui.ReviewView, error) {
	home, present := source.env.Lookup(zellij.EnvironmentHome)
	if !present || home == "" {
		return nil, errs.New(errs.Preflight, "HOME is not set")
	}
	requests, err := source.github.ListReviewRequests(ctx, github.Directory(home))
	if err != nil {
		return nil, errs.Wrap(errs.External, "list review requests", err)
	}

	// A repository maps to a project only when the code root holds a directory of
	// the same name; anything else is a review request on something not cloned.
	projects := make(map[string]struct{})
	for _, name := range project.ListNames(project.Directory(home)) {
		projects[name] = struct{}{}
	}

	views := make([]tui.ReviewView, len(requests))
	var group sync.WaitGroup
	slots := make(chan struct{}, reviewRefWorkers)
	for index, request := range requests {
		view := tui.ReviewView{
			Number:     string(request.Number),
			Repository: request.Repository,
			Title:      request.Title,
			Author:     request.Author,
		}
		if _, ok := projects[request.RepositoryName()]; ok {
			view.Project = request.RepositoryName()
		}
		views[index] = view

		group.Add(1)
		go func(index int, request github.ReviewRequest) {
			defer group.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			source.annotate(ctx, home, &views[index], request)
		}(index, request)
	}
	group.Wait()
	return views, nil
}

// annotate fills in one row's branch detail and local state. Every step is
// best-effort: a pull request whose refs or worktree cannot be read is still a
// real review request and must keep its row.
func (source reviewSource) annotate(ctx context.Context, home string, view *tui.ReviewView, request github.ReviewRequest) {
	refs, err := source.github.ViewPullRequestRefs(ctx, github.Directory(home), request.Repository, request.Number)
	if err != nil {
		return
	}
	view.Base = refs.BaseRefName
	view.Head = refs.HeadRefName
	if view.Project == "" {
		return
	}

	path := managedPullRequestPath(home, view.Project, string(request.Number))
	if _, statErr := os.Stat(string(path)); statErr != nil {
		return
	}
	view.Worktree = string(path)
	// Stale means the checkout no longer points at the pull request's head, so
	// reviewing it would read outdated code. Only a successful comparison marks
	// staleness — an unreadable HEAD must not imply "up to date" or "stale".
	head, err := source.git.ResolveCommit(ctx, git.Directory(path), "HEAD")
	if err != nil {
		return
	}
	view.Stale = string(head) != refs.HeadOid
}

// managedPullRequestPath is where wpr parks a pull request's worktree:
// <code root>/.wt/<project>/pr-<number>. Built through the same helper wpr uses
// so the two cannot drift.
func managedPullRequestPath(home, projectName, number string) worktree.Path {
	root := worktree.Path(filepath.Join(home, "code", ".wt", projectName))
	return worktree.ManagedWorktreePath(root, "pr-"+number)
}

// reviewLauncher checks out a pull request and hands it to a review agent.
type reviewLauncher struct {
	service cli.Service
	github  github.Client
	config  zellij.Config
	env     zellij.Environment
}

func newReviewLauncher() reviewLauncher {
	config := zellij.Config{Runner: zellij.SystemRunner{}, Environment: zellij.SystemEnvironment{}}
	return reviewLauncher{
		service: NewSystemService(),
		github:  github.NewClient(github.Config{}),
		config:  config,
		env:     zellij.SystemEnvironment{},
	}
}

// Launch checks out the pull request worktree (the same wpr path the dashboard's
// plain checkout uses, so the tab is created and focused identically) and then
// opens a pane in that tab running the review agent, seeded with a prompt naming
// the pull request and the branches it actually spans.
func (launcher reviewLauncher) Launch(ctx context.Context, projectName, repository, selector string, force bool) error {
	if _, err := launcher.service.Execute(ctx, cli.Invocation{
		Project: cli.ProjectNameOrPath(projectName),
		Action:  cli.PullRequest{Selector: cli.PullRequestSelector(selector), Force: force},
	}); err != nil {
		return err
	}

	home, present := launcher.env.Lookup(zellij.EnvironmentHome)
	if !present || home == "" {
		return errs.New(errs.Preflight, "HOME is not set")
	}
	refs, err := launcher.github.ViewPullRequestRefs(ctx, github.Directory(home), repository, github.PullRequestNumber(selector))
	if err != nil {
		return errs.Wrap(errs.External, "read pull request refs", err)
	}
	path := string(managedPullRequestPath(home, projectName, selector))

	// The checkout above focused the pull request's tab, so a plain `zellij run`
	// lands the agent pane inside it.
	command := zellij.Command{
		Name: zellij.CommandZellij,
		Args: []string{
			"run", "--cwd", path, "--name", "review", "--",
			reviewAgent, path, "--prompt", reviewPrompt(repository, selector, refs),
		},
	}
	output, err := launcher.config.Runner.Run(ctx, command)
	if err != nil {
		message := fmt.Sprintf("start review agent (%s)", reviewAgent)
		if stderr := strings.TrimSpace(output.Stderr); stderr != "" {
			message += ": " + stderr
		}
		return errs.Wrap(errs.External, message, err)
	}
	return nil
}

// Browse opens a pull request on GitHub. It needs neither a checkout nor a
// worktree, so it works for every row in the review queue.
func (launcher reviewLauncher) Browse(ctx context.Context, repository, selector string) error {
	home, _ := launcher.env.Lookup(zellij.EnvironmentHome)
	if err := launcher.github.BrowsePullRequest(ctx, github.Directory(home), repository, github.PullRequestNumber(selector)); err != nil {
		return errs.Wrap(errs.External, fmt.Sprintf("open %s#%s in a browser", repository, selector), err)
	}
	return nil
}

// reviewPrompt seeds the review agent. The base branch comes from the pull
// request itself, never a default branch: a stacked pull request merges into the
// branch below it, so diffing against the default branch would show every commit
// from the branches underneath as if it were part of this review.
func reviewPrompt(repository, number string, refs github.PullRequestRefs) string {
	return fmt.Sprintf(
		"Review pull request #%s in %s.\n"+
			"Base branch (merge target): %s\n"+
			"Head branch: %s\n"+
			"Diff range: %s...%s\n\n"+
			"This pull request targets %s, not the repository's default branch, so review only "+
			"the commits in that range. Use `git diff origin/%s...HEAD` if the local base ref is missing.",
		number, repository,
		refs.BaseRefName, refs.HeadRefName,
		refs.BaseRefName, refs.HeadRefName,
		refs.BaseRefName, refs.BaseRefName,
	)
}
