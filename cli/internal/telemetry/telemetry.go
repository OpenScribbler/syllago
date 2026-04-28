package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// apiKey is embedded at build time via ldflags:
//
//	-X github.com/OpenScribbler/syllago/cli/internal/telemetry.apiKey=phc_...
//
// When empty (dev builds), Init() returns immediately and no events are sent.
var apiKey string

// defaultEndpoint is the PostHog ingest endpoint.
const defaultEndpoint = "https://us.i.posthog.com/capture/"

// postHogPayload is the JSON body sent to the PostHog ingest API.
type postHogPayload struct {
	APIKey     string         `json:"api_key"`
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
}

// state holds all runtime telemetry state. Initialized by Init().
var state struct {
	mu          sync.Mutex
	disabled    bool
	anonymousID string
	endpoint    string
	client      *http.Client
	wg          sync.WaitGroup
}

// sysBuildVersion is set by the root command before Init() is called.
var sysBuildVersion string

// SetVersion sets the version string for event properties.
func SetVersion(v string) {
	sysBuildVersion = v
}

// NoticeWriter is retained for back-compat with tests and command-line code
// that may still write status messages to stderr. The opt-out-era first-run
// notice is no longer printed; consent is collected via the consent modal
// (CLI prompt + TUI overlay). See cli/internal/telemetry/prompt.go and
// cli/internal/tui/telemetry_consent.go.
var NoticeWriter io.Writer = os.Stderr

// enrichedProps stores command-specific properties set via Enrich().
// These are merged into the next TrackCommand() call, then cleared.
var enrichedProps = map[string]any{}
var enrichMu sync.Mutex

// Enrich adds a property that will be included in the next TrackCommand() call.
// Call from command RunE functions to add context like provider, content_type, etc.
func Enrich(key string, value any) {
	enrichMu.Lock()
	defer enrichMu.Unlock()
	enrichedProps[key] = value
}

// TrackCommand fires a command_executed event with the given command name and
// any properties set via Enrich(). Called from PersistentPostRun so every
// command is tracked automatically. Enriched properties are cleared after use.
func TrackCommand(command string) {
	enrichMu.Lock()
	props := make(map[string]any, len(enrichedProps)+1)
	props["command"] = command
	for k, v := range enrichedProps {
		props[k] = v
	}
	enrichedProps = map[string]any{}
	enrichMu.Unlock()

	Track("command_executed", props)
}

// ResetEnrichment clears any pending enriched properties without firing an event.
// Used in tests.
func ResetEnrichment() {
	enrichMu.Lock()
	enrichedProps = map[string]any{}
	enrichMu.Unlock()
}

// Init initializes the telemetry subsystem. Must be called once per process,
// early in the command lifecycle (PersistentPreRun).
//
// Telemetry is opt-in: events fire only when the user has explicitly recorded
// consent (Config.ConsentRecorded == true) AND chosen to enable
// (Config.Enabled == true). Init never prompts; the consent modal is invoked
// separately by the CLI (cmd/syllago) and TUI (internal/tui).
//
// Order of operations:
//  1. If apiKey is empty (dev build), return immediately.
//  2. If DO_NOT_TRACK is truthy, return immediately.
//  3. If /etc/syllago/telemetry.json has enabled:false, return immediately.
//  4. Attempt to load user config. If unreadable/unwritable, disable for session.
//  5. If config missing, create with safe defaults (Enabled=false, ConsentRecorded=false).
//  6. Migrate legacy opt-out-era configs in memory and on disk.
//  7. If user has not recorded consent OR has disabled telemetry, return without firing.
//  8. Initialize HTTP client.
func Init() {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Step 1 — dev build guard.
	if apiKey == "" {
		state.disabled = true
		return
	}

	// Step 2 — DO_NOT_TRACK.
	if isDNTSet() {
		state.disabled = true
		return
	}

	// Step 3 — system-level config.
	sc, err := loadSysConfig()
	if err == nil && sc != nil && !sc.Enabled {
		state.disabled = true
		return
	}

	// Step 4 — user config.
	cfg, err := loadUserConfig()
	if err != nil {
		state.disabled = true
		return
	}

	// Step 5 — create config if missing.
	created := false
	if cfg == nil {
		cfg, err = newConfig()
		if err != nil {
			state.disabled = true
			return
		}
		created = true
	}

	// Step 6 — migrate legacy opt-out-era configs (only when loaded from disk).
	// Anyone who installed before the opt-in switch has Enabled=true with no
	// recorded consent; force them off until they explicitly re-opt-in. Users
	// who had previously run `syllago telemetry off` are preserved as opted-out
	// without re-prompt. Fresh configs are left as-is so the consent modal
	// appears on first interactive launch.
	migrated := false
	if !created {
		migrated = migrateConfig(cfg)
	}
	if created || migrated {
		if err := saveUserConfig(cfg); err != nil {
			state.disabled = true
			return
		}
	}

	// Step 7 — must have explicit consent AND be enabled.
	if !cfg.ConsentRecorded || !cfg.Enabled {
		state.disabled = true
		return
	}

	// Step 8 — initialize HTTP client.
	state.disabled = false
	state.anonymousID = cfg.AnonymousID
	state.endpoint = cfg.Endpoint
	if state.endpoint == "" {
		state.endpoint = defaultEndpoint
	}
	state.client = &http.Client{Timeout: 2 * time.Second}
}

// migrateConfig rewrites legacy opt-out-era configs in place. Returns true if
// the config changed and needs to be saved. Designed to run only against
// configs loaded from disk — fresh configs created by newConfig() should not
// be passed in, since their (Enabled=false, ConsentRecorded=false) state is
// the same shape as a legacy explicit-off without the NoticeSeen marker.
//
// Migration rules:
//
//   - Already has ConsentRecorded=true: nothing to do.
//   - Has Enabled=true with no recorded consent: force Enabled=false and
//     clear NoticeSeen. The consent modal will run on the next interactive
//     launch and let the user make a real, informed choice.
//   - Has NoticeSeen=true with Enabled=false: user previously ran
//     `syllago telemetry off`. Preserve the opted-out state and mark
//     consent as recorded so we don't re-prompt them. Clear NoticeSeen
//     so migration is idempotent across loads.
//
// NoticeSeen is cleared in both branches so a second migration pass (e.g.
// from NeedsConsent reloading the persisted post-migration config) does
// not re-fire the explicit-off branch.
func migrateConfig(cfg *Config) bool {
	if cfg.ConsentRecorded {
		return false
	}
	if cfg.Enabled {
		cfg.Enabled = false
		cfg.NoticeSeen = false
		return true
	}
	if cfg.NoticeSeen {
		cfg.ConsentRecorded = true
		cfg.NoticeSeen = false
		return true
	}
	return false
}

// NeedsConsent reports whether the user has not yet recorded an explicit
// consent decision. Callers (CLI prompt, TUI modal) use this to decide
// whether to show the consent UI before any work runs.
//
// Returns false in any condition where the consent modal should not appear:
// dev builds (no API key), DO_NOT_TRACK set, system-level disable, missing
// home dir, or unreadable config.
func NeedsConsent() bool {
	if apiKey == "" {
		return false
	}
	if isDNTSet() {
		return false
	}
	if sc, err := loadSysConfig(); err == nil && sc != nil && !sc.Enabled {
		return false
	}
	cfg, err := loadUserConfig()
	if err != nil {
		return false
	}
	if cfg == nil {
		return true
	}
	// Apply migration so callers see post-migration state.
	migrateConfig(cfg)
	return !cfg.ConsentRecorded
}

// RecordConsent persists the user's explicit choice. Sets both Enabled and
// ConsentRecorded so that future runs respect the decision and never re-prompt.
// Call this from the consent UI (CLI prompt or TUI modal) once the user has
// answered Yes or No.
func RecordConsent(enabled bool) error {
	cfg, err := loadUserConfig()
	if err != nil {
		return fmt.Errorf("loading telemetry config: %w", err)
	}
	if cfg == nil {
		cfg, err = newConfig()
		if err != nil {
			return err
		}
	}
	cfg.Enabled = enabled
	cfg.ConsentRecorded = true
	return saveUserConfig(cfg)
}

// Track fires an event to PostHog asynchronously. Returns immediately — the POST
// runs in a goroutine. Call Shutdown() to wait for it to complete.
// If telemetry is disabled, Track() is a no-op.
func Track(event string, properties map[string]any) {
	state.mu.Lock()
	disabled := state.disabled
	id := state.anonymousID
	endpoint := state.endpoint
	client := state.client
	state.mu.Unlock()

	if disabled || client == nil {
		return
	}

	// Merge standard properties.
	props := map[string]any{
		"version": sysBuildVersion,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
	for k, v := range properties {
		props[k] = v
	}

	payload := postHogPayload{
		APIKey:     apiKey,
		Event:      event,
		DistinctID: id,
		Properties: props,
	}

	state.wg.Add(1)
	go func() {
		defer state.wg.Done()
		sendEvent(client, endpoint, payload)
	}()
}

// Shutdown waits up to 2 seconds for any in-flight Track goroutines to complete.
func Shutdown() {
	done := make(chan struct{})
	go func() {
		state.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

// sendEvent performs the HTTP POST to the PostHog ingest endpoint.
// All errors are silently discarded — telemetry failure is never user-visible.
func sendEvent(client *http.Client, endpoint string, payload postHogPayload) {
	data, err := json.Marshal(payload) //nolint:gosec // G117: api_key is a public PostHog project key, not a secret
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close() //nolint:errcheck // telemetry errors are intentionally silent
	_, _ = io.Copy(io.Discard, resp.Body)
}

// Reset generates a new anonymous ID, saves it, and returns the new ID.
func Reset() (string, error) {
	cfg, err := loadUserConfig()
	if err != nil {
		return "", fmt.Errorf("loading telemetry config: %w", err)
	}
	if cfg == nil {
		cfg, err = newConfig()
		if err != nil {
			return "", err
		}
	}
	newID, err := generateID()
	if err != nil {
		return "", fmt.Errorf("generating new ID: %w", err)
	}
	cfg.AnonymousID = newID
	if err := saveUserConfig(cfg); err != nil {
		return "", fmt.Errorf("saving telemetry config: %w", err)
	}
	return newID, nil
}

// SetEnabled sets the enabled flag in the user config and saves it. Running
// `syllago telemetry on` or `syllago telemetry off` is itself an explicit
// consent decision, so we also mark ConsentRecorded=true to suppress any
// future consent prompt.
func SetEnabled(enabled bool) error {
	cfg, err := loadUserConfig()
	if err != nil {
		return fmt.Errorf("loading telemetry config: %w", err)
	}
	if cfg == nil {
		cfg, err = newConfig()
		if err != nil {
			return err
		}
	}
	cfg.Enabled = enabled
	cfg.ConsentRecorded = true
	return saveUserConfig(cfg)
}

// Status returns a snapshot of the current telemetry config for display.
// Returns a zero-value Config with Enabled=false if the config is missing or unreadable.
func Status() Config {
	cfg, err := loadUserConfig()
	if err != nil || cfg == nil {
		return Config{Enabled: false}
	}
	return *cfg
}
