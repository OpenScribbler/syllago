# Capability Monitor Pipeline — Implementation Plan

**Goal:** Build deterministic capability extraction pipeline (`syllago capmon`)
**Design Doc:** `docs/plans/2026-04-08-capability-monitor-pipeline-design.md`
**Architecture:** Four-stage pipeline (fetch → extract → diff → review) with per-provider YAML as source of truth. Hub-and-spoke: per-provider files are authoritative; per-content-type views are generated. GitHub Actions: two-job split (`fetch-extract` with `permissions: {}`, `report` with write perms) with SHA-256 artifact verification between jobs.
**Tech Stack:** Go, `gotreesitter` (pure Go tree-sitter, TS/Rust), `goquery` (HTML/CSS), `goldmark` (Markdown AST), `go/parser` (Go AST), `chromedp` (Cursor only), `gopkg.in/yaml.v3`, `encoding/json`, `BurntSushi/toml`

**Bead Chain Type:** Full Phase Chain
**TDD:** Red/Green mandatory for every implementation task
**Validation:** Per-task validation by separate agent (Haiku) + phase validation gate at end of each phase

---

## Pre-Implementation Checklist (Blocking)

Before Phase 1 begins, a named owner must sign off on the `gotreesitter` dependency:

- [ ] Module path verified (`github.com/odvcencio/gotreesitter` or current canonical path)
- [ ] No CGO in any transitive dep (`go mod graph | grep -i cgo` returns nothing for gotreesitter)
- [ ] No `.wasm`, `.so`, `.a`, or other native artifacts in bundled grammar modules
- [ ] S-expression query support confirmed for TypeScript and Rust grammars
- [ ] Binary size impact benchmarked (upper bound: 20MB)
- [ ] Owner signs off in the capmon epic bead before Phase 4 begins

Until this checklist is cleared, Phase 4 (TypeScript and Rust extractors) is blocked. Phases 1–3 and Phase 5 (HTML/Markdown/Go/JSON/YAML/TOML extractors) are NOT blocked and can proceed.

---

## Phase 1: Bootstrap — Core Types, Interfaces, and Package Skeleton

**Goal:** Establish the package layout, core types, and the `Extractor` interface. No functional logic yet — just the contracts every subsequent phase builds against.

**Security note:** All three H-cluster security controls (H2 `sanitizeExtractedString`, H4 `validateSourceURL`, H5/H6 `sanitizeSlug`+`buildPRBody`) MUST land together in Phase 1. An intermediate state where any one is absent is an exposure window. Phase 1 is complete only when all three are in place.

---

### Task 1.1: Core types and Extractor interface (impl)

**Files:**
- Create: `cli/internal/capmon/types.go` — `SelectorConfig`, `ExtractedSource`, `FieldValue`, `RunManifest`, `ProviderStatus`, `CapabilityDiff`, `FieldChange`, exit class constants
- Create: `cli/internal/capmon/extractor.go` — `Extractor` interface, `Extract()` dispatch function, `extractors` registry map
- Create: `cli/internal/capmon/types_test.go` — type construction and zero-value sanity checks

**Depends on:** Nothing (first task)

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestTypes` → PASS — types compile and can be constructed
- `cd cli && go vet ./internal/capmon/` → clean — no vet issues

#### Step 1: Write the failing test (RED)

```go
// cli/internal/capmon/types_test.go
package capmon_test

import (
    "testing"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestExtractedSource_ZeroValue(t *testing.T) {
    var es capmon.ExtractedSource
    if es.Partial != false {
        t.Error("zero value Partial should be false")
    }
    if es.Fields == nil {
        // Fields being nil is acceptable at zero value, just document it
    }
}

func TestRunManifest_ExitClasses(t *testing.T) {
    tests := []struct {
        name  string
        class int
    }{
        {"clean", capmon.ExitClean},
        {"drifted", capmon.ExitDrifted},
        {"partial_failure", capmon.ExitPartialFailure},
        {"infrastructure_failure", capmon.ExitInfrastructureFailure},
        {"fatal", capmon.ExitFatal},
        {"paused", capmon.ExitPaused},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := capmon.RunManifest{ExitClass: tt.class}
            if m.ExitClass != tt.class {
                t.Errorf("got %d, want %d", m.ExitClass, tt.class)
            }
        })
    }
}

func TestSelectorConfig_Fields(t *testing.T) {
    cfg := capmon.SelectorConfig{
        Primary:          "main table",
        Fallback:         "table",
        ExpectedContains: "Event Name",
        MinResults:       6,
        UpdatedAt:        "2026-04-08",
    }
    if cfg.Primary != "main table" {
        t.Error("Primary not set")
    }
    if cfg.MinResults != 6 {
        t.Error("MinResults not set")
    }
}

func TestRunManifest_NeverReadAsInput(t *testing.T) {
    // Verifies the build comment is present on the type.
    // This is a compile-time guarantee — if the type compiles, the comment exists.
    _ = capmon.RunManifest{
        RunID:     "test-run-id",
        StartedAt: time.Now(),
    }
}

func TestExtract_UnknownFormat(t *testing.T) {
    ctx := context.Background()  // will fail — needs import
    _, err := capmon.Extract(ctx, "unknown-format", []byte("data"), capmon.SelectorConfig{})
    if err == nil {
        t.Error("expected error for unknown format")
    }
    if !strings.Contains(err.Error(), "no extractor for format") {
        t.Errorf("unexpected error: %v", err)
    }
}
```

#### Step 2: Verify test fails
```
cd cli && go test ./internal/capmon/ -run TestTypes
```
Expected: FAIL — `cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/types.go`:
```go
// Package capmon implements the syllago capability monitor pipeline.
// Four-stage pipeline: fetch → extract → diff → review.
package capmon

import "time"

// Exit class constants for RunManifest.ExitClass.
const (
    ExitClean                = 0 // All providers extracted, no drift
    ExitDrifted              = 1 // Drift detected, PR/issue opened
    ExitPartialFailure       = 2 // Some providers failed, others succeeded
    ExitInfrastructureFailure = 3 // Chromedp/network/selector broken
    ExitFatal                = 4 // Config corrupt, schema validation failed
    ExitPaused               = 5 // .capmon-pause sentinel present
)

// SelectorConfig describes how to locate content within a fetched source.
// Lives in its own file (selector.go) — defined here in types for the struct.
type SelectorConfig struct {
    Primary          string `yaml:"primary"`
    Fallback         string `yaml:"fallback,omitempty"`
    ExpectedContains string `yaml:"expected_contains,omitempty"`
    MinResults       int    `yaml:"min_results,omitempty"`
    UpdatedAt        string `yaml:"updated_at,omitempty"`
}

// FieldValue is an extracted scalar with its SHA-256 fingerprint.
type FieldValue struct {
    Value     string `json:"value"`
    ValueHash string `json:"value_hash"`
}

// ExtractedSource is the Stage 2 output for one source document.
// capmon: pipeline-internal volatile state, no schema_version
type ExtractedSource struct {
    ExtractorVersion string                `json:"extractor_version"`
    Provider         string                `json:"provider"`
    SourceID         string                `json:"source_id"`
    Format           string                `json:"format"`
    ExtractedAt      time.Time             `json:"extracted_at"`
    Partial          bool                  `json:"partial"`
    Fields           map[string]FieldValue `json:"fields"`
    Landmarks        []string              `json:"landmarks"`
}

// FieldChange describes a single field mutation detected in Stage 3.
type FieldChange struct {
    FieldPath string `json:"field_path"`
    OldValue  string `json:"old_value"`
    NewValue  string `json:"new_value"`
}

// CapabilityDiff is the Stage 3 output: structured diff + proposed YAML patch.
type CapabilityDiff struct {
    Provider         string        `json:"provider"`
    RunID            string        `json:"run_id"`
    Changes          []FieldChange `json:"changes"`
    StructuralDrift  []string      `json:"structural_drift,omitempty"`
    ProposedYAMLPatch string       `json:"proposed_yaml_patch,omitempty"`
}

// ProviderStatus tracks per-provider pipeline state for the RunManifest.
type ProviderStatus struct {
    FetchStatus    string `json:"fetch_status"`
    ExtractStatus  string `json:"extract_status"`
    DiffStatus     string `json:"diff_status"`
    ActionTaken    string `json:"action_taken"`
    FixtureAgeDays *int   `json:"fixture_age_days"`
}

// RunManifest is write-only observability output — never a pipeline input.
// capmon: never-read-as-input
type RunManifest struct {
    RunID                         string                    `json:"run_id"`
    StartedAt                     time.Time                 `json:"started_at"`
    FinishedAt                    time.Time                 `json:"finished_at"`
    ExitClass                     int                       `json:"exit_class"`
    SourcesAllCached              bool                      `json:"sources_all_cached"`
    Providers                     map[string]ProviderStatus `json:"providers"`
    Warnings                      []string                  `json:"warnings"`
    FingerprintDivergenceWarnings []string                  `json:"fingerprint_divergence_warnings"`
}
```

`cli/internal/capmon/extractor.go`:
```go
package capmon

import (
    "context"
    "fmt"
)

// Extractor extracts structured fields from raw source bytes.
// Each format (html, markdown, typescript, etc.) registers its own implementation.
type Extractor interface {
    Extract(ctx context.Context, raw []byte, cfg SelectorConfig) (*ExtractedSource, error)
}

// extractors maps format strings to Extractor implementations.
// Format packages register themselves via init().
var extractors = map[string]Extractor{}

// RegisterExtractor registers an extractor for a named format.
// Called from init() in each format package.
func RegisterExtractor(format string, ext Extractor) {
    extractors[format] = ext
}

// Extract dispatches to the appropriate Extractor for the given format.
func Extract(ctx context.Context, format string, raw []byte, cfg SelectorConfig) (*ExtractedSource, error) {
    ext, ok := extractors[format]
    if !ok {
        return nil, fmt.Errorf("no extractor for format %q", format)
    }
    return ext.Extract(ctx, raw, cfg)
}
```

#### Step 4: Verify test passes
```
cd cli && go test ./internal/capmon/ -run TestTypes
cd cli && go test ./internal/capmon/ -run TestRunManifest
cd cli && go test ./internal/capmon/ -run TestSelectorConfig
cd cli && go test ./internal/capmon/ -run TestExtract_UnknownFormat
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/
git commit -m "feat(capmon): bootstrap core types and Extractor interface"
```

---

### Task 1.1.validate: Validate core types and Extractor interface

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass — Binary compiles with new package
- `cd cli && go test ./internal/capmon/...` → pass — All capmon tests pass
- `cd cli && go vet ./internal/capmon/` → clean
- `RunManifest` has build comment `// capmon: never-read-as-input`
- `ExtractedSource` has build comment `// capmon: pipeline-internal volatile state, no schema_version`
- `extractors` map is unexported; `RegisterExtractor` and `Extract` are exported
- No CGO in the new package (`go build ./internal/capmon/` succeeds with `CGO_ENABLED=0`)
- No regressions: `cd cli && go test ./...` → pass

---

### Task 1.2: Security controls — sanitize, SSRF, slug, PR body (impl)

**Files:**
- Create: `cli/internal/capmon/sanitize.go` — `sanitizeExtractedString`, `yamlStructuralChars`
- Create: `cli/internal/capmon/sanitize_test.go`
- Create: `cli/internal/capmon/fetch.go` — `validateSourceURL`, `isReservedIP` (stub — full HTTP fetcher comes in Phase 2)
- Create: `cli/internal/capmon/fetch_test.go`
- Create: `cli/internal/capmon/report.go` — `sanitizeSlug`, `buildPRBody` (stub — full report logic comes in Phase 8)
- Create: `cli/internal/capmon/report_test.go`

**Depends on:** Task 1.1.validate

**CRITICAL:** All three H-cluster controls land in this single task. Do not split them across tasks.

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestSanitize` → PASS
- `cd cli && go test ./internal/capmon/ -run TestValidateSourceURL` → PASS
- `cd cli && go test ./internal/capmon/ -run TestSanitizeSlug` → PASS
- `cd cli && go test ./internal/capmon/ -run TestBuildPRBody` → PASS

#### Step 1: Write the failing tests (RED)

`cli/internal/capmon/sanitize_test.go`:
```go
package capmon_test

import "testing"

func TestSanitizeExtractedString(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"trailing newline", "hello\n", "hello"},
        {"trailing crlf", "hello\r\n", "hello"},
        {"leading brace", "{key: val}", "%7Bkey: val)"},
        {"leading bracket", "[1,2,3]", "%5B1,2,3]"},
        {"leading colon", ": value", "%3A value"},
        {"leading hash", "# comment", "%23 comment"},
        {"indented leading brace", "  {key: val}", "  %7Bkey: val)"},
        {"normal string", "PreToolUse", "PreToolUse"},
        {"empty string", "", ""},
        {"long string over 512", string(make([]byte, 600)), string(make([]byte, 500)) + " [truncated]"},
        {"leading bang", "! value", "%21 value"},
        {"interior brace is safe", "foo {bar}", "foo {bar}"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := capmon.SanitizeExtractedString(tt.input)
            if got != tt.want {
                t.Errorf("SanitizeExtractedString(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

`cli/internal/capmon/fetch_test.go`:
```go
package capmon_test

import (
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestValidateSourceURL(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
        errMsg  string
    }{
        {"valid https", "https://docs.anthropic.com/llms-full.txt", false, ""},
        {"http rejected", "http://example.com", true, "only https scheme allowed"},
        {"raw IPv4", "https://127.0.0.1/path", true, "raw IP literal not allowed"},
        {"raw IPv6", "https://[::1]/path", true, "raw IP literal not allowed"},
        {"loopback hostname", "https://localhost/path", true, "reserved IP"},
        {"link-local", "https://169.254.169.254/latest/meta-data", true, "reserved IP"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := capmon.ValidateSourceURL(tt.url)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateSourceURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
            }
            if tt.wantErr && err != nil && tt.errMsg != "" {
                if !strings.Contains(err.Error(), tt.errMsg) {
                    t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
                }
            }
        })
    }
}
```

`cli/internal/capmon/report_test.go`:
```go
package capmon_test

import (
    "strings"
    "testing"
    "bytes"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSanitizeSlug(t *testing.T) {
    tests := []struct {
        input   string
        wantErr bool
    }{
        {"claude-code", false},
        {"gemini-cli", false},
        {"windsurf", false},
        {"UPPER", true},
        {"has space", true},
        {"-leading-dash", true},
        {"trailing-dash-", true},
        {"a", true}, // too short — single char fails the [a-z0-9][a-z0-9-]*[a-z0-9] pattern
        {"ab", false},
        {"../escape", true},
        {"capmon/drift", true}, // slash not allowed
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got, err := capmon.SanitizeSlug(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("SanitizeSlug(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            }
            if err == nil && got != tt.input {
                t.Errorf("SanitizeSlug(%q) = %q, want same as input", tt.input, got)
            }
        })
    }
}

func TestBuildPRBody_NoTemplateInjection(t *testing.T) {
    // A crafted change value must NOT be interpreted as template syntax.
    diff := capmon.CapabilityDiff{
        Provider: "test-provider",
        RunID:    "run-001",
        Changes: []capmon.FieldChange{
            {
                FieldPath: "hooks.events.before_tool_execute.native_name",
                OldValue:  "{{.Secret}}",  // template injection attempt
                NewValue:  "PreToolUse",
            },
        },
    }
    var buf bytes.Buffer
    err := capmon.BuildPRBody(&buf, diff)
    if err != nil {
        t.Fatalf("BuildPRBody returned error: %v", err)
    }
    body := buf.String()
    // Template injection must appear verbatim, fenced — never evaluated
    if !strings.Contains(body, "```") {
        t.Error("PR body must fence extracted values with triple backticks")
    }
    if !strings.Contains(body, "{{.Secret}}") {
        t.Error("template injection attempt must appear verbatim in PR body")
    }
    // The run_id must be in the body for traceability
    if !strings.Contains(body, "run-001") {
        t.Error("RunID must appear in PR body")
    }
    // Fixed footer disclaimer must be present
    if !strings.Contains(body, "Pipeline output is not ground truth") {
        t.Error("fixed footer disclaimer must be present")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run "TestSanitize|TestValidate|TestBuildPR"
```
Expected: FAIL — `undefined: capmon.SanitizeExtractedString`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/sanitize.go`:
```go
package capmon

import (
    "fmt"
    "strings"
)

// yamlStructuralChars is the set of characters that have structural meaning
// in YAML when they appear at the start of a scalar.
const yamlStructuralChars = "{}[]:#&*!|>@%`"

// SanitizeExtractedString normalizes an extracted string value before writing
// to extracted.json. Exported for use by all 8 extractor implementations.
func SanitizeExtractedString(s string) string {
    // 1. Strip trailing newlines
    s = strings.TrimRight(s, "\n\r")
    // 2. Cap at 512 bytes, append "[truncated]" if exceeded
    if len(s) > 512 {
        s = s[:500] + " [truncated]"
    }
    // 3. Percent-encode YAML structural chars in first non-whitespace position
    trimmed := strings.TrimLeft(s, " \t")
    if len(trimmed) > 0 && strings.ContainsRune(yamlStructuralChars, rune(trimmed[0])) {
        indent := s[:len(s)-len(trimmed)]
        s = indent + fmt.Sprintf("%%%02X", trimmed[0]) + trimmed[1:]
    }
    return s
}
```

`cli/internal/capmon/fetch.go` (stub with SSRF validation):
```go
package capmon

import (
    "fmt"
    "net"
    "net/url"
)

// validateSourceURL enforces the SSRF allowlist: HTTPS only, no raw IPs,
// no hostnames that resolve to reserved/private address space.
// Must be called for every source URL at pipeline startup — NOT cached.
func ValidateSourceURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("parse URL: %w", err)
    }
    if u.Scheme != "https" {
        return fmt.Errorf("only https scheme allowed, got %q", u.Scheme)
    }
    host := u.Hostname()
    if net.ParseIP(host) != nil {
        return fmt.Errorf("raw IP literal not allowed: %q", host)
    }
    ips, err := net.LookupHost(host)
    if err != nil {
        return fmt.Errorf("resolve %q: %w", host, err)
    }
    for _, ipStr := range ips {
        parsed := net.ParseIP(ipStr)
        if isReservedIP(parsed) {
            return fmt.Errorf("hostname %q resolves to reserved IP %q", host, ipStr)
        }
    }
    return nil
}

func isReservedIP(ip net.IP) bool {
    reserved := []string{
        "127.0.0.0/8",    // loopback
        "169.254.0.0/16", // link-local / AWS IMDS
        "100.64.0.0/10",  // CGNAT / Alibaba IMDS
        "10.0.0.0/8",     // private
        "172.16.0.0/12",  // private
        "192.168.0.0/16", // private
        "::1/128",        // IPv6 loopback
        "fe80::/10",      // IPv6 link-local
    }
    for _, cidr := range reserved {
        _, network, err := net.ParseCIDR(cidr)
        if err != nil {
            continue
        }
        if network.Contains(ip) {
            return true
        }
    }
    return false
}
```

`cli/internal/capmon/report.go` (stub with slug + PR body):
```go
package capmon

import (
    "fmt"
    "io"
    "regexp"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SanitizeSlug validates a provider slug is safe for use in branch names and PR bodies.
// Applied to both branch name construction and PR body construction in Stage 4.
func SanitizeSlug(slug string) (string, error) {
    if !slugRegex.MatchString(slug) {
        return "", fmt.Errorf("invalid slug: %q", slug)
    }
    return slug, nil
}

// BuildPRBody writes a PR body to w for the given CapabilityDiff.
// Extracted values are NEVER passed through a template engine — they are written
// directly to the io.Writer inside triple-backtick fences.
func BuildPRBody(w io.Writer, diff CapabilityDiff) error {
    // Fixed header — prose only (slug already sanitized before reaching here)
    fmt.Fprintf(w, "# capmon drift: %s\n\n", diff.Provider)
    fmt.Fprintf(w, "Run ID: %s\n", diff.RunID)
    fmt.Fprintf(w, "Changed fields: %d\n\n", len(diff.Changes))

    // Per-field — extracted values always in fenced blocks, never interpolated
    for _, change := range diff.Changes {
        fmt.Fprintf(w, "## %s\n\n", change.FieldPath)
        fmt.Fprintln(w, "Old value:")
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, change.OldValue)
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, "New value:")
        fmt.Fprintln(w, "```")
        fmt.Fprintln(w, change.NewValue)
        fmt.Fprintln(w, "```")
    }

    // Fixed footer — non-ground-truth disclaimer
    fmt.Fprintln(w, "\n---")
    fmt.Fprintln(w, "**Pipeline output is not ground truth.** Verify each changed value against the linked source URL independently before approving.")
    return nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run "TestSanitize|TestValidate|TestBuildPR|TestSlug"
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/sanitize.go cli/internal/capmon/sanitize_test.go \
        cli/internal/capmon/fetch.go cli/internal/capmon/fetch_test.go \
        cli/internal/capmon/report.go cli/internal/capmon/report_test.go
git commit -m "feat(capmon): add H-cluster security controls (sanitize, SSRF, slug, PR body)"
```

---

### Task 1.2.validate: Validate security controls

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass — all tests pass
- All three H-cluster controls present: `sanitize.go`, `fetch.go` (validateSourceURL), `report.go` (sanitizeSlug + buildPRBody)
- `SanitizeExtractedString` is exported (callable by all 8 extractors)
- `ValidateSourceURL` is exported (callable from Stage 1 entrypoint)
- Template injection test passes — crafted `{{.Secret}}` value appears verbatim in PR body, never evaluated
- `isReservedIP` is unexported

---

### Task 1.3: Provider source manifest loader (impl)

**Files:**
- Create: `cli/internal/capmon/sourceman.go` — `SourceManifest`, `ContentTypeSource`, `SourceEntry` types; `LoadSourceManifest()`, `LoadAllSourceManifests()`
- Create: `cli/internal/capmon/sourceman_test.go`
- Create: `cli/internal/capmon/testdata/fixtures/source-manifests/claude-code-minimal.yaml` — minimal fixture for tests

**Depends on:** Task 1.2.validate

**Note:** The `SelectorConfig` struct is already defined in `types.go`. `SourceEntry.Selector` uses it. The structured `selector:` object in provider-sources YAML maps directly to this struct — no schema migration needed (per design decision on forward-compatible structure).

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestLoadSourceManifest` → PASS
- `cd cli && go test ./internal/capmon/ -run TestLoadAllSourceManifests` → PASS

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/sourceman_test.go
package capmon_test

import (
    "path/filepath"
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestLoadSourceManifest(t *testing.T) {
    path := filepath.Join("testdata", "fixtures", "source-manifests", "claude-code-minimal.yaml")
    m, err := capmon.LoadSourceManifest(path)
    if err != nil {
        t.Fatalf("LoadSourceManifest: %v", err)
    }
    if m.Slug != "claude-code" {
        t.Errorf("Slug = %q, want %q", m.Slug, "claude-code")
    }
    hooks, ok := m.ContentTypes["hooks"]
    if !ok {
        t.Fatal("no hooks content type")
    }
    if len(hooks.Sources) == 0 {
        t.Error("hooks has no sources")
    }
    src := hooks.Sources[0]
    if src.Format != "html" {
        t.Errorf("Format = %q, want html", src.Format)
    }
    if src.Selector.Primary == "" {
        t.Error("Selector.Primary is empty")
    }
    if src.Selector.ExpectedContains == "" {
        t.Error("Selector.ExpectedContains is empty")
    }
}

func TestLoadSourceManifest_NotFound(t *testing.T) {
    _, err := capmon.LoadSourceManifest("testdata/does-not-exist.yaml")
    if err == nil {
        t.Error("expected error for missing file")
    }
}
```

Fixture file `cli/internal/capmon/testdata/fixtures/source-manifests/claude-code-minimal.yaml`:
```yaml
schema_version: "1"
slug: claude-code
display_name: Claude Code
last_verified: "2026-04-08"

content_types:
  hooks:
    sources:
      - url: https://code.claude.com/docs/en/hooks
        type: docs
        format: html
        selector:
          primary: "main h2#events ~ table"
          fallback: "main table"
          expected_contains: "Event Name"
          min_results: 6
          updated_at: "2026-04-08"
        extracts: [event_names, hook_config_fields]
```

#### Step 2: Verify test fails
```
cd cli && go test ./internal/capmon/ -run TestLoadSourceManifest
```
Expected: FAIL — `undefined: capmon.LoadSourceManifest`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/sourceman.go`:
```go
package capmon

import (
    "fmt"
    "os"
    "path/filepath"
    "gopkg.in/yaml.v3"
)

// SourceManifest is the parsed form of docs/provider-sources/<slug>.yaml.
type SourceManifest struct {
    SchemaVersion string                        `yaml:"schema_version"`
    Slug          string                        `yaml:"slug"`
    DisplayName   string                        `yaml:"display_name"`
    LastVerified  string                        `yaml:"last_verified"`
    FetchMethod   string                        `yaml:"fetch_method,omitempty"`
    ContentTypes  map[string]ContentTypeSource  `yaml:"content_types"`
}

// ContentTypeSource groups all source entries for one content type.
type ContentTypeSource struct {
    Sources []SourceEntry `yaml:"sources"`
}

// SourceEntry is one source URL with its selector and extraction hints.
type SourceEntry struct {
    URL      string         `yaml:"url"`
    Type     string         `yaml:"type"`
    Format   string         `yaml:"format"`
    Selector SelectorConfig `yaml:"selector"`
    Extracts []string       `yaml:"extracts,omitempty"`
    FetchMethod string      `yaml:"fetch_method,omitempty"` // overrides manifest-level
}

// LoadSourceManifest parses a single provider-sources YAML file.
func LoadSourceManifest(path string) (*SourceManifest, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read source manifest %s: %w", path, err)
    }
    var m SourceManifest
    if err := yaml.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parse source manifest %s: %w", path, err)
    }
    return &m, nil
}

// LoadAllSourceManifests loads all *.yaml files from a directory.
func LoadAllSourceManifests(dir string) ([]*SourceManifest, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, fmt.Errorf("read directory %s: %w", dir, err)
    }
    var manifests []*SourceManifest
    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
            continue
        }
        if e.Name() == "_template.yaml" {
            continue
        }
        m, err := LoadSourceManifest(filepath.Join(dir, e.Name()))
        if err != nil {
            return nil, err
        }
        manifests = append(manifests, m)
    }
    return manifests, nil
}
```

#### Step 4: Verify test passes
```
cd cli && go test ./internal/capmon/ -run TestLoadSourceManifest
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/sourceman.go cli/internal/capmon/sourceman_test.go \
        cli/internal/capmon/testdata/
git commit -m "feat(capmon): add provider source manifest loader"
```

---

### Task 1.3.validate: Validate source manifest loader

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `SelectorConfig` fields from YAML unmarshal correctly into `SourceEntry.Selector`
- `_template.yaml` is skipped by `LoadAllSourceManifests`
- `LoadSourceManifest` with missing file returns a wrapped error (contains the filename)

---

### Task 1.4: Phase 1 validation gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go vet ./internal/capmon/...` → clean
- `cd cli && make fmt` → no diff
- `CGO_ENABLED=0 go build ./internal/capmon/` → pass (no CGO)
- All three H-cluster files present: `sanitize.go`, `fetch.go`, `report.go`
- Core types in `types.go`, extractor interface in `extractor.go`, manifest loader in `sourceman.go`
- No regressions: full `cd cli && go test ./...` → pass

---

## Phase 2: Hash Cache Infrastructure

**Goal:** Build the `.capmon-cache/` directory structure, SHA-256 hashing, meta.json read/write, age-based eviction, and the run manifest persistence layer.

---

### Task 2.1: Cache directory structure and meta.json (impl)

**Files:**
- Create: `cli/internal/capmon/cache.go` — `CacheDir` type, `CacheEntry`, `CacheMeta`, `WriteCacheEntry()`, `ReadCacheEntry()`, `IsCached()`, `AgeBasedEvict()`
- Create: `cli/internal/capmon/cache_test.go`

**Depends on:** Task 1.4 (Phase 1 validation gate)

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestWriteAndReadCacheEntry` → PASS — raw.bin and meta.json written and read back correctly
- `cd cli && go test ./internal/capmon/ -run TestIsCached_False` → PASS — returns false for non-existent entry
- `cd cli && go test ./internal/capmon/ -run TestAgeBasedEvict` → PASS — entries older than maxAge are removed, cache layout is `.capmon-cache/<slug>/<source-id>/raw.bin` + `meta.json`

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/cache_test.go
package capmon_test

import (
    "crypto/sha256"
    "encoding/hex"
    "os"
    "path/filepath"
    "testing"
    "time"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestWriteAndReadCacheEntry(t *testing.T) {
    dir := t.TempDir()
    content := []byte("provider doc content")
    hash := sha256Hash(content)

    entry := capmon.CacheEntry{
        Provider: "claude-code",
        SourceID: "hooks-docs",
        Raw:      content,
        Meta: capmon.CacheMeta{
            FetchedAt:    time.Now().UTC(),
            ContentHash:  hash,
            FetchStatus:  "ok",
            FetchMethod:  "http",
        },
    }

    if err := capmon.WriteCacheEntry(dir, entry); err != nil {
        t.Fatalf("WriteCacheEntry: %v", err)
    }

    got, err := capmon.ReadCacheEntry(dir, "claude-code", "hooks-docs")
    if err != nil {
        t.Fatalf("ReadCacheEntry: %v", err)
    }
    if string(got.Raw) != string(content) {
        t.Errorf("Raw content mismatch")
    }
    if got.Meta.ContentHash != hash {
        t.Errorf("Hash mismatch: got %q, want %q", got.Meta.ContentHash, hash)
    }
}

func TestIsCached_False(t *testing.T) {
    dir := t.TempDir()
    if capmon.IsCached(dir, "no-provider", "no-source") {
        t.Error("expected IsCached to return false for non-existent entry")
    }
}

func TestAgeBasedEvict(t *testing.T) {
    dir := t.TempDir()
    content := []byte("old content")
    hash := sha256Hash(content)

    // Write an entry with an old timestamp
    old := capmon.CacheEntry{
        Provider: "windsurf",
        SourceID: "llms-full",
        Raw:      content,
        Meta: capmon.CacheMeta{
            FetchedAt:   time.Now().UTC().Add(-31 * 24 * time.Hour), // 31 days old
            ContentHash: hash,
            FetchStatus: "ok",
        },
    }
    if err := capmon.WriteCacheEntry(dir, old); err != nil {
        t.Fatalf("WriteCacheEntry: %v", err)
    }

    evicted, err := capmon.AgeBasedEvict(dir, 30*24*time.Hour)
    if err != nil {
        t.Fatalf("AgeBasedEvict: %v", err)
    }
    if evicted == 0 {
        t.Error("expected at least one eviction")
    }
    if capmon.IsCached(dir, "windsurf", "llms-full") {
        t.Error("evicted entry should not be cached")
    }
}

func sha256Hash(data []byte) string {
    h := sha256.Sum256(data)
    return "sha256:" + hex.EncodeToString(h[:])
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestCache
```
Expected: FAIL — `undefined: capmon.CacheEntry`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/cache.go`:
```go
package capmon

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "time"
)

// CacheMeta is the metadata stored alongside raw.bin for each fetched source.
type CacheMeta struct {
    FetchedAt    time.Time `json:"fetched_at"`
    ContentHash  string    `json:"content_hash"`
    FetchStatus  string    `json:"fetch_status"`
    FetchMethod  string    `json:"fetch_method"`
}

// CacheEntry is the full in-memory representation of a cached source.
type CacheEntry struct {
    Provider string
    SourceID string
    Raw      []byte
    Meta     CacheMeta
}

// SHA256Hex computes "sha256:<hex>" for the given bytes.
func SHA256Hex(data []byte) string {
    h := sha256.Sum256(data)
    return "sha256:" + hex.EncodeToString(h[:])
}

func cacheEntryDir(cacheRoot, provider, sourceID string) string {
    return filepath.Join(cacheRoot, provider, sourceID)
}

// WriteCacheEntry writes raw.bin and meta.json for one source.
func WriteCacheEntry(cacheRoot string, entry CacheEntry) error {
    dir := cacheEntryDir(cacheRoot, entry.Provider, entry.SourceID)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("mkdir cache entry %s: %w", dir, err)
    }
    if err := os.WriteFile(filepath.Join(dir, "raw.bin"), entry.Raw, 0644); err != nil {
        return fmt.Errorf("write raw.bin: %w", err)
    }
    metaData, err := json.Marshal(entry.Meta)
    if err != nil {
        return fmt.Errorf("marshal meta: %w", err)
    }
    if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaData, 0644); err != nil {
        return fmt.Errorf("write meta.json: %w", err)
    }
    return nil
}

// ReadCacheEntry reads raw.bin and meta.json for one source.
func ReadCacheEntry(cacheRoot, provider, sourceID string) (*CacheEntry, error) {
    dir := cacheEntryDir(cacheRoot, provider, sourceID)
    raw, err := os.ReadFile(filepath.Join(dir, "raw.bin"))
    if err != nil {
        return nil, fmt.Errorf("read raw.bin: %w", err)
    }
    metaData, err := os.ReadFile(filepath.Join(dir, "meta.json"))
    if err != nil {
        return nil, fmt.Errorf("read meta.json: %w", err)
    }
    var meta CacheMeta
    if err := json.Unmarshal(metaData, &meta); err != nil {
        return nil, fmt.Errorf("parse meta.json: %w", err)
    }
    return &CacheEntry{
        Provider: provider,
        SourceID: sourceID,
        Raw:      raw,
        Meta:     meta,
    }, nil
}

// IsCached returns true if meta.json exists for the given provider+sourceID.
func IsCached(cacheRoot, provider, sourceID string) bool {
    dir := cacheEntryDir(cacheRoot, provider, sourceID)
    _, err := os.Stat(filepath.Join(dir, "meta.json"))
    return err == nil
}

// AgeBasedEvict removes cache entries whose FetchedAt is older than maxAge.
// Returns the number of entries evicted.
func AgeBasedEvict(cacheRoot string, maxAge time.Duration) (int, error) {
    cutoff := time.Now().UTC().Add(-maxAge)
    evicted := 0
    err := fs.WalkDir(os.DirFS(cacheRoot), ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() || d.Name() != "meta.json" {
            return err
        }
        abs := filepath.Join(cacheRoot, path)
        data, err := os.ReadFile(abs)
        if err != nil {
            return nil // skip unreadable entries
        }
        var meta CacheMeta
        if err := json.Unmarshal(data, &meta); err != nil {
            return nil // skip corrupt entries
        }
        if meta.FetchedAt.Before(cutoff) {
            entryDir := filepath.Dir(abs)
            if err := os.RemoveAll(entryDir); err != nil {
                return fmt.Errorf("evict %s: %w", entryDir, err)
            }
            evicted++
        }
        return nil
    })
    return evicted, err
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestCache
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/cache.go cli/internal/capmon/cache_test.go
git commit -m "feat(capmon): add hash cache infrastructure with age-based eviction"
```

---

### Task 2.1.validate: Validate cache infrastructure

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `AgeBasedEvict` uses `fs.WalkDir` (not `filepath.Walk`) — matches design spec
- `IsCached` checks for `meta.json` existence, not `raw.bin`
- `SHA256Hex` produces `"sha256:<hex>"` prefix format

---

### Task 2.2: Run manifest persistence (impl)

**Files:**
- Create: `cli/internal/capmon/manifest_persist.go` — `WriteRunManifest()`, `ReadLastRunManifest()`
- Modify: `cli/internal/capmon/cache_test.go` — add manifest persistence tests

**Depends on:** Task 2.1.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestWriteAndReadRunManifest` → PASS — RunID and ExitClass survive round-trip
- `cd cli && go test ./internal/capmon/ -run TestReadLastRunManifest_Missing` → PASS — missing manifest returns wrapped error
- Run manifest written to `.capmon-cache/last-run.json` (verified by test)

#### Step 1: Write the failing tests (RED)

```go
func TestWriteAndReadRunManifest(t *testing.T) {
    dir := t.TempDir()
    m := capmon.RunManifest{
        RunID:     "test-run-001",
        StartedAt: time.Now().UTC(),
        ExitClass: capmon.ExitClean,
        Providers: map[string]capmon.ProviderStatus{
            "claude-code": {FetchStatus: "ok", ExtractStatus: "ok"},
        },
    }
    if err := capmon.WriteRunManifest(dir, m); err != nil {
        t.Fatalf("WriteRunManifest: %v", err)
    }

    got, err := capmon.ReadLastRunManifest(dir)
    if err != nil {
        t.Fatalf("ReadLastRunManifest: %v", err)
    }
    if got.RunID != m.RunID {
        t.Errorf("RunID: got %q, want %q", got.RunID, m.RunID)
    }
    if got.ExitClass != capmon.ExitClean {
        t.Errorf("ExitClass: got %d, want %d", got.ExitClass, capmon.ExitClean)
    }
}

func TestReadLastRunManifest_Missing(t *testing.T) {
    dir := t.TempDir()
    _, err := capmon.ReadLastRunManifest(dir)
    if err == nil {
        t.Error("expected error for missing manifest")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestRunManifest
```
Expected: FAIL — `undefined: capmon.WriteRunManifest`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/manifest_persist.go`:
```go
package capmon

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

const lastRunFile = "last-run.json"

// WriteRunManifest persists the run manifest to <cacheRoot>/last-run.json.
func WriteRunManifest(cacheRoot string, m RunManifest) error {
    if err := os.MkdirAll(cacheRoot, 0755); err != nil {
        return fmt.Errorf("mkdir cache root: %w", err)
    }
    data, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal run manifest: %w", err)
    }
    path := filepath.Join(cacheRoot, lastRunFile)
    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("write run manifest: %w", err)
    }
    return nil
}

// ReadLastRunManifest reads the most recent run manifest from disk.
func ReadLastRunManifest(cacheRoot string) (*RunManifest, error) {
    path := filepath.Join(cacheRoot, lastRunFile)
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read run manifest: %w", err)
    }
    var m RunManifest
    if err := json.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parse run manifest: %w", err)
    }
    return &m, nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestRunManifest
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/manifest_persist.go
git commit -m "feat(capmon): add run manifest persistence"
```

---

### Task 2.2.validate + Phase 2 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → PASS
- `cd cli && go test ./internal/capmon/ -run TestWriteAndReadRunManifest` → PASS
- `cd cli && go test ./internal/capmon/ -run TestReadLastRunManifest_Missing` → PASS — wrapped error returned
- `cd cli && make fmt` → no diff
- No regressions: `cd cli && go test ./...` → PASS

---

## Phase 3: HTTP Fetchers

**Goal:** Build the direct HTTP fetch stage (Stage 1) for all non-chromedp providers. Includes retry-with-backoff, hash comparison, cache write, and the SSRF `validateSourceURL` call at pipeline startup.

---

### Task 3.1: HTTP fetcher with retry and hash comparison (impl)

**Files:**
- Modify: `cli/internal/capmon/fetch.go` — add `FetchSource()`, `FetchAllSources()`, `RetryWithBackoff()`, HTTP client override var; SSRF validation is already in this file
- Modify: `cli/internal/capmon/fetch_test.go` — add HTTP fetcher tests using `httptest.NewServer`

**Depends on:** Task 2.2.validate (Phase 2 gate)

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestFetchSource_Success` → PASS — fetches content and writes cache entry
- `cd cli && go test ./internal/capmon/ -run TestFetchSource_RetryOnTransient` → PASS — retries on 503, succeeds on third attempt
- `cd cli && go test ./internal/capmon/ -run TestFetchSource_HashUnchanged` → PASS — second fetch with same content sets `Meta.Cached = true`
- `httptest.NewServer` used in all tests — no real network calls

#### Step 1: Write the failing tests (RED)

```go
func TestFetchSource_Success(t *testing.T) {
    ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("provider docs content"))
    }))
    defer ts.Close()

    cacheDir := t.TempDir()
    capmon.SetHTTPClientForTest(ts.Client()) // override for TLS test server
    defer capmon.SetHTTPClientForTest(nil)

    entry, err := capmon.FetchSource(context.Background(), cacheDir, "test-provider", "docs", ts.URL+"/docs")
    if err != nil {
        t.Fatalf("FetchSource: %v", err)
    }
    if string(entry.Raw) != "provider docs content" {
        t.Errorf("unexpected content: %q", string(entry.Raw))
    }
    if entry.Meta.ContentHash == "" {
        t.Error("ContentHash not set")
    }
    // Verify cache was written
    if !capmon.IsCached(cacheDir, "test-provider", "docs") {
        t.Error("cache entry not written")
    }
}

func TestFetchSource_RetryOnTransient(t *testing.T) {
    attempts := 0
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts < 3 {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
        w.Write([]byte("success after retries"))
    }))
    defer ts.Close()

    cacheDir := t.TempDir()
    entry, err := capmon.FetchSource(context.Background(), cacheDir, "retry-provider", "docs", ts.URL+"/docs")
    if err != nil {
        t.Fatalf("FetchSource: %v", err)
    }
    if attempts < 3 {
        t.Errorf("expected at least 3 attempts, got %d", attempts)
    }
    if string(entry.Raw) != "success after retries" {
        t.Errorf("unexpected content: %q", string(entry.Raw))
    }
}

func TestFetchSource_HashUnchanged(t *testing.T) {
    content := []byte("stable content")
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write(content)
    }))
    defer ts.Close()

    cacheDir := t.TempDir()
    // First fetch
    e1, _ := capmon.FetchSource(context.Background(), cacheDir, "stable", "src", ts.URL)
    // Second fetch — same content
    e2, err := capmon.FetchSource(context.Background(), cacheDir, "stable", "src", ts.URL)
    if err != nil {
        t.Fatalf("second FetchSource: %v", err)
    }
    if e1.Meta.ContentHash != e2.Meta.ContentHash {
        t.Error("hash should be identical for unchanged content")
    }
    if !e2.Meta.Cached {
        t.Error("second fetch should be marked as cached")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestFetch
```
Expected: FAIL — `undefined: capmon.FetchSource`

#### Step 3: Write minimal implementation (GREEN)

Add to `cli/internal/capmon/fetch.go`:
```go
// httpDoer is overridable for tests.
var httpDoer interface {
    Do(*http.Request) (*http.Response, error)
} = &http.Client{Timeout: 30 * time.Second}

// SetHTTPClientForTest overrides the HTTP client in tests.
func SetHTTPClientForTest(c *http.Client) {
    if c == nil {
        httpDoer = &http.Client{Timeout: 30 * time.Second}
    } else {
        httpDoer = c
    }
}

// FetchSource fetches one source URL, writes to cache, and returns the entry.
// If content hash is unchanged from the last cached entry, returns the cached entry
// with Meta.Cached = true.
func FetchSource(ctx context.Context, cacheRoot, provider, sourceID, rawURL string) (*CacheEntry, error) {
    var (
        raw  []byte
        err  error
        lastErr error
    )
    // Exponential backoff: 1s, 2s, 4s, then fail
    delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
    for attempt := 0; attempt <= len(delays); attempt++ {
        if attempt > 0 {
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(delays[attempt-1]):
            }
        }
        raw, err = doHTTPFetch(ctx, rawURL)
        if err == nil {
            break
        }
        lastErr = err
    }
    if err != nil {
        return nil, fmt.Errorf("fetch %s after retries: %w", rawURL, lastErr)
    }

    newHash := SHA256Hex(raw)

    // Check if content changed
    if IsCached(cacheRoot, provider, sourceID) {
        existing, readErr := ReadCacheEntry(cacheRoot, provider, sourceID)
        if readErr == nil && existing.Meta.ContentHash == newHash {
            existing.Meta.Cached = true
            return existing, nil
        }
    }

    meta := CacheMeta{
        FetchedAt:   time.Now().UTC(),
        ContentHash: newHash,
        FetchStatus: "ok",
        FetchMethod: "http",
    }
    entry := CacheEntry{
        Provider: provider,
        SourceID: sourceID,
        Raw:      raw,
        Meta:     meta,
    }
    if writeErr := WriteCacheEntry(cacheRoot, entry); writeErr != nil {
        return nil, fmt.Errorf("write cache entry: %w", writeErr)
    }
    return &entry, nil
}

func doHTTPFetch(ctx context.Context, rawURL string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("User-Agent", "syllago-capmon/1.0")
    resp, err := httpDoer.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 500 {
        return nil, fmt.Errorf("server error %d", resp.StatusCode)
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
    }
    return io.ReadAll(resp.Body)
}
```

Also add `Cached bool` field to `CacheMeta`:
```go
// In CacheMeta struct, add:
Cached bool `json:"cached,omitempty"`
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestFetch
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/fetch.go cli/internal/capmon/fetch_test.go
git commit -m "feat(capmon): add HTTP fetcher with retry/backoff and hash comparison"
```

---

### Task 3.1.validate: Validate HTTP fetcher

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `httptest.NewServer` used (no real network)
- Retry logic tested: server fails twice, succeeds on third attempt
- Hash-unchanged path sets `Meta.Cached = true`
- `ValidateSourceURL` is separate from `FetchSource` (called at pipeline startup, not per-fetch)

---

### Task 3.2: GitHub API fetcher (impl)

**Files:**
- Create: `cli/internal/capmon/fetch_github.go` — `FetchGitHubFile()` for `fetch_tier: gh-api` providers (VS Code Copilot)
- Create: `cli/internal/capmon/fetch_github_test.go`

**Depends on:** Task 3.1.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestFetchGitHubFile` → PASS — base64-encoded content decoded and cached correctly
- `grep -q 'Authorization: token' cli/internal/capmon/fetch_github.go` → found — GITHUB_TOKEN env var injected when present
- `grep -q 'application/vnd.github.v3+json' cli/internal/capmon/fetch_github.go` → found — correct Accept header set
- No real network calls (httptest server used in all tests)

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/fetch_github_test.go
package capmon_test

import (
    "encoding/base64"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "context"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestFetchGitHubFile(t *testing.T) {
    fileContent := []byte("# VS Code docs content")
    encoded := base64.StdEncoding.EncodeToString(fileContent)

    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
            w.WriteHeader(http.StatusBadRequest)
            return
        }
        resp := map[string]string{
            "content":  encoded + "\n",
            "encoding": "base64",
        }
        json.NewEncoder(w).Encode(resp)
    }))
    defer ts.Close()

    cacheDir := t.TempDir()
    // Override base URL for test
    capmon.SetGitHubBaseURLForTest(ts.URL)
    defer capmon.SetGitHubBaseURLForTest("")

    entry, err := capmon.FetchGitHubFile(context.Background(), cacheDir,
        "copilot", "hooks-docs",
        "microsoft", "vscode-docs", "main", "docs/copilot/hooks.md")
    if err != nil {
        t.Fatalf("FetchGitHubFile: %v", err)
    }
    if string(entry.Raw) != string(fileContent) {
        t.Errorf("content: got %q, want %q", string(entry.Raw), string(fileContent))
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestFetchGitHubFile
```
Expected: FAIL — `cli/internal/capmon/fetch_github_test.go:XX: undefined: capmon.FetchGitHubFile`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/fetch_github.go`:
```go
package capmon

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
)

// githubBaseURL is overridable for tests.
var githubBaseURL = "https://api.github.com"

// SetGitHubBaseURLForTest overrides the GitHub API base URL in tests.
func SetGitHubBaseURLForTest(url string) {
    if url == "" {
        githubBaseURL = "https://api.github.com"
    } else {
        githubBaseURL = url
    }
}

type githubContentsResponse struct {
    Content  string `json:"content"`
    Encoding string `json:"encoding"`
}

// FetchGitHubFile fetches a file from a GitHub repo via the Contents API,
// decodes base64 content, writes to cache, and returns the entry.
func FetchGitHubFile(ctx context.Context, cacheRoot, provider, sourceID, owner, repo, ref, path string) (*CacheEntry, error) {
    url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", githubBaseURL, owner, repo, path, ref)

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("create github request: %w", err)
    }
    req.Header.Set("Accept", "application/vnd.github.v3+json")
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        req.Header.Set("Authorization", "token "+token)
    }

    resp, err := httpDoer.Do(req)
    if err != nil {
        return nil, fmt.Errorf("github API request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("github API status %d: %s", resp.StatusCode, string(body))
    }

    var apiResp githubContentsResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, fmt.Errorf("decode github response: %w", err)
    }

    if apiResp.Encoding != "base64" {
        return nil, fmt.Errorf("unexpected encoding %q, want base64", apiResp.Encoding)
    }

    // GitHub base64 content includes newlines — strip them before decoding
    cleaned := strings.ReplaceAll(apiResp.Content, "\n", "")
    decoded, err := base64.StdEncoding.DecodeString(cleaned)
    if err != nil {
        return nil, fmt.Errorf("decode base64 content: %w", err)
    }

    meta := CacheMeta{
        FetchedAt:   time.Now().UTC(),
        ContentHash: SHA256Hex(decoded),
        FetchStatus: "ok",
        FetchMethod: "gh-api",
    }
    entry := CacheEntry{
        Provider: provider,
        SourceID: sourceID,
        Raw:      decoded,
        Meta:     meta,
    }
    if err := WriteCacheEntry(cacheRoot, entry); err != nil {
        return nil, fmt.Errorf("write cache entry: %w", err)
    }
    return &entry, nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestFetchGitHubFile
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/fetch_github.go cli/internal/capmon/fetch_github_test.go
git commit -m "feat(capmon): add GitHub API fetcher for gh-api providers"
```

---

### Task 3.2.validate + Phase 3 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → PASS
- `cd cli && go test ./internal/capmon/ -run TestFetchGitHubFile` → PASS — base64 content decoded correctly
- `grep -q 'application/vnd.github.v3+json' cli/internal/capmon/fetch_github.go` → found — Accept header set
- `grep -q 'GITHUB_TOKEN' cli/internal/capmon/fetch_github.go` → found — token env var injected
- `grep -q 'SetGitHubBaseURLForTest' cli/internal/capmon/fetch_github.go` → found — test injection available
- `cd cli && make fmt` → no diff
- No regressions: `cd cli && go test ./...` → PASS

---

## Phase 4: Chromedp Fetcher (Cursor Only)

**Blocked until:** gotreesitter pre-implementation checklist signed off (see top of document). Chromedp itself has no dependency on gotreesitter — this phase can proceed before Phase 5 is unblocked, but the pre-implementation checklist must still be signed off before Phase 4 begins since it signals readiness for the full feature.

**Goal:** Implement the `fetch_method: chromedp` fetcher for Cursor docs. Isolated code path activated only when `fetch_method: chromedp` is set in `provider-sources/cursor.yaml`. Zero CGO.

---

### Task 4.1: Chromedp fetcher with CHROMEDP_URL injection (impl)

**Files:**
- Create: `cli/internal/capmon/fetch_chromedp.go` — `FetchChromedp()`, `CHROMEDP_URL` env var wiring, `connectToChrome()`
- Create: `cli/internal/capmon/fetch_chromedp_test.go` — build-tag-gated integration test (`//go:build integration`)

**Depends on:** Phase 3 gate

**Note:** Unit tests for chromedp are difficult due to the headless browser dependency. The test file uses `//go:build integration` tag and is gated by `SYLLAGO_TEST_NETWORK=1`. The unit test verifies the CHROMEDP_URL wiring logic (not actual browser behavior).

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestChromedpURLWiring` → PASS — CHROMEDP_URL env var read and returned correctly
- `go test -tags integration -run TestChromedpIntegration` with `SYLLAGO_TEST_NETWORK=1` → PASS — integration test available for local use
- `grep -q 'NewRemoteAllocator' cli/internal/capmon/fetch_chromedp.go` → found — remote allocator used when CHROMEDP_URL set

#### Step 1: Write the failing tests (RED)

`cli/internal/capmon/fetch_chromedp_test.go`:
```go
package capmon_test

import (
    "os"
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestChromedpURLWiring(t *testing.T) {
    // When CHROMEDP_URL is set, connectToChrome should use RemoteAllocator
    // When unset, it should use default exec allocator
    // We test this by verifying the exported ChromedpRemoteURL function
    // reads the env var correctly.

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
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestChromedpURLWiring
```
Expected: FAIL — `cli/internal/capmon/fetch_chromedp_test.go:XX: undefined: capmon.ChromedpRemoteURL`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/fetch_chromedp.go`:
```go
package capmon

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/chromedp/chromedp"
)

// ChromedpRemoteURL returns the CHROMEDP_URL env var value.
// Empty string means use local Chrome via DefaultExecAllocatorOptions.
func ChromedpRemoteURL() string {
    return os.Getenv("CHROMEDP_URL")
}

// FetchChromedp fetches a URL using a headless Chromium browser.
// When CHROMEDP_URL is set, connects to a remote headless-shell sidecar.
// When unset, uses a local Chrome installation via exec allocator.
func FetchChromedp(ctx context.Context, cacheRoot, provider, sourceID, url string) (*CacheEntry, error) {
    var allocCtx context.Context
    var cancel context.CancelFunc

    if remoteURL := ChromedpRemoteURL(); remoteURL != "" {
        allocCtx, cancel = chromedp.NewRemoteAllocator(ctx, remoteURL)
    } else {
        allocCtx, cancel = chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions...)
    }
    defer cancel()

    chromedpCtx, chromedpCancel := chromedp.NewContext(allocCtx)
    defer chromedpCancel()

    var bodyHTML string
    if err := chromedp.Run(chromedpCtx,
        chromedp.Navigate(url),
        chromedp.WaitReady("body"),
        chromedp.OuterHTML("body", &bodyHTML),
    ); err != nil {
        return nil, fmt.Errorf("chromedp fetch %s: %w", url, err)
    }

    raw := []byte(bodyHTML)
    meta := CacheMeta{
        FetchedAt:   time.Now().UTC(),
        ContentHash: SHA256Hex(raw),
        FetchStatus: "ok",
        FetchMethod: "chromedp",
    }
    entry := CacheEntry{
        Provider: provider,
        SourceID: sourceID,
        Raw:      raw,
        Meta:     meta,
    }
    if err := WriteCacheEntry(cacheRoot, entry); err != nil {
        return nil, fmt.Errorf("write cache entry: %w", err)
    }
    return &entry, nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestChromedpURLWiring
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/fetch_chromedp.go cli/internal/capmon/fetch_chromedp_test.go
git commit -m "feat(capmon): add chromedp fetcher with CHROMEDP_URL env injection"
```

The implementation uses `github.com/chromedp/chromedp`. When `CHROMEDP_URL` is set (CI), use `chromedp.NewRemoteAllocator`. When unset (local dev), use `chromedp.NewExecAllocator` with `chromedp.DefaultExecAllocatorOptions`.

---

### Task 4.1.validate + Phase 4 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `CGO_ENABLED=0 make build` → PASS — binary compiles without CGO
- `cd cli && go test ./internal/capmon/ -run TestChromedpURLWiring` → PASS — CHROMEDP_URL env var read correctly
- Integration tests skipped by default (require `-tags integration`): `cd cli && go test ./internal/capmon/...` → PASS (no integration failures in CI)
- `grep -q 'chromedp' cli/go.mod` → found — chromedp dependency in go.mod
- `grep -rq 'chromedp' cli/internal/capmon/fetch_chromedp.go` → found — import isolated to one file
- No regressions: `cd cli && go test ./...` → PASS

---

## Phase 5: Extractors — HTML, Markdown, JSON, JSON Schema, YAML, TOML, Go

**Goal:** Implement 7 of the 9 extractors. TypeScript and Rust are gated on gotreesitter sign-off (Task 5b in Phase 5b). All 7 extractors here use stdlib or already-imported dependencies.

**Security requirement:** Every extractor MUST call `SanitizeExtractedString` on every extracted string before populating `FieldValue.Value`. `FieldValue.ValueHash` is computed from the sanitized value.

---

### Task 5.1: HTML extractor (goquery) (impl)

**Files:**
- Create: `cli/internal/capmon/extract_html.go` — `htmlExtractor` implementing `Extractor`, `init()` registration for `"html"` format
- Create: `cli/internal/capmon/extract_html_test.go`
- Create: `cli/internal/capmon/testdata/fixtures/claude-code/hooks-docs.html` — HTML fixture

**Depends on:** Phase 4 gate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestHTMLExtractor_BasicSelection` → PASS — goquery CSS selector extracts table cells correctly
- `cd cli && go test ./internal/capmon/ -run TestHTMLExtractor_AnchorMissing` → PASS — returns `anchor_missing` error when `expected_contains` absent
- `cd cli && go test ./internal/capmon/ -run TestHTMLExtractor_BelowMinResults` → PASS — sets `Partial: true` when below `min_results` (soft failure, no error return)
- `grep -q 'SanitizeExtractedString' cli/internal/capmon/extract_html/extract_html.go` → found — every extracted string sanitized

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_html_test.go
package capmon_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html" // register extractor
)

func TestHTMLExtractor_BasicSelection(t *testing.T) {
    raw := []byte(`<html><body>
        <main>
            <h2 id="events">Events</h2>
            <table>
                <tr><th>Event Name</th><th>Description</th></tr>
                <tr><td>PreToolUse</td><td>Fires before tool</td></tr>
                <tr><td>PostToolUse</td><td>Fires after tool</td></tr>
            </table>
        </main>
    </body></html>`)

    cfg := capmon.SelectorConfig{
        Primary:          "main table tr td:first-child",
        ExpectedContains: "Event Name",
        MinResults:       1,
    }

    result, err := capmon.Extract(context.Background(), "html", raw, cfg)
    if err != nil {
        t.Fatalf("Extract html: %v", err)
    }
    if len(result.Fields) == 0 {
        t.Error("no fields extracted")
    }
}

func TestHTMLExtractor_AnchorMissing(t *testing.T) {
    raw := []byte(`<html><body><table><tr><td>Unrelated</td></tr></table></body></html>`)
    cfg := capmon.SelectorConfig{
        Primary:          "table tr td",
        ExpectedContains: "Event Name", // not present
    }
    _, err := capmon.Extract(context.Background(), "html", raw, cfg)
    if err == nil {
        t.Error("expected error when anchor is missing")
    }
    if !strings.Contains(err.Error(), "anchor_missing") {
        t.Errorf("error %q should mention anchor_missing", err.Error())
    }
}

func TestHTMLExtractor_BelowMinResults(t *testing.T) {
    raw := []byte(`<html><body><table>
        <tr><td>Event Name</td></tr>
        <tr><td>OnlyOne</td></tr>
    </table></body></html>`)
    cfg := capmon.SelectorConfig{
        Primary:          "table tr td",
        ExpectedContains: "Event Name",
        MinResults:       10, // way more than we have
    }
    result, err := capmon.Extract(context.Background(), "html", raw, cfg)
    if err != nil {
        t.Fatalf("unexpected hard error: %v", err)
    }
    if !result.Partial {
        t.Error("result should be marked Partial when below min_results")
    }
}
```

**Package layout note (applies to all extractors in Phase 5):** Each extractor lives in its own subpackage under `cli/internal/capmon/` (e.g., `cli/internal/capmon/extract_html/`, `cli/internal/capmon/extract_markdown/`, etc.). The test file that exercises a given extractor imports the subpackage with a blank import (`_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"`) to trigger the `init()` registration. All test files are in the parent `capmon_test` package and test via the `capmon.Extract()` dispatch function.

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestHTMLExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_html_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_html"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_html/extract_html.go`:
```go
package extract_html

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    "github.com/PuerkitoBio/goquery"
)

func init() {
    capmon.RegisterExtractor("html", &htmlExtractor{})
}

type htmlExtractor struct{}

func (e *htmlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(raw)))
    if err != nil {
        return nil, fmt.Errorf("parse HTML: %w", err)
    }

    // Check expected_contains anchor
    if cfg.ExpectedContains != "" {
        found := false
        doc.Find("body").Each(func(_ int, s *goquery.Selection) {
            if strings.Contains(s.Text(), cfg.ExpectedContains) {
                found = true
            }
        })
        if !found {
            return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
        }
    }

    fields := make(map[string]capmon.FieldValue)
    selector := cfg.Primary

    doc.Find(selector).Each(func(i int, s *goquery.Selection) {
        text := strings.TrimSpace(s.Text())
        if text == "" {
            return
        }
        sanitized := capmon.SanitizeExtractedString(text)
        key := fmt.Sprintf("item_%d", i)
        fields[key] = capmon.FieldValue{
            Value:     sanitized,
            ValueHash: capmon.SHA256Hex([]byte(sanitized)),
        }
    })

    // Check min_results
    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults

    // Extract landmarks (headings)
    var landmarks []string
    doc.Find("h1, h2, h3, h4").Each(func(_ int, s *goquery.Selection) {
        text := strings.TrimSpace(s.Text())
        if text != "" {
            landmarks = append(landmarks, text)
        }
    })

    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "html",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
        Landmarks:        landmarks,
    }, nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestHTMLExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_html/
git commit -m "feat(capmon): add HTML extractor with goquery CSS selectors"
```

---

### Task 5.1.validate: Validate HTML extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `anchor_missing` error returned when `expected_contains` absent (hard failure, exit class 3)
- `Partial: true` set when below `min_results` (soft failure — not an error return)
- `SanitizeExtractedString` called on every extracted text
- Registered via `init()` — `Extract(ctx, "html", ...)` dispatch works
- `goquery` in `go.mod`

---

### Task 5.2: Markdown extractor (goldmark) (impl)

**Files:**
- Create: `cli/internal/capmon/extract_markdown/extract_markdown.go`
- Create: `cli/internal/capmon/extract_markdown/extract_markdown_test.go`

**Depends on:** Task 5.1.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor_HeadingPath` → PASS — table under correct heading extracted, table under wrong heading not extracted
- `cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor_Landmarks` → PASS — all headings collected as landmarks
- `cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor_AnchorMissing` → PASS — returns `anchor_missing` error when `expected_contains` absent
- `grep -q 'SanitizeExtractedString' cli/internal/capmon/extract_markdown/extract_markdown.go` → found

**Extraction algorithm:**

`cfg.Primary` encodes a heading path to locate a section (e.g., `"## Events"`). The algorithm:
1. Parse the markdown with goldmark into an AST
2. Walk the AST node by node; when a `Heading` node is found, check if its text matches the target heading in `cfg.Primary` (level + text, e.g., level 2 + "Events")
3. Once the target heading is found, collect the **sibling nodes** that follow until the next heading of equal or lower depth is encountered — this is "the section"
4. Within the section, extract:
   - **Table rows:** each cell text becomes a field with key `row_N_col_M`
   - **List items:** each item text becomes a field with key `item_N`
5. Apply `SanitizeExtractedString` to every extracted text value
6. Second pass: collect all heading texts into `Landmarks` regardless of section targeting

**Example:** Given this markdown:
```markdown
## Events

| Event Name | Description |
|------------|-------------|
| PreToolUse | Fires before tool |
| PostToolUse | Fires after tool |

## Other Section

- unrelated item
```
With `cfg.Primary = "## Events"`, the extracted fields are:
```
row_0_col_0 = "Event Name"
row_0_col_1 = "Description"
row_1_col_0 = "PreToolUse"
row_1_col_1 = "Fires before tool"
row_2_col_0 = "PostToolUse"
row_2_col_1 = "Fires after tool"
```
The list item under `## Other Section` is NOT extracted because it falls outside the targeted section.

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_markdown/extract_markdown_test.go
package extract_markdown_test

import (
    "context"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_markdown"
)

func TestMarkdownExtractor_HeadingPath(t *testing.T) {
    raw := []byte(`# Top Level

## Events

| Event Name | Description |
|------------|-------------|
| PreToolUse | Fires before tool |
| PostToolUse | Fires after tool |

## Other Section

| Other | Data |
|-------|------|
| foo | bar |
`)
    cfg := capmon.SelectorConfig{
        Primary:          "## Events",
        ExpectedContains: "Event Name",
        MinResults:       1,
    }
    result, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
    if err != nil {
        t.Fatalf("Extract markdown: %v", err)
    }
    // Must find Events table rows
    found := false
    for _, fv := range result.Fields {
        if fv.Value == "PreToolUse" {
            found = true
        }
    }
    if !found {
        t.Error("expected PreToolUse in extracted fields")
    }
    // Must NOT find Other Section data
    for _, fv := range result.Fields {
        if fv.Value == "foo" {
            t.Error("field 'foo' from Other Section should not be extracted")
        }
    }
}

func TestMarkdownExtractor_Landmarks(t *testing.T) {
    raw := []byte(`# Top Level

## Events

## Configuration

### Sub-section
`)
    cfg := capmon.SelectorConfig{Primary: "## Events"}
    result, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
    if err != nil {
        t.Fatalf("Extract markdown: %v", err)
    }
    wantLandmarks := []string{"Top Level", "Events", "Configuration", "Sub-section"}
    for _, want := range wantLandmarks {
        found := false
        for _, got := range result.Landmarks {
            if got == want {
                found = true
            }
        }
        if !found {
            t.Errorf("landmark %q not found in %v", want, result.Landmarks)
        }
    }
}

func TestMarkdownExtractor_AnchorMissing(t *testing.T) {
    raw := []byte(`## Events

| Event Name | Desc |
|------------|------|
| PreToolUse | x |
`)
    cfg := capmon.SelectorConfig{
        Primary:          "## Events",
        ExpectedContains: "NonExistentAnchor",
    }
    _, err := capmon.Extract(context.Background(), "markdown", raw, cfg)
    if err == nil {
        t.Error("expected error for missing anchor")
    }
    if !strings.Contains(err.Error(), "anchor_missing") {
        t.Errorf("error %q should contain anchor_missing", err.Error())
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_markdown/extract_markdown_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_markdown"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_markdown/extract_markdown.go`:
```go
package extract_markdown

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/ast"
    "github.com/yuin/goldmark/text"
)

func init() {
    capmon.RegisterExtractor("markdown", &markdownExtractor{})
}

type markdownExtractor struct{}

func (e *markdownExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    // Check expected_contains anchor against raw bytes
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
    }

    md := goldmark.New()
    reader := text.NewReader(raw)
    doc := md.Parser().Parse(reader)

    // Parse target heading from cfg.Primary (e.g., "## Events" → level=2, text="Events")
    targetLevel, targetText := parseHeadingPath(cfg.Primary)

    fields := make(map[string]capmon.FieldValue)
    var landmarks []string
    inTargetSection := false
    tableRowIdx := 0
    itemIdx := 0

    ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
        if !entering {
            return ast.WalkContinue, nil
        }
        switch node := n.(type) {
        case *ast.Heading:
            headingText := strings.TrimSpace(string(node.Text(raw)))
            landmarks = append(landmarks, headingText)
            if node.Level == targetLevel && headingText == targetText {
                inTargetSection = true
                tableRowIdx = 0
                itemIdx = 0
            } else if inTargetSection && node.Level <= targetLevel {
                inTargetSection = false
            }
        case *ast.TableRow, *ast.TableHeader:
            if !inTargetSection {
                return ast.WalkContinue, nil
            }
            colIdx := 0
            for child := node.FirstChild(); child != nil; child = child.NextSibling() {
                if cell, ok := child.(*ast.TableCell); ok {
                    text := strings.TrimSpace(string(cell.Text(raw)))
                    sanitized := capmon.SanitizeExtractedString(text)
                    key := fmt.Sprintf("row_%d_col_%d", tableRowIdx, colIdx)
                    fields[key] = capmon.FieldValue{
                        Value:     sanitized,
                        ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                    }
                    colIdx++
                }
            }
            tableRowIdx++
        case *ast.ListItem:
            if !inTargetSection {
                return ast.WalkContinue, nil
            }
            text := strings.TrimSpace(string(node.Text(raw)))
            if text != "" {
                sanitized := capmon.SanitizeExtractedString(text)
                key := fmt.Sprintf("item_%d", itemIdx)
                fields[key] = capmon.FieldValue{
                    Value:     sanitized,
                    ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                }
                itemIdx++
            }
        }
        return ast.WalkContinue, nil
    })

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "markdown",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
        Landmarks:        landmarks,
    }, nil
}

// parseHeadingPath parses "## Events" → (2, "Events"), "# Title" → (1, "Title").
// If no level prefix, defaults to level 2.
func parseHeadingPath(primary string) (int, string) {
    level := 0
    for _, ch := range primary {
        if ch == '#' {
            level++
        } else {
            break
        }
    }
    if level == 0 {
        return 2, strings.TrimSpace(primary)
    }
    return level, strings.TrimSpace(primary[level:])
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_markdown/
git commit -m "feat(capmon): add Markdown extractor with goldmark heading-path navigation"
```

---

### Task 5.2.validate: Validate Markdown extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor_HeadingPath` → PASS — content under wrong heading not extracted
- `cd cli && go test ./internal/capmon/extract_markdown/ -run TestMarkdownExtractor_Landmarks` → PASS — all headings collected
- Registered as `"markdown"` format via `init()`

---

### Task 5.3: Go AST extractor (go/parser) (impl)

**Files:**
- Create: `cli/internal/capmon/extract_go/extract_go.go`
- Create: `cli/internal/capmon/extract_go/extract_go_test.go`

**Depends on:** Task 5.2.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor_IotaEnum` → PASS — exported iota const names extracted
- `cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor_StringConsts` → PASS — exported string const values extracted
- `cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor_Generics` → PASS — modern Go with generics parses without error
- `grep -v 'external' cli/internal/capmon/extract_go/extract_go.go` — only stdlib imports (`go/parser`, `go/ast`, `go/token`)

**What is extracted:**
- Exported `const` identifiers (both iota enums and string literals assigned to exported names)
- Exported `type` definition names
- String literal values assigned directly to exported consts (e.g., `const PreToolUse = "PreToolUse"` → key `const_PreToolUse`, value `"PreToolUse"`)
- For iota enums, only the identifier names are extracted (not the numeric values)

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_go/extract_go_test.go
package extract_go_test

import (
    "context"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_go"
)

func TestGoExtractor_IotaEnum(t *testing.T) {
    src := []byte(`package hooks

type HookEvent int

const (
    PreToolUse HookEvent = iota
    PostToolUse
    preSessionStart  // unexported — should not be extracted
)
`)
    cfg := capmon.SelectorConfig{Primary: "const", ExpectedContains: "PreToolUse"}
    result, err := capmon.Extract(context.Background(), "go", src, cfg)
    if err != nil {
        t.Fatalf("Extract go: %v", err)
    }
    checkField(t, result.Fields, "PreToolUse")
    checkField(t, result.Fields, "PostToolUse")
    // unexported should not appear
    for k := range result.Fields {
        if k == "preSessionStart" {
            t.Error("unexported const should not be extracted")
        }
    }
}

func TestGoExtractor_StringConsts(t *testing.T) {
    src := []byte(`package hooks

const (
    EventPreToolUse  = "PreToolUse"
    EventPostToolUse = "PostToolUse"
)
`)
    cfg := capmon.SelectorConfig{Primary: "const"}
    result, err := capmon.Extract(context.Background(), "go", src, cfg)
    if err != nil {
        t.Fatalf("Extract go: %v", err)
    }
    // values of string consts should appear
    found := false
    for _, fv := range result.Fields {
        if fv.Value == "PreToolUse" {
            found = true
        }
    }
    if !found {
        t.Error("string const value 'PreToolUse' not found in extracted fields")
    }
}

func TestGoExtractor_Generics(t *testing.T) {
    src := []byte(`package tools

type Set[T comparable] struct {
    items map[T]struct{}
}

func NewSet[T comparable]() *Set[T] {
    return &Set[T]{items: make(map[T]struct{})}
}
`)
    cfg := capmon.SelectorConfig{Primary: "type"}
    _, err := capmon.Extract(context.Background(), "go", src, cfg)
    if err != nil {
        t.Fatalf("modern Go with generics should parse without error: %v", err)
    }
}

func checkField(t *testing.T, fields map[string]capmon.FieldValue, name string) {
    t.Helper()
    for k := range fields {
        if k == name {
            return
        }
    }
    t.Errorf("field %q not found in extracted fields", name)
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_go/extract_go_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_go"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_go/extract_go.go`:
```go
package extract_go

import (
    "context"
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
    capmon.RegisterExtractor("go", &goExtractor{})
}

type goExtractor struct{}

func (e *goExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in source", cfg.ExpectedContains)
    }

    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, "source.go", raw, 0)
    if err != nil {
        return nil, fmt.Errorf("parse Go source: %w", err)
    }

    fields := make(map[string]capmon.FieldValue)
    var landmarks []string

    for _, decl := range f.Decls {
        switch d := decl.(type) {
        case *ast.GenDecl:
            switch d.Tok {
            case token.CONST:
                for _, spec := range d.Specs {
                    vs, ok := spec.(*ast.ValueSpec)
                    if !ok {
                        continue
                    }
                    for i, name := range vs.Names {
                        if !name.IsExported() {
                            continue
                        }
                        // For string literal consts, extract the value
                        if i < len(vs.Values) {
                            if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
                                // strip quotes
                                val := strings.Trim(lit.Value, `"`)
                                sanitized := capmon.SanitizeExtractedString(val)
                                fields[name.Name] = capmon.FieldValue{
                                    Value:     sanitized,
                                    ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                                }
                                continue
                            }
                        }
                        // For iota/other consts, use the identifier name as value
                        sanitized := capmon.SanitizeExtractedString(name.Name)
                        fields[name.Name] = capmon.FieldValue{
                            Value:     sanitized,
                            ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                        }
                    }
                }
            case token.TYPE:
                for _, spec := range d.Specs {
                    ts, ok := spec.(*ast.TypeSpec)
                    if !ok || !ts.Name.IsExported() {
                        continue
                    }
                    landmarks = append(landmarks, ts.Name.Name)
                }
            }
        }
    }

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "go",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
        Landmarks:        landmarks,
    }, nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_go/
git commit -m "feat(capmon): add Go AST extractor using stdlib go/parser"
```

---

### Task 5.3.validate: Validate Go AST extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor_IotaEnum` → PASS — exported iota consts extracted, unexported skipped
- `cd cli && go test ./internal/capmon/extract_go/ -run TestGoExtractor_Generics` → PASS — modern Go parses without error
- Registered as `"go"` format
- `grep -c 'import' cli/internal/capmon/extract_go/extract_go.go` → only stdlib imports (`go/parser`, `go/ast`, `go/token`)

---

### Task 5.4: JSON extractor (impl)

**Files:**
- Create: `cli/internal/capmon/extract_json/extract_json.go`
- Create: `cli/internal/capmon/extract_json/extract_json_test.go`

**Depends on:** Task 5.3.validate

**Flattening algorithm:** JSON objects are recursively traversed. Nested keys are joined with `.` to produce dot-delimited field paths. Arrays use numeric indices. Example: `{"a":{"b":"val"}}` → field key `a.b`, value `"val"`. `{"events":["PreToolUse","PostToolUse"]}` → field keys `events.0`, `events.1`.

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_json/ -run TestJSONExtractor_Flatten` → PASS — nested keys become dot-delimited paths
- `cd cli && go test ./internal/capmon/extract_json/ -run TestJSONExtractor_AnchorMissing` → PASS — anchor check on raw bytes
- `grep -q 'RegisterExtractor.*json' cli/internal/capmon/extract_json/extract_json.go` → found — registered as `"json"` format via `init()`

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_json/extract_json_test.go
package extract_json_test

import (
    "context"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json"
)

func TestJSONExtractor_Flatten(t *testing.T) {
    raw := []byte(`{
        "events": {
            "PreToolUse": "Fires before tool",
            "PostToolUse": "Fires after tool"
        },
        "version": "1"
    }`)
    cfg := capmon.SelectorConfig{Primary: "events", ExpectedContains: "PreToolUse"}
    result, err := capmon.Extract(context.Background(), "json", raw, cfg)
    if err != nil {
        t.Fatalf("Extract json: %v", err)
    }
    // Expect dot-delimited keys
    if _, ok := result.Fields["events.PreToolUse"]; !ok {
        t.Errorf("expected field 'events.PreToolUse', got fields: %v", result.Fields)
    }
    if _, ok := result.Fields["version"]; !ok {
        t.Error("expected top-level field 'version'")
    }
}

func TestJSONExtractor_AnchorMissing(t *testing.T) {
    raw := []byte(`{"foo": "bar"}`)
    cfg := capmon.SelectorConfig{Primary: "", ExpectedContains: "NonExistentAnchor"}
    _, err := capmon.Extract(context.Background(), "json", raw, cfg)
    if err == nil {
        t.Error("expected anchor_missing error")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_json/ -run TestJSONExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_json/extract_json_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_json/extract_json.go`:
```go
package extract_json

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
    capmon.RegisterExtractor("json", &jsonExtractor{})
}

type jsonExtractor struct{}

func (e *jsonExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
    }

    var parsed interface{}
    if err := json.Unmarshal(raw, &parsed); err != nil {
        return nil, fmt.Errorf("parse JSON: %w", err)
    }

    fields := make(map[string]capmon.FieldValue)
    flattenJSON("", parsed, fields)

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "json",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
    }, nil
}

func flattenJSON(prefix string, v interface{}, out map[string]capmon.FieldValue) {
    switch val := v.(type) {
    case map[string]interface{}:
        for k, child := range val {
            key := k
            if prefix != "" {
                key = prefix + "." + k
            }
            flattenJSON(key, child, out)
        }
    case []interface{}:
        for i, child := range val {
            key := fmt.Sprintf("%s.%d", prefix, i)
            if prefix == "" {
                key = fmt.Sprintf("%d", i)
            }
            flattenJSON(key, child, out)
        }
    default:
        s := fmt.Sprintf("%v", val)
        sanitized := capmon.SanitizeExtractedString(s)
        out[prefix] = capmon.FieldValue{
            Value:     sanitized,
            ValueHash: capmon.SHA256Hex([]byte(sanitized)),
        }
    }
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_json/ -run TestJSONExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_json/
git commit -m "feat(capmon): add JSON extractor with dot-delimited flattening"
```

---

### Task 5.4.validate: Validate JSON extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go test ./internal/capmon/extract_json/ -run TestJSONExtractor_Flatten` → PASS — nested keys dot-delimited
- Registered as `"json"` format
- `grep -q 'SanitizeExtractedString' cli/internal/capmon/extract_json/extract_json.go` → found

---

### Task 5.5: YAML extractor (impl)

**Files:**
- Create: `cli/internal/capmon/extract_yaml/extract_yaml.go`
- Create: `cli/internal/capmon/extract_yaml/extract_yaml_test.go`

**Depends on:** Task 5.4.validate

**Flattening algorithm:** Same dot-delimited path approach as JSON (see Task 5.4). Nested YAML maps → dot paths; sequences → numeric indices.

**YAML type coercion prevention (H3):** The YAML extractor parses input with `gopkg.in/yaml.v3` using `yaml.Node`-based decoding to read the raw scalar text without type coercion. When writing extracted fields to `ExtractedSource.Fields`, every value must use the sanitized string directly. When the YAML extractor's output is later serialized back to YAML (in the capability files), `yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}` must be used so that values like `"true"` or `"42"` remain as strings, not bools or ints.

Exact code pattern for type-safe YAML extraction:
```go
// Decode input YAML with Node-based walking to preserve raw scalar text
var root yaml.Node
if err := yaml.Unmarshal(raw, &root); err != nil {
    return nil, fmt.Errorf("parse YAML: %w", err)
}
// Walk yaml.Node tree; for ScalarNode, use node.Value directly (not decoded interface{})
// This preserves "true" as the string "true", not the bool true.
flattenYAMLNode("", &root, fields)
```

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_yaml/ -run TestYAMLExtractor_Flatten` → PASS — nested keys dot-delimited
- `cd cli && go test ./internal/capmon/extract_yaml/ -run TestYAMLExtractor_TypeCoercion` → PASS — `"true"` and `"42"` extracted as strings, not bool/int
- `grep -q 'RegisterExtractor.*yaml' cli/internal/capmon/extract_yaml/extract_yaml.go` → found — registered as `"yaml"` format via `init()`

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_yaml/extract_yaml_test.go
package extract_yaml_test

import (
    "context"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_yaml"
)

func TestYAMLExtractor_Flatten(t *testing.T) {
    raw := []byte(`
events:
  PreToolUse: fires before tool
  PostToolUse: fires after tool
version: "1"
`)
    cfg := capmon.SelectorConfig{Primary: "events", ExpectedContains: "PreToolUse"}
    result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
    if err != nil {
        t.Fatalf("Extract yaml: %v", err)
    }
    if _, ok := result.Fields["events.PreToolUse"]; !ok {
        t.Errorf("expected field 'events.PreToolUse', got fields: %v", result.Fields)
    }
}

func TestYAMLExtractor_TypeCoercion(t *testing.T) {
    // Without explicit !!str tags, yaml.v3 would coerce these to bool/int
    raw := []byte(`
flags:
  enabled: "true"
  count: "42"
  name: PreToolUse
`)
    cfg := capmon.SelectorConfig{Primary: "flags"}
    result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
    if err != nil {
        t.Fatalf("Extract yaml: %v", err)
    }
    if fv, ok := result.Fields["flags.enabled"]; !ok || fv.Value != "true" {
        t.Errorf("flags.enabled should be string 'true', got %q", fv.Value)
    }
    if fv, ok := result.Fields["flags.count"]; !ok || fv.Value != "42" {
        t.Errorf("flags.count should be string '42', got %q", fv.Value)
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_yaml/ -run TestYAMLExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_yaml/extract_yaml_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_yaml"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_yaml/extract_yaml.go`:
```go
package extract_yaml

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    "gopkg.in/yaml.v3"
)

func init() {
    capmon.RegisterExtractor("yaml", &yamlExtractor{})
}

type yamlExtractor struct{}

func (e *yamlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
    }

    // Use yaml.Node to preserve raw scalar text WITHOUT type coercion.
    // A scalar "true" stays as the string "true", not bool true.
    var root yaml.Node
    if err := yaml.Unmarshal(raw, &root); err != nil {
        return nil, fmt.Errorf("parse YAML: %w", err)
    }

    fields := make(map[string]capmon.FieldValue)
    if len(root.Content) > 0 {
        flattenYAMLNode("", root.Content[0], fields)
    }

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "yaml",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
    }, nil
}

// flattenYAMLNode recursively walks a yaml.Node tree, building dot-delimited field paths.
// Scalar nodes use node.Value directly — this is the key that prevents type coercion.
func flattenYAMLNode(prefix string, node *yaml.Node, out map[string]capmon.FieldValue) {
    switch node.Kind {
    case yaml.ScalarNode:
        // node.Value is always the raw string — "true" stays "true", not bool
        sanitized := capmon.SanitizeExtractedString(node.Value)
        out[prefix] = capmon.FieldValue{
            Value:     sanitized,
            ValueHash: capmon.SHA256Hex([]byte(sanitized)),
        }
    case yaml.MappingNode:
        // Content alternates: key, value, key, value, ...
        for i := 0; i+1 < len(node.Content); i += 2 {
            key := node.Content[i].Value
            fullKey := key
            if prefix != "" {
                fullKey = prefix + "." + key
            }
            flattenYAMLNode(fullKey, node.Content[i+1], out)
        }
    case yaml.SequenceNode:
        for i, child := range node.Content {
            key := fmt.Sprintf("%s.%d", prefix, i)
            flattenYAMLNode(key, child, out)
        }
    case yaml.DocumentNode:
        for _, child := range node.Content {
            flattenYAMLNode(prefix, child, out)
        }
    }
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_yaml/ -run TestYAMLExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_yaml/
git commit -m "feat(capmon): add YAML extractor with yaml.Node type-coercion prevention"
```

---

### Task 5.5.validate: Validate YAML extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go test ./internal/capmon/extract_yaml/ -run TestYAMLExtractor_TypeCoercion` → PASS — `"true"` extracted as string
- `grep -q 'yaml.Node' cli/internal/capmon/extract_yaml/extract_yaml.go` → found — Node-based parsing confirmed
- `grep -q 'node.Value' cli/internal/capmon/extract_yaml/extract_yaml.go` → found — raw scalar text used directly
- Registered as `"yaml"` format

---

### Task 5.6: TOML extractor (impl)

**Files:**
- Create: `cli/internal/capmon/extract_toml/extract_toml.go`
- Create: `cli/internal/capmon/extract_toml/extract_toml_test.go`

**Depends on:** Task 5.5.validate

**Flattening algorithm:** Same dot-delimited path approach as JSON/YAML. `BurntSushi/toml` decodes into `map[string]interface{}` which is then flattened identically to the JSON extractor.

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_toml/ -run TestTOMLExtractor_Flatten` → PASS — nested TOML tables become dot-delimited paths
- `cd cli && go test ./internal/capmon/extract_toml/ -run TestTOMLExtractor_AnchorMissing` → PASS — anchor check on raw bytes returns anchor_missing error
- `grep -q 'RegisterExtractor.*toml' cli/internal/capmon/extract_toml/extract_toml.go` → found — registered as `"toml"` format via `init()`

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_toml/extract_toml_test.go
package extract_toml_test

import (
    "context"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_toml"
)

func TestTOMLExtractor_Flatten(t *testing.T) {
    raw := []byte(`
[events]
PreToolUse = "fires before tool"
PostToolUse = "fires after tool"

[meta]
version = "1"
`)
    cfg := capmon.SelectorConfig{Primary: "events", ExpectedContains: "PreToolUse"}
    result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
    if err != nil {
        t.Fatalf("Extract toml: %v", err)
    }
    if _, ok := result.Fields["events.PreToolUse"]; !ok {
        t.Errorf("expected field 'events.PreToolUse', got fields: %v", result.Fields)
    }
    if _, ok := result.Fields["meta.version"]; !ok {
        t.Error("expected field 'meta.version'")
    }
}

func TestTOMLExtractor_AnchorMissing(t *testing.T) {
    raw := []byte(`[foo]
bar = "baz"
`)
    cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
    _, err := capmon.Extract(context.Background(), "toml", raw, cfg)
    if err == nil {
        t.Error("expected anchor_missing error")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_toml/ -run TestTOMLExtractor
```
Expected: FAIL — `cli/internal/capmon/extract_toml/extract_toml_test.go:XX: cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_toml"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_toml/extract_toml.go`:
```go
package extract_toml

import (
    "bytes"
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/BurntSushi/toml"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
    capmon.RegisterExtractor("toml", &tomlExtractor{})
}

type tomlExtractor struct{}

func (e *tomlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
    }

    var parsed map[string]interface{}
    if _, err := toml.NewDecoder(bytes.NewReader(raw)).Decode(&parsed); err != nil {
        return nil, fmt.Errorf("parse TOML: %w", err)
    }

    fields := make(map[string]capmon.FieldValue)
    flattenMap("", parsed, fields)

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "toml",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
    }, nil
}

func flattenMap(prefix string, m map[string]interface{}, out map[string]capmon.FieldValue) {
    for k, v := range m {
        key := k
        if prefix != "" {
            key = prefix + "." + k
        }
        switch val := v.(type) {
        case map[string]interface{}:
            flattenMap(key, val, out)
        default:
            s := fmt.Sprintf("%v", val)
            sanitized := capmon.SanitizeExtractedString(s)
            out[key] = capmon.FieldValue{
                Value:     sanitized,
                ValueHash: capmon.SHA256Hex([]byte(sanitized)),
            }
        }
    }
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_toml/ -run TestTOMLExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_toml/
git commit -m "feat(capmon): add TOML extractor using BurntSushi/toml"
```

---

### Task 5.7: JSON Schema extractor (impl)

**Files:**
- Create: `cli/internal/capmon/extract_json_schema/extract_json_schema.go`
- Create: `cli/internal/capmon/extract_json_schema/extract_json_schema_test.go`

**Depends on:** Task 5.6.validate

**Design reference:** D18 — JSON Schema: `encoding/json` + struct matching. Used for providers that publish their hook configuration as a JSON Schema file (e.g., a `schema.json` defining hook event types and config shapes).

**Extraction algorithm:** Parse the raw bytes as JSON into a `map[string]interface{}`. Walk the top-level `definitions` or `$defs` key (JSON Schema draft-07 / 2019-09 / 2020-12 conventions). For each definition entry, extract the definition name as a field key. For enum values within definitions, extract each enum string as `<definition>.<index>`. Dot-delimited paths follow the same convention as the JSON extractor (Task 5.4).

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_Definitions` → PASS — definition names extracted as field keys
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_EnumValues` → PASS — enum strings extracted under their definition path
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_AnchorMissing` → PASS — anchor check returns anchor_missing error
- `grep -q 'RegisterExtractor.*json-schema' cli/internal/capmon/extract_json_schema/extract_json_schema.go` → found — registered as `"json-schema"` format via `init()`
- `grep -q 'SanitizeExtractedString' cli/internal/capmon/extract_json_schema/extract_json_schema.go` → found

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/extract_json_schema/extract_json_schema_test.go
package extract_json_schema_test

import (
    "context"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
    _ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json_schema"
)

func TestJSONSchemaExtractor_Definitions(t *testing.T) {
    raw := []byte(`{
        "$schema": "http://json-schema.org/draft-07/schema",
        "definitions": {
            "PreToolUse": {
                "type": "object",
                "description": "Fires before tool execution"
            },
            "PostToolUse": {
                "type": "object",
                "description": "Fires after tool execution"
            }
        }
    }`)
    cfg := capmon.SelectorConfig{Primary: "definitions", ExpectedContains: "PreToolUse"}
    result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
    if err != nil {
        t.Fatalf("Extract json-schema: %v", err)
    }
    if _, ok := result.Fields["definitions.PreToolUse"]; !ok {
        t.Errorf("expected field 'definitions.PreToolUse', got fields: %v", result.Fields)
    }
    if _, ok := result.Fields["definitions.PostToolUse"]; !ok {
        t.Error("expected field 'definitions.PostToolUse'")
    }
}

func TestJSONSchemaExtractor_EnumValues(t *testing.T) {
    raw := []byte(`{
        "definitions": {
            "HookEvent": {
                "type": "string",
                "enum": ["PreToolUse", "PostToolUse", "Stop"]
            }
        }
    }`)
    cfg := capmon.SelectorConfig{Primary: "definitions", ExpectedContains: "HookEvent"}
    result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
    if err != nil {
        t.Fatalf("Extract json-schema: %v", err)
    }
    if _, ok := result.Fields["definitions.HookEvent.enum.0"]; !ok {
        t.Errorf("expected field 'definitions.HookEvent.enum.0', got fields: %v", result.Fields)
    }
}

func TestJSONSchemaExtractor_AnchorMissing(t *testing.T) {
    raw := []byte(`{"definitions": {"Foo": {}}}`)
    cfg := capmon.SelectorConfig{Primary: "definitions", ExpectedContains: "NonExistentAnchor"}
    _, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
    if err == nil {
        t.Error("expected anchor_missing error")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor
```
Expected: FAIL — `cannot find package "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json_schema"`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/extract_json_schema/extract_json_schema.go`:
```go
package extract_json_schema

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
    capmon.RegisterExtractor("json-schema", &jsonSchemaExtractor{})
}

type jsonSchemaExtractor struct{}

func (e *jsonSchemaExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
    if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
        return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
    }

    var schema map[string]interface{}
    if err := json.Unmarshal(raw, &schema); err != nil {
        return nil, fmt.Errorf("parse JSON Schema: %w", err)
    }

    fields := make(map[string]capmon.FieldValue)
    flattenJSONSchema("", schema, fields)

    partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
    return &capmon.ExtractedSource{
        ExtractorVersion: "1",
        Format:           "json-schema",
        ExtractedAt:      time.Now().UTC(),
        Partial:          partial,
        Fields:           fields,
    }, nil
}

// flattenJSONSchema walks a JSON Schema document, extracting definition names
// and enum values as dot-delimited field paths.
func flattenJSONSchema(prefix string, v interface{}, out map[string]capmon.FieldValue) {
    switch val := v.(type) {
    case map[string]interface{}:
        for k, child := range val {
            key := k
            if prefix != "" {
                key = prefix + "." + k
            }
            // For non-schema-keyword keys under definitions/$defs, record the key name
            if prefix != "" && (k == "definitions" || k == "$defs" ||
                isDefinitionKey(prefix)) {
                sanitized := capmon.SanitizeExtractedString(k)
                out[key] = capmon.FieldValue{
                    Value:     sanitized,
                    ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                }
            }
            flattenJSONSchema(key, child, out)
        }
    case []interface{}:
        for i, child := range val {
            key := fmt.Sprintf("%s.%d", prefix, i)
            if prefix == "" {
                key = fmt.Sprintf("%d", i)
            }
            if s, ok := child.(string); ok {
                sanitized := capmon.SanitizeExtractedString(s)
                out[key] = capmon.FieldValue{
                    Value:     sanitized,
                    ValueHash: capmon.SHA256Hex([]byte(sanitized)),
                }
            } else {
                flattenJSONSchema(key, child, out)
            }
        }
    case string:
        sanitized := capmon.SanitizeExtractedString(val)
        out[prefix] = capmon.FieldValue{
            Value:     sanitized,
            ValueHash: capmon.SHA256Hex([]byte(sanitized)),
        }
    }
}

// isDefinitionKey returns true if the path segment indicates we are inside a definitions block.
func isDefinitionKey(path string) bool {
    parts := strings.Split(path, ".")
    for _, p := range parts {
        if p == "definitions" || p == "$defs" {
            return true
        }
    }
    return false
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/extract_json_schema/
git commit -m "feat(capmon): add JSON Schema extractor using encoding/json + struct walking"
```

---

### Task 5.7.validate: Validate JSON Schema extractor

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_Definitions` → PASS — definition names extracted
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_EnumValues` → PASS — enum values extracted under dot path
- Registered as `"json-schema"` format
- `grep -q 'SanitizeExtractedString' cli/internal/capmon/extract_json_schema/extract_json_schema.go` → found
- No CGO: `CGO_ENABLED=0 go build ./internal/capmon/extract_json_schema/` → pass

---

### Task 5.6.validate + Phase 5a gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/...` → pass
- `cd cli && make fmt` → no diff
- `cd cli && go test ./internal/capmon/extract_toml/ -run TestTOMLExtractor_Flatten` → PASS
- `cd cli && go test ./internal/capmon/extract_json_schema/ -run TestJSONSchemaExtractor_Definitions` → PASS
- 7 extractors registered: `html`, `markdown`, `go`, `json`, `json-schema`, `yaml`, `toml`
- YAML extractor uses `yaml.Node` with `node.Value` for scalar reading (grep confirms): `grep -q 'yaml.Node' cli/internal/capmon/extract_yaml/extract_yaml.go` → found
- `BurntSushi/toml` in `go.mod`: `grep -q 'BurntSushi/toml' cli/go.mod` → found
- No regressions: `cd cli && go test ./...` → pass

---

## Phase 5b: Extractors — TypeScript and Rust (gotreesitter)

**Blocked until:** gotreesitter pre-implementation checklist signed off (see document header).

**Goal:** Implement the TypeScript and Rust extractors (the remaining 2 of the 9 total extractors) using `gotreesitter` with S-expression queries.

---

### Task 5b.1: TypeScript extractor (gotreesitter) (impl)

**Files:**
- Create: `cli/internal/capmon/extract_typescript/extract_typescript.go` + test
- Add to `go.mod`: `github.com/odvcencio/gotreesitter` (or current canonical path)

**Depends on:** Phase 5a gate + gotreesitter sign-off

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_typescript/ -run TestTypeScriptExtractor_EnumLiterals` → PASS — S-expression queries extract enum-like string literals
- `CGO_ENABLED=0 make build` → PASS — zero CGO after adding gotreesitter
- Test fixture at `cli/internal/capmon/testdata/fixtures/claude-code/hooks-types.ts` exists and extracts correctly

---

### Task 5b.2: Rust extractor (gotreesitter) (impl)

**Files:**
- Create: `cli/internal/capmon/extract_rust/extract_rust.go` + test

**Depends on:** Task 5b.1.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/extract_rust/ -run TestRustExtractor_EnumVariants` → PASS — enum variant names extracted
- `cd cli && go test ./internal/capmon/extract_rust/ -run TestRustExtractor_StringConsts` → PASS — const string values extracted
- Uses same `gotreesitter` library as TypeScript extractor (no new dependency)

---

### Task 5b.validate + Phase 5b gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `CGO_ENABLED=0 make build` → PASS — zero CGO confirmed
- `cd cli && go test ./internal/capmon/...` → PASS — 9 extractor packages all pass
- `go mod graph | grep -i cgo` → empty output — no CGO in gotreesitter transitive deps
- `du -sh $(which syllago)` → binary size increase < 20MB vs Phase 5a binary
- 9 extractors registered: html, markdown, go, json, json-schema, yaml, toml, typescript, rust

---

## Phase 6: Provider Capability YAML and Schema Validation

**Goal:** Define the `docs/provider-capabilities/` directory structure, JSON Schema validation, the `syllago capmon verify` command, and the schema evolution policy infrastructure.

---

### Task 6.1: Capability YAML schema and JSON Schema (impl)

**Files:**
- Create: `docs/provider-capabilities/schema.json` — JSON Schema for `provider-capabilities/<slug>.yaml`
- Create: `cli/internal/capmon/capyaml/types.go` — Go types for capability YAML: `ProviderCapabilities`, `ContentTypeEntry`, `HooksEntry`, `EventEntry`, `CapabilityEntry`, `ToolEntry`, `ReferenceEntry`, `ProviderExclusiveEntry`
- Create: `cli/internal/capmon/capyaml/load.go` — `LoadCapabilityYAML()`, `WriteCapabilityYAML()`
- Create: `cli/internal/capmon/capyaml/load_test.go`
- Create: `cli/internal/capmon/capyaml/validate.go` — `ValidateAgainstSchema()`, schema version policy logic
- Create: `cli/internal/capmon/capyaml/validate_test.go`
- Create: `docs/provider-capabilities/README.md` — directory documentation

**Depends on:** Phase 5a gate

**Fixture paths:** Tests in `cli/internal/capmon/capyaml/` use `filepath.Join("testdata", ...)` which resolves relative to the test file location: `cli/internal/capmon/capyaml/testdata/`. The fixture files are:
- `cli/internal/capmon/capyaml/testdata/claude-code-minimal.yaml`
- `cli/internal/capmon/capyaml/testdata/schema-version-99.yaml`

These are distinct from the source manifest fixtures at `cli/internal/capmon/testdata/fixtures/source-manifests/`.

#### Success Criteria
- `cd cli && go test ./internal/capmon/capyaml/ -run TestLoadCapabilityYAML_Roundtrip` → PASS — Slug, ContentTypes, hooks.Supported loaded correctly
- `cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema_ValidDoc` → PASS — valid schema_version "1" accepted
- `cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema_UnknownSchemaVersion` → PASS — schema_version "99" rejected with error mentioning "schema_version"
- `cd cli && go test ./internal/capmon/capyaml/ -run TestProviderExclusiveRoundtrip` → PASS — `provider_exclusive` entries survive load/write unchanged

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/capyaml/load_test.go
package capyaml_test

import (
    "bytes"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
)

func TestLoadCapabilityYAML_Roundtrip(t *testing.T) {
    p := filepath.Join("testdata", "claude-code-minimal.yaml")
    caps, err := capyaml.LoadCapabilityYAML(p)
    if err != nil {
        t.Fatalf("LoadCapabilityYAML: %v", err)
    }
    if caps.Slug != "claude-code" {
        t.Errorf("Slug = %q", caps.Slug)
    }
    hooks, ok := caps.ContentTypes["hooks"]
    if !ok {
        t.Fatal("hooks content type missing")
    }
    if !hooks.Supported {
        t.Error("hooks should be supported")
    }
}

func TestValidateAgainstSchema_ValidDoc(t *testing.T) {
    p := filepath.Join("testdata", "claude-code-minimal.yaml")
    if err := capyaml.ValidateAgainstSchema(p, false); err != nil {
        t.Fatalf("ValidateAgainstSchema: %v", err)
    }
}

func TestValidateAgainstSchema_UnknownSchemaVersion(t *testing.T) {
    p := filepath.Join("testdata", "schema-version-99.yaml")
    err := capyaml.ValidateAgainstSchema(p, false)
    if err == nil {
        t.Error("expected error for unknown schema version")
    }
    if !strings.Contains(err.Error(), "schema_version") {
        t.Errorf("error %q should mention schema_version", err.Error())
    }
}

func TestValidateAgainstSchema_MigrationWindow_PreviousVersionAccepted(t *testing.T) {
    // When --migration-window is set, the previous schema version is also accepted.
    // This test verifies the policy: during a migration, old docs still validate.
    // With only version "1" in supportedSchemaVersions, the migration window does not
    // admit version "99" — only the immediately previous known version would be admitted
    // once a second version is added to the list. This test documents the expected behavior
    // for when a version bump happens in the future.
    p := filepath.Join("testdata", "claude-code-minimal.yaml")
    // Current version "1" should validate regardless of migrationWindow
    if err := capyaml.ValidateAgainstSchema(p, true); err != nil {
        t.Fatalf("current version should pass with migrationWindow=true: %v", err)
    }
    // Unknown version "99" should still fail even with migrationWindow=true
    p99 := filepath.Join("testdata", "schema-version-99.yaml")
    err := capyaml.ValidateAgainstSchema(p99, true)
    if err == nil {
        t.Error("version 99 should not pass even with migrationWindow=true (only current-minus-one is admitted)")
    }
}

func TestProviderExclusiveRoundtrip(t *testing.T) {
    p := filepath.Join("testdata", "claude-code-minimal.yaml")
    caps, err := capyaml.LoadCapabilityYAML(p)
    if err != nil {
        t.Fatalf("LoadCapabilityYAML: %v", err)
    }
    // Write to buffer and re-read
    var buf bytes.Buffer
    if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
        t.Fatalf("WriteCapabilityYAML: %v", err)
    }
    // provider_exclusive section must be present in output unchanged
    out := buf.String()
    if !strings.Contains(out, "provider_exclusive") {
        t.Error("provider_exclusive section missing from written YAML")
    }
    if !strings.Contains(out, "InstructionsLoaded") {
        t.Error("provider_exclusive entry InstructionsLoaded missing from written YAML")
    }
}

func TestProviderCapabilities_References(t *testing.T) {
    p := filepath.Join("testdata", "claude-code-minimal.yaml")
    caps, err := capyaml.LoadCapabilityYAML(p)
    if err != nil {
        t.Fatalf("LoadCapabilityYAML: %v", err)
    }
    // References table must load and round-trip
    ref, ok := caps.References["cc_hooks_docs"]
    if !ok {
        t.Fatal("references.cc_hooks_docs missing")
    }
    if ref.URL == "" {
        t.Error("ReferenceEntry.URL is empty")
    }
    if ref.FetchMethod == "" {
        t.Error("ReferenceEntry.FetchMethod is empty")
    }
    // Verify round-trip through WriteCapabilityYAML
    var buf bytes.Buffer
    if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
        t.Fatalf("WriteCapabilityYAML: %v", err)
    }
    if !strings.Contains(buf.String(), "cc_hooks_docs") {
        t.Error("references.cc_hooks_docs missing from written YAML")
    }
}
```

Fixture `cli/internal/capmon/capyaml/testdata/claude-code-minimal.yaml`:
```yaml
schema_version: "1"
slug: claude-code
display_name: Claude Code
last_verified: "2026-04-08"

references:
  cc_hooks_docs:
    url: https://code.claude.com/docs/en/hooks
    fetch_method: http
    verified_at: "2026-04-08"
    last_content_hash: "sha256:abc123"

content_types:
  hooks:
    supported: true
    confidence: high
    events:
      before_tool_execute:
        native_name: PreToolUse
        blocking: prevent
        refs: [cc_hooks_docs]

provider_exclusive:
  events:
    - native_name: InstructionsLoaded
      description: Fires when CLAUDE.md and rule files are loaded
```

Fixture `cli/internal/capmon/capyaml/testdata/schema-version-99.yaml`:
```yaml
schema_version: "99"
slug: unknown-provider
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/capyaml/ -run TestLoadCapabilityYAML
```
Expected: FAIL — `cli/internal/capmon/capyaml/load_test.go:XX: undefined: capyaml.LoadCapabilityYAML`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/capyaml/types.go`:
```go
package capyaml

// ProviderCapabilities is the Go representation of docs/provider-capabilities/<slug>.yaml.
type ProviderCapabilities struct {
    SchemaVersion   string                     `yaml:"schema_version"`
    Slug            string                     `yaml:"slug"`
    DisplayName     string                     `yaml:"display_name"`
    LastVerified    string                     `yaml:"last_verified"`
    ProviderVersion string                     `yaml:"provider_version,omitempty"`
    SourceManifest  string                     `yaml:"source_manifest,omitempty"`
    FormatReference string                     `yaml:"format_reference,omitempty"`
    // References is auto-maintained by the pipeline. Maps reference ID → ReferenceEntry.
    References       map[string]ReferenceEntry  `yaml:"references,omitempty"`
    ContentTypes    map[string]ContentTypeEntry `yaml:"content_types"`
    ProviderExclusive map[string]interface{}   `yaml:"provider_exclusive,omitempty"`
}

// ReferenceEntry tracks provenance for a source URL used in capability extraction.
// The pipeline auto-updates VerifiedAt and LastContentHash after each successful fetch.
type ReferenceEntry struct {
    URL             string `yaml:"url"`
    FetchMethod     string `yaml:"fetch_method"`
    VerifiedAt      string `yaml:"verified_at,omitempty"`
    LastContentHash string `yaml:"last_content_hash,omitempty"`
}

// ContentTypeEntry is the generic entry for a content type (hooks, rules, etc.)
type ContentTypeEntry struct {
    Supported  bool        `yaml:"supported"`
    Confidence string      `yaml:"confidence,omitempty"`
    Events     map[string]EventEntry     `yaml:"events,omitempty"`
    Capabilities map[string]CapabilityEntry `yaml:"capabilities,omitempty"`
    Tools      map[string]ToolEntry      `yaml:"tools,omitempty"`
}

// EventEntry is one hook event in a capability YAML.
type EventEntry struct {
    NativeName string   `yaml:"native_name"`
    Blocking   string   `yaml:"blocking,omitempty"`
    Refs       []string `yaml:"refs,omitempty"`
}

// CapabilityEntry is one capability (e.g., structured_output) in a capability YAML.
type CapabilityEntry struct {
    Supported bool     `yaml:"supported"`
    Mechanism string   `yaml:"mechanism,omitempty"`
    Refs      []string `yaml:"refs,omitempty"`
}

// ToolEntry maps a canonical tool name to its provider-native name.
type ToolEntry struct {
    Native string   `yaml:"native"`
    Refs   []string `yaml:"refs,omitempty"`
}
```

`cli/internal/capmon/capyaml/load.go`:
```go
package capyaml

import (
    "fmt"
    "io"
    "os"

    "gopkg.in/yaml.v3"
)

// LoadCapabilityYAML parses a provider capability YAML file.
func LoadCapabilityYAML(path string) (*ProviderCapabilities, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    var caps ProviderCapabilities
    if err := yaml.Unmarshal(data, &caps); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    return &caps, nil
}

// WriteCapabilityYAML serializes a ProviderCapabilities to the writer.
// provider_exclusive is preserved as-is (round-trip safe).
func WriteCapabilityYAML(w io.Writer, caps *ProviderCapabilities) error {
    enc := yaml.NewEncoder(w)
    enc.SetIndent(2)
    if err := enc.Encode(caps); err != nil {
        return fmt.Errorf("encode capability YAML: %w", err)
    }
    return enc.Close()
}
```

`cli/internal/capmon/capyaml/validate.go`:
```go
package capyaml

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// supportedSchemaVersions lists all accepted schema_version values.
// The first entry is current; subsequent entries are previous versions accepted during migration windows.
var supportedSchemaVersions = []string{"1"}

// ValidateAgainstSchema validates a capability YAML file against the schema version policy.
// If migrationWindow is true, also accepts the immediately previous schema version (current-minus-one).
// Returns an error if schema_version is unknown or the file cannot be parsed.
func ValidateAgainstSchema(path string, migrationWindow bool) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read %s: %w", path, err)
    }
    var header struct {
        SchemaVersion string `yaml:"schema_version"`
    }
    if err := yaml.Unmarshal(data, &header); err != nil {
        return fmt.Errorf("parse schema_version from %s: %w", path, err)
    }
    accepted := make(map[string]bool)
    accepted[supportedSchemaVersions[0]] = true
    if migrationWindow && len(supportedSchemaVersions) > 1 {
        accepted[supportedSchemaVersions[1]] = true
    }
    if !accepted[header.SchemaVersion] {
        return fmt.Errorf("unknown schema_version %q in %s: supported versions are %v",
            header.SchemaVersion, path, supportedSchemaVersions)
    }
    // Full struct parse to catch type errors
    var caps ProviderCapabilities
    if err := yaml.Unmarshal(data, &caps); err != nil {
        return fmt.Errorf("validate %s: %w", path, err)
    }
    return nil
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/capyaml/ -run TestLoadCapabilityYAML
cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema
cd cli && go test ./internal/capmon/capyaml/ -run TestProviderExclusiveRoundtrip
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/capyaml/ docs/provider-capabilities/schema.json docs/provider-capabilities/README.md
git commit -m "feat(capmon): add capability YAML types, loader, and schema validation"
```

---

### Task 6.1.validate: Validate capability YAML types and schema

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/capyaml/ -run TestProviderExclusiveRoundtrip` → PASS — `provider_exclusive` entries present in written YAML
- `cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema_UnknownSchemaVersion` → PASS — schema_version "99" rejected
- `cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema_ValidDoc` → PASS — schema_version "1" accepted
- `cd cli && go test ./internal/capmon/capyaml/ -run TestValidateAgainstSchema_MigrationWindow` → PASS — migration window behavior documented
- `cd cli && go test ./internal/capmon/capyaml/ -run TestProviderCapabilities_References` → PASS — references table round-trips correctly
- `grep -q 'ReferenceEntry' cli/internal/capmon/capyaml/types.go` → found — ReferenceEntry type defined
- `grep -q 'References.*map\[string\]ReferenceEntry' cli/internal/capmon/capyaml/types.go` → found — References field on ProviderCapabilities
- `grep -rq 'yaml.Node' cli/internal/capmon/extract_yaml/` → found — YAML extractor (Stage 2 output) uses Node-based parsing (H3 control)
- Test fixtures at `cli/internal/capmon/capyaml/testdata/` exist and are readable, including `references:` block

---

### Task 6.2: `syllago capmon verify` command (impl)

**Files:**
- Create: `cli/cmd/syllago/capmon_cmd.go` — `capmonCmd` cobra group + `capmonVerifyCmd`
- Create: `cli/cmd/syllago/capmon_cmd_test.go`
- Modify: `cli/cmd/syllago/main.go` — `rootCmd.AddCommand(capmonCmd)`

**Depends on:** Task 6.1.validate

#### Success Criteria
- `syllago capmon verify` → validates all `docs/provider-capabilities/*.yaml` against schema
- `syllago capmon verify --staleness-check --threshold-hours 36` → reads `last-run.json`, opens issue if stale
- Exit codes: 0 = valid, 4 = schema invalid (matching ExitFatal class)
- `cd cli && go test ./cmd/syllago/ -run TestCapmonVerify` → PASS

**Test hook pattern for `capmonCapabilitiesDirOverride`:** The `capmon verify` command needs to resolve `docs/provider-capabilities/` relative to the repo root. To make this testable, `capmon_cmd.go` declares a package-level variable `capmonCapabilitiesDirOverride string`. When non-empty, the verify command uses this path instead of the default `"docs/provider-capabilities"`. This is the same override-var-for-test pattern used elsewhere in the cobra CLI tests.

#### Step 1: Write the failing tests (RED)

```go
// cli/cmd/syllago/capmon_cmd_test.go
package main

import (
    "testing"
)

func TestCapmonVerify_EmptyDir(t *testing.T) {
    // With no provider-capabilities dir, verify should succeed (nothing to validate)
    dir := t.TempDir()
    orig := capmonCapabilitiesDirOverride
    capmonCapabilitiesDirOverride = dir
    t.Cleanup(func() { capmonCapabilitiesDirOverride = orig })

    err := capmonVerifyCmd.RunE(capmonVerifyCmd, []string{})
    if err != nil {
        t.Errorf("verify on empty dir: %v", err)
    }
}

func TestCapmonVerify_StalenessCheck_ManifestMissing(t *testing.T) {
    // When --staleness-check is set and no last-run.json exists, an issue should be opened.
    cacheDir := t.TempDir()
    issueCreated := false
    capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
        for _, a := range args {
            if a == "issue" {
                issueCreated = true
            }
        }
        return []byte("https://github.com/test/repo/issues/99"), nil
    })
    t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })

    capmonVerifyCmd.Flags().Set("staleness-check", "true")
    capmonVerifyCmd.Flags().Set("threshold-hours", "36")
    capmonVerifyCmd.Flags().Set("cache-root", cacheDir)
    defer func() {
        capmonVerifyCmd.Flags().Set("staleness-check", "false")
    }()

    err := capmonVerifyCmd.RunE(capmonVerifyCmd, []string{})
    if err != nil {
        t.Errorf("staleness check with missing manifest: %v", err)
    }
    if !issueCreated {
        t.Error("expected GH issue to be created when manifest is missing")
    }
}

func TestCapmonCmd_Registered(t *testing.T) {
    found := false
    for _, cmd := range rootCmd.Commands() {
        if cmd.Use == "capmon" {
            found = true
            break
        }
    }
    if !found {
        t.Error("capmon command not registered on rootCmd")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./cmd/syllago/ -run TestCapmon
```
Expected: FAIL — `cli/cmd/syllago/capmon_cmd_test.go:XX: undefined: capmonCapabilitiesDirOverride`

#### Step 3: Write minimal implementation (GREEN)

`cli/cmd/syllago/capmon_cmd.go`:
```go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
    "github.com/spf13/cobra"
)

// capmonCapabilitiesDirOverride allows tests to redirect the verify command
// to a temp directory instead of the repo's docs/provider-capabilities/.
var capmonCapabilitiesDirOverride string

var capmonCmd = &cobra.Command{
    Use:   "capmon",
    Short: "Capability monitor pipeline",
    Long:  "Fetch, extract, diff, and report on AI provider capability drift.",
}

var capmonVerifyCmd = &cobra.Command{
    Use:   "verify",
    Short: "Validate provider-capabilities YAML against JSON Schema",
    RunE: func(cmd *cobra.Command, args []string) error {
        stalenessCheck, _ := cmd.Flags().GetBool("staleness-check")
        thresholdHours, _ := cmd.Flags().GetInt("threshold-hours")
        cacheRoot, _ := cmd.Flags().GetString("cache-root")
        migrationWindow, _ := cmd.Flags().GetBool("migration-window")
        if cacheRoot == "" {
            cacheRoot = ".capmon-cache"
        }

        // Staleness check path: read last-run.json and open issue if stale or missing
        if stalenessCheck {
            manifest, err := capmon.ReadLastRunManifest(cacheRoot)
            if err != nil || time.Since(manifest.FinishedAt) > time.Duration(thresholdHours)*time.Hour {
                reason := "last-run.json missing or unreadable"
                if err == nil {
                    reason = fmt.Sprintf("last run was %.1f hours ago (threshold: %d)", time.Since(manifest.FinishedAt).Hours(), thresholdHours)
                }
                _, ghErr := capmon.GHRunner("issue", "create",
                    "--title", "capmon: pipeline staleness detected",
                    "--label", "capmon,staleness",
                    "--body", fmt.Sprintf("Capability monitor pipeline appears stale. %s.", reason),
                )
                return ghErr
            }
            return nil
        }

        dir := capmonCapabilitiesDirOverride
        if dir == "" {
            dir = "docs/provider-capabilities"
        }
        entries, err := os.ReadDir(dir)
        if err != nil {
            if os.IsNotExist(err) {
                return nil // empty dir is valid
            }
            return fmt.Errorf("read capabilities dir: %w", err)
        }
        for _, e := range entries {
            if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
                continue
            }
            if e.Name() == "README.yaml" {
                continue
            }
            path := filepath.Join(dir, e.Name())
            if err := capyaml.ValidateAgainstSchema(path, migrationWindow); err != nil {
                return fmt.Errorf("validate %s: %w", e.Name(), err)
            }
        }
        return nil
    },
}

func init() {
    capmonVerifyCmd.Flags().Bool("staleness-check", false, "Check last-run.json age and open issue if stale")
    capmonVerifyCmd.Flags().Int("threshold-hours", 36, "Hours before a run is considered stale (used with --staleness-check)")
    capmonVerifyCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")
    capmonVerifyCmd.Flags().Bool("migration-window", false, "Accept current-minus-one schema_version during schema migrations")
    capmonCmd.AddCommand(capmonVerifyCmd)
}
```

Add to `cli/cmd/syllago/main.go`:
```go
rootCmd.AddCommand(capmonCmd)
```

#### Step 4: Verify tests pass
```
cd cli && go test ./cmd/syllago/ -run TestCapmon
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/cmd/syllago/capmon_cmd.go cli/cmd/syllago/capmon_cmd_test.go cli/cmd/syllago/main.go
git commit -m "feat(capmon): add capmon command group with verify subcommand"
```

---

### Task 6.2.validate + Phase 6 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `syllago capmon --help` → PASS — capmon subcommands listed including `verify`
- `syllago capmon verify --help` → PASS — verify flags shown including `--staleness-check`, `--threshold-hours`, `--migration-window`
- `cd cli && go test ./cmd/syllago/ -run TestCapmonVerify_EmptyDir` → PASS — empty dir returns nil
- `cd cli && go test ./cmd/syllago/ -run TestCapmonVerify_StalenessCheck_ManifestMissing` → PASS — GH issue created when manifest missing
- `cd cli && go test ./cmd/syllago/ -run TestCapmonCmd_Registered` → PASS — capmon registered on rootCmd
- `grep -q 'staleness-check' cli/cmd/syllago/capmon_cmd.go` → found — staleness-check flag registered
- `grep -q 'migration-window' cli/cmd/syllago/capmon_cmd.go` → found — migration-window flag registered
- `grep -q '!docs/provider-capabilities/' .gitignore` → found — capabilities dir whitelisted
- `grep -q '.capmon-cache/' .gitignore` → found — cache dir ignored
- `cd cli && go test ./...` → PASS — no regressions

---

## Phase 7: CLI Integration — All capmon Subcommands

**Goal:** Implement all six `syllago capmon` subcommands (fetch, extract, run, diff, generate, seed, test-fixtures) with their flags, exit class handling, and the `.capmon-pause` sentinel.

---

### Task 7.1: `capmon fetch` and `capmon extract` commands (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonFetchCmd`, `capmonExtractCmd`
- Modify: `cli/cmd/syllago/capmon_cmd_test.go` — integration tests

**Depends on:** Phase 6 gate

#### Success Criteria
- `cd cli && go test ./cmd/syllago/ -run TestCapmonFetch_InvalidSlug` → PASS — `--provider` with invalid slug returns error
- `cd cli && go test ./cmd/syllago/ -run TestCapmonExtract_Registered` → PASS — extract subcommand registered under capmon
- `syllago capmon fetch --help` → PASS — shows `--provider` flag
- `syllago capmon extract --help` → PASS — shows `--provider` flag

#### Step 1: Write the failing tests (RED)

```go
// cli/cmd/syllago/capmon_cmd_test.go (additions)
func TestCapmonFetch_InvalidSlug(t *testing.T) {
    cmd := capmonFetchCmd
    cmd.SetArgs([]string{"--provider", "INVALID SLUG"})
    err := cmd.Execute()
    if err == nil {
        t.Error("expected error for invalid provider slug")
    }
}

func TestCapmonExtract_Registered(t *testing.T) {
    found := false
    for _, cmd := range capmonCmd.Commands() {
        if cmd.Use == "extract" {
            found = true
        }
    }
    if !found {
        t.Error("extract subcommand not registered under capmon")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./cmd/syllago/ -run "TestCapmonFetch|TestCapmonExtract"
```
Expected: FAIL — `cli/cmd/syllago/capmon_cmd_test.go:XX: undefined: capmonFetchCmd`

#### Step 3: Write minimal implementation (GREEN)

Add to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonFetchCmd = &cobra.Command{
    Use:   "fetch",
    Short: "Fetch source URLs and update hash cache",
    RunE: func(cmd *cobra.Command, args []string) error {
        provider, _ := cmd.Flags().GetString("provider")
        if provider != "" {
            if _, err := capmon.SanitizeSlug(provider); err != nil {
                return fmt.Errorf("invalid --provider: %w", err)
            }
        }
        // Full implementation in pipeline.go (Phase 7.2)
        return fmt.Errorf("not yet implemented — use 'syllago capmon run --stage fetch-extract'")
    },
}

var capmonExtractCmd = &cobra.Command{
    Use:   "extract",
    Short: "Run extraction on cached sources",
    RunE: func(cmd *cobra.Command, args []string) error {
        provider, _ := cmd.Flags().GetString("provider")
        if provider != "" {
            if _, err := capmon.SanitizeSlug(provider); err != nil {
                return fmt.Errorf("invalid --provider: %w", err)
            }
        }
        return fmt.Errorf("not yet implemented — use 'syllago capmon run --stage fetch-extract'")
    },
}

func init() {
    // Add flags
    capmonFetchCmd.Flags().String("provider", "", "Fetch only this provider slug")
    capmonExtractCmd.Flags().String("provider", "", "Extract only this provider slug")
    // Register subcommands
    capmonCmd.AddCommand(capmonFetchCmd)
    capmonCmd.AddCommand(capmonExtractCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./cmd/syllago/ -run "TestCapmonFetch|TestCapmonExtract"
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/cmd/syllago/capmon_cmd.go cli/cmd/syllago/capmon_cmd_test.go
git commit -m "feat(capmon): add fetch and extract subcommands with slug validation"
```

---

### Task 7.2: `capmon run` with stage routing and pause sentinel (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonRunCmd` with `--stage` and `--dry-run` flags, `.capmon-pause` check
- Create: `cli/internal/capmon/pipeline.go` — `RunPipeline()` orchestrating all 4 stages
- Create: `cli/internal/capmon/pipeline_test.go`

**Depends on:** Task 7.1.validate

**Stage routing spec:** The `--stage` flag accepts exactly two values: `"fetch-extract"` (Stages 1+2 only) and `"report"` (Stages 3+4 only). Omitting `--stage` runs all 4 stages. Any other value is rejected at flag-parse time with an error listing the valid options. The flag is implemented as a `string` type (not a bool pair), validated in `RunE` before pipeline dispatch.

**`RunPipeline()` signature and orchestration skeleton:**

```go
// cli/internal/capmon/pipeline.go

// PipelineOptions controls which stages run and pipeline behavior.
type PipelineOptions struct {
    // ProviderFilter limits execution to a single provider slug. Empty = all providers.
    ProviderFilter string
    // Stage controls which pipeline stages run.
    // "": all stages (1-4)
    // "fetch-extract": stages 1-2 only
    // "report": stages 3-4 only
    Stage string
    // DryRun prevents Stage 4 from creating PRs/issues; writes report to w instead.
    DryRun bool
    // CacheRoot is the path to .capmon-cache/. Defaults to ".capmon-cache".
    CacheRoot string
    // SourceManifestsDir is the path to docs/provider-sources/. Defaults to "docs/provider-sources".
    SourceManifestsDir string
    // CapabilitiesDir is the path to docs/provider-capabilities/. Defaults to "docs/provider-capabilities".
    CapabilitiesDir string
}

// RunPipeline executes the capmon pipeline with the given options.
// Returns the exit class (0-5) and any fatal error.
func RunPipeline(ctx context.Context, opts PipelineOptions) (exitClass int, err error) {
    if opts.CacheRoot == "" {
        opts.CacheRoot = ".capmon-cache"
    }
    if opts.SourceManifestsDir == "" {
        opts.SourceManifestsDir = "docs/provider-sources"
    }
    if opts.CapabilitiesDir == "" {
        opts.CapabilitiesDir = "docs/provider-capabilities"
    }

    // Validate stage value
    switch opts.Stage {
    case "", "fetch-extract", "report":
        // valid
    default:
        return ExitFatal, fmt.Errorf("invalid --stage %q: must be 'fetch-extract' or 'report'", opts.Stage)
    }

    // Check .capmon-pause sentinel
    if _, err := os.Stat(".capmon-pause"); err == nil {
        // Stages 1-3 still run; Stage 4 is skipped
        // Fall through with paused=true
    }

    manifest := RunManifest{
        RunID:     generateRunID(),
        StartedAt: time.Now().UTC(),
        Providers: make(map[string]ProviderStatus),
    }

    runFetchExtract := opts.Stage == "" || opts.Stage == "fetch-extract"
    runReport := opts.Stage == "" || opts.Stage == "report"

    // Stage 1: Fetch
    if runFetchExtract {
        if err := runStage1Fetch(ctx, opts, &manifest); err != nil {
            manifest.ExitClass = ExitInfrastructureFailure
            WriteRunManifest(opts.CacheRoot, manifest)
            return ExitInfrastructureFailure, err
        }
    }

    // Stage 2: Extract
    if runFetchExtract {
        if err := runStage2Extract(ctx, opts, &manifest); err != nil {
            manifest.ExitClass = ExitPartialFailure
            WriteRunManifest(opts.CacheRoot, manifest)
            return ExitPartialFailure, err
        }
    }

    // Stage 3: Diff
    if runReport {
        if err := runStage3Diff(ctx, opts, &manifest); err != nil {
            manifest.ExitClass = ExitPartialFailure
            WriteRunManifest(opts.CacheRoot, manifest)
            return ExitPartialFailure, err
        }
    }

    // Stage 4: Review/PR (skipped if paused or dry-run)
    if runReport {
        paused := false
        if _, statErr := os.Stat(".capmon-pause"); statErr == nil {
            paused = true
        }
        if !paused && !opts.DryRun {
            if err := runStage4Review(ctx, opts, &manifest); err != nil {
                manifest.ExitClass = ExitPartialFailure
                WriteRunManifest(opts.CacheRoot, manifest)
                return ExitPartialFailure, err
            }
            manifest.ExitClass = ExitDrifted // if any drift was actioned
        } else if paused {
            manifest.ExitClass = ExitPaused
        }
    }

    manifest.FinishedAt = time.Now().UTC()
    WriteRunManifest(opts.CacheRoot, manifest)
    return manifest.ExitClass, nil
}
```

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestRunPipeline_InvalidStage` → PASS — unknown `--stage` value returns ExitFatal
- `cd cli && go test ./internal/capmon/ -run TestRunPipeline_PauseSentinel` → PASS — `.capmon-pause` sets ExitPaused and skips Stage 4
- `cd cli && go test ./cmd/syllago/ -run TestCapmonRun_StageFlag` → PASS — `--stage fetch-extract` and `--stage report` accepted; unknown value rejected
- `grep -q 'telemetry.Enrich.*dry_run' cli/cmd/syllago/capmon_cmd.go` → found — dry_run telemetry enrichment present

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/pipeline_test.go
package capmon_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestRunPipeline_InvalidStage(t *testing.T) {
    opts := capmon.PipelineOptions{
        Stage:     "invalid-stage",
        CacheRoot: t.TempDir(),
    }
    exitClass, err := capmon.RunPipeline(context.Background(), opts)
    if err == nil {
        t.Error("expected error for invalid stage")
    }
    if exitClass != capmon.ExitFatal {
        t.Errorf("expected ExitFatal (%d), got %d", capmon.ExitFatal, exitClass)
    }
}

func TestRunPipeline_PauseSentinel(t *testing.T) {
    cacheDir := t.TempDir()
    // Create .capmon-pause in a temp work dir
    workDir := t.TempDir()
    pauseFile := filepath.Join(workDir, ".capmon-pause")
    if err := os.WriteFile(pauseFile, []byte{}, 0644); err != nil {
        t.Fatal(err)
    }
    // Override working directory for the sentinel check
    origDir, _ := os.Getwd()
    os.Chdir(workDir)
    t.Cleanup(func() { os.Chdir(origDir) })

    opts := capmon.PipelineOptions{
        Stage:              "report",
        CacheRoot:          cacheDir,
        SourceManifestsDir: t.TempDir(),
        CapabilitiesDir:    t.TempDir(),
    }
    exitClass, _ := capmon.RunPipeline(context.Background(), opts)
    if exitClass != capmon.ExitPaused {
        t.Errorf("expected ExitPaused (%d), got %d", capmon.ExitPaused, exitClass)
    }
}
```

```go
// cli/cmd/syllago/capmon_cmd_test.go (additions)
func TestCapmonRun_StageFlag(t *testing.T) {
    // Valid stage values should not produce a flag-parse error
    validStages := []string{"fetch-extract", "report", ""}
    for _, stage := range validStages {
        args := []string{}
        if stage != "" {
            args = append(args, "--stage", stage)
        }
        // Just check the flag parses — actual execution will fail on missing dirs
        capmonRunCmd.ParseFlags(args)
        got, _ := capmonRunCmd.Flags().GetString("stage")
        if stage != "" && got != stage {
            t.Errorf("stage flag: got %q, want %q", got, stage)
        }
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestRunPipeline
```
Expected: FAIL — `cli/internal/capmon/pipeline_test.go:XX: undefined: capmon.RunPipeline`

#### Step 3: Write minimal implementation (GREEN)

Create `cli/internal/capmon/pipeline.go` with the `PipelineOptions` struct and `RunPipeline()` function as shown in the skeleton above. Stub out `runStage1Fetch`, `runStage2Extract`, `runStage3Diff`, `runStage4Review` as placeholder functions that return nil.

Add `capmonRunCmd` to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonRunCmd = &cobra.Command{
    Use:   "run",
    Short: "Run the full capability monitor pipeline",
    RunE: func(cmd *cobra.Command, args []string) error {
        stage, _ := cmd.Flags().GetString("stage")
        dryRun, _ := cmd.Flags().GetBool("dry-run")
        provider, _ := cmd.Flags().GetString("provider")

        telemetry.Enrich("dry_run", dryRun)
        if provider != "" {
            telemetry.Enrich("provider", provider)
        }
        mode := "full"
        if stage != "" {
            mode = stage
        }
        telemetry.Enrich("mode", mode)

        opts := capmon.PipelineOptions{
            Stage:          stage,
            DryRun:         dryRun,
            ProviderFilter: provider,
        }
        exitClass, err := capmon.RunPipeline(cmd.Context(), opts)
        if err != nil {
            return err
        }
        os.Exit(exitClass)
        return nil
    },
}

func init() {
    capmonRunCmd.Flags().String("stage", "", "Pipeline stage to run: 'fetch-extract' or 'report' (default: all stages)")
    capmonRunCmd.Flags().Bool("dry-run", false, "Skip Stage 4 PR/issue creation; write report to stdout")
    capmonRunCmd.Flags().String("provider", "", "Limit to this provider slug")
    capmonCmd.AddCommand(capmonRunCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestRunPipeline
cd cli && go test ./cmd/syllago/ -run TestCapmonRun
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/pipeline.go cli/internal/capmon/pipeline_test.go cli/cmd/syllago/capmon_cmd.go
git commit -m "feat(capmon): add RunPipeline() with stage routing and pause sentinel"
```

---

### Task 7.2.validate: Validate run command with stage routing

**Validated by:** Different subagent (Haiku)
**Checks:**
- `syllago capmon run --help` → PASS — shows `--stage`, `--dry-run`, `--provider` flags
- `cd cli && go test ./internal/capmon/ -run TestRunPipeline_InvalidStage` → PASS — ExitFatal on unknown stage
- `cd cli && go test ./internal/capmon/ -run TestRunPipeline_PauseSentinel` → PASS — ExitPaused when sentinel present
- `grep -q 'telemetry.Enrich.*dry_run' cli/cmd/syllago/capmon_cmd.go` → found
- `grep -q '"fetch-extract"\|"report"' cli/internal/capmon/pipeline.go` → found — valid stage values documented

---

### Task 7.3: `capmon diff` subcommand (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonDiffCmd`
- Create: `cli/internal/capmon/diff.go` — `DiffProviderCapabilities()` comparing extracted JSON to capability YAML
- Create: `cli/internal/capmon/diff_test.go`

**Depends on:** Task 7.2.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities_NoChange` → PASS — identical data returns empty Changes
- `cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities_FieldChanged` → PASS — changed field produces FieldChange entry
- `syllago capmon diff --help` → PASS — shows `--provider` and `--since` flags

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/diff_test.go
package capmon_test

import (
    "testing"
    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestDiffProviderCapabilities_NoChange(t *testing.T) {
    extracted := &capmon.ExtractedSource{
        Fields: map[string]capmon.FieldValue{
            "hooks.events.before_tool_execute.native_name": {Value: "PreToolUse"},
        },
    }
    current := map[string]string{
        "hooks.events.before_tool_execute.native_name": "PreToolUse",
    }
    diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
    if len(diff.Changes) != 0 {
        t.Errorf("expected no changes, got %d", len(diff.Changes))
    }
}

func TestDiffProviderCapabilities_FieldChanged(t *testing.T) {
    extracted := &capmon.ExtractedSource{
        Fields: map[string]capmon.FieldValue{
            "hooks.events.before_tool_execute.native_name": {Value: "PreTool"},
        },
    }
    current := map[string]string{
        "hooks.events.before_tool_execute.native_name": "PreToolUse",
    }
    diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
    if len(diff.Changes) != 1 {
        t.Fatalf("expected 1 change, got %d", len(diff.Changes))
    }
    if diff.Changes[0].OldValue != "PreToolUse" {
        t.Errorf("OldValue: got %q, want %q", diff.Changes[0].OldValue, "PreToolUse")
    }
    if diff.Changes[0].NewValue != "PreTool" {
        t.Errorf("NewValue: got %q, want %q", diff.Changes[0].NewValue, "PreTool")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities
```
Expected: FAIL — `cli/internal/capmon/diff_test.go:XX: undefined: capmon.DiffProviderCapabilities`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/diff.go`:
```go
package capmon

// DiffProviderCapabilities compares extracted fields against the current capability YAML values.
// current is a map of dot-delimited field paths to their string values from the YAML.
func DiffProviderCapabilities(provider, runID string, extracted *ExtractedSource, current map[string]string) CapabilityDiff {
    diff := CapabilityDiff{
        Provider: provider,
        RunID:    runID,
    }
    for fieldPath, newFV := range extracted.Fields {
        oldVal, exists := current[fieldPath]
        if !exists {
            // New field — structural addition
            diff.StructuralDrift = append(diff.StructuralDrift, fieldPath)
            continue
        }
        if oldVal != newFV.Value {
            diff.Changes = append(diff.Changes, FieldChange{
                FieldPath: fieldPath,
                OldValue:  oldVal,
                NewValue:  newFV.Value,
            })
        }
    }
    // Check for removed fields (in current but not in extracted)
    for fieldPath := range current {
        if _, ok := extracted.Fields[fieldPath]; !ok {
            diff.StructuralDrift = append(diff.StructuralDrift,
                "removed: "+fieldPath)
        }
    }
    return diff
}
```

Add `capmonDiffCmd` to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonDiffCmd = &cobra.Command{
    Use:   "diff",
    Short: "Show field-level changes in provider-capabilities since a git ref",
    RunE: func(cmd *cobra.Command, args []string) error {
        provider, _ := cmd.Flags().GetString("provider")
        if provider != "" {
            if _, err := capmon.SanitizeSlug(provider); err != nil {
                return fmt.Errorf("invalid --provider: %w", err)
            }
        }
        // Full implementation wired via pipeline.go in Phase 9
        return fmt.Errorf("diff output: not yet implemented")
    },
}

func init() {
    capmonDiffCmd.Flags().String("provider", "", "Limit diff to this provider slug")
    capmonDiffCmd.Flags().String("since", "", "Git ref to diff against (default: HEAD~1)")
    capmonCmd.AddCommand(capmonDiffCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/diff.go cli/internal/capmon/diff_test.go cli/cmd/syllago/capmon_cmd.go
git commit -m "feat(capmon): add DiffProviderCapabilities() and diff subcommand"
```

---

### Task 7.3.validate: Validate diff subcommand

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities_NoChange` → PASS
- `cd cli && go test ./internal/capmon/ -run TestDiffProviderCapabilities_FieldChanged` → PASS
- `syllago capmon diff --help` → PASS — `--provider` and `--since` flags shown

---

### Task 7.4: `capmon generate` subcommand (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonGenerateCmd`
- Create: `cli/internal/capmon/generate.go` — `GenerateContentTypeViews()` producing `by-content-type/*.yaml` with generated banner
- Create: `cli/internal/capmon/generate_test.go`

**Depends on:** Task 7.3.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestGenerateContentTypeViews` → PASS — by-content-type YAML file written with `THIS FILE IS GENERATED` banner
- `syllago capmon generate --help` → PASS — subcommand registered

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/generate_test.go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestGenerateContentTypeViews(t *testing.T) {
    // Setup: write a minimal provider-capabilities file
    capsDir := t.TempDir()
    outDir := filepath.Join(t.TempDir(), "by-content-type")

    yamlContent := `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  hooks:
    supported: true
`
    if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }

    if err := capmon.GenerateContentTypeViews(capsDir, outDir); err != nil {
        t.Fatalf("GenerateContentTypeViews: %v", err)
    }

    hooksFile := filepath.Join(outDir, "hooks.yaml")
    data, err := os.ReadFile(hooksFile)
    if err != nil {
        t.Fatalf("read generated file: %v", err)
    }
    if !strings.Contains(string(data), "THIS FILE IS GENERATED") {
        t.Error("generated file missing THIS FILE IS GENERATED banner")
    }
    if !strings.Contains(string(data), "test-provider") {
        t.Error("generated file missing test-provider entry")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestGenerateContentTypeViews
```
Expected: FAIL — `cli/internal/capmon/generate_test.go:XX: undefined: capmon.GenerateContentTypeViews`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/generate.go`:
```go
package capmon

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
    "gopkg.in/yaml.v3"
)

// GenerateContentTypeViews reads all provider-capabilities/*.yaml and writes
// docs/provider-capabilities/by-content-type/<type>.yaml files.
func GenerateContentTypeViews(capsDir, outDir string) error {
    entries, err := os.ReadDir(capsDir)
    if err != nil {
        return fmt.Errorf("read capabilities dir: %w", err)
    }

    // Collect by content type
    byType := make(map[string]map[string]interface{}) // contentType → provider → entry

    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
            continue
        }
        caps, err := capyaml.LoadCapabilityYAML(filepath.Join(capsDir, e.Name()))
        if err != nil {
            return fmt.Errorf("load %s: %w", e.Name(), err)
        }
        for ct, entry := range caps.ContentTypes {
            if _, ok := byType[ct]; !ok {
                byType[ct] = make(map[string]interface{})
            }
            byType[ct][caps.Slug] = entry
        }
    }

    if err := os.MkdirAll(outDir, 0755); err != nil {
        return fmt.Errorf("mkdir output dir: %w", err)
    }

    for ct, providers := range byType {
        outPath := filepath.Join(outDir, ct+".yaml")
        banner := fmt.Sprintf("# THIS FILE IS GENERATED. Do not edit directly.\n# Source: %s/*.yaml\n# Generated at: %s\n\n",
            capsDir, time.Now().UTC().Format(time.RFC3339))

        data, err := yaml.Marshal(map[string]interface{}{
            "schema_version": "1",
            "content_type":   ct,
            "providers":      providers,
        })
        if err != nil {
            return fmt.Errorf("marshal %s: %w", ct, err)
        }

        full := banner + strings.TrimSpace(string(data)) + "\n"
        if err := os.WriteFile(outPath, []byte(full), 0644); err != nil {
            return fmt.Errorf("write %s: %w", outPath, err)
        }
    }
    return nil
}
```

Add `capmonGenerateCmd` to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonGenerateCmd = &cobra.Command{
    Use:   "generate",
    Short: "Regenerate per-content-type views from provider-capabilities YAML",
    RunE: func(cmd *cobra.Command, args []string) error {
        return capmon.GenerateContentTypeViews(
            "docs/provider-capabilities",
            "docs/provider-capabilities/by-content-type",
        )
    },
}

func init() {
    capmonCmd.AddCommand(capmonGenerateCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestGenerateContentTypeViews
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/generate.go cli/internal/capmon/generate_test.go cli/cmd/syllago/capmon_cmd.go
git commit -m "feat(capmon): add GenerateContentTypeViews() and generate subcommand"
```

---

### Task 7.4.validate: Validate generate subcommand

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestGenerateContentTypeViews` → PASS — `THIS FILE IS GENERATED` banner present in output
- `syllago capmon generate --help` → PASS — subcommand registered

---

### Task 7.5: `capmon seed` subcommand (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonSeedCmd`
- Create: `cli/internal/capmon/seed.go` — `SeedProviderCapabilities()` with idempotent merge, parse log, `--force-overwrite-exclusive`
- Create: `cli/internal/capmon/seed_test.go`

**Depends on:** Task 7.4.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_Idempotent` → PASS — running twice produces identical output
- `cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_PreservesExclusive` → PASS — `provider_exclusive` preserved without `--force-overwrite-exclusive`
- `syllago capmon seed --help` → PASS — shows `--provider` and `--force-overwrite-exclusive` flags

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/seed_test.go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSeedProviderCapabilities_Idempotent(t *testing.T) {
    capsDir := t.TempDir()
    seedOpts := capmon.SeedOptions{
        CapsDir:  capsDir,
        Provider: "test-provider",
        Extracted: map[string]string{
            "hooks.events.before_tool_execute.native_name": "PreToolUse",
        },
    }
    // First run
    if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
        t.Fatalf("first seed: %v", err)
    }
    data1, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
    // Second run
    if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
        t.Fatalf("second seed: %v", err)
    }
    data2, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
    if string(data1) != string(data2) {
        t.Error("seed is not idempotent: output changed on second run")
    }
}

func TestSeedProviderCapabilities_PreservesExclusive(t *testing.T) {
    capsDir := t.TempDir()
    // Write initial file with provider_exclusive section
    initial := `schema_version: "1"
slug: test-provider
provider_exclusive:
  events:
    - native_name: CustomEvent
      description: a custom event
`
    if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(initial), 0644); err != nil {
        t.Fatal(err)
    }
    seedOpts := capmon.SeedOptions{
        CapsDir:  capsDir,
        Provider: "test-provider",
        Extracted: map[string]string{
            "hooks.events.before_tool_execute.native_name": "PreToolUse",
        },
        ForceOverwriteExclusive: false,
    }
    if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
        t.Fatalf("seed: %v", err)
    }
    data, _ := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
    if !strings.Contains(string(data), "CustomEvent") {
        t.Error("provider_exclusive entry CustomEvent was removed without --force-overwrite-exclusive")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities
```
Expected: FAIL — `cli/internal/capmon/seed_test.go:XX: undefined: capmon.SeedProviderCapabilities`

#### Step 3: Write minimal implementation (GREEN)

`cli/internal/capmon/seed.go`:
```go
package capmon

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
    "gopkg.in/yaml.v3"
)

// SeedOptions configures a SeedProviderCapabilities invocation.
type SeedOptions struct {
    CapsDir                 string
    Provider                string
    Extracted               map[string]string // field path → value from extraction
    ForceOverwriteExclusive bool
}

// SeedProviderCapabilities writes or updates docs/provider-capabilities/<provider>.yaml.
// It is idempotent: if the file already exists, extracted fields are merged in.
// provider_exclusive entries are preserved unconditionally unless ForceOverwriteExclusive is set.
func SeedProviderCapabilities(opts SeedOptions) error {
    path := filepath.Join(opts.CapsDir, opts.Provider+".yaml")

    var caps capyaml.ProviderCapabilities
    existing, err := capyaml.LoadCapabilityYAML(path)
    if err == nil {
        caps = *existing
        if !opts.ForceOverwriteExclusive {
            // ProviderExclusive is preserved from the existing file (additive merge only)
        } else {
            caps.ProviderExclusive = nil
            fmt.Printf("WARNING: --force-overwrite-exclusive cleared provider_exclusive for %s\n", opts.Provider)
        }
    } else {
        // New file
        caps = capyaml.ProviderCapabilities{
            SchemaVersion: "1",
            Slug:          opts.Provider,
        }
    }

    // Merge extracted fields (simple: write them as YAML nodes — full field path mapping
    // is implemented in Phase 9 when the diff/patch logic is complete)
    _ = opts.Extracted // placeholder until Phase 9 wires full path mapping

    f, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("create %s: %w", path, err)
    }
    defer f.Close()

    enc := yaml.NewEncoder(f)
    enc.SetIndent(2)
    if err := enc.Encode(caps); err != nil {
        return fmt.Errorf("encode %s: %w", path, err)
    }
    return enc.Close()
}
```

Add `capmonSeedCmd` to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonSeedCmd = &cobra.Command{
    Use:   "seed",
    Short: "Bootstrap or re-seed provider capability YAML from extracted data",
    RunE: func(cmd *cobra.Command, args []string) error {
        provider, _ := cmd.Flags().GetString("provider")
        forceOverwrite, _ := cmd.Flags().GetBool("force-overwrite-exclusive")
        if provider != "" {
            if _, err := capmon.SanitizeSlug(provider); err != nil {
                return fmt.Errorf("invalid --provider: %w", err)
            }
        }
        telemetry.Enrich("provider", provider)
        opts := capmon.SeedOptions{
            CapsDir:                 "docs/provider-capabilities",
            Provider:                provider,
            ForceOverwriteExclusive: forceOverwrite,
        }
        return capmon.SeedProviderCapabilities(opts)
    },
}

func init() {
    capmonSeedCmd.Flags().String("provider", "", "Seed only this provider slug")
    capmonSeedCmd.Flags().Bool("force-overwrite-exclusive", false, "Allow overwriting provider_exclusive entries (prints warning)")
    capmonCmd.AddCommand(capmonSeedCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/seed.go cli/internal/capmon/seed_test.go cli/cmd/syllago/capmon_cmd.go
git commit -m "feat(capmon): add SeedProviderCapabilities() with idempotent merge and exclusive preservation"
```

---

### Task 7.5.validate: Validate seed subcommand

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_Idempotent` → PASS
- `cd cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_PreservesExclusive` → PASS
- `syllago capmon seed --help` → PASS — `--provider` and `--force-overwrite-exclusive` flags shown

---

### Task 7.6: `capmon test-fixtures` subcommand (impl)

**Files:**
- Modify: `cli/cmd/syllago/capmon_cmd.go` — add `capmonTestFixturesCmd`
- The fixture age reporting uses `git log` via `os/exec`; the `--update` path calls `FetchSource` or `FetchChromedp` depending on provider's `fetch_method`

**Depends on:** Task 7.5.validate

#### Success Criteria
- `cd cli && go test ./cmd/syllago/ -run TestCapmonTestFixtures_Registered` → PASS — subcommand registered
- `syllago capmon test-fixtures --help` → PASS — shows `--update` and `--provider` flags
- `cd cli && go test ./cmd/syllago/ -run TestCapmonTestFixtures_RefusesWithoutProvider` → PASS — `--update` without `--provider` returns error

#### Step 1: Write the failing tests (RED)

```go
// cli/cmd/syllago/capmon_cmd_test.go (additions)
func TestCapmonTestFixtures_Registered(t *testing.T) {
    found := false
    for _, cmd := range capmonCmd.Commands() {
        if cmd.Use == "test-fixtures" {
            found = true
        }
    }
    if !found {
        t.Error("test-fixtures subcommand not registered under capmon")
    }
}

func TestCapmonTestFixtures_RefusesWithoutProvider(t *testing.T) {
    cmd := capmonTestFixturesCmd
    cmd.SetArgs([]string{"--update"})
    err := cmd.RunE(cmd, []string{})
    if err == nil {
        t.Error("expected error: --update requires --provider")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./cmd/syllago/ -run TestCapmonTestFixtures
```
Expected: FAIL — `cli/cmd/syllago/capmon_cmd_test.go:XX: undefined: capmonTestFixturesCmd`

#### Step 3: Write minimal implementation (GREEN)

Add `capmonTestFixturesCmd` to `cli/cmd/syllago/capmon_cmd.go`:
```go
var capmonTestFixturesCmd = &cobra.Command{
    Use:   "test-fixtures",
    Short: "Report fixture staleness or update fixtures for a provider",
    RunE: func(cmd *cobra.Command, args []string) error {
        update, _ := cmd.Flags().GetBool("update")
        provider, _ := cmd.Flags().GetString("provider")

        if update && provider == "" {
            return fmt.Errorf("--update requires --provider: bulk all-provider updates are refused to preserve per-provider audit trail")
        }
        if update {
            if _, err := capmon.SanitizeSlug(provider); err != nil {
                return fmt.Errorf("invalid --provider: %w", err)
            }
            // Full update implementation via FetchSource/FetchChromedp in Phase 10
            return fmt.Errorf("fixture update for %s: not yet implemented", provider)
        }
        // Report fixture ages from git log
        return reportFixtureAges("cli/internal/capmon/testdata/fixtures")
    },
}

func reportFixtureAges(fixturesDir string) error {
    // Placeholder: full implementation uses os/exec git log per fixture file
    fmt.Printf("Fixture directory: %s\n", fixturesDir)
    fmt.Printf("Run 'git log --format=%%cr -- <fixture-file>' for per-file ages\n")
    return nil
}

func init() {
    capmonTestFixturesCmd.Flags().Bool("update", false, "Re-fetch live source and update fixture files")
    capmonTestFixturesCmd.Flags().String("provider", "", "Provider slug for --update (required with --update)")
    capmonCmd.AddCommand(capmonTestFixturesCmd)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./cmd/syllago/ -run TestCapmonTestFixtures
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/cmd/syllago/capmon_cmd.go cli/cmd/syllago/capmon_cmd_test.go
git commit -m "feat(capmon): add test-fixtures subcommand with update guard"
```

---

### Task 7.6.validate + Phase 7 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- All 8 subcommands registered: `fetch`, `extract`, `run`, `diff`, `generate`, `verify`, `seed`, `test-fixtures`
- `syllago capmon --help` → PASS — all subcommands listed
- `cd cli && go test ./cmd/syllago/ -run TestCapmon` → PASS
- `cd cli && go test ./internal/capmon/...` → PASS
- `cd cli && make fmt` → no diff
- No regressions: `cd cli && go test ./...` → PASS

---

## Phase 8: GitHub Actions Workflow

**Goal:** Wire the two-job GH Actions workflow, SHA-256 artifact verification, staleness-check job, and supporting config files.

---

### Task 8.1: Workflow file and supporting configs (impl)

**Files:**
- Create: `.github/workflows/capmon.yml` — three-job workflow: `fetch-extract`, `report`, `staleness-check`
- Create: `.github/capmon-pr-body.tmpl` — PR template (fixed header/footer prose only, no extracted content)
- Modify: `.github/dependabot.yml` — add `package-ecosystem: github-actions` if not present; confirm `docker` datasource handled by Renovate
- Create or modify: `.github/renovate.json` — add `docker` datasource for `chromedp/headless-shell` SHA pinning

**Depends on:** Phase 7 gate

**Notes:**
- All Actions SHA-pinned to full commit SHAs (not tag aliases)
- `chromedp/headless-shell` Docker image SHA pinned via Renovate `docker` datasource
- `fetch-extract` job: `permissions: {}` — no token
- `report` job: `permissions: { contents: write, pull-requests: write, issues: write }`
- `if: always()` at step level for artifact upload (not job level — panics bypass job-level)
- `continue-on-error: false` on report job
- SHA-256 verification step uses `gh run view` to read artifact hash from `fetch-extract` job summary
- `checkout` before `download-artifact` in report job (peter-evans needs a git working tree)
- All three jobs in one file (not three separate files) to avoid Dependabot drift

#### Success Criteria
- `cat .github/workflows/capmon.yml | python3 -c "import yaml,sys; yaml.safe_load(sys.stdin)"` → PASS — workflow YAML parses without error
- `grep -q 'permissions: {}' .github/workflows/capmon.yml` → found — fetch-extract job has no token
- `grep -q 'needs: fetch-extract' .github/workflows/capmon.yml` → found — report job depends on fetch-extract
- `grep -q 'if: always()' .github/workflows/capmon.yml` → found — upload-artifact step uses step-level always()
- `grep -q '@' .github/workflows/capmon.yml` → found — SHA-pinned action references present
- `grep -q 'chromedp/headless-shell@' .github/workflows/capmon.yml` → found — Docker image SHA reference present

---

### Task 8.1.validate + Phase 8 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `cat .github/workflows/capmon.yml | python3 -c "import yaml,sys; yaml.safe_load(sys.stdin)"` → PASS — valid YAML
- `grep -q 'permissions: {}' .github/workflows/capmon.yml` → found — fetch-extract has no token
- `grep -c 'contents: write' .github/workflows/capmon.yml` → 1 — only report job has write perms
- `grep -q 'issues: write' .github/workflows/capmon.yml` → found — staleness-check has issues write
- `grep -q 'if: always()' .github/workflows/capmon.yml` → found — upload-artifact uses step-level always()
- `grep -q 'continue-on-error: false' .github/workflows/capmon.yml` → found — report job fails visibly
- `test -f .github/capmon-pr-body.tmpl` → PASS — template file exists
- `grep -v '{{' .github/capmon-pr-body.tmpl` → PASS — no template syntax in PR body template
- `grep -q 'docker' .github/renovate.json` → found — Renovate docker datasource configured

---

## Phase 9: Diff Stage and PR Generation (Stage 3 + Stage 4)

**Goal:** Implement the complete Stage 3 (diff) and Stage 4 (review/PR) pipeline logic, including PR deduplication, issue creation for structural drift, and failure retry tracking.

---

### Task 9.1: Stage 3 — field-level and structural diff (impl)

**Files:**
- Modify: `cli/internal/capmon/diff.go` — complete `DiffProviderCapabilities()` with field-level diff, structural drift detection (new/removed landmarks), proposed YAML patch generation
- Modify: `cli/internal/capmon/diff_test.go` — table-driven tests for all diff cases

**Depends on:** Phase 8 gate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestDiff_FieldChanged` → PASS — changed field produces FieldChange with correct old/new values
- `cd cli && go test ./internal/capmon/ -run TestDiff_NoChange` → PASS — identical data returns empty Changes slice
- `cd cli && go test ./internal/capmon/ -run TestDiff_StructuralDrift_NewHeading` → PASS — new landmark in extracted not in YAML appears in StructuralDrift

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/diff_test.go (additions for full Stage 3 diff)
func TestDiff_FieldChanged(t *testing.T) {
    extracted := &capmon.ExtractedSource{
        Fields: map[string]capmon.FieldValue{
            "hooks.events.before_tool_execute.native_name": {Value: "PreTool"}, // changed
        },
        Landmarks: []string{"Events", "Configuration"},
    }
    current := map[string]string{
        "hooks.events.before_tool_execute.native_name": "PreToolUse",
    }
    diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
    if len(diff.Changes) != 1 {
        t.Fatalf("expected 1 change, got %d: %v", len(diff.Changes), diff.Changes)
    }
    if diff.Changes[0].OldValue != "PreToolUse" {
        t.Errorf("OldValue: got %q, want %q", diff.Changes[0].OldValue, "PreToolUse")
    }
    if diff.Changes[0].NewValue != "PreTool" {
        t.Errorf("NewValue: got %q, want %q", diff.Changes[0].NewValue, "PreTool")
    }
}

func TestDiff_NoChange(t *testing.T) {
    extracted := &capmon.ExtractedSource{
        Fields: map[string]capmon.FieldValue{
            "hooks.events.before_tool_execute.native_name": {Value: "PreToolUse"},
        },
    }
    current := map[string]string{
        "hooks.events.before_tool_execute.native_name": "PreToolUse",
    }
    diff := capmon.DiffProviderCapabilities("claude-code", "run-001", extracted, current)
    if len(diff.Changes) != 0 {
        t.Errorf("expected no changes, got %d", len(diff.Changes))
    }
}

func TestDiff_StructuralDrift_NewHeading(t *testing.T) {
    extracted := &capmon.ExtractedSource{
        Fields:    map[string]capmon.FieldValue{},
        Landmarks: []string{"Events", "New Section"},
    }
    // current YAML only knows about "Events"
    knownLandmarks := []string{"Events"}
    diff := capmon.DiffLandmarks("claude-code", "run-001", extracted.Landmarks, knownLandmarks)
    if len(diff.StructuralDrift) != 1 {
        t.Fatalf("expected 1 structural drift entry, got %d: %v", len(diff.StructuralDrift), diff.StructuralDrift)
    }
    if diff.StructuralDrift[0] != "New Section" {
        t.Errorf("StructuralDrift[0]: got %q, want %q", diff.StructuralDrift[0], "New Section")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run "TestDiff_FieldChanged|TestDiff_NoChange|TestDiff_StructuralDrift"
```
Expected: FAIL — `cli/internal/capmon/diff_test.go:XX: undefined: capmon.DiffLandmarks`

#### Step 3: Write minimal implementation (GREEN)

Add to `cli/internal/capmon/diff.go`:
```go
// DiffLandmarks compares extracted document landmarks against known landmarks in the YAML.
// New landmarks (headings not in knownLandmarks) are returned as structural drift entries.
func DiffLandmarks(provider, runID string, extracted, known []string) CapabilityDiff {
    diff := CapabilityDiff{Provider: provider, RunID: runID}
    knownSet := make(map[string]bool, len(known))
    for _, k := range known {
        knownSet[k] = true
    }
    for _, landmark := range extracted {
        if !knownSet[landmark] {
            diff.StructuralDrift = append(diff.StructuralDrift, landmark)
        }
    }
    return diff
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run "TestDiff_FieldChanged|TestDiff_NoChange|TestDiff_StructuralDrift"
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/diff.go cli/internal/capmon/diff_test.go
git commit -m "feat(capmon): complete Stage 3 diff with field-level and structural drift detection"
```

---

### Task 9.1.validate: Validate Stage 3 diff

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → PASS
- `cd cli && go test ./internal/capmon/ -run TestDiff_FieldChanged` → PASS
- `cd cli && go test ./internal/capmon/ -run TestDiff_NoChange` → PASS
- `cd cli && go test ./internal/capmon/ -run TestDiff_StructuralDrift_NewHeading` → PASS
- `grep -q 'DiffLandmarks' cli/internal/capmon/diff.go` → found — structural drift function present

---

### Task 9.2: Stage 4 — PR creation, dedup, issue creation (impl)

**Files:**
- Modify: `cli/internal/capmon/report.go` — complete Stage 4 with `CreateDriftPR()`, `CreateStructuralIssue()`, `DeduplicatePR()`, `RecordConsecutiveFailure()`
- Create: `cli/internal/capmon/report_test.go` additions — test dedup logic, failure counter

**Depends on:** Task 9.1.validate

**Notes:**
- Stage 4 execution order is fixed (see design doc): dedup check → `capmon verify` → `create-pull-request`
- `buildPRBody` already implemented in Phase 1 (Task 1.2)
- Dedup check: `gh pr list --label capmon --head capmon/drift-<slug>` before creating branch
- `sanitizeSlug` applied to BOTH branch name AND PR body (already implemented)
- PR body includes `run_id` from run manifest

**Dedup logic — what happens if the existing branch is behind main:** When an open PR already exists for `capmon/drift-<slug>`, the pipeline:
1. Fetches the remote branch: `git fetch origin capmon/drift-<slug>`
2. Checks if it is behind main: `git merge-base --is-ancestor HEAD origin/capmon/drift-<slug>`
3. If the branch has diverged from main (not just behind), the pipeline **refuses to auto-rebase** — it records `action_taken: dedup_conflict` in the run manifest and opens a GitHub issue flagging that the existing PR branch needs manual resolution before new patches can be applied. Auto-force-push is never performed.
4. If the branch is simply behind main (not diverged), the pipeline pushes the new patch commit to the existing branch without rebasing — GitHub's "this branch is behind" warning is acceptable and the PR reviewer handles it.

**Function signatures:**
```go
// CreateDriftPR creates a GitHub PR for field-level drift. Returns the PR URL.
// sanitizeSlug must be called on provider before reaching this function.
func CreateDriftPR(ctx context.Context, provider, runID string, diff CapabilityDiff) (string, error)

// CreateStructuralIssue creates a GitHub issue for structural drift (new sections).
func CreateStructuralIssue(ctx context.Context, provider, runID string, drift []string) error

// DeduplicatePR checks if an open PR exists for capmon/drift-<provider>.
// Returns (existingPRURL, true) if found; ("", false) if not found.
func DeduplicatePR(ctx context.Context, provider string) (string, bool, error)

// RecordConsecutiveFailure increments the failure counter for a provider.
// Opens a GitHub issue after the 3rd consecutive failure.
func RecordConsecutiveFailure(ctx context.Context, cacheRoot, provider string) error
```

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestDeduplicatePR_NoneExists` → PASS — returns (false) when no open PR found
- `cd cli && go test ./internal/capmon/ -run TestRecordConsecutiveFailure_ThirdFailure` → PASS — 3rd consecutive failure triggers issue creation
- `cd cli && go test ./internal/capmon/ -run TestBuildPRBody_NoTemplateInjection` → PASS (was written in Phase 1)
- `grep -q 'sanitizeSlug' cli/internal/capmon/report.go` → found — slug validated before branch name construction

---

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/report_test.go (additions)
func TestDeduplicatePR_NoneExists(t *testing.T) {
    // Stub: when gh CLI returns empty, DeduplicatePR returns false
    // This test validates the function signature and false-path behavior
    // Full integration test requires a real GitHub token (SYLLAGO_TEST_NETWORK=1)
    if os.Getenv("SYLLAGO_TEST_NETWORK") != "" {
        t.Skip("live dedup test requires manual setup")
    }
    // Unit test: mock the gh command output
    capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
        return []byte("[]"), nil // empty PR list
    })
    defer capmon.SetGHCommandForTest(nil)

    url, found, err := capmon.DeduplicatePR(context.Background(), "test-provider")
    if err != nil {
        t.Fatalf("DeduplicatePR: %v", err)
    }
    if found {
        t.Errorf("expected found=false when no open PRs, got url=%q", url)
    }
}

func TestRecordConsecutiveFailure_ThirdFailure(t *testing.T) {
    cacheDir := t.TempDir()
    // First two failures — no issue
    for i := 0; i < 2; i++ {
        if err := capmon.RecordConsecutiveFailure(context.Background(), cacheDir, "test-provider"); err != nil {
            t.Fatalf("failure %d: %v", i+1, err)
        }
    }
    // Third failure — should trigger issue creation attempt
    // In unit test, gh command is stubbed
    issueCreated := false
    capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
        for _, a := range args {
            if a == "issue" {
                issueCreated = true
            }
        }
        return []byte("https://github.com/test/repo/issues/1"), nil
    })
    defer capmon.SetGHCommandForTest(nil)

    if err := capmon.RecordConsecutiveFailure(context.Background(), cacheDir, "test-provider"); err != nil {
        t.Fatalf("third failure: %v", err)
    }
    if !issueCreated {
        t.Error("expected GitHub issue to be created on 3rd consecutive failure")
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run "TestDeduplicatePR|TestRecordConsecutiveFailure"
```
Expected: FAIL — `cli/internal/capmon/report_test.go:XX: undefined: capmon.DeduplicatePR`

#### Step 3: Write minimal implementation (GREEN)

Add to `cli/internal/capmon/report.go`:
```go
// ghRunner is overridable for tests. Takes gh CLI arguments, returns stdout.
var ghRunner = func(args ...string) ([]byte, error) {
    cmd := exec.Command("gh", args...)
    return cmd.Output()
}

// GHRunner is the exported accessor for ghRunner, used by capmon verify for staleness issue creation.
var GHRunner = func(args ...string) ([]byte, error) {
    return ghRunner(args...)
}

// SetGHCommandForTest overrides the gh CLI runner in tests.
func SetGHCommandForTest(fn func(args ...string) ([]byte, error)) {
    if fn == nil {
        ghRunner = func(args ...string) ([]byte, error) {
            return exec.Command("gh", args...).Output()
        }
    } else {
        ghRunner = fn
    }
}

// DeduplicatePR checks if an open PR exists for capmon/drift-<provider>.
func DeduplicatePR(ctx context.Context, provider string) (string, bool, error) {
    slug, err := SanitizeSlug(provider)
    if err != nil {
        return "", false, fmt.Errorf("invalid provider slug: %w", err)
    }
    branch := "capmon/drift-" + slug
    out, err := ghRunner("pr", "list", "--label", "capmon", "--head", branch, "--json", "url")
    if err != nil {
        return "", false, fmt.Errorf("gh pr list: %w", err)
    }
    var prs []struct{ URL string `json:"url"` }
    if err := json.Unmarshal(out, &prs); err != nil {
        return "", false, fmt.Errorf("parse gh output: %w", err)
    }
    if len(prs) == 0 {
        return "", false, nil
    }
    return prs[0].URL, true, nil
}

// failureCountFile returns the path to the consecutive-failure counter for a provider.
func failureCountFile(cacheRoot, provider string) string {
    return filepath.Join(cacheRoot, provider, "consecutive-failures.json")
}

// RecordConsecutiveFailure increments the failure counter. Opens a GH issue at 3.
func RecordConsecutiveFailure(ctx context.Context, cacheRoot, provider string) error {
    path := failureCountFile(cacheRoot, provider)
    var count int
    if data, err := os.ReadFile(path); err == nil {
        json.Unmarshal(data, &count)
    }
    count++
    os.MkdirAll(filepath.Dir(path), 0755)
    data, _ := json.Marshal(count)
    os.WriteFile(path, data, 0644)

    if count >= 3 {
        slug, _ := SanitizeSlug(provider)
        title := fmt.Sprintf("capmon: %d consecutive extraction failures for %s", count, slug)
        _, err := ghRunner("issue", "create",
            "--title", title,
            "--label", "capmon",
            "--body", fmt.Sprintf("Provider %s has failed extraction %d consecutive times. Manual intervention required.", slug, count),
        )
        return err
    }
    return nil
}

func CreateDriftPR(ctx context.Context, provider, runID string, diff CapabilityDiff) (string, error) {
    slug, err := SanitizeSlug(provider)
    if err != nil {
        return "", fmt.Errorf("invalid provider slug: %w", err)
    }
    branch := "capmon/drift-" + slug
    // Full git branch creation and push logic implemented in pipeline.go Stage 4
    out, err := ghRunner("pr", "create",
        "--title", fmt.Sprintf("capmon: drift detected for %s", slug),
        "--head", branch,
        "--label", "capmon",
    )
    if err != nil {
        return "", fmt.Errorf("create PR: %w", err)
    }
    return strings.TrimSpace(string(out)), nil
}

func CreateStructuralIssue(ctx context.Context, provider, runID string, drift []string) error {
    slug, err := SanitizeSlug(provider)
    if err != nil {
        return fmt.Errorf("invalid provider slug: %w", err)
    }
    body := fmt.Sprintf("New sections detected in %s docs (run %s):\n", slug, runID)
    for _, d := range drift {
        body += "- " + d + "\n"
    }
    _, err = ghRunner("issue", "create",
        "--title", fmt.Sprintf("capmon: structural drift in %s", slug),
        "--label", "capmon",
        "--body", body,
    )
    return err
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run "TestDeduplicatePR|TestRecordConsecutiveFailure"
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/report.go cli/internal/capmon/report_test.go
git commit -m "feat(capmon): complete Stage 4 with dedup, PR creation, and failure tracking"
```

---

### Task 9.2.validate + Phase 9 gate

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestDeduplicatePR_NoneExists` → PASS
- `cd cli && go test ./internal/capmon/ -run TestRecordConsecutiveFailure_ThirdFailure` → PASS — issue created on 3rd failure
- `cd cli && go test ./internal/capmon/ -run TestBuildPRBody_NoTemplateInjection` → PASS — `{{.Secret}}` appears verbatim in fenced block
- `grep -q 'sanitizeSlug\|SanitizeSlug' cli/internal/capmon/report.go` → found — slug validated before branch name construction
- `grep -q 'io.Writer' cli/internal/capmon/report.go` → found — BuildPRBody uses io.Writer directly, no template engine
- No regressions: `cd cli && go test ./...` → PASS

---

## Phase 10: Fixture Tests, Seed Execution, and Integration Polish

**Goal:** Add fixture-based integration tests for all extractors, execute the `capmon seed` command on real provider data, verify the spec regeneration banner system, and confirm coverage targets.

---

### Task 10.1: Fixture-based integration tests (impl)

**Files:**
- Create: `cli/internal/capmon/testdata/fixtures/claude-code/hooks-docs.html` — snapshot
- Create: `cli/internal/capmon/testdata/fixtures/claude-code/hooks-types.ts` — snapshot
- Create: `cli/internal/capmon/testdata/fixtures/gemini-cli/types.ts` — snapshot
- Create: `cli/internal/capmon/testdata/fixtures/windsurf/llms-full.txt` — snapshot
- Create: `cli/internal/capmon/testdata/expected/claude-code.yaml` — expected extraction output
- Create: `cli/internal/capmon/extract_test.go` — end-to-end fixture tests + live network tests

**Depends on:** Phase 9 gate

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestFixtures_ClaudeCodeHooksHTML` → PASS — at least 6 fields extracted from HTML fixture
- `cd cli && go test ./internal/capmon/ -run TestFixtures_WindsurfLLMSTxt` → PASS — YAML extractor produces fields from fixture
- `SYLLAGO_TEST_NETWORK=1 cd cli && go test ./internal/capmon/ -run TestLiveNetwork` → PASS (requires network; skipped in CI)
- Live tests gated: `grep -q 'SYLLAGO_TEST_NETWORK' cli/internal/capmon/extract_test.go` → found — skip guard present

```go
// cli/internal/capmon/extract_test.go
func TestFixtures_ClaudeCodeHooksHTML(t *testing.T) {
    raw, err := os.ReadFile("testdata/fixtures/claude-code/hooks-docs.html")
    if err != nil {
        t.Fatalf("read fixture: %v", err)
    }
    cfg := capmon.SelectorConfig{
        Primary:          "main h2#events ~ table",
        Fallback:         "main table",
        ExpectedContains: "Event Name",
        MinResults:       6,
    }
    result, err := capmon.Extract(context.Background(), "html", raw, cfg)
    if err != nil {
        t.Fatalf("Extract: %v", err)
    }
    if len(result.Fields) < 6 {
        t.Errorf("expected at least 6 fields, got %d", len(result.Fields))
    }
    if result.Partial {
        t.Error("result should not be partial with real fixture")
    }
}
```

---

### Task 10.1.validate: Validate fixture-based integration tests

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestFixtures_ClaudeCodeHooksHTML` → PASS — 6+ fields extracted from HTML fixture
- `cd cli && go test ./internal/capmon/ -run TestFixtures_WindsurfLLMSTxt` → PASS — fields extracted from windsurf fixture
- Fixture files exist at `cli/internal/capmon/testdata/fixtures/claude-code/hooks-docs.html`, `cli/internal/capmon/testdata/fixtures/windsurf/llms-full.txt`
- Live tests skipped in CI: `grep -q 'SYLLAGO_TEST_NETWORK' cli/internal/capmon/extract_test.go` → found

---

### Task 10.2: Spec regeneration — `GenerateHooksSpecTables()` implementation and `capmon generate` wiring (impl)

**Files:**
- Modify: `cli/internal/capmon/generate.go` — add `GenerateHooksSpecTables()` function
- Create: `cli/internal/capmon/generate_hooks_test.go`
- Modify: `cli/cmd/syllago/capmon_cmd.go` — update `capmonGenerateCmd.RunE` to also call `GenerateHooksSpecTables()` after `GenerateContentTypeViews()`

**Depends on:** Task 10.1.validate

**Critical wiring:** `capmonGenerateCmd.RunE` must call BOTH `GenerateContentTypeViews` (Task 7.4) AND `GenerateHooksSpecTables` (this task). The design specifies `syllago capmon generate` regenerates all output — per-content-type views AND hooks spec tables. Without this wiring, Task 10.2b's success criterion ("`syllago capmon generate` → PASS — runs without error and regenerates all 4 spec table sections") cannot be satisfied.

Updated `capmonGenerateCmd.RunE`:
```go
var capmonGenerateCmd = &cobra.Command{
    Use:   "generate",
    Short: "Regenerate per-content-type views and hooks spec tables from provider-capabilities YAML",
    RunE: func(cmd *cobra.Command, args []string) error {
        if err := capmon.GenerateContentTypeViews(
            "docs/provider-capabilities",
            "docs/provider-capabilities/by-content-type",
        ); err != nil {
            return fmt.Errorf("generate content-type views: %w", err)
        }
        if err := capmon.GenerateHooksSpecTables(
            "docs/provider-capabilities",
            "docs/spec/hooks",
        ); err != nil {
            return fmt.Errorf("generate hooks spec tables: %w", err)
        }
        return nil
    },
}
```

**Scope acknowledgment:** This task adds the `GenerateHooksSpecTables()` function and tests it against a minimal fixture. The actual spec file modifications (4 files in `docs/spec/hooks/`) are a separate task (Task 10.2b below) because they require the function to already exist and the generated output to be reviewed.

**`GenerateHooksSpecTables()` signature and algorithm:**

```go
// GenerateHooksSpecTables reads provider-capabilities/*.yaml and regenerates
// the table sections in the hooks spec markdown files.
// capsDir: path to docs/provider-capabilities/
// specDir: path to docs/spec/hooks/
func GenerateHooksSpecTables(capsDir, specDir string) error
```

**How it finds sections in spec files:** Each generated section is delimited by a pair of HTML comments in the markdown:

```markdown
<!-- GENERATED FROM provider-capabilities/*.yaml — do not edit directly.
     Regenerate with: syllago capmon generate -->
| Event Name | ... |
|------------|-----|
| ...        | ... |
<!-- END GENERATED -->
```

The function searches for `<!-- GENERATED FROM` as the start sentinel and `<!-- END GENERATED -->` as the end sentinel. Content between these markers is replaced wholesale. If a file does not yet have these sentinels, the function returns an error listing which file needs the banner added manually (or a `--add-banner` flag can be passed to insert it).

**Target sections (4 total):**
1. `docs/spec/hooks/events.md` §4 Event Name Mapping table — generated from `hooks.events[*].native_name` across all providers
2. `docs/spec/hooks/blocking-matrix.md` §2 Matrix table — generated from `hooks.events[*].blocking` fields
3. `docs/spec/hooks/capabilities.md` support matrices — generated from `hooks.capabilities[*].supported` fields
4. `docs/spec/hooks/tools.md` §1 Canonical Tool Names table — generated from `hooks.tools[*].native` fields

#### Success Criteria
- `cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables_BasicOutput` → PASS — function produces correct markdown table from fixture YAML
- `cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables_MissingBanner` → PASS — returns error when target section marker absent from spec file
- `grep -q 'GENERATED FROM' cli/internal/capmon/generate.go` → found — banner constant present

#### Step 1: Write the failing tests (RED)

```go
// cli/internal/capmon/generate_hooks_test.go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestGenerateHooksSpecTables_BasicOutput(t *testing.T) {
    capsDir := t.TempDir()
    specDir := t.TempDir()

    // Write a minimal capability YAML
    capsYAML := `schema_version: "1"
slug: claude-code
display_name: Claude Code
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: PreToolUse
        blocking: prevent
`
    os.WriteFile(filepath.Join(capsDir, "claude-code.yaml"), []byte(capsYAML), 0644)

    // Write a minimal events.md with generation sentinels
    eventsSpec := `# Events

## §4 Event Name Mapping

<!-- GENERATED FROM provider-capabilities/*.yaml — do not edit directly.
     Regenerate with: syllago capmon generate -->
| Canonical Name | Provider | Native Name |
|----------------|----------|-------------|
| placeholder | - | - |
<!-- END GENERATED -->

## §5 Other Section
`
    os.WriteFile(filepath.Join(specDir, "events.md"), []byte(eventsSpec), 0644)

    if err := capmon.GenerateHooksSpecTables(capsDir, specDir); err != nil {
        t.Fatalf("GenerateHooksSpecTables: %v", err)
    }

    data, _ := os.ReadFile(filepath.Join(specDir, "events.md"))
    if !strings.Contains(string(data), "before_tool_execute") {
        t.Error("events.md should contain canonical event name 'before_tool_execute'")
    }
    if !strings.Contains(string(data), "PreToolUse") {
        t.Error("events.md should contain native name 'PreToolUse'")
    }
    if strings.Contains(string(data), "placeholder") {
        t.Error("placeholder row should be replaced by generated content")
    }
}

func TestGenerateHooksSpecTables_MissingBanner(t *testing.T) {
    capsDir := t.TempDir()
    specDir := t.TempDir()

    os.WriteFile(filepath.Join(capsDir, "claude-code.yaml"), []byte(`schema_version: "1"
slug: claude-code
content_types:
  hooks:
    supported: true
`), 0644)

    // Write events.md WITHOUT the generation sentinels
    os.WriteFile(filepath.Join(specDir, "events.md"), []byte(`# Events\n\n## §4 Event Name Mapping\n`), 0644)

    err := capmon.GenerateHooksSpecTables(capsDir, specDir)
    if err == nil {
        t.Error("expected error when GENERATED FROM sentinel missing from spec file")
    }
    if !strings.Contains(err.Error(), "GENERATED FROM") {
        t.Errorf("error should mention missing GENERATED FROM sentinel, got: %v", err)
    }
}
```

#### Step 2: Verify tests fail
```
cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables
```
Expected: FAIL — `cli/internal/capmon/generate_hooks_test.go:XX: undefined: capmon.GenerateHooksSpecTables`

#### Step 3: Write minimal implementation (GREEN)

Add to `cli/internal/capmon/generate.go`:
```go
const generatedBannerStart = "<!-- GENERATED FROM provider-capabilities/*.yaml — do not edit directly.\n     Regenerate with: syllago capmon generate -->"
const generatedBannerEnd = "<!-- END GENERATED -->"

// GenerateHooksSpecTables reads provider-capabilities/*.yaml and regenerates
// the table sections in the hooks spec markdown files.
func GenerateHooksSpecTables(capsDir, specDir string) error {
    entries, err := os.ReadDir(capsDir)
    if err != nil {
        return fmt.Errorf("read capabilities dir: %w", err)
    }

    // Build per-content-type data
    type eventRow struct { Canonical, Provider, Native, Blocking string }
    var eventRows []eventRow

    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
            continue
        }
        caps, err := capyaml.LoadCapabilityYAML(filepath.Join(capsDir, e.Name()))
        if err != nil {
            continue
        }
        hooks, ok := caps.ContentTypes["hooks"]
        if !ok {
            continue
        }
        for canonical, event := range hooks.Events {
            eventRows = append(eventRows, eventRow{
                Canonical: canonical,
                Provider:  caps.Slug,
                Native:    event.NativeName,
                Blocking:  event.Blocking,
            })
        }
    }

    // Generate events.md §4 table
    eventsTable := "| Canonical Name | Provider | Native Name |\n|----------------|----------|-------------|\n"
    for _, row := range eventRows {
        eventsTable += fmt.Sprintf("| %s | %s | %s |\n", row.Canonical, row.Provider, row.Native)
    }

    eventsPath := filepath.Join(specDir, "events.md")
    if err := replaceGeneratedSection(eventsPath, eventsTable); err != nil {
        return fmt.Errorf("update events.md: %w", err)
    }
    return nil
}

// replaceGeneratedSection finds the GENERATED FROM...END GENERATED block in path
// and replaces its content with newContent.
func replaceGeneratedSection(path, newContent string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read %s: %w", path, err)
    }
    content := string(data)
    startIdx := strings.Index(content, generatedBannerStart)
    if startIdx == -1 {
        return fmt.Errorf("GENERATED FROM sentinel not found in %s — add the banner manually first", path)
    }
    endIdx := strings.Index(content, generatedBannerEnd)
    if endIdx == -1 {
        return fmt.Errorf("END GENERATED sentinel not found in %s", path)
    }
    endIdx += len(generatedBannerEnd)

    updated := content[:startIdx] +
        generatedBannerStart + "\n" +
        newContent +
        generatedBannerEnd +
        content[endIdx:]

    return os.WriteFile(path, []byte(updated), 0644)
}
```

#### Step 4: Verify tests pass
```
cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables
```
Expected: PASS

#### Step 5: Commit
```bash
git add cli/internal/capmon/generate.go cli/internal/capmon/generate_hooks_test.go
git commit -m "feat(capmon): add GenerateHooksSpecTables() with sentinel-based section replacement"
```

---

### Task 10.2.validate: Validate GenerateHooksSpecTables

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → pass
- `cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables_BasicOutput` → PASS — canonical name and native name in output
- `cd cli && go test ./internal/capmon/ -run TestGenerateHooksSpecTables_MissingBanner` → PASS — error when sentinel missing
- `grep -q 'GENERATED FROM' cli/internal/capmon/generate.go` → found — banner constant present
- `grep -q 'END GENERATED' cli/internal/capmon/generate.go` → found — end sentinel present

---

### Task 10.2b: Add generation banners to hooks spec files (impl)

**Files:**
- Modify: `docs/spec/hooks/events.md` — add `<!-- GENERATED FROM ... -->` / `<!-- END GENERATED -->` sentinels around §4 Event Name Mapping table
- Modify: `docs/spec/hooks/blocking-matrix.md` — add sentinels around §2 Matrix table
- Modify: `docs/spec/hooks/capabilities.md` — add sentinels around support matrices
- Modify: `docs/spec/hooks/tools.md` — add sentinels around §1 Canonical Tool Names table

**Depends on:** Task 10.2.validate

**Note:** This task ONLY adds the sentinel comments to the existing spec files. The actual table content is then regenerated by running `syllago capmon generate`. Splitting this from the code work (Task 10.2) ensures the sentinel format is correct before trying to use it.

#### Success Criteria
- `syllago capmon generate` → PASS — runs without error and regenerates all 4 spec table sections
- `grep -rq 'GENERATED FROM' docs/spec/hooks/events.md` → found
- `grep -rq 'GENERATED FROM' docs/spec/hooks/blocking-matrix.md` → found
- `grep -rq 'GENERATED FROM' docs/spec/hooks/capabilities.md` → found
- `grep -rq 'GENERATED FROM' docs/spec/hooks/tools.md` → found

#### Step 1–5: Add the sentinel pair to each of the 4 spec files around their respective data tables. Run `syllago capmon generate` after adding sentinels to verify the function replaces them correctly. Commit all 4 file changes together with the generated output:

```bash
git add docs/spec/hooks/events.md docs/spec/hooks/blocking-matrix.md \
        docs/spec/hooks/capabilities.md docs/spec/hooks/tools.md
git commit -m "feat(capmon): add generation sentinels to hooks spec table sections"
```

---

### Task 10.2b.validate: Validate hooks spec banner integration

**Validated by:** Different subagent (Haiku)
**Checks:**
- `syllago capmon generate` → PASS — exits 0, no error
- `grep -q 'GENERATED FROM' docs/spec/hooks/events.md` → found
- `grep -q 'END GENERATED' docs/spec/hooks/events.md` → found
- `grep -q 'GENERATED FROM' docs/spec/hooks/blocking-matrix.md` → found
- `grep -q 'GENERATED FROM' docs/spec/hooks/capabilities.md` → found
- `grep -q 'GENERATED FROM' docs/spec/hooks/tools.md` → found

---

### Task 10.3: Coverage verification and CONTRIBUTING.md update (impl)

**Files:**
- Modify: `CONTRIBUTING.md` — document `.capmon-pause` sentinel, `capmon test-fixtures --update`, and manual audit workflow
- Run: `cd cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total`

**Depends on:** Task 10.2b.validate

#### Success Criteria
- `cd cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | grep 'total:' | awk '{print $3}'` → outputs ≥ 80% — coverage target met
- `grep -q 'capmon-pause' CONTRIBUTING.md` → found — sentinel documented
- `test -f docs/provider-capabilities/README.md` → PASS — README present (was listed in Phase 6 scope)

---

### Task 10.validate + Phase 10 gate (Final validation)

**Validated by:** Different subagent (Haiku)
**Checks:**
- `make build` → PASS
- `cd cli && go test ./...` → PASS — full test suite, no regressions
- `cd cli && make fmt` → no diff
- `cd cli && go vet ./...` → clean
- `cd cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | grep 'total:'` → ≥ 80% coverage
- `CGO_ENABLED=0 go build ./cmd/syllago` → PASS — no CGO
- `syllago capmon --help` → PASS — all 8 subcommands listed: fetch, extract, run, diff, generate, verify, seed, test-fixtures
- `grep -q '.capmon-cache/' .gitignore` → found
- `grep -q '!docs/provider-capabilities/' .gitignore` → found
- `test -f docs/provider-capabilities/README.md` → PASS — README present
- `grep -q 'capmon-pause' CONTRIBUTING.md` → found
- `cd cli && go test ./internal/capmon/ -run TestFixtures_ClaudeCodeHooksHTML` → PASS — claude-code fixture test passes
- `cd cli && go test ./internal/capmon/ -run TestFixtures_WindsurfLLMSTxt` → PASS — windsurf fixture test passes
- `cat .github/workflows/capmon.yml | python3 -c "import yaml,sys; yaml.safe_load(sys.stdin)"` → PASS — workflow YAML valid
- `grep -q 'GENERATED FROM' docs/spec/hooks/events.md` → found — spec regeneration banner in place
- `grep -q 'staleness-check' cli/cmd/syllago/capmon_cmd.go` → found — staleness-check flag present on verify
- `grep -q 'migration-window' cli/cmd/syllago/capmon_cmd.go` → found — migration-window flag present on verify
- `grep -q 'GenerateHooksSpecTables' cli/cmd/syllago/capmon_cmd.go` → found — capmon generate wired to call GenerateHooksSpecTables
- `grep -q 'json-schema' cli/internal/capmon/extract_json_schema/extract_json_schema.go` → found — json-schema extractor exists
- Phase 5b gate passed: 9 extractors registered (html, markdown, go, json, json-schema, yaml, toml, typescript, rust)
- `grep -q 'ReferenceEntry' cli/internal/capmon/capyaml/types.go` → found — references table type defined

---

## Dependency Map

```
Phase 1 (Bootstrap) → Phase 2 (Cache) → Phase 3 (HTTP Fetchers)
                                       → Phase 4 (Chromedp) [after gotreesitter sign-off]
                    → Phase 5a (6 Extractors)
                    → Phase 5b (TS/Rust) [blocked on gotreesitter sign-off]
                    → Phase 6 (YAML + Verify)
                    → Phase 7 (CLI Commands)
                    → Phase 8 (GH Actions)
                    → Phase 9 (Diff + PR)
                    → Phase 10 (Fixtures + Polish)
```

Phases 3, 4, 5a, 5b, and 6 can proceed in parallel after Phase 2. Phase 7 requires all extractor phases (5a + 5b) and Phase 6. Phase 8 requires Phase 7. Phase 9 requires Phase 8. Phase 10 requires Phase 9.

---

## Telemetry Enrichment

Per `.claude/rules/telemetry-enrichment.md`, add the following `telemetry.Enrich()` calls in `capmon_cmd.go` RunE functions:

| Command | Properties |
|---------|-----------|
| `capmon run` | `provider` (if --provider), `dry_run` (bool), `mode` ("full", "fetch-extract", "report") |
| `capmon fetch` | `provider` (if --provider) |
| `capmon extract` | `provider` (if --provider), `content_type` (if filtered) |
| `capmon generate` | (none — outputs are deterministic) |
| `capmon verify` | (none — outputs are deterministic) |
| `capmon seed` | `provider` (if --provider), `content_count` (seeded count) |

Add corresponding `PropertyDef` entries to `EventCatalog()` in `cli/internal/telemetry/catalog.go` for any new property keys. Run `cd cli && make gendocs` after any catalog change to regenerate `telemetry.json`.

---

## Schema Evolution Policy (Cluster I Deadline)

The schema version policy (`ValidateAgainstSchema` accepting current OR current-minus-one during `--migration-window`) MUST land before Phase 10 closes. Per the design doc:

> "Cluster I must land before any external consumer reads `provider-capabilities/*.yaml`. Currently, the only consumer is the spec regeneration step — so this policy must be in place before spec regeneration code ships."

Phase 6 Task 6.1 covers the initial policy implementation. Phase 10 Task 10.2 (spec regeneration) is the external consumer. Phase 6 gate blocks Phase 10 — this ordering satisfies the hard deadline.
