package capmon_test

import (
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestChromedpURLWiring(t *testing.T) {
	orig := os.Getenv("CHROMEDP_URL")
	defer os.Setenv("CHROMEDP_URL", orig)

	os.Setenv("CHROMEDP_URL", "ws://localhost:9222")
	if capmon.ChromedpRemoteURL() != "ws://localhost:9222" {
		t.Error("ChromedpRemoteURL did not read CHROMEDP_URL env var")
	}

	os.Unsetenv("CHROMEDP_URL")
	if capmon.ChromedpRemoteURL() != "" {
		t.Error("ChromedpRemoteURL should return empty when env var not set")
	}
}
