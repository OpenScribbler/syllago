// Package updater checks for and installs updates from GitHub Releases.
// It is intentionally dependency-free (standard library only) so it can be
// imported by both the TUI and the CLI without pulling in TUI dependencies.
package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// githubAPIURL is a var so tests can override it with an httptest server URL.
var githubAPIURL = "https://api.github.com/repos/OpenScribbler/syllago/releases/latest"

// httpClient is a shared client with a 15-second timeout. GitHub is generally
// fast; we don't want to hang the TUI or CLI waiting on a slow network.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// ReleaseInfo holds information about the latest GitHub release.
type ReleaseInfo struct {
	Version     string // e.g. "0.5.0" (no "v" prefix)
	TagName     string // e.g. "v0.5.0"
	Body        string // release notes markdown
	UpdateAvail bool   // true if Version is newer than the version passed to CheckLatest
	AssetURL    string // direct download URL for the correct platform binary
	ChecksumURL string // direct download URL for checksums.txt
}

// githubRelease is the subset of the GitHub API response we care about.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckLatest fetches the latest release from GitHub and compares it to
// currentVersion (which should be without the "v" prefix, e.g. "0.4.0").
func CheckLatest(currentVersion string) (ReleaseInfo, error) {
	req, err := http.NewRequest(http.MethodGet, githubAPIURL, nil)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("building request: %w", err)
	}
	// GitHub requires a User-Agent header; requests without one get a 403.
	req.Header.Set("User-Agent", "syllago-updater")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("fetching release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ReleaseInfo{}, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ReleaseInfo{}, fmt.Errorf("parsing release JSON: %w", err)
	}

	latestVersion := strings.TrimPrefix(rel.TagName, "v")

	info := ReleaseInfo{
		Version:     latestVersion,
		TagName:     rel.TagName,
		Body:        rel.Body,
		UpdateAvail: versionNewer(latestVersion, currentVersion),
	}

	// Find the asset URL for the current platform and for checksums.txt.
	want := assetName()
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			info.AssetURL = a.BrowserDownloadURL
		case "checksums.txt":
			info.ChecksumURL = a.BrowserDownloadURL
		}
	}

	if info.AssetURL == "" && info.UpdateAvail {
		// The release exists but has no binary for this platform. Return the
		// info anyway so callers can at least show the version; Update() will
		// return a clear error when it tries to actually install.
		return info, nil
	}

	return info, nil
}

// Update downloads and installs the latest release binary for the current
// platform. progress is called with human-readable status messages as the
// update proceeds. Returns nil on success; the caller should prompt the user
// to restart syllago.
func Update(currentVersion string, progress func(string)) error {
	info, err := CheckLatest(currentVersion)
	if err != nil {
		return fmt.Errorf("checking for update: %w", err)
	}

	if !info.UpdateAvail {
		progress("Already on latest version")
		return nil
	}

	if info.AssetURL == "" {
		return fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	progress(fmt.Sprintf("Downloading syllago v%s...", info.Version))

	// Use a temp directory so both the binary and checksums file land together.
	tmpDir, err := os.MkdirTemp("", "syllago-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // clean up on any error path (no-op after Rename)

	// Download checksums.txt first (small file).
	var expectedHash string
	if info.ChecksumURL != "" {
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadFile(info.ChecksumURL, checksumPath); err != nil {
			return fmt.Errorf("downloading checksums: %w", err)
		}
		expectedHash, err = findChecksum(checksumPath, assetName())
		if err != nil {
			return fmt.Errorf("reading checksum: %w", err)
		}
	}

	// Download the binary.
	binaryName := assetName()
	tmpBinary := filepath.Join(tmpDir, binaryName)
	if err := downloadFile(info.AssetURL, tmpBinary); err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}

	// Verify the checksum if we have one.
	progress("Verifying checksum...")
	if expectedHash != "" {
		actualHash, err := sha256File(tmpBinary)
		if err != nil {
			return fmt.Errorf("computing checksum: %w", err)
		}
		if !strings.EqualFold(actualHash, expectedHash) {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}
	}

	// Resolve the path to the currently running binary (follow symlinks so we
	// replace the real file, not the symlink).
	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current executable: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Make the downloaded binary executable before moving it into place.
	if err := os.Chmod(tmpBinary, 0755); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	// Atomic rename: on Unix this is guaranteed to be atomic within the same
	// filesystem. If tmp and the install location are on different filesystems
	// (rare), os.Rename falls back to a copy+delete and is still safe.
	if err := os.Rename(tmpBinary, currentBin); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	// tmpDir's deferred RemoveAll is now harmless — the binary has been moved out.

	progress(fmt.Sprintf("Updated to v%s. Restart syllago to use the new version.", info.Version))
	return nil
}

// assetName returns the expected GitHub release asset filename for the current
// platform. Matches the naming convention used in syllago's release workflow.
func assetName() string {
	name := fmt.Sprintf("syllago-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// downloadFile fetches url and writes it to destPath.
func downloadFile(url, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "syllago-updater")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
}

// sha256File computes the hex-encoded SHA-256 digest of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findChecksum parses a checksums.txt file (Go-tool format: "<hash>  <filename>")
// and returns the expected SHA-256 for filename. Returns an error if not found.
func findChecksum(checksumPath, filename string) (string, error) {
	f, err := os.Open(checksumPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "<sha256hash>  <filename>" (two spaces) or "<sha256hash>  <path/filename>"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// The filename field may include a path prefix; match only the base name.
		if filepath.Base(fields[1]) == filename {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum entry found for %s", filename)
}

// parseVersion parses a version string ("v0.5.0" or "0.5.0") into a
// [major, minor, patch] array. Pre-release suffixes are ignored.
func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release/build suffix at the first '-' or '+'
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}

// versionNewer returns true if a is strictly newer than b.
func versionNewer(a, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}
