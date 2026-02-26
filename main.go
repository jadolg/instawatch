package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

var templates *template.Template

var (
	cache      = make(map[string]string)
	cacheMu    sync.RWMutex
	cookieFile string
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

	// Set up persistent cookie storage
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	instawatchDir := filepath.Join(configDir, "instawatch")
	if err := os.MkdirAll(instawatchDir, 0700); err != nil {
		log.Printf("Warning: could not create config directory: %v", err)
		instawatchDir = "."
	}
	cookieFile = filepath.Join(instawatchDir, "cookies.txt")
	log.Printf("Cookie file: %s", cookieFile)

	mux := http.NewServeMux()

	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	mux.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
		handleVideo(w, r, tmpDir)
	})

	mux.HandleFunc("/api/cookie", handleCookiePost)
	mux.HandleFunc("/api/cookie/status", handleCookieStatus)

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

	scheduleVideoDeletion(urlHash, videoPath, 1*time.Hour)

	servePlayer(w, urlHash)
}

func downloadVideo(igURL, tmpDir, urlHash string) (string, error) {
	outPath := filepath.Join(tmpDir, urlHash+".mp4")

	args := []string{
		"--no-warnings",
		"--no-playlist",
		"-f", "bv*+ba/b",
		"--merge-output-format", "mp4",
		"-o", outPath,
	}

	// Add cookies if available
	if cookieFile != "" {
		if _, err := os.Stat(cookieFile); err == nil {
			args = append(args, "--cookies", cookieFile)
		}
	}

	args = append(args, igURL)

	cmd := exec.Command("yt-dlp", args...)

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

func handleCookiePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"sessionid"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid JSON"})
		return
	}

	if req.SessionID == "" {
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Missing sessionid"})
		return
	}

	// Create Netscape cookie file format
	// Format: domain	flag	path	secure	expiration	name	value
	cookieContent := fmt.Sprintf(`# Netscape HTTP Cookie File
.instagram.com	TRUE	/	TRUE	0	sessionid	%s
`, req.SessionID)

	if err := os.WriteFile(cookieFile, []byte(cookieContent), 0600); err != nil {
		log.Printf("Failed to write cookie file: %v", err)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Failed to save cookie"})
		return
	}

	log.Printf("Instagram cookie saved to %s", cookieFile)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func handleCookieStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hasCookie := false
	if cookieFile != "" {
		if _, err := os.Stat(cookieFile); err == nil {
			hasCookie = true
		}
	}

	json.NewEncoder(w).Encode(map[string]any{"hasCookie": hasCookie})
}
