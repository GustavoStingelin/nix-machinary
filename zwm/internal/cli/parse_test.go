package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRawGrammar_rejects_invalid_input_before_service_when_input_is_unsupported(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "missing subcommand", message: "missing subcommand"},
		{name: "unknown command", args: []string{"unknown"}, message: "unknown subcommand 'unknown'"},
		{name: "implicit help command", args: []string{"help"}, message: "unknown subcommand 'help'"},
		{name: "removed checkout command", args: []string{"co", "topic"}, message: "unknown subcommand 'co'"},
		{name: "removed pull request command", args: []string{"pr", "123"}, message: "unknown subcommand 'pr'"},

		{name: "missing short project value", args: []string{"-C"}, message: "flag needs an argument: -C"},
		{name: "long project value swallows following option", args: []string{"--project", "--help"}, message: "missing subcommand"},
		{name: "concatenated project option", args: []string{"-Crepo", "wco", "topic"}, message: "flag provided but not defined: -Crepo"},
		{name: "unknown global option", args: []string{"--unknown", "wco"}, message: "flag provided but not defined: -unknown"},

		{name: "missing checkout branch", args: []string{"wco"}, message: "wco requires an existing local branch"},
		{name: "empty checkout branch", args: []string{"wco", ""}, message: "wco requires an existing local branch"},
		{name: "option-like checkout branch", args: []string{"wco", "-topic"}, message: "flag provided but not defined: -topic"},
		{name: "numeric option-like checkout branch", args: []string{"wco", "-123"}, message: "unknown wco option '-123'"},
		{name: "extra checkout branch", args: []string{"wco", "topic", "extra"}, message: "wco accepts exactly one existing local branch"},
		{name: "missing new checkout branch", args: []string{"wco", "-b"}, message: "wco -b requires a new branch"},
		{name: "empty new checkout branch", args: []string{"wco", "-b", ""}, message: "wco -b requires a new branch"},
		{name: "option-like new checkout branch", args: []string{"wco", "-b", "-new-topic"}, message: "flag provided but not defined: -new-topic"},
		{name: "empty explicit start point", args: []string{"wco", "-b", "topic", ""}, message: "wco -b requires a non-empty start-point when provided"},
		{name: "extra new checkout argument", args: []string{"wco", "-b", "topic", "origin/main", "extra"}, message: "wco -b accepts a new branch and optional start-point"},
		{name: "nested checkout help flag", args: []string{"wco", "--help"}, message: "flag provided but not defined: -help"},

		{name: "missing open project", args: []string{"o"}, message: "o requires a project name or path"},
		{name: "empty open project", args: []string{"o", ""}, message: "o requires a project name or path"},
		{name: "extra open project argument", args: []string{"o", "repo", "extra"}, message: "o accepts exactly one project name or path"},
		{name: "option-like open project", args: []string{"o", "-repo"}, message: "flag provided but not defined: -repo"},
		{name: "short project option before open", args: []string{"-C", "selected", "o", "repo"}, message: "o does not accept -C/--project"},
		{name: "long project option before open", args: []string{"--project", "selected", "o", "repo"}, message: "o does not accept -C/--project"},
		{name: "project option after open", args: []string{"o", "repo", "--project", "selected"}, message: "o does not accept -C/--project"},
		{name: "equals project option after open", args: []string{"o", "repo", "--project=selected"}, message: "o does not accept -C/--project"},

		{name: "missing pull request selector", args: []string{"wpr"}, message: "wpr requires a pull request selector"},
		{name: "empty pull request selector", args: []string{"wpr", ""}, message: "wpr requires a pull request selector"},
		{name: "extra pull request selector", args: []string{"wpr", "123", "extra"}, message: "wpr accepts exactly one pull request selector"},
		{name: "hyphen pull request selector", args: []string{"wpr", "-123"}, message: "invalid pull request selector '-123'"},
		{name: "nested pull request help flag", args: []string{"wpr", "--help"}, message: "flag provided but not defined: -help"},
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

func TestRawGrammar_delegates_approved_input_when_values_are_valid(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		result         Result
		expectedOutput string
		assertion      func(*testing.T, Invocation)
	}{
		{
			name: "existing checkout with project",
			args: []string{"--project", "named-project", "wco", "feature/topic"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("named-project"), invocation.Project)
				require.Equal(t, CheckoutExisting{Branch: BranchName("feature/topic")}, invocation.Action)
			},
		},
		{
			name: "existing checkout with equals project option",
			args: []string{"--project=named-project", "wco", "feature/topic"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("named-project"), invocation.Project)
				require.Equal(t, CheckoutExisting{Branch: BranchName("feature/topic")}, invocation.Action)
			},
		},
		{
			name: "existing checkout with project option after subcommand",
			args: []string{"wco", "feature/topic", "--project", "named-project"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("named-project"), invocation.Project)
				require.Equal(t, CheckoutExisting{Branch: BranchName("feature/topic")}, invocation.Action)
			},
		},
		{
			name: "new checkout with explicit start point",
			args: []string{"-C", "path/project", "wco", "-b", "new/topic", "origin/main"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("path/project"), invocation.Project)
				require.Equal(t, CheckoutNew{Branch: BranchName("new/topic"), StartPoint: StartPoint("origin/main")}, invocation.Action)
			},
		},
		{
			name: "new checkout with omitted start point",
			args: []string{"wco", "-b", "new/topic"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, CheckoutNew{Branch: BranchName("new/topic")}, invocation.Action)
			},
		},
		{
			name: "open project",
			args: []string{"o", "named-project"},
			result: OpenProjectResult{
				ProjectRoot: "/tmp/project",
				TabAction:   "created",
				TabTitle:    "project",
				TabCwd:      "/tmp/project",
			},
			expectedOutput: expectedOpenProjectResultOutput,
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, ProjectNameOrPath("named-project"), invocation.Project)
				require.Equal(t, OpenProject{}, invocation.Action)
			},
		},
		{
			name: "pull request selector",
			args: []string{"wpr", "https://github.com/org/repo/pull/123"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Equal(t, PullRequest{Selector: PullRequestSelector("https://github.com/org/repo/pull/123")}, invocation.Action)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			service := &recordingService{result: test.result}
			expectedOutput := test.expectedOutput
			if expectedOutput == "" {
				expectedOutput = expectedResultOutput
			}

			// When
			exitCode, stdout, stderr := runCLI(t, service, test.args...)

			// Then
			require.Equal(t, 0, exitCode)
			require.Equal(t, expectedOutput, stdout)
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
	exitCode, stdout, stderr := runCLI(t, service, "-C", "first", "--project", "second", "wco", "topic")

	// Then
	require.Equal(t, 0, exitCode)
	require.Equal(t, expectedResultOutput, stdout)
	require.Empty(t, stderr)
	require.Len(t, service.invocations, 1)
	require.Equal(t, ProjectNameOrPath("second"), service.invocations[0].Project)
	require.Equal(t, CheckoutExisting{Branch: BranchName("topic")}, service.invocations[0].Action)
}
