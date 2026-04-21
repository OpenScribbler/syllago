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
	if !strings.Contains(notice.String(), "To opt out") {
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

	count := strings.Count(notice.String(), "To opt out")
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

// TestEndToEnd_FullLifecycle exercises the complete telemetry lifecycle as a
// user would experience it: fresh install → first-run notice → events sent →
// second run (no notice) → off → on → reset. Uses httptest to verify events
// actually arrive at the ingest endpoint with correct payloads.
func TestEndToEnd_FullLifecycle(t *testing.T) {
	// --- Setup: capture HTTP events ---
	var received []postHogPayload
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		var p postHogPayload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Errorf("invalid JSON payload: %v", err)
		}
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	origKey := apiKey
	apiKey = "phc_e2e_test"
	origVersion := sysBuildVersion
	sysBuildVersion = "1.0.0-test"
	t.Cleanup(func() {
		apiKey = origKey
		sysBuildVersion = origVersion
		resetState()
	})
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	// --- Phase 1: First run (fresh install) ---
	Init()

	// Config file should exist now.
	cfgPath := filepath.Join(home, ".syllago", "telemetry.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// First-run notice should have been written.
	if !strings.Contains(notice.String(), "To opt out") {
		t.Errorf("first-run notice missing; got: %q", notice.String())
	}

	// State should be active with a valid anonymous ID.
	if state.disabled {
		t.Fatal("telemetry should be enabled after first Init()")
	}
	if !isValidID(state.anonymousID) {
		t.Errorf("invalid anonymous ID after Init(): %s", state.anonymousID)
	}
	firstID := state.anonymousID

	// Override endpoint to our test server.
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	// --- Phase 2: Send events ---
	Track("command_executed", map[string]any{
		"command":  "install",
		"provider": "claude-code",
		"success":  true,
	})
	Track("command_executed", map[string]any{
		"command": "convert",
		"success": true,
	})
	Shutdown()

	// Verify both events arrived.
	mu.Lock()
	eventCount := len(received)
	mu.Unlock()
	if eventCount != 2 {
		t.Fatalf("expected 2 events, got %d", eventCount)
	}

	mu.Lock()
	for i, ev := range received {
		if ev.APIKey != "phc_e2e_test" {
			t.Errorf("event %d: wrong API key %q", i, ev.APIKey)
		}
		if ev.DistinctID != firstID {
			t.Errorf("event %d: wrong distinct_id %q, want %q", i, ev.DistinctID, firstID)
		}
		if ev.Properties["version"] != "1.0.0-test" {
			t.Errorf("event %d: missing or wrong version property", i)
		}
		if ev.Properties["os"] == nil {
			t.Errorf("event %d: missing os property", i)
		}
		if ev.Properties["arch"] == nil {
			t.Errorf("event %d: missing arch property", i)
		}
	}
	// Find the install event (order is non-deterministic due to goroutines).
	foundInstall := false
	for _, ev := range received {
		if ev.Event == "command_executed" && ev.Properties["command"] == "install" {
			foundInstall = true
			if ev.Properties["provider"] != "claude-code" {
				t.Errorf("install event: wrong provider %v", ev.Properties["provider"])
			}
		}
	}
	if !foundInstall {
		t.Error("install event not found in received events")
	}
	mu.Unlock()

	// --- Phase 3: Second run (no notice) ---
	resetState()
	notice.Reset()

	Init()
	if notice.Len() > 0 {
		t.Errorf("notice should not appear on second run; got: %q", notice.String())
	}
	if state.disabled {
		t.Error("telemetry should still be enabled on second run")
	}
	Shutdown()

	// --- Phase 4: Disable → re-enable → verify ---
	resetState()
	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}

	cfg := Status()
	if cfg.Enabled {
		t.Error("expected disabled after SetEnabled(false)")
	}

	// Init with telemetry disabled should not create an HTTP client.
	Init()
	if !state.disabled {
		t.Error("Init() should respect enabled=false in config")
	}

	// Track should be a no-op when disabled.
	mu.Lock()
	prevCount := len(received)
	mu.Unlock()

	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	Track("should_not_send", nil)
	Shutdown()

	mu.Lock()
	if len(received) != prevCount {
		t.Errorf("event sent while disabled: got %d events, want %d", len(received), prevCount)
	}
	mu.Unlock()

	// Re-enable.
	resetState()
	if err := SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}

	// --- Phase 5: Reset ID ---
	newID, err := Reset()
	if err != nil {
		t.Fatalf("Reset(): %v", err)
	}
	if newID == firstID {
		t.Error("Reset() should generate a different ID")
	}
	if !isValidID(newID) {
		t.Errorf("Reset() returned invalid ID: %s", newID)
	}

	// Verify the new ID is used for subsequent events.
	resetState()
	Init()
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	Track("command_executed", map[string]any{"command": "version"})
	Shutdown()

	mu.Lock()
	lastEvent := received[len(received)-1]
	mu.Unlock()
	if lastEvent.DistinctID != newID {
		t.Errorf("event after Reset() used old ID %q, want %q", lastEvent.DistinctID, newID)
	}
}

// setupEnrichTestServer spins up an httptest server that records every
// PostHog payload it receives, plus enables telemetry with a valid apiKey,
// creates a temp home dir, and points the endpoint at the server. It
// returns the received-events slice + its mutex so tests can assert on the
// live payloads. All cleanup (reset, etc.) is registered via t.Cleanup.
func setupEnrichTestServer(t *testing.T) (*[]postHogPayload, *sync.Mutex) {
	t.Helper()
	var received []postHogPayload
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p postHogPayload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Errorf("invalid JSON payload: %v", err)
		}
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	origKey := apiKey
	apiKey = "phc_enrich_test"
	t.Cleanup(func() {
		apiKey = origKey
		resetState()
		ResetEnrichment()
	})

	overrideHome(t, t.TempDir())
	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()

	return &received, &mu
}

// TestEnrich_PropertiesReachPayload is the integration contract for Enrich +
// TrackCommand: every property set via Enrich() must appear in the outgoing
// PostHog event payload with its exact value, alongside the "command" key
// TrackCommand injects. Without this test, a regression that silently dropped
// enrichedProps during merge would slip through the drift-detection test
// (which only checks catalog vs. Enrich() call sites, not the payload path).
func TestEnrich_PropertiesReachPayload(t *testing.T) {
	received, mu := setupEnrichTestServer(t)

	Enrich("provider", "claude-code")
	Enrich("content_type", "rules")
	Enrich("content_count", 3)
	Enrich("dry_run", true)
	TrackCommand("install")
	Shutdown()

	mu.Lock()
	defer mu.Unlock()
	if len(*received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*received))
	}
	ev := (*received)[0]

	if ev.Event != "command_executed" {
		t.Errorf("event name: got %q, want %q", ev.Event, "command_executed")
	}
	want := map[string]any{
		"command":       "install",
		"provider":      "claude-code",
		"content_type":  "rules",
		"content_count": float64(3), // JSON numbers decode to float64
		"dry_run":       true,
	}
	for k, v := range want {
		got, ok := ev.Properties[k]
		if !ok {
			t.Errorf("payload missing property %q (got properties: %v)", k, ev.Properties)
			continue
		}
		if got != v {
			t.Errorf("property %q: got %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}
}

// TestEnrich_ScopeIsolation pins the critical invariant that enriched
// properties DO NOT leak between commands. When TrackCommand fires, it
// must clear enrichedProps so the next command only sees its own context.
// Without this, cross-command telemetry would include stale state that's
// both misleading for analysis and a potential privacy leak.
func TestEnrich_ScopeIsolation(t *testing.T) {
	received, mu := setupEnrichTestServer(t)

	// Command 1: enriched with provider=claude-code.
	Enrich("provider", "claude-code")
	Enrich("content_count", 5)
	TrackCommand("install")

	// Command 2: only enriches content_type. Previous provider/content_count
	// must be absent — TrackCommand #1 must have cleared enrichedProps.
	Enrich("content_type", "loadouts")
	TrackCommand("list")

	Shutdown()

	mu.Lock()
	defer mu.Unlock()
	if len(*received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(*received))
	}

	// Find each event by command name (goroutine delivery order isn't guaranteed).
	var installEv, listEv *postHogPayload
	for i := range *received {
		switch (*received)[i].Properties["command"] {
		case "install":
			installEv = &(*received)[i]
		case "list":
			listEv = &(*received)[i]
		}
	}
	if installEv == nil || listEv == nil {
		t.Fatalf("did not find both events; received=%+v", *received)
	}

	// Install event must have its own enriched props.
	if installEv.Properties["provider"] != "claude-code" {
		t.Errorf("install event missing provider=claude-code: %v", installEv.Properties)
	}
	if installEv.Properties["content_count"] != float64(5) {
		t.Errorf("install event missing content_count=5: %v", installEv.Properties)
	}

	// List event must NOT have install's enriched props.
	if _, leaked := listEv.Properties["provider"]; leaked {
		t.Errorf("SCOPE LEAK: list event carries provider from install event: %v", listEv.Properties)
	}
	if _, leaked := listEv.Properties["content_count"]; leaked {
		t.Errorf("SCOPE LEAK: list event carries content_count from install event: %v", listEv.Properties)
	}
	// List event must have its own content_type.
	if listEv.Properties["content_type"] != "loadouts" {
		t.Errorf("list event missing content_type=loadouts: %v", listEv.Properties)
	}
}

// TestResetEnrichment_ClearsPending pins that ResetEnrichment() drops any
// pending enriched props without firing an event. Used in tests and in
// early-exit command paths.
func TestResetEnrichment_ClearsPending(t *testing.T) {
	received, mu := setupEnrichTestServer(t)

	Enrich("provider", "should-be-dropped")
	Enrich("content_count", 99)
	ResetEnrichment()

	// Now fire a command with no enrichment — event must carry only "command".
	TrackCommand("version")
	Shutdown()

	mu.Lock()
	defer mu.Unlock()
	if len(*received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*received))
	}
	ev := (*received)[0]
	if _, leaked := ev.Properties["provider"]; leaked {
		t.Errorf("ResetEnrichment did not clear provider; got: %v", ev.Properties)
	}
	if _, leaked := ev.Properties["content_count"]; leaked {
		t.Errorf("ResetEnrichment did not clear content_count; got: %v", ev.Properties)
	}
	if ev.Properties["command"] != "version" {
		t.Errorf("expected command=version, got: %v", ev.Properties["command"])
	}
}
