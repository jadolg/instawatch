package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func fetchVersions() {
	if out, err := exec.Command("yt-dlp", "--version").Output(); err == nil {
		ytdlpVersion = strings.TrimSpace(string(out))
	} else {
		ytdlpVersion = "unknown"
	}
	if out, err := exec.Command("python3", "-c", "import curl_cffi; print(curl_cffi.__version__)").Output(); err == nil {
		curlCffiVersion = strings.TrimSpace(string(out))
	} else {
		curlCffiVersion = "unknown"
	}
	log.Printf("yt-dlp %s, curl_cffi %s", ytdlpVersion, curlCffiVersion)
}

func initDataDir() {
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
}

func initCookies() {
	writeCookieFromEnv("INSTAGRAM_SESSION_ID", igCookieFile, ".instagram.com", "sessionid")
	writeCookieFromEnv("FACEBOOK_SESSION_ID", fbCookieFile, ".facebook.com", "xs")
}

func writeCookieFromEnv(envVar, cookieFile, domain, cookieName string) {
	sessionID := os.Getenv(envVar)
	if sessionID == "" {
		return
	}
	content := fmt.Sprintf("# Netscape HTTP Cookie File\n%s\tTRUE\t/\tTRUE\t0\t%s\t%s\n", domain, cookieName, sessionID)
	if err := os.WriteFile(cookieFile, []byte(content), 0600); err != nil {
		log.Printf("Warning: could not write cookie file %s: %v", cookieFile, err)
		return
	}
	log.Printf("%s cookie written from %s (%s=%s)", domain, envVar, cookieName, maskSessionID(sessionID))
}

func maskSessionID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:4] + strings.Repeat("*", len(sessionID)-8) + sessionID[len(sessionID)-4:]
}
