package cli

import (
	"strings"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

type parsedInvocation struct {
	help          bool
	invocation    Invocation
	frameworkArgs []string
}

func parse(arguments []string) (parsedInvocation, error) {
	var project ProjectNameOrPath

	for argumentIndex := 0; argumentIndex < len(arguments); {
		argument := arguments[argumentIndex]
		switch argument {
		case "--help", "-h":
			if len(arguments) != 1 || project != "" {
				return parsedInvocation{}, usageError("option '" + argument + "' must be used on its own")
			}
			return parsedInvocation{help: true}, nil
		case "-C", "--project":
			if argumentIndex+1 >= len(arguments) || arguments[argumentIndex+1] == "" || strings.HasPrefix(arguments[argumentIndex+1], "-") {
				return parsedInvocation{}, usageError("option '" + argument + "' requires a value")
			}
			project = ProjectNameOrPath(arguments[argumentIndex+1])
			argumentIndex += 2
		case "":
			return parsedInvocation{}, usageError("unknown subcommand ''")
		default:
			if strings.HasPrefix(argument, "--project=") || strings.HasPrefix(argument, "-C") {
				return parsedInvocation{}, usageError("unknown global option '" + argument + "'")
			}
			if strings.HasPrefix(argument, "-") {
				return parsedInvocation{}, usageError("unknown global option '" + argument + "'")
			}
			return parseSubcommand(project, argument, arguments[argumentIndex+1:])
		}
	}

	return parsedInvocation{}, usageError("missing subcommand")
}

func parseSubcommand(project ProjectNameOrPath, subcommand string, arguments []string) (parsedInvocation, error) {
	if err := rejectLateProjectOptions(arguments); err != nil {
		return parsedInvocation{}, err
	}

	switch subcommand {
	case "wco":
		return parseCheckout(project, arguments)
	case "o":
		return parseOpenProject(project, arguments)
	case "wpr":
		return parsePullRequest(project, arguments)
	default:
		return parsedInvocation{}, usageError("unknown subcommand '" + subcommand + "'")
	}
}

func parseCheckout(project ProjectNameOrPath, arguments []string) (parsedInvocation, error) {
	if len(arguments) == 0 || arguments[0] == "" {
		return parsedInvocation{}, usageError("wco requires an existing local branch")
	}
	if arguments[0] == "-b" {
		if len(arguments) < 2 || arguments[1] == "" {
			return parsedInvocation{}, usageError("wco -b requires a new branch")
		}
		if len(arguments) > 3 {
			return parsedInvocation{}, usageError("wco -b accepts a new branch and optional start-point")
		}
		if len(arguments) == 3 && arguments[2] == "" {
			return parsedInvocation{}, usageError("wco -b requires a non-empty start-point when provided")
		}

		action := CheckoutNew{Branch: BranchName(arguments[1])}
		if len(arguments) == 3 {
			action.StartPoint = StartPoint(arguments[2])
		}
		return parsedInvocation{
			invocation:    Invocation{Project: project, Action: action},
			frameworkArgs: argumentsForCheckoutNew(action),
		}, nil
	}
	if strings.HasPrefix(arguments[0], "-") {
		return parsedInvocation{}, usageError("unknown wco option '" + arguments[0] + "'")
	}
	if len(arguments) != 1 {
		return parsedInvocation{}, usageError("wco accepts exactly one existing local branch")
	}

	action := CheckoutExisting{Branch: BranchName(arguments[0])}
	return parsedInvocation{
		invocation:    Invocation{Project: project, Action: action},
		frameworkArgs: []string{"zwm", "wco", string(action.Branch)},
	}, nil
}

func parseOpenProject(project ProjectNameOrPath, arguments []string) (parsedInvocation, error) {
	if project != "" {
		return parsedInvocation{}, usageError("o does not accept -C/--project")
	}
	if len(arguments) == 0 || arguments[0] == "" {
		return parsedInvocation{}, usageError("o requires a project name or path")
	}
	if len(arguments) != 1 {
		return parsedInvocation{}, usageError("o accepts exactly one project name or path")
	}
	if strings.HasPrefix(arguments[0], "-") {
		return parsedInvocation{}, usageError("invalid project name or path '" + arguments[0] + "'")
	}

	selectedProject := ProjectNameOrPath(arguments[0])
	return parsedInvocation{
		invocation:    Invocation{Project: selectedProject, Action: OpenProject{}},
		frameworkArgs: []string{"zwm", "o", string(selectedProject)},
	}, nil
}

func parsePullRequest(project ProjectNameOrPath, arguments []string) (parsedInvocation, error) {
	if len(arguments) == 0 {
		return parsedInvocation{}, usageError("wpr requires a pull request selector")
	}
	if len(arguments) != 1 {
		return parsedInvocation{}, usageError("wpr accepts exactly one pull request selector")
	}
	if arguments[0] == "" {
		return parsedInvocation{}, usageError("wpr requires a pull request selector")
	}
	if strings.HasPrefix(arguments[0], "-") {
		return parsedInvocation{}, usageError("invalid pull request selector '" + arguments[0] + "'")
	}

	action := PullRequest{Selector: PullRequestSelector(arguments[0])}
	return parsedInvocation{
		invocation:    Invocation{Project: project, Action: action},
		frameworkArgs: []string{"zwm", "wpr", string(action.Selector)},
	}, nil
}

func rejectLateProjectOptions(arguments []string) error {
	for _, argument := range arguments {
		if argument == "-C" || argument == "--project" || strings.HasPrefix(argument, "--project=") || strings.HasPrefix(argument, "-C") {
			return usageError("global option '" + argument + "' must appear before the subcommand")
		}
	}
	return nil
}

func argumentsForCheckoutNew(action CheckoutNew) []string {
	arguments := []string{"zwm", "wco", "-b", string(action.Branch)}
	if action.StartPoint != "" {
		return append(arguments, string(action.StartPoint))
	}
	return arguments
}

func usageError(message string) error {
	return errs.New(errs.Usage, message)
}
