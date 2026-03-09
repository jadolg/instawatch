package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

var templates *template.Template

var igCookieFile string
var fbCookieFile string

const (
	httpsPrefix = "https://"
	httpPrefix  = "http://"
	videoRoute  = "/video/"
)

var hashPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func init() {
	err := mime.AddExtensionType(".mp4", "video/mp4")
	if err != nil {
		log.Fatalf("Failed to add extension type: %v", err)
	}
	templates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))
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
	igCookieFile = filepath.Join(dataDir, "ig_cookies.txt")
	fbCookieFile = filepath.Join(dataDir, "fb_cookies.txt")
	log.Printf("Instagram cookie file: %s", igCookieFile)
	log.Printf("Facebook cookie file: %s", fbCookieFile)

	if sessionID := os.Getenv("INSTAGRAM_SESSION_ID"); sessionID != "" {
		content := fmt.Sprintf("# Netscape HTTP Cookie File\n.instagram.com\tTRUE\t/\tTRUE\t0\tsessionid\t%s\n", sessionID)
		if err := os.WriteFile(igCookieFile, []byte(content), 0600); err != nil {
			log.Printf("Warning: could not write Instagram cookie file: %v", err)
		} else {
			masked := maskSessionID(sessionID)
			log.Printf("Instagram session cookie written from INSTAGRAM_SESSION_ID (sessionid=%s)", masked)
		}
	}

	if sessionID := os.Getenv("FACEBOOK_SESSION_ID"); sessionID != "" {
		content := fmt.Sprintf("# Netscape HTTP Cookie File\n.facebook.com\tTRUE\t/\tTRUE\t0\txs\t%s\n", sessionID)
		if err := os.WriteFile(fbCookieFile, []byte(content), 0600); err != nil {
			log.Printf("Warning: could not write Facebook cookie file: %v", err)
		} else {
			masked := maskSessionID(sessionID)
			log.Printf("Facebook session cookie written from FACEBOOK_SESSION_ID (xs=%s)", masked)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("GET "+videoRoute+"{hash}", handleVideo)
	mux.HandleFunc("GET /description/{hash}", handleDescription)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		handleRoot(w, r, tmpDir)
	})

	addr := ":8080"
	log.Printf("InstaWatch listening on %s", addr)
	if err := http.ListenAndServe(addr, securityHeaders(mux)); err != nil {
		log.Fatal(err)
	}
}

func maskSessionID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:4] + strings.Repeat("*", len(sessionID)-8) + sessionID[len(sessionID)-4:]
}
