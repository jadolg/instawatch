package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

var templates *template.Template

var (
	cache   = make(map[string]string)
	cacheMu sync.RWMutex
)

func init() {
	templates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))
}

func main() {
	tmpDir, err := os.MkdirTemp("", "instawatch-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	log.Printf("Video cache directory: %s", tmpDir)

	mux := http.NewServeMux()

	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	mux.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
		handleVideo(w, r, tmpDir)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRoot(w, r, tmpDir)
	})

	addr := ":8080"
	log.Printf("InstaWatch listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request, tmpDir string) {
	path := r.URL.Path

	if path == "/" {
		templates.ExecuteTemplate(w, "index.html", nil)
		return
	}

	igURL := strings.TrimPrefix(path, "/")

	if r.URL.RawQuery != "" {
		igURL += "?" + r.URL.RawQuery
	}

	igURL = strings.Replace(igURL, "https:/", "https://", 1)
	igURL = strings.Replace(igURL, "http:/", "http://", 1)

	if !strings.Contains(igURL, "instagram.com") {
		http.Error(w, "Not a valid Instagram URL", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(igURL, "http") {
		igURL = "https://" + igURL
	}

	urlHash := hashURL(igURL)
	cacheMu.RLock()
	cachedPath, exists := cache[urlHash]
	cacheMu.RUnlock()

	if exists {
		if _, err := os.Stat(cachedPath); err == nil {
			servePlayer(w, urlHash)
			return
		}
		cacheMu.Lock()
		delete(cache, urlHash)
		cacheMu.Unlock()
	}

	videoPath, err := downloadVideo(igURL, tmpDir, urlHash)
	if err != nil {
		log.Printf("Error downloading video from %s: %v", igURL, err)
		http.Error(w, fmt.Sprintf("Could not download video: %v", err), http.StatusInternalServerError)
		return
	}

	cacheMu.Lock()
	cache[urlHash] = videoPath
	cacheMu.Unlock()

	servePlayer(w, urlHash)
}

func downloadVideo(igURL, tmpDir, urlHash string) (string, error) {
	outPath := filepath.Join(tmpDir, urlHash+".mp4")

	cmd := exec.Command("yt-dlp",
		"--no-warnings",
		"--no-playlist",
		"-f", "bv*+ba/b",
		"--merge-output-format", "mp4",
		"-o", outPath,
		igURL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w\nOutput: %s", err, string(output))
	}

	matches, _ := filepath.Glob(filepath.Join(tmpDir, urlHash+".*"))
	if len(matches) == 0 {
		return "", fmt.Errorf("yt-dlp produced no output file")
	}

	log.Printf("Downloaded video: %s", matches[0])
	return matches[0], nil
}

func servePlayer(w http.ResponseWriter, urlHash string) {
	data := struct {
		VideoURL string
	}{
		VideoURL: "/video/" + urlHash,
	}
	err := templates.ExecuteTemplate(w, "player.html", data)
	if err != nil {
		log.Println(err)
	}
}

func handleVideo(w http.ResponseWriter, r *http.Request, tmpDir string) {
	urlHash := strings.TrimPrefix(r.URL.Path, "/video/")
	if urlHash == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	cacheMu.RLock()
	videoPath, exists := cache[urlHash]
	cacheMu.RUnlock()

	if !exists {
		http.Error(w, "Video not found — try loading the Instagram URL again", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, videoPath)
}

func hashURL(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:8])
}
