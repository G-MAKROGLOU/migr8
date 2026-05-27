package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const githubReleaseURL = "https://api.github.com/repos/G-MAKROGLOU/migr8/releases/latest"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade migr8 to the latest release",
	Long: `Check GitHub for the latest migr8 release.
If a newer version is available, the appropriate binary for the current
OS and architecture is downloaded and replaces the running executable.`,
	Run: selfUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func selfUpgrade(_ *cobra.Command, _ []string) {
	current := rootCmd.Version

	// Local builds have version = "dev" (no ldflags injection).
	// Self-upgrade has no meaningful source version to compare against, so
	// point the developer at `go install` instead.
	if current == "dev" {
		color.Yellow("[UPGRADE:] RUNNING A LOCAL / DEV BUILD — SELF-UPGRADE IS NOT AVAILABLE")
		color.Yellow("[UPGRADE:] Install a tagged release with:")
		color.Yellow("[UPGRADE:]   go install github.com/G-MAKROGLOU/migr8@latest")
		return
	}

	color.Cyan("[UPGRADE:] CURRENT VERSION => v%s", current)
	color.Cyan("[UPGRADE:] CHECKING FOR UPDATES...")

	release, err := fetchLatestRelease()
	if err != nil {
		color.Red("[ERR:] FAILED TO FETCH LATEST RELEASE => %s", err.Error())
		return
	}

	// Strip a leading 'v' from the tag (v1.2.0 → 1.2.0) so we can compare
	// directly against rootCmd.Version which has no prefix.
	latest := strings.TrimPrefix(release.TagName, "v")

	if latest == current {
		color.Green("[UPGRADE:] migr8 IS ALREADY UP TO DATE (v%s)", current)
		return
	}

	color.Cyan("[UPGRADE:] NEW VERSION AVAILABLE => v%s  (installed: v%s)", latest, current)

	assetName := assetNameForPlatform()
	downloadURL := ""
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		color.Red("[ERR:] NO BINARY FOUND FOR %s/%s (expected asset: %s)", runtime.GOOS, runtime.GOARCH, assetName)
		color.Yellow("[HINT:] Download manually from: https://github.com/G-MAKROGLOU/migr8/releases/latest")
		return
	}

	color.Cyan("[UPGRADE:] DOWNLOADING %s...", assetName)

	exePath, err := os.Executable()
	if err != nil {
		color.Red("[ERR:] CANNOT DETERMINE EXECUTABLE PATH => %s", err.Error())
		return
	}

	if err := downloadAndReplace(downloadURL, exePath); err != nil {
		color.Red("[ERR:] UPGRADE FAILED => %s", err.Error())
		return
	}

	color.Green("[UPGRADE:] migr8 UPGRADED SUCCESSFULLY TO v%s", latest)
	color.Green("[UPGRADE:] RESTART YOUR TERMINAL TO USE THE NEW VERSION")
}

// fetchLatestRelease calls the GitHub API and returns the latest release metadata.
func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, githubReleaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	// GitHub recommends setting Accept and User-Agent headers.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "migr8-cli/"+rootCmd.Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found for this repository")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release JSON: %w", err)
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("release JSON contained no tag_name")
	}
	return &release, nil
}

// assetNameForPlatform returns the expected release asset name for the current
// OS and architecture, matching the naming convention used by goreleaser:
//
//	migr8_linux_amd64
//	migr8_darwin_arm64
//	migr8_windows_amd64.exe
func assetNameForPlatform() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("migr8_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// downloadAndReplace downloads the binary at url and atomically replaces dest.
//
// The new file is first written to a sibling temp file (same filesystem as the
// destination) so that the final os.Rename is an atomic move rather than a
// copy-then-delete.  On Windows, where the OS locks running executables, the
// old binary is first renamed out of the way before the new one is moved into
// position; a best-effort attempt is made to delete the leftover .old file.
func downloadAndReplace(url, dest string) error {
	resp, err := http.Get(url) //nolint:noctx // upgrade path, timeout not critical
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	// Create the temp file in the same directory as the target so that
	// os.Rename stays on the same filesystem (required for atomicity).
	tmp, err := os.CreateTemp(filepath.Dir(dest), "migr8_upgrade_*")
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Always clean up the temp file if something goes wrong; this is a no-op
	// if the rename succeeded because the path no longer exists.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("error writing download to disk: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot close temp file: %w", err)
	}

	// Mirror the permissions of the current executable (typically 0755).
	if info, statErr := os.Stat(dest); statErr == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	} else {
		_ = os.Chmod(tmpPath, 0o755)
	}

	if runtime.GOOS == "windows" {
		return replaceOnWindows(tmpPath, dest)
	}

	// Unix: os.Rename is atomic — the old inode stays open until the process
	// using it exits, so replacing a running binary is safe.
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("cannot replace binary: %w", err)
	}
	return nil
}

// replaceOnWindows performs the two-step rename needed on Windows because the
// OS prevents in-place overwrite of a running executable.
func replaceOnWindows(tmpPath, dest string) error {
	oldPath := dest + ".old"

	// Remove any leftover from a previous failed upgrade.
	_ = os.Remove(oldPath)

	// Step 1: rename the running exe out of the way.
	if err := os.Rename(dest, oldPath); err != nil {
		return fmt.Errorf("cannot move current binary aside: %w", err)
	}

	// Step 2: move the newly downloaded binary into position.
	if err := os.Rename(tmpPath, dest); err != nil {
		// Best-effort rollback.
		_ = os.Rename(oldPath, dest)
		return fmt.Errorf("cannot install new binary: %w", err)
	}

	// Best-effort cleanup — Windows may keep the file locked until the next
	// invocation, so ignore errors here.
	_ = os.Remove(oldPath)
	return nil
}
