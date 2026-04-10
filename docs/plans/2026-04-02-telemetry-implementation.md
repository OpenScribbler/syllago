# PostHog Telemetry — Implementation Plan

**Design doc:** `docs/plans/2026-04-02-telemetry-design.md`
**Feature branch:** `feat/telemetry`

---

## Overview

This plan implements anonymous opt-out usage telemetry via PostHog. The work spans a new
`cli/internal/telemetry/` package, a `syllago telemetry` subcommand, root command integration
hooks, a first-run notice on both the CLI and TUI paths, Track() call sites in four commands,
build system changes to embed the API key, and a docs site telemetry page.

Tasks are ordered so each depends only on what came before. Each task is 2–5 minutes of
mechanical work once reading is done.

---

## Task 1 — Telemetry config types and read/write

**Files:** `cli/internal/telemetry/config.go`, `cli/internal/telemetry/config_test.go`
**Depends on:** nothing
**Why first:** Everything else reads or writes the config. Establishing the types and IO
primitives with tests before layering logic on top prevents rework.

### TDD Approach

Write tests first:
1. Create `config_test.go` with the test code below
2. Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/telemetry/ -run TestLoad -v`
3. Tests fail (package doesn't exist yet)
4. Create `config.go` and implement to make tests pass

### Implementation

Create `cli/internal/telemetry/config.go`:

```go
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	configFileName    = "telemetry.json"
	sysDirPath        = "/etc/syllago"
	sysConfigFileName = "telemetry.json"
)

// Config is the user-level telemetry config stored at ~/.syllago/telemetry.json.
type Config struct {
	Enabled     bool      `json:"enabled"`
	AnonymousID string    `json:"anonymousId"`
	NoticeSeen  bool      `json:"noticeSeen"`
	Endpoint    string    `json:"endpoint,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SysConfig is the system-level config at /etc/syllago/telemetry.json.
// Only the Enabled field is honoured; all other settings come from the user config.
type SysConfig struct {
	Enabled bool `json:"enabled"`
}

// UserHomeDirFn is the home directory resolver. Exported so tests in other packages
// (e.g., cmd tests) can override it without build constraints.
// Override in tests: telemetry.UserHomeDirFn = func() (string, error) { return tmpDir, nil }
var UserHomeDirFn = os.UserHomeDir

// userConfigPath returns ~/.syllago/telemetry.json, or an error if home is unknown.
// Note: /etc/syllago/telemetry.json (sysConfigPath) is Unix/Linux only.
// On Windows, system-level config is not supported; only user ~/.syllago is used.
// This is acceptable for initial release.
func userConfigPath() (string, error) {
	home, err := UserHomeDirFn()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".syllago", configFileName), nil
}

// sysConfigPath returns /etc/syllago/telemetry.json.
func sysConfigPath() string {
	return filepath.Join(sysDirPath, sysConfigFileName)
}

// loadUserConfig reads ~/.syllago/telemetry.json.
// Returns (nil, nil) if the file does not exist.
// Returns (nil, err) if the file exists but is unreadable or malformed — callers
// must treat this as telemetry disabled.
func loadUserConfig() (*Config, error) {
	path, err := userConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading telemetry config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing telemetry config: %w", err)
	}
	return &cfg, nil
}

// saveUserConfig writes cfg to ~/.syllago/telemetry.json atomically.
// Returns an error if the directory cannot be created or the write fails.
func saveUserConfig(cfg *Config) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	// Verify writability by checking directory permissions before attempting write.
	if err := checkWritable(filepath.Dir(path)); err != nil {
		return fmt.Errorf("config dir not writable: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tmp := path + ".tmp." + hex.EncodeToString(suffix)
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}

// loadSysConfig reads /etc/syllago/telemetry.json.
// Returns (nil, nil) if the file does not exist — absence means no system override.
func loadSysConfig() (*SysConfig, error) {
	data, err := os.ReadFile(sysConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading system telemetry config: %w", err)
	}
	var sc SysConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parsing system telemetry config: %w", err)
	}
	return &sc, nil
}

// checkWritable returns nil if the given directory is writable by the current process.
func checkWritable(dir string) error {
	probe := filepath.Join(dir, ".write-probe-"+hex.EncodeToString([]byte{0, 1, 2, 3}))
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	f.Close()
	_ = os.Remove(probe)
	return nil
}

// newConfig creates a default config with a fresh anonymous ID and the current time.
func newConfig() (*Config, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	return &Config{
		Enabled:     true,
		AnonymousID: id,
		NoticeSeen:  false,
		CreatedAt:   time.Now().UTC(),
	}, nil
}
```

### Tests (`config_test.go`)

```go
package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUserConfig_Missing(t *testing.T) {
	t.Parallel()
	overrideHome(t, t.TempDir())
	cfg, err := loadUserConfig()
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestLoadUserConfig_Valid(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	overrideHome(t, home)
	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	body := `{"enabled":true,"anonymousId":"syl_aabbccdd1122","noticeSeen":true,"createdAt":"2026-04-02T00:00:00Z"}`
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte(body), 0644)

	cfg, err := loadUserConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.AnonymousID != "syl_aabbccdd1122" {
		t.Errorf("unexpected ID: %s", cfg.AnonymousID)
	}
	if !cfg.NoticeSeen {
		t.Error("expected noticeSeen true")
	}
}

func TestLoadUserConfig_Malformed(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	overrideHome(t, home)
	dir := filepath.Join(home, ".syllago")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte("{bad json"), 0644)

	_, err := loadUserConfig()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSaveUserConfig_Roundtrip(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	overrideHome(t, home)

	want := &Config{
		Enabled:     true,
		AnonymousID: "syl_aabbccdd1122",
		NoticeSeen:  false,
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
	if err := saveUserConfig(want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := loadUserConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got.AnonymousID != want.AnonymousID {
		t.Errorf("ID mismatch: got %s want %s", got.AnonymousID, want.AnonymousID)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled mismatch: got %v want %v", got.Enabled, want.Enabled)
	}
}

func TestLoadSysConfig_Missing(t *testing.T) {
	t.Parallel()
	// /etc/syllago/telemetry.json won't exist in test env — expect nil, nil.
	sc, err := loadSysConfig()
	if err != nil {
		t.Logf("skipping: /etc/syllago/telemetry.json read error (expected in test): %v", err)
	}
	_ = sc // nil is the expected result in most test environments
}

func TestNewConfig_DefaultsAndID(t *testing.T) {
	t.Parallel()
	cfg, err := newConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected enabled true by default")
	}
	if cfg.NoticeSeen {
		t.Error("expected noticeSeen false by default")
	}
	if len(cfg.AnonymousID) == 0 {
		t.Error("expected non-empty anonymous ID")
	}
}

// overrideHome temporarily replaces UserHomeDirFn for the duration of the test.
// Defined here in config_test.go — no separate testhelpers file needed.
func overrideHome(t *testing.T, dir string) {
	t.Helper()
	orig := UserHomeDirFn
	UserHomeDirFn = func() (string, error) { return dir, nil }
	t.Cleanup(func() { UserHomeDirFn = orig })
}
```

**Note:** `UserHomeDirFn` is declared at package level in `config.go` (exported, capital U) so
that tests in other packages (e.g., `cmd/syllago`) can override it without requiring
internal test access. The declaration and `userConfigPath()` above already incorporate this —
no additional changes are needed. The `overrideHome()` helper in `config_test.go` shows
the pattern for this package's own tests.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/telemetry/ -run TestLoad -v
```

Expected: fail (package doesn't exist yet). Create the files, then tests pass.

---

## Task 2 — Anonymous ID generation

**Files:** `cli/internal/telemetry/idgen.go`, `cli/internal/telemetry/idgen_test.go`
**Depends on:** Task 1 (package exists)

### TDD Approach

Write tests first:
1. Create `idgen_test.go` with the test code below
2. Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/telemetry/ -run TestGenerate -v`
3. Tests fail (idgen.go doesn't exist yet)
4. Create `idgen.go` and implement to make tests pass

### Implementation

Create `cli/internal/telemetry/idgen.go`:

```go
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const idPrefix = "syl_"
const idHexBytes = 6 // 6 bytes = 12 hex chars

// generateID returns a new pseudonymous ID (anonymous ID) in the form syl_a1b2c3d4e5f6.
// Uses crypto/rand — not derived from any machine or user information.
// The ID is persistent across sessions (stored in ~/.syllago/telemetry.json) but is
// pseudonymous, not truly anonymous — it can be used to correlate events over time.
// Documented as pseudonymous in syllago.dev/telemetry.
func generateID() (string, error) {
	b := make([]byte, idHexBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating anonymous ID: %w", err)
	}
	return idPrefix + hex.EncodeToString(b), nil
}

// isValidID returns true if id has the correct syl_ prefix and hex suffix.
func isValidID(id string) bool {
	if len(id) != len(idPrefix)+idHexBytes*2 {
		return false
	}
	if id[:len(idPrefix)] != idPrefix {
		return false
	}
	for _, c := range id[len(idPrefix):] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
```

### Tests (`idgen_test.go`)

```go
package telemetry

import (
	"strings"
	"testing"
)

func TestGenerateID_Format(t *testing.T) {
	t.Parallel()
	id, err := generateID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(id, "syl_") {
		t.Errorf("ID missing syl_ prefix: %s", id)
	}
	if len(id) != 16 { // 4 + 12
		t.Errorf("unexpected ID length %d: %s", len(id), id)
	}
	if !isValidID(id) {
		t.Errorf("isValidID returned false for %s", id)
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := generateID()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestIsValidID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id    string
		valid bool
	}{
		{"syl_a1b2c3d4e5f6", true},
		{"syl_aabbccddeeff", true},
		{"syl_AABBCCDDEEFF", false},  // uppercase not valid
		{"syl_a1b2c3d4e5", false},    // too short
		{"syl_a1b2c3d4e5f6ff", false}, // too long
		{"abc_a1b2c3d4e5f6", false},  // wrong prefix
		{"", false},
	}
	for _, tc := range cases {
		if got := isValidID(tc.id); got != tc.valid {
			t.Errorf("isValidID(%q) = %v, want %v", tc.id, got, tc.valid)
		}
	}
}
```

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/telemetry/ -run TestGenerate -v
```

---

## Task 3 — DO_NOT_TRACK checking

**Files:** `cli/internal/telemetry/dnt.go`, `cli/internal/telemetry/dnt_test.go`
**Depends on:** Task 1 (package exists)

### TDD Approach

Write tests first:
1. Create `dnt_test.go` with the test code below
2. Run: `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/telemetry/ -run TestIsDNT -v`
3. Tests fail (dnt.go doesn't exist yet)
4. Create `dnt.go` and implement to make tests pass

### Implementation

**Important:** `isDNTSet()` must handle all truthy values:
- `"1"`, `"true"`, `"yes"`, `"on"` (all case-insensitive)
- Empty string or `"0"` is falsy
- Any other value is falsy

See test cases below for the complete matrix. A developer implementing from scratch should
not use stricter matching (e.g., `== "1"` only) — the full truthy set is required.

Create `cli/internal/telemetry/dnt.go`:

```go
package telemetry

import (
	"os"
	"strings"
)

// isDNTSet returns true if the DO_NOT_TRACK environment variable is set to any
// truthy value: 1, true, yes, on (case-insensitive). Any other value is falsy.
func isDNTSet() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DO_NOT_TRACK")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
```

### Tests (`dnt_test.go`)

```go
package telemetry

import (
	"testing"
)

func TestIsDNTSet(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"random", false},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("DO_NOT_TRACK", tc.env)
			if got := isDNTSet(); got != tc.want {
				t.Errorf("isDNTSet() with DO_NOT_TRACK=%q = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestIsDNTSet_Unset(t *testing.T) {
	t.Parallel()
	// Ensure variable is absent, not just empty.
	t.Setenv("DO_NOT_TRACK", "")
	if isDNTSet() {
		t.Error("expected false when DO_NOT_TRACK is unset/empty")
	}
}
```

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/telemetry/ -run TestIsDNT -v
```

---

## Task 4 — Core telemetry: Init(), Track(), Shutdown()

**Files:** `cli/internal/telemetry/telemetry.go`, `cli/internal/telemetry/telemetry_test.go`
**Depends on:** Tasks 1, 2, 3

### Why

This is the heart of the feature. `Init()` implements the documented order-of-operations.
`Track()` fires the async HTTP POST. `Shutdown()` drains in-flight sends.
Tests use `httptest.NewServer` — no real network calls.

The PostHog API key is injected at build time via ldflags into the package-level `apiKey`
variable. When empty (dev builds), `Init()` returns immediately without initializing the
HTTP client. This means contributors never send telemetry from local builds without
any special opt-out action.

### Implementation

Create `cli/internal/telemetry/telemetry.go`:

```go
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
//   -X github.com/OpenScribbler/syllago/cli/internal/telemetry.apiKey=phc_...
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

// sysBuildVersion is set by the root command (main.go) before Init() is called.
// It carries the binary's embedded version string.
var sysBuildVersion string

// SetVersion sets the version string for event properties. Called from main.go
// before Init().
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
		// Unreadable config → disable for session (safe default).
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
		// Attempt to save; failure means we disable for the session.
		if err := saveUserConfig(cfg); err != nil {
			state.disabled = true
			return
		}
	} else if created {
		// Newly created config but somehow noticeSeen was already true (shouldn't happen,
		// but save it now so we don't re-create next run).
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
// Call from PersistentPostRun in main.go.
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
	data, err := json.Marshal(payload)
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
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

// firstRunNotice is the exact first-run notice text printed to stderr.
// Wording is locked — do not modify without updating docs/plans/2026-04-02-telemetry-design.md.
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
// Returns an error if the config cannot be loaded or saved.
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
// Returns an error if the config cannot be read or written.
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
```

### Tests (`telemetry_test.go`)

```go
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
	t.Parallel()
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
	t.Parallel()
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

	// Make UserHomeDirFn return an error (containerized/CI environment).
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
	t.Cleanup(func() { os.Chmod(dir, 0755) }) // restore so TempDir cleanup works

	var notice strings.Builder
	NoticeWriter = &notice
	t.Cleanup(func() { NoticeWriter = os.Stderr })

	Init()
	if !state.disabled {
		t.Error("expected disabled when config dir is not writable")
	}
}
```

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/telemetry/ -v
```

All tests must pass before proceeding.

---

## Task 5 — `syllago telemetry` subcommand

This task is split into three sub-tasks (5a, 5b, 5c) for granularity. They share one
implementation file and one test file — the split is conceptual, not physical. Work
through them in order: implement + test `status`, then `on`/`off`, then `reset`.

**Files:** `cli/cmd/syllago/telemetry_cmd.go`, `cli/cmd/syllago/telemetry_cmd_test.go`
**Depends on:** Task 4 (telemetry package must exist before cmd tests can import it)

**Note on Task 6 ordering:** Task 6 (root cmd integration) can run in parallel with
Task 5, but completing Task 5 first keeps all telemetry code together before wiring
the root command. Cobra's `init()` functions run during package initialization
regardless of order — there is no runtime dependency.

### Why

The subcommand exposes user-facing controls. All four sub-subcommands delegate to the
`telemetry` package functions from Task 4. Tests follow the exact same `output.SetForTest`
+ `RunE` pattern used throughout the CLI.

---

### Task 5a — `status` subcommand

Implement `telemetryStatusCmd` and `runTelemetryStatus`. Write `TestTelemetryStatusCmd_HumanReadable`
and `TestTelemetryStatusCmd_JSON` first, then implement.

---

### Task 5b — `on` and `off` subcommands

Implement `telemetryOnCmd` and `telemetryOffCmd`. Write `TestTelemetryOnCmd` and
`TestTelemetryOffCmd` first, then implement.

---

### Task 5c — `reset` subcommand

Implement `telemetryResetCmd`. Write `TestTelemetryResetCmd` and `TestTelemetryResetCmd_JSON`
first, then implement.

### Implementation

Create `cli/cmd/syllago/telemetry_cmd.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Manage usage analytics settings",
	Long:  "View and control anonymous usage data collection. Run 'syllago telemetry status' for details.",
	Example: `  syllago telemetry status
  syllago telemetry off
  syllago telemetry on
  syllago telemetry reset`,
	RunE: runTelemetryStatus,
}

var telemetryStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show telemetry state and anonymous ID",
	Example: `  syllago telemetry status`,
	RunE:    runTelemetryStatus,
}

func runTelemetryStatus(cmd *cobra.Command, args []string) error {
	cfg := telemetry.Status()

	if output.JSON {
		type statusOut struct {
			Enabled     bool   `json:"enabled"`
			AnonymousID string `json:"anonymousId"`
			Endpoint    string `json:"endpoint,omitempty"`
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = "https://us.i.posthog.com/capture/"
		}
		data, _ := json.MarshalIndent(statusOut{
			Enabled:     cfg.Enabled,
			AnonymousID: cfg.AnonymousID,
			Endpoint:    endpoint,
		}, "", "  ")
		fmt.Fprintln(output.Writer, string(data))
		return nil
	}

	state := "disabled"
	if cfg.Enabled {
		state = "enabled"
	}
	fmt.Fprintf(output.Writer, "Telemetry: %s\n", state)
	fmt.Fprintf(output.Writer, "Anonymous ID: %s\n\n", cfg.AnonymousID)
	fmt.Fprintf(output.Writer, "Events tracked:\n")
	fmt.Fprintf(output.Writer, "  command_executed    Command name, provider, content type, success/failure,\n")
	fmt.Fprintf(output.Writer, "                      syllago version, OS, architecture\n")
	fmt.Fprintf(output.Writer, "  error_occurred      Structured error code on failure\n")
	fmt.Fprintf(output.Writer, "  tui_session_started TUI opened (no interaction details)\n\n")
	fmt.Fprintf(output.Writer, "Never tracked:\n")
	fmt.Fprintf(output.Writer, "  File contents, paths, usernames, hostnames, registry URLs,\n")
	fmt.Fprintf(output.Writer, "  hook commands, MCP configs, or any content you manage.\n\n")
	fmt.Fprintf(output.Writer, "Disable:  syllago telemetry off\n")
	fmt.Fprintf(output.Writer, "Reset ID: syllago telemetry reset\n")
	fmt.Fprintf(output.Writer, "Docs:     https://syllago.dev/telemetry\n")
	return nil
}

var telemetryOnCmd = &cobra.Command{
	Use:     "on",
	Short:   "Enable telemetry",
	Example: `  syllago telemetry on`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetEnabled(true); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not save telemetry config: %v\n", err)
			fmt.Fprintf(output.ErrWriter, "Telemetry state may not persist across sessions.\n")
			return nil
		}
		if output.JSON {
			fmt.Fprintln(output.Writer, `{"enabled":true}`)
			return nil
		}
		fmt.Fprintln(output.Writer, "Telemetry enabled.")
		return nil
	},
}

var telemetryOffCmd = &cobra.Command{
	Use:     "off",
	Short:   "Disable telemetry",
	Example: `  syllago telemetry off`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetEnabled(false); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not save telemetry config: %v\n", err)
			fmt.Fprintf(output.ErrWriter, "Telemetry state may not persist across sessions.\n")
			return nil
		}
		if output.JSON {
			fmt.Fprintln(output.Writer, `{"enabled":false}`)
			return nil
		}
		fmt.Fprintln(output.Writer, "Telemetry disabled.")
		return nil
	},
}

var telemetryResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Generate a new anonymous ID",
	Long: `Generates a new anonymous ID. Previously collected data under your old ID
is not deleted from PostHog. To request deletion, email privacy@syllago.dev
with your old ID.`,
	Example: `  syllago telemetry reset`,
	RunE: func(cmd *cobra.Command, args []string) error {
		newID, err := telemetry.Reset()
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not reset telemetry ID: %v\n", err)
			return nil
		}
		if output.JSON {
			data, _ := json.MarshalIndent(map[string]string{"anonymousId": newID}, "", "  ")
			fmt.Fprintln(output.Writer, string(data))
			return nil
		}
		fmt.Fprintf(output.Writer, "Anonymous ID rotated: %s\n\n", newID)
		fmt.Fprintf(output.Writer, "Note: Previously collected data under your old ID is not deleted.\n")
		fmt.Fprintf(output.Writer, "To request deletion, email privacy@syllago.dev with your old ID.\n")
		return nil
	},
}

func init() {
	telemetryCmd.AddCommand(telemetryStatusCmd, telemetryOnCmd, telemetryOffCmd, telemetryResetCmd)
	rootCmd.AddCommand(telemetryCmd)
}
```

### Tests (`telemetry_cmd_test.go`)

```go
package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

func TestTelemetryStatusCmd_HumanReadable(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryStatusCmd.RunE(telemetryStatusCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Telemetry:") {
		t.Errorf("missing Telemetry: line; got:\n%s", out)
	}
	if !strings.Contains(out, "Anonymous ID:") {
		t.Errorf("missing Anonymous ID: line; got:\n%s", out)
	}
	if !strings.Contains(out, "https://syllago.dev/telemetry") {
		t.Errorf("missing docs URL; got:\n%s", out)
	}
}

func TestTelemetryStatusCmd_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	overrideTelemetryHome(t)

	if err := telemetryStatusCmd.RunE(telemetryStatusCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"enabled"`) {
		t.Errorf("JSON missing enabled field; got:\n%s", out)
	}
	if !strings.Contains(out, `"anonymousId"`) {
		t.Errorf("JSON missing anonymousId field; got:\n%s", out)
	}
}

func TestTelemetryOnCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryOffCmd.RunE(telemetryOffCmd, nil); err != nil {
		t.Fatalf("off command failed: %v", err)
	}
	if err := telemetryOnCmd.RunE(telemetryOnCmd, nil); err != nil {
		t.Fatalf("on command failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "enabled") {
		t.Errorf("expected 'enabled' in output; got: %s", stdout.String())
	}

	cfg := telemetry.Status()
	if !cfg.Enabled {
		t.Error("expected telemetry enabled after 'on'")
	}
}

func TestTelemetryOffCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryOffCmd.RunE(telemetryOffCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "disabled") {
		t.Errorf("expected 'disabled' in output; got: %s", stdout.String())
	}

	cfg := telemetry.Status()
	if cfg.Enabled {
		t.Error("expected telemetry disabled after 'off'")
	}
}

func TestTelemetryResetCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	// Seed initial config.
	telemetry.SetEnabled(true)
	before := telemetry.Status()

	if err := telemetryResetCmd.RunE(telemetryResetCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "rotated") {
		t.Errorf("missing 'rotated' in output; got: %s", out)
	}
	if !strings.Contains(out, "not deleted") {
		t.Errorf("missing deletion note in output; got: %s", out)
	}

	after := telemetry.Status()
	if after.AnonymousID == before.AnonymousID {
		t.Error("ID should change after reset")
	}
}

func TestTelemetryResetCmd_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	overrideTelemetryHome(t)

	if err := telemetryResetCmd.RunE(telemetryResetCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"anonymousId"`) {
		t.Errorf("JSON missing anonymousId; got: %s", stdout.String())
	}
}

// overrideTelemetryHome creates a temp dir and wires it as the home dir for
// the telemetry package for the duration of this test.
// Requires telemetry.UserHomeDirFn to be exported (capital U) — see Task 1.
func overrideTelemetryHome(t *testing.T) {
	t.Helper()
	temp := t.TempDir()
	orig := telemetry.UserHomeDirFn
	telemetry.UserHomeDirFn = func() (string, error) { return temp, nil }
	t.Cleanup(func() { telemetry.UserHomeDirFn = orig })
}
```

**Note:** `overrideTelemetryHome` uses `telemetry.UserHomeDirFn` (exported, capital U) which
is declared in Task 1's `config.go`. The implementation and export are already shown in
Task 1 — no additional changes are needed here. The pattern mirrors `overrideHome()` in
the telemetry package's own tests, but crosses a package boundary.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./cmd/syllago/ -run TestTelemetry -v
```

---

## Task 6 — Root command integration (PersistentPreRun / PersistentPostRun)

**Files:** `cli/cmd/syllago/main.go`
**Depends on:** Task 4 (Task 5 should be complete first, but can run in parallel — see Task 5 note)

### Why

`PersistentPreRun` and `PersistentPostRun` fire on every command. We add telemetry
initialization to PreRun and shutdown to PostRun. This requires care because the existing
`PersistentPreRunE` sets global output flags — we must call `telemetry.Init()` after those
flags are parsed, so the notice (if any) respects `--no-color`.

### Add the import to `main.go`

The existing import block in `main.go` (after the standard library imports, with other
internal packages) — add the telemetry package alongside them:

```go
import (
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/config"
    "github.com/OpenScribbler/syllago/cli/internal/metadata"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
    "github.com/OpenScribbler/syllago/cli/internal/telemetry"  // ADD THIS
    "github.com/OpenScribbler/syllago/cli/internal/tui"
    "github.com/OpenScribbler/syllago/cli/internal/updater"
    // ... rest of imports unchanged ...
)
```

### Changes to `main.go`

**Current state** (the existing `PersistentPreRunE` in `init()`):
```go
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    noColor, _ := cmd.Flags().GetBool("no-color")
    if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
        lipgloss.SetColorProfile(termenv.Ascii)
    }

    quiet, _ := cmd.Flags().GetBool("quiet")
    output.Quiet = quiet

    verbose, _ := cmd.Flags().GetBool("verbose")
    output.Verbose = verbose

    return nil
}
```

**After Task 6** — add `telemetry.Init()` call and new `PersistentPostRun`:
```go
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    noColor, _ := cmd.Flags().GetBool("no-color")
    if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
        lipgloss.SetColorProfile(termenv.Ascii)
    }

    quiet, _ := cmd.Flags().GetBool("quiet")
    output.Quiet = quiet

    verbose, _ := cmd.Flags().GetBool("verbose")
    output.Verbose = verbose

    // Initialize telemetry after output flags are set. Init() checks DO_NOT_TRACK,
    // /etc config, and user config. It writes the first-run notice to stderr if needed.
    // Note: telemetry.Init() automatically writes to NoticeWriter (os.Stderr by default).
    // The notice respects --no-color because lipgloss.SetColorProfile() runs first above.
    // telemetry.SetVersion() must be called before Init() — see main() below.
    telemetry.Init()

    return nil
}

rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
    telemetry.Shutdown()
}
```

Add `telemetry.SetVersion(version)` in `main()` before `rootCmd.Execute()`:

**Current state** (existing `main()`):
```go
func main() {
    if buildCommit != "" {
        ensureUpToDate()
    }
    if err := rootCmd.Execute(); err != nil {
        printExecuteError(err)
        os.Exit(output.ExitError)
    }
}
```

**After Task 6:**
```go
func main() {
    if buildCommit != "" {
        ensureUpToDate()
    }
    telemetry.SetVersion(version)  // ADD THIS — must be before rootCmd.Execute()
    if err := rootCmd.Execute(); err != nil {
        printExecuteError(err)
        os.Exit(output.ExitError)
    }
}
```

**PersistentPostRun limitation (accepted):** Cobra does not run `PersistentPostRun` if the
command returns an error. This means in-flight Track() goroutines are not explicitly drained
on error paths. This is acceptable — each goroutine has a 2-second HTTP client timeout and
will complete or time out on its own. Shutdown is a best-effort drain, not a guarantee.
This behavior is documented as an accepted limitation, not a bug.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
make build
# Verify notice appears once then disappears:
DO_NOT_TRACK=1 ./syllago version  # no notice
./syllago version                 # notice on first run
./syllago version                 # no notice on second run
```

---

---

## Task 8 — First-run notice: TUI toast path

**Files:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app_update.go`

**Depends on:** Task 4

### Pre-task verification

Before implementing, verify that `toastWarning` is defined in `cli/internal/tui/toast.go`:
```bash
grep -n "toastWarning" /home/hhewett/.local/src/syllago/cli/internal/tui/toast.go
```
Expected: `toastWarning toastLevel = iota` (or similar). This constant is already defined
in the existing codebase — no addition needed. If it's missing for any reason, add it to
`toast.go` before proceeding.

### Why

The TUI calls `tui.NewApp()` which is separate from the cobra command lifecycle. When the
TUI launches and `noticeSeen` is false, it should display an info toast. The toast uses the
existing `toastModel.Push()` mechanism.

### Implementation

In `app.go`, add a `telemetryNotice bool` field to `App` and populate it in `NewApp()`:

```go
// In App struct:
telemetryNotice bool // true if first-run telemetry notice should be shown as toast

// In NewApp(), before building the App struct:
showTelemetryNotice := false
telemetryCfg := telemetry.Status()
if !telemetryCfg.NoticeSeen && telemetryCfg.Enabled {
    showTelemetryNotice = true
}

// In the returned App:
a := App{
    // ... existing fields ...
    telemetryNotice: showTelemetryNotice,
}
```

In `app.go`'s `Init()`:

```go
func (a App) Init() tea.Cmd {
    if a.telemetryNotice {
        return func() tea.Msg {
            return telemetryNoticeMsg{}
        }
    }
    return nil
}
```

Add the message type and handler in `app_update.go`. The handler pushes the toast and fires
the Track() for `tui_session_started`:

```go
// Message type — add to cli/internal/tui/app_update.go (near the top, with other message types):
type telemetryNoticeMsg struct{}

// In App.Update():
case telemetryNoticeMsg:
    const noticeText = "Syllago collects anonymous usage data to help prioritize\n" +
        "development. No file contents or identifying info is collected.\n" +
        `Run "syllago telemetry off" to disable.` + "\n" +
        "syllago.dev/telemetry"
    cmd := a.toast.Push(noticeText, toastWarning)
    return a, cmd
```

Add the import to `app.go`:
```go
"github.com/OpenScribbler/syllago/cli/internal/telemetry"
```

**Note:** `toastWarning` is used (not `toastSuccess`) because the notice is informational but
deserves attention. Warning toasts have a 5-second auto-dismiss duration which gives users
time to read it.

**Note on double-notice:** The CLI path (`Init()` in `telemetry.go`) sets `noticeSeen: true`
and saves the config when the CLI command runs before the TUI launches. The TUI path checks
`Status()` before `Init()` has run in the CLI path. To avoid both showing simultaneously,
note that `PersistentPreRun` runs `telemetry.Init()` first, which sets `noticeSeen: true`
and saves it. Then `runTUI()` calls `tui.NewApp()`, which calls `telemetry.Status()` and
reads the now-saved config — finding `noticeSeen: true`. The toast will NOT fire on the same
invocation as the CLI notice. This is correct behavior.

**Note on App.Init():** The current `Init()` in `app.go` returns `nil`. The implementation
must change it to return a command conditionally:
```go
func (a App) Init() tea.Cmd {
    if a.telemetryNotice {
        return func() tea.Msg { return telemetryNoticeMsg{} }
    }
    return nil
}
```
This is the only change to `Init()` — the existing `nil` return becomes the else-branch.

**Note on golden tests:** Adding `telemetryNotice bool` to the App struct and modifying
`Init()` does not change any visual output. Golden tests do not need to be regenerated
for this task. However, run `make test` to confirm no regressions.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
rm ~/.syllago/telemetry.json
# Run TUI, verify toast appears at bottom-right with telemetry notice
./syllago
```

---

## Task 9 — Track() call sites in existing commands

This task is split into five sub-tasks (9a–9e). Sub-tasks 9a–9d are independent and can
be parallelized. Sub-task 9e (TUI) should run after Task 8 is complete.

**Files:**
- Task 9a: `cli/cmd/syllago/install_cmd.go`
- Task 9b: `cli/cmd/syllago/add_cmd.go`
- Task 9c: `cli/cmd/syllago/convert_cmd.go`
- Task 9d: `cli/cmd/syllago/export_cmd.go`
- Task 9e: `cli/cmd/syllago/main.go` (TUI session event — depends on Task 8)

**Depends on:** Task 4 (for 9a–9d); Task 4 + Task 8 (for 9e)

### Why

Track() calls are one-liners at the end of each command's `RunE`, after all work is done.
They don't change command flow — they're fire-and-forget. The call goes at the end of the
success path; error paths can use a separate `error_occurred` event or just skip tracking.

### Pattern

Each call follows this form:
```go
telemetry.Track("command_executed", map[string]any{
    "command":       "install",
    "provider":      toSlug,
    "content_type":  typeFilter,
    "content_count": len(installed),
    "success":       true,
})
return nil
```

For error paths, add before returning the error:
```go
telemetry.Track("error_occurred", map[string]any{
    "command":    "install",
    "error_code": output.ErrProviderNotFound,
})
```

### Task 9a — install_cmd.go

At the end of `runInstall`, just before `return nil`:
```go
telemetry.Track("command_executed", map[string]any{
    "command":       "install",
    "provider":      toSlug,
    "content_type":  typeFilter,
    "content_count": len(result.Installed),
    "success":       true,
    "dry_run":       dryRun,
})
```

### Task 9b — add_cmd.go

Read `add_cmd.go` to identify the success return point(s). `runAdd` delegates to
sub-functions (`runAddHooks`, `runAddMcp`, `runAddFromShared`) which each return errors.
The Track() call should go in `runAdd`, after the sub-function returns nil, using
variables already in scope:

```go
// source_type: derive from flags/args — "shared" if fromSlug=="shared",
// otherwise just omit or use "provider"
telemetry.Track("command_executed", map[string]any{
    "command":     "add",
    "from":        fromSlug, // already in scope
    "success":     true,
})
```

Note: `sourceType` and `contentType` are not named variables in `runAdd`'s outer scope.
Use `fromSlug` (already declared) instead. The executor must read `add_cmd.go` carefully
to locate all success return paths — `runAdd` has early returns for errors before the
sub-calls, so Track() only fires on the nil return path.

### Task 9c — convert_cmd.go

`runConvert` delegates to `convertFile` or `convertLibraryItem` and returns their result
directly — there is no code after the two `return` statements where Track() could be placed
without restructuring. The Track() call must be added by capturing the error and tracking
before returning:

```go
func runConvert(cmd *cobra.Command, args []string) error {
    // ... existing logic ...
    var err error
    if isFilePath(input) {
        err = convertFile(input, fromSlug, toSlug, typeStr, outputPath, *toProv, showDiff)
    } else {
        err = convertLibraryItem(input, fromSlug, toSlug, outputPath, *toProv, showDiff)
    }
    if err == nil {
        telemetry.Track("command_executed", map[string]any{
            "command":       "convert",
            "from_provider": fromSlug,
            "to_provider":   toSlug,
            "success":       true,
        })
    }
    return err
}
```

Note: `contentType` is not a variable in `runConvert`'s scope. Omit it from this call site.
The executor must refactor the two-return structure into a captured-error pattern.

### Task 9d — export_cmd.go

**Note:** `runExport` in `export_cmd.go` is currently a stub that returns
`fmt.Errorf("export is not yet implemented")`. There is no success path to instrument.
Skip this task until export is implemented. The Track() call will be added at that time
alongside the real implementation.

~~At the end of the success path in `runExport`:~~
```go
// DEFERRED — export is not yet implemented (see export_cmd.go:43)
// Add when export is implemented:
telemetry.Track("command_executed", map[string]any{
    "command":  "export",
    "success":  true,
})
```

### Task 9e — TUI tui_session_started event (depends on Task 8)

In `runTUI` in `main.go`, after the program runs successfully (after `p.Run()` returns nil):
```go
telemetry.Track("tui_session_started", map[string]any{
    "success": true,
})
```

This fires after the TUI exits (when the user quits), not when it starts. This is intentional:
if the TUI crashes, we don't count it as a completed session. The event fires from the CLI
path, not the TUI itself, so there's no double-counting.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli
make build
go test ./cmd/syllago/ -v   # existing tests must still pass
```

---

## Task 10 — Build system: embed PostHog API key via ldflags

**Files:** `cli/Makefile`
**Depends on:** Task 4

### Why

The API key must be absent from source code (attackers could fork and use it) but present in
release binaries. ldflags is the standard Go mechanism for this — the same pattern used for
`version` and `buildCommit`. CI/release environments set `SYLLAGO_POSTHOG_KEY`. Dev builds
leave it unset, meaning `apiKey == ""` and telemetry is compiled out.

### Changes

In `cli/Makefile`:

```makefile
POSTHOG_KEY := $(SYLLAGO_POSTHOG_KEY)

LDFLAGS := -X main.repoRoot=$(REPO_ROOT) -X main.buildCommit=$(COMMIT) -X main.version=$(VERSION)
ifneq ($(SIGNING_KEY),)
  LDFLAGS += -X github.com/OpenScribbler/syllago/cli/internal/updater.SigningPublicKey=$(SIGNING_KEY)
endif
ifneq ($(POSTHOG_KEY),)
  LDFLAGS += -X github.com/OpenScribbler/syllago/cli/internal/telemetry.apiKey=$(POSTHOG_KEY)
endif
```

The existing `build` target already uses `$(LDFLAGS)`, so no changes to the build recipe.

### CI configuration

In the GitHub Actions release workflow (`.github/workflows/release.yml`), add the secret to
the build step's environment:
```yaml
env:
  SYLLAGO_POSTHOG_KEY: ${{ secrets.SYLLAGO_POSTHOG_KEY }}
```

Add the secret `SYLLAGO_POSTHOG_KEY` with the actual PostHog project API key
(configured during release setup, not stored in source) to the GitHub repository's
Actions secrets.

### Verification

```bash
# Dev build — no key embedded:
cd /home/hhewett/.local/src/syllago/cli && make build
strings ./syllago | grep phc_   # should print nothing

# Release simulation — key embedded:
SYLLAGO_POSTHOG_KEY=phc_test make build
strings ./syllago | grep phc_   # should print phc_test
```

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli && make build
```

---

## Task 11 — Docs site telemetry page

**Files:** `docs/telemetry.md` (new)
**Depends on:** all previous tasks (content depends on implemented behavior)

### Docs site status

The `docs/` directory at `/home/hhewett/.local/src/syllago/docs/` contains only markdown
files and subdirectories — no `package.json`, `config.toml`, `mkdocs.yml`, or `index.html`.
There is no docs site framework configured yet. The `docs/` directory is currently an
unbuilt collection of markdown design docs.

**What this means for Task 11:** Create the telemetry page as a markdown file at
`docs/telemetry.md`. When a docs site is eventually set up (Astro, Hugo, or mkdocs),
this file will be incorporated into it. There is no local preview to run at this time.

**File to create:** `/home/hhewett/.local/src/syllago/docs/telemetry.md`

### Required content (nine sections, all mandatory per design doc)

The page must cover all nine sections specified in the design doc:

1. **What we collect** — event list with example payloads and what question each answers
2. **What we never collect** — the privacy guarantees table from the design doc
3. **How to disable** — `syllago telemetry off`, `DO_NOT_TRACK=1`, config file path,
   `/etc/syllago/telemetry.json` for fleet management
4. **Why we collect it** — honest explanation (solo developer, steer priorities, find bugs)
5. **How it works** — fire-and-forget POST, no local storage, fails silently
6. **Data retention** — 1 year (PostHog Cloud free tier default), automatically deleted
7. **Data deletion** — email `privacy@syllago.dev` with anonymous ID(s), 30-day processing
8. **PostHog compliance** — links to PostHog SOC2 report, DPA, data residency, GDPR posture,
   IP stripping documentation (https://posthog.com/docs/privacy), and note that "Discard
   client IP data" is enabled on the syllago PostHog project
9. **Enterprise deployment** — fleet-wide opt-out via `/etc/syllago/telemetry.json`,
   self-hosted endpoint configuration via `endpoint` field, `DO_NOT_TRACK` for CI,
   verification at scale via `syllago telemetry status --json`

### Links from other pages (to add separately, not blocking)

After the page is created:
- Add one line to README near installation instructions: `Syllago collects anonymous usage data to help improve the tool. [Learn more and opt out.](https://syllago.dev/telemetry)`
- Add to CONTRIBUTING.md: note that dev builds have telemetry compiled out (no `SYLLAGO_POSTHOG_KEY` in local builds)
- The `syllago telemetry status` output already links to the page (implemented in Task 5)
- The first-run notice already links to the page (implemented in Task 4)

### Commands

```bash
# No docs site framework exists yet — just create the markdown file.
# Verify the file renders correctly by opening it in a markdown viewer.
ls /home/hhewett/.local/src/syllago/docs/telemetry.md  # confirm file exists
```

---

## Post-Implementation Verification

Run after Tasks 4 and 6 are complete to verify the first-run notice behaves correctly
end-to-end. No new code is added here — this is a verification checklist only.

### First-run notice scenarios

1. **Fresh install** — delete `~/.syllago/telemetry.json`, run `syllago version`. Notice
   appears on stderr, not stdout. `syllago version | cat` shows only the version.

2. **Second run** — run `syllago version` again. Notice does not appear.

3. **DO_NOT_TRACK=1** — notice does not appear, no config file is created.

4. **Piped output** — `syllago list | jq .` works correctly on the first run (notice goes
   to stderr, not stdout).

5. **--json flag** — `syllago telemetry status --json` on first run still shows the notice
   to stderr (not stdout), and JSON output is clean.

### Commands

```bash
cd /home/hhewett/.local/src/syllago/cli && make build
rm -f ~/.syllago/telemetry.json

# Test 1: notice to stderr, not stdout
./syllago version 2>/dev/null       # should show only the version
./syllago version 2>&1 1>/dev/null  # should show only the notice

# Test 2: no notice on second run
./syllago version 2>/dev/null   # version only, no notice to stderr

# Test 3: DO_NOT_TRACK suppresses notice
rm ~/.syllago/telemetry.json
DO_NOT_TRACK=1 ./syllago version 2>&1  # notice should NOT appear
```

---

## Final integration checklist

Run after all tasks are complete:

```bash
cd /home/hhewett/.local/src/syllago/cli

# 1. Format check
make fmt
git diff --name-only  # should be empty after fmt

# 2. Full test suite
make test

# 3. Build and smoke test
SYLLAGO_POSTHOG_KEY=phc_test make build

# 4. DO_NOT_TRACK works
rm -f ~/.syllago/telemetry.json
DO_NOT_TRACK=1 ./syllago version 2>&1  # no notice, no network call

# 5. First-run notice appears once to stderr
rm ~/.syllago/telemetry.json
./syllago version 2>/dev/null     # clean stdout
./syllago version 2>&1 1>/dev/null  # notice on stderr only
./syllago version 2>&1 1>/dev/null  # no notice on second run

# 6. Telemetry subcommand works
./syllago telemetry status
./syllago telemetry off
./syllago telemetry status  # should show disabled
./syllago telemetry on
./syllago telemetry reset   # should output "not deleted" note
./syllago telemetry status --json

# 7. Piped output not corrupted
rm ~/.syllago/telemetry.json
./syllago list --json | jq .  # JSON must parse cleanly (notice goes to stderr)

# 8. Dev build has no API key
strings ./syllago | grep phc_  # should be empty (dev build)
```

---

## Task dependency graph

```
Task 1 (config types)
  └─ Task 2 (ID gen)           ← can run in parallel with Task 3
  └─ Task 3 (DNT)              ← can run in parallel with Task 2
  └─ Task 4 (core Init/Track/Shutdown) ← depends on 1, 2, 3
      └─ Task 5a (status subcommand)
      └─ Task 5b (on/off subcommands)
      └─ Task 5c (reset subcommand)
      └─ Task 6 (root cmd integration) ← can run in parallel with Task 5
      └─ Task 8 (TUI toast)
      │   └─ Task 9e (TUI session event)
      └─ Task 9a (install Track() call)   ← 9a–9d can be parallelized
      └─ Task 9b (add Track() call)
      └─ Task 9c (convert Track() call)
      └─ Task 9d (export Track() call)
Task 10 (Makefile ldflags) ← independent, can run anytime
Task 11 (docs page) ← depends on all, go last
Post-Implementation Verification ← depends on Task 4 + Task 6, no new code
```

Tasks 1–3 can be written in parallel (same package, no inter-dependency).
Tasks 5a/5b/5c, 6, 8, and 9a–9d can all be parallelized once Task 4 is done.
Task 9e (TUI event) should follow Task 8 to keep TUI changes together.
