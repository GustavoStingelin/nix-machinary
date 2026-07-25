package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/stretchr/testify/require"
)

const expectedHelp = "NAME:\n" +
	"   zwm - Zellij worktree manager\n" +
	"\n" +
	"USAGE:\n" +
	"   zwm [global options] [command [command options]]\n" +
	"\n" +
	"COMMANDS:\n" +
	"   wco  check out a branch in a worktree\n" +
	"   o    open a project\n" +
	"   wpr  check out a pull request in a worktree\n" +
	"\n" +
	"GLOBAL OPTIONS:\n" +
	"   --project string, -C string  select a project before the subcommand\n" +
	"   --help, -h                   show help\n"

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

func TestExitFormatting_formats_service_errors_at_the_cli_boundary(t *testing.T) {
	// Given
	service := &recordingService{err: errs.New(errs.Project, "cannot resolve project")}

	// When
	exitCode, stdout, stderr := runCLI(t, service, "wco", "feature/topic")

	// Then
	require.Equal(t, 65, exitCode)
	require.Empty(t, stdout)
	require.Equal(t, "zwm: project: cannot resolve project\n", stderr)
	require.Len(t, service.invocations, 1)
}

func TestResultRendering_preserves_worktree_output_for_checkout_and_pull_request(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "checkout", args: []string{"wco", "feature/topic"}},
		{name: "pull request", args: []string{"wpr", "123"}},
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
		})
	}
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

func runCLIWithCompleter(t *testing.T, completer Completer, args ...string) (int, string, string) {
	t.Helper()

	var stdout strings.Builder
	var stderr strings.Builder
	exitCode := Run(context.Background(), Config{
		Arguments: args,
		Service:   &recordingService{},
		Completer: completer,
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
	if service.result == nil {
		return testResult, service.err
	}
	return service.result, service.err
}

var testResult = WorktreeResult{
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

const expectedOpenProjectResultOutput = "project_root=/tmp/project\n" +
	"tab_action=created\n" +
	"tab_title=project\n" +
	"tab_cwd=/tmp/project\n"
