package command

import (
	"context"
	"os"

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

// PullRequests returns open pull requests for the selected project as
// "<number>:<title>" completion entries.
func (completer Completer) PullRequests(ctx context.Context, selected cli.ProjectNameOrPath) []string {
	resolution, ok := completer.resolve(ctx, selected)
	if !ok {
		return nil
	}
	summaries, err := completer.github.ListOpenPullRequests(ctx, github.Directory(resolution.ProjectRoot))
	if err != nil {
		return nil
	}
	entries := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		entries = append(entries, string(summary.Number)+":"+summary.Title)
	}
	return entries
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
