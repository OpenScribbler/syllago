package moat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Canonical shape of a trusted_root.json. The loader only needs the bytes to
// parse as JSON; it does not schema-validate (sigstore-go does that at
// verify time). So a minimal object is enough to exercise the happy path.
var minimalTrustedRootJSON = []byte(`{
  "mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
  "tlogs": [],
  "certificateAuthorities": [],
  "ctlogs": [],
  "timestampAuthorities": []
}`)

// TestTrustedRootFromPath_Success — valid file returns Source=path,
// Status=Fresh, bytes populated, no staleness fields set.
func TestTrustedRootFromPath_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_root.json")
	if err := os.WriteFile(path, minimalTrustedRootJSON, 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	info, err := TrustedRootFromPath(path, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Source != TrustedRootSourcePathFlag {
		t.Errorf("Source = %q, want %q", info.Source, TrustedRootSourcePathFlag)
	}
	if info.Status != TrustedRootStatusFresh {
		t.Errorf("Status = %v, want Fresh — override paths are never staleness-gated", info.Status)
	}
	if len(info.Bytes) == 0 {
		t.Error("Bytes must be populated on success")
	}
	if !info.IssuedAt.IsZero() {
		t.Errorf("IssuedAt must be zero for override paths (operator-owned), got %v", info.IssuedAt)
	}
	if info.AgeDays != 0 {
		t.Errorf("AgeDays must be 0 for override paths, got %d", info.AgeDays)
	}
}

// TestTrustedRootFromPath_EmptyPath — defensive: empty path is a caller
// bug (callers should gate on reg.TrustedRoot != "" before calling). Must
// surface as an error, not a silent success with empty bytes.
func TestTrustedRootFromPath_EmptyPath(t *testing.T) {
	t.Parallel()
	_, err := TrustedRootFromPath("", time.Now())
	if err == nil {
		t.Fatal("empty path must error")
	}
}

// TestTrustedRootFromPath_MissingFile — nonexistent path must error with
// a message that names the path (for the MOAT_007 Details line).
func TestTrustedRootFromPath_MissingFile(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := TrustedRootFromPath(missing, time.Now())
	if err == nil {
		t.Fatal("missing file must error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error must name the path for operator triage: %v", err)
	}
}

// TestTrustedRootFromPath_MalformedJSON — file exists but parse fails.
// This guards against operators pointing at the wrong file (README, cert
// bundle, truncated download).
func TestTrustedRootFromPath_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json at all {"), 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	_, err := TrustedRootFromPath(path, time.Now())
	if err == nil {
		t.Fatal("malformed JSON must error")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error must indicate a parse failure, got %v", err)
	}
}

// TestTrustedRootFromPath_PermissionDenied — file exists but unreadable.
// Exercise the os.ReadFile failure branch distinct from ErrNotExist.
func TestTrustedRootFromPath_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(path, minimalTrustedRootJSON, 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	_, err := TrustedRootFromPath(path, time.Now())
	if err == nil {
		t.Fatal("unreadable file must error")
	}
}
