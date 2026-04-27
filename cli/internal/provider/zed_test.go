package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestZedDetect: Zed stores its config at ~/.config/zed/settings.json (Linux)
// — that file is created by Zed on first launch and syllago never writes
// inside ~/.config/zed/. Trust the zed binary on PATH or the settings.json
// marker file.
func TestZedDetect(t *testing.T) {
	t.Run("empty home + no binary", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		if Zed.Detect(home) {
			t.Error("expected false on empty home with no zed binary")
		}
	})

	t.Run("syllago-content-only home", func(t *testing.T) {
		// Zed has no global syllago install path under ~/.config/zed (rules go
		// to project root only), but exercise the regression case by creating
		// a sibling syllago-shaped dir to confirm Detect doesn't match it.
		home := t.TempDir()
		scrubPATH(t)
		if err := os.MkdirAll(filepath.Join(home, ".config", "zed-syllago-fake"), 0755); err != nil {
			t.Fatal(err)
		}
		if Zed.Detect(home) {
			t.Error("expected false when only unrelated dirs exist (regression for syllago-a6ibm)")
		}
	})

	t.Run("binary on PATH", func(t *testing.T) {
		home := t.TempDir()
		makeFakeBinary(t, "zed")
		if !Zed.Detect(home) {
			t.Error("expected true when zed binary is on PATH")
		}
	})

	t.Run("settings.json marker present", func(t *testing.T) {
		home := t.TempDir()
		scrubPATH(t)
		dir := filepath.Join(home, ".config", "zed")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		if !Zed.Detect(home) {
			t.Error("expected true when ~/.config/zed/settings.json exists")
		}
	})
}

func TestZedSupportsTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.MCP} {
		if !Zed.SupportsType(ct) {
			t.Errorf("Zed.SupportsType(%s) = false, want true", ct)
		}
	}
	for _, ct := range []catalog.ContentType{catalog.Skills, catalog.Agents, catalog.Hooks, catalog.Commands} {
		if Zed.SupportsType(ct) {
			t.Errorf("Zed.SupportsType(%s) = true, want false", ct)
		}
	}
}

func TestZedDiscoveryPaths(t *testing.T) {
	t.Parallel()
	paths := Zed.DiscoveryPaths("/tmp/project", catalog.Rules)
	if len(paths) == 0 {
		t.Fatal("expected at least one discovery path for Rules, got none")
	}
	found := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".rules") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a discovery path ending in .rules, got: %v", paths)
	}
}

func TestZedEmitPath(t *testing.T) {
	t.Parallel()
	path := Zed.EmitPath("/tmp/project")
	if path == "" {
		t.Fatal("expected non-empty emit path")
	}
	if !strings.HasSuffix(path, ".rules") {
		t.Errorf("expected emit path ending in .rules, got %q", path)
	}
}

func TestZedFileFormat(t *testing.T) {
	t.Parallel()
	if got := Zed.FileFormat(catalog.Rules); got != FormatMarkdown {
		t.Errorf("Zed.FileFormat(Rules) = %q, want %q", got, FormatMarkdown)
	}
	if got := Zed.FileFormat(catalog.MCP); got != FormatJSON {
		t.Errorf("Zed.FileFormat(MCP) = %q, want %q", got, FormatJSON)
	}
}
