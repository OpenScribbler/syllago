package capmon_test

import (
	"os"
	"strings"
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

func TestRealisticUA_NotHeadless(t *testing.T) {
	t.Parallel()
	ua := capmon.RealisticUA
	if ua == "" {
		t.Fatal("RealisticUA must not be empty")
	}
	for _, bad := range []string{"HeadlessChrome", "headless", "Headless"} {
		if strings.Contains(ua, bad) {
			t.Errorf("RealisticUA contains bot-detectable substring %q: %s", bad, ua)
		}
	}
	// Should look like a real browser UA
	if !strings.Contains(ua, "Mozilla/5.0") {
		t.Error("RealisticUA should contain Mozilla/5.0 prefix")
	}
	if !strings.Contains(ua, "Chrome/") {
		t.Error("RealisticUA should contain Chrome/ version")
	}
}
