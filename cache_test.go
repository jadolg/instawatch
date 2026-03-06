package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduleVideoDeletion(t *testing.T) {
	tmpDir := t.TempDir()

	videoPath := filepath.Join(tmpDir, "test_video.mp4")
	err := os.WriteFile(videoPath, []byte("dummy content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	urlHash := "test_hash"

	cacheMu.Lock()
	cache[urlHash] = videoInfo{Path: videoPath, Title: "Test Video"}
	cacheMu.Unlock()

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		t.Fatalf("Temp file %s should exist before deletion", videoPath)
	}

	delay := 2 * time.Second
	scheduleVideoDeletion(urlHash, videoPath, delay)

	time.Sleep(1 * time.Second)
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		t.Fatalf("Temp file %s was deleted too early (before %v)", videoPath, delay)
	}

	cacheMu.RLock()
	if _, exists := cache[urlHash]; !exists {
		t.Fatalf("Cache entry for %s was deleted too early", urlHash)
	}
	cacheMu.RUnlock()

	time.Sleep(1500 * time.Millisecond)

	if _, err := os.Stat(videoPath); !os.IsNotExist(err) {
		t.Fatalf("Temp file %s was not deleted after %v", videoPath, delay)
	}

	cacheMu.RLock()
	if _, exists := cache[urlHash]; exists {
		t.Fatalf("Cache entry for %s was not deleted after %v", urlHash, delay)
	}
	cacheMu.RUnlock()
}
