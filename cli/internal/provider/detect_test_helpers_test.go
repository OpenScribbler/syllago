package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// makeFakeBinary writes an executable file named `name` into a fresh temp
// dir, sets PATH to that dir for the duration of the test, and returns the
// temp dir. Used by provider Detect() tests to simulate "binary on PATH"
// without depending on the host having the real CLI installed.
//
// Why a real file instead of mocking exec.LookPath: Detect() functions call
// the binaryOnPath helper directly, which calls exec.LookPath. Mocking would
// require threading a function variable through every provider, which
// complicates the API for one test code path. PATH redirection costs nothing
// at runtime and keeps Detect() implementations simple.
func makeFakeBinary(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir
}

// scrubPATH points PATH at a non-existent dir so binaryOnPath returns false
// for any name. Used to assert the "no binary" branch of Detect().
func scrubPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "/syllago-nonexistent-dir-xyz")
}
