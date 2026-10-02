package loadout

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// holdInstallLock takes the install lock for the rest of the test and
// shortens the wait so a blocked writer fails fast.
func holdInstallLock(t *testing.T) {
	t.Helper()
	origDir := config.GlobalDirOverride
	config.GlobalDirOverride = filepath.Join(t.TempDir(), ".syllago")
	origTimeout := syllagolock.DefaultTimeout
	syllagolock.DefaultTimeout = 50 * time.Millisecond

	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() {
		release()
		syllagolock.DefaultTimeout = origTimeout
		config.GlobalDirOverride = origDir
	})
}

func requireLocked(t *testing.T, err error) {
	t.Helper()
	var se output.StructuredError
	if !errors.As(err, &se) || se.Code != output.ErrSystemLocked {
		t.Fatalf("want %s, got %v", output.ErrSystemLocked, err)
	}
}

func TestApplyWaitsForInstallLock(t *testing.T) {
	holdInstallLock(t)
	_, err := Apply(&Manifest{}, nil, stubProviderForPreview("claude-code"), ApplyOptions{ProjectRoot: t.TempDir()})
	requireLocked(t, err)
}

func TestRemoveWaitsForInstallLock(t *testing.T) {
	holdInstallLock(t)
	_, err := Remove(RemoveOptions{ProjectRoot: t.TempDir()})
	requireLocked(t, err)
}
