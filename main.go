package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

const (
	httpsPrefix = "https://"
	httpPrefix  = "http://"
	videoRoute  = "/video/"
)

var allowedInstagramHosts = map[string]bool{
	"instagram.com":     true,
	"www.instagram.com": true,
}

var hashPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func hasCookies() bool {
	if cookieFile == "" {
		return false
	}
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; media-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func validateInstagramURL(raw string) (string, error) {
	// Restore double-slashes collapsed by path routing.
	raw = strings.Replace(raw, "https:/", httpsPrefix, 1)
	raw = strings.Replace(raw, "http:/", httpPrefix, 1)

	// Prepend scheme if missing so url.Parse works correctly.
	if !strings.HasPrefix(raw, httpPrefix) && !strings.HasPrefix(raw, httpsPrefix) {
		raw = httpsPrefix + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL")
	}

	// Enforce HTTPS only.
	if u.Scheme != "https" {
		return "", fmt.Errorf("only HTTPS Instagram URLs are accepted")
	}

	// Strict host allowlist — prevents SSRF via subdomains or query tricks.
	host := strings.ToLower(u.Hostname())
	if !allowedInstagramHosts[host] {
		return "", fmt.Errorf("not a valid Instagram URL")
	}

	return u.String(), nil
}

func main() {
	tmpDir, err := os.MkdirTemp("", "instawatch-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Printf("Warning: could not remove temporary directory: %v", err)
		}
	}(tmpDir)
	log.Printf("Video cache directory: %s", tmpDir)

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

	if sessionID := os.Getenv("INSTAGRAM_SESSION_ID"); sessionID != "" {
		content := fmt.Sprintf("# Netscape HTTP Cookie File\n.instagram.com\tTRUE\t/\tTRUE\t0\tsessionid\t%s\n", sessionID)
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

	mux.HandleFunc("GET "+videoRoute+"{hash}", handleVideo)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		handleRoot(w, r, tmpDir)
	})

	addr := ":8080"
	log.Printf("InstaWatch listening on %s", addr)
	if err := http.ListenAndServe(addr, securityHeaders(mux)); err != nil {
		log.Fatal(err)
	}
}

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
		rawURL += "?" + r.URL.RawQuery
	}

	igURL, err := validateInstagramURL(rawURL)
	if err != nil {
		renderIndex(w, http.StatusBadRequest, err.Error())
		return
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
		renderIndex(w, http.StatusInternalServerError, "Could not download video. Please check the URL and try again.")
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

	matches, err := filepath.Glob(filepath.Join(tmpDir, urlHash+".*"))
	if err != nil {
		return "", fmt.Errorf("failed to search for output file: %w", err)
	}
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
		VideoURL: videoRoute + urlHash,
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

	// Validate hash format to prevent path traversal and cache poisoning.
	if !hashPattern.MatchString(urlHash) {
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
	// Use 16 bytes (32 hex chars) to reduce collision probability.
	return hex.EncodeToString(h[:16])
}
