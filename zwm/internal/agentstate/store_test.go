package agentstate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDir_prefers_xdg_state_home(t *testing.T) {
	env := func(key string) (string, bool) {
		switch key {
		case "XDG_STATE_HOME":
			return "/xdg/state", true
		case "HOME":
			return "/home/head", true
		}
		return "", false
	}
	require.Equal(t, filepath.Join("/xdg/state", "zwm", "agents"), Dir(env))
}

func TestDir_falls_back_to_home_local_state(t *testing.T) {
	env := func(key string) (string, bool) {
		if key == "HOME" {
			return "/home/head", true
		}
		return "", false
	}
	require.Equal(t, filepath.Join("/home/head", ".local", "state", "zwm", "agents"), Dir(env))
}

func TestStore_writes_then_loads_records_grouped_by_raw_session(t *testing.T) {
	store := NewStore(t.TempDir())

	// A session name with filesystem-hostile characters exercises sanitization
	// while the raw name is preserved in the record.
	require.NoError(t, store.Write(Record{Session: "feat/x y", PaneID: "3", Agent: "claude", State: Waiting}))
	require.NoError(t, store.Write(Record{Session: "bitcoin", PaneID: "7", State: Working}))

	records, err := store.Load()
	require.NoError(t, err)
	require.Len(t, records, 2)

	bySession := map[string]Record{}
	for _, record := range records {
		bySession[record.Session] = record
	}
	require.Equal(t, Waiting, bySession["feat/x y"].State)
	require.Equal(t, "claude", bySession["feat/x y"].Agent)
	require.Equal(t, Working, bySession["bitcoin"].State)
	require.False(t, bySession["bitcoin"].UpdatedAt.IsZero(), "UpdatedAt is stamped on write")
}

func TestStore_write_replaces_the_record_for_the_same_pane(t *testing.T) {
	store := NewStore(t.TempDir())

	require.NoError(t, store.Write(Record{Session: "bitcoin", PaneID: "3", State: Working}))
	require.NoError(t, store.Write(Record{Session: "bitcoin", PaneID: "3", State: Waiting}))

	records, err := store.Load()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, Waiting, records[0].State)
}

func TestStore_load_tolerates_a_missing_directory(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"))
	records, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestStore_prune_drops_dead_sessions_and_stale_records(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	store := &Store{dir: dir, now: func() time.Time { return now }}

	require.NoError(t, store.Write(Record{Session: "live", PaneID: "1", State: Working, UpdatedAt: now}))
	require.NoError(t, store.Write(Record{Session: "dead", PaneID: "1", State: Done, UpdatedAt: now}))
	require.NoError(t, store.Write(Record{Session: "live", PaneID: "2", State: Done, UpdatedAt: now.Add(-48 * time.Hour)}))

	store.Prune([]string{"live"}, 24*time.Hour)

	records, err := store.Load()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "live", records[0].Session)
	require.Equal(t, "1", records[0].PaneID)
}
