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

// NoticeWriter is where the first-run notice is written. Defaults to os.Stderr.
// Override in tests to capture notice output.
var NoticeWriter io.Writer = os.Stderr

// Init initializes the telemetry subsystem. Must be called once per process,
// early in the command lifecycle (PersistentPreRun).
//
// Order of operations:
//  1. If apiKey is empty (dev build), return immediately.
//  2. If DO_NOT_TRACK is truthy, return immediately.
//  3. If /etc/syllago/telemetry.json has enabled:false, return immediately.
//  4. Attempt to load user config. If unreadable/unwritable, disable for session.
//  5. If config missing, create with defaults (enabled:true, new ID, noticeSeen:false).
//  6. If noticeSeen:false, print first-run notice to NoticeWriter, set noticeSeen:true.
//  7. If enabled:false, return without initializing HTTP client.
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

	// Step 6 — first-run notice.
	if !cfg.NoticeSeen {
		fmt.Fprint(NoticeWriter, firstRunNotice)
		cfg.NoticeSeen = true
		if err := saveUserConfig(cfg); err != nil {
			state.disabled = true
			return
		}
	} else if created {
		if err := saveUserConfig(cfg); err != nil {
			state.disabled = true
			return
		}
	}

	// Step 7 — check enabled flag.
	if !cfg.Enabled {
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

// firstRunNotice is the exact first-run notice text printed to stderr.
const firstRunNotice = `
syllago collects anonymous usage data (commands run, provider
types, error codes) to help prioritize development. Syllago is
a solo-developer project and this data is invaluable for
steering its direction.

No file contents, paths, or identifying information is collected.

  Disable:  syllago telemetry off
  Env var:  DO_NOT_TRACK=1
  Details:  https://syllago.dev/telemetry

`

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

// SetEnabled sets the enabled flag in the user config and saves it.
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
