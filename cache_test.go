package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupExpiredVideos(t *testing.T) {
	tmpDir := t.TempDir()

	freshPath := filepath.Join(tmpDir, "fresh.mp4")
	stalePath := filepath.Join(tmpDir, "stale.mp4")
	for _, p := range []string{freshPath, stalePath} {
		if err := os.WriteFile(p, []byte("dummy content"), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
	}

	retention := time.Hour

	cacheMu.Lock()
	cache["fresh_hash"] = videoInfo{Path: freshPath, LastAccessed: time.Now()}
	cache["stale_hash"] = videoInfo{Path: stalePath, LastAccessed: time.Now().Add(-2 * retention)}
	cacheMu.Unlock()

	t.Cleanup(func() {
		cacheMu.Lock()
		delete(cache, "fresh_hash")
		delete(cache, "stale_hash")
		cacheMu.Unlock()
	})

	cleanupExpiredVideos(retention)

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("Stale video %s should have been deleted", stalePath)
	}
	cacheMu.RLock()
	_, staleExists := cache["stale_hash"]
	cacheMu.RUnlock()
	if staleExists {
		t.Fatalf("Stale cache entry should have been removed")
	}

	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("Fresh video %s should not have been deleted: %v", freshPath, err)
	}
	cacheMu.RLock()
	_, freshExists := cache["fresh_hash"]
	cacheMu.RUnlock()
	if !freshExists {
		t.Fatalf("Fresh cache entry should have been kept")
	}
}

func TestTouchVideo(t *testing.T) {
	urlHash := "touch_hash"
	old := time.Now().Add(-time.Hour)

	cacheMu.Lock()
	cache[urlHash] = videoInfo{Path: "/tmp/whatever.mp4", LastAccessed: old}
	cacheMu.Unlock()

	t.Cleanup(func() {
		cacheMu.Lock()
		delete(cache, urlHash)
		cacheMu.Unlock()
	})

	touchVideo(urlHash)

	cacheMu.RLock()
	updated := cache[urlHash].LastAccessed
	cacheMu.RUnlock()

	if !updated.After(old) {
		t.Fatalf("touchVideo did not refresh LastAccessed: got %v, want after %v", updated, old)
	}
}
