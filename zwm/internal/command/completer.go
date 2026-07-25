package command

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

// Completer resolves shell-completion candidates from live Git, GitHub, and
// filesystem state. Every method is best-effort: any failure yields no
// candidates so completion never disrupts the shell.
type Completer struct {
	git      git.Client
	github   github.Client
	projects project.Resolver
	env      zellij.Environment
}

// NewSystemCompleter builds a Completer backed by the real git and gh binaries.
func NewSystemCompleter() Completer {
	gitClient := git.NewClient(git.Config{})
	return Completer{
		git:      gitClient,
		github:   github.NewClient(github.Config{}),
		projects: project.NewResolver(projectRepository{client: gitClient}),
		env:      zellij.SystemEnvironment{},
	}
}

// Branches returns local branch names for the selected project.
func (completer Completer) Branches(ctx context.Context, selected cli.ProjectNameOrPath) []string {
	resolution, ok := completer.resolve(ctx, selected)
	if !ok {
		return nil
	}
	branches, err := completer.git.ListLocalBranches(ctx, git.Directory(resolution.ProjectRoot))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(branches))
	for _, branch := range branches {
		names = append(names, string(branch))
	}
	return names
}

// Projects returns selectable project names under the code root.
func (completer Completer) Projects(_ context.Context) []string {
	home, present := completer.env.Lookup(zellij.EnvironmentHome)
	if !present || home == "" {
		return nil
	}
	return project.ListNames(project.Directory(home))
}

// PullRequests returns open pull requests for the selected project, newest
// first, as "<number>:<author>  <title>" completion entries.
func (completer Completer) PullRequests(ctx context.Context, selected cli.ProjectNameOrPath) []string {
	resolution, ok := completer.resolve(ctx, selected)
	if !ok {
		return nil
	}
	summaries, err := completer.github.ListOpenPullRequests(ctx, github.Directory(resolution.ProjectRoot))
	if err != nil {
		return nil
	}
	return pullRequestCandidates(summaries)
}

// pullRequestCandidates renders pull request summaries as zsh completion
// entries. Entries are ordered by descending number (newest first) and the
// author handle is padded into a column so titles line up.
func pullRequestCandidates(summaries []github.PullRequestSummary) []string {
	ordered := append([]github.PullRequestSummary(nil), summaries...)
	sort.Slice(ordered, func(left, right int) bool {
		return pullRequestNumber(ordered[left].Number) > pullRequestNumber(ordered[right].Number)
	})

	authorWidth := 0
	for _, summary := range ordered {
		if handle := authorHandle(summary.Author); len(handle) > authorWidth {
			authorWidth = len(handle)
		}
	}

	entries := make([]string, 0, len(ordered))
	for _, summary := range ordered {
		// "<number>:<description>" — zsh _describe splits on the first colon,
		// so the number is the inserted selector and the rest is shown as help.
		entries = append(entries, fmt.Sprintf("%s:%-*s  %s", summary.Number, authorWidth, authorHandle(summary.Author), summary.Title))
	}
	return entries
}

func authorHandle(login string) string {
	if login == "" {
		return "@?"
	}
	return "@" + login
}

// pullRequestNumber parses a validated pull request number for ordering. The
// GitHub client only emits digit-only numbers, so a parse failure sorts last.
func pullRequestNumber(number github.PullRequestNumber) int {
	value, err := strconv.Atoi(string(number))
	if err != nil {
		return -1
	}
	return value
}

func (completer Completer) resolve(ctx context.Context, selected cli.ProjectNameOrPath) (project.Resolution, bool) {
	home, present := completer.env.Lookup(zellij.EnvironmentHome)
	if !present || home == "" {
		return project.Resolution{}, false
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return project.Resolution{}, false
	}
	resolution, err := completer.projects.Resolve(ctx, project.Request{
		Home:             project.Directory(home),
		Project:          project.Value(selected),
		WorkingDirectory: project.Directory(workingDirectory),
	})
	if err != nil {
		return project.Resolution{}, false
	}
	return resolution, true
}
