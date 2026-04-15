# CLI Provider Extension Follow-ups — Implementation Plan

**Status:** Ready for execution
**Date:** 2026-04-14
**Design doc:** `../../../syllago-docs/docs/plans/2026-04-14-provider-pages-redesign-design.md`
**Author:** Holden Hewett + Maive

---

## 1. Overview

This plan covers four CLI-side tasks that follow from the provider pages redesign design document (decisions D1, D13, D14, D16). They all touch the `docs/provider-formats/*.yaml` schema and downstream tooling — the aggregator, the validator, and the `syllago info` command.

### What these tasks deliver

- **D1** — Three new optional fields (`required`, `value_type`, `examples`) on `ProviderExtension` in both the YAML schema (Go struct `capmon.ProviderExtension` in `formatdoc.go`) and the JSON output struct (`gencapabilities.go`). 14 YAML files get the new field slots added (empty/null is valid — no flag day).
- **D13** — Allow-list validation in `capmon/formatdoc_validate.go` for `value_type`, `example.lang`, and `sources[].section`. Produces **warnings**, not build-breaking errors.
- **D14** — GitHub issue automation driven by those warnings. CI uses `GITHUB_TOKEN` to open/update/close issues; local builds log to stderr. **Integrated into `syllago-jtafb`** (see Section 2 below).
- **D16** — A `data_quality` block in `capabilities.json` output and a footer in `syllago info providers <slug>` (new sub-subcommand). Also relies on D14's issue infrastructure for the per-provider tracking issue URL.

### Dependency graph

```
D1 (schema + YAML + gencapabilities)
  └── D13 (allow-list warnings) — depends on D1
        └── D14 (GH issue automation) — depends on D1 + D13; integrated into syllago-jtafb
  └── D16 (data_quality + info footer) — depends on D1; independent of D13/D14
```

D16 can proceed in parallel with D13 once D1 is complete. D14 is a delta inside `syllago-jtafb` work.

### Files touched per task

| Task | Files modified |
|------|---------------|
| D1 | `cli/internal/capmon/formatdoc.go`, `cli/cmd/syllago/gencapabilities.go`, `cli/cmd/syllago/gencapabilities_test.go`, `cli/internal/capmon/formatdoc_validate.go`, `cli/internal/capmon/formatdoc_validate_test.go`, all 14 `docs/provider-formats/*.yaml` |
| D13 | `cli/internal/capmon/formatdoc_validate.go`, `cli/internal/capmon/formatdoc_validate_test.go`, `cli/cmd/syllago/capmon_validate_format_doc_cmd.go`, `cli/cmd/syllago/capmon_validate_format_doc_cmd_test.go` |
| D14 | Delta inside `syllago-jtafb` execution — see Section 2 |
| D16 | `cli/cmd/syllago/gencapabilities.go`, `cli/cmd/syllago/gencapabilities_test.go`, `cli/cmd/syllago/info.go`, `cli/cmd/syllago/info_test.go` |

---

## 2. D14 Overlap Analysis

### What `syllago-jtafb` covers

`syllago-jtafb` (Capability monitor pipeline) is an in-progress epic that implements the full capmon pipeline: `capmon fetch`, `capmon extract`, `capmon diff`, `capmon derive`, `capmon check`, `capmon onboard`, and the CI workflow (`capmon-check.yml`). The design is finalized (67 beads created) and Phase 1 execution is starting. The pipeline's `capmon check` step already opens GitHub issues for drift: it calls the GitHub API to file PRs and/or issues when capability data has changed significantly. The warning-triggered issue flow for D14 is structurally identical to what `capmon check` already does.

### Overlap determination

D14 (auto-file GitHub issues from validation warnings) is **not standalone work**. It is a delta task inside `syllago-jtafb` for these reasons:

1. The GitHub API integration (auth via `GITHUB_TOKEN`, local-build fallback to stderr, dedup by keyed hash, auto-close on clean build) is infrastructure that `capmon check` is already building. Adding D14 reuses that infrastructure rather than duplicating it.
2. D14's warning source is `ValidateFormatDoc` warnings — these warnings surface during `capmon check`'s Step 2 (`capmon validate-format-doc`). The natural place to consume them and open issues is inside `capmon check`, not as an independent command.
3. `syllago-jtafb` already has a bead for `capmon validate-format-doc`. The D14 delta is: extend the issue-opening logic that `capmon check` uses to also open issues for warnings (not just drift).

### Concrete answer: integrate into jtafb

D14 is a **delta task inside `syllago-jtafb`**. The plan section below (Section 6) describes only the delta: the warning-collection interface that `formatdoc_validate.go` must expose, and the calling convention inside `capmon check` that consumes it. The GitHub API client, dedup logic, and close-on-clean behavior are implemented in the `syllago-jtafb` bead chain, not in this plan.

**Dedup key specification (resolved here for jtafb implementers):**

```
key = sha256(file + "\x00" + field + "\x00" + value)
```

Where `file` is the absolute path to the YAML file that triggered the warning, `field` is the dotted field path (e.g., `content_types.skills.provider_extensions[cool_feature].value_type`), and `value` is the offending string value. Encode as the first 16 hex characters of the SHA-256 digest for the issue title prefix (e.g., `[capmon-warn-3a7f4b2e1c9d0f6a]`).

---

## 3. D1 Tasks — ProviderExtension Schema Expansion

### D1.1 — Extend `ProviderExtension` Go struct in `formatdoc.go`

**File:** `cli/internal/capmon/formatdoc.go`
**Depends on:** nothing
**Time estimate:** 2 minutes

Add three optional fields to `ProviderExtension`. The `Required` field uses `*bool` (pointer) so that `null` (unset) is distinguishable from `false`. `ValueType` is a plain string. `Examples` is a slice of `ExtensionExample`.

Add the new `ExtensionExample` type immediately above `ProviderExtension` in the file. Add the three new fields after `GraduationNotes` in `ProviderExtension`.

```go
// ExtensionExample is a single usage example for a ProviderExtension.
// The lang field must match the allow-list in formatdoc_validate.go.
type ExtensionExample struct {
    Title string `yaml:"title,omitempty"` // optional caption
    Lang  string `yaml:"lang"`            // required, e.g., "yaml", "json"
    Code  string `yaml:"code"`            // required, non-empty
    Note  string `yaml:"note,omitempty"`  // optional prose annotation
}
```

Fields to add to `ProviderExtension`:
```go
Required    *bool              `yaml:"required"`              // nullable bool
ValueType   string             `yaml:"value_type,omitempty"`
Examples    []ExtensionExample `yaml:"examples,omitempty"`
```

**Verification:** `cd cli && go build ./...` must succeed with no errors.

### D1.2 — Add corresponding test for the new struct fields (TDD red step)

**File:** `cli/internal/capmon/formatdoc_test.go` (new file if it doesn't exist, or extend existing)
**Depends on:** D1.1
**Time estimate:** 3 minutes

Write a test that round-trips a YAML snippet containing all three new fields through `LoadFormatDoc`. Verify:
- `required: true` → `*bool` pointing to `true`
- `required: false` → `*bool` pointing to `false`
- `required` absent → `nil` pointer
- `value_type: "string"` → `ValueType == "string"`
- `examples` with title, lang, code, note → fully populated `ExtensionExample` slice

```go
func TestProviderExtension_NewFieldRoundTrip(t *testing.T) {
    t.Parallel()
    yamlContent := `
provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: model_field
        name: Model Field
        description: "Controls which model is used."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - title: "Fast model"
            lang: yaml
            code: |
              model: claude-haiku
            note: "Default if absent."
      - id: optional_field
        name: Optional Field
        description: "An optional capability."
        source_ref: "https://example.com"
        required: false
      - id: unspecified_field
        name: Unspecified Field
        description: "We do not know if this is required."
        source_ref: "https://example.com"
`
    dir := t.TempDir()
    path := filepath.Join(dir, "test-provider.yaml")
    if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
        t.Fatal(err)
    }
    doc, err := LoadFormatDoc(path)
    if err != nil {
        t.Fatalf("LoadFormatDoc: %v", err)
    }
    exts := doc.ContentTypes["skills"].ProviderExtensions
    if len(exts) != 3 {
        t.Fatalf("expected 3 extensions, got %d", len(exts))
    }

    // required: true
    if exts[0].Required == nil || *exts[0].Required != true {
        t.Errorf("exts[0].Required: want *true, got %v", exts[0].Required)
    }
    if exts[0].ValueType != "string" {
        t.Errorf("exts[0].ValueType = %q, want %q", exts[0].ValueType, "string")
    }
    if len(exts[0].Examples) != 1 {
        t.Fatalf("exts[0].Examples: want 1, got %d", len(exts[0].Examples))
    }
    ex := exts[0].Examples[0]
    if ex.Title != "Fast model" || ex.Lang != "yaml" || ex.Note != "Default if absent." {
        t.Errorf("example fields wrong: %+v", ex)
    }
    if ex.Code == "" {
        t.Error("example.code must be non-empty")
    }

    // required: false
    if exts[1].Required == nil || *exts[1].Required != false {
        t.Errorf("exts[1].Required: want *false, got %v", exts[1].Required)
    }

    // required absent → nil
    if exts[2].Required != nil {
        t.Errorf("exts[2].Required: want nil, got %v", exts[2].Required)
    }
}
```

Run: `cd cli && go test ./internal/capmon/ -run TestProviderExtension_NewFieldRoundTrip` — expect **PASS** after D1.1.

### D1.3 — Extend `gencapabilities.go` YAML and JSON structs

**File:** `cli/cmd/syllago/gencapabilities.go`
**Depends on:** D1.1
**Time estimate:** 4 minutes

Two structs need updating in the same file in a single Edit call (per Go import rule):

**YAML input struct `capExtensionYAML`** — add three new fields:
```go
Required    *bool                    `yaml:"required"`
ValueType   string                   `yaml:"value_type,omitempty"`
Examples    []capExtensionExampleYAML `yaml:"examples,omitempty"`
```

Also add the `capExtensionExampleYAML` struct (new, placed before `capExtensionYAML`):
```go
// capExtensionExampleYAML is one entry in provider_extensions[i].examples.
type capExtensionExampleYAML struct {
    Title string `yaml:"title,omitempty"`
    Lang  string `yaml:"lang"`
    Code  string `yaml:"code"`
    Note  string `yaml:"note,omitempty"`
}
```

**JSON output struct `CapExtension`** — add three new fields:
```go
Required   *bool              `json:"required"`           // null when unset
ValueType  string             `json:"value_type,omitempty"`
Examples   []CapExampleEntry  `json:"examples,omitempty"`
```

Also add the `CapExampleEntry` struct (new, placed before `CapExtension`):
```go
// CapExampleEntry is the public-facing example entry in provider_extensions.
type CapExampleEntry struct {
    Title string `json:"title,omitempty"`
    Lang  string `json:"lang"`
    Code  string `json:"code"`
    Note  string `json:"note,omitempty"`
}
```

**Update `buildCapEntry`** — in the extension-building loop, propagate the new fields:
```go
examples := make([]CapExampleEntry, 0, len(ext.Examples))
for _, ex := range ext.Examples {
    examples = append(examples, CapExampleEntry{
        Title: ex.Title,
        Lang:  ex.Lang,
        Code:  ex.Code,
        Note:  ex.Note,
    })
}
extensions = append(extensions, CapExtension{
    ID:          ext.ID,
    Name:        ext.Name,
    Description: strings.TrimSpace(ext.Description),
    SourceRef:   ext.SourceRef,
    Required:    ext.Required,
    ValueType:   ext.ValueType,
    Examples:    examples,
})
```

Note: `Required` must use `*bool` in both YAML and JSON structs. The JSON tag must be `json:"required"` (no `omitempty`) so that `null` is emitted explicitly for unset fields — downstream consumers (syllago-docs Zod schema) rely on `null` vs absence for the three-state badge.

**Verification:** `cd cli && go build ./... && go test ./cmd/syllago/ -run TestGencapabilities` — all existing tests must pass.

### D1.4 — Add gencapabilities tests for the new extension fields

**File:** `cli/cmd/syllago/gencapabilities_test.go`
**Depends on:** D1.3
**Time estimate:** 4 minutes

Add a YAML fixture constant with all three new fields and write four test functions:

```go
const extensionWithNewFieldsYAML = `
provider: rich-extension
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: model_field
        name: Model Field
        description: "Which model to use."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - lang: yaml
            code: "model: claude-haiku"
            note: "Fast tier."
      - id: opt_field
        name: Optional Field
        description: "An optional thing."
        source_ref: "https://example.com"
        required: false
      - id: unspec_field
        name: Unspecified Field
        description: "Unknown required status."
        source_ref: "https://example.com"
        graduation_candidate: false
`
```

Test functions:

**`TestGencapabilities_ExtensionRequiredFieldTrue`** — verify `exts[0].Required` is non-nil pointer to `true`.

**`TestGencapabilities_ExtensionRequiredFieldFalse`** — verify `exts[1].Required` is non-nil pointer to `false`.

**`TestGencapabilities_ExtensionRequiredFieldNull`** — load unspec_field, verify `Required == nil`. Also check that the raw JSON output contains `"required":null` (not absent).

```go
func TestGencapabilities_ExtensionRequiredFieldNull(t *testing.T) {
    dir := capFixtureDir(t, map[string]string{
        "rich-extension.yaml": extensionWithNewFieldsYAML,
    })
    raw := captureStdout(t, func() {
        origDir := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = origDir }()
        origSpec := canonicalKeysSpecPath
        canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
        defer func() { canonicalKeysSpecPath = origSpec }()
        gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
    })
    if !strings.Contains(string(raw), `"required":null`) {
        t.Error("extension with absent required must emit \"required\":null in JSON")
    }
}
```

**`TestGencapabilities_ExtensionExamplesPassthrough`** — verify examples lang/code/note fields are present in the manifest.

**`TestGencapabilities_GraduationCandidateStillNotEmitted`** — regression: adding new fields must not accidentally re-emit `graduation_candidate`.

Run: `cd cli && go test ./cmd/syllago/ -run TestGencapabilities` — all must pass.

### D1.5 — Add `name` and `section` fields to `CapSource` (D9 from design doc)

**File:** `cli/cmd/syllago/gencapabilities.go`, `cli/internal/capmon/formatdoc.go`
**Depends on:** D1.3
**Time estimate:** 3 minutes

D9 adds two optional fields to the `sources[]` schema. These are separate from the extension fields but part of the D1 bead's scope (the design doc lists them in the same D1/D9 section of the architecture sketch).

In `formatdoc.go`, extend `SourceRef`:
```go
Name    string `yaml:"name,omitempty"`
Section string `yaml:"section,omitempty"`
```

In `gencapabilities.go`, extend `capSourceYAML` (the YAML input):
```go
Name    string `yaml:"name,omitempty"`
Section string `yaml:"section,omitempty"`
```

Extend `CapSource` (the JSON output):
```go
Name    string `json:"name,omitempty"`
Section string `json:"section,omitempty"`
```

Update `buildCapEntry`'s source-building loop to propagate `Name` and `Section`:
```go
sources = append(sources, CapSource{
    URI:       s.URI,
    Type:      s.Type,
    FetchedAt: s.FetchedAt,
    Name:      s.Name,
    Section:   s.Section,
})
```

Add one test: `TestGencapabilities_SourceNameAndSectionPassthrough` — write a fixture YAML with `name` and `section` on a source, verify they appear in the manifest.

### D1.6 — Validate `formatdoc_validate.go` still passes with new fields present

**File:** `cli/internal/capmon/formatdoc_validate.go`
**Depends on:** D1.1
**Time estimate:** 2 minutes

`ValidateFormatDoc` currently checks `provider_extensions` for `id`, `name`, `description`, `source_ref`. The three new fields are optional — no change to the error rules is needed at this stage. This task is a verification task only:

Run the existing test suite: `cd cli && go test ./internal/capmon/ -run TestValidateFormatDoc` — all must pass without modification. If any test fails due to the struct changes in D1.1, fix the test fixture (not the validator logic).

### D1.7 — Update the 14 provider YAML files with new field slots

**Files:** All 14 `docs/provider-formats/*.yaml`
**Depends on:** D1.1
**Time estimate:** 10 minutes (parallel across files)

For each YAML file, add an empty comment block after each `provider_extensions` entry that indicates the three new optional fields. The fields are **not required** (parallel rollout — D3 from design doc), so adding them as absent (not written) is valid. However, the capmon subagent will need the field slots documented in a schema comment to know what to populate.

The change per file is minimal: add a YAML comment block once per file (not per extension) noting that the three new optional fields are available:

```yaml
# Optional extension depth fields (populated by capmon subagent):
# - required: true | false | null (null = source doesn't say)
# - value_type: see allow-list in formatdoc_validate.go
# - examples: [{title?, lang, code, note?}]
```

Place this comment block at the top of the first `provider_extensions:` list in each file, or in a file-level comment if the file has no extensions.

Verification: `cd cli && go test ./internal/capmon/ -run TestLoadFormatDoc` — loading each YAML via `LoadFormatDoc` must succeed (comments are inert to YAML parser).

Additionally, enrich the `claude-code.yaml` skills section's first extension (`supporting_files`) with the new fields as a concrete example for the subagent to follow in future runs:
```yaml
      - id: supporting_files
        name: Supporting Files
        description: >
          ...existing description...
        source_ref: "https://code.claude.com/docs/en/skills.md#add-supporting-files"
        graduation_candidate: true
        required: false
        value_type: "path"
        examples:
          - lang: yaml
            code: "# No YAML field; this is a filesystem convention"
            note: "Supporting files live in the skill directory alongside SKILL.md."
```

This provides a concrete precedent without flag-daying all 14 files.

### D1 Phase validation

After completing D1.1–D1.7:

```bash
cd cli && make fmt && make build && go test ./internal/capmon/... ./cmd/syllago/... -count=1
```

All tests must pass. Run coverage check:
```bash
cd cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
```

Target: ≥80% for `capmon` package.

---

## 4. D16 Tasks — Data-Quality Block and Info Footer

D16 can start once D1.3 is complete (the output structs must carry `Required`/`ValueType`/`Examples` for the counts to make sense).

### D16.1 — Add `DataQuality` types to `gencapabilities.go`

**File:** `cli/cmd/syllago/gencapabilities.go`
**Depends on:** D1.3
**Time estimate:** 3 minutes

Add two new types in a single Edit call (to avoid goimports stripping unused imports):

```go
// DataQualityEntry holds unspecified-field counts for one provider.
type DataQualityEntry struct {
    UnspecifiedRequiredCount   int    `json:"unspecified_required_count"`
    UnspecifiedValueTypeCount  int    `json:"unspecified_value_type_count"`
    UnspecifiedExamplesCount   int    `json:"unspecified_examples_count"`
    TrackingIssue              string `json:"tracking_issue,omitempty"`
}

// DataQuality is the top-level data_quality block in capabilities.json.
type DataQuality struct {
    Providers map[string]DataQualityEntry `json:"providers"`
}
```

Update `CapabilitiesManifest` to include `DataQuality`:
```go
type CapabilitiesManifest struct {
    Version       string                                 `json:"version"`
    GeneratedAt   string                                 `json:"generated_at"`
    DataQuality   DataQuality                            `json:"data_quality"`
    CanonicalKeys map[string]map[string]CanonicalKeyMeta `json:"canonical_keys"`
    Providers     map[string]map[string]CapContentType   `json:"providers"`
}
```

The `TrackingIssue` URL is populated by D14's GitHub issue infrastructure. For D16's initial implementation, it is left empty (zero-value `""` → `omitempty` omits it from JSON output). When D14 is implemented inside `syllago-jtafb`, the capmon pipeline will write the issue URL into the format doc (or pass it to `gencapabilities` via a side file), and this field will be populated.

### D16.2 — Compute `DataQuality` counts in `buildCapEntry` and `runGencapabilities`

**File:** `cli/cmd/syllago/gencapabilities.go`
**Depends on:** D16.1
**Time estimate:** 4 minutes

Add a `computeDataQuality` function that iterates the built providers map and tallies counts:

```go
// computeDataQuality builds the data_quality block from the already-built providers map.
func computeDataQuality(providers map[string]map[string]CapContentType) DataQuality {
    dq := DataQuality{Providers: make(map[string]DataQualityEntry)}
    for slug, contentTypes := range providers {
        var entry DataQualityEntry
        for _, ct := range contentTypes {
            for _, ext := range ct.ProviderExtensions {
                if ext.Required == nil {
                    entry.UnspecifiedRequiredCount++
                }
                if ext.ValueType == "" {
                    entry.UnspecifiedValueTypeCount++
                }
                if len(ext.Examples) == 0 {
                    entry.UnspecifiedExamplesCount++
                }
            }
        }
        dq.Providers[slug] = entry
    }
    return dq
}
```

Update `runGencapabilities` to call it:
```go
manifest := CapabilitiesManifest{
    Version:       "1",
    GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
    DataQuality:   computeDataQuality(entries),
    CanonicalKeys: canonicalKeys,
    Providers:     entries,
}
```

### D16.3 — Write tests for `computeDataQuality` and the manifest structure

**File:** `cli/cmd/syllago/gencapabilities_test.go`
**Depends on:** D16.2
**Time estimate:** 4 minutes

Add `extensionWithMixedQualityYAML` fixture (one extension with all fields set, one with none):

```go
const extensionWithMixedQualityYAML = `
provider: quality-test
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: full_ext
        name: Full Extension
        description: "Has all depth fields."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - lang: yaml
            code: "model: x"
      - id: bare_ext
        name: Bare Extension
        description: "Has no depth fields."
        source_ref: "https://example.com"
`
```

Tests:

**`TestGencapabilities_DataQualityBlockPresent`** — verify `data_quality` key exists in raw JSON output.

**`TestGencapabilities_DataQualityCountsAccurate`** — load the fixture, check that `quality-test` has `unspecified_required_count == 1`, `unspecified_value_type_count == 1`, `unspecified_examples_count == 1` (the bare_ext counts; full_ext does not).

**`TestGencapabilities_DataQualityAllZero`** — load a fixture where all extensions have all three fields; counts must all be zero.

**`TestGencapabilities_DataQualityGeneratedAtIsRFC3339UTC`** — verify `generated_at` matches `time.RFC3339` and ends with `Z`.

```go
func TestGencapabilities_DataQualityGeneratedAtIsRFC3339UTC(t *testing.T) {
    dir := capFixtureDir(t, map[string]string{
        "quality-test.yaml": extensionWithMixedQualityYAML,
    })
    m := loadCapManifest(t, dir)
    if _, err := time.Parse(time.RFC3339, m.GeneratedAt); err != nil {
        t.Errorf("generated_at %q is not RFC3339: %v", m.GeneratedAt, err)
    }
    if !strings.HasSuffix(m.GeneratedAt, "Z") {
        t.Errorf("generated_at %q must end with Z (UTC)", m.GeneratedAt)
    }
}
```

### D16.4 — Add `syllago info providers <slug>` sub-subcommand

**File:** `cli/cmd/syllago/info.go`
**Depends on:** D16.2
**Time estimate:** 5 minutes

The design doc (D16) specifies a `syllago info providers <slug>` footer that prints:
- The provider's name, slug, and supported content types (existing providers list behavior).
- A data-quality summary line at the bottom.
- The tracking issue URL if set.

This requires a new `infoProvidersSlugCmd` subcommand that accepts one positional argument (the slug). It reads `capabilities.json` from a known location (the build artifact), or from a configurable path override for testing.

**Implementation approach:** Rather than reading the built `capabilities.json` (which may not be present in dev environments), call `computeDataQuality` directly from the providers map loaded from `docs/provider-formats/*.yaml`. This avoids a chicken-and-egg dependency on the artifact.

Add a package-level override variable for the formats dir (mirrors the pattern in `gencapabilities.go`):
```go
// infoProviderFormatsDir is overridable in tests.
var infoProviderFormatsDir = filepath.Join("..", "docs", "provider-formats")
```

Add the subcommand registration in `init()`:
```go
infoProvidersCmd.AddCommand(infoProvidersSlugCmd)
```

Define `infoProvidersSlugCmd`:
```go
var infoProvidersSlugCmd = &cobra.Command{
    Use:   "<slug>",
    Short: "Show data quality summary for a provider",
    Example: `  syllago info providers claude-code
  syllago info providers claude-code --json`,
    Args: cobra.ExactArgs(1),
    RunE: runInfoProvidersSlug,
}

func runInfoProvidersSlug(cmd *cobra.Command, args []string) error {
    slug := args[0]

    providers, err := loadProviderFormatsDir(infoProviderFormatsDir)
    if err != nil {
        return fmt.Errorf("loading provider formats: %w", err)
    }
    dq := computeDataQuality(providers)

    entry, ok := dq.Providers[slug]
    if !ok {
        return fmt.Errorf("provider %q not found in %s", slug, infoProviderFormatsDir)
    }

    if output.JSON {
        output.Print(map[string]any{
            "slug":                        slug,
            "unspecified_required_count":  entry.UnspecifiedRequiredCount,
            "unspecified_value_type_count": entry.UnspecifiedValueTypeCount,
            "unspecified_examples_count":  entry.UnspecifiedExamplesCount,
            "tracking_issue":              entry.TrackingIssue,
        })
        return nil
    }

    fmt.Fprintf(output.Writer, "Data quality for %s:\n", slug)
    fmt.Fprintf(output.Writer, "  Extensions missing required field:   %d\n", entry.UnspecifiedRequiredCount)
    fmt.Fprintf(output.Writer, "  Extensions missing value_type field:  %d\n", entry.UnspecifiedValueTypeCount)
    fmt.Fprintf(output.Writer, "  Extensions missing examples:          %d\n", entry.UnspecifiedExamplesCount)
    if entry.TrackingIssue != "" {
        fmt.Fprintf(output.Writer, "\n  Tracking issue: %s\n", entry.TrackingIssue)
    } else {
        fmt.Fprintf(output.Writer, "\n  Tracking issue: (not yet filed)\n")
    }
    return nil
}
```

Note: `computeDataQuality` is already defined in `gencapabilities.go` (D16.2), which is in `package main`. `info.go` is also `package main`, so this function is available without import.

### D16.5 — Write tests for `infoProvidersSlugCmd`

**File:** `cli/cmd/syllago/info_test.go`
**Depends on:** D16.4
**Time estimate:** 4 minutes

Use `capFixtureDir` from `gencapabilities_test.go` (same package, so accessible) to create a temp formats dir with a fixture YAML containing mixed quality data.

```go
func TestInfoProvidersSlug_TextOutput(t *testing.T) {
    dir := capFixtureDir(t, map[string]string{
        "quality-test.yaml": extensionWithMixedQualityYAML,
    })

    origDir := infoProviderFormatsDir
    infoProviderFormatsDir = dir
    t.Cleanup(func() { infoProviderFormatsDir = origDir })

    stdout, _ := output.SetForTest(t)
    err := infoProvidersSlugCmd.RunE(infoProvidersSlugCmd, []string{"quality-test"})
    if err != nil {
        t.Fatalf("infoProvidersSlugCmd failed: %v", err)
    }
    out := stdout.String()
    if !strings.Contains(out, "quality-test") {
        t.Error("output missing slug name")
    }
    if !strings.Contains(out, "1") {
        t.Error("output missing count of 1 for unspecified fields")
    }
}

func TestInfoProvidersSlug_JSONOutput(t *testing.T) {
    dir := capFixtureDir(t, map[string]string{
        "quality-test.yaml": extensionWithMixedQualityYAML,
    })
    origDir := infoProviderFormatsDir
    infoProviderFormatsDir = dir
    t.Cleanup(func() { infoProviderFormatsDir = origDir })

    output.JSON = true
    t.Cleanup(func() { output.JSON = false })

    stdout, _ := output.SetForTest(t)
    err := infoProvidersSlugCmd.RunE(infoProvidersSlugCmd, []string{"quality-test"})
    if err != nil {
        t.Fatalf("infoProvidersSlugCmd JSON failed: %v", err)
    }
    var result map[string]any
    if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
        t.Fatalf("invalid JSON: %v", err)
    }
    if result["slug"] != "quality-test" {
        t.Errorf("slug = %v, want quality-test", result["slug"])
    }
}

func TestInfoProvidersSlug_UnknownSlugError(t *testing.T) {
    dir := capFixtureDir(t, map[string]string{
        "quality-test.yaml": extensionWithMixedQualityYAML,
    })
    origDir := infoProviderFormatsDir
    infoProviderFormatsDir = dir
    t.Cleanup(func() { infoProviderFormatsDir = origDir })

    err := infoProvidersSlugCmd.RunE(infoProvidersSlugCmd, []string{"nonexistent-provider"})
    if err == nil {
        t.Fatal("expected error for unknown provider slug")
    }
    if !strings.Contains(err.Error(), "not found") {
        t.Errorf("error should mention 'not found', got: %v", err)
    }
}
```

### D16 Phase validation

```bash
cd cli && make fmt && make build && go test ./cmd/syllago/... -run "TestGencapabilities_DataQuality|TestInfoProvidersSlug" -v
```

All tests must pass. Then run the full suite:
```bash
cd cli && go test ./cmd/syllago/... -count=1
```

Coverage check:
```bash
cd cli && go test ./cmd/syllago/ -coverprofile=cov.out && go tool cover -func=cov.out | grep total
```

---

## 5. D13 Tasks — Allow-List Validation

D13 depends on D1 because the fields being validated (`value_type`, `example.lang`, `sources[].section`) are added in D1.

### D13.1 — Define allow-lists and warning collection type in `formatdoc_validate.go`

**File:** `cli/internal/capmon/formatdoc_validate.go`
**Depends on:** D1.1 (struct changes in `formatdoc.go`)
**Time estimate:** 3 minutes

Add allow-list constants and a `ValidationWarning` type. Add in a single Edit call (same file, no import issues):

```go
// allowedValueTypes is the controlled vocabulary for provider_extensions[i].value_type.
// Values outside this list produce a warning (not an error).
var allowedValueTypes = map[string]bool{
    "string":           true,
    "string[]":         true,
    "string | string[]": true,
    "bool":             true,
    "int":              true,
    "object":           true,
    "object[]":         true,
    "path":             true,
}

// allowedExampleLangs is the controlled vocabulary for provider_extensions[i].examples[j].lang.
var allowedExampleLangs = map[string]bool{
    "yaml":       true,
    "json":       true,
    "toml":       true,
    "bash":       true,
    "javascript": true,
    "typescript": true,
    "python":     true,
    "markdown":   true,
    "mdx":        true,
    "ini":        true,
    "dotenv":     true,
}

// allowedSourceSections is the controlled vocabulary for sources[i].section.
// Extension-specific values like "Extension: <field-name>" are component-generated,
// not authored in YAML, so they do not appear here.
var allowedSourceSections = map[string]bool{
    "All":                true,
    "Native Format":      true,
    "Canonical Mappings": true,
    "Extensions":         true,
}

// ValidationWarning is a non-fatal allow-list violation found in a format doc.
type ValidationWarning struct {
    File    string // absolute path to the YAML file
    Field   string // dotted field path, e.g., "content_types.skills.provider_extensions[model].value_type"
    Value   string // the offending value
    Message string // human-readable explanation
}

// DeduplicationKey returns the SHA-256-based key used to deduplicate GitHub issues for this warning.
// Key format: sha256(<file> + "\x00" + <field> + "\x00" + <value>), first 16 hex chars.
func (w ValidationWarning) DeduplicationKey() string {
    h := sha256.Sum256([]byte(w.File + "\x00" + w.Field + "\x00" + w.Value))
    return fmt.Sprintf("%x", h[:8])
}
```

This requires adding `"crypto/sha256"` to the imports — do it in the same Edit call.

### D13.2 — Extend `ValidateFormatDoc` to return warnings

**File:** `cli/internal/capmon/formatdoc_validate.go`
**Depends on:** D13.1
**Time estimate:** 5 minutes

The current `ValidateFormatDoc` signature returns only `error`. We need a new function that also returns warnings, without breaking the existing callers.

Add `ValidateFormatDocWithWarnings`:

```go
// ValidateFormatDocWithWarnings validates a format doc and returns both
// blocking errors and non-blocking allow-list warnings.
// The caller decides what to do with warnings (log, open GH issue, etc.).
func ValidateFormatDocWithWarnings(formatsDir, canonicalKeysPath, provider string) ([]ValidationWarning, error) {
    // Run the existing blocking validation first.
    if err := ValidateFormatDoc(formatsDir, canonicalKeysPath, provider); err != nil {
        return nil, err
    }

    docPath := FormatDocPath(formatsDir, provider)
    doc, err := LoadFormatDoc(docPath)
    if err != nil {
        return nil, err
    }

    var warnings []ValidationWarning

    for ct, ctDoc := range doc.ContentTypes {
        // Warn on sources[].section outside allow-list.
        for i, src := range ctDoc.Sources {
            if src.Section != "" && !allowedSourceSections[src.Section] {
                field := fmt.Sprintf("content_types.%s.sources[%d].section", ct, i)
                warnings = append(warnings, ValidationWarning{
                    File:    docPath,
                    Field:   field,
                    Value:   src.Section,
                    Message: fmt.Sprintf("section %q not in allow-list %v", src.Section, sortedKeys(allowedSourceSections)),
                })
            }
        }

        // Warn on provider_extensions[].value_type and examples[].lang outside allow-lists.
        for _, ext := range ctDoc.ProviderExtensions {
            if ext.ValueType != "" && !allowedValueTypes[ext.ValueType] {
                field := fmt.Sprintf("content_types.%s.provider_extensions[%s].value_type", ct, ext.ID)
                warnings = append(warnings, ValidationWarning{
                    File:    docPath,
                    Field:   field,
                    Value:   ext.ValueType,
                    Message: fmt.Sprintf("value_type %q not in allow-list %v", ext.ValueType, sortedKeys(allowedValueTypes)),
                })
            }
            for j, ex := range ext.Examples {
                if ex.Lang == "" {
                    field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].lang", ct, ext.ID, j)
                    warnings = append(warnings, ValidationWarning{
                        File:    docPath,
                        Field:   field,
                        Value:   "",
                        Message: "examples[].lang is required and must be non-empty",
                    })
                    continue
                }
                if !allowedExampleLangs[ex.Lang] {
                    field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].lang", ct, ext.ID, j)
                    warnings = append(warnings, ValidationWarning{
                        File:    docPath,
                        Field:   field,
                        Value:   ex.Lang,
                        Message: fmt.Sprintf("lang %q not in allow-list %v", ex.Lang, sortedKeys(allowedExampleLangs)),
                    })
                }
                if ex.Code == "" {
                    field := fmt.Sprintf("content_types.%s.provider_extensions[%s].examples[%d].code", ct, ext.ID, j)
                    warnings = append(warnings, ValidationWarning{
                        File:    docPath,
                        Field:   field,
                        Value:   "",
                        Message: "examples[].code is required and must be non-empty",
                    })
                }
            }
        }
    }

    return warnings, nil
}

// sortedKeys returns a sorted slice of the keys of a string→bool map.
// Used for deterministic error messages.
func sortedKeys(m map[string]bool) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}
```

This requires adding `"sort"` to imports in the same Edit call.

### D13.3 — Write tests for `ValidateFormatDocWithWarnings`

**File:** `cli/internal/capmon/formatdoc_validate_test.go`
**Depends on:** D13.2
**Time estimate:** 5 minutes

Add a table-driven test covering:

```go
func TestValidateFormatDocWithWarnings(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    canonicalKeysPath := writeTestCanonicalKeys(t, dir)
    formatsDir := filepath.Join(dir, "formats")
    if err := os.MkdirAll(formatsDir, 0755); err != nil {
        t.Fatal(err)
    }

    tests := []struct {
        name          string
        yamlContent   string
        wantWarnCount int
        wantWarnField string // substring to find in at least one warning's Field
    }{
        {
            name: "no_warnings_when_empty",
            yamlContent: validFormatDocContent, // existing fixture — no new fields
            wantWarnCount: 0,
        },
        {
            name: "value_type_not_in_allow_list",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: bad_type
        name: Bad Type
        description: "test"
        source_ref: "https://example.com"
        value_type: "array<string>"
`,
            wantWarnCount: 1,
            wantWarnField: "value_type",
        },
        {
            name: "example_lang_not_in_allow_list",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: bad_lang
        name: Bad Lang
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: "cobol"
            code: "MOVE 1 TO X."
`,
            wantWarnCount: 1,
            wantWarnField: "examples",
        },
        {
            name: "example_lang_empty_is_warning",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: no_lang
        name: No Lang
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: ""
            code: "x: 1"
`,
            wantWarnCount: 1,
            wantWarnField: "lang",
        },
        {
            name: "example_code_empty_is_warning",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: no_code
        name: No Code
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: "yaml"
            code: ""
`,
            wantWarnCount: 1,
            wantWarnField: "code",
        },
        {
            name: "source_section_not_in_allow_list",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-14T00:00:00Z"
        section: "Extension: model"
    canonical_mappings: {}
    provider_extensions: []
`,
            wantWarnCount: 1,
            wantWarnField: "section",
        },
        {
            name: "allow_listed_value_type_no_warning",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: ok_type
        name: OK Type
        description: "test"
        source_ref: "https://example.com"
        value_type: "string | string[]"
        examples:
          - lang: yaml
            code: "model: x"
`,
            wantWarnCount: 0,
        },
        {
            name: "multiple_violations_accumulate",
            yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: multi
        name: Multi
        description: "test"
        source_ref: "https://example.com"
        value_type: "not-valid"
        examples:
          - lang: "perl"
            code: "x"
`,
            wantWarnCount: 2,
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            subDir := filepath.Join(dir, "formats-"+tc.name)
            if err := os.MkdirAll(subDir, 0755); err != nil {
                t.Fatal(err)
            }
            writeTestFormatDoc(t, subDir, "test-provider", tc.yamlContent)
            warnings, err := ValidateFormatDocWithWarnings(subDir, canonicalKeysPath, "test-provider")
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if len(warnings) != tc.wantWarnCount {
                t.Errorf("warning count = %d, want %d; warnings: %v", len(warnings), tc.wantWarnCount, warnings)
            }
            if tc.wantWarnField != "" {
                found := false
                for _, w := range warnings {
                    if strings.Contains(w.Field, tc.wantWarnField) || strings.Contains(w.Message, tc.wantWarnField) {
                        found = true
                        break
                    }
                }
                if !found {
                    t.Errorf("no warning mentioning %q in fields %v", tc.wantWarnField, warnings)
                }
            }
        })
    }
}
```

### D13.4 — Test `ValidationWarning.DeduplicationKey` uniqueness

**File:** `cli/internal/capmon/formatdoc_validate_test.go`
**Depends on:** D13.1
**Time estimate:** 2 minutes

```go
func TestValidationWarning_DeduplicationKey(t *testing.T) {
    t.Parallel()
    w1 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[x].value_type", Value: "bad"}
    w2 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[x].value_type", Value: "bad"}
    w3 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[y].value_type", Value: "bad"}

    if w1.DeduplicationKey() != w2.DeduplicationKey() {
        t.Error("identical warnings must produce identical dedup keys")
    }
    if w1.DeduplicationKey() == w3.DeduplicationKey() {
        t.Error("warnings with different fields must produce different dedup keys")
    }
    if len(w1.DeduplicationKey()) != 16 {
        t.Errorf("dedup key must be 16 hex chars, got len %d: %q", len(w1.DeduplicationKey()), w1.DeduplicationKey())
    }
}
```

### D13.5 — Wire `ValidateFormatDocWithWarnings` into `capmon validate-format-doc` command

**File:** `cli/cmd/syllago/capmon_validate_format_doc_cmd.go`
**Depends on:** D13.2
**Time estimate:** 3 minutes

Update the `RunE` function to call `ValidateFormatDocWithWarnings` instead of `ValidateFormatDoc`, and print warnings to stderr:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    // ... (existing flag reads and telemetry.Enrich unchanged) ...

    warnings, err := capmon.ValidateFormatDocWithWarnings(formatsDir, canonicalKeys, provider)
    if err != nil {
        return err
    }
    fmt.Printf("✓ Schema valid\n✓ All checks passed for provider %q\n", provider)
    if len(warnings) > 0 {
        fmt.Fprintf(os.Stderr, "\n%d validation warning(s) for provider %q:\n", len(warnings), provider)
        for _, w := range warnings {
            fmt.Fprintf(os.Stderr, "  ⚠ [%s] %s: %s\n", w.DeduplicationKey(), w.Field, w.Message)
        }
    }
    return nil
},
```

This requires adding `"os"` to imports in the same Edit call (if not already present).

### D13.6 — Add command-level test for warning output

**File:** `cli/cmd/syllago/capmon_validate_format_doc_cmd_test.go`
**Depends on:** D13.5
**Time estimate:** 3 minutes

Check the existing test file structure first, then add:

```go
func TestCapmonValidateFormatDocCmd_WarningPrintedToStderr(t *testing.T) {
    dir := t.TempDir()
    // Write a format doc with a bad value_type.
    content := `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: bad
        name: Bad
        description: "test"
        source_ref: "https://example.com"
        value_type: "not-in-list"
`
    formatsDir := filepath.Join(dir, "formats")
    os.MkdirAll(formatsDir, 0755)
    os.WriteFile(filepath.Join(formatsDir, "test-provider.yaml"), []byte(content), 0644)

    // canonical-keys.yaml placeholder
    specDir := filepath.Join(dir, "spec")
    os.MkdirAll(specDir, 0755)
    os.WriteFile(filepath.Join(specDir, "canonical-keys.yaml"), []byte(`content_types:
  skills:
    display_name:
      description: test
      type: string
`), 0644)

    origFmts := capmonFormatDocsDirOverride
    origKeys := capmonCanonicalKeysDirOverride
    capmonFormatDocsDirOverride = formatsDir
    capmonCanonicalKeysDirOverride = filepath.Join(specDir, "canonical-keys.yaml")
    t.Cleanup(func() {
        capmonFormatDocsDirOverride = origFmts
        capmonCanonicalKeysDirOverride = origKeys
    })

    _, stderr := output.SetForTest(t)

    capmonValidateFormatDocCmd.Flags().Set("provider", "test-provider")
    defer capmonValidateFormatDocCmd.Flags().Set("provider", "")

    err := capmonValidateFormatDocCmd.RunE(capmonValidateFormatDocCmd, []string{})
    if err != nil {
        t.Fatalf("command returned error: %v", err)
    }

    stderrOut := stderr.String()
    if !strings.Contains(stderrOut, "warning") && !strings.Contains(stderrOut, "⚠") {
        t.Errorf("expected warning on stderr, got: %q", stderrOut)
    }
}
```

Note: `output.SetForTest` must capture stderr as well as stdout. Check whether the existing `SetForTest` function supports this; if it only captures stdout, capture stderr via `os.Stderr` redirect in the test.

### D13 Phase validation

```bash
cd cli && make fmt && make build && go test ./internal/capmon/... ./cmd/syllago/... -run "TestValidateFormatDoc|TestValidationWarning|TestCapmonValidateFormatDocCmd" -v
```

Coverage check:
```bash
cd cli && go test ./internal/capmon/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
```

Target: ≥80% for `capmon` package.

---

## 6. D14 Tasks — GitHub Issue Automation (Delta Inside `syllago-jtafb`)

D14 is not a standalone deliverable. The GitHub API client, authentication logic, and issue-lifecycle management are being built inside `syllago-jtafb`. This section describes only the **delta** that the jtafb implementers must incorporate: the interface that `ValidateFormatDocWithWarnings` exposes, and the calling contract inside `capmon check`.

### D14-delta.1 — Calling contract inside `capmon check`

`capmon check`'s Step 2 already calls `capmon validate-format-doc`. After D13 lands, it must switch to the programmatic API:

```go
warnings, err := capmon.ValidateFormatDocWithWarnings(formatsDir, canonicalKeysPath, providerSlug)
if err != nil {
    // Blocking error: fail the provider's check step.
    return err
}
if len(warnings) > 0 {
    // Non-blocking: pass to issue manager.
    issueManager.OpenOrUpdate(ctx, warnings)
}
```

`issueManager.OpenOrUpdate` is the jtafb-owned component. Its contract:

- **In CI** (env var `GITHUB_TOKEN` is set): calls GitHub API to open or update issues.
- **Locally** (no `GITHUB_TOKEN`): logs each warning to stderr with dedup key and field path.
- **Dedup:** Key is `w.DeduplicationKey()` (defined in D13.1). On open: title = `[capmon-warn-<key>] Allow-list violation: <field>`. On update: append timestamp + occurrence count to issue body.
- **Auto-close:** At end of a clean `capmon check` run (no warnings for a provider), call `issueManager.CloseResolved(ctx, providerSlug)` — it closes any open issues whose dedup key was not seen in the current run.

### D14-delta.2 — TrackingIssue URL back-feed into DataQuality

When `issueManager.OpenOrUpdate` creates a new issue, it must write the issue URL back into `DataQualityEntry.TrackingIssue`. The mechanism: the capmon pipeline writes a `data_quality_patch.json` sidecar file alongside the format doc update (or directly updates the format doc YAML with a new `tracking_issue` top-level field). `gencapabilities.go` reads this sidecar when present and merges it into the `DataQuality` block.

Sidecar path: `docs/provider-formats/<slug>.quality.json`, structure:
```json
{
  "tracking_issue": "https://github.com/OpenScribbler/syllago/issues/123"
}
```

Update `loadProviderFormatsDir` in `gencapabilities.go` to look for this sidecar and populate `TrackingIssue` when found:
```go
// After building contentTypes for slug:
qualityPath := filepath.Join(dir, slug+".quality.json")
var trackingIssue string
if raw, err := os.ReadFile(qualityPath); err == nil {
    var q struct{ TrackingIssue string `json:"tracking_issue"` }
    if json.Unmarshal(raw, &q) == nil {
        trackingIssue = q.TrackingIssue
    }
}
// Store for computeDataQuality to pick up.
```

This requires a small refactor: `computeDataQuality` needs access to the per-provider tracking issue URLs. The cleanest approach is to add `trackingIssues map[string]string` as a parameter:

```go
func computeDataQuality(providers map[string]map[string]CapContentType, trackingIssues map[string]string) DataQuality
```

Update all callers accordingly.

The sidecar files are gitignored (they are pipeline-runtime state, not committed). Add `*.quality.json` to `.gitignore` under `docs/provider-formats/`.

---

## 7. Final Validation

After all tasks in D1, D13, and D16 are complete (D14 rides with jtafb):

### Step 1 — Format check

```bash
cd cli && make fmt
```

Must produce no diff. If `gofmt` changed anything, stage and commit the formatting changes before the code changes.

### Step 2 — Build

```bash
cd cli && make build
```

Binary must rebuild without errors. Check with `syllago version` or `~/.local/bin/syllago --version`.

### Step 3 — Full test suite

```bash
cd cli && go test ./... -count=1
```

All tests must pass. If golden tests fail due to output changes in `syllago info`, regenerate:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

### Step 4 — Coverage check

```bash
cd cli && go test ./internal/capmon/... -coverprofile=capmon.out && go tool cover -func=capmon.out | grep total
cd cli && go test ./cmd/syllago/ -coverprofile=cmd.out && go tool cover -func=cmd.out | grep total
```

Both must be ≥80%.

### Step 5 — Integration test with real providers

```bash
cd cli && go test ./cmd/syllago/ -run TestGencapabilities_AllRealProviders -v
```

Must pass and list all 14 providers.

### Step 6 — Smoke test `syllago info providers <slug>`

```bash
cd cli && make build && syllago info providers claude-code
syllago info providers claude-code --json
syllago info providers nonexistent-slug  # must print error
```

### Step 7 — Regenerate `commands.json`

After adding `syllago info providers <slug>`:
```bash
cd cli && ./syllago _gendocs > ../commands.json
```

The pre-push hook checks `commands.json` freshness. Commit this file alongside the code changes.

### Step 8 — Telemetry catalog check

The new `infoProvidersSlugCmd` does not add any new properties (it reads data and prints it, no tracked events beyond what `command_executed` already captures). No catalog entry is needed. Verify:

```bash
cd cli && go test ./cmd/syllago/ -run TestGentelemetry_CatalogMatchesEnrichCalls -v
```

Must pass without changes.

---

## 8. Rollback Plan

All changes in this plan are **additive and optional** in their schema impact. Rollback is safe at any stage.

### If D1 needs to roll back

The three new fields on `ProviderExtension` are `omitempty` (or nullable). Reverting the Go structs in `formatdoc.go` and `gencapabilities.go` leaves the YAML files untouched — the YAML parser simply ignores fields it doesn't know. The capabilities.json schema consumers (syllago-docs Zod schema) declare the new fields as `.optional()`, so removing them from the JSON output does not break any downstream consumer.

**Rollback steps:**
1. Revert `formatdoc.go` and `gencapabilities.go` to their pre-D1 state via `git revert` or `git checkout`.
2. Run `cd cli && make build && go test ./... -count=1`.
3. The YAML files retain the comment blocks added in D1.7 — these are inert.

### If D13 needs to roll back

`ValidateFormatDocWithWarnings` is a new function alongside the unchanged `ValidateFormatDoc`. The existing command still works (D13.5 replaced `ValidateFormatDoc` with `ValidateFormatDocWithWarnings`, but the error return is identical). Rollback: revert `formatdoc_validate.go` and `capmon_validate_format_doc_cmd.go`. No schema or data changes.

### If D16 needs to roll back

The `data_quality` block in `capabilities.json` is a new top-level key. Zod schemas in syllago-docs use `.optional()` for new fields, so removing `data_quality` from the output does not break the docs build. The `syllago info providers <slug>` subcommand is a new command — removing it has no user-visible impact on existing workflows.

**Rollback steps:**
1. Revert `gencapabilities.go` (remove `DataQuality` types and `computeDataQuality` call) and `info.go` (remove `infoProvidersSlugCmd`).
2. Regenerate `commands.json`.
3. Run `cd cli && make build && go test ./... -count=1`.

---

## Appendix: Task Checklist

| Task | File(s) | Depends on | Status |
|------|---------|-----------|--------|
| D1.1 — Extend `ProviderExtension` struct | `cli/internal/capmon/formatdoc.go` | — | pending |
| D1.2 — Struct round-trip test | `cli/internal/capmon/formatdoc_test.go` | D1.1 | pending |
| D1.3 — Extend gencapabilities YAML+JSON structs | `cli/cmd/syllago/gencapabilities.go` | D1.1 | pending |
| D1.4 — Gencapabilities tests for new fields | `cli/cmd/syllago/gencapabilities_test.go` | D1.3 | pending |
| D1.5 — Add `name`/`section` to `CapSource` | `gencapabilities.go`, `formatdoc.go` | D1.3 | pending |
| D1.6 — Verify validator still passes | — (run only) | D1.1 | pending |
| D1.7 — Update 14 provider YAML files | `docs/provider-formats/*.yaml` | D1.1 | pending |
| D1 gate — Full test pass | — | D1.1–D1.7 | pending |
| D16.1 — DataQuality types | `gencapabilities.go` | D1.3 | pending |
| D16.2 — computeDataQuality function | `gencapabilities.go` | D16.1 | pending |
| D16.3 — DataQuality tests | `gencapabilities_test.go` | D16.2 | pending |
| D16.4 — `info providers <slug>` command | `info.go` | D16.2 | pending |
| D16.5 — Info command tests | `info_test.go` | D16.4 | pending |
| D16 gate — Full test pass | — | D16.1–D16.5 | pending |
| D13.1 — Allow-lists + ValidationWarning type | `formatdoc_validate.go` | D1.1 | pending |
| D13.2 — ValidateFormatDocWithWarnings | `formatdoc_validate.go` | D13.1 | pending |
| D13.3 — Warning tests (table-driven) | `formatdoc_validate_test.go` | D13.2 | pending |
| D13.4 — DeduplicationKey uniqueness test | `formatdoc_validate_test.go` | D13.1 | pending |
| D13.5 — Wire into validate-format-doc command | `capmon_validate_format_doc_cmd.go` | D13.2 | pending |
| D13.6 — Command-level warning test | `capmon_validate_format_doc_cmd_test.go` | D13.5 | pending |
| D13 gate — Full test pass | — | D13.1–D13.6 | pending |
| D14-delta.1 — Calling contract (jtafb) | Inside `syllago-jtafb` bead chain | D13.2 | deferred to jtafb |
| D14-delta.2 — TrackingIssue back-feed | `gencapabilities.go` + `.quality.json` | D14-delta.1 | deferred to jtafb |
| Final — commands.json regen | `commands.json` | D16.4 | pending |
| Final — make build + go test ./... | — | all D1/D13/D16 | pending |
