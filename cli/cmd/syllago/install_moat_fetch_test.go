package main

// Tests for install_moat_fetch.go (bead syllago-u128o).
//
// Scope:
//   - downloadTarball — 200 path, non-200 failure, oversize guard.
//   - extractGzipTarball — regular files, nested dirs, path-traversal
//     rejection (absolute, "..", symlinks out of tree).
//   - fetchAndRecord — end-to-end Proceed path: fetch + hash-verify +
//     extract + RecordInstall + lf.Save. Also exercises the two early
//     refusals (non-UNSIGNED tier, non-https scheme) and the hash-mismatch
//     failure.
//
// All tests use httptest.NewServer for HTTPS tarballs (served over HTTP —
// the client's URL string uses https:// only in the manifest field; the
// Proceed-path check on the SourceURI verifies the declared scheme, which
// is fine because the real scheme check happens in Go not in the network
// layer). Hash inputs are deterministic so lockfile entries can be
// asserted exactly.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// withTLSClient swaps moatFetchClient for one that trusts httptest's
// self-signed cert. Tests that exercise the Proceed path must serve over
// TLS because fetchAndRecord enforces https:// on SourceURI.
func withTLSClient(t *testing.T) func() {
	t.Helper()
	orig := moatFetchClient
	moatFetchClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	return func() { moatFetchClient = orig }
}

func TestDownloadTarball_OK(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := []byte("hello world")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	got, err := downloadTarball(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("downloadTarball: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch: got %q, want %q", got, body)
	}
}

func TestDownloadTarball_Non200(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := downloadTarball(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got %v", err)
	}
}

func TestDownloadTarball_Oversize(t *testing.T) {
	// Temporarily lower the cap so the oversize path is cheap to exercise.
	// The cap is a const in package main; we can't flip it, so instead we
	// install a client that returns a body larger than the cap via a
	// httptest server and rely on the LimitReader to trip at cap+1.
	//
	// Easier: serve a body of cap+1 bytes. That is exactly moatFetchMaxBytes+1,
	// which is 100 MiB + 1 — too slow. So: swap moatFetchClient for one that
	// streams a sentinel body through a small-cap ReaderCloser, but the cap
	// itself is a const so we exercise an artificially lowered mock.
	//
	// Practical approach: stub the client to return a response with a
	// Content-Length and io.Reader that returns cap+1 bytes of zero. We use
	// a fake RoundTripper to avoid shipping 100 MiB over localhost.
	origClient := moatFetchClient
	moatFetchClient = &http.Client{
		Transport: oversizeRoundTripper{size: moatFetchMaxBytes + 10},
	}
	t.Cleanup(func() { moatFetchClient = origClient })

	_, err := downloadTarball(context.Background(), "https://example.com/big.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected oversize error, got %v", err)
	}
}

type oversizeRoundTripper struct{ size int }

func (rt oversizeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body := &zeroReader{n: rt.size}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(body),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

type zeroReader struct {
	n    int
	read int
}

func (r *zeroReader) Read(p []byte) (int, error) {
	if r.read >= r.n {
		return 0, io.EOF
	}
	remaining := r.n - r.read
	if remaining > len(p) {
		remaining = len(p)
	}
	for i := 0; i < remaining; i++ {
		p[i] = 0
	}
	r.read += remaining
	return remaining, nil
}

func TestExtractGzipTarball_Ok(t *testing.T) {
	body := buildTarGz(t, map[string]string{
		"SKILL.md":      "# My Skill\n",
		"prompts/a.md":  "A\n",
		"prompts/b.md":  "B\n",
		"nested/deep/x": "x\n",
	})
	dest := filepath.Join(t.TempDir(), "out")
	if err := extractGzipTarball(body, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}

	// Verify each file landed where expected.
	for name, want := range map[string]string{
		"SKILL.md":      "# My Skill\n",
		"prompts/a.md":  "A\n",
		"prompts/b.md":  "B\n",
		"nested/deep/x": "x\n",
	} {
		p := filepath.Join(dest, name)
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("read %q: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%q = %q, want %q", name, got, want)
		}
	}
}

func TestExtractGzipTarball_RejectsPathTraversal(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"dotdot", "../etc/passwd"},
		{"absolute", "/etc/passwd"},
		{"nested-dotdot", "foo/../../../etc/passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := buildTarGz(t, map[string]string{tc.path: "evil"})
			dest := filepath.Join(t.TempDir(), "out")
			err := extractGzipTarball(body, dest)
			if err == nil || !strings.Contains(err.Error(), "escapes destination") {
				t.Errorf("expected escapes-destination error, got %v", err)
			}
		})
	}
}

func TestExtractGzipTarball_SkipsSymlinks(t *testing.T) {
	// A symlink-type header must not be materialized. Build the tar by
	// hand to include a symlink header.
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "evil-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     "regular.txt",
		Typeflag: tar.TypeReg,
		Size:     4,
		Mode:     0o644,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hi!\n")); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gzw.Close()

	dest := filepath.Join(t.TempDir(), "out")
	if err := extractGzipTarball(buf.Bytes(), dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "evil-link")); !os.IsNotExist(err) {
		t.Errorf("symlink should have been skipped; got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "regular.txt")); err != nil {
		t.Errorf("regular file should have been written: %v", err)
	}
}

func TestFetchAndRecord_Happy_Unsigned(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := buildTarGz(t, map[string]string{"SKILL.md": "# hi\n"})
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		Type:        "skill",
		ContentHash: entryHash,
		SourceURI:   srv.URL,
		AttestedAt:  time.Now().UTC(),
	}

	origNow := moatInstallNow
	moatInstallNow = func() time.Time { return time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { moatInstallNow = origNow })

	dir, err := fetchAndRecord(
		context.Background(),
		entry,
		"example",
		"https://example.com/manifest.json",
		moat.LockfilePath(projectRoot),
		lf,
		nil, // UNSIGNED → no profile required
		nil, // UNSIGNED → no trusted root required
	)
	if err != nil {
		t.Fatalf("fetchAndRecord: %v", err)
	}

	// Cache directory: <root>/example/my-skill/<12 hex>/
	if !strings.Contains(dir, "example/my-skill") {
		t.Errorf("cache dir path missing registry/item components: %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md missing from cache: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Fatalf("lockfile should have 1 entry, got %d", len(lf.Entries))
	}
	if lf.Entries[0].ContentHash != entryHash {
		t.Errorf("lockfile hash = %q, want %q", lf.Entries[0].ContentHash, entryHash)
	}
	if lf.Entries[0].TrustTier != "UNSIGNED" {
		t.Errorf("lockfile trust_tier = %q, want UNSIGNED", lf.Entries[0].TrustTier)
	}

	// Re-read the persisted lockfile to confirm lf.Save actually ran.
	onDisk, err := moat.LoadLockfile(moat.LockfilePath(projectRoot))
	if err != nil {
		t.Fatalf("LoadLockfile: %v", err)
	}
	if len(onDisk.Entries) != 1 {
		t.Errorf("on-disk lockfile should have 1 entry, got %d", len(onDisk.Entries))
	}
}

func TestFetchAndRecord_HashMismatch(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := []byte("not-a-tarball-but-bytes")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	wrongHash := "sha256:" + strings.Repeat("aa", 32)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	lf := &moat.Lockfile{}
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		ContentHash: wrongHash,
		SourceURI:   srv.URL,
	}
	_, err := fetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lockfile.json", lf, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "content_hash mismatch") {
		t.Errorf("expected hash-mismatch message; got %+v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated on hash-mismatch; got %d entries", len(lf.Entries))
	}
}

// withVerifyItemStub swaps the per-item verification seam so wiring tests
// can short-circuit the real sigstore-go crypto path. Returns a *captured*
// pointer that records the AttestationItem and rekorRaw bytes the
// production code passed in — wiring tests assert on those to prove the
// ContentEntry → AttestationItem mapping and the Rekor body round-trip
// were correct.
func withVerifyItemStub(t *testing.T, result moat.VerificationResult, retErr error) *struct {
	Item    moat.AttestationItem
	Profile *moat.SigningProfile
	Rekor   []byte
	Called  int
} {
	t.Helper()
	captured := &struct {
		Item    moat.AttestationItem
		Profile *moat.SigningProfile
		Rekor   []byte
		Called  int
	}{}
	orig := moatVerifyItem
	moatVerifyItem = func(item moat.AttestationItem, profile *moat.SigningProfile, rekorRaw []byte, trustedRootJSON []byte) (moat.VerificationResult, error) {
		captured.Item = item
		captured.Profile = profile
		captured.Rekor = append([]byte(nil), rekorRaw...)
		captured.Called++
		return result, retErr
	}
	t.Cleanup(func() { moatVerifyItem = orig })
	return captured
}

// withRekorStub swaps the package-level rekorBaseURL so FetchRekorEntry
// hits a httptest server. Wiring tests use this rather than stubbing
// FetchRekorEntry itself — proves the byte pipe is intact end-to-end.
func withRekorStub(t *testing.T, body []byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	orig := moat.RekorBaseURLForTest()
	moat.SetRekorBaseURLForTest(srv.URL)
	t.Cleanup(func() { moat.SetRekorBaseURLForTest(orig) })
}

// signedFixture returns a fully-populated SIGNED-tier ContentEntry whose
// SourceURI is the supplied tarball server. Used by the happy-path SIGNED
// + DUAL-ATTESTED tests.
func signedFixture(srvURL, contentHash string, withProfile bool) *moat.ContentEntry {
	logIndex := int64(1336116369)
	entry := &moat.ContentEntry{
		Name:          "my-skill",
		Type:          "skill",
		ContentHash:   contentHash,
		SourceURI:     srvURL,
		AttestedAt:    time.Now().UTC(),
		RekorLogIndex: &logIndex,
	}
	if withProfile {
		entry.SigningProfile = &moat.SigningProfile{
			Issuer:  "https://token.actions.githubusercontent.com",
			Subject: "https://github.com/example/repo/.github/workflows/sign.yml@refs/heads/main",
		}
	}
	return entry
}

func TestFetchAndRecord_Happy_Signed(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := buildTarGz(t, map[string]string{"SKILL.md": "# hi\n"})
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	rekorBytes := []byte(`{"abc123":{"body":"...","integratedTime":1700000000,"logID":"deadbeef","logIndex":1336116369,"verification":{"inclusionProof":{"checkpoint":"","hashes":[],"logIndex":1336116369,"rootHash":"","treeSize":100},"signedEntryTimestamp":""}}}`)
	withRekorStub(t, rekorBytes)
	captured := withVerifyItemStub(t, moat.VerificationResult{}, nil)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := signedFixture(srv.URL, entryHash, false) // SIGNED — no per-item profile
	registryProfile := &moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	origNow := moatInstallNow
	moatInstallNow = func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { moatInstallNow = origNow })

	dir, err := fetchAndRecord(
		context.Background(),
		entry,
		"example",
		"https://example.com/manifest.json",
		moat.LockfilePath(projectRoot),
		lf,
		registryProfile,
		[]byte(`{"trusted":"root"}`),
	)
	if err != nil {
		t.Fatalf("fetchAndRecord: %v", err)
	}
	if !strings.Contains(dir, "example/my-skill") {
		t.Errorf("cache dir path missing registry/item components: %s", dir)
	}

	// Verify call: SIGNED tier should fall back to registry-level profile.
	if captured.Called != 1 {
		t.Fatalf("verify called %d times, want 1", captured.Called)
	}
	if captured.Profile != registryProfile {
		t.Errorf("SIGNED tier should pass registry-level profile to verify, got %+v", captured.Profile)
	}
	if captured.Item.ContentHash != entryHash {
		t.Errorf("AttestationItem.ContentHash = %q, want %q", captured.Item.ContentHash, entryHash)
	}
	if captured.Item.RekorLogIndex != 1336116369 {
		t.Errorf("AttestationItem.RekorLogIndex = %d, want 1336116369", captured.Item.RekorLogIndex)
	}
	if !bytes.Equal(captured.Rekor, rekorBytes) {
		t.Errorf("rekor bytes did not round-trip verbatim")
	}

	// Lockfile entry: SIGNED tier, AttestationBundle == rekorBytes.
	if len(lf.Entries) != 1 {
		t.Fatalf("lockfile should have 1 entry, got %d", len(lf.Entries))
	}
	if lf.Entries[0].TrustTier != "SIGNED" {
		t.Errorf("lockfile trust_tier = %q, want SIGNED", lf.Entries[0].TrustTier)
	}
	if !bytes.Equal(lf.Entries[0].AttestationBundle, rekorBytes) {
		t.Errorf("lockfile AttestationBundle did not preserve Rekor bytes")
	}
}

func TestFetchAndRecord_Happy_DualAttested(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := buildTarGz(t, map[string]string{"SKILL.md": "# hi\n"})
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	rekorBytes := []byte(`{"abc123":{"body":"...","integratedTime":1700000000,"logID":"deadbeef","logIndex":1336116369,"verification":{"inclusionProof":{"checkpoint":"","hashes":[],"logIndex":1336116369,"rootHash":"","treeSize":100},"signedEntryTimestamp":""}}}`)
	withRekorStub(t, rekorBytes)
	captured := withVerifyItemStub(t, moat.VerificationResult{}, nil)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := signedFixture(srv.URL, entryHash, true) // DUAL-ATTESTED — per-item profile
	registryProfile := &moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	if _, err := fetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		moat.LockfilePath(projectRoot), lf, registryProfile, []byte(`{"trusted":"root"}`),
	); err != nil {
		t.Fatalf("fetchAndRecord: %v", err)
	}

	// DUAL-ATTESTED MUST use the per-item signing profile, not the
	// registry-level one. This is the spec contract for tier resolution.
	if captured.Profile != entry.SigningProfile {
		t.Errorf("DUAL-ATTESTED must use per-item profile; got %+v", captured.Profile)
	}
	if lf.Entries[0].TrustTier != "DUAL-ATTESTED" {
		t.Errorf("lockfile trust_tier = %q, want DUAL-ATTESTED", lf.Entries[0].TrustTier)
	}
}

func TestFetchAndRecord_Signed_RekorFetchFails(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := buildTarGz(t, map[string]string{"SKILL.md": "# hi\n"})
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	// Rekor server returns 404 — fetch should fail before verify.
	rekorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(rekorSrv.Close)
	orig := moat.RekorBaseURLForTest()
	moat.SetRekorBaseURLForTest(rekorSrv.URL)
	t.Cleanup(func() { moat.SetRekorBaseURLForTest(orig) })

	verifyCalled := 0
	origVerify := moatVerifyItem
	moatVerifyItem = func(_ moat.AttestationItem, _ *moat.SigningProfile, _ []byte, _ []byte) (moat.VerificationResult, error) {
		verifyCalled++
		return moat.VerificationResult{}, nil
	}
	t.Cleanup(func() { moatVerifyItem = origVerify })

	cacheRoot := t.TempDir()
	origCache := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = origCache })

	lf := &moat.Lockfile{}
	entry := signedFixture(srv.URL, entryHash, false)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := fetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	if verifyCalled != 0 {
		t.Errorf("verify must not run when Rekor fetch fails; called %d times", verifyCalled)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when Rekor fetch fails; got %d entries", len(lf.Entries))
	}
}

func TestFetchAndRecord_Signed_VerifyFails(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	body := buildTarGz(t, map[string]string{"SKILL.md": "# hi\n"})
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	rekorBytes := []byte(`{"abc123":{"body":"...","logIndex":1336116369}}`)
	withRekorStub(t, rekorBytes)
	verifyErr := errors.New("identity mismatch")
	withVerifyItemStub(t, moat.VerificationResult{}, verifyErr)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	lf := &moat.Lockfile{}
	entry := signedFixture(srv.URL, entryHash, false)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := fetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when verify fails; got %d entries", len(lf.Entries))
	}
}

func TestFetchAndRecord_Signed_RequiresProfile(t *testing.T) {
	// SIGNED tier with neither a per-item nor manifest-level profile is a
	// structural error. The install gate should not have proceeded; fail
	// here as a defense-in-depth backstop rather than silently install
	// without an identity check.
	idx := int64(42)
	entry := &moat.ContentEntry{
		Name:          "my-skill",
		ContentHash:   "sha256:" + strings.Repeat("cc", 32),
		SourceURI:     "https://example.com/x.tar.gz",
		RekorLogIndex: &idx,
	}
	_, err := fetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", &moat.Lockfile{}, nil, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
}

func TestFetchAndRecord_RefusesNonHTTPSScheme(t *testing.T) {
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		ContentHash: "sha256:" + strings.Repeat("cc", 32),
		SourceURI:   "git+https://example.com/repo.git",
	}
	_, err := fetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lockfile.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "scheme not supported") {
		t.Errorf("expected scheme-not-supported refusal; got %+v", err)
	}
}

func TestFetchAndRecord_NilGuards(t *testing.T) {
	if _, err := fetchAndRecord(context.Background(), nil, "r", "u", "p", &moat.Lockfile{}, nil, nil); err == nil {
		t.Error("expected error on nil entry")
	}
	entry := &moat.ContentEntry{Name: "x", ContentHash: "sha256:aa", SourceURI: "https://x"}
	if _, err := fetchAndRecord(context.Background(), entry, "r", "u", "p", nil, nil, nil); err == nil {
		t.Error("expected error on nil lockfile")
	}
}

func TestFetchAndRecord_ExtractFailsOnCorruptGzip(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	// Serve bytes whose sha256 matches but are not valid gzip — this
	// exercises the extractGzipTarball error path after a successful
	// hash-verify.
	body := []byte("definitely not gzip")
	sum := sha256.Sum256(body)
	entryHash := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	cacheRoot := t.TempDir()
	orig := moatSourceCacheDir
	moatSourceCacheDir = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { moatSourceCacheDir = orig })

	entry := &moat.ContentEntry{
		Name:        "my-skill",
		ContentHash: entryHash,
		SourceURI:   srv.URL,
	}
	_, err := fetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lf.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrSystemIO)
}

func TestFetchAndRecord_FetchFailure(t *testing.T) {
	t.Cleanup(withTLSClient(t))
	// No server running at this URL — the client should fail cleanly.
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		ContentHash: "sha256:" + strings.Repeat("aa", 32),
		SourceURI:   "https://127.0.0.1:1/unreachable.tar.gz",
	}
	_, err := fetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lf.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "could not fetch") {
		t.Errorf("expected fetch-failure message; got %+v", err)
	}
}

// buildTarGz builds an in-memory .tar.gz from the given files map.
// Keys are slash-separated paths relative to the archive root.
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	_ = fmt.Sprintf // silence unused-import guard across edits
	return buf.Bytes()
}
