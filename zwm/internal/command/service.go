package command

import (
	"context"
	"os"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

type Service struct {
	branches     app.BranchService
	env          zellij.Environment
	preflight    zellij.Config
	projects     project.Resolver
	pullRequests app.PullRequestService
	tabs         app.TabLauncher
}

func NewSystemService() Service {
	runner := zellij.SystemRunner{}
	environment := zellij.SystemEnvironment{}
	zellijConfig := zellij.Config{Runner: runner, Environment: environment}
	gitClient := git.NewClient(git.Config{})
	tabs := app.TabLauncherFunc(func(ctx context.Context, input zellij.Input) (zellij.Result, error) {
		return zellij.Launch(ctx, zellijConfig, input)
	})

	return Service{
		branches:  app.NewBranchService(gitClient, tabs),
		env:       environment,
		preflight: zellijConfig,
		projects:  project.NewResolver(projectRepository{client: gitClient}),
		pullRequests: app.NewPullRequestService(
			app.NewSystemPullRequestGit(""),
			github.NewClient(github.Config{}),
		),
		tabs: tabs,
	}
}

func (service Service) Execute(ctx context.Context, invocation cli.Invocation) (cli.Result, error) {
	if err := zellij.Preflight(ctx, service.preflight); err != nil {
		return cli.Result{}, err
	}
	home, present := service.env.Lookup(zellij.EnvironmentHome)
	if !present || home == "" {
		return cli.Result{}, errs.New(errs.Preflight, "HOME is not available")
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return cli.Result{}, errs.Wrap(errs.External, "determine working directory", err)
	}
	resolution, err := service.projects.Resolve(ctx, project.Request{
		Home:             project.Directory(home),
		Project:          project.Value(invocation.Project),
		WorkingDirectory: project.Directory(workingDirectory),
	})
	if err != nil {
		return cli.Result{}, err
	}

	switch action := invocation.Action.(type) {
	case cli.CheckoutExisting:
		result, checkoutErr := service.branches.CheckoutExisting(ctx, app.CheckoutExistingInput{
			Project: resolution,
			Branch:  git.Branch(action.Branch),
		})
		if checkoutErr != nil {
			return cli.Result{}, checkoutErr
		}
		return branchResult(result), nil
	case cli.CheckoutNew:
		result, checkoutErr := service.branches.CheckoutNew(ctx, app.CheckoutNewInput{
			Project:    resolution,
			Branch:     git.Branch(action.Branch),
			StartPoint: git.Commitish(action.StartPoint),
		})
		if checkoutErr != nil {
			return cli.Result{}, checkoutErr
		}
		return branchResult(result), nil
	case cli.PullRequest:
		result, checkoutErr := service.pullRequests.Checkout(ctx, app.PullRequestInput{
			Project:  resolution,
			Selector: github.PullRequestSelector(action.Selector),
		})
		if checkoutErr != nil {
			return cli.Result{}, checkoutErr
		}
		tab, launchErr := service.tabs.Launch(ctx, zellij.Input{
			Title:    zellij.TabTitle(string(resolution.Key) + ":" + result.Display),
			Worktree: zellij.WorktreePath(result.Worktree),
		})
		if launchErr != nil {
			return cli.Result{}, launchErr
		}
		return cli.Result{
			Worktree:        string(result.Worktree),
			DisplayIdentity: result.Display,
			TabAction:       string(tab.Action),
			TabTitle:        string(tab.Title),
			TabWorktree:     string(tab.Worktree),
		}, nil
	default:
		return cli.Result{}, errs.New(errs.External, "unsupported CLI action")
	}
}

func branchResult(result app.CheckoutResult) cli.Result {
	return cli.Result{
		Worktree:        string(result.Worktree),
		DisplayIdentity: result.DisplayIdentity,
		TabAction:       string(result.TabAction),
		TabTitle:        string(result.TabTitle),
		TabWorktree:     string(result.TabWorktree),
	}
}

type projectRepository struct {
	client git.Client
}

func (repository projectRepository) WorktreeRoot(ctx context.Context, directory project.Directory) (project.Directory, error) {
	root, err := repository.client.WorktreeRoot(ctx, git.Directory(directory))
	return project.Directory(root), err
}

func (repository projectRepository) PrimaryWorktreeRoot(ctx context.Context, directory project.Directory) (project.Directory, error) {
	root, err := repository.client.PrimaryWorktreeRoot(ctx, git.Directory(directory))
	return project.Directory(root), err
}
