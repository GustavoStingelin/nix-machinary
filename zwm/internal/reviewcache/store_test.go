package reviewcache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/reviewcache"
	"github.com/stretchr/testify/require"
)

func TestPath_honors_XDG_CACHE_HOME_then_falls_back_to_home(t *testing.T) {
	xdg := reviewcache.Path(func(name string) (string, bool) {
		if name == "XDG_CACHE_HOME" {
			return "/xdg", true
		}
		return "/home/user", true
	})
	require.Equal(t, filepath.Join("/xdg", "zwm", "reviews.json"), xdg)

	fallback := reviewcache.Path(func(name string) (string, bool) {
		if name == "HOME" {
			return "/home/user", true
		}
		return "", false
	})
	require.Equal(t, filepath.Join("/home/user", ".cache", "zwm", "reviews.json"), fallback)
}

func TestStore_round_trips_a_snapshot_and_creates_its_directory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "reviews.json")
	store := reviewcache.NewStore(path)
	fetched := time.Now().Add(-90 * time.Minute).Round(time.Second)

	require.NoError(t, store.Save(reviewcache.Snapshot{
		FetchedAt: fetched,
		Reviews: []reviewcache.Review{{
			Number: "1305", Repository: "btcsuite/btcwallet", Project: "btcwallet",
			Title: "wallet: define watch-only", Author: "yyforyongyu",
			Base: "itests/watch-only-create", Head: "task-watch-policy",
			Worktree: "/wt/pr-1305", Stale: true,
		}},
	}))

	snapshot, ok := reviewcache.NewStore(path).Load()
	require.True(t, ok)
	require.True(t, fetched.Equal(snapshot.FetchedAt))
	require.Len(t, snapshot.Reviews, 1)
	// The base branch is the field a review actually depends on, so it must
	// survive the round trip intact.
	require.Equal(t, "itests/watch-only-create", snapshot.Reviews[0].Base)
	require.True(t, snapshot.Reviews[0].Stale)
}

func TestStore_stamps_the_fetch_time_when_the_caller_leaves_it_zero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.json")
	before := time.Now()

	require.NoError(t, reviewcache.NewStore(path).Save(reviewcache.Snapshot{}))

	snapshot, ok := reviewcache.NewStore(path).Load()
	require.True(t, ok)
	require.False(t, snapshot.FetchedAt.Before(before.Truncate(time.Second)))
}

func TestStore_Load_reports_no_cache_rather_than_failing(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{
		"missing":      "",
		"malformed":    "{not json",
		"no timestamp": `{"reviews":[{"number":"1"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(directory, name+".json")
			if content != "" {
				require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
			}

			_, ok := reviewcache.NewStore(path).Load()

			// Every one of these means "fetch it fresh". A torn or ancient-format
			// file must never become an error the dashboard has to explain, and a
			// snapshot with no timestamp can't have its age shown honestly.
			require.False(t, ok)
		})
	}
}

func TestStore_Save_leaves_no_temp_files_behind(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "reviews.json")
	store := reviewcache.NewStore(path)

	require.NoError(t, store.Save(reviewcache.Snapshot{FetchedAt: time.Now()}))
	require.NoError(t, store.Save(reviewcache.Snapshot{FetchedAt: time.Now()}))

	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, entries, 1, "the temp file must be renamed into place, not accumulate")
	require.Equal(t, "reviews.json", entries[0].Name())
}

func TestStore_Save_replaces_the_previous_snapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.json")
	store := reviewcache.NewStore(path)
	require.NoError(t, store.Save(reviewcache.Snapshot{
		FetchedAt: time.Now(), Reviews: []reviewcache.Review{{Number: "1"}, {Number: "2"}},
	}))

	require.NoError(t, store.Save(reviewcache.Snapshot{
		FetchedAt: time.Now(), Reviews: []reviewcache.Review{{Number: "3"}},
	}))

	snapshot, ok := store.Load()
	require.True(t, ok)
	require.Len(t, snapshot.Reviews, 1)
	require.Equal(t, "3", snapshot.Reviews[0].Number)
}
