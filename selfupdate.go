package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/minio/selfupdate"
	"github.com/ulikunitz/xz"
	"golang.org/x/mod/semver"
)

// updateHTTPClient is used for update check and download requests.
var updateHTTPClient = &http.Client{Timeout: 30 * time.Second}

var updateDownloadClient = &http.Client{Timeout: 10 * time.Minute}

var githubAPIBaseURL = "https://api.github.com"

const latestReleaseResponseLimit = 1 << 20

func latestReleaseURL() string {
	return fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBaseURL, GithubRepo)
}

// fetchLatestVersion queries GitHub and returns the latest release tag.
func fetchLatestVersion() (string, error) {
	resp, err := updateHTTPClient.Get(latestReleaseURL())
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, latestReleaseResponseLimit)).Decode(&release); err != nil {
		return "", fmt.Errorf("parse failed: %w", err)
	}
	return release.Name, nil
}

// applyUpdate downloads and applies the given version in-place.
func applyUpdate(version string) error {
	ext := "xz"
	if runtime.GOOS == "windows" {
		ext = "exe.xz"
	}
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/claude-quota-%s-%s.%s",
		GithubRepo, version, runtime.GOOS, runtime.GOARCH, ext,
	)

	opts := selfupdate.Options{}
	if err := opts.CheckPermissions(); err != nil {
		return fmt.Errorf("permission denied (download manually: %s)", downloadURL)
	}

	log.Printf("Downloading %s...", downloadURL)
	// Use a generous timeout so stalled connections do not block the tray update forever.
	dlResp, err := updateDownloadClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", dlResp.StatusCode)
	}

	r, err := xz.NewReader(dlResp.Body)
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	if err := selfupdate.Apply(r, opts); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	log.Printf("Updated to %s successfully.", version)
	return nil
}

func selfUpdate() {
	fmt.Printf("Current version: %s-%s\n", Version, CommitHash)

	latestRelease, err := fetchLatestVersion()
	if err != nil {
		log.Fatalf("Failed to check for updates: %v", err)
	}
	fmt.Printf("Latest release: %s\n", latestRelease)

	switch semver.Compare(latestRelease, Version) {
	case -1:
		fmt.Println("You have a newer version than the latest release.")
		return
	case 0:
		fmt.Println("Already up to date.")
		return
	case 1:
		fmt.Println("New version available, upgrading...")
		if Version == "v0.0.0" {
			fmt.Print("Development build detected, press Enter to proceed: ")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		}
	}

	if err := applyUpdate(latestRelease); err != nil {
		log.Fatalf("Update failed: %v", err)
	}
}
