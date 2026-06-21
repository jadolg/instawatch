package main

import (
	"log"
	"os"
	"sync"
	"time"
)

type videoInfo struct {
	Path         string
	Title        string
	Description  string
	OriginalURL  string
	LastAccessed time.Time
}

const (
	videoRetention  = 24 * time.Hour
	janitorInterval = time.Hour
)

var (
	cache   = make(map[string]videoInfo)
	cacheMu sync.RWMutex
)

// touchVideo refreshes the last-accessed time of a cached video so that files
// people are still watching are kept alive instead of being re-downloaded.
func touchVideo(urlHash string) {
	cacheMu.Lock()
	if info, ok := cache[urlHash]; ok {
		info.LastAccessed = time.Now()
		cache[urlHash] = info
	}
	cacheMu.Unlock()
}

// startCacheJanitor periodically deletes videos that have not been accessed
// within the retention period.
func startCacheJanitor(interval, retention time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredVideos(retention)
		}
	}()
}

func cleanupExpiredVideos(retention time.Duration) {
	now := time.Now()

	cacheMu.Lock()
	expiredPaths := make(map[string]string)
	for urlHash, info := range cache {
		if now.Sub(info.LastAccessed) >= retention {
			expiredPaths[urlHash] = info.Path
			delete(cache, urlHash)
		}
	}
	cacheMu.Unlock()

	for _, videoPath := range expiredPaths {
		if err := os.Remove(videoPath); err != nil {
			log.Printf("Failed to delete video %s: %v", videoPath, err)
		} else {
			log.Printf("Deleted video not accessed for %v: %s", retention, videoPath)
		}
	}
}
