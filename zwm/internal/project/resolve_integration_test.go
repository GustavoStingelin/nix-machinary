package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

func TestResolveProject_normalizesLinkedWorktreeAndPreservesDirtySource(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	primary := filepath.Join(home, "code", "project")
	linked := filepath.Join(t.TempDir(), "linked project")
	initializeRepository(t, primary)
	runGit(t, primary, "worktree", "add", "--quiet", "-b", "linked-project", linked)
	writeFile(t, filepath.Join(primary, "tracked.txt"), "dirty\n")
	before := snapshotSource(t, primary)
	resolver := NewResolver(realRepository{})

	// When
	result, err := resolver.Resolve(context.Background(), Request{
		Home:             Directory(home),
		WorkingDirectory: Directory(linked),
	})

	// Then
	if err != nil {
		t.Fatalf("resolve linked worktree: %v", err)
	}
	if result.InvocationWorktree != Directory(canonicalDirectory(t, linked)) {
		t.Fatalf("invocation worktree = %q, want %q", result.InvocationWorktree, linked)
	}
	if result.ProjectRoot != Directory(canonicalDirectory(t, primary)) {
		t.Fatalf("project root = %q, want %q", result.ProjectRoot, primary)
	}
	if result.Key != "project" {
		t.Fatalf("key = %q, want project", result.Key)
	}
	if result.ManagedRoot != Directory(filepath.Join(canonicalDirectory(t, home), "code", ".worktrees", "project")) {
		t.Fatalf("managed root = %q", result.ManagedRoot)
	}
	assertSourceUnchanged(t, before, primary)
}

func TestResolveProject_disambiguatesSameBasenameAndCanonicalizesSymlinks(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	first := filepath.Join(t.TempDir(), "external one", "shared")
	second := filepath.Join(t.TempDir(), "external-two", "shared")
	link := filepath.Join(t.TempDir(), "external symlink")
	initializeRepository(t, first)
	initializeRepository(t, second)
	makeDirectory(t, filepath.Join(home, "code"))
	if err := os.Symlink(first, link); err != nil {
		t.Fatalf("make symlink: %v", err)
	}
	resolver := NewResolver(realRepository{})
	request := func(value string) Request {
		return Request{Home: Directory(home), Project: Value(value), WorkingDirectory: Directory(t.TempDir())}
	}

	// When
	firstResult, firstErr := resolver.Resolve(context.Background(), request(first))
	firstRepeat, firstRepeatErr := resolver.Resolve(context.Background(), request(first))
	secondResult, secondErr := resolver.Resolve(context.Background(), request(second))
	linkResult, linkErr := resolver.Resolve(context.Background(), request(link))

	// Then
	if firstErr != nil || firstRepeatErr != nil || secondErr != nil || linkErr != nil {
		t.Fatalf("resolve externals: %v / %v / %v / %v", firstErr, firstRepeatErr, secondErr, linkErr)
	}
	if firstResult.Key == secondResult.Key {
		t.Fatalf("same-basename external projects collided at key %q", firstResult.Key)
	}
	if firstRepeat != firstResult {
		t.Fatalf("repeated resolution = %+v, want %+v", firstRepeat, firstResult)
	}
	if linkResult != firstResult {
		t.Fatalf("symlink resolution = %+v, want %+v", linkResult, firstResult)
	}
}

func TestResolveProject_sanitizesExternalBasenamesWithoutRejectingControlBytes(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	spaced := filepath.Join(t.TempDir(), "Space Project")
	control := filepath.Join(t.TempDir(), "unsafe\nproject")
	initializeRepository(t, spaced)
	initializeRepository(t, control)
	makeDirectory(t, filepath.Join(home, "code"))
	resolver := NewResolver(realRepository{})

	// When
	spacedResult, spacedErr := resolver.Resolve(context.Background(), Request{
		Home: Directory(home), Project: Value(spaced), WorkingDirectory: Directory(t.TempDir()),
	})
	controlResult, controlErr := resolver.Resolve(context.Background(), Request{
		Home: Directory(home), Project: Value(control), WorkingDirectory: Directory(t.TempDir()),
	})

	// Then
	if spacedErr != nil || controlErr != nil {
		t.Fatalf("resolve external projects: %v / %v", spacedErr, controlErr)
	}
	spacedWant := Key("Space-Project-" + shortCanonicalHash(t, spaced))
	if spacedResult.Key != spacedWant {
		t.Fatalf("spaced key = %q, want %q", spacedResult.Key, spacedWant)
	}
	controlWant := Key("unsafe-project-" + shortCanonicalHash(t, control))
	if controlResult.Key != controlWant {
		t.Fatalf("control key = %q, want %q", controlResult.Key, controlWant)
	}
}

func TestResolveProject_rejectsUnsafeDirectKeyWithoutChangingRepository(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	unsafeName := "unsafe\nproject"
	projectRoot := filepath.Join(home, "code", unsafeName)
	initializeRepository(t, projectRoot)
	before := snapshotSource(t, projectRoot)
	resolver := NewResolver(realRepository{})

	// When
	_, err := resolver.Resolve(context.Background(), Request{
		Home:             Directory(home),
		Project:          Value(unsafeName),
		WorkingDirectory: Directory(t.TempDir()),
	})

	// Then
	if !errors.Is(err, errs.ErrProject) {
		t.Fatalf("error = %v, want project class", err)
	}
	wantMessage := "direct project basename contains control characters and cannot be used as a project key"
	if err.Error() != wantMessage {
		t.Fatalf("error = %q, want %q", err, wantMessage)
	}
	assertSourceUnchanged(t, before, projectRoot)
}
