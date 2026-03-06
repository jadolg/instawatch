package main

import (
	"log"
	"os"
	"sync"
	"time"
)

type videoInfo struct {
	Path  string
	Title string
}

var (
	cache   = make(map[string]videoInfo)
	cacheMu sync.RWMutex
)

func scheduleVideoDeletion(urlHash, videoPath string, delay time.Duration) {
	time.AfterFunc(delay, func() {
		cacheMu.Lock()
		delete(cache, urlHash)
		cacheMu.Unlock()

		if err := os.Remove(videoPath); err != nil {
			log.Printf("Failed to delete video %s: %v", videoPath, err)
		} else {
			log.Printf("Deleted video after %v: %s", delay, videoPath)
		}
	})
}
