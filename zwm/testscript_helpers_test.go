package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
	"github.com/rogpeppe/go-internal/testscript"
)

type commandRecord struct {
	Sequence int      `json:"sequence"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	CWD      string   `json:"cwd"`
}

func setupScript(env *testscript.Env) error {
	home := filepath.Join(env.WorkDir, "home")
	projectPath := filepath.Join(home, "code", "project")
	newlineProjectPath := filepath.Join(env.WorkDir, "external", "newline\nproject")
	for _, repository := range []string{projectPath, newlineProjectPath} {
		if err := initializeRepository(repository); err != nil {
			return err
		}
	}
	if err := runGit(projectPath, "branch", "feature/ready"); err != nil {
		return err
	}
	if err := runGit(newlineProjectPath, "branch", "feature/newline"); err != nil {
		return err
	}

	binaryPath := filepath.Join(env.WorkDir, "bin", "zwm")
	if err := buildBinary(binaryPath); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(env.WorkDir, "empty"), 0o755); err != nil {
		return fmt.Errorf("create empty path: %w", err)
	}
	commit, err := gitOutput(projectPath, "rev-parse", "HEAD")
	if err != nil {
		return err
	}

	managedRoot := filepath.Join(home, "code", ".worktrees", "project")
	newlineKey := externalKey(newlineProjectPath)
	env.Setenv("HOME", home)
	env.Setenv("PATH", filepath.Dir(binaryPath)+string(os.PathListSeparator)+env.Getenv("PATH"))
	env.Setenv("ZWM_TEST_PATH", env.Getenv("PATH"))
	env.Setenv("ZELLIJ", "test-session")
	env.Setenv("ZWM_TEST_GH_COMMIT", commit)
	env.Setenv("ZWM_TEST_LOG", filepath.Join(env.WorkDir, "external-log.jsonl"))
	env.Setenv("ZWM_TEST_TABS", filepath.Join(env.WorkDir, "tabs"))
	env.Setenv("ZWM_TEST_REAL_GIT", gitExecutable())
	env.Setenv("ZWM_TEST_EXISTING_PATH", string(worktree.ManagedWorktreePath(worktree.Path(managedRoot), "feature/ready")))
	env.Setenv("ZWM_TEST_SELECTED_PATH", string(worktree.ManagedWorktreePath(worktree.Path(managedRoot), "feature/selected")))
	env.Setenv("ZWM_TEST_NEW_PATH", string(worktree.ManagedWorktreePath(worktree.Path(managedRoot), "feature/new")))
	env.Setenv("ZWM_TEST_NEWLINE_PROJECT", newlineProjectPath)
	env.Setenv("ZWM_TEST_NEWLINE_KEY", newlineKey)
	env.Setenv("ZWM_TEST_NEWLINE_PATH", string(worktree.ManagedWorktreePath(worktree.Path(filepath.Join(home, "code", ".worktrees", newlineKey)), "feature/newline")))
	env.Setenv("ZWM_TEST_PR_PATH", string(worktree.ManagedWorktreePath(worktree.Path(managedRoot), "pr-123-feature/pr-ready")))
	env.Setenv("ZWM_TEST_PR_BRANCH", pullRequestBranch(projectPath, "123", "feature/pr-ready"))
	env.Setenv("ZWM_TEST_FAILURE_PR_PATH", string(worktree.ManagedWorktreePath(worktree.Path(managedRoot), "pr-456-feature/failure")))
	env.Setenv("ZWM_TEST_FAILURE_PR_BRANCH", pullRequestBranch(projectPath, "456", "feature/failure"))
	return nil
}

func initializeRepository(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create repository directory: %w", err)
	}
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "zwm-test@example.invalid"},
		{"config", "user.name", "zwm test"},
	} {
		if err := runGit(path, arguments...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(path, "tracked.txt"), []byte("initial\n"), 0o600); err != nil {
		return fmt.Errorf("write tracked fixture: %w", err)
	}
	for _, arguments := range [][]string{{"add", "tracked.txt"}, {"commit", "--quiet", "-m", "initial"}} {
		if err := runGit(path, arguments...); err != nil {
			return err
		}
	}
	return nil
}

func buildBinary(path string) error {
	moduleRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine module root: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}
	command := exec.Command("go", "build", "-buildvcs=false", "-o", path, ".")
	command.Dir = moduleRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build zwm: %w: %s", err, output)
	}
	return nil
}

func fakeGit() {
	recordCommand("git", os.Args[1:])
	if os.Getenv("ZWM_TEST_GIT_FAILURE") == "worktree-add" && slices.Equal(os.Args[1:3], []string{"worktree", "add"}) {
		fmt.Fprintln(os.Stderr, "fake git: injected worktree add failure")
		mainExit(75)
	}
	runExternal(gitExecutable(), os.Args[1:])
}

func runRealGit() {
	runExternal(gitExecutable(), os.Args[1:])
}

func fakeGitHub() {
	recordCommand("gh", os.Args[1:])
	arguments := os.Args[1:]
	if slices.Equal(arguments[:2], []string{"pr", "view"}) {
		runPullRequestView(arguments)
		return
	}
	if slices.Equal(arguments[:2], []string{"pr", "checkout"}) {
		runPullRequestCheckout(arguments)
		return
	}
	fmt.Fprintln(os.Stderr, "fake gh: unexpected command")
	mainExit(64)
}

func runPullRequestView(arguments []string) {
	if len(arguments) != 7 || !slices.Equal(arguments[3:], []string{"--json", "number,headRefName", "--jq", "[.number, .headRefName] | @tsv"}) {
		fmt.Fprintln(os.Stderr, "fake gh: unexpected pr view arguments")
		mainExit(64)
	}
	switch arguments[2] {
	case "123", "https://github.com/example/project/pull/123", "feature/pr-ready":
		fmt.Fprint(os.Stdout, "123\tfeature/pr-ready\n")
	case "456":
		fmt.Fprint(os.Stdout, "456\tfeature/failure\n")
	case "789":
		fmt.Fprint(os.Stdout, "789\tfeature/malformed\nunexpected\n")
	default:
		fmt.Fprintf(os.Stderr, "fake gh: pull request selector %s was not found\n", arguments[2])
		mainExit(4)
	}
}

func runPullRequestCheckout(arguments []string) {
	if len(arguments) != 5 || arguments[3] != "--branch" {
		fmt.Fprintln(os.Stderr, "fake gh: unexpected pr checkout arguments")
		mainExit(64)
	}
	if arguments[2] == "456" {
		fmt.Fprintln(os.Stderr, "fake gh: injected checkout failure for 456")
		mainExit(75)
	}
	if arguments[2] != "123" && arguments[2] != "https://github.com/example/project/pull/123" && arguments[2] != "feature/pr-ready" {
		fmt.Fprintln(os.Stderr, "fake gh: unexpected successful checkout selector")
		mainExit(64)
	}
	runExternal(gitExecutable(), []string{"checkout", "--quiet", "-b", arguments[4], os.Getenv("ZWM_TEST_GH_COMMIT")})
}

func fakeZellij() {
	recordCommand("zellij", os.Args[1:])
	arguments := os.Args[1:]
	switch {
	case slices.Equal(arguments, []string{"action", "query-tab-names"}):
		contents, err := os.ReadFile(os.Getenv("ZWM_TEST_TABS"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, err)
			mainExit(1)
		}
		_, _ = os.Stdout.Write(contents)
	case len(arguments) == 3 && slices.Equal(arguments[:2], []string{"action", "go-to-tab-name"}):
		if !tabExists(arguments[2]) {
			fmt.Fprintln(os.Stderr, "fake zellij: tab does not exist")
			mainExit(4)
		}
		writeFakeZellijOutput()
	case len(arguments) == 8 && slices.Equal(arguments[:2], []string{"action", "new-tab"}) && arguments[2] == "--layout" && arguments[4] == "--name" && arguments[6] == "--cwd":
		if tabExists(arguments[5]) {
			fmt.Fprintln(os.Stderr, "fake zellij: duplicate tab")
			mainExit(1)
		}
		appendFile(os.Getenv("ZWM_TEST_TABS"), []byte(arguments[5]+"\n"))
		writeFakeZellijOutput()
	default:
		fmt.Fprintln(os.Stderr, "fake zellij: unexpected command")
		mainExit(64)
	}
}

func writeFakeZellijOutput() {
	if output := os.Getenv("ZWM_TEST_ZELLIJ_OUTPUT"); output != "" {
		fmt.Fprint(os.Stdout, output)
	}
}

func tabExists(title string) bool {
	contents, err := os.ReadFile(os.Getenv("ZWM_TEST_TABS"))
	if err != nil {
		return false
	}
	return slices.Contains(strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n"), title)
}

func recordCommand(name string, arguments []string) {
	directory, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	path := os.Getenv("ZWM_TEST_LOG")
	sequence := commandCount(path) + 1
	record := commandRecord{Sequence: sequence, Command: name, Args: arguments, CWD: directory}
	encoded, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	appendFile(path, append(encoded, '\n'))
}

func commandCount(path string) int {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	return count
}

func appendFile(path string, contents []byte) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
}

func runExternal(executable string, arguments []string) {
	command := exec.Command(executable, arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err == nil {
		return
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		mainExit(exitError.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	mainExit(1)
}

func runGit(directory string, arguments ...string) error {
	command := exec.Command(gitExecutable(), arguments...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, output)
	}
	return nil
}

func gitOutput(directory string, arguments ...string) (string, error) {
	command := exec.Command(gitExecutable(), arguments...)
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(arguments, " "), err)
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func gitExecutable() string {
	if executable := os.Getenv("ZWM_TEST_REAL_GIT"); executable != "" {
		return executable
	}
	if testRealGit != "" {
		return testRealGit
	}
	executable, err := exec.LookPath("git")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		mainExit(1)
	}
	return executable
}

func externalKey(projectPath string) string {
	baseName := filepath.Base(projectPath)
	display := worktree.ManagedDisplay(baseName)
	hash := sha256.Sum256([]byte(projectPath))
	return display + "-" + hex.EncodeToString(hash[:])[:8]
}

func pullRequestBranch(projectPath, number, head string) string {
	identity := fmt.Sprintf("project:%d:%s\nnumber:%d:%s\nhead:%d:%s\n", len(projectPath), projectPath, len(number), number, len(head), head)
	hash := sha256.Sum256([]byte(identity))
	return "zwm/pr-" + number + "-" + hex.EncodeToString(hash[:])[:8]
}

func assertExit(script *testscript.TestScript, negated bool, arguments []string) {
	if negated || len(arguments) < 2 {
		script.Fatalf("usage: assert-exit <status> <command> [arguments...]")
	}
	expected, err := strconv.Atoi(arguments[0])
	if err != nil {
		script.Fatalf("parse expected exit status: %v", err)
	}
	err = script.Exec(arguments[1], arguments[2:]...)
	if expected == 0 && err == nil {
		return
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != expected {
		script.Fatalf("expected exit status %d, got %v", expected, err)
	}
}

func assertLog(script *testscript.TestScript, negated bool, arguments []string) {
	if negated || len(arguments) != 2 {
		script.Fatalf("usage: assert-log <actual> <expected>")
	}
	actual := decodeRecords(script, arguments[0], false)
	expected := decodeRecords(script, arguments[1], true)
	if !slices.EqualFunc(actual, expected, func(left, right commandRecord) bool {
		return left.Sequence == right.Sequence && left.Command == right.Command && left.CWD == right.CWD && slices.Equal(left.Args, right.Args)
	}) {
		script.Fatalf("structured external command log differs\nactual: %#v\nexpected: %#v", actual, expected)
	}
	for _, record := range actual {
		for _, argument := range record.Args {
			if slices.Contains([]string{"--force", "-B", "reset", "remove", "prune", "repair", "move", "session", "attach", "switch"}, argument) {
				script.Fatalf("forbidden structured command argument %q in %#v", argument, record)
			}
		}
	}
}

func decodeRecords(script *testscript.TestScript, path string, expandVariables bool) []commandRecord {
	contents, err := os.ReadFile(script.MkAbs(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		script.Fatalf("open structured log: %v", err)
	}
	if expandVariables {
		contents = []byte(os.Expand(string(contents), script.Getenv))
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	var records []commandRecord
	for {
		var record commandRecord
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records
		}
		if err != nil {
			script.Fatalf("decode structured log: %v", err)
		}
		record.CWD = strings.ReplaceAll(record.CWD, "\\", "/")
		records = append(records, record)
	}
}
