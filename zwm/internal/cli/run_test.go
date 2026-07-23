package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/stretchr/testify/require"
)

const expectedHelp = "Usage:\n" +
	"  zwm --help | -h\n" +
	"  zwm [-C <name-or-path> | --project <name-or-path>] <command>\n" +
	"\n" +
	"Commands:\n" +
	"  co <branch> | co -b <new-branch> [<start-point>]\n" +
	"  pr <number|url|branch>\n" +
	"\n" +
	"Global options:\n" +
	"  -C <name-or-path>          Select a project before the subcommand.\n" +
	"  --project <name-or-path>   Select a project before the subcommand.\n"

func TestHelp_prints_exact_help_when_root_help_is_alone(t *testing.T) {
	// Given
	service := &recordingService{}

	// When
	exitCode, stdout, stderr := runCLI(t, service, "--help")

	// Then
	require.Equal(t, 0, exitCode)
	require.Equal(t, expectedHelp, stdout)
	require.Empty(t, stderr)
	require.Empty(t, service.invocations)
}

func TestHelp_prints_help_when_short_root_help_is_alone(t *testing.T) {
	// Given
	service := &recordingService{}

	// When
	exitCode, stdout, stderr := runCLI(t, service, "-h")

	// Then
	require.Equal(t, 0, exitCode)
	require.Equal(t, expectedHelp, stdout)
	require.Empty(t, stderr)
	require.Empty(t, service.invocations)
}

func TestRawGrammar_rejects_invalid_input_before_service_when_input_is_unsupported(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "missing subcommand", message: "missing subcommand"},
		{name: "missing short project value", args: []string{"-C"}, message: "option '-C' requires a value"},
		{name: "missing long project value", args: []string{"--project", "--help"}, message: "option '--project' requires a value"},
		{name: "equals project option", args: []string{"--project=repo", "co", "topic"}, message: "unknown global option '--project=repo'"},
		{name: "concatenated project option", args: []string{"-Crepo", "co", "topic"}, message: "unknown global option '-Crepo'"},
		{name: "unknown global option", args: []string{"--unknown", "co"}, message: "unknown global option '--unknown'"},
		{name: "unknown command", args: []string{"unknown"}, message: "unknown subcommand 'unknown'"},
		{name: "late long project option", args: []string{"co", "topic", "--project", "repo"}, message: "global option '--project' must appear before the subcommand"},
		{name: "late short project option", args: []string{"co", "topic", "-C", "repo"}, message: "global option '-C' must appear before the subcommand"},
		{name: "late equals project option", args: []string{"co", "topic", "--project=repo"}, message: "global option '--project=repo' must appear before the subcommand"},
		{name: "late concatenated project option", args: []string{"co", "topic", "-Crepo"}, message: "global option '-Crepo' must appear before the subcommand"},
		{name: "implicit help command", args: []string{"help"}, message: "unknown subcommand 'help'"},
		{name: "nested checkout help", args: []string{"co", "--help"}, message: "unknown co option '--help'"},
		{name: "nested pull request help", args: []string{"pr", "--help"}, message: "invalid pull request selector '--help'"},
		{name: "help with another argument", args: []string{"--help", "co"}, message: "option '--help' must be used on its own"},
		{name: "missing checkout branch", args: []string{"co"}, message: "co requires an existing local branch"},
		{name: "empty checkout branch", args: []string{"co", ""}, message: "co requires an existing local branch"},
		{name: "missing new checkout branch", args: []string{"co", "-b"}, message: "co -b requires a new branch"},
		{name: "empty new checkout branch", args: []string{"co", "-b", ""}, message: "co -b requires a new branch"},
		{name: "empty explicit start point", args: []string{"co", "-b", "topic", ""}, message: "co -b requires a non-empty start-point when provided"},
		{name: "extra new checkout argument", args: []string{"co", "-b", "topic", "origin/main", "extra"}, message: "co -b accepts a new branch and optional start-point"},
		{name: "checkout option", args: []string{"co", "-topic"}, message: "unknown co option '-topic'"},
		{name: "missing pull request selector", args: []string{"pr"}, message: "pr requires a pull request selector"},
		{name: "empty pull request selector", args: []string{"pr", ""}, message: "pr requires a pull request selector"},
		{name: "extra pull request selector", args: []string{"pr", "123", "extra"}, message: "pr accepts exactly one pull request selector"},
		{name: "hyphen pull request selector", args: []string{"pr", "-123"}, message: "invalid pull request selector '-123'"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			service := &recordingService{}

			// When
			exitCode, stdout, stderr := runCLI(t, service, test.args...)

			// Then
			require.Equal(t, 64, exitCode)
			require.Empty(t, stdout)
			require.Equal(t, "zwm: usage: "+test.message+"\n", stderr)
			require.Empty(t, service.invocations)
		})
	}
}

func TestExitFormatting_formats_service_errors_at_the_cli_boundary(t *testing.T) {
	// Given
	service := &recordingService{err: errs.New(errs.Project, "cannot resolve project")}

	// When
	exitCode, stdout, stderr := runCLI(t, service, "co", "feature/topic")

	// Then
	require.Equal(t, 65, exitCode)
	require.Empty(t, stdout)
	require.Equal(t, "zwm: project: cannot resolve project\n", stderr)
	require.Len(t, service.invocations, 1)
}

func TestRawGrammar_delegates_approved_input_when_values_are_valid(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		assertion func(*testing.T, Invocation)
	}{
		{
			name: "existing checkout with project",
			args: []string{"--project", "named-project", "co", "feature/topic"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("named-project"), invocation.Project)
				require.Equal(t, CheckoutExisting{Branch: BranchName("feature/topic")}, invocation.Action)
			},
		},
		{
			name: "new checkout with explicit start point",
			args: []string{"-C", "path/project", "co", "-b", "new/topic", "origin/main"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("path/project"), invocation.Project)
				require.Equal(t, CheckoutNew{Branch: BranchName("new/topic"), StartPoint: StartPoint("origin/main")}, invocation.Action)
			},
		},
		{
			name: "new checkout with omitted start point",
			args: []string{"co", "-b", "new/topic"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, CheckoutNew{Branch: BranchName("new/topic")}, invocation.Action)
			},
		},
		{
			name: "new checkout preserves hyphen positional values",
			args: []string{"co", "-b", "-new-topic", "-start-point"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Empty(t, invocation.Project)
				require.Equal(t, CheckoutNew{Branch: BranchName("-new-topic"), StartPoint: StartPoint("-start-point")}, invocation.Action)
			},
		},
		{
			name: "pull request selector",
			args: []string{"pr", "https://github.com/org/repo/pull/123"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, PullRequest{Selector: PullRequestSelector("https://github.com/org/repo/pull/123")}, invocation.Action)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			service := &recordingService{}

			// When
			exitCode, stdout, stderr := runCLI(t, service, test.args...)

			// Then
			require.Equal(t, 0, exitCode)
			require.Equal(t, expectedResultOutput, stdout)
			require.Empty(t, stderr)
			require.Len(t, service.invocations, 1)
			test.assertion(t, service.invocations[0])
		})
	}
}

func TestRawGrammar_uses_last_project_option_when_project_options_repeat(t *testing.T) {
	// Given
	service := &recordingService{}

	// When
	exitCode, stdout, stderr := runCLI(t, service, "-C", "first", "--project", "second", "co", "topic")

	// Then
	require.Equal(t, 0, exitCode)
	require.Equal(t, expectedResultOutput, stdout)
	require.Empty(t, stderr)
	require.Len(t, service.invocations, 1)
	require.Equal(t, ProjectNameOrPath("second"), service.invocations[0].Project)
	require.Equal(t, CheckoutExisting{Branch: BranchName("topic")}, service.invocations[0].Action)
}

func runCLI(t *testing.T, service Service, args ...string) (int, string, string) {
	t.Helper()

	var stdout strings.Builder
	var stderr strings.Builder
	exitCode := Run(context.Background(), Config{
		Arguments: args,
		Service:   service,
		Stdout:    &stdout,
		Stderr:    &stderr,
	})

	return exitCode, stdout.String(), stderr.String()
}

type recordingService struct {
	invocations []Invocation
	result      Result
	err         error
}

func (service *recordingService) Execute(_ context.Context, invocation Invocation) (Result, error) {
	service.invocations = append(service.invocations, invocation)
	if service.result == (Result{}) {
		return testResult, service.err
	}
	return service.result, service.err
}

var testResult = Result{
	Worktree:        "/tmp/worktree",
	DisplayIdentity: "feature/topic",
	TabAction:       "created",
	TabTitle:        "project:feature/topic",
	TabWorktree:     "/tmp/worktree",
}

const expectedResultOutput = "worktree_path=/tmp/worktree\n" +
	"display_identity=feature/topic\n" +
	"tab_action=created\n" +
	"tab_title=project:feature/topic\n" +
	"tab_worktree=/tmp/worktree\n"
