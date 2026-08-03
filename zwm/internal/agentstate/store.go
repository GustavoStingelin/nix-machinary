package agentstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Dir returns the directory holding agent state records, honoring
// XDG_STATE_HOME and falling back to ~/.local/state. lookup is os.LookupEnv in
// production; tests pass a fake. This is state (not cache), and XDG_RUNTIME_DIR
// is usually unset on darwin, so ~/.local/state is the portable default.
func Dir(lookup func(string) (string, bool)) string {
	if base, ok := lookup("XDG_STATE_HOME"); ok && base != "" {
		return filepath.Join(base, "zwm", "agents")
	}
	home, _ := lookup("HOME")
	return filepath.Join(home, ".local", "state", "zwm", "agents")
}

// Store persists and loads agent attention records under a base directory.
type Store struct {
	dir string
	now func() time.Time
}

// NewStore returns a Store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir, now: time.Now}
}

// Write persists a record atomically at <session>/<pane_id>.json. The session
// name is sanitized for the directory only; the record keeps the raw name, so
// readers group by the stored name and never depend on the sanitization being
// reversible.
func (store *Store) Write(record Record) error {
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = store.now()
	}
	sessionDir := filepath.Join(store.dir, sanitize(record.Session))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	// Write to a temp file in the same directory and rename, so a concurrent
	// reader never observes a half-written record.
	temp, err := os.CreateTemp(sessionDir, "."+sanitize(record.PaneID)+"-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	return os.Rename(tempPath, filepath.Join(sessionDir, sanitize(record.PaneID)+".json"))
}

// Delete removes the record for a session+pane. A missing file is not an error,
// so a stale agent can be forgotten idempotently.
func (store *Store) Delete(session, paneID string) error {
	path := filepath.Join(store.dir, sanitize(session), sanitize(paneID)+".json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Load reads every record under the base directory. Missing directory, and
// unreadable or malformed files, are skipped rather than failing the caller —
// a single torn or stale file must not blank the whole dashboard.
func (store *Store) Load() ([]Record, error) {
	matches, err := filepath.Glob(filepath.Join(store.dir, "*", "*.json"))
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(matches))
	for _, match := range matches {
		data, readErr := os.ReadFile(match)
		if readErr != nil {
			continue
		}
		var record Record
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

// Prune removes records for sessions not in live and records older than ttl,
// keeping the directory from growing without bound as panes and sessions close.
// Zellij 0.43.1 gives no event when a pane closes, so ttl is the backstop. It is
// best-effort: removal errors are ignored.
func (store *Store) Prune(live []string, ttl time.Duration) {
	liveSet := make(map[string]struct{}, len(live))
	for _, name := range live {
		liveSet[name] = struct{}{}
	}
	cutoff := store.now().Add(-ttl)

	sessionDirs, err := filepath.Glob(filepath.Join(store.dir, "*"))
	if err != nil {
		return
	}
	for _, sessionDir := range sessionDirs {
		files, globErr := filepath.Glob(filepath.Join(sessionDir, "*.json"))
		if globErr != nil {
			continue
		}
		remaining := 0
		for _, file := range files {
			if store.stale(file, liveSet, cutoff) {
				os.Remove(file)
				continue
			}
			remaining++
		}
		if remaining == 0 {
			os.Remove(sessionDir)
		}
	}
}

func (store *Store) stale(file string, live map[string]struct{}, cutoff time.Time) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return true
	}
	var record Record
	if json.Unmarshal(data, &record) != nil {
		return true
	}
	if _, ok := live[record.Session]; !ok {
		return true
	}
	return record.UpdatedAt.Before(cutoff)
}

// sanitize maps a session name or pane id to a filesystem-safe path segment.
// Collisions are possible but harmless: readers group by the raw session name
// stored inside each record, so the directory name is only an organizational
// hint used for cheap per-session cleanup.
func sanitize(name string) string {
	if name == "" {
		return "_"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, name)
}
