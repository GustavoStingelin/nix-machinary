package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

type identityRepository struct{}

func (identityRepository) WorktreeRoot(_ context.Context, directory Directory) (Directory, error) {
	return directory, nil
}

func (identityRepository) PrimaryWorktreeRoot(_ context.Context, worktree Directory) (Directory, error) {
	return worktree, nil
}

type cancelledRepository struct {
	called bool
}

func (repository *cancelledRepository) WorktreeRoot(ctx context.Context, _ Directory) (Directory, error) {
	repository.called = true
	return "", ctx.Err()
}

func (repository *cancelledRepository) PrimaryWorktreeRoot(context.Context, Directory) (Directory, error) {
	return "", errors.New("unreachable")
}

func TestResolveProject_retainsRawDirectKeys_whenBareValuesAreSafe(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		key   Key
	}{
		{name: "space", value: "space project", key: "space project"},
		{name: "tilde name", value: "~foo", key: "~foo"},
		{name: "unicode", value: "项目", key: "项目"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			home := filepath.Join(t.TempDir(), "home")
			projectRoot := filepath.Join(home, "code", string(test.value))
			makeDirectory(t, projectRoot)
			resolver := NewResolver(identityRepository{})

			// When
			result, err := resolver.Resolve(context.Background(), Request{
				Home:             Directory(home),
				Project:          test.value,
				WorkingDirectory: Directory(t.TempDir()),
			})

			// Then
			if err != nil {
				t.Fatalf("resolve project: %v", err)
			}
			wantRoot := Directory(canonicalDirectory(t, projectRoot))
			if result.InvocationWorktree != wantRoot || result.ProjectRoot != wantRoot {
				t.Fatalf("resolved roots = %+v, want %q", result, wantRoot)
			}
			if result.Key != test.key {
				t.Fatalf("key = %q, want %q", result.Key, test.key)
			}
			wantManaged := Directory(filepath.Join(canonicalDirectory(t, home), "code", ".wt", string(test.key)))
			if result.ManagedRoot != wantManaged {
				t.Fatalf("managed root = %q, want %q", result.ManagedRoot, wantManaged)
			}
		})
	}
}

func TestResolveProject_usesConfiguredWorkingDirectory_whenProjectValueIsPathLike(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	workingDirectory := filepath.Join(t.TempDir(), "working")
	child := filepath.Join(workingDirectory, "child")
	projectRoot := filepath.Join(home, "code", "project")
	makeDirectory(t, child)
	makeDirectory(t, projectRoot)
	resolver := NewResolver(identityRepository{})

	// When
	result, err := resolver.Resolve(context.Background(), Request{
		Home:             Directory(home),
		Project:          "./child",
		WorkingDirectory: Directory(workingDirectory),
	})

	// Then
	if err != nil {
		t.Fatalf("resolve relative project: %v", err)
	}
	want := Directory(canonicalDirectory(t, child))
	if result.InvocationWorktree != want || result.ProjectRoot != want {
		t.Fatalf("resolved roots = %+v, want %q", result, want)
	}
}

func TestResolveProject_preservesPathLikeSelectionRules_whenResolvingCandidates(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	workingDirectory := filepath.Join(t.TempDir(), "working")
	parentDirectory := filepath.Dir(workingDirectory)
	nested := filepath.Join(workingDirectory, "nested")
	directProject := filepath.Join(home, "code", "project")
	external := filepath.Join(t.TempDir(), "external")
	makeDirectory(t, nested)
	makeDirectory(t, directProject)
	makeDirectory(t, external)
	resolver := NewResolver(identityRepository{})
	tests := []struct {
		name       string
		value      Value
		want       string
		wantReject bool
	}{
		{name: "omitted", want: workingDirectory},
		{name: "dot", value: ".", want: workingDirectory},
		{name: "dot dot", value: "..", want: parentDirectory},
		{name: "home", value: "~", wantReject: true},
		{name: "home slash", value: "~/", wantReject: true},
		{name: "home child", value: "~/code/project", want: directProject},
		{name: "relative", value: "./nested", want: nested},
		{name: "absolute", value: Value(external), want: external},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			result, err := resolver.Resolve(context.Background(), Request{
				Home:             Directory(home),
				Project:          test.value,
				WorkingDirectory: Directory(workingDirectory),
			})

			// Then
			if test.wantReject {
				if !errors.Is(err, errs.ErrProject) {
					t.Fatalf("resolve %q error = %v, want project class", test.value, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve %q: %v", test.value, err)
			}
			want := Directory(canonicalDirectory(t, test.want))
			if result.InvocationWorktree != want || result.ProjectRoot != want {
				t.Fatalf("resolved roots = %+v, want %q", result, want)
			}
		})
	}
}

func TestResolveProject_derivesStableExternalKey_whenBasenameRequiresSanitizing(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	external := filepath.Join(t.TempDir(), "Café 项目")
	makeDirectory(t, filepath.Join(home, "code"))
	makeDirectory(t, external)
	resolver := NewResolver(identityRepository{})
	request := Request{Home: Directory(home), Project: Value(external), WorkingDirectory: Directory(t.TempDir())}

	// When
	first, firstErr := resolver.Resolve(context.Background(), request)
	second, secondErr := resolver.Resolve(context.Background(), request)

	// Then
	if firstErr != nil || secondErr != nil {
		t.Fatalf("resolve external project: %v / %v", firstErr, secondErr)
	}
	canonical := canonicalDirectory(t, external)
	sum := sha256.Sum256([]byte(canonical))
	wantKey := Key("Caf-" + hex.EncodeToString(sum[:])[:8])
	if first.Key != wantKey || second.Key != wantKey {
		t.Fatalf("keys = %q / %q, want %q", first.Key, second.Key, wantKey)
	}
	wantManaged := Directory(filepath.Join(canonicalDirectory(t, home), "code", ".wt", string(wantKey)))
	if first.ManagedRoot != wantManaged {
		t.Fatalf("managed root = %q, want %q", first.ManagedRoot, wantManaged)
	}
	if first != second {
		t.Fatalf("repeated resolution was unstable: %+v / %+v", first, second)
	}
}

func TestResolveProject_usesProjectFallback_whenExternalBasenameHasNoASCIIAlphanumericBytes(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	external := filepath.Join(t.TempDir(), "项目")
	makeDirectory(t, filepath.Join(home, "code"))
	makeDirectory(t, external)
	resolver := NewResolver(identityRepository{})

	// When
	result, err := resolver.Resolve(context.Background(), Request{
		Home:             Directory(home),
		Project:          Value(external),
		WorkingDirectory: Directory(t.TempDir()),
	})

	// Then
	if err != nil {
		t.Fatalf("resolve external project: %v", err)
	}
	canonical := canonicalDirectory(t, external)
	sum := sha256.Sum256([]byte(canonical))
	want := Key("project-" + hex.EncodeToString(sum[:])[:8])
	if result.Key != want {
		t.Fatalf("key = %q, want %q", result.Key, want)
	}
	wantManaged := Directory(filepath.Join(canonicalDirectory(t, home), "code", ".wt", string(want)))
	if result.ManagedRoot != wantManaged {
		t.Fatalf("managed root = %q, want %q", result.ManagedRoot, wantManaged)
	}
}

func TestResolveProject_treatsLegacyWorktreesDirectoryAsOrdinaryProjectPath(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	projectRoot := filepath.Join(home, "code", ".worktrees", "legacy-project")
	makeDirectory(t, projectRoot)
	resolver := NewResolver(identityRepository{})

	// When
	result, err := resolver.Resolve(context.Background(), Request{
		Home:             Directory(home),
		Project:          Value(projectRoot),
		WorkingDirectory: Directory(t.TempDir()),
	})

	// Then
	if err != nil {
		t.Fatalf("resolve legacy worktrees project: %v", err)
	}
	canonical := canonicalDirectory(t, projectRoot)
	sum := sha256.Sum256([]byte(canonical))
	wantKey := Key("legacy-project-" + hex.EncodeToString(sum[:])[:8])
	wantManaged := Directory(filepath.Join(canonicalDirectory(t, home), "code", ".wt", string(wantKey)))
	if result.Key != wantKey {
		t.Fatalf("key = %q, want %q", result.Key, wantKey)
	}
	if result.ManagedRoot != wantManaged {
		t.Fatalf("managed root = %q, want %q", result.ManagedRoot, wantManaged)
	}
}

func TestResolveProject_returnsProjectError_whenRepositoryReceivesCanceledContext(t *testing.T) {
	// Given
	home := filepath.Join(t.TempDir(), "home")
	workingDirectory := filepath.Join(t.TempDir(), "working")
	makeDirectory(t, filepath.Join(home, "code"))
	makeDirectory(t, workingDirectory)
	repository := &cancelledRepository{}
	resolver := NewResolver(repository)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	_, err := resolver.Resolve(ctx, Request{Home: Directory(home), WorkingDirectory: Directory(workingDirectory)})

	// Then
	if !repository.called {
		t.Fatal("resolver did not call the context-aware repository seam")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if !errors.Is(err, errs.ErrProject) {
		t.Fatalf("error = %v, want project class", err)
	}
}
