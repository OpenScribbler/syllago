package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points HOME at a throwaway directory, so a test that installs
// without isolating itself writes its install records and lock there rather
// than into the developer's real ~/.syllago.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "syllago-cmd-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
