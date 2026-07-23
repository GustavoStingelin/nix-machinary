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
		{name: "missing short project value", args: []string{"-C"}, message: "option '-C' requires a value"},
		{name: "missing long project value", args: []string{"--project", "--help"}, message: "option '--project' requires a value"},
		{name: "equals project option", args: []string{"--project=repo", "wco", "topic"}, message: "unknown global option '--project=repo'"},
		{name: "concatenated project option", args: []string{"-Crepo", "wco", "topic"}, message: "unknown global option '-Crepo'"},
		{name: "unknown global option", args: []string{"--unknown", "wco"}, message: "unknown global option '--unknown'"},
		{name: "unknown command", args: []string{"unknown"}, message: "unknown subcommand 'unknown'"},
		{name: "late long project option", args: []string{"wco", "topic", "--project", "repo"}, message: "global option '--project' must appear before the subcommand"},
		{name: "late short project option", args: []string{"wco", "topic", "-C", "repo"}, message: "global option '-C' must appear before the subcommand"},
		{name: "late equals project option", args: []string{"wco", "topic", "--project=repo"}, message: "global option '--project=repo' must appear before the subcommand"},
		{name: "late concatenated project option", args: []string{"wco", "topic", "-Crepo"}, message: "global option '-Crepo' must appear before the subcommand"},
		{name: "implicit help command", args: []string{"help"}, message: "unknown subcommand 'help'"},
		{name: "nested checkout help", args: []string{"wco", "--help"}, message: "unknown wco option '--help'"},
		{name: "nested pull request help", args: []string{"pr", "--help"}, message: "invalid pull request selector '--help'"},
		{name: "help with another argument", args: []string{"--help", "wco"}, message: "option '--help' must be used on its own"},
		{name: "removed checkout command", args: []string{"co", "topic"}, message: "unknown subcommand 'co'"},
		{name: "missing checkout branch", args: []string{"wco"}, message: "wco requires an existing local branch"},
		{name: "empty checkout branch", args: []string{"wco", ""}, message: "wco requires an existing local branch"},
		{name: "missing new checkout branch", args: []string{"wco", "-b"}, message: "wco -b requires a new branch"},
		{name: "empty new checkout branch", args: []string{"wco", "-b", ""}, message: "wco -b requires a new branch"},
		{name: "empty explicit start point", args: []string{"wco", "-b", "topic", ""}, message: "wco -b requires a non-empty start-point when provided"},
		{name: "extra new checkout argument", args: []string{"wco", "-b", "topic", "origin/main", "extra"}, message: "wco -b accepts a new branch and optional start-point"},
		{name: "checkout option", args: []string{"wco", "-topic"}, message: "unknown wco option '-topic'"},
		{name: "extra checkout branch", args: []string{"wco", "topic", "extra"}, message: "wco accepts exactly one existing local branch"},
		{name: "missing open project", args: []string{"o"}, message: "o requires a project name or path"},
		{name: "empty open project", args: []string{"o", ""}, message: "o requires a project name or path"},
		{name: "extra open project argument", args: []string{"o", "repo", "extra"}, message: "o accepts exactly one project name or path"},
		{name: "option-like open project", args: []string{"o", "-repo"}, message: "invalid project name or path '-repo'"},
		{name: "short project option before open", args: []string{"-C", "selected", "o", "repo"}, message: "o does not accept -C/--project"},
		{name: "long project option before open", args: []string{"--project", "selected", "o", "repo"}, message: "o does not accept -C/--project"},
		{name: "late long project option for open", args: []string{"o", "repo", "--project", "selected"}, message: "global option '--project' must appear before the subcommand"},
		{name: "late short project option for open", args: []string{"o", "repo", "-C", "selected"}, message: "global option '-C' must appear before the subcommand"},
		{name: "late equals project option for open", args: []string{"o", "repo", "--project=selected"}, message: "global option '--project=selected' must appear before the subcommand"},
		{name: "late concatenated project option for open", args: []string{"o", "repo", "-Cselected"}, message: "global option '-Cselected' must appear before the subcommand"},
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
			name: "new checkout preserves hyphen positional values",
			args: []string{"wco", "-b", "-new-topic", "-start-point"},
			assertion: func(t *testing.T, invocation Invocation) {
				require.Empty(t, invocation.Project)
				require.Equal(t, CheckoutNew{Branch: BranchName("-new-topic"), StartPoint: StartPoint("-start-point")}, invocation.Action)
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
			args: []string{"pr", "https://github.com/org/repo/pull/123"},
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

func TestRawGrammar_forwards_exact_framework_arguments_when_worktree_input_is_valid(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "existing branch",
			args:     []string{"wco", "feature/topic"},
			expected: []string{"zwm", "wco", "feature/topic"},
		},
		{
			name:     "new branch without start point",
			args:     []string{"wco", "-b", "new/topic"},
			expected: []string{"zwm", "wco", "-b", "new/topic"},
		},
		{
			name:     "new branch with start point",
			args:     []string{"wco", "-b", "new/topic", "origin/main"},
			expected: []string{"zwm", "wco", "-b", "new/topic", "origin/main"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			parsed, err := parse(test.args)

			// Then
			require.NoError(t, err)
			require.Equal(t, test.expected, parsed.frameworkArgs)
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
