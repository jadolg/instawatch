package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedInstagramHosts = map[string]bool{
	"instagram.com":     true,
	"www.instagram.com": true,
}

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

func validateInstagramURL(raw string) (string, error) {
	// Double-slashes are collapsed by path routing.
	if strings.HasPrefix(raw, "https:/") && !strings.HasPrefix(raw, httpsPrefix) {
		raw = strings.Replace(raw, "https:/", httpsPrefix, 1)
	} else if strings.HasPrefix(raw, "http:/") && !strings.HasPrefix(raw, httpPrefix) {
		raw = strings.Replace(raw, "http:/", httpPrefix, 1)
	}

	// url.Parse works properly only when a scheme is present.
	if !strings.HasPrefix(raw, httpPrefix) && !strings.HasPrefix(raw, httpsPrefix) {
		if !strings.Contains(raw, "://") {
			raw = httpsPrefix + raw
		}
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL")
	}

	if u.Scheme != "https" {
		return "", fmt.Errorf("only HTTPS Instagram URLs are accepted")
	}

	// Prevents SSRF via subdomains or query tricks.
	host := strings.ToLower(u.Hostname())
	if !allowedInstagramHosts[host] {
		return "", fmt.Errorf("not a valid Instagram URL")
	}

	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

func downloadVideo(igURL, tmpDir, urlHash string) (string, string, error) {
	outPath := filepath.Join(tmpDir, urlHash+".mp4")
	titlePath := filepath.Join(tmpDir, urlHash+".title")

	sleepReq := fmt.Sprintf("%.1f", 1.5+rand.Float64()*1.5) // Random float between 1.5s and 3.0s

	args := []string{
		"--no-warnings",
		"--no-playlist",
		"--impersonate", "Safari",
		"--sleep-requests", sleepReq, // Pauses between API requests (JSON, HTML)
		"--sleep-interval", "1", // Pauses before the MP4 stream
		"--max-sleep-interval", "3",
		"-f", "bv*+ba/b",
		"-S", "vcodec:h264,res,acodec:m4a",
		"--merge-output-format", "mp4",
		"--remux-video", "mp4",
		"--postprocessor-args", "ffmpeg:-movflags faststart",
		"--print-to-file", "%(title)s", titlePath,
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
		return "", "", fmt.Errorf("yt-dlp failed: %w\nOutput: %s", err, string(output))
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, urlHash+".*"))
	if err != nil {
		return "", "", fmt.Errorf("failed to search for output file: %w", err)
	}
	var videoFile string
	for _, m := range matches {
		if filepath.Ext(m) != ".title" {
			videoFile = m
			break
		}
	}
	if videoFile == "" {
		return "", "", fmt.Errorf("yt-dlp produced no output file")
	}

	// iOS Safari requires the 'moov' atom at the beginning of the file.
	// yt-dlp might skip post-processing if it doesn't remux, so we enforce it here.
	faststartFile := filepath.Join(tmpDir, urlHash+"_fs.mp4")
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-i", videoFile, "-c", "copy", "-movflags", "faststart", faststartFile)
	if err := ffmpegCmd.Run(); err == nil {
		err := os.Remove(videoFile)
		if err != nil {
			log.Printf("Warning: could not remove temporary file: %v", err)
		}
		videoFile = faststartFile
	} else {
		log.Printf("Warning: ffmpeg faststart failed: %v", err)
	}

	titleBytes, err := os.ReadFile(titlePath)
	title := "Video"
	if err == nil {
		title = strings.TrimSpace(string(titleBytes))
		err := os.Remove(titlePath)
		if err != nil {
			log.Printf("Warning: could not remove temporary file: %v", err)
		}
	}

	log.Printf("Downloaded video: %s (Title: %s)", videoFile, title)
	return videoFile, title, nil
}
