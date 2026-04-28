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

func TestInit_FirstRun_DisabledNoNotice(t *testing.T) {
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()

	// Opt-in: a fresh install must not auto-enable telemetry and must not
	// print any inline notice. The consent UI (CLI prompt + TUI modal) is
	// responsible for any disclosure.
	if !state.disabled {
		t.Error("expected telemetry disabled on first run when consent has not been recorded")
	}
	if notice.Len() != 0 {
		t.Errorf("Init must not write any inline notice on first run; got: %q", notice.String())
	}
	if !NeedsConsent() {
		t.Error("NeedsConsent should report true on first run")
	}
}

func TestInit_LegacyOptOut_PreservedNoPrompt(t *testing.T) {
	// Users who previously ran `syllago telemetry off` end up with
	// Enabled=false and ConsentRecorded=false (legacy schema). Migration
	// must preserve the opted-out state without re-prompting them.
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	body := `{"enabled":false,"anonymousId":"syl_legacyoff","noticeSeen":true,"createdAt":"2026-04-02T00:00:00Z"}`
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte(body), 0644)

	Init()

	if !state.disabled {
		t.Error("legacy opted-out user must remain disabled")
	}
	cfg, _ := loadUserConfig()
	if cfg == nil || !cfg.ConsentRecorded {
		t.Error("legacy opted-out user should have ConsentRecorded=true after migration")
	}
	if cfg != nil && cfg.Enabled {
		t.Error("legacy opted-out user must remain Enabled=false after migration")
	}
	if NeedsConsent() {
		t.Error("legacy opted-out user must not be re-prompted")
	}
}

func TestInit_LegacySilentOptIn_ForcedOff(t *testing.T) {
	// Users on the previous opt-out scheme had Enabled=true with no real
	// recorded consent. Migration must force them off and leave consent
	// unrecorded so the modal appears on the next interactive launch.
	origKey := apiKey
	apiKey = "phc_test"
	t.Cleanup(func() { apiKey = origKey; resetState() })
	home := t.TempDir()
	overrideHome(t, home)

	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	body := `{"enabled":true,"anonymousId":"syl_legacyon","noticeSeen":true,"createdAt":"2026-04-02T00:00:00Z"}`
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte(body), 0644)

	Init()

	if !state.disabled {
		t.Error("legacy opt-out-era user must be force-disabled until they re-opt-in")
	}
	cfg, _ := loadUserConfig()
	if cfg == nil {
		t.Fatal("config should still exist after migration")
	}
	if cfg.Enabled {
		t.Error("Enabled must be forced to false during migration")
	}
	if cfg.ConsentRecorded {
		t.Error("ConsentRecorded must remain false so the modal re-prompts")
	}
	if !NeedsConsent() {
		t.Error("NeedsConsent must be true so the consent modal appears")
	}
}

// TestMigrateConfig_TableDriven exercises every documented migration branch
// directly against the migrateConfig function, without going through Init().
// This is the regression layer for the "ambiguity after migration" bug fixed
// alongside the opt-in switchover: the silent-opt-in case (Enabled=true,
// NoticeSeen=true) and the explicit-off case (Enabled=false, NoticeSeen=true)
// share the same shape post-migration if NoticeSeen isn't cleared, which let
// a second migration call mis-classify them.
func TestMigrateConfig_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         Config
		wantChange bool
		wantOut    Config
	}{
		{
			name: "fresh_install_untouched",
			// newConfig() shape: ConsentRecorded=false, Enabled=false,
			// NoticeSeen=false. Must NOT migrate — the consent modal will
			// appear naturally on next launch.
			in:         Config{Enabled: false, ConsentRecorded: false, NoticeSeen: false},
			wantChange: false,
			wantOut:    Config{Enabled: false, ConsentRecorded: false, NoticeSeen: false},
		},
		{
			name: "already_consented_yes",
			// Modern config with explicit yes — never re-touched.
			in:         Config{Enabled: true, ConsentRecorded: true, NoticeSeen: false},
			wantChange: false,
			wantOut:    Config{Enabled: true, ConsentRecorded: true, NoticeSeen: false},
		},
		{
			name: "already_consented_no",
			// Modern config with explicit no — never re-touched.
			in:         Config{Enabled: false, ConsentRecorded: true, NoticeSeen: false},
			wantChange: false,
			wantOut:    Config{Enabled: false, ConsentRecorded: true, NoticeSeen: false},
		},
		{
			name: "legacy_silent_opt_in_forced_off",
			// Opt-out era default: Enabled=true, NoticeSeen=true, no consent.
			// Force off and clear NoticeSeen so a second migration pass is
			// a no-op. ConsentRecorded stays false → consent modal shows.
			in:         Config{Enabled: true, ConsentRecorded: false, NoticeSeen: true},
			wantChange: true,
			wantOut:    Config{Enabled: false, ConsentRecorded: false, NoticeSeen: false},
		},
		{
			name: "legacy_explicit_opt_out_preserved",
			// User previously ran `syllago telemetry off`: Enabled=false,
			// NoticeSeen=true, no consent. Preserve the off state, mark
			// consent recorded so they aren't re-prompted, clear NoticeSeen.
			in:         Config{Enabled: false, ConsentRecorded: false, NoticeSeen: true},
			wantChange: true,
			wantOut:    Config{Enabled: false, ConsentRecorded: true, NoticeSeen: false},
		},
		{
			name: "legacy_silent_opt_in_with_id_preserved",
			// Anonymous ID and timestamp are out-of-scope for migration —
			// only the three flags should change. Verifies migrate doesn't
			// stomp unrelated fields.
			in: Config{
				Enabled: true, ConsentRecorded: false, NoticeSeen: true,
				AnonymousID: "syl_legacyABC", Endpoint: "https://custom/",
			},
			wantChange: true,
			wantOut: Config{
				Enabled: false, ConsentRecorded: false, NoticeSeen: false,
				AnonymousID: "syl_legacyABC", Endpoint: "https://custom/",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.in
			gotChange := migrateConfig(&cfg)
			if gotChange != tt.wantChange {
				t.Errorf("migrateConfig changed=%v, want %v", gotChange, tt.wantChange)
			}
			if cfg.Enabled != tt.wantOut.Enabled {
				t.Errorf("Enabled=%v, want %v", cfg.Enabled, tt.wantOut.Enabled)
			}
			if cfg.ConsentRecorded != tt.wantOut.ConsentRecorded {
				t.Errorf("ConsentRecorded=%v, want %v", cfg.ConsentRecorded, tt.wantOut.ConsentRecorded)
			}
			if cfg.NoticeSeen != tt.wantOut.NoticeSeen {
				t.Errorf("NoticeSeen=%v, want %v", cfg.NoticeSeen, tt.wantOut.NoticeSeen)
			}
			if cfg.AnonymousID != tt.wantOut.AnonymousID {
				t.Errorf("AnonymousID=%q, want %q", cfg.AnonymousID, tt.wantOut.AnonymousID)
			}
			if cfg.Endpoint != tt.wantOut.Endpoint {
				t.Errorf("Endpoint=%q, want %q", cfg.Endpoint, tt.wantOut.Endpoint)
			}
		})
	}
}

// TestMigrateConfig_Idempotent is the regression test for the bug fixed
// during the opt-in conversion: running migrateConfig twice must produce the
// same result as running it once. The original implementation flipped
// silent-opt-in (Enabled=true, NoticeSeen=true) to (Enabled=false,
// NoticeSeen=true) — but that output shape matched the legacy explicit-off
// case, so a second call would silently flip ConsentRecorded to true and
// suppress the consent modal that the migration was supposed to trigger.
func TestMigrateConfig_Idempotent(t *testing.T) {
	t.Parallel()
	startStates := []struct {
		name string
		cfg  Config
	}{
		{"fresh_install", Config{}},
		{"legacy_silent_opt_in", Config{Enabled: true, NoticeSeen: true}},
		{"legacy_explicit_off", Config{Enabled: false, NoticeSeen: true}},
		{"already_consented_yes", Config{Enabled: true, ConsentRecorded: true}},
		{"already_consented_no", Config{Enabled: false, ConsentRecorded: true}},
	}
	for _, s := range startStates {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			cfg := s.cfg
			migrateConfig(&cfg)
			afterFirst := cfg

			changed := migrateConfig(&cfg)
			if changed {
				t.Errorf("second migrateConfig call must be a no-op, but reported changed=true")
			}
			if cfg != afterFirst {
				t.Errorf("second call mutated config:\n  after first:  %+v\n  after second: %+v", afterFirst, cfg)
			}
		})
	}
}

func TestRecordConsent_Yes(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	if err := RecordConsent(true); err != nil {
		t.Fatalf("RecordConsent(true): %v", err)
	}
	cfg, _ := loadUserConfig()
	if cfg == nil || !cfg.Enabled || !cfg.ConsentRecorded {
		t.Errorf("expected enabled=true, consentRecorded=true; got %+v", cfg)
	}
	if NeedsConsent() {
		t.Error("NeedsConsent must be false after RecordConsent")
	}
}

func TestRecordConsent_No(t *testing.T) {
	home := t.TempDir()
	overrideHome(t, home)

	if err := RecordConsent(false); err != nil {
		t.Fatalf("RecordConsent(false): %v", err)
	}
	cfg, _ := loadUserConfig()
	if cfg == nil || cfg.Enabled || !cfg.ConsentRecorded {
		t.Errorf("expected enabled=false, consentRecorded=true; got %+v", cfg)
	}
	if NeedsConsent() {
		t.Error("a recorded No is still a recorded decision")
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

	// Simulate explicit user opt-in so Init activates the HTTP path.
	if err := RecordConsent(true); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}

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

	if err := RecordConsent(true); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}

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

// TestEndToEnd_FullLifecycle exercises the complete opt-in telemetry
// lifecycle: fresh install (disabled, awaiting consent) → user opts in →
// events sent → second run (still on, no prompt) → off → on → reset.
// Uses httptest to verify events arrive with the correct payloads.
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

	// --- Phase 1: First run (fresh install) — must stay disabled until consent ---
	Init()
	if !state.disabled {
		t.Fatal("fresh install must leave telemetry disabled until the user opts in")
	}
	if !NeedsConsent() {
		t.Error("NeedsConsent must report true on first run")
	}

	// Track on a fresh install must be a no-op even with the test server up.
	state.mu.Lock()
	state.endpoint = srv.URL
	state.mu.Unlock()
	Track("should_not_send_pre_consent", nil)
	Shutdown()
	mu.Lock()
	if len(received) != 0 {
		t.Errorf("no events should fire pre-consent; got %d", len(received))
	}
	mu.Unlock()

	// --- Phase 2: User opts in via the consent UI ---
	if err := RecordConsent(true); err != nil {
		t.Fatalf("RecordConsent(true): %v", err)
	}
	cfgPath := filepath.Join(home, ".syllago", "telemetry.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	resetState()
	Init()
	if state.disabled {
		t.Fatal("telemetry should be enabled after RecordConsent(true)")
	}
	if !isValidID(state.anonymousID) {
		t.Errorf("invalid anonymous ID: %s", state.anonymousID)
	}
	firstID := state.anonymousID

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

	// --- Phase 3: Second run — consent already recorded, no re-prompt ---
	resetState()
	Init()
	if state.disabled {
		t.Error("telemetry should still be enabled on second run after a recorded yes")
	}
	if NeedsConsent() {
		t.Error("NeedsConsent must return false once consent has been recorded")
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

	// Simulate explicit user opt-in so Init activates the HTTP path. Under
	// the new opt-in semantics, Init leaves state.disabled=true unless the
	// config has both Enabled=true and ConsentRecorded=true.
	if err := RecordConsent(true); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}

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
