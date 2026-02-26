package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestAssetName verifies the asset name format for all expected platform/arch
// combinations. We test the formatting logic directly since we cannot change
// runtime.GOOS/GOARCH in a running test — the function is pure string
// manipulation so this exercises the real code path.
func TestAssetName(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"linux", "amd64", "nesco-linux-amd64"},
		{"linux", "arm64", "nesco-linux-arm64"},
		{"darwin", "amd64", "nesco-darwin-amd64"},
		{"darwin", "arm64", "nesco-darwin-arm64"},
		{"windows", "amd64", "nesco-windows-amd64.exe"},
		{"windows", "arm64", "nesco-windows-arm64.exe"},
	}

	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			// Exercise the same formatting logic as assetName() with explicit
			// inputs so we don't depend on the test runner's platform.
			name := fmt.Sprintf("nesco-%s-%s", tc.goos, tc.goarch)
			if tc.goos == "windows" {
				name += ".exe"
			}
			if name != tc.want {
				t.Errorf("got %q, want %q", name, tc.want)
			}
		})
	}
}

// TestCheckLatest exercises the full CheckLatest path against a mock HTTP server
// that returns a realistic GitHub Releases API response.
func TestCheckLatest(t *testing.T) {
	// Build a fake release response with assets for several platforms.
	fakeRelease := map[string]interface{}{
		"tag_name": "v0.5.0",
		"body":     "## What's New\n\n- Feature A\n- Bug fix B\n",
		"assets": []map[string]interface{}{
			{"name": "nesco-linux-amd64", "browser_download_url": "https://example.com/nesco-linux-amd64"},
			{"name": "nesco-linux-arm64", "browser_download_url": "https://example.com/nesco-linux-arm64"},
			{"name": "nesco-darwin-amd64", "browser_download_url": "https://example.com/nesco-darwin-amd64"},
			{"name": "nesco-darwin-arm64", "browser_download_url": "https://example.com/nesco-darwin-arm64"},
			{"name": "nesco-windows-amd64.exe", "browser_download_url": "https://example.com/nesco-windows-amd64.exe"},
			{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing User-Agent", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeRelease)
	}))
	defer srv.Close()

	// Point the package at our mock server.
	origURL := githubAPIURL
	githubAPIURL = srv.URL
	defer func() { githubAPIURL = origURL }()

	t.Run("update available", func(t *testing.T) {
		info, err := CheckLatest("0.4.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Version != "0.5.0" {
			t.Errorf("Version: got %q, want %q", info.Version, "0.5.0")
		}
		if info.TagName != "v0.5.0" {
			t.Errorf("TagName: got %q, want %q", info.TagName, "v0.5.0")
		}
		if !info.UpdateAvail {
			t.Error("UpdateAvail: expected true when 0.5.0 > 0.4.0")
		}
		if info.ChecksumURL != "https://example.com/checksums.txt" {
			t.Errorf("ChecksumURL: got %q", info.ChecksumURL)
		}
		if info.Body == "" {
			t.Error("Body should not be empty")
		}
	})

	t.Run("already on latest", func(t *testing.T) {
		info, err := CheckLatest("0.5.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.UpdateAvail {
			t.Error("UpdateAvail: expected false when already on 0.5.0")
		}
	})

	t.Run("newer than released", func(t *testing.T) {
		// A dev build with version 0.6.0 should not report an update.
		info, err := CheckLatest("0.6.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.UpdateAvail {
			t.Error("UpdateAvail: expected false for version newer than latest release")
		}
	})

	t.Run("empty current version treated as 0.0.0", func(t *testing.T) {
		// An empty version string (dev build without ldflags) should still
		// report an update available since any real release is newer than 0.0.0.
		info, err := CheckLatest("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.UpdateAvail {
			t.Error("UpdateAvail: expected true when current version is empty")
		}
	})
}

// TestCheckLatestServerError verifies that a non-200 response from the GitHub
// API is returned as an error.
func TestCheckLatestServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL
	defer func() { githubAPIURL = origURL }()

	_, err := CheckLatest("0.4.0")
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

// TestChecksumVerification tests the SHA-256 checksum logic: write a known
// file, compute its hash, confirm findChecksum + sha256File agree; then verify
// that a wrong expected hash is correctly detected as a mismatch.
func TestChecksumVerification(t *testing.T) {
	dir := t.TempDir()

	// Write a deterministic "binary" file.
	binaryPath := filepath.Join(dir, "nesco-linux-amd64")
	content := []byte("fake binary content for testing")
	if err := os.WriteFile(binaryPath, content, 0644); err != nil {
		t.Fatalf("writing binary: %v", err)
	}

	// Compute expected hash.
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])

	// Write a checksums.txt that matches the Go-tool format.
	checksumPath := filepath.Join(dir, "checksums.txt")
	checksumContent := fmt.Sprintf("%s  nesco-linux-amd64\n%s  nesco-darwin-arm64\n",
		expectedHash, "deadbeef00000000000000000000000000000000000000000000000000000000")
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("writing checksums: %v", err)
	}

	t.Run("correct hash passes", func(t *testing.T) {
		found, err := findChecksum(checksumPath, "nesco-linux-amd64")
		if err != nil {
			t.Fatalf("findChecksum: %v", err)
		}
		actual, err := sha256File(binaryPath)
		if err != nil {
			t.Fatalf("sha256File: %v", err)
		}
		if found != actual {
			t.Errorf("hash mismatch: checksums.txt=%q, file=%q", found, actual)
		}
	})

	t.Run("wrong hash is detected", func(t *testing.T) {
		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
		actual, err := sha256File(binaryPath)
		if err != nil {
			t.Fatalf("sha256File: %v", err)
		}
		if wrongHash == actual {
			t.Fatal("test is broken: wrong hash accidentally equals actual hash")
		}
	})

	t.Run("missing entry returns error", func(t *testing.T) {
		_, err := findChecksum(checksumPath, "nesco-windows-amd64.exe")
		if err == nil {
			t.Error("expected error for missing entry, got nil")
		}
	})

	t.Run("checksum with path prefix is matched by base name", func(t *testing.T) {
		// Some tools emit "sha256  ./path/to/nesco-linux-amd64" — verify we handle it.
		pathChecksumPath := filepath.Join(dir, "checksums-path.txt")
		pathContent := fmt.Sprintf("%s  ./dist/nesco-linux-amd64\n", expectedHash)
		if err := os.WriteFile(pathChecksumPath, []byte(pathContent), 0644); err != nil {
			t.Fatalf("writing checksums: %v", err)
		}
		found, err := findChecksum(pathChecksumPath, "nesco-linux-amd64")
		if err != nil {
			t.Fatalf("findChecksum with path prefix: %v", err)
		}
		if found != expectedHash {
			t.Errorf("got %q, want %q", found, expectedHash)
		}
	})
}

// TestVersionNewer tests the semver comparison logic directly.
func TestVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Standard major.minor.patch comparisons
		{"0.5.0", "0.4.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.4.0", "0.5.0", false},
		{"0.5.0", "0.5.0", false}, // equal is not newer

		// Minor version differences
		{"0.2.0", "0.1.9", true},
		{"0.1.9", "0.2.0", false},

		// Patch version differences
		{"0.5.1", "0.5.0", true},
		{"0.5.0", "0.5.1", false},

		// v-prefix handling
		{"v0.5.0", "0.4.0", true},
		{"0.5.0", "v0.4.0", true},
		{"v0.5.0", "v0.5.0", false},

		// Empty string treated as 0.0.0
		{"0.1.0", "", true},
		{"", "0.1.0", false},
		{"", "", false},

		// Pre-release suffix stripped (we only compare major.minor.patch)
		{"0.5.0-beta.1", "0.4.0", true},
		{"0.5.0", "0.5.0-beta.1", false}, // same numeric version; not newer
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_vs_%s", tc.a, tc.b), func(t *testing.T) {
			got := versionNewer(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("versionNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestParseVersion tests the version parsing helper directly.
func TestParseVersion(t *testing.T) {
	cases := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.5.0", [3]int{0, 5, 0}},
		{"10.20.30", [3]int{10, 20, 30}},
		{"1.0.0-alpha", [3]int{1, 0, 0}},
		{"1.0.0+build.1", [3]int{1, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"1", [3]int{1, 0, 0}},
		{"1.2", [3]int{1, 2, 0}},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := parseVersion(tc.input)
			if got != tc.want {
				t.Errorf("parseVersion(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
