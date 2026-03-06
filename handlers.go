package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type indexData struct {
	Error string
}

func renderIndex(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := templates.ExecuteTemplate(w, "index.html", indexData{Error: errMsg}); err != nil {
		log.Printf("Error rendering index template: %v", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request, tmpDir string) {
	path := r.URL.Path

	if path == "/" {
		if err := templates.ExecuteTemplate(w, "index.html", indexData{}); err != nil {
			log.Printf("Error rendering index template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	rawURL := strings.TrimPrefix(path, "/")

	if r.URL.RawQuery != "" {
		http.Redirect(w, r, path, http.StatusMovedPermanently)
		return
	}

	igURL, err := validateInstagramURL(rawURL)
	if err != nil {
		renderIndex(w, http.StatusBadRequest, err.Error())
		return
	}

	urlHash := hashURL(igURL)
	cacheMu.RLock()
	cachedVal, exists := cache[urlHash]
	cacheMu.RUnlock()

	if exists {
		if _, err := os.Stat(cachedVal.Path); err == nil {
			servePlayer(w, urlHash, cachedVal.Title)
			return
		}
		cacheMu.Lock()
		delete(cache, urlHash)
		cacheMu.Unlock()
	}

	videoPath, title, err := downloadVideo(igURL, tmpDir, urlHash)
	if err != nil {
		log.Printf("Error downloading video from %s: %v", igURL, err)
		renderIndex(w, http.StatusInternalServerError, "Could not download video. Please check the URL and try again.")
		return
	}

	cacheMu.Lock()
	cache[urlHash] = videoInfo{Path: videoPath, Title: title}
	cacheMu.Unlock()

	scheduleVideoDeletion(urlHash, videoPath, 1*time.Hour)

	servePlayer(w, urlHash, title)
}

func servePlayer(w http.ResponseWriter, urlHash, title string) {
	data := struct {
		VideoURL string
		Title    string
	}{
		VideoURL: videoRoute + urlHash,
		Title:    title,
	}
	if err := templates.ExecuteTemplate(w, "player.html", data); err != nil {
		log.Printf("Error rendering player template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleVideo(w http.ResponseWriter, r *http.Request) {
	urlHash := r.PathValue("hash")
	if urlHash == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Prevents path traversal and cache poisoning.
	if !hashPattern.MatchString(urlHash) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	cacheMu.RLock()
	cachedVal, exists := cache[urlHash]
	cacheMu.RUnlock()

	if !exists {
		http.Error(w, "Video not found — try loading the Instagram URL again", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, cachedVal.Path)
}
