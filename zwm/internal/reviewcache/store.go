// Package reviewcache persists the last review queue the dashboard fetched, so
// the section has something to show the moment it opens instead of an empty
// panel while GitHub is queried.
//
// This is a cache, not state: every entry is reconstructible from GitHub, and
// losing the file costs one slow render rather than any information. That is why
// it lives under XDG_CACHE_HOME while agent records live under XDG_STATE_HOME.
package reviewcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Review is one cached pull request. It mirrors what the dashboard renders,
// deliberately as its own type: the cache file is a serialization format that
// outlives any one build, so it must not move whenever a view struct does.
type Review struct {
	Number     string `json:"number"`
	Repository string `json:"repository"`
	Project    string `json:"project,omitempty"`
	Title      string `json:"title"`
	Author     string `json:"author,omitempty"`
	Base       string `json:"base,omitempty"`
	Head       string `json:"head,omitempty"`
	Worktree   string `json:"worktree,omitempty"`
	Stale      bool   `json:"stale,omitempty"`
}

// Snapshot is a whole queue plus when it was read, so the dashboard can say how
// old the rows on screen are rather than presenting cached data as current.
type Snapshot struct {
	FetchedAt time.Time `json:"fetched_at"`
	Reviews   []Review  `json:"reviews"`
}

// Path returns the cache file, honoring XDG_CACHE_HOME and falling back to
// ~/.cache. lookup is os.LookupEnv in production; tests pass a fake.
func Path(lookup func(string) (string, bool)) string {
	if base, ok := lookup("XDG_CACHE_HOME"); ok && base != "" {
		return filepath.Join(base, "zwm", "reviews.json")
	}
	home, _ := lookup("HOME")
	return filepath.Join(home, ".cache", "zwm", "reviews.json")
}

// Store reads and writes one snapshot file.
type Store struct {
	path string
	now  func() time.Time
}

func NewStore(path string) *Store {
	return &Store{path: path, now: time.Now}
}

// Load returns the cached snapshot. The second result is false when there is no
// usable cache — missing, unreadable, or malformed all look the same to a
// caller, because in every case the answer is "fetch it fresh". A torn file must
// never surface as an error the dashboard has to explain.
func (store *Store) Load() (Snapshot, bool) {
	data, err := os.ReadFile(store.path)
	if err != nil {
		return Snapshot{}, false
	}
	var snapshot Snapshot
	if json.Unmarshal(data, &snapshot) != nil {
		return Snapshot{}, false
	}
	if snapshot.FetchedAt.IsZero() {
		// Without a timestamp the age can't be shown honestly, so treat it as no
		// cache rather than implying the rows are current.
		return Snapshot{}, false
	}
	return snapshot, true
}

// Save writes the snapshot atomically, stamping FetchedAt when the caller left
// it zero. Writing through a temp file and renaming means a dashboard starting
// up never reads a half-written queue.
func (store *Store) Save(snapshot Snapshot) error {
	if snapshot.FetchedAt.IsZero() {
		snapshot.FetchedAt = store.now()
	}
	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(directory, ".reviews-*.tmp")
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
	return os.Rename(tempPath, store.path)
}
