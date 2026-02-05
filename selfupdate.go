package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/minio/selfupdate"
	"github.com/ulikunitz/xz"
	"golang.org/x/mod/semver"
)

func selfUpdate() {
	fmt.Printf("Current version: %s-%s\n", Version, CommitHash)

	// Fetch latest release from GitHub API.
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GithubRepo)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Fatalf("Failed to parse release info: %v", err)
	}

	latestRelease := release.Name
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

	// Construct platform-specific download URL.
	ext := "xz"
	if runtime.GOOS == "windows" {
		ext = "exe.xz"
	}
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/claude-quota-%s-%s.%s",
		GithubRepo, latestRelease, runtime.GOOS, runtime.GOARCH, ext,
	)

	opts := selfupdate.Options{}
	if err := opts.CheckPermissions(); err != nil {
		fmt.Printf("Cannot update in place (permission denied).\nDownload manually: %s\n", downloadURL)
		return
	}

	fmt.Printf("Downloading %s...\n", downloadURL)
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		log.Fatalf("Download returned HTTP %d", dlResp.StatusCode)
	}

	r, err := xz.NewReader(dlResp.Body)
	if err != nil {
		log.Fatalf("XZ decompression failed: %v", err)
	}

	if err := selfupdate.Apply(r, opts); err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	fmt.Printf("Updated to %s successfully.\n", latestRelease)
}
