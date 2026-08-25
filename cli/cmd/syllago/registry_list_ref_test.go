package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// makeCloneAtRef creates a git repo at the registry clone path for name.
// When detach is true the checkout is left on a detached HEAD, which is what
// registry.IsPinned reports as pinned.
func makeCloneAtRef(t *testing.T, name string, detach bool) {
	t.Helper()
	dir, err := registry.CloneDir(name)
	if err != nil {
		t.Fatalf("registry.CloneDir(%q): %v", name, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitRun("init", "-b", "main")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("name: test-reg\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRun("add", "registry.yaml")
	gitRun("commit", "-m", "initial commit")
	if detach {
		gitRun("checkout", "--detach", gitRun("rev-parse", "HEAD"))
	}
}

func TestRegistryList_RefColumn(t *testing.T) {
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "example/tagged", URL: "https://github.com/example/tagged", Ref: "v1.2.0"},
			{Name: "example/plain", URL: "https://github.com/example/plain"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true
	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "REF") {
		t.Errorf("expected REF column header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "v1.2.0") {
		t.Errorf("expected configured ref v1.2.0 in REF column, got:\n%s", got)
	}
	// An unset ref must not render the internal "default" sentinel.
	if strings.Contains(got, "default") {
		t.Errorf("expected unset ref to render as a dash, not %q, got:\n%s", "default", got)
	}
}

func TestRegistryList_PinnedMarker(t *testing.T) {
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "example/pinned", URL: "https://github.com/example/pinned", Ref: "v1.0.0"},
			{Name: "example/tracking", URL: "https://github.com/example/tracking", Ref: "main"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	makeCloneAtRef(t, "example/pinned", true)
	makeCloneAtRef(t, "example/tracking", false)
	stdout, _ := output.SetForTest(t)

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true
	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	lines := strings.Split(got, "\n")
	var pinnedLine, trackingLine string
	for _, l := range lines {
		if strings.Contains(l, "example/pinned") {
			pinnedLine = l
		}
		if strings.Contains(l, "example/tracking") {
			trackingLine = l
		}
	}
	if pinnedLine == "" || trackingLine == "" {
		t.Fatalf("expected both registries listed, got:\n%s", got)
	}
	if !strings.Contains(pinnedLine, "(pinned)") {
		t.Errorf("expected detached-HEAD clone marked (pinned), got line:\n%s", pinnedLine)
	}
	if strings.Contains(trackingLine, "(pinned)") {
		t.Errorf("expected branch-tracking clone not marked pinned, got line:\n%s", trackingLine)
	}
}

func TestRegistryList_JSONIncludesPinned(t *testing.T) {
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "example/pinned", URL: "https://github.com/example/pinned", Ref: "v1.0.0"},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	makeCloneAtRef(t, "example/pinned", true)
	stdout, _ := output.SetForTest(t)

	origJSON := output.JSON
	output.JSON = true
	t.Cleanup(func() { output.JSON = origJSON })

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true
	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	var items []struct {
		Name   string `json:"name"`
		Ref    string `json:"ref"`
		Pinned bool   `json:"pinned"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal registry list JSON: %v\n%s", err, stdout.String())
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Ref != "v1.0.0" {
		t.Errorf("expected ref v1.0.0 in JSON, got %q", items[0].Ref)
	}
	if !items[0].Pinned {
		t.Errorf("expected pinned=true in JSON for detached-HEAD clone")
	}
}
