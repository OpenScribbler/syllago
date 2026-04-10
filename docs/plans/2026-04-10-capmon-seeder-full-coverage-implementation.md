# Capmon Seeder Full Coverage — Implementation Plan

**Based on:** `docs/plans/2026-04-10-capmon-seeder-full-coverage-design.md`
**Date:** 2026-04-10
**Executor:** This plan is self-contained. A subagent can execute any task from this document alone.

---

## Execution Rules

1. **Red/Green TDD for every implementation task.** Write the test first, verify it fails (Red), then write the implementation until it passes (Green).
2. **Always run `make build` from `cli/` after Go changes.** The binary must rebuild before any manual test.
3. **Always run `cd cli && make fmt` before any commit.** CI enforces gofmt. The pre-commit hook blocks unformatted commits.
4. **Every phase ends with a phase gate: full build + full test suite passes before the next phase begins.**
5. **Do not write files other than those explicitly listed in each task.**
6. **Do not pre-set `human_action` in any seeder spec file. It must be left as empty string `""` in every spec a subagent writes.**

---

## Provider List

There are **15** provider slugs in `docs/provider-sources/` (the design doc says 14 — the actual count is 15):

```
amp, claude-code, cline, codex, copilot-cli, crush, cursor,
factory-droid, gemini-cli, kiro, opencode, pi, roo-code, windsurf, zed
```

One provider (`opencode`) has `status: archived`. One (`kiro`) has `status: beta`. All 15 have entries in `docs/provider-capabilities/`. All 15 need recognizer registrations.

---

## Canonical Skills Capability Keys

The 13 canonical keys used throughout this plan (from the design doc):

| Canonical Key | Concept |
|---------------|---------|
| `display_name` | Skill declares a human-readable display name |
| `description` | Skill declares a description (used for invocation routing) |
| `license` | Skill declares a license field |
| `compatibility` | Skill declares platform/version compatibility constraints |
| `metadata_map` | Skill declares an arbitrary key-value metadata block |
| `disable_model_invocation` | Provider allows skill to suppress LLM calls |
| `user_invocable` | Skill can be triggered directly by the user |
| `version` | Skill declares a version string |
| `project_scope` | Provider supports project-local skill storage |
| `global_scope` | Provider supports user-global skill storage |
| `shared_scope` | Provider supports org/shared skill storage |
| `canonical_filename` | Provider uses a fixed canonical filename (e.g., SKILL.md) |
| `custom_filename` | Provider uses a provider-specific or free-form filename scheme |

---

## Confidence Enum Values

- `confirmed` — mapping derived from a human-authored format doc
- `inferred` — mapping derived from cache extraction or subagent reasoning
- `unknown` — not yet assessed

---

## Scope of This Plan

This plan delivers the **Go infrastructure** for the capmon seeder full coverage feature. The following items are outside this plan's task scope and will be completed via subsequent bead/human workflows:

| Item | Why deferred | How it gets done |
|------|-------------|-----------------|
| 15 seeder spec files with `human_action: approve` | Requires running inspection bead + human review per provider | Run Phase 4 workflow bead for each provider, then human reviews and approves |
| Real recognizer implementations for 13 providers (not crush/roo-code) | Requires approved seeder specs (inspection bead output) to know the correct mappings | After seeder specs are approved, each provider gets a recognizer bead |
| `docs/provider-formats/crush.md` and `docs/provider-formats/pi.md` | Created by the inspection bead workflow | Phase 4 bead for crush and pi writes these (per the inspection workflow doc) |

What this plan does deliver: all Go infrastructure (registry, validate-spec, spec gate, confidence field, canonical key renaming, inspection bead workflow document, crush/roo-code real recognizers, and 13 provider stubs that satisfy the registry completeness test).

---

## Existing Code State (Read Before Modifying)

**`cli/internal/capmon/recognize.go`** — current public API:

```go
func RecognizeContentTypeDotPaths(fields map[string]FieldValue) map[string]string
func recognizeSkillsGoStruct(fields map[string]FieldValue) map[string]string  // unexported
func LoadAndRecognizeCache(cacheRoot, provider string) (map[string]string, error)
func mergeInto(dst, src map[string]string)
```

The `RecognizeContentTypeDotPaths` signature **will change** in Phase 1. Existing tests in `recognize_test.go` call this function — they will break and must be updated in the same task as the signature change.

**`cli/internal/capmon/seed.go`** — contains `SeedOptions` struct and `SeedProviderCapabilities(opts SeedOptions) error`. The `SeedOptions.Extracted` field maps dot-paths to values. This file will gain seeder-spec validation in Phase 3.

**`cli/internal/capmon/capyaml/types.go`** — contains `CapabilityEntry` struct. Currently:
```go
type CapabilityEntry struct {
    Supported bool     `yaml:"supported"`
    Mechanism string   `yaml:"mechanism,omitempty"`
    Refs      []string `yaml:"refs,omitempty"`
}
```
Phase 0 adds a `Confidence` field.

**`cli/cmd/syllago/capmon_cmd.go`** — contains Cobra command wiring. Phase 2 adds `capmonValidateSpecCmd` as a new subcommand.

**`cli/internal/capmon/seed.go`** — the `SeedProviderCapabilities` function currently processes dot-paths including `skills.capabilities.<key>.supported` and `skills.capabilities.<key>.mechanism`. Phase 0 adds handling for `skills.capabilities.<key>.confidence`. Phase 3 adds the seeder-spec validation gate.

---

## Phase 0 — Capability YAML Schema: Add `confidence` Field

**Goal:** Add `confidence` field to `CapabilityEntry` in the Go struct and ensure the seeder writes it when processing dot-paths.

**Why first:** All subsequent phases depend on the recognizers emitting `confidence` dot-paths and the seeder writing them. Establishing the schema before writing any recognizer avoids a two-pass update.

### Task 0.1 — Test: `CapabilityEntry` round-trips `confidence` field

**File to create:** `cli/internal/capmon/capyaml/load_test.go` (add test to existing file)

**What to add (append to existing test file):**
```go
func TestCapabilityEntry_ConfidenceField(t *testing.T) {
    yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    supported: true
    capabilities:
      display_name:
        supported: true
        mechanism: "yaml frontmatter key: name"
        confidence: confirmed
      description:
        supported: true
        mechanism: "yaml frontmatter key: description"
        confidence: inferred
`
    dir := t.TempDir()
    path := filepath.Join(dir, "test.yaml")
    if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }
    caps, err := capyaml.LoadCapabilityYAML(path)
    if err != nil {
        t.Fatalf("LoadCapabilityYAML: %v", err)
    }
    skills := caps.ContentTypes["skills"]
    dn := skills.Capabilities["display_name"]
    if dn.Confidence != "confirmed" {
        t.Errorf("display_name.confidence: got %q, want %q", dn.Confidence, "confirmed")
    }
    desc := skills.Capabilities["description"]
    if desc.Confidence != "inferred" {
        t.Errorf("description.confidence: got %q, want %q", desc.Confidence, "inferred")
    }
    // Round-trip: write and re-read
    var buf bytes.Buffer
    if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
        t.Fatalf("WriteCapabilityYAML: %v", err)
    }
    written := buf.String()
    if !strings.Contains(written, "confidence: confirmed") {
        t.Error("written YAML missing 'confidence: confirmed'")
    }
    if !strings.Contains(written, "confidence: inferred") {
        t.Error("written YAML missing 'confidence: inferred'")
    }
}
```

**Run (expect FAIL — Red):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/capyaml/ -run TestCapabilityEntry_ConfidenceField
```

**Expected failure:** The `Confidence` field does not exist on `CapabilityEntry`, so the test either fails to compile or the values are empty strings.

---

### Task 0.2 — Implement: Add `confidence` to `CapabilityEntry`

**File to modify:** `cli/internal/capmon/capyaml/types.go`

**Change:** Add the `Confidence` field to `CapabilityEntry`:

```go
type CapabilityEntry struct {
    Supported  bool     `yaml:"supported"`
    Mechanism  string   `yaml:"mechanism,omitempty"`
    Confidence string   `yaml:"confidence,omitempty"`
    Refs       []string `yaml:"refs,omitempty"`
}
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/capyaml/ -run TestCapabilityEntry_ConfidenceField
```

---

### Task 0.3 — Test: Seeder writes `confidence` dot-path

**File to modify:** `cli/internal/capmon/seed_test.go` (add test)

**What to add:**
```go
func TestSeedProviderCapabilities_WritesConfidence(t *testing.T) {
    capsDir := t.TempDir()
    seedOpts := capmon.SeedOptions{
        CapsDir:  capsDir,
        Provider: "test-provider",
        Extracted: map[string]string{
            "skills.supported":                             "true",
            "skills.capabilities.display_name.supported":  "true",
            "skills.capabilities.display_name.mechanism":  "yaml frontmatter key: name",
            "skills.capabilities.display_name.confidence": "confirmed",
            "skills.capabilities.description.supported":   "true",
            "skills.capabilities.description.mechanism":   "yaml frontmatter key: description",
            "skills.capabilities.description.confidence":  "inferred",
        },
    }
    if err := capmon.SeedProviderCapabilities(seedOpts); err != nil {
        t.Fatalf("seed: %v", err)
    }
    data, err := os.ReadFile(filepath.Join(capsDir, "test-provider.yaml"))
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    out := string(data)
    if !strings.Contains(out, "confidence: confirmed") {
        t.Errorf("missing 'confidence: confirmed' in output:\n%s", out)
    }
    if !strings.Contains(out, "confidence: inferred") {
        t.Errorf("missing 'confidence: inferred' in output:\n%s", out)
    }
}
```

**Run (expect FAIL — Red):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_WritesConfidence
```

**Expected failure:** Seeder ignores the `confidence` dot-path field.

---

### Task 0.4 — Implement: Seeder handles `confidence` dot-path

**File to modify:** `cli/internal/capmon/seed.go`

**Change:** In the dot-path switch inside `SeedProviderCapabilities`, add `confidence` handling alongside `supported` and `mechanism`:

```go
case len(parts) == 4 && parts[1] == "capabilities":
    capKey, field := parts[2], parts[3]
    if ctEntry.Capabilities == nil {
        ctEntry.Capabilities = make(map[string]capyaml.CapabilityEntry)
    }
    ce := ctEntry.Capabilities[capKey]
    switch field {
    case "supported":
        ce.Supported = value == "true"
    case "mechanism":
        ce.Mechanism = value
    case "confidence":
        ce.Confidence = value
    }
    ctEntry.Capabilities[capKey] = ce
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeedProviderCapabilities_WritesConfidence
```

---

### Phase 0 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

All tests must pass. Build must succeed. Proceed to Phase 1 only after this passes.

---

## Phase 1 — init()-Based Recognizer Registry

**Goal:** Replace the hardcoded single-path dispatch in `RecognizeContentTypeDotPaths` with an `init()`-based registry. Update the function signature to include `provider string`. Update all call sites.

**Why this order:** The registry is the foundation for all per-provider recognizer files. Establishing it before writing any per-provider file means every subsequent phase writes against a stable interface.

### Task 1.1 — Test: All provider slugs are registered in recognizerRegistry

**File to create:** `cli/internal/capmon/recognize_registry_test.go`

This test defines the acceptance criterion for the entire Phase 5 (per-provider stub registration). Writing it in Phase 1 documents the intent and creates the Red phase for the registry-completeness assertion.

```go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// TestAllProviderSlugsRegistered asserts that every provider slug found in
// docs/provider-sources/ has a recognizer registered in recognizerRegistry.
// This test will fail until all per-provider recognizer stubs are written in Phase 5.
func TestAllProviderSlugsRegistered(t *testing.T) {
    // Walk docs/provider-sources/ relative to the repo root.
    // The test binary runs from cli/, so we use ../../docs/provider-sources/.
    sourcesDir := filepath.Join("..", "..", "docs", "provider-sources")
    entries, err := os.ReadDir(sourcesDir)
    if err != nil {
        t.Fatalf("cannot read docs/provider-sources/: %v", err)
    }

    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
            continue
        }
        slug := strings.TrimSuffix(e.Name(), ".yaml")
        if slug == "_template" {
            continue
        }
        // RecognizeContentTypeDotPaths with an unknown provider returns an empty map.
        // If the provider IS registered, calling with empty fields returns an empty map too.
        // We need a way to distinguish "registered but empty" from "not registered".
        // IsRecognizerRegistered is the exported check function (implemented in Task 1.3).
        if !capmon.IsRecognizerRegistered(slug) {
            t.Errorf("provider %q has no registered recognizer; add recognize_%s.go with init() registration",
                slug, strings.ReplaceAll(slug, "-", "_"))
        }
    }
}
```

**Run (expect FAIL — Red, compile error):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestAllProviderSlugsRegistered
```

Expected: compile error — `capmon.IsRecognizerRegistered` does not exist yet.

---

### Task 1.2 — Test: Updated `RecognizeContentTypeDotPaths` signature

**File to modify:** `cli/internal/capmon/recognize_test.go`

The existing tests call `capmon.RecognizeContentTypeDotPaths(fields)` (one argument). The new signature will be `RecognizeContentTypeDotPaths(provider string, fields map[string]FieldValue) map[string]string`. Update the test file to use the new signature.

Replace all occurrences of `capmon.RecognizeContentTypeDotPaths(fields)` with `capmon.RecognizeContentTypeDotPaths("crush", fields)` — crush is the provider whose existing behavior uses the GoStruct helper, which will be called by the crush recognizer in Phase 6.

**Specifically, in `TestRecognizeContentTypeDotPaths_SkillGoStruct`:**
- Change: `result := capmon.RecognizeContentTypeDotPaths(fields)`
- To: `result := capmon.RecognizeContentTypeDotPaths("crush", fields)`

**In `TestRecognizeContentTypeDotPaths_EmptyFields`:**
- Change: `result := capmon.RecognizeContentTypeDotPaths(map[string]capmon.FieldValue{})`
- To: `result := capmon.RecognizeContentTypeDotPaths("crush", map[string]capmon.FieldValue{})`

**In `TestRecognizeContentTypeDotPaths_NoSkillStruct`:**
- Change: `result := capmon.RecognizeContentTypeDotPaths(fields)`
- To: `result := capmon.RecognizeContentTypeDotPaths("crush", fields)`

**Note on test expectations:** After Phase 1, the GoStruct recognizer is no longer called from dispatch — it will only be called by the crush (and roo-code) recognizer directly. In Phase 1, the crush recognizer does not exist yet, so `RecognizeContentTypeDotPaths("crush", fields)` will return an empty map. Update the existing test assertions accordingly:

- `TestRecognizeContentTypeDotPaths_SkillGoStruct` should now assert the result is empty (Phase 1 only — crush recognizer wired in Phase 6)
- `TestRecognizeContentTypeDotPaths_EmptyFields` assertion unchanged (still empty)
- `TestRecognizeContentTypeDotPaths_NoSkillStruct` assertion unchanged (still no skills.supported)

Add a comment to `TestRecognizeContentTypeDotPaths_SkillGoStruct`:
```go
// NOTE: Until Phase 6 wires the crush recognizer, this call returns an empty map.
// The GoStruct recognizer is no longer called from dispatch — crush's init() registration
// will call it internally in recognize_crush.go.
```

**Run (expect FAIL — Red):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeContentTypeDotPaths
```

Expected: compile error — wrong number of arguments to `RecognizeContentTypeDotPaths`.

---

### Task 1.3 — Implement: Registry + updated `RecognizeContentTypeDotPaths`

**File to modify:** `cli/internal/capmon/recognize.go`

Replace the entire file content with the following (preserving `recognizeSkillsGoStruct` and `mergeInto` as utilities):

```go
package capmon

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// recognizerRegistry maps provider slugs to their recognizer functions.
// Entries are registered via init() in per-provider recognize_<slug>.go files.
var recognizerRegistry = map[string]func(map[string]FieldValue) map[string]string{}

// RegisterRecognizer adds a provider recognizer to the registry.
// Called from init() in per-provider recognize_*.go files.
func RegisterRecognizer(provider string, fn func(map[string]FieldValue) map[string]string) {
    recognizerRegistry[provider] = fn
}

// IsRecognizerRegistered reports whether a recognizer is registered for the given provider slug.
// Used in tests to assert full provider coverage.
func IsRecognizerRegistered(provider string) bool {
    _, ok := recognizerRegistry[provider]
    return ok
}

// RecognizeContentTypeDotPaths dispatches to the registered recognizer for the provider.
// Returns a dot-path → value map suitable for passing to SeedProviderCapabilities.
//
// Dot-path format: "<content_type>.<section>.<key>.<field>" → value
// Examples:
//   - "skills.supported" → "true"
//   - "skills.capabilities.display_name.mechanism" → "yaml frontmatter key: name"
//   - "skills.capabilities.display_name.confidence" → "confirmed"
//
// If no recognizer is registered for the provider, logs a warning and returns an empty map.
// Recognition is pattern-based and deterministic — no LLM calls.
func RecognizeContentTypeDotPaths(provider string, fields map[string]FieldValue) map[string]string {
    fn, ok := recognizerRegistry[provider]
    if !ok {
        fmt.Fprintf(os.Stderr, "capmon: warning: no recognizer registered for provider %q\n", provider)
        return make(map[string]string)
    }
    result := make(map[string]string)
    mergeInto(result, fn(fields))
    return result
}

// recognizeSkillsGoStruct recognizes the Agent Skills standard struct pattern.
// Keys of the form "Skill.<FieldName>" with yaml key values map to skills frontmatter capabilities.
// This utility is called by individual recognizer functions (e.g., recognizeCrushSkills)
// that implement the Agent Skills open standard.
//
// IMPORTANT: This function is NOT called from RecognizeContentTypeDotPaths directly.
// Individual provider recognizers call it when appropriate.
func recognizeSkillsGoStruct(fields map[string]FieldValue) map[string]string {
    result := make(map[string]string)
    for k, fv := range fields {
        if len(k) < 7 || k[:6] != "Skill." {
            continue
        }
        yamlKey := fv.Value // e.g., "name", "description", "license"
        if yamlKey == "" || yamlKey == "-" {
            continue
        }
        capKey := canonicalKeyFromYAMLKey(yamlKey)
        result["skills.capabilities."+capKey+".supported"] = "true"
        result["skills.capabilities."+capKey+".mechanism"] = "yaml frontmatter key: " + yamlKey
        result["skills.capabilities."+capKey+".confidence"] = "confirmed"
        result["skills.supported"] = "true"
    }
    return result
}

// canonicalKeyFromYAMLKey maps a YAML frontmatter key name to the canonical capability key.
// Unknown keys are prefixed with display_ to avoid silent drops.
func canonicalKeyFromYAMLKey(yamlKey string) string {
    switch yamlKey {
    case "name":
        return "display_name"
    case "description":
        return "description"
    case "license":
        return "license"
    case "compatibility":
        return "compatibility"
    case "metadata":
        return "metadata_map"
    case "disable-model-invocation", "disable_model_invocation":
        return "disable_model_invocation"
    case "user-invocable", "user_invocable":
        return "user_invocable"
    case "version":
        return "version"
    default:
        return "unknown_" + yamlKey
    }
}

// capabilityDotPaths returns the three dot-path entries for a single canonical capability.
// Used by individual recognizer functions to avoid boilerplate.
func capabilityDotPaths(canonicalKey, mechanism, confidence string) map[string]string {
    prefix := "skills.capabilities." + canonicalKey
    return map[string]string{
        prefix + ".supported":  "true",
        prefix + ".mechanism":  mechanism,
        prefix + ".confidence": confidence,
    }
}

// LoadAndRecognizeCache reads all extracted.json files for the given provider
// from the cache root, runs the registered recognizer, and returns a merged
// dot-path → value map. Source directories that are missing or corrupt are silently skipped.
func LoadAndRecognizeCache(cacheRoot, provider string) (map[string]string, error) {
    providerDir := filepath.Join(cacheRoot, provider)
    entries, err := os.ReadDir(providerDir)
    if err != nil {
        return nil, err
    }
    allFields := make(map[string]FieldValue)
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        extPath := filepath.Join(providerDir, e.Name(), "extracted.json")
        data, err := os.ReadFile(extPath)
        if err != nil {
            continue
        }
        var src ExtractedSource
        if err := json.Unmarshal(data, &src); err != nil {
            continue
        }
        for k, fv := range src.Fields {
            allFields[k] = fv
        }
    }
    return RecognizeContentTypeDotPaths(provider, allFields), nil
}

func mergeInto(dst, src map[string]string) {
    for k, v := range src {
        dst[k] = v
    }
}
```

**Important implementation notes:**
- `recognizeSkillsGoStruct` now uses `canonicalKeyFromYAMLKey` instead of the old `"frontmatter_" + yamlKey` pattern. This updates the output from `frontmatter_name` to `display_name`, `frontmatter_description` to `description`, etc.
- `recognizeSkillsGoStruct` now emits `confidence: confirmed` on each entry.
- `capabilityDotPaths` is a helper that individual recognizers can use.
- The `strings` import is removed (no longer needed in this file); add `"fmt"` for the warning.

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeContentTypeDotPaths
```

---

### Task 1.4 — Fix: Update `LoadAndRecognizeCache` call site in `capmon_cmd.go`

The seed command in `cli/cmd/syllago/capmon_cmd.go` calls `capmon.LoadAndRecognizeCache(cacheRoot, provider)` — this signature is already correct (it already passes `provider`). No change needed to `capmon_cmd.go`.

Verify by running:
```bash
cd /home/hhewett/.local/src/syllago/cli && go build ./...
```

If a compile error appears about `RecognizeContentTypeDotPaths`, trace the call site and fix it.

---

### Task 1.5 — Verify: Existing seed tests still pass

The seed tests in `seed_test.go` do not call `RecognizeContentTypeDotPaths` directly — they call `SeedProviderCapabilities` with pre-built `Extracted` maps. They should still pass.

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeed
```

---

### Task 1.6 — Verify: `TestSeedProviderCapabilities_AppliesDotPaths` needs key name update

The existing test in `seed_test.go` uses `frontmatter_name` as a key in the extracted map:
```go
"skills.capabilities.frontmatter_name.supported": "true",
"skills.capabilities.frontmatter_name.mechanism": "yaml key: name",
```

These are dot-paths passed directly to `SeedProviderCapabilities`. The seeder is dot-path-agnostic — it will still write `frontmatter_name` as the capability key if given `frontmatter_name` in the dot-path. **The seed test does not need updating** because it bypasses the recognizer entirely (the dot-paths are hand-crafted).

However, once Phase 6 completes and the crush/roo-code recognizers produce `display_name` instead of `frontmatter_name`, the actual `docs/provider-capabilities/crush.yaml` and `roo-code.yaml` files will need updating too. That happens in Phase 6, not here.

---

### Phase 1 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

Expected state after Phase 1:
- `TestAllProviderSlugsRegistered` **FAILS** (Red — no per-provider files yet; this is expected and intentional)
- All other tests pass
- Build succeeds

The `TestAllProviderSlugsRegistered` failure is the Red phase for Phase 5. It is the "canary" that makes the registry pattern enforceable. Do not fix it now.

---

## Phase 2 — Seeder Spec Parser + `validate-spec` Command

**Goal:** Define the `SeederSpec` Go struct, implement `LoadSeederSpec` and `ValidateSeederSpec`, and wire a `syllago capmon validate-spec` Cobra subcommand.

**File layout after Phase 2:**
```
cli/internal/capmon/seederspec.go        (new)
cli/internal/capmon/seederspec_test.go   (new)
cli/cmd/syllago/capmon_cmd.go            (modified — add validate-spec subcommand)
```

### Task 2.1 — Test: `SeederSpec` parses a valid spec file

**File to create:** `cli/internal/capmon/seederspec_test.go`

```go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

const validSpecYAML = `provider: claude-code
content_type: skills
format: markdown
format_doc_provenance: human
extraction_gaps:
  - "Some gap"
source_excerpt: |
  ## Skills Frontmatter
  name: required
proposed_mappings:
  - canonical_key: display_name
    supported: true
    mechanism: "yaml frontmatter key: name (required)"
    source_field: "Skill.Name"
    source_value: "name"
    confidence: confirmed
    notes: ""
  - canonical_key: description
    supported: true
    mechanism: "yaml frontmatter key: description (required)"
    source_field: "Skill.Description"
    source_value: "description"
    confidence: confirmed
    notes: ""
human_action: ""
reviewed_at: ""
notes: ""
`

func TestLoadSeederSpec_Valid(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "claude-code-skills.yaml")
    if err := os.WriteFile(path, []byte(validSpecYAML), 0644); err != nil {
        t.Fatal(err)
    }
    spec, err := capmon.LoadSeederSpec(path)
    if err != nil {
        t.Fatalf("LoadSeederSpec: %v", err)
    }
    if spec.Provider != "claude-code" {
        t.Errorf("Provider = %q, want %q", spec.Provider, "claude-code")
    }
    if spec.ContentType != "skills" {
        t.Errorf("ContentType = %q, want %q", spec.ContentType, "skills")
    }
    if len(spec.ProposedMappings) != 2 {
        t.Errorf("ProposedMappings len = %d, want 2", len(spec.ProposedMappings))
    }
    if spec.ProposedMappings[0].CanonicalKey != "display_name" {
        t.Errorf("ProposedMappings[0].CanonicalKey = %q, want %q", spec.ProposedMappings[0].CanonicalKey, "display_name")
    }
    if spec.ProposedMappings[0].Confidence != "confirmed" {
        t.Errorf("ProposedMappings[0].Confidence = %q, want %q", spec.ProposedMappings[0].Confidence, "confirmed")
    }
    if spec.HumanAction != "" {
        t.Errorf("HumanAction = %q, want empty string", spec.HumanAction)
    }
}

func TestLoadSeederSpec_FileNotFound(t *testing.T) {
    _, err := capmon.LoadSeederSpec("/nonexistent/path.yaml")
    if err == nil {
        t.Error("expected error for missing file")
    }
}

func TestLoadSeederSpec_InvalidYAML(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "bad.yaml")
    if err := os.WriteFile(path, []byte("not: valid:\n\tyaml:"), 0644); err != nil {
        t.Fatal(err)
    }
    _, err := capmon.LoadSeederSpec(path)
    if err == nil {
        t.Error("expected error for invalid YAML")
    }
}

func TestValidateSeederSpec_EmptyHumanAction(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "claude-code",
        ContentType: "skills",
        HumanAction: "",
        ReviewedAt:  "",
    }
    err := capmon.ValidateSeederSpec(spec)
    if err == nil {
        t.Fatal("expected error for empty human_action")
    }
    if !strings.Contains(err.Error(), "human_action") {
        t.Errorf("error %q should mention human_action", err.Error())
    }
    if !strings.Contains(err.Error(), "claude-code") {
        t.Errorf("error %q should mention provider slug", err.Error())
    }
}

func TestValidateSeederSpec_InvalidHumanAction(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "crush",
        ContentType: "skills",
        HumanAction: "yes-please",
        ReviewedAt:  "2026-04-10T12:00:00Z",
    }
    err := capmon.ValidateSeederSpec(spec)
    if err == nil {
        t.Fatal("expected error for invalid human_action value")
    }
    if !strings.Contains(err.Error(), "human_action") {
        t.Errorf("error %q should mention human_action", err.Error())
    }
}

func TestValidateSeederSpec_EmptyReviewedAt(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "crush",
        ContentType: "skills",
        HumanAction: "approve",
        ReviewedAt:  "",
    }
    err := capmon.ValidateSeederSpec(spec)
    if err == nil {
        t.Fatal("expected error for empty reviewed_at when human_action is set")
    }
    if !strings.Contains(err.Error(), "reviewed_at") {
        t.Errorf("error %q should mention reviewed_at", err.Error())
    }
}

func TestValidateSeederSpec_ApproveWithReviewedAt(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "crush",
        ContentType: "skills",
        HumanAction: "approve",
        ReviewedAt:  "2026-04-10T12:00:00Z",
    }
    if err := capmon.ValidateSeederSpec(spec); err != nil {
        t.Errorf("approved spec with reviewed_at should pass validation: %v", err)
    }
}

func TestValidateSeederSpec_AdjustIsValid(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "crush",
        ContentType: "skills",
        HumanAction: "adjust",
        ReviewedAt:  "2026-04-10T12:00:00Z",
    }
    if err := capmon.ValidateSeederSpec(spec); err != nil {
        t.Errorf("adjust with reviewed_at should pass validation: %v", err)
    }
}

func TestValidateSeederSpec_SkipIsValid(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "opencode",
        ContentType: "skills",
        HumanAction: "skip",
        ReviewedAt:  "2026-04-10T12:00:00Z",
    }
    if err := capmon.ValidateSeederSpec(spec); err != nil {
        t.Errorf("skip with reviewed_at should pass validation: %v", err)
    }
}

func TestValidateSeederSpec_MappingConfidenceValidValues(t *testing.T) {
    validConfidences := []string{"confirmed", "inferred", "unknown"}
    for _, conf := range validConfidences {
        spec := &capmon.SeederSpec{
            Provider:    "crush",
            ContentType: "skills",
            HumanAction: "approve",
            ReviewedAt:  "2026-04-10T12:00:00Z",
            ProposedMappings: []capmon.ProposedMapping{
                {CanonicalKey: "display_name", Confidence: conf},
            },
        }
        if err := capmon.ValidateSeederSpec(spec); err != nil {
            t.Errorf("confidence %q should be valid: %v", conf, err)
        }
    }
}

func TestValidateSeederSpec_MappingInvalidConfidence(t *testing.T) {
    spec := &capmon.SeederSpec{
        Provider:    "crush",
        ContentType: "skills",
        HumanAction: "approve",
        ReviewedAt:  "2026-04-10T12:00:00Z",
        ProposedMappings: []capmon.ProposedMapping{
            {CanonicalKey: "display_name", Confidence: "maybe"},
        },
    }
    err := capmon.ValidateSeederSpec(spec)
    if err == nil {
        t.Fatal("expected error for invalid confidence value 'maybe'")
    }
    if !strings.Contains(err.Error(), "confidence") {
        t.Errorf("error %q should mention confidence", err.Error())
    }
}
```

**Run (expect FAIL — Red, compile error):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestLoadSeederSpec
```

---

### Task 2.2 — Implement: `seederspec.go`

**File to create:** `cli/internal/capmon/seederspec.go`

```go
package capmon

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// SeederSpec is the Go representation of .develop/seeder-specs/<provider>-<content-type>.yaml.
// It documents proposed canonical capability mappings for a provider × content type pair.
// Subagents write this file; humans review it and set HumanAction and ReviewedAt before seeding.
type SeederSpec struct {
    Provider             string            `yaml:"provider"`
    ContentType          string            `yaml:"content_type"`
    Format               string            `yaml:"format"`
    FormatDocProvenance  string            `yaml:"format_doc_provenance"` // human | subagent
    ExtractionGaps       []string          `yaml:"extraction_gaps,omitempty"`
    SourceExcerpt        string            `yaml:"source_excerpt,omitempty"`
    ProposedMappings     []ProposedMapping `yaml:"proposed_mappings,omitempty"`
    HumanAction          string            `yaml:"human_action"` // approve | adjust | skip | ""
    ReviewedAt           string            `yaml:"reviewed_at"`  // ISO 8601 or ""
    Notes                string            `yaml:"notes,omitempty"`
}

// ProposedMapping is one canonical capability key → provider mapping entry in a SeederSpec.
// One entry per canonical key (not one per sub-property).
type ProposedMapping struct {
    CanonicalKey string `yaml:"canonical_key"`
    Supported    bool   `yaml:"supported"`
    Mechanism    string `yaml:"mechanism"`
    SourceField  string `yaml:"source_field"`  // which extracted field triggered this mapping
    SourceValue  string `yaml:"source_value"`  // the value observed in extracted cache
    Confidence   string `yaml:"confidence"`    // confirmed | inferred | unknown
    Notes        string `yaml:"notes,omitempty"`
}

// validHumanActions is the set of accepted human_action values (excluding empty string,
// which is valid before review but not valid for seeding).
var validHumanActions = map[string]bool{
    "approve": true,
    "adjust":  true,
    "skip":    true,
}

// validConfidenceValues is the set of accepted confidence values.
var validConfidenceValues = map[string]bool{
    "confirmed": true,
    "inferred":  true,
    "unknown":   true,
}

// LoadSeederSpec parses a seeder spec YAML file.
func LoadSeederSpec(path string) (*SeederSpec, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read seeder spec %s: %w", path, err)
    }
    var spec SeederSpec
    if err := yaml.Unmarshal(data, &spec); err != nil {
        return nil, fmt.Errorf("parse seeder spec %s: %w", path, err)
    }
    return &spec, nil
}

// ValidateSeederSpec checks a SeederSpec for schema and review completeness.
// Returns an error if:
//   - HumanAction is empty string (spec not yet reviewed)
//   - HumanAction is not one of: approve, adjust, skip
//   - HumanAction is approve or adjust and ReviewedAt is empty
//   - Any ProposedMapping.Confidence is not one of: confirmed, inferred, unknown
func ValidateSeederSpec(spec *SeederSpec) error {
    if spec.HumanAction == "" {
        return fmt.Errorf(
            "seeder spec for %s has not been reviewed: set human_action to 'approve', 'adjust', or 'skip' before seeding",
            spec.Provider,
        )
    }
    if !validHumanActions[spec.HumanAction] {
        return fmt.Errorf(
            "seeder spec for %s: invalid human_action %q (must be approve, adjust, or skip)",
            spec.Provider, spec.HumanAction,
        )
    }
    if spec.HumanAction == "approve" || spec.HumanAction == "adjust" {
        if spec.ReviewedAt == "" {
            return fmt.Errorf(
                "seeder spec for %s: reviewed_at must be set when human_action is %q",
                spec.Provider, spec.HumanAction,
            )
        }
    }
    for i, m := range spec.ProposedMappings {
        if m.Confidence != "" && !validConfidenceValues[m.Confidence] {
            return fmt.Errorf(
                "seeder spec for %s: proposed_mappings[%d] (%s) has invalid confidence %q (must be confirmed, inferred, or unknown)",
                spec.Provider, i, m.CanonicalKey, m.Confidence,
            )
        }
    }
    return nil
}

// SeederSpecPath returns the conventional path for a seeder spec file.
// Callers must ensure the directory exists.
func SeederSpecPath(seederSpecsDir, provider, contentType string) string {
    return fmt.Sprintf("%s/%s-%s.yaml", seederSpecsDir, provider, contentType)
}
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run "TestLoadSeederSpec|TestValidateSeederSpec"
```

---

### Task 2.3 — Test: `validate-spec` CLI command

**File to create:** `cli/cmd/syllago/capmon_validate_spec_cmd_test.go`

```go
package main

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/output"
)

const approvedSpecYAML = `provider: crush
content_type: skills
format: go
format_doc_provenance: subagent
human_action: approve
reviewed_at: "2026-04-10T12:00:00Z"
notes: ""
`

const unapprovedSpecYAML = `provider: crush
content_type: skills
format: go
format_doc_provenance: subagent
human_action: ""
reviewed_at: ""
notes: ""
`

func TestCapmonValidateSpecCmd_ApprovedSpec(t *testing.T) {
    dir := t.TempDir()
    specsDir := filepath.Join(dir, ".develop", "seeder-specs")
    if err := os.MkdirAll(specsDir, 0755); err != nil {
        t.Fatal(err)
    }
    specPath := filepath.Join(specsDir, "crush-skills.yaml")
    if err := os.WriteFile(specPath, []byte(approvedSpecYAML), 0644); err != nil {
        t.Fatal(err)
    }

    // Override seeder specs dir for the command
    origSeederSpecsDir := capmonSeederSpecsDirOverride
    capmonSeederSpecsDirOverride = specsDir
    t.Cleanup(func() { capmonSeederSpecsDirOverride = origSeederSpecsDir })

    stdout, _ := output.SetForTest(t)
    capmonValidateSpecCmd.Flags().Set("provider", "crush")
    defer capmonValidateSpecCmd.Flags().Set("provider", "")

    err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
    if err != nil {
        t.Errorf("approved spec should pass validation, got error: %v", err)
    }
    _ = stdout
}

func TestCapmonValidateSpecCmd_UnapprovedSpec(t *testing.T) {
    dir := t.TempDir()
    specsDir := filepath.Join(dir, ".develop", "seeder-specs")
    if err := os.MkdirAll(specsDir, 0755); err != nil {
        t.Fatal(err)
    }
    specPath := filepath.Join(specsDir, "crush-skills.yaml")
    if err := os.WriteFile(specPath, []byte(unapprovedSpecYAML), 0644); err != nil {
        t.Fatal(err)
    }

    origSeederSpecsDir := capmonSeederSpecsDirOverride
    capmonSeederSpecsDirOverride = specsDir
    t.Cleanup(func() { capmonSeederSpecsDirOverride = origSeederSpecsDir })

    _, _ = output.SetForTest(t)
    capmonValidateSpecCmd.Flags().Set("provider", "crush")
    defer capmonValidateSpecCmd.Flags().Set("provider", "")

    err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
    if err == nil {
        t.Fatal("unapproved spec should return error")
    }
    if !strings.Contains(err.Error(), "human_action") {
        t.Errorf("error %q should mention human_action", err.Error())
    }
}

func TestCapmonValidateSpecCmd_MissingProvider(t *testing.T) {
    _, _ = output.SetForTest(t)
    // Ensure provider flag is empty
    capmonValidateSpecCmd.Flags().Set("provider", "")

    err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
    if err == nil {
        t.Fatal("missing --provider should return error")
    }
    if !strings.Contains(err.Error(), "--provider") {
        t.Errorf("error %q should mention --provider", err.Error())
    }
}

func TestCapmonValidateSpecCmd_SpecFileNotFound(t *testing.T) {
    dir := t.TempDir()
    // Use a dir with no spec files
    origSeederSpecsDir := capmonSeederSpecsDirOverride
    capmonSeederSpecsDirOverride = filepath.Join(dir, "no-such-dir")
    t.Cleanup(func() { capmonSeederSpecsDirOverride = origSeederSpecsDir })

    _, _ = output.SetForTest(t)
    capmonValidateSpecCmd.Flags().Set("provider", "crush")
    defer capmonValidateSpecCmd.Flags().Set("provider", "")

    err := capmonValidateSpecCmd.RunE(capmonValidateSpecCmd, []string{})
    if err == nil {
        t.Fatal("missing spec file should return error")
    }
}
```

**Run (expect FAIL — Red, compile error):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./cmd/syllago/ -run TestCapmonValidateSpecCmd
```

Expected: `capmonValidateSpecCmd` and `capmonSeederSpecsDirOverride` are not defined.

---

### Task 2.4 — Implement: `validate-spec` Cobra subcommand

**File to modify:** `cli/cmd/syllago/capmon_cmd.go`

Add the following to the file:

1. Add package-level variable for test override (near `capmonCapabilitiesDirOverride`):
```go
// capmonSeederSpecsDirOverride allows tests to redirect the validate-spec command.
var capmonSeederSpecsDirOverride string
```

2. Add new Cobra command:
```go
var capmonValidateSpecCmd = &cobra.Command{
    Use:   "validate-spec",
    Short: "Validate a seeder spec YAML file",
    Long:  "Validates .develop/seeder-specs/<slug>-skills.yaml: checks schema, human_action, and reviewed_at.",
    RunE: func(cmd *cobra.Command, args []string) error {
        provider, _ := cmd.Flags().GetString("provider")
        contentType, _ := cmd.Flags().GetString("content-type")
        if provider == "" {
            return fmt.Errorf("--provider is required: specify a provider slug")
        }
        if _, err := capmon.SanitizeSlug(provider); err != nil {
            return fmt.Errorf("invalid --provider: %w", err)
        }

        specsDir := capmonSeederSpecsDirOverride
        if specsDir == "" {
            specsDir = ".develop/seeder-specs"
        }
        specPath := capmon.SeederSpecPath(specsDir, provider, contentType)

        spec, err := capmon.LoadSeederSpec(specPath)
        if err != nil {
            return fmt.Errorf("load seeder spec: %w", err)
        }
        if err := capmon.ValidateSeederSpec(spec); err != nil {
            return err
        }
        fmt.Printf("OK: seeder spec for %s (%s) is valid — human_action: %s\n",
            provider, contentType, spec.HumanAction)
        return nil
    },
}
```

3. Add flag registration and subcommand wiring in `init()`:
```go
capmonValidateSpecCmd.Flags().String("provider", "", "Provider slug to validate spec for (required)")
capmonValidateSpecCmd.Flags().String("content-type", "skills", "Content type (default: skills)")
capmonCmd.AddCommand(capmonValidateSpecCmd)
```

4. Add telemetry enrichment inside `RunE` (after the SanitizeSlug check):
```go
telemetry.Enrich("provider", provider)
telemetry.Enrich("content_type", contentType)
```

5. Update `cli/internal/telemetry/catalog.go` — add `"capmon_validate_spec"` to the `Commands` list for both the `provider` and `content_type` property definitions (find these by searching for the property name in the existing `Commands` slice). Then regenerate:
```bash
cd /home/hhewett/.local/src/syllago/cli && make gendocs
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./cmd/syllago/ -run TestCapmonValidateSpecCmd
```

---

### Phase 2 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

Optional smoke-test (runs against local binary, path may vary by environment):
```bash
~/.local/bin/syllago capmon validate-spec --help
```

All tests pass (except `TestAllProviderSlugsRegistered` which remains Red intentionally).

---

## Phase 3 — `human_action` Gate in Seed Command

**Goal:** The `seed` command reads `.develop/seeder-specs/<slug>-skills.yaml` if it exists and returns an error if `human_action` is empty or `reviewed_at` is unset. If the spec file does not exist, the gate is skipped.

### Task 3.1 — Test: Seed with unapproved spec returns error

**File to create:** `cli/internal/capmon/seed_spec_gate_test.go`

```go
package capmon_test

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestSeedValidatesSeederSpec_UnapprovedBlocks(t *testing.T) {
    capsDir := t.TempDir()
    specsDir := t.TempDir()

    // Write an unapproved spec
    specContent := `provider: crush
content_type: skills
format: go
format_doc_provenance: subagent
human_action: ""
reviewed_at: ""
`
    specPath := filepath.Join(specsDir, "crush-skills.yaml")
    if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
        t.Fatal(err)
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "crush",
        Extracted:      map[string]string{"skills.supported": "true"},
        SeederSpecsDir: specsDir,
    }
    err := capmon.SeedProviderCapabilities(opts)
    if err == nil {
        t.Fatal("expected error when seeder spec has empty human_action")
    }
    if !strings.Contains(err.Error(), "human_action") {
        t.Errorf("error %q should mention human_action", err.Error())
    }
}

func TestSeedValidatesSeederSpec_ApprovedPasses(t *testing.T) {
    capsDir := t.TempDir()
    specsDir := t.TempDir()

    specContent := `provider: crush
content_type: skills
format: go
format_doc_provenance: subagent
human_action: approve
reviewed_at: "2026-04-10T12:00:00Z"
`
    specPath := filepath.Join(specsDir, "crush-skills.yaml")
    if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
        t.Fatal(err)
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "crush",
        Extracted:      map[string]string{"skills.supported": "true"},
        SeederSpecsDir: specsDir,
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Errorf("approved spec should allow seeding: %v", err)
    }
}

func TestSeedValidatesSeederSpec_NoSpecProceeds(t *testing.T) {
    capsDir := t.TempDir()
    specsDir := t.TempDir()
    // No spec file written — seeder spec dir is empty

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "amp",
        Extracted:      map[string]string{"skills.supported": "true"},
        SeederSpecsDir: specsDir,
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Errorf("missing spec should not block seeding: %v", err)
    }
}

func TestSeedValidatesSeederSpec_AdjustPasses(t *testing.T) {
    capsDir := t.TempDir()
    specsDir := t.TempDir()

    specContent := `provider: crush
content_type: skills
format: go
format_doc_provenance: subagent
human_action: adjust
reviewed_at: "2026-04-10T12:00:00Z"
`
    specPath := filepath.Join(specsDir, "crush-skills.yaml")
    if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
        t.Fatal(err)
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "crush",
        Extracted:      map[string]string{"skills.supported": "true"},
        SeederSpecsDir: specsDir,
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Errorf("adjust spec should allow seeding: %v", err)
    }
}

func TestSeedValidatesSeederSpec_SkipBlocks(t *testing.T) {
    // human_action: skip means the provider should not be seeded.
    // The seed command treats skip as a rejection.
    capsDir := t.TempDir()
    specsDir := t.TempDir()

    specContent := `provider: opencode
content_type: skills
format: unknown
format_doc_provenance: subagent
human_action: skip
reviewed_at: "2026-04-10T12:00:00Z"
`
    specPath := filepath.Join(specsDir, "opencode-skills.yaml")
    if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
        t.Fatal(err)
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "opencode",
        Extracted:      map[string]string{},
        SeederSpecsDir: specsDir,
    }
    err := capmon.SeedProviderCapabilities(opts)
    if err == nil {
        t.Fatal("skip human_action should block seeding with an error")
    }
    if !strings.Contains(err.Error(), "skip") {
        t.Errorf("error %q should mention skip", err.Error())
    }
}

func TestSeedValidatesSeederSpec_EmptySpecsDirProceeds(t *testing.T) {
    capsDir := t.TempDir()
    // Pass empty string for SeederSpecsDir — should proceed without checking specs
    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "amp",
        Extracted:      map[string]string{"skills.supported": "true"},
        SeederSpecsDir: "",
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Errorf("empty SeederSpecsDir should not block seeding: %v", err)
    }
}
```

**Run (expect FAIL — Red, compile error):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeedValidatesSeederSpec
```

Expected: `SeedOptions` does not have `SeederSpecsDir` field.

---

### Task 3.2 — Implement: `SeederSpecsDir` field and spec gate in `seed.go`

**File to modify:** `cli/internal/capmon/seed.go`

1. Add `SeederSpecsDir` to `SeedOptions`:
```go
type SeedOptions struct {
    CapsDir                 string
    Provider                string
    Extracted               map[string]string
    ForceOverwriteExclusive bool
    SeederSpecsDir          string // Optional: path to .develop/seeder-specs/. Empty = skip spec gate.
}
```

2. Add spec gate validation at the top of `SeedProviderCapabilities`, before any file I/O:
```go
// Seeder spec gate: if SeederSpecsDir is set, check for a spec and validate human_action.
if opts.SeederSpecsDir != "" {
    specPath := SeederSpecPath(opts.SeederSpecsDir, opts.Provider, "skills")
    spec, err := LoadSeederSpec(specPath)
    if err != nil {
        if !os.IsNotExist(err) {
            // File exists but cannot be read — hard error
            return fmt.Errorf("read seeder spec: %w", err)
        }
        // File does not exist — spec gate is skipped (provider may not have been inspected yet)
    } else {
        if err := ValidateSeederSpec(spec); err != nil {
            return err
        }
        if spec.HumanAction == "skip" {
            return fmt.Errorf("seeder spec for %s has human_action: skip — this provider should not be seeded", opts.Provider)
        }
    }
}
```

**Note on import:** `LoadSeederSpec` and `ValidateSeederSpec` are in the same package (`capmon`), so no import is needed. `SeederSpecPath` is also in the same package.

**Note on `os.IsNotExist` behavior with wrapped errors:** `LoadSeederSpec` returns `fmt.Errorf("read seeder spec %s: %w", path, err)`. The `os.IsNotExist` check must unwrap:
```go
if !os.IsNotExist(errors.Unwrap(err)) {
```
Or better, restructure to read the file first with `os.Stat` before calling `LoadSeederSpec`:
```go
specPath := SeederSpecPath(opts.SeederSpecsDir, opts.Provider, "skills")
if _, statErr := os.Stat(specPath); statErr == nil {
    spec, err := LoadSeederSpec(specPath)
    if err != nil {
        return fmt.Errorf("read seeder spec: %w", err)
    }
    if err := ValidateSeederSpec(spec); err != nil {
        return err
    }
    if spec.HumanAction == "skip" {
        return fmt.Errorf("seeder spec for %s has human_action: skip — this provider should not be seeded", opts.Provider)
    }
    // Warn (not error) if any mapping has confidence: inferred
    for _, m := range spec.ProposedMappings {
        if m.Confidence == "inferred" {
            fmt.Printf("warning: seeder spec for %s has inferred mapping for %s — verify against format doc before trusting output\n", opts.Provider, m.CanonicalKey)
        }
    }
}
```

This pattern (check existence with `os.Stat` before loading) is cleaner and avoids the error-wrapping issue.

**Add import:** The `errors` package is not needed with the `os.Stat` approach. No new imports required — `os` is already imported.

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeedValidatesSeederSpec
```

---

### Task 3.3 — Update: `capmon_cmd.go` passes `SeederSpecsDir` to seed

**File to modify:** `cli/cmd/syllago/capmon_cmd.go`

In `capmonSeedCmd.RunE`, pass `SeederSpecsDir` to `SeedOptions`:
```go
opts := capmon.SeedOptions{
    CapsDir:                 "docs/provider-capabilities",
    Provider:                provider,
    Extracted:               extracted,
    ForceOverwriteExclusive: forceOverwrite,
    SeederSpecsDir:          ".develop/seeder-specs",
}
```

Add a `--skip-spec-gate` flag to allow running seed without spec validation (for bootstrap/development use):
```go
skipSpecGate, _ := cmd.Flags().GetBool("skip-spec-gate")
seederSpecsDir := ".develop/seeder-specs"
if skipSpecGate {
    seederSpecsDir = ""
}
opts := capmon.SeedOptions{
    CapsDir:                 "docs/provider-capabilities",
    Provider:                provider,
    Extracted:               extracted,
    ForceOverwriteExclusive: forceOverwrite,
    SeederSpecsDir:          seederSpecsDir,
}
```

Register the flag in `init()`:
```go
capmonSeedCmd.Flags().Bool("skip-spec-gate", false, "Skip seeder spec human_action gate (for development only)")
```

**Run:**
```bash
cd /home/hhewett/.local/src/syllago/cli && make build && go test ./cmd/syllago/ -run TestCapmon
```

---

### Phase 3 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

All tests pass (except `TestAllProviderSlugsRegistered` which remains Red intentionally).

---

## Phase 4 — Inspection Bead Prompt Document

**Goal:** Write the documented bead workflow that guides an inspection agent through reading provider source docs + extracted cache and producing a seeder spec YAML.

**Note:** This phase is documentation only. No Go code. No tests. No build required.

**Note on `human_action`:** The workflow document must explicitly state that the inspection bead must leave `human_action: ""` in every spec it writes. This is a critical invariant. The workflow document is how this constraint is communicated to the bead agent.

### Task 4.1 — Write: `docs/workflows/inspect-provider-skills.md`

**File to create:** `docs/workflows/inspect-provider-skills.md`

The document must include all of the following sections:

---

**`docs/workflows/inspect-provider-skills.md` content:**

```markdown
# Inspect Provider Skills — Bead Workflow

**Scope:** Skills content type only. One inspection per provider × content type.
**Output:** `.develop/seeder-specs/<slug>-skills.yaml`

---

## Purpose

This workflow guides an inspection bead agent through reading a provider's format
documentation and extracted cache data, then writing a structured seeder spec YAML
that proposes canonical capability mappings for human review.

The inspection bead does NOT write Go code. It does NOT set `human_action`. It ONLY
produces the seeder spec YAML for a human to review.

---

## What to Read

**Step 1: Check for a human-authored format doc**

Check if `docs/provider-formats/<slug>.md` exists.

- If it exists with no `method: subagent` header: this is human-authored ground truth.
  Read it carefully. All mappings derived from this doc get `confidence: confirmed`.
- If it exists with `method: subagent` header: this was generated by a previous bead.
  Read it but treat all its mappings as `confidence: inferred`.
- If it does not exist: you must generate one. Write it after inspecting the cache.
  Label it with `method: subagent` and mark all mappings `confidence: inferred`.

**Step 2: Read the extracted cache**

Read all files at `.capmon-cache/<slug>/*/extracted.json`. Each file contains:
- `provider`: the provider slug
- `source_id`: the source identifier (e.g., `skills.docs.0`)
- `format`: how the source was fetched and extracted
- `fields`: map of field_name → `{value, value_hash}`
- `landmarks`: list of notable section headings found

Note which `Skill.*` fields are present (these are GoStruct-pattern fields).
Note which non-`Skill.*` fields are present that might indicate skill file locations,
naming conventions, scope, or behavioral capabilities.

**Step 3: Read raw source excerpts**

For each source in `.capmon-cache/<slug>/*/raw.bin`, read the first 2000 characters.
Use this to write a `source_excerpt` in the seeder spec that gives the human reviewer
direct evidence for the proposed mappings.

---

## What to Write

**Primary output: `.develop/seeder-specs/<slug>-skills.yaml`**

Write the seeder spec YAML using the format below. Ensure the directory
`.develop/seeder-specs/` exists before writing.

**CRITICAL INVARIANT: Leave `human_action: ""` and `reviewed_at: ""` in every spec
you write. Never pre-set these fields. They are set exclusively by the human reviewer.
The seed command will refuse to run if `human_action` is not approved by a human.**

**Secondary output (if no human format doc exists):**

If `docs/provider-formats/<slug>.md` does not exist, also write it. Use the following
header to mark it as subagent-generated:
```markdown
---
method: subagent
generated_at: <ISO timestamp>
warning: This file was generated by an inspection bead. It has not been verified by a human.
         All capability mappings derived from this file have confidence: inferred.
---
```

If `docs/provider-formats/<slug>.md` already exists (human-authored), do NOT modify it.
If you find gaps or proposed additions, write them to `docs/provider-formats/<slug>.md.proposed-additions`
instead. Never overwrite a human-authored format doc.

---

## Seeder Spec YAML Format

```yaml
provider: <slug>
content_type: skills
format: <markdown | go | typescript | html | yaml>
format_doc_provenance: <human | subagent>
extraction_gaps:
  - "<description of what was missing or unclear in the extraction>"
source_excerpt: |
  <paste the most relevant section from the raw source, max 30 lines>
proposed_mappings:
  - canonical_key: <one of the canonical keys below>
    supported: <true | false>
    mechanism: "<how the provider implements this — e.g., 'yaml frontmatter key: name (required)'>"
    source_field: "<which extracted field triggered this mapping, e.g., 'Skill.Name'>"
    source_value: "<the value observed in that field, e.g., 'name'>"
    confidence: <confirmed | inferred | unknown>
    notes: "<any caveats, ambiguities, or open questions>"
human_action: ""    # DO NOT SET — reviewer fills this in
reviewed_at: ""     # DO NOT SET — reviewer fills this in
notes: ""           # General notes for the reviewer
```

---

## Canonical Skills Capability Keys

One `proposed_mappings` entry per applicable canonical key. If the provider does not
support a key, omit the entry (do not write `supported: false` unless you have evidence
it was deliberately not implemented).

| Canonical Key | Concept | Example Mechanism |
|---------------|---------|-------------------|
| `display_name` | Skill declares a human-readable name | `yaml frontmatter key: name (required)` |
| `description` | Skill declares a description used for invocation routing | `yaml frontmatter key: description (required)` |
| `license` | Skill declares a license field | `yaml frontmatter key: license (optional)` |
| `compatibility` | Skill declares platform/version compatibility constraints | `yaml frontmatter key: compatibility (optional)` |
| `metadata_map` | Skill declares an arbitrary key-value metadata block | `yaml frontmatter key: metadata (optional map)` |
| `disable_model_invocation` | Provider allows skill to suppress LLM calls | `yaml frontmatter key: disable-model-invocation (optional bool)` |
| `user_invocable` | Skill can be triggered directly by the user (not just the model) | `yaml frontmatter key: user-invocable (optional bool)` |
| `version` | Skill declares a version string | `yaml frontmatter key: version (optional)` |
| `project_scope` | Provider supports project-local skill storage | `.kiro/powers/<name>/power.md` |
| `global_scope` | Provider supports user-global skill storage | `~/.config/crush/skills/` |
| `shared_scope` | Provider supports org/shared skill storage | `mechanism: not observed` |
| `canonical_filename` | Provider uses a fixed canonical filename | `SKILL.md (required, fixed name)` |
| `custom_filename` | Provider uses a free-form or provider-specific filename scheme | `.kiro/powers/<name>/power.md (variable)` |

---

## Instructions for Providers Without a Format Doc

For providers without a human-authored `docs/provider-formats/<slug>.md`
(currently: crush, pi — and any new provider without a manually written doc):

1. Read the source manifest at `docs/provider-sources/<slug>.yaml` to understand
   what sources were fetched and what was expected to be extracted.
2. Read all `extracted.json` files and `raw.bin` files in `.capmon-cache/<slug>/`.
3. Write `docs/provider-formats/<slug>.md` with the `method: subagent` header.
4. Mark ALL proposed_mappings with `confidence: inferred`.
5. Note in `extraction_gaps` any fields that the extraction missed.

---

## Instructions for the GoStruct Pattern (crush, roo-code)

crush and roo-code both implement the Agent Skills open standard from `agentskills.io`.
Their skills source code contains a `Skill` Go struct in `internal/skills/skills.go`.
The extraction for these providers produces `Skill.Name`, `Skill.Description`, etc.

For these providers, the canonical key mappings are:
- `Skill.Name` → `display_name`, mechanism: `yaml frontmatter key: name (required)`, confidence: confirmed
- `Skill.Description` → `description`, mechanism: `yaml frontmatter key: description (required)`, confidence: confirmed
- `Skill.License` → `license`, mechanism: `yaml frontmatter key: license (optional)`, confidence: confirmed
- `Skill.Compatibility` → `compatibility`, mechanism: `yaml frontmatter key: compatibility (optional)`, confidence: confirmed
- `Skill.Metadata` → `metadata_map`, mechanism: `yaml frontmatter key: metadata (optional map)`, confidence: confirmed

File location (project_scope): `.crush/skills/<name>/SKILL.md` (and cross-provider compat paths)
File location (global_scope): `~/.config/crush/skills/`
Canonical filename: `SKILL.md` (fixed)

---

## After Writing the Spec

After writing `.develop/seeder-specs/<slug>-skills.yaml`:

1. Do NOT set `human_action` or `reviewed_at`.
2. Optionally run `syllago capmon validate-spec --provider=<slug>` to check the spec
   schema (this will warn that human_action is empty — that is expected).
3. Notify the human reviewer that the spec is ready for review.

The human reviewer will:
1. Read the spec
2. Set `human_action: approve | adjust | skip`
3. Set `reviewed_at: <ISO 8601 timestamp>`
4. Optionally adjust proposed_mappings if the bead's proposals were incorrect
5. Run `syllago capmon validate-spec --provider=<slug>` to confirm
```

---

### Phase 4 Gate

No build or test gate. Verify the file exists:
```bash
ls /home/hhewett/.local/src/syllago/docs/workflows/inspect-provider-skills.md
```

---

## Phase 5 — Per-Provider Recognizer Stubs (All 15 Providers)

**Goal:** Create one `recognize_<slug_underscored>.go` file per provider, each with an `init()` registration and a stub function that returns an empty map. After Phase 5, `TestAllProviderSlugsRegistered` passes.

**Why stubs first:** Stubs establish the registration pattern and make the registry completeness test pass. Real implementations follow in Phase 6 (crush, roo-code) and subsequent phases (other providers, post-inspection).

**File naming convention:** Hyphens in slugs become underscores in file names:
- `claude-code` → `recognize_claude_code.go`
- `copilot-cli` → `recognize_copilot_cli.go`
- `factory-droid` → `recognize_factory_droid.go`
- `gemini-cli` → `recognize_gemini_cli.go`
- `roo-code` → `recognize_roo_code.go`

### Task 5.1 — Test: Each provider's stub function exists and returns a map

**File to create:** `cli/internal/capmon/recognize_stubs_test.go`

```go
package capmon_test

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// TestAllRegisteredRecognizersReturnMap verifies that every registered recognizer
// returns a non-nil map (even if empty) for empty input.
// This catches recognizers that return nil instead of make(map[string]string).
func TestAllRegisteredRecognizersReturnMap(t *testing.T) {
    allProviders := []string{
        "amp",
        "claude-code",
        "cline",
        "codex",
        "copilot-cli",
        "crush",
        "cursor",
        "factory-droid",
        "gemini-cli",
        "kiro",
        "opencode",
        "pi",
        "roo-code",
        "windsurf",
        "zed",
    }
    for _, slug := range allProviders {
        slug := slug
        t.Run(slug, func(t *testing.T) {
            result := capmon.RecognizeContentTypeDotPaths(slug, map[string]capmon.FieldValue{})
            if result == nil {
                t.Errorf("provider %q: recognizer returned nil, expected empty map", slug)
            }
        })
    }
}
```

**Run (expect PASS — this test verifies non-nil returns, not registration; it passes even before Phase 5):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestAllRegisteredRecognizersReturnMap
```

`RecognizeContentTypeDotPaths` returns `make(map[string]string)` (not nil) when no recognizer is found. So this test will pass immediately — its purpose is to guard against stub implementations accidentally returning `nil` instead of an empty map. The real Red test that blocks is `TestAllProviderSlugsRegistered`. Proceed to 5.2.

---

### Task 5.2 — Implement: Create 15 stub recognizer files

Create the following 15 files. Each follows the same pattern. Replace `<slug>`, `<SlugCamelCase>`, and `<slug_underscored>` with the appropriate values.

**Template for each file:**
```go
package capmon

func init() {
    RegisterRecognizer("<slug>", recognize<SlugCamelCase>Skills)
}

// recognize<SlugCamelCase>Skills recognizes skills capabilities for <slug>.
// TODO: Implement after inspection bead produces seeder spec and human approves.
func recognize<SlugCamelCase>Skills(_ map[string]FieldValue) map[string]string {
    return make(map[string]string)
}
```

**Files to create:**

1. `cli/internal/capmon/recognize_amp.go`
   - slug: `amp`, function: `recognizeAmpSkills`

2. `cli/internal/capmon/recognize_claude_code.go`
   - slug: `claude-code`, function: `recognizeClaudeCodeSkills`

3. `cli/internal/capmon/recognize_cline.go`
   - slug: `cline`, function: `recognizeClineSkills`

4. `cli/internal/capmon/recognize_codex.go`
   - slug: `codex`, function: `recognizeCodexSkills`

5. `cli/internal/capmon/recognize_copilot_cli.go`
   - slug: `copilot-cli`, function: `recognizeCopilotCliSkills`

6. `cli/internal/capmon/recognize_crush.go`
   - slug: `crush`, function: `recognizeCrushSkills`
   - NOTE: This will be updated in Phase 6 with a real implementation.

7. `cli/internal/capmon/recognize_cursor.go`
   - slug: `cursor`, function: `recognizeCursorSkills`

8. `cli/internal/capmon/recognize_factory_droid.go`
   - slug: `factory-droid`, function: `recognizeFactoryDroidSkills`

9. `cli/internal/capmon/recognize_gemini_cli.go`
   - slug: `gemini-cli`, function: `recognizeGeminiCliSkills`

10. `cli/internal/capmon/recognize_kiro.go`
    - slug: `kiro`, function: `recognizeKiroSkills`

11. `cli/internal/capmon/recognize_opencode.go`
    - slug: `opencode`, function: `recognizeOpencodeSkills`

12. `cli/internal/capmon/recognize_pi.go`
    - slug: `pi`, function: `recognizePiSkills`

13. `cli/internal/capmon/recognize_roo_code.go`
    - slug: `roo-code`, function: `recognizeRooCodeSkills`
    - NOTE: This will be updated in Phase 6 with a real implementation.

14. `cli/internal/capmon/recognize_windsurf.go`
    - slug: `windsurf`, function: `recognizeWindsurfSkills`

15. `cli/internal/capmon/recognize_zed.go`
    - slug: `zed`, function: `recognizeZedSkills`

**Example of `cli/internal/capmon/recognize_crush.go` (Phase 5 stub):**
```go
package capmon

func init() {
    RegisterRecognizer("crush", recognizeCrushSkills)
}

// recognizeCrushSkills recognizes skills capabilities for crush.
// TODO: Implement after inspection bead produces seeder spec and human approves.
// crush implements the Agent Skills open standard; see recognize.go's recognizeSkillsGoStruct helper.
func recognizeCrushSkills(_ map[string]FieldValue) map[string]string {
    return make(map[string]string)
}
```

**Run (expect PASS — Green for `TestAllProviderSlugsRegistered`):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run "TestAllProviderSlugsRegistered|TestAllRegisteredRecognizersReturnMap"
```

---

### Task 5.3 — Fix: `recognize_test.go` existing tests now need a registered provider

The existing test `TestRecognizeContentTypeDotPaths_SkillGoStruct` uses `"crush"` as the provider. In Phase 5, the crush stub returns an empty map. The test was updated in Task 1.2 to expect an empty result.

After Phase 6 replaces the crush stub with the real implementation, update this test again to expect the actual GoStruct output. For now, verify the existing tests pass:

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeContentTypeDotPaths
```

---

### Phase 5 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

Expected state after Phase 5:
- `TestAllProviderSlugsRegistered` **PASSES** (Green — all 15 providers registered)
- `TestAllRegisteredRecognizersReturnMap` **PASSES**
- All other tests pass
- Build succeeds

---

## Phase 6 — Crush and Roo-Code Recognizers (Real Implementation)

**Goal:** Replace the crush and roo-code stubs with real implementations that call `recognizeSkillsGoStruct` and also emit scope capability keys (project_scope, global_scope, canonical_filename).

**Why these two first:** They are the only providers with existing seeded data in `docs/provider-capabilities/`. They both implement the Agent Skills open standard (same Go struct), confirmed by the source manifests.

**What the recognizers must emit:** Based on the design doc and existing crush/roo-code capability YAMLs:

For **crush**:
- `display_name` (confirmed, via `Skill.Name` → `yaml frontmatter key: name`)
- `description` (confirmed, via `Skill.Description` → `yaml frontmatter key: description`)
- `license` (confirmed, via `Skill.License` → `yaml frontmatter key: license`)
- `compatibility` (confirmed, via `Skill.Compatibility` → `yaml frontmatter key: compatibility`)
- `metadata_map` (confirmed, via `Skill.Metadata` → `yaml frontmatter key: metadata`)
- `project_scope` (confirmed, mechanism: `.crush/skills/<name>/SKILL.md` or `.agents/skills/<name>/SKILL.md`)
- `global_scope` (confirmed, mechanism: `~/.config/crush/skills/`)
- `canonical_filename` (confirmed, mechanism: `SKILL.md (required, fixed name)`)

For **roo-code**:
- Same GoStruct fields as crush (both implement Agent Skills standard)
- `project_scope` (confirmed, mechanism: `.roo/skills/<name>/SKILL.md`)
- `canonical_filename` (confirmed, mechanism: `SKILL.md (required, fixed name)`)

**Note:** The existing crush/roo-code capability YAMLs use old key names (`frontmatter_name`, etc.). After Phase 6, these files will have updated key names (`display_name`, etc.) — the seeder will overwrite them with the new canonical names when run. This is expected behavior.

### Task 6.1 — Test: Crush recognizer produces correct dot-paths

**File to create:** `cli/internal/capmon/recognize_crush_test.go`

```go
package capmon_test

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// crushGoStructFixture simulates the extracted fields from crush's skills.go GoStruct extraction.
var crushGoStructFixture = map[string]capmon.FieldValue{
    "Skill.Name": {
        Value:     "name",
        ValueHash: "sha256:abc",
    },
    "Skill.Description": {
        Value:     "description",
        ValueHash: "sha256:def",
    },
    "Skill.License": {
        Value:     "license",
        ValueHash: "sha256:ghi",
    },
    "Skill.Compatibility": {
        Value:     "compatibility",
        ValueHash: "sha256:jkl",
    },
    "Skill.Metadata": {
        Value:     "metadata",
        ValueHash: "sha256:mno",
    },
    // Non-Skill fields that should not generate capability entries
    "MaxNameLength": {
        Value:     "64",
        ValueHash: "sha256:pqr",
    },
    "SkillFileName": {
        Value:     "SKILL.md",
        ValueHash: "sha256:stu",
    },
}

func TestRecognizeCrushSkills_GoStructFields(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("crush", crushGoStructFixture)

    // skills.supported must be set
    if result["skills.supported"] != "true" {
        t.Errorf("skills.supported: got %q, want %q", result["skills.supported"], "true")
    }

    // Each canonical key must be present with correct mechanism and confidence
    cases := []struct {
        dotPath string
        want    string
    }{
        // display_name
        {"skills.capabilities.display_name.supported", "true"},
        {"skills.capabilities.display_name.mechanism", "yaml frontmatter key: name"},
        {"skills.capabilities.display_name.confidence", "confirmed"},
        // description
        {"skills.capabilities.description.supported", "true"},
        {"skills.capabilities.description.mechanism", "yaml frontmatter key: description"},
        {"skills.capabilities.description.confidence", "confirmed"},
        // license
        {"skills.capabilities.license.supported", "true"},
        {"skills.capabilities.license.mechanism", "yaml frontmatter key: license"},
        {"skills.capabilities.license.confidence", "confirmed"},
        // compatibility
        {"skills.capabilities.compatibility.supported", "true"},
        {"skills.capabilities.compatibility.mechanism", "yaml frontmatter key: compatibility"},
        {"skills.capabilities.compatibility.confidence", "confirmed"},
        // metadata_map
        {"skills.capabilities.metadata_map.supported", "true"},
        {"skills.capabilities.metadata_map.mechanism", "yaml frontmatter key: metadata"},
        {"skills.capabilities.metadata_map.confidence", "confirmed"},
    }
    for _, tc := range cases {
        got, ok := result[tc.dotPath]
        if !ok {
            t.Errorf("missing dot-path %q", tc.dotPath)
            continue
        }
        if got != tc.want {
            t.Errorf("dot-path %q: got %q, want %q", tc.dotPath, got, tc.want)
        }
    }
}

func TestRecognizeCrushSkills_ScopeKeys(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("crush", crushGoStructFixture)

    // Scope keys must be present regardless of extracted fields
    scopeCases := []struct {
        dotPath string
        want    string
    }{
        {"skills.capabilities.project_scope.supported", "true"},
        {"skills.capabilities.project_scope.confidence", "confirmed"},
        {"skills.capabilities.global_scope.supported", "true"},
        {"skills.capabilities.global_scope.confidence", "confirmed"},
        {"skills.capabilities.canonical_filename.supported", "true"},
        {"skills.capabilities.canonical_filename.confidence", "confirmed"},
    }
    for _, tc := range scopeCases {
        got, ok := result[tc.dotPath]
        if !ok {
            t.Errorf("missing scope dot-path %q", tc.dotPath)
            continue
        }
        if got != tc.want {
            t.Errorf("scope dot-path %q: got %q, want %q", tc.dotPath, got, tc.want)
        }
    }
}

func TestRecognizeCrushSkills_EmptyFields(t *testing.T) {
    // With empty fields: scope keys still present, GoStruct keys absent
    result := capmon.RecognizeContentTypeDotPaths("crush", map[string]capmon.FieldValue{})
    
    // Scope keys should still be present (they are hardcoded, not field-derived)
    if result["skills.capabilities.project_scope.supported"] != "true" {
        t.Error("project_scope should be present even with empty fields")
    }
    if result["skills.capabilities.canonical_filename.supported"] != "true" {
        t.Error("canonical_filename should be present even with empty fields")
    }
    // But GoStruct-derived keys should be absent
    if _, ok := result["skills.capabilities.display_name.supported"]; ok {
        t.Error("display_name should not be present with empty fields")
    }
}

func TestRecognizeCrushSkills_NoFrontmatterPrefixInOutput(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("crush", crushGoStructFixture)
    // Ensure old-style frontmatter_ keys are not produced
    for k := range result {
        if len(k) > 24 && k[18:30] == "frontmatter" {
            t.Errorf("old-style frontmatter_ key found: %q (should use canonical names)", k)
        }
    }
}
```

**Run (expect FAIL — Red):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeCrushSkills
```

Expected failure: stub returns empty map; all assertions about non-empty results fail.

---

### Task 6.2 — Implement: `recognize_crush.go` real implementation

**File to modify:** `cli/internal/capmon/recognize_crush.go`

Replace the stub with:

```go
package capmon

func init() {
    RegisterRecognizer("crush", recognizeCrushSkills)
}

// recognizeCrushSkills recognizes skills capabilities for Crush (Charmbracelet).
// Crush implements the Agent Skills open standard (agentskills.io).
// Source: https://raw.githubusercontent.com/charmbracelet/crush/main/internal/skills/skills.go
//
// Scope paths (from source manifest):
//   Project: .crush/skills/<name>/SKILL.md  (and cross-compat: .agents/skills/, .claude/skills/, .cursor/skills/)
//   Global:  ~/.config/crush/skills/
//   Filename: SKILL.md (fixed, required)
func recognizeCrushSkills(fields map[string]FieldValue) map[string]string {
    result := make(map[string]string)

    // GoStruct-derived frontmatter capabilities (display_name, description, license, etc.)
    mergeInto(result, recognizeSkillsGoStruct(fields))

    // Scope capabilities: hardcoded from source manifest (confirmed from source code)
    mergeInto(result, capabilityDotPaths(
        "project_scope",
        ".crush/skills/<name>/SKILL.md (also: .agents/skills/, .claude/skills/, .cursor/skills/)",
        "confirmed",
    ))
    mergeInto(result, capabilityDotPaths(
        "global_scope",
        "~/.config/crush/skills/",
        "confirmed",
    ))
    mergeInto(result, capabilityDotPaths(
        "canonical_filename",
        "SKILL.md (required, fixed name)",
        "confirmed",
    ))

    return result
}
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeCrushSkills
```

---

### Task 6.3 — Test: Roo-code recognizer produces correct dot-paths

**File to create:** `cli/internal/capmon/recognize_roo_code_test.go`

```go
package capmon_test

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// rooCodeGoStructFixture: roo-code sources its GoStruct from crush's skills.go
// (both implement the same Agent Skills standard — same fields).
var rooCodeGoStructFixture = map[string]capmon.FieldValue{
    "Skill.Name":          {Value: "name", ValueHash: "sha256:a01"},
    "Skill.Description":   {Value: "description", ValueHash: "sha256:b02"},
    "Skill.License":       {Value: "license", ValueHash: "sha256:c03"},
    "Skill.Compatibility": {Value: "compatibility", ValueHash: "sha256:d04"},
    "Skill.Metadata":      {Value: "metadata", ValueHash: "sha256:e05"},
}

func TestRecognizeRooCodeSkills_GoStructFields(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("roo-code", rooCodeGoStructFixture)

    if result["skills.supported"] != "true" {
        t.Errorf("skills.supported: got %q, want %q", result["skills.supported"], "true")
    }

    cases := []struct {
        dotPath string
        want    string
    }{
        {"skills.capabilities.display_name.supported", "true"},
        {"skills.capabilities.display_name.mechanism", "yaml frontmatter key: name"},
        {"skills.capabilities.display_name.confidence", "confirmed"},
        {"skills.capabilities.description.supported", "true"},
        {"skills.capabilities.description.mechanism", "yaml frontmatter key: description"},
        {"skills.capabilities.description.confidence", "confirmed"},
        {"skills.capabilities.license.supported", "true"},
        {"skills.capabilities.compatibility.supported", "true"},
        {"skills.capabilities.metadata_map.supported", "true"},
    }
    for _, tc := range cases {
        got, ok := result[tc.dotPath]
        if !ok {
            t.Errorf("missing dot-path %q", tc.dotPath)
            continue
        }
        if got != tc.want {
            t.Errorf("dot-path %q: got %q, want %q", tc.dotPath, got, tc.want)
        }
    }
}

func TestRecognizeRooCodeSkills_ScopeKeys(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("roo-code", rooCodeGoStructFixture)

    scopeCases := []struct {
        dotPath string
        want    string
    }{
        {"skills.capabilities.project_scope.supported", "true"},
        {"skills.capabilities.project_scope.confidence", "confirmed"},
        {"skills.capabilities.canonical_filename.supported", "true"},
        {"skills.capabilities.canonical_filename.confidence", "confirmed"},
    }
    for _, tc := range scopeCases {
        got, ok := result[tc.dotPath]
        if !ok {
            t.Errorf("missing scope dot-path %q", tc.dotPath)
            continue
        }
        if got != tc.want {
            t.Errorf("scope dot-path %q: got %q, want %q", tc.dotPath, got, tc.want)
        }
    }
}

func TestRecognizeRooCodeSkills_EmptyFields(t *testing.T) {
    result := capmon.RecognizeContentTypeDotPaths("roo-code", map[string]capmon.FieldValue{})
    if result["skills.capabilities.project_scope.supported"] != "true" {
        t.Error("project_scope should be present even with empty fields")
    }
    if result["skills.capabilities.canonical_filename.supported"] != "true" {
        t.Error("canonical_filename should be present even with empty fields")
    }
}
```

**Run (expect FAIL — Red):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeRooCodeSkills
```

---

### Task 6.4 — Implement: `recognize_roo_code.go` real implementation

**File to modify:** `cli/internal/capmon/recognize_roo_code.go`

Replace the stub with:

```go
package capmon

func init() {
    RegisterRecognizer("roo-code", recognizeRooCodeSkills)
}

// recognizeRooCodeSkills recognizes skills capabilities for Roo Code.
// Roo Code implements the Agent Skills open standard (agentskills.io).
// It sources the GoStruct from crush's internal/skills/skills.go — same fields.
// Source: https://raw.githubusercontent.com/charmbracelet/crush/main/internal/skills/skills.go
//         (referenced by roo-code source manifest as GoStruct authority)
//
// Scope paths (from source manifest):
//   Project: .roo/skills/<name>/SKILL.md
//   Global:  not observed in source manifest
//   Filename: SKILL.md (fixed, required)
func recognizeRooCodeSkills(fields map[string]FieldValue) map[string]string {
    result := make(map[string]string)

    // GoStruct-derived frontmatter capabilities (display_name, description, license, etc.)
    mergeInto(result, recognizeSkillsGoStruct(fields))

    // Scope capabilities: from source manifest
    mergeInto(result, capabilityDotPaths(
        "project_scope",
        ".roo/skills/<name>/SKILL.md",
        "confirmed",
    ))
    mergeInto(result, capabilityDotPaths(
        "canonical_filename",
        "SKILL.md (required, fixed name)",
        "confirmed",
    ))

    return result
}
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeRooCodeSkills
```

---

### Task 6.5 — Update: `recognize_test.go` existing GoStruct tests

Now that crush's recognizer is real, update `TestRecognizeContentTypeDotPaths_SkillGoStruct` to expect the actual canonical-key output instead of an empty result:

**File to modify:** `cli/internal/capmon/recognize_test.go`

Remove the Phase 1 comment and update the test assertions to match crush's real output:

```go
func TestRecognizeContentTypeDotPaths_SkillGoStruct(t *testing.T) {
    fields := map[string]capmon.FieldValue{
        "Skill.Name": {
            Value:     "name",
            ValueHash: "sha256:abc",
        },
        "Skill.Description": {
            Value:     "description",
            ValueHash: "sha256:def",
        },
        "Skill.License": {
            Value:     "license",
            ValueHash: "sha256:ghi",
        },
        // Non-Skill fields should not generate skills capabilities via GoStruct
        "MaxNameLength": {
            Value:     "64",
            ValueHash: "sha256:jkl",
        },
        "SkillFileName": {
            Value:     "SKILL.md",
            ValueHash: "sha256:mno",
        },
    }

    result := capmon.RecognizeContentTypeDotPaths("crush", fields)

    // skills.supported should be set
    if result["skills.supported"] != "true" {
        t.Errorf("skills.supported: got %q, want %q", result["skills.supported"], "true")
    }

    // Each Skill field should generate canonical dot-paths with confidence
    cases := []struct{ key, wantVal string }{
        {"skills.capabilities.display_name.supported", "true"},
        {"skills.capabilities.display_name.mechanism", "yaml frontmatter key: name"},
        {"skills.capabilities.display_name.confidence", "confirmed"},
        {"skills.capabilities.description.supported", "true"},
        {"skills.capabilities.description.mechanism", "yaml frontmatter key: description"},
        {"skills.capabilities.description.confidence", "confirmed"},
        {"skills.capabilities.license.supported", "true"},
        {"skills.capabilities.license.mechanism", "yaml frontmatter key: license"},
        {"skills.capabilities.license.confidence", "confirmed"},
    }
    for _, tc := range cases {
        got, ok := result[tc.key]
        if !ok {
            t.Errorf("missing key %q in result", tc.key)
            continue
        }
        if got != tc.wantVal {
            t.Errorf("key %q: got %q, want %q", tc.key, got, tc.wantVal)
        }
    }

    // Old-style frontmatter_ keys must NOT appear
    for k := range result {
        if len(k) > 18 && k[18:29] == "frontmatter" {
            t.Errorf("old-style frontmatter_ key found: %q", k)
        }
    }
}
```

**Run (expect PASS — Green):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestRecognizeContentTypeDotPaths
```

---

### Phase 6 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

All tests pass. The crush and roo-code capability YAMLs are now ready to be regenerated with `syllago capmon seed` (run manually after this phase, not part of CI).

**Check coverage:**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -coverprofile=cov.out && go tool cover -func=cov.out | grep -E "(capmon\b|total)"
```

Target: 80% minimum for the `capmon` package.

---

## Phase 7 — End-to-End Seed Integration Test

**Goal:** Test that `LoadAndRecognizeCache` + `SeedProviderCapabilities` produces a non-empty, correct capability YAML for crush and roo-code using fixture cache data.

**Why needed:** Phases 1–6 test recognizers in isolation. Phase 7 tests the full pipeline: cache reading → recognizer dispatch → capability YAML writing. This catches integration bugs (e.g., provider name mismatch in cache vs. registry key).

### Task 7.1 — Test: End-to-end seed integration for crush

**File to create:** `cli/internal/capmon/seed_integration_test.go`

The test creates a minimal fixture `.capmon-cache/crush/skills.go.0/extracted.json` in a temp dir, calls `LoadAndRecognizeCache`, then calls `SeedProviderCapabilities` and asserts the output YAML contains `confidence: confirmed` and canonical key names.

```go
package capmon_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func writeCacheFixture(t *testing.T, cacheRoot, provider, sourceID string, fields map[string]capmon.FieldValue) {
    t.Helper()
    dir := filepath.Join(cacheRoot, provider, sourceID)
    if err := os.MkdirAll(dir, 0755); err != nil {
        t.Fatal(err)
    }
    src := capmon.ExtractedSource{
        ExtractorVersion: "test",
        Provider:         provider,
        SourceID:         sourceID,
        Format:           "go",
        ExtractedAt:      time.Now(),
        Fields:           fields,
    }
    data, err := json.Marshal(src)
    if err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(dir, "extracted.json"), data, 0644); err != nil {
        t.Fatal(err)
    }
}

func TestSeedIntegration_CrushEndToEnd(t *testing.T) {
    cacheRoot := t.TempDir()
    capsDir := t.TempDir()

    // Write crush GoStruct fixture into cache
    writeCacheFixture(t, cacheRoot, "crush", "skills.go.0", map[string]capmon.FieldValue{
        "Skill.Name":          {Value: "name", ValueHash: "sha256:a1"},
        "Skill.Description":   {Value: "description", ValueHash: "sha256:b2"},
        "Skill.License":       {Value: "license", ValueHash: "sha256:c3"},
        "Skill.Compatibility": {Value: "compatibility", ValueHash: "sha256:d4"},
        "Skill.Metadata":      {Value: "metadata", ValueHash: "sha256:e5"},
    })

    extracted, err := capmon.LoadAndRecognizeCache(cacheRoot, "crush")
    if err != nil {
        t.Fatalf("LoadAndRecognizeCache: %v", err)
    }
    if len(extracted) == 0 {
        t.Fatal("LoadAndRecognizeCache returned empty map for crush with GoStruct fixtures")
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "crush",
        Extracted:      extracted,
        SeederSpecsDir: "", // Skip spec gate for this integration test
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Fatalf("SeedProviderCapabilities: %v", err)
    }

    data, err := os.ReadFile(filepath.Join(capsDir, "crush.yaml"))
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    out := string(data)

    checks := []struct {
        want string
        desc string
    }{
        {"skills:", "skills content type"},
        {"supported: true", "skills supported"},
        {"display_name:", "display_name canonical key"},
        {"description:", "description canonical key"},
        {"license:", "license canonical key"},
        {"compatibility:", "compatibility canonical key"},
        {"metadata_map:", "metadata_map canonical key"},
        {"project_scope:", "project_scope capability"},
        {"global_scope:", "global_scope capability"},
        {"canonical_filename:", "canonical_filename capability"},
        {"confidence: confirmed", "confidence field written"},
    }
    for _, c := range checks {
        if !strings.Contains(out, c.want) {
            t.Errorf("output missing %s (%q)\nFull output:\n%s", c.desc, c.want, out)
        }
    }

    // Old-style frontmatter_ keys must NOT appear in the capability YAML
    if strings.Contains(out, "frontmatter_") {
        t.Errorf("output contains old-style frontmatter_ key name\nFull output:\n%s", out)
    }
}

func TestSeedIntegration_RooCodeEndToEnd(t *testing.T) {
    cacheRoot := t.TempDir()
    capsDir := t.TempDir()

    writeCacheFixture(t, cacheRoot, "roo-code", "skills.go.0", map[string]capmon.FieldValue{
        "Skill.Name":          {Value: "name", ValueHash: "sha256:f1"},
        "Skill.Description":   {Value: "description", ValueHash: "sha256:g2"},
        "Skill.License":       {Value: "license", ValueHash: "sha256:h3"},
        "Skill.Compatibility": {Value: "compatibility", ValueHash: "sha256:i4"},
        "Skill.Metadata":      {Value: "metadata", ValueHash: "sha256:j5"},
    })

    extracted, err := capmon.LoadAndRecognizeCache(cacheRoot, "roo-code")
    if err != nil {
        t.Fatalf("LoadAndRecognizeCache: %v", err)
    }
    if len(extracted) == 0 {
        t.Fatal("LoadAndRecognizeCache returned empty map for roo-code with GoStruct fixtures")
    }

    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "roo-code",
        Extracted:      extracted,
        SeederSpecsDir: "",
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Fatalf("SeedProviderCapabilities: %v", err)
    }

    data, err := os.ReadFile(filepath.Join(capsDir, "roo-code.yaml"))
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    out := string(data)

    checks := []struct {
        want string
        desc string
    }{
        {"skills:", "skills content type"},
        {"supported: true", "skills supported"},
        {"display_name:", "display_name canonical key"},
        {"project_scope:", "project_scope capability"},
        {"canonical_filename:", "canonical_filename capability"},
        {"confidence: confirmed", "confidence field written"},
    }
    for _, c := range checks {
        if !strings.Contains(out, c.want) {
            t.Errorf("output missing %s (%q)\nFull output:\n%s", c.desc, c.want, out)
        }
    }
}

func TestSeedIntegration_UnknownProviderWarns(t *testing.T) {
    cacheRoot := t.TempDir()
    capsDir := t.TempDir()

    // Write some fields for a provider with no registered recognizer
    // (All 15 are registered after Phase 5, so use empty fields + stub)
    // After Phase 5, all providers return empty maps from stubs.
    // Test that seeding with empty extracted still produces a valid YAML.
    opts := capmon.SeedOptions{
        CapsDir:        capsDir,
        Provider:       "amp",
        Extracted:      map[string]string{},
        SeederSpecsDir: "",
    }
    if err := capmon.SeedProviderCapabilities(opts); err != nil {
        t.Errorf("seed with empty extracted should succeed: %v", err)
    }
    // Output should exist (even if minimal)
    data, err := os.ReadFile(filepath.Join(capsDir, "amp.yaml"))
    if err != nil {
        t.Fatalf("read output: %v", err)
    }
    if len(data) == 0 {
        t.Error("expected non-empty YAML output for amp even with empty extracted")
    }
}
```

**Run (expect PASS — Green, given Phase 6 is complete):**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/ -run TestSeedIntegration
```

---

### Phase 7 Gate

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

**Final coverage check:**
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | tail -5
```

Target: 80% minimum for all capmon packages.

**Final state verification:**

After Phase 7, verify these conditions hold:

1. `TestAllProviderSlugsRegistered` — PASSES (all 15 providers registered)
2. `TestAllRegisteredRecognizersReturnMap` — PASSES (all 15 stubs return non-nil maps)
3. `TestRecognizeCrushSkills_*` — PASSES (crush returns canonical keys with confidence)
4. `TestRecognizeRooCodeSkills_*` — PASSES (roo-code returns canonical keys with confidence)
5. `TestSeedIntegration_CrushEndToEnd` — PASSES (full pipeline produces correct YAML)
6. `TestSeedIntegration_RooCodeEndToEnd` — PASSES (full pipeline produces correct YAML)
7. `TestLoadSeederSpec_*` — PASSES
8. `TestValidateSeederSpec_*` — PASSES
9. `TestCapmonValidateSpecCmd_*` — PASSES
10. `TestSeedValidatesSeederSpec_*` — PASSES
11. `TestCapabilityEntry_ConfidenceField` — PASSES
12. `TestSeedProviderCapabilities_WritesConfidence` — PASSES
13. Build: `make build` — PASSES

---

## Summary of Files Created/Modified

### New Files
| File | Phase | Purpose |
|------|-------|---------|
| `cli/internal/capmon/seederspec.go` | 2 | `SeederSpec`, `ProposedMapping`, `LoadSeederSpec`, `ValidateSeederSpec` |
| `cli/internal/capmon/seederspec_test.go` | 2 | Tests for seederspec.go |
| `cli/internal/capmon/seed_spec_gate_test.go` | 3 | Tests for spec gate in seed |
| `cli/cmd/syllago/capmon_validate_spec_cmd_test.go` | 2 | Tests for validate-spec command |
| `docs/workflows/inspect-provider-skills.md` | 4 | Inspection bead workflow document |
| `cli/internal/capmon/recognize_registry_test.go` | 1 | `TestAllProviderSlugsRegistered` |
| `cli/internal/capmon/recognize_stubs_test.go` | 5 | `TestAllRegisteredRecognizersReturnMap` |
| `cli/internal/capmon/recognize_crush_test.go` | 6 | Tests for crush recognizer |
| `cli/internal/capmon/recognize_roo_code_test.go` | 6 | Tests for roo-code recognizer |
| `cli/internal/capmon/seed_integration_test.go` | 7 | End-to-end seed integration tests |
| `cli/internal/capmon/recognize_amp.go` | 5 | Amp stub |
| `cli/internal/capmon/recognize_claude_code.go` | 5 | Claude Code stub |
| `cli/internal/capmon/recognize_cline.go` | 5 | Cline stub |
| `cli/internal/capmon/recognize_codex.go` | 5 | Codex stub |
| `cli/internal/capmon/recognize_copilot_cli.go` | 5 | Copilot CLI stub |
| `cli/internal/capmon/recognize_crush.go` | 5+6 | Crush stub → real implementation |
| `cli/internal/capmon/recognize_cursor.go` | 5 | Cursor stub |
| `cli/internal/capmon/recognize_factory_droid.go` | 5 | Factory Droid stub |
| `cli/internal/capmon/recognize_gemini_cli.go` | 5 | Gemini CLI stub |
| `cli/internal/capmon/recognize_kiro.go` | 5 | Kiro stub |
| `cli/internal/capmon/recognize_opencode.go` | 5 | Opencode stub |
| `cli/internal/capmon/recognize_pi.go` | 5 | Pi stub |
| `cli/internal/capmon/recognize_roo_code.go` | 5+6 | Roo Code stub → real implementation |
| `cli/internal/capmon/recognize_windsurf.go` | 5 | Windsurf stub |
| `cli/internal/capmon/recognize_zed.go` | 5 | Zed stub |

### Modified Files
| File | Phase | Change |
|------|-------|--------|
| `cli/internal/capmon/capyaml/types.go` | 0 | Add `Confidence string` to `CapabilityEntry` |
| `cli/internal/capmon/capyaml/load_test.go` | 0 | Add `TestCapabilityEntry_ConfidenceField` |
| `cli/internal/capmon/seed.go` | 0, 3 | Add confidence dot-path handling; add `SeederSpecsDir` + spec gate |
| `cli/internal/capmon/seed_test.go` | 0 | Add `TestSeedProviderCapabilities_WritesConfidence` |
| `cli/internal/capmon/recognize.go` | 1 | Full rewrite: registry, updated signature, `canonicalKeyFromYAMLKey`, `capabilityDotPaths` |
| `cli/internal/capmon/recognize_test.go` | 1, 6 | Update signature calls; update assertions for canonical keys + confidence |
| `cli/cmd/syllago/capmon_cmd.go` | 2, 3 | Add `capmonValidateSpecCmd`; add `--skip-spec-gate`; add `capmonSeederSpecsDirOverride`; pass `SeederSpecsDir` |

---

## Gotchas and Implementation Notes

### `recognizeSkillsGoStruct` key name change
In Phase 1, `recognizeSkillsGoStruct` changes from `"frontmatter_" + yamlKey` to `canonicalKeyFromYAMLKey(yamlKey)`. This changes the output key names from `frontmatter_name` to `display_name`, etc. The existing `seed_test.go` uses hardcoded dot-paths like `frontmatter_name` — these are passed directly to the seeder (bypassing the recognizer) and do not need updating. However, the actual capability YAMLs for crush and roo-code in `docs/provider-capabilities/` will have stale `frontmatter_*` keys until a human runs `syllago capmon seed --provider=crush --skip-spec-gate`. This is expected and documented.

### `strings.HasPrefix` vs. manual prefix check
The original `recognizeSkillsGoStruct` uses `strings.HasPrefix(k, "Skill.")`. In the rewrite, if `strings` is removed from the imports (since `recognize.go` no longer needs it for other uses), a manual check `len(k) >= 6 && k[:6] == "Skill."` avoids the import. Either approach is correct — use whichever keeps the code clean.

### `capmon_cmd.go` import for `capmon.SeederSpecPath`
The `capmonValidateSpecCmd` calls `capmon.SeederSpecPath(...)`. This is a new exported function in `seederspec.go`. Ensure the import is present in `capmon_cmd.go` — the `capmon` package is already imported.

### `ExtractedSource` is unexported in tests
The `ExtractedSource` type is defined in `cli/internal/capmon/types.go`. The integration test in Phase 7 uses `capmon.ExtractedSource{}` in `package capmon_test`, so it must be exported. Confirm `ExtractedSource` is already exported (it is — capital E).

### `os.Stat` vs. `os.IsNotExist`
In Task 3.2, the spec gate uses `os.Stat` to check if the spec file exists before calling `LoadSeederSpec`. This is more readable than unwrapping errors. `LoadSeederSpec` wraps the underlying `os.ReadFile` error with `fmt.Errorf("read seeder spec %s: %w", path, err)`, so `errors.Is(err, os.ErrNotExist)` works via `%w`, but the `os.Stat` approach avoids confusion.

### Test parallelism
The stub recognizer files all modify the package-level `recognizerRegistry` via `init()`. Since `init()` runs before any test, and tests only read the registry (they never write to it), the registry is safe to read in parallel tests. Tests that call `RecognizeContentTypeDotPaths` can use `t.Parallel()`.

### `make gendocs` after adding CLI flag
After adding `--skip-spec-gate` and `--content-type` flags (Phase 2, Phase 3), regenerate `commands.json`:
```bash
cd /home/hhewett/.local/src/syllago/cli && ./syllago _gendocs > ../commands.json 2>/dev/null || true
```
If this fails (binary not built yet), run `make build` first.

### Coverage target
The `capmon` package currently has significant test coverage from existing tests. Adding seederspec.go, 15 stub files, and integration tests should keep coverage above 80%. The stub recognizer files (`recognize_amp.go`, etc.) each contain two lines of testable code — the `init()` and the stub function. They are exercised by `TestAllRegisteredRecognizersReturnMap` and `TestAllProviderSlugsRegistered`.

---

## Execution Order Summary

| Phase | Key Deliverable | Test Gate |
|-------|-----------------|-----------|
| 0 | `CapabilityEntry.Confidence` field + seeder writes it | `TestCapabilityEntry_ConfidenceField`, `TestSeedProviderCapabilities_WritesConfidence` |
| 1 | `init()`-based registry, updated `RecognizeContentTypeDotPaths` signature | `TestRecognizeContentTypeDotPaths_*` pass; `TestAllProviderSlugsRegistered` Red (expected) |
| 2 | `SeederSpec` struct, `LoadSeederSpec`, `ValidateSeederSpec`, `validate-spec` command | `TestLoadSeederSpec_*`, `TestValidateSeederSpec_*`, `TestCapmonValidateSpecCmd_*` |
| 3 | Spec gate in seed command, `SeederSpecsDir` field | `TestSeedValidatesSeederSpec_*` |
| 4 | Inspection bead workflow document | File exists, no code tests |
| 5 | 15 provider stub registrations | `TestAllProviderSlugsRegistered` Green |
| 6 | Crush + roo-code real recognizers with canonical keys + confidence | `TestRecognizeCrushSkills_*`, `TestRecognizeRooCodeSkills_*` |
| 7 | End-to-end seed integration | `TestSeedIntegration_*` |
| 8 | Provider onboarding docs | `docs/adding-a-provider.md` exists |

---

## Phase 8 — Provider Onboarding Documentation

**Goal:** Write `docs/adding-a-provider.md` and verify the `validate-spec` command works for a hypothetical net-new provider. This phase is documentation-only — no Go code, no tests.

### Task 8.1 — Write `docs/adding-a-provider.md`

**File to create:** `docs/adding-a-provider.md`

The document must cover the full onboarding sequence from the design doc:

```markdown
# Adding a New Provider to Syllago

## Prerequisites
- Provider slug (kebab-case, matches filesystem convention)
- Source documentation URLs

## Steps

### 1. Create the provider source manifest
Write `docs/provider-sources/<slug>.yaml` manually. Use `docs/provider-sources/claude-code.yaml` as a template.

### 2. Create or verify the format reference doc
Write `docs/provider-formats/<slug>.md` — the human-authored ground truth for this provider's skills format.
If you don't have one yet, the inspection bead (Step 4) will generate a draft.

### 3. Fetch and extract
\`\`\`bash
syllago capmon run --stage=fetch-extract --provider=<slug>
\`\`\`

### 4. Run the inspection bead
Follow the workflow in `docs/workflows/inspect-provider-skills.md` with `--provider=<slug>`.
This produces `.develop/seeder-specs/<slug>-skills.yaml`.

### 5. Review and approve the seeder spec
Open `.develop/seeder-specs/<slug>-skills.yaml`.
Review `proposed_mappings`. Set `human_action: approve` and `reviewed_at: <ISO timestamp>`.
Optionally run: `syllago capmon validate-spec --provider=<slug>`

### 6. Implement the recognizer
Implement `recognizeXxxSkills()` in `cli/internal/capmon/recognize_<slug_underscored>.go`
using the approved seeder spec as the source of truth.

### 7. Seed the provider
\`\`\`bash
syllago capmon seed --provider=<slug>
\`\`\`

### 8. Verify output
Check `docs/provider-capabilities/<slug>.yaml` for a populated `content_types.skills` section
with `confidence: confirmed` entries.
```

### Task 8.2 — Smoke-test with scratch provider

Manually verify that the validate-spec command handles an unknown provider gracefully (no panic, clear error):

```bash
# Create a minimal scratch spec
mkdir -p .develop/seeder-specs
cat > .develop/seeder-specs/scratch-test-skills.yaml << 'EOF'
provider: scratch-test
content_type: skills
format: markdown
format_doc_provenance: human
extraction_gaps: []
source_excerpt: ""
proposed_mappings: []
human_action: ""
reviewed_at: ""
notes: ""
EOF

# Validate with empty human_action — should error
syllago capmon validate-spec --provider=scratch-test

# Set approval and re-validate — should pass
# (edit human_action and reviewed_at in the file, then re-run)

# Clean up
rm .develop/seeder-specs/scratch-test-skills.yaml
```

Expected: first run errors with "seeder spec for scratch-test has not been reviewed"; approved run succeeds.

### Phase 8 Gate

No build gate — this phase is documentation only. Verify:
- `docs/adding-a-provider.md` exists and covers all 8 steps
- Smoke-test above ran without unexpected errors
