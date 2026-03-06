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

var cookieFile string

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
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
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
