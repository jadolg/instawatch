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

var allowedFacebookHosts = map[string]bool{
	"facebook.com":     true,
	"www.facebook.com": true,
	"fb.watch":         true,
	"m.facebook.com":   true,
}

func hasCookies(file string) bool {
	if file == "" {
		return false
	}
	data, err := os.ReadFile(file)
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

func validateURL(raw string) (string, error) {
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

	if strings.ContainsAny(raw, " \n\r\t\"'`$;|<>(){}") {
		return "", fmt.Errorf("URL contains invalid characters")
	}

	if u.Scheme != "https" {
		return "", fmt.Errorf("only HTTPS URLs are accepted")
	}

	// Prevents SSRF via subdomains or query tricks.
	host := strings.ToLower(u.Hostname())
	isInstagram := allowedInstagramHosts[host]
	isFacebook := allowedFacebookHosts[host]

	if !isInstagram && !isFacebook {
		return "", fmt.Errorf("not a supported URL")
	}

	if isFacebook {
		// Facebook watch?v=... needs the query parameter.
		// Other FB/Instagram URLs might work without it, but let's keep it for FB.
	} else {
		u.RawQuery = ""
	}
	u.Fragment = ""

	return u.String(), nil
}

func downloadVideo(videoURL, tmpDir, urlHash string) (string, string, string, error) {
	outPath := filepath.Join(tmpDir, urlHash+".mp4")
	titlePath := filepath.Join(tmpDir, urlHash+".title")
	descPath := filepath.Join(tmpDir, urlHash+".description")

	sleepReq := fmt.Sprintf("%.1f", 1.5+rand.Float64()*1.5) // Random float between 1.5s and 3.0s

	args := []string{
		"--no-warnings",
		"--no-playlist",
		"--impersonate", "Chrome",
		"--sleep-requests", sleepReq, // Pauses between API requests (JSON, HTML)
		"--sleep-interval", "1", // Pauses before the MP4 stream
		"--max-sleep-interval", "3",
		"-f", "bv*+ba/b",
		"-S", "vcodec:h264,res,acodec:m4a",
		"--merge-output-format", "mp4",
		"--remux-video", "mp4",
		"--postprocessor-args", "ffmpeg:-movflags faststart",
		"--print-to-file", "%(description)s", descPath,
		"--print-to-file", "%(title)s", titlePath,
		"-o", outPath,
	}

	u, _ := url.Parse(videoURL)
	host := strings.ToLower(u.Hostname())
	isFacebook := allowedFacebookHosts[host]

	if isFacebook {
		if hasCookies(fbCookieFile) {
			args = append(args, "--cookies", fbCookieFile)
			log.Printf("Downloading Facebook video with cookies: %s", videoURL)
		} else {
			log.Printf("Downloading Facebook video without cookies: %s", videoURL)
		}
	} else {
		if hasCookies(igCookieFile) {
			args = append(args, "--cookies", igCookieFile)
			log.Printf("Downloading Instagram video with cookies: %s", videoURL)
		} else {
			log.Printf("Downloading Instagram video without cookies: %s", videoURL)
		}
	}

	args = append(args, "--", videoURL)

	const maxAttempts = 3
	var (
		output    []byte
		ytdlpErr  error
		videoFile string
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := exec.Command("yt-dlp", args...)
		output, ytdlpErr = cmd.CombinedOutput()

		matches, err := filepath.Glob(filepath.Join(tmpDir, urlHash+".*"))
		if err != nil {
			return "", "", "", fmt.Errorf("failed to search for output file: %w", err)
		}
		videoFile = ""
		for _, m := range matches {
			ext := filepath.Ext(m)
			if ext != ".title" && ext != ".description" && ext != ".part" {
				videoFile = m
				break
			}
		}

		if ytdlpErr == nil || videoFile != "" {
			// Success or rename-race: file is present.
			if ytdlpErr != nil {
				log.Printf("Warning: yt-dlp exited with error but output file is present, continuing: %v\nOutput: %s", ytdlpErr, string(output))
			}
			break
		}

		log.Printf("yt-dlp attempt %d/%d failed: %v\nOutput: %s", attempt, maxAttempts, ytdlpErr, string(output))
		if attempt < maxAttempts {
			// Clean up any leftover .part file before retrying.
			_ = os.Remove(filepath.Join(tmpDir, urlHash+".mp4.part"))
			if partFiles, _ := filepath.Glob(filepath.Join(tmpDir, urlHash+".*.part")); len(partFiles) > 0 {
				for _, pf := range partFiles {
					_ = os.Remove(pf)
				}
			}
		}
	}

	if videoFile == "" {
		return "", "", "", fmt.Errorf("yt-dlp failed after %d attempts: %w\nOutput: %s", maxAttempts, ytdlpErr, string(output))
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

	descBytes, err := os.ReadFile(descPath)
	description := ""
	if err == nil {
		description = strings.TrimSpace(string(descBytes))
		if description == "NA" {
			description = ""
		}
		err := os.Remove(descPath)
		if err != nil {
			log.Printf("Warning: could not remove temporary file: %v", err)
		}
	}

	log.Printf("Downloaded video: %s (Title: %s)", videoFile, title)
	return videoFile, title, description, nil
}
