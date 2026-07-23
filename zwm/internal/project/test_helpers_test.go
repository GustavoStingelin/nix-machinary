package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type realRepository struct{}

func (realRepository) WorktreeRoot(ctx context.Context, directory Directory) (Directory, error) {
	return gitPath(ctx, string(directory), "rev-parse", "--show-toplevel")
}

func (realRepository) PrimaryWorktreeRoot(ctx context.Context, worktree Directory) (Directory, error) {
	output, err := gitOutput(ctx, string(worktree), "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", err
	}

	for attribute := range bytes.SplitSeq(output, []byte{0}) {
		const prefix = "worktree "
		if bytes.HasPrefix(attribute, []byte(prefix)) {
			return Directory(string(attribute[len(prefix):])), nil
		}
	}
	return "", fmt.Errorf("worktree porcelain did not contain a worktree path")
}

func gitPath(ctx context.Context, directory string, arguments ...string) (Directory, error) {
	output, err := gitOutput(ctx, directory, arguments...)
	if err != nil {
		return "", err
	}
	return Directory(strings.TrimSuffix(string(output), "\n")), nil
}

func gitOutput(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
	args := make([]string, 0, len(arguments)+2)
	args = append(args, "-C", directory)
	args = append(args, arguments...)
	return exec.CommandContext(ctx, "git", args...).Output()
}

func initializeRepository(t *testing.T, directory string) {
	t.Helper()
	makeDirectory(t, directory)
	runGit(t, directory, "init", "--quiet")
	writeFile(t, filepath.Join(directory, "tracked.txt"), "initial\n")
	runGit(t, directory, "add", "tracked.txt")
	runGit(t, directory, "-c", "user.name=zwm test", "-c", "user.email=zwm@example.invalid", "commit", "--quiet", "-m", "initial")
}

func makeDirectory(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create directory %q: %v", directory, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func runGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	args := make([]string, 0, len(arguments)+2)
	args = append(args, "-C", directory)
	args = append(args, arguments...)
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %q: %v\n%s", arguments, err, output)
	}
}

func canonicalDirectory(t *testing.T, directory string) string {
	t.Helper()
	abs, err := filepath.Abs(directory)
	if err != nil {
		t.Fatalf("make %q absolute: %v", directory, err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatalf("canonicalize %q: %v", directory, err)
	}
	return canonical
}

func shortCanonicalHash(t *testing.T, directory string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(canonicalDirectory(t, directory)))
	return hex.EncodeToString(sum[:])[:8]
}

type sourceSnapshot struct {
	content   []byte
	refs      []byte
	status    []byte
	worktrees []byte
}

func snapshotSource(t *testing.T, root string) sourceSnapshot {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, "tracked.txt"))
	if err != nil {
		t.Fatalf("read tracked source content: %v", err)
	}
	status, err := gitOutput(context.Background(), root, "status", "--porcelain=v1")
	if err != nil {
		t.Fatalf("read source status: %v", err)
	}
	refs, err := gitOutput(context.Background(), root, "show-ref", "--head")
	if err != nil {
		t.Fatalf("read source refs: %v", err)
	}
	worktrees, err := gitOutput(context.Background(), root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		t.Fatalf("read source worktrees: %v", err)
	}
	return sourceSnapshot{content: content, refs: refs, status: status, worktrees: worktrees}
}

func assertSourceUnchanged(t *testing.T, before sourceSnapshot, root string) {
	t.Helper()
	after := snapshotSource(t, root)
	if !bytes.Equal(before.content, after.content) {
		t.Fatal("project resolution changed tracked source content")
	}
	if !bytes.Equal(before.refs, after.refs) {
		t.Fatal("project resolution changed source refs")
	}
	if !bytes.Equal(before.status, after.status) {
		t.Fatal("project resolution changed source status")
	}
	if !bytes.Equal(before.worktrees, after.worktrees) {
		t.Fatal("project resolution changed registered worktrees")
	}
}
