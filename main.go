package main

import (
	"embed"
	"html/template"
	"log"
	"mime"
	"net/http"
	"os"
	"regexp"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

var templates *template.Template

var igCookieFile string
var fbCookieFile string

var ytdlpVersion string
var curlCffiVersion string

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
	fetchVersions()

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

	initDataDir()
	initCookies()
	startCacheJanitor(janitorInterval, videoRetention)

	mux := setupRoutes(tmpDir)

	addr := ":8080"
	log.Printf("InstaWatch listening on %s", addr)
	if err := http.ListenAndServe(addr, securityHeaders(mux)); err != nil {
		log.Fatal(err)
	}
}
