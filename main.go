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

// hasCookies reports whether cookieFile exists and contains at least one
// real cookie entry (i.e. a non-empty line that is not a comment).
func hasCookies() bool {
	if cookieFile == "" {
		return false
	}
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}
	return false
}

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
	// Use DATA_DIR env var if set (for Docker), otherwise use user config dir
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			configDir = "."
		}
		dataDir = filepath.Join(configDir, "instawatch")
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Printf("Warning: could not create data directory: %v", err)
		dataDir = "."
	}
	cookieFile = filepath.Join(dataDir, "cookies.txt")
	log.Printf("Cookie file: %s", cookieFile)

	// If INSTAGRAM_SESSION_ID is set, write (or overwrite) the cookie file at startup.
	if sessionID := os.Getenv("INSTAGRAM_SESSION_ID"); sessionID != "" {
		expiry := time.Now().Add(365 * 24 * time.Hour).Unix()
		content := fmt.Sprintf("# Netscape HTTP Cookie File\n.instagram.com\tTRUE\t/\tTRUE\t%d\tsessionid\t%s\n", expiry, sessionID)
		if err := os.WriteFile(cookieFile, []byte(content), 0600); err != nil {
			log.Printf("Warning: could not write cookie file: %v", err)
		} else {
			masked := sessionID
			if len(sessionID) > 8 {
				masked = sessionID[:4] + strings.Repeat("*", len(sessionID)-8) + sessionID[len(sessionID)-4:]
			}
			log.Printf("Instagram session cookie written from INSTAGRAM_SESSION_ID (sessionid=%s)", masked)
		}
	}

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
	if hasCookies() {
		args = append(args, "--cookies", cookieFile)
		log.Printf("Downloading with cookies: %s", igURL)
	} else {
		log.Printf("Downloading without cookies (no valid cookie file): %s", igURL)
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
