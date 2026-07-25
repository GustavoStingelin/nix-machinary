package project

import (
	"os"
	"path/filepath"
	"sort"
)

// managedRootsDirectory is the reserved directory under the code root that
// holds managed worktrees; it is never a selectable project.
const managedRootsDirectory = ".wt"

// ListNames returns the selectable project names under the home code root
// (its immediate subdirectories, excluding hidden entries and the managed
// worktree root). It is intended for shell completion and returns an empty
// slice when the code root is unavailable.
func ListNames(home Directory) []string {
	codeRoot := canonicalCodeRoot(home)
	entries, err := os.ReadDir(codeRoot)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == managedRootsDirectory || name[0] == '.' {
			continue
		}
		if isDirectory(entry, filepath.Join(codeRoot, name)) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// isDirectory reports whether the entry is a directory, following symlinks so
// symlinked project checkouts are still offered.
func isDirectory(entry os.DirEntry, path string) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
