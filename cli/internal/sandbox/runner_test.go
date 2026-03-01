package sandbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSession_DirSafetyFails(t *testing.T) {
	// Home directory should fail dir safety validation.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	w := devNull(t)
	err = RunSession(RunConfig{
		ProviderSlug: "claude-code",
		ProjectDir:   home,
		HomeDir:      home,
	}, w)

	if err == nil {
		t.Fatal("expected error for unsafe dir (home dir)")
	}
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "too close") {
		t.Errorf("expected directory safety error, got: %s", err)
	}
}

func TestRunSession_ForceDirPrintsWarning(t *testing.T) {
	// With ForceDir=true, an unsafe dir should get past safety check but still
	// fail later at pre-flight (unknown provider won't resolve).
	tmpDir := t.TempDir()

	tmpFile := filepath.Join(t.TempDir(), "output")
	w, err := os.Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	_ = RunSession(RunConfig{
		ProviderSlug: "nonexistent-provider",
		ProjectDir:   tmpDir,
		HomeDir:      t.TempDir(),
		ForceDir:     true,
	}, w)
	w.Close()

	data, _ := os.ReadFile(tmpFile)
	if !strings.Contains(string(data), "WARNING") {
		t.Error("expected WARNING in output when ForceDir is used")
	}
}

func TestRunSession_CleansStaleDirs(t *testing.T) {
	// Create a stale staging dir.
	stale := filepath.Join("/tmp", "syllago-sandbox-stalerunnertest")
	os.MkdirAll(stale, 0700)
	defer os.RemoveAll(stale) // cleanup if test fails

	// RunSession will call CleanStale, then fail at pre-flight (unknown provider).
	w := devNull(t)
	_ = RunSession(RunConfig{
		ProviderSlug: "nonexistent-provider",
		ProjectDir:   t.TempDir(),
		HomeDir:      t.TempDir(),
		ForceDir:     true,
	}, w)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("expected stale staging dir to be cleaned up")
	}
}

func TestRunSession_EnvSummaryPrinted(t *testing.T) {
	// This test requires bwrap+socat on PATH to reach the summary.
	// Skip if they're not available.
	if _, err := findBwrap(); err != nil {
		t.Skip("bwrap not available, skipping summary test")
	}
	if _, err := findSocat(); err != nil {
		t.Skip("socat not available, skipping summary test")
	}
	t.Skip("full integration test requires provider binary — skipped in unit tests")
}

// devNull returns an os.File that discards all writes.
func devNull(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// findBwrap is a test helper to check if bwrap is available.
func findBwrap() (string, error) {
	var buf bytes.Buffer
	_ = buf // suppress unused
	r := Check("", "/tmp", "/tmp")
	if !r.BwrapOK {
		return "", os.ErrNotExist
	}
	return "bwrap", nil
}

// findSocat is a test helper to check if socat is available.
func findSocat() (string, error) {
	r := Check("", "/tmp", "/tmp")
	if !r.SocatOK {
		return "", os.ErrNotExist
	}
	return "socat", nil
}
