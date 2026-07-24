package app_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/app"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/github"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/stretchr/testify/require"
)

type realPullRequestFixture struct {
	project  project.Resolution
	primary  string
	source   string
	prCommit string
	gh       string
	ghRecord string
}

type githubInvocation struct {
	Directory string
	Arguments []string
}

type repositorySnapshot struct {
	refs       []byte
	worktrees  []byte
	sourceFile []byte
	status     []byte
}

func newRealPullRequestFixture(t *testing.T) realPullRequestFixture {
	t.Helper()

	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	root = canonicalRoot
	primary := filepath.Join(root, "project")
	runGit(t, root, "init", primary)
	runGit(t, primary, "config", "user.name", "ZWM Test")
	runGit(t, primary, "config", "user.email", "zwm@example.test")
	require.NoError(t, os.WriteFile(filepath.Join(primary, "tracked.txt"), []byte("base\n"), 0o600))
	runGit(t, primary, "add", "tracked.txt")
	runGit(t, primary, "commit", "--quiet", "-m", "initial")
	runGit(t, primary, "branch", "-M", "master")

	source := filepath.Join(root, "source")
	runGit(t, primary, "worktree", "add", "--quiet", "-b", "source/current", source, "master")
	require.NoError(t, os.WriteFile(filepath.Join(source, "source.txt"), []byte("source branch\n"), 0o600))
	runGit(t, source, "add", "source.txt")
	runGit(t, source, "commit", "--quiet", "-m", "source branch")

	fixtureWorktree := filepath.Join(root, "pull-request")
	runGit(t, primary, "worktree", "add", "--quiet", "-b", "fixture/pr-ready", fixtureWorktree, "master")
	require.NoError(t, os.WriteFile(filepath.Join(fixtureWorktree, "pull-request.txt"), []byte("pull request branch\n"), 0o600))
	runGit(t, fixtureWorktree, "add", "pull-request.txt")
	runGit(t, fixtureWorktree, "commit", "--quiet", "-m", "pull request fixture")

	gh, ghRecord := fakeRealGH(t)
	t.Setenv("GH_GIT", "git")
	t.Setenv("GH_COMMIT", strings.TrimSpace(runGit(t, fixtureWorktree, "rev-parse", "HEAD")))
	t.Setenv("GH_VIEW_STDOUT", "123\tfeature/pr-ready\n")

	return realPullRequestFixture{
		project: project.Resolution{
			InvocationWorktree: project.Directory(source),
			ManagedRoot:        project.Directory(filepath.Join(root, "code", ".wt", "project")),
			ProjectRoot:        project.Directory(primary),
		},
		primary:  primary,
		source:   source,
		prCommit: strings.TrimSpace(runGit(t, fixtureWorktree, "rev-parse", "HEAD")),
		gh:       gh,
		ghRecord: ghRecord,
	}
}

func realPullRequestService(fixture realPullRequestFixture) app.PullRequestService {
	return app.NewPullRequestService(
		app.NewSystemPullRequestGit("git"),
		github.NewClient(github.Config{Executable: fixture.gh}),
	)
}

func fakeRealGH(t *testing.T) (string, string) {
	t.Helper()

	directory := t.TempDir()
	helper := filepath.Join(directory, "gh")
	recordPath := filepath.Join(directory, "invocations")
	require.NoError(t, os.WriteFile(helper, []byte(`#!/bin/sh
if [ -n "${GH_RECORD:-}" ]; then
  printf '%s\037' "$PWD" >> "$GH_RECORD"
  for argument in "$@"; do
    printf '%s\037' "$argument" >> "$GH_RECORD"
  done
  printf '\n' >> "$GH_RECORD"
fi

case "$1:$2" in
  pr:view)
    if [ "$#" -ne 7 ] || [ "$4" != '--json' ] || [ "$5" != 'number,headRefName' ] || [ "$6" != '--jq' ] || [ "$7" != '[.number, .headRefName] | @tsv' ]; then
      printf 'fake gh: unexpected pr view arguments: %s\n' "$*" >&2
      exit 64
    fi
    printf '%s' "${GH_VIEW_STDOUT:-}"
    printf '%s' "${GH_VIEW_STDERR:-}" >&2
    exit "${GH_VIEW_EXIT:-0}"
    ;;
  pr:checkout)
    if [ "$#" -ne 5 ] || [ "$4" != '--branch' ]; then
      printf 'fake gh: unexpected pr checkout arguments: %s\n' "$*" >&2
      exit 64
    fi
    printf '%s' "${GH_CHECKOUT_STDERR:-}" >&2
    if [ "${GH_CHECKOUT_EXIT:-0}" -ne 0 ]; then
      exit "${GH_CHECKOUT_EXIT}"
    fi
    case "${GH_CHECKOUT_MODE:-branch}" in
      branch)
        "$GH_GIT" checkout --quiet -b "$5" "$GH_COMMIT"
        ;;
      false-success)
        ;;
      *)
        printf 'fake gh: invalid checkout mode %s\n' "$GH_CHECKOUT_MODE" >&2
        exit 64
        ;;
    esac
    ;;
  *)
    printf 'fake gh: unexpected command: %s\n' "$*" >&2
    exit 64
    ;;
esac
`), 0o700))
	t.Setenv("GH_RECORD", recordPath)

	return helper, recordPath
}

func githubInvocations(t *testing.T, recordPath string) []githubInvocation {
	t.Helper()

	raw, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	invocations := make([]githubInvocation, 0, len(lines))
	for _, line := range lines {
		fields := bytes.Split(bytes.TrimSuffix(line, []byte{0x1f}), []byte{0x1f})
		arguments := make([]string, len(fields)-1)
		for index, field := range fields[1:] {
			arguments[index] = string(field)
		}
		invocations = append(invocations, githubInvocation{Directory: string(fields[0]), Arguments: arguments})
	}
	return invocations
}

func clearGitHubInvocations(t *testing.T, recordPath string) {
	t.Helper()
	require.NoError(t, os.WriteFile(recordPath, nil, 0o600))
}

func snapshot(t *testing.T, fixture realPullRequestFixture) repositorySnapshot {
	t.Helper()

	return repositorySnapshot{
		refs:       runGitBytes(t, fixture.primary, "show-ref", "--head"),
		worktrees:  runGitBytes(t, fixture.primary, "worktree", "list", "--porcelain", "-z"),
		sourceFile: readFile(t, filepath.Join(fixture.source, "tracked.txt")),
		status:     runGitBytes(t, fixture.source, "status", "--porcelain=v1"),
	}
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	return string(runGitBytes(t, directory, arguments...))
}

func runGitBytes(t *testing.T, directory string, arguments ...string) []byte {
	t.Helper()

	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %s: %s", strings.Join(arguments, " "), output)
	return output
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return contents
}

func symbolicHead(t *testing.T, directory string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, directory, "symbolic-ref", "--short", "HEAD"))
}

func detachedHead(t *testing.T, directory string) {
	t.Helper()
	command := exec.Command("git", "-C", directory, "symbolic-ref", "--quiet", "HEAD")
	require.Error(t, command.Run())
}

func contextForTest(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
