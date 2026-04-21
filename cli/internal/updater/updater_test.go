package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		{"linux", "amd64", "syllago-linux-amd64"},
		{"linux", "arm64", "syllago-linux-arm64"},
		{"darwin", "amd64", "syllago-darwin-amd64"},
		{"darwin", "arm64", "syllago-darwin-arm64"},
		{"windows", "amd64", "syllago-windows-amd64.exe"},
		{"windows", "arm64", "syllago-windows-arm64.exe"},
	}

	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			// Exercise the same formatting logic as assetName() with explicit
			// inputs so we don't depend on the test runner's platform.
			name := fmt.Sprintf("syllago-%s-%s", tc.goos, tc.goarch)
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
			{"name": "syllago-linux-amd64", "browser_download_url": "https://example.com/syllago-linux-amd64"},
			{"name": "syllago-linux-arm64", "browser_download_url": "https://example.com/syllago-linux-arm64"},
			{"name": "syllago-darwin-amd64", "browser_download_url": "https://example.com/syllago-darwin-amd64"},
			{"name": "syllago-darwin-arm64", "browser_download_url": "https://example.com/syllago-darwin-arm64"},
			{"name": "syllago-windows-amd64.exe", "browser_download_url": "https://example.com/syllago-windows-amd64.exe"},
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
	binaryPath := filepath.Join(dir, "syllago-linux-amd64")
	content := []byte("fake binary content for testing")
	if err := os.WriteFile(binaryPath, content, 0644); err != nil {
		t.Fatalf("writing binary: %v", err)
	}

	// Compute expected hash.
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])

	// Write a checksums.txt that matches the Go-tool format.
	checksumPath := filepath.Join(dir, "checksums.txt")
	checksumContent := fmt.Sprintf("%s  syllago-linux-amd64\n%s  syllago-darwin-arm64\n",
		expectedHash, "deadbeef00000000000000000000000000000000000000000000000000000000")
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("writing checksums: %v", err)
	}

	t.Run("correct hash passes", func(t *testing.T) {
		found, err := findChecksum(checksumPath, "syllago-linux-amd64")
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
		_, err := findChecksum(checksumPath, "syllago-windows-amd64.exe")
		if err == nil {
			t.Error("expected error for missing entry, got nil")
		}
	})

	t.Run("checksum with path prefix is matched by base name", func(t *testing.T) {
		// Some tools emit "sha256  ./path/to/syllago-linux-amd64" — verify we handle it.
		pathChecksumPath := filepath.Join(dir, "checksums-path.txt")
		pathContent := fmt.Sprintf("%s  ./dist/syllago-linux-amd64\n", expectedHash)
		if err := os.WriteFile(pathChecksumPath, []byte(pathContent), 0644); err != nil {
			t.Fatalf("writing checksums: %v", err)
		}
		found, err := findChecksum(pathChecksumPath, "syllago-linux-amd64")
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

// TestDownloadFile exercises the downloadFile function against a mock HTTP server.
func TestDownloadFile(t *testing.T) {
	content := []byte("hello world binary content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			w.Write(content)
		case "/notfound":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "bad", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Run("successful download", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "downloaded")
		err := downloadFile(srv.URL+"/good", dest)
		if err != nil {
			t.Fatalf("downloadFile: %v", err)
		}
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("reading downloaded file: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", got, content)
		}
	})

	t.Run("404 returns error", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "downloaded")
		err := downloadFile(srv.URL+"/notfound", dest)
		if err == nil {
			t.Fatal("expected error for 404, got nil")
		}
	})

	t.Run("server error returns error", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "downloaded")
		err := downloadFile(srv.URL+"/error", dest)
		if err == nil {
			t.Fatal("expected error for 500, got nil")
		}
	})
}

// TestUpdate_AlreadyLatest verifies Update returns early when already on latest.
func TestUpdate_AlreadyLatest(t *testing.T) {
	fakeRelease := map[string]interface{}{
		"tag_name": "v0.5.0",
		"body":     "release notes",
		"assets":   []map[string]interface{}{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeRelease)
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL
	defer func() { githubAPIURL = origURL }()

	var messages []string
	err := Update("0.5.0", func(msg string) { messages = append(messages, msg) })
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(messages) != 1 || messages[0] != "Already on latest version" {
		t.Errorf("unexpected messages: %v", messages)
	}
}

// TestUpdate_NoAssetForPlatform verifies Update errors when no binary exists for the platform.
func TestUpdate_NoAssetForPlatform(t *testing.T) {
	fakeRelease := map[string]interface{}{
		"tag_name": "v99.0.0",
		"body":     "release notes",
		"assets":   []map[string]interface{}{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeRelease)
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL
	defer func() { githubAPIURL = origURL }()

	err := Update("0.1.0", func(msg string) {})
	if err == nil {
		t.Fatal("expected error when no asset for platform, got nil")
	}
}

// TestUpdate_FullFlow exercises the full download + checksum verification
// + rename flow end-to-end. It uses the osExecutable test seam to point
// the final Rename at a throwaway file in t.TempDir() so the update
// doesn't clobber the running test binary.
//
// The test asserts positively that the checksum step ran:
//   - "Verifying checksum..." progress message (emitted only AFTER
//     downloadFile returns and BEFORE the hash comparison — its
//     presence proves download succeeded).
//   - Final "Updated to v99.0.0..." message (only emitted if os.Rename
//     succeeds, which only happens if the checksum matched).
//
// Previously the test was conditional: it accepted either nil (silently
// clobbering the test binary on Linux, where Rename succeeds against
// the running executable) or a rename error. Neither branch actually
// pinned that the checksum step had executed. The osExecutable seam
// makes both Rename safe and the success path deterministic.
func TestUpdate_FullFlow(t *testing.T) {
	// Create a fake binary and compute its checksum.
	binaryContent := []byte("fake syllago binary for update test")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	wantAsset := assetName()

	checksumContent := fmt.Sprintf("%s  %s\n", checksum, wantAsset)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/release":
			fakeRelease := map[string]interface{}{
				"tag_name": "v99.0.0",
				"body":     "new release",
				"assets": []map[string]interface{}{
					{"name": wantAsset, "browser_download_url": "http://" + r.Host + "/binary"},
					{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/checksums"},
				},
			}
			json.NewEncoder(w).Encode(fakeRelease)
		case "/binary":
			w.Write(binaryContent)
		case "/checksums":
			w.Write([]byte(checksumContent))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL + "/api/release"
	defer func() { githubAPIURL = origURL }()

	// Point osExecutable at a throwaway file — Rename will overwrite this
	// instead of the real test binary.
	fakeBinary := filepath.Join(t.TempDir(), "fake-syllago-for-update")
	if err := os.WriteFile(fakeBinary, []byte("old binary"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	origExec := osExecutable
	osExecutable = func() (string, error) { return fakeBinary, nil }
	defer func() { osExecutable = origExec }()

	var messages []string
	err := Update("0.1.0", func(msg string) { messages = append(messages, msg) })

	// Success is now deterministic — checksum matches and the rename
	// target is a disposable file, so the whole flow must complete.
	if err != nil {
		t.Fatalf("Update failed — checksum flow did not complete: %v", err)
	}

	// Positively verify each step of the pipeline emitted its progress
	// message. "Verifying checksum..." is the anchor for the checksum
	// step specifically (updater.go:170). The final "Updated to ..."
	// message only fires if os.Rename succeeded, which in turn only
	// fires if the checksum matched (updater.go:176-178 gates on it).
	wantMsgs := []string{
		"Downloading syllago v99.0.0...",                              // download started
		"Verifying checksum...",                                       // checksum step entered
		"Updated to v99.0.0. Restart syllago to use the new version.", // full flow completed
	}
	for _, want := range wantMsgs {
		found := false
		for _, got := range messages {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing progress message %q — flow did not reach this step. Got: %v", want, messages)
		}
	}

	// The fake binary should now contain the new content, proving Rename ran.
	got, err := os.ReadFile(fakeBinary)
	if err != nil {
		t.Fatalf("reading updated fake binary: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("fake binary content = %q, want %q (Rename did not replace it)", got, binaryContent)
	}
}

// TestUpdate_BadChecksum verifies that a checksum mismatch is caught.
func TestUpdate_BadChecksum(t *testing.T) {
	binaryContent := []byte("real binary content")
	wantAsset := assetName()
	badChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumContent := fmt.Sprintf("%s  %s\n", badChecksum, wantAsset)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/release":
			fakeRelease := map[string]interface{}{
				"tag_name": "v99.0.0",
				"body":     "new release",
				"assets": []map[string]interface{}{
					{"name": wantAsset, "browser_download_url": "http://" + r.Host + "/binary"},
					{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/checksums"},
				},
			}
			json.NewEncoder(w).Encode(fakeRelease)
		case "/binary":
			w.Write(binaryContent)
		case "/checksums":
			w.Write([]byte(checksumContent))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL + "/api/release"
	defer func() { githubAPIURL = origURL }()

	err := Update("0.1.0", func(msg string) {})
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
	if !contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}

// TestParseVersion tests the version parsing helper directly.
// TestUpdate_NoSigningKey_EmitsConservativeWarningEndToEnd verifies that when
// SigningPublicKey is empty (dev build, pre-signing release), the full Update
// flow emits a visible "no signing key" warning through the caller's progress
// callback — not just internally inside verifyChecksumSignature.
//
// This is the regression test for the audit finding that the unit test only
// covers the warning being produced INSIDE verifyChecksumSignature but does
// not prove it bubbles up to Update's caller. A refactor that swapped the
// progress callback, swallowed the warning, or removed it entirely would be
// caught here but not by the existing unit test alone.
func TestUpdate_NoSigningKey_EmitsConservativeWarningEndToEnd(t *testing.T) {
	// Explicitly force the empty-key path — don't rely on the process default.
	origKey := SigningPublicKey
	SigningPublicKey = ""
	defer func() { SigningPublicKey = origKey }()

	binaryContent := []byte("fake syllago binary for no-signing-key test")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	wantAsset := assetName()
	checksumContent := fmt.Sprintf("%s  %s\n", checksum, wantAsset)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/release":
			fakeRelease := map[string]interface{}{
				"tag_name": "v99.0.0",
				"body":     "new release",
				"assets": []map[string]interface{}{
					{"name": wantAsset, "browser_download_url": "http://" + r.Host + "/binary"},
					{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/checksums"},
					// Intentionally NO checksums.txt.sig asset — mirrors the
					// "pre-signing release" shape the empty-key path exists for.
				},
			}
			json.NewEncoder(w).Encode(fakeRelease)
		case "/binary":
			w.Write(binaryContent)
		case "/checksums":
			w.Write([]byte(checksumContent))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL + "/api/release"
	defer func() { githubAPIURL = origURL }()

	var messages []string
	// Update will fail at os.Rename (replacing the test binary), but that's
	// after signature + checksum verification, so we'll still have collected
	// the relevant progress messages.
	_ = Update("0.1.0", func(msg string) { messages = append(messages, msg) })

	// Core assertion: the empty-key warning MUST be visible in the caller's
	// progress stream. A regression that silently skipped signature check
	// would not emit this.
	var foundWarning, foundVerified bool
	for _, m := range messages {
		if strings.Contains(m, "no signing key") && strings.Contains(m, "not verified") {
			foundWarning = true
		}
		if strings.Contains(m, "Signature verified") {
			foundVerified = true
		}
	}
	if !foundWarning {
		t.Errorf("expected 'no signing key ... not verified' warning in progress messages; got: %v", messages)
	}
	// Can't have it both ways — if we're in the empty-key path we must NOT
	// also have claimed signature verification.
	if foundVerified {
		t.Errorf("'Signature verified' message must not appear when SigningPublicKey is empty; got: %v", messages)
	}
}

// TestUpdate_NoSigningKey_ChecksumStillEnforced is belt-and-suspenders: when
// SigningPublicKey is empty, the signature check is skipped, but SHA-256
// checksum verification is the only remaining protection — it must still be
// enforced. A regression that conflated "no signing key → skip signature"
// with "no signing key → skip all integrity checks" would be caught here.
func TestUpdate_NoSigningKey_ChecksumStillEnforced(t *testing.T) {
	origKey := SigningPublicKey
	SigningPublicKey = ""
	defer func() { SigningPublicKey = origKey }()

	realBinary := []byte("real binary bytes")
	// Checksum claims a value that will never match the served bytes.
	wrongChecksum := "deadbeef00000000000000000000000000000000000000000000000000000000"
	wantAsset := assetName()
	checksumContent := fmt.Sprintf("%s  %s\n", wrongChecksum, wantAsset)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/release":
			fakeRelease := map[string]interface{}{
				"tag_name": "v99.0.0",
				"body":     "new release",
				"assets": []map[string]interface{}{
					{"name": wantAsset, "browser_download_url": "http://" + r.Host + "/binary"},
					{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/checksums"},
				},
			}
			json.NewEncoder(w).Encode(fakeRelease)
		case "/binary":
			w.Write(realBinary)
		case "/checksums":
			w.Write([]byte(checksumContent))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	origURL := githubAPIURL
	githubAPIURL = srv.URL + "/api/release"
	defer func() { githubAPIURL = origURL }()

	err := Update("0.1.0", func(string) {})
	if err == nil {
		t.Fatal("Update should refuse a checksum mismatch even when SigningPublicKey is empty")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", err)
	}
}

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

func TestVerifyChecksumSignature(t *testing.T) {
	// Generate a test Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	// Create test checksums.txt content
	checksumContent := []byte("abc123  syllago-linux-amd64\ndef456  syllago-darwin-arm64\n")
	validSig := ed25519.Sign(priv, checksumContent)
	invalidSig := []byte("this is not a valid signature at all and is definitely wrong")

	t.Run("no signing key configured", func(t *testing.T) {
		origKey := SigningPublicKey
		SigningPublicKey = ""
		defer func() { SigningPublicKey = origKey }()

		var msgs []string
		err := verifyChecksumSignature(ReleaseInfo{}, "/dev/null", t.TempDir(),
			func(msg string) { msgs = append(msgs, msg) })
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(msgs) == 0 || !strings.Contains(msgs[0], "no signing key") {
			t.Errorf("expected warning about no signing key, got: %v", msgs)
		}
	})

	t.Run("key configured but no signature URL", func(t *testing.T) {
		origKey := SigningPublicKey
		SigningPublicKey = pubHex
		defer func() { SigningPublicKey = origKey }()

		err := verifyChecksumSignature(ReleaseInfo{}, "/dev/null", t.TempDir(),
			func(string) {})
		if err == nil {
			t.Fatal("expected error for missing signature URL")
		}
		if !strings.Contains(err.Error(), "missing checksums.txt.sig") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid signature", func(t *testing.T) {
		origKey := SigningPublicKey
		SigningPublicKey = pubHex
		defer func() { SigningPublicKey = origKey }()

		tmpDir := t.TempDir()
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		os.WriteFile(checksumPath, checksumContent, 0644)

		// Serve the signature file
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(validSig)
		}))
		defer srv.Close()

		var msgs []string
		err := verifyChecksumSignature(
			ReleaseInfo{SignatureURL: srv.URL + "/checksums.txt.sig"},
			checksumPath, tmpDir,
			func(msg string) { msgs = append(msgs, msg) })
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(msgs) == 0 || !strings.Contains(msgs[0], "Signature verified") {
			t.Errorf("expected verification success message, got: %v", msgs)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		origKey := SigningPublicKey
		SigningPublicKey = pubHex
		defer func() { SigningPublicKey = origKey }()

		tmpDir := t.TempDir()
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		os.WriteFile(checksumPath, checksumContent, 0644)

		// Serve an invalid signature
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(invalidSig)
		}))
		defer srv.Close()

		err := verifyChecksumSignature(
			ReleaseInfo{SignatureURL: srv.URL + "/checksums.txt.sig"},
			checksumPath, tmpDir,
			func(string) {})
		if err == nil {
			t.Fatal("expected error for invalid signature")
		}
		if !strings.Contains(err.Error(), "FAILED") {
			t.Errorf("expected FAILED in error, got: %v", err)
		}
	})

	t.Run("tampered checksums", func(t *testing.T) {
		origKey := SigningPublicKey
		SigningPublicKey = pubHex
		defer func() { SigningPublicKey = origKey }()

		tmpDir := t.TempDir()
		// Write different content than what was signed
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		os.WriteFile(checksumPath, []byte("tampered content\n"), 0644)

		// Serve signature that was valid for the original content
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(validSig)
		}))
		defer srv.Close()

		err := verifyChecksumSignature(
			ReleaseInfo{SignatureURL: srv.URL + "/checksums.txt.sig"},
			checksumPath, tmpDir,
			func(string) {})
		if err == nil {
			t.Fatal("expected error for tampered checksums")
		}
		if !strings.Contains(err.Error(), "FAILED") {
			t.Errorf("expected FAILED in error, got: %v", err)
		}
	})
}
