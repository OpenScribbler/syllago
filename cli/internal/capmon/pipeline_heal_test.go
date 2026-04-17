package capmon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Minimal manifest used by ProposeManifestHealPR in healing tests.
const pipelineHealManifest = `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources:
      - url: "__URL__"
        type: documentation
        format: md
        selector: {}
`

func TestTryHealSource_DisabledReturnsNil(t *testing.T) {
	disabled := false
	src := SourceEntry{
		URL:     "https://example.com/missing.md",
		Healing: &HealingConfig{Enabled: &disabled},
	}
	evt := tryHealSource(context.Background(), PipelineOptions{}, "test-provider", "skills", 0, src, errors.New("404"), "run-1")
	if evt != nil {
		t.Errorf("expected nil when healing disabled, got %+v", evt)
	}
}

func TestTryHealSource_FailureRecordsCounter(t *testing.T) {
	// No reachable server, variant strategy only — all probes will fail.
	cacheRoot := t.TempDir()
	src := SourceEntry{
		URL:     "https://127.0.0.1:1/docs/nope.md",
		Healing: &HealingConfig{Strategies: []string{"variant"}},
	}
	opts := PipelineOptions{CacheRoot: cacheRoot, RepoRoot: t.TempDir()}

	evt := tryHealSource(context.Background(), opts, "test-provider", "skills", 0, src, errors.New("404"), "run-1")
	if evt == nil {
		t.Fatal("expected event, got nil")
	}
	if evt.Success {
		t.Errorf("expected Success=false")
	}
	if evt.FailReason == "" {
		t.Error("FailReason should be set on failure")
	}
	// Counter file should have been written.
	counterPath := healFailureCountFile(cacheRoot, "test-provider", "skills", 0)
	if _, err := os.Stat(counterPath); err != nil {
		t.Errorf("counter file not created: %v", err)
	}
}

func TestTryHealSource_DryRunDoesNotOpenPR(t *testing.T) {
	// Set up a variant that succeeds, but DryRun should short-circuit PR open.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write(validBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		t.Errorf("gh must not be called under DryRun, got: %v", args)
		return nil, nil
	})
	defer SetGHCommandForTest(nil)
	SetGitRunnerForTest(func(dir string, args ...string) ([]byte, error) {
		t.Errorf("git must not be called under DryRun, got: %v", args)
		return nil, nil
	})
	defer SetGitRunnerForTest(nil)

	src := SourceEntry{
		URL:     srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{Strategies: []string{"variant"}},
	}
	opts := PipelineOptions{DryRun: true, CacheRoot: t.TempDir(), RepoRoot: t.TempDir()}
	evt := tryHealSource(context.Background(), opts, "test-provider", "skills", 0, src, errors.New("404"), "run-1")
	if evt == nil || !evt.Success {
		t.Fatalf("expected success event, got %+v", evt)
	}
	if evt.PRURL != "" {
		t.Errorf("DryRun must not populate PRURL, got %q", evt.PRURL)
	}
	if evt.Strategy != "variant" {
		t.Errorf("Strategy = %q, want variant", evt.Strategy)
	}
}

func TestTryHealSource_SuccessOpensPRAndClearsCounter(t *testing.T) {
	// Set up a variant that succeeds and stub gh/git so PR open works.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write(validBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	repoDir := t.TempDir()
	manifestsDir := filepath.Join(repoDir, "sources")
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(manifestsDir, "test-provider.yaml")
	manifestBody := strings.ReplaceAll(pipelineHealManifest, "__URL__", srv.URL+"/docs/foo-bar.md")
	if err := os.WriteFile(manifestPath, []byte(manifestBody), 0644); err != nil {
		t.Fatal(err)
	}

	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if args[0] == "pr" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if args[0] == "pr" && args[1] == "create" {
			return []byte("https://github.com/org/repo/pull/777\n"), nil
		}
		return nil, nil
	})
	defer SetGHCommandForTest(nil)
	SetGitRunnerForTest(func(dir string, args ...string) ([]byte, error) { return nil, nil })
	defer SetGitRunnerForTest(nil)

	// Seed a stale counter so we can verify ResolveHealFailure cleared it.
	cacheRoot := t.TempDir()
	counterPath := healFailureCountFile(cacheRoot, "test-provider", "skills", 0)
	if err := os.MkdirAll(filepath.Dir(counterPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(counterPath, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}

	src := SourceEntry{
		URL:     srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{Strategies: []string{"variant"}},
	}
	opts := PipelineOptions{
		CacheRoot:          cacheRoot,
		RepoRoot:           repoDir,
		SourceManifestsDir: manifestsDir,
	}
	evt := tryHealSource(context.Background(), opts, "test-provider", "skills", 0, src, errors.New("404"), "run-42")
	if evt == nil || !evt.Success {
		t.Fatalf("expected success event, got %+v", evt)
	}
	if evt.PRURL != "https://github.com/org/repo/pull/777" {
		t.Errorf("PRURL = %q", evt.PRURL)
	}
	if _, err := os.Stat(counterPath); !os.IsNotExist(err) {
		t.Errorf("counter file should have been removed: %v", err)
	}
}
