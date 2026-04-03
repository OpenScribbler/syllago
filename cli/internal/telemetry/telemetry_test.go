package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func resetState() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.disabled = false
	state.anonymousID = ""
	state.endpoint = ""
	state.client = nil
	state.wg = sync.WaitGroup{}
}

func TestInit_DevBuild_NoAPIKey(t *testing.T) {
	origKey := apiKey
	apiKey = ""
	t.Cleanup(func() { apiKey = origKey; resetState() })
	overrideHome(t, t.TempDir())

	Init()
	if !state.disabled {
		t.Error("expected disabled when apiKey is empty")
	}
}

func TestInit_DNTSet(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	t.Setenv("DO_NOT_TRACK", "1")
	overrideHome(t, t.TempDir())

	Init()
	if !state.disabled {
		t.Error("expected disabled when DO_NOT_TRACK=1")
	}
}

func TestInit_UnreadableConfig_DisabledForSession(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	// Create a malformed config.
	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte("{bad"), 0644)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	if !state.disabled {
		t.Error("expected disabled for unreadable config")
	}
}

func TestInit_FirstRun_NoticeWritten(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	if !strings.Contains(notice.String(), "syllago collects anonymous usage data") {
		t.Errorf("first-run notice not written; got: %q", notice.String())
	}
}

func TestInit_NoticeWrittenOnce(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	resetState()
	Init() // second call

	count := strings.Count(notice.String(), "syllago collects anonymous usage data")
	if count != 1 {
		t.Errorf("expected notice exactly once, got %d occurrences", count)
	}
}

func TestTrack_SendsEvent(t *testing.T) {
	var received []postHogPayload
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p postHogPayload
		json.Unmarshal(body, &p)
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	// Override endpoint to the test server after Init().
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	Track("test_event", map[string]any{"command": "install"})
	Shutdown()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Event != "test_event" {
		t.Errorf("unexpected event name: %s", received[0].Event)
	}
	if received[0].APIKey != "phc_test" {
		t.Errorf("unexpected API key: %s", received[0].APIKey)
	}
	if received[0].Properties["command"] != "install" {
		t.Errorf("command property missing")
	}
	if received[0].Properties["os"] == nil {
		t.Error("os property missing")
	}
}

func TestTrack_Disabled_NoHTTP(t *testing.T) {
	origKey := apiKey
	apiKey = ""
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer srv.Close()

	Init()
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	Track("should_not_send", nil)
	Shutdown()

	if called {
		t.Error("HTTP was called despite telemetry being disabled")
	}
}

func TestTrack_Offline_SilentDrop(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	// Point at a guaranteed-unreachable address.
	state.mu.Lock()
	state.endpoint = "http://127.0.0.1:1"
	state.mu.Unlock()

	// Must not panic or return error.
	Track("test", nil)
	Shutdown()
}

func TestSetEnabled(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	cfg, _ := loadUserConfig()
	if cfg == nil || cfg.Enabled {
		t.Error("expected enabled=false after SetEnabled(false)")
	}

	if err := SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true) failed: %v", err)
	}
	cfg, _ = loadUserConfig()
	if cfg == nil || !cfg.Enabled {
		t.Error("expected enabled=true after SetEnabled(true)")
	}
}

func TestReset_GeneratesNewID(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	// Seed with initial config.
	initial, _ := newConfig()
	saveUserConfig(initial)

	newID, err := Reset()
	if err != nil {
		t.Fatalf("Reset() failed: %v", err)
	}
	if newID == initial.AnonymousID {
		t.Error("expected new ID to differ from original")
	}
	if !isValidID(newID) {
		t.Errorf("new ID invalid: %s", newID)
	}
}

func TestInit_NoHomeDir(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })

	orig := UserHomeDirFn
	UserHomeDirFn = func() (string, error) { return "", fmt.Errorf("no home dir") }
	t.Cleanup(func() { UserHomeDirFn = orig })

	Init()
	if !state.disabled {
		t.Error("expected disabled when home dir unavailable")
	}
}

func TestInit_UnwritableConfigDir_DisabledForSession(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })

	home := t.TempDir()
	overrideHome(t, home)

	// Create a read-only .syllago directory so saves fail.
	dir := filepath.Join(home, ".syllago")
	os.Mkdir(dir, 0755)
	os.Chmod(dir, 0555) // read-only
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	if !state.disabled {
		t.Error("expected disabled when config dir is not writable")
	}
}

func TestSetVersion(t *testing.T) {
	orig := sysBuildVersion
	t.Cleanup(func() { sysBuildVersion = orig })
	SetVersion("1.2.3")
	if sysBuildVersion != "1.2.3" {
		t.Errorf("expected 1.2.3, got %s", sysBuildVersion)
	}
}

func TestStatus_WithConfig(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	_ = SetEnabled(true)
	cfg := Status()
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if !isValidID(cfg.AnonymousID) {
		t.Errorf("invalid anonymous ID: %s", cfg.AnonymousID)
	}
}

func TestStatus_MissingConfig(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	cfg := Status()
	if cfg.Enabled {
		t.Error("expected disabled status for missing config")
	}
}
