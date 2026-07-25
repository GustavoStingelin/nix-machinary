package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListNames_returns_sorted_project_directories_excluding_reserved_and_hidden(t *testing.T) {
	// Given
	home := t.TempDir()
	codeRoot := filepath.Join(home, "code")
	for _, name := range []string{"zebra", "alpha", ".hidden", ".wt", "middle"} {
		require.NoError(t, os.MkdirAll(filepath.Join(codeRoot, name), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(codeRoot, "loose-file"), []byte("x"), 0o600))

	// When
	names := ListNames(Directory(home))

	// Then
	require.Equal(t, []string{"alpha", "middle", "zebra"}, names)
}

func TestListNames_returns_nothing_when_code_root_is_missing(t *testing.T) {
	// When
	names := ListNames(Directory(t.TempDir()))

	// Then
	require.Empty(t, names)
}
