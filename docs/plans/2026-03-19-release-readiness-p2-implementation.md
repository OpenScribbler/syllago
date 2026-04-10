# Release Readiness Phase 2: Architecture + Error System -- Implementation Plan

*Generated: 2026-03-20*
*Design doc: `docs/plans/2026-03-19-release-readiness-p2-design.md`*

---

## Overview

13 tasks organized into 4 groups. Execution order: Group A (loadout core) first, then Groups B, C, D in parallel since they have no cross-group dependencies. Within each group, tasks are strictly sequential.

**Dependency graph:**
```
A1 -> A2 -> A3
B1 -> B2 -> B3 -> B4
C1 -> C2 -> C3 -> C4
D1 -> D2
```

---

## Group A: Shared Loadout Creation Core

### A1: Create `loadout.BuildManifest()` and `loadout.WriteManifest()`

**Description:** Create `cli/internal/loadout/create.go` with two new exported functions. `BuildManifest` constructs a validated `*Manifest` from discrete inputs. `WriteManifest` handles YAML marshaling, directory creation, and file writing. Pure logic extraction -- no caller changes yet.

**Files to create:**
- `cli/internal/loadout/create.go`
- `cli/internal/loadout/create_test.go`

**Key context:**
- Existing `Manifest` struct in `manifest.go` lines 13-25
- CLI duplicates this as `loadoutCreateManifest` in `loadout_create.go` lines 31-43
- TUI already imports `loadout.Manifest` in `app.go` lines 363-385
- YAML marshal + write duplicated: CLI lines 204-212, TUI app.go lines 410-418

**Signatures:**
```go
func BuildManifest(provider, name, description string, items map[catalog.ContentType][]string) *Manifest

// WriteManifest marshals m to YAML and writes to destDir/<m.Name>/loadout.yaml.
// Returns the written file path.
func WriteManifest(m *Manifest, destDir string) (string, error)
```

**Test cases (write first):**
- `TestBuildManifest_Basic` -- Kind/Version set automatically
- `TestBuildManifest_AllItems` -- all 6 content type slices populated
- `TestBuildManifest_EmptyItems` -- empty slices produce nil fields (yaml omitempty)
- `TestBuildManifest_NilItems` -- nil input produces valid manifest
- `TestWriteManifest_CreatesFile` -- YAML file written at destDir/name/loadout.yaml
- `TestWriteManifest_CreatesDir` -- parent dir created via os.MkdirAll
- `TestWriteManifest_RoundTrip` -- Write then Parse produces equal manifest
- `TestWriteManifest_UnwritablePath` -- error on permission failure

**Deferred:** The design doc specifies a standalone `ValidateManifest(m *Manifest) error` function. This is omitted from the current plan since validation already exists in `Parse()` (manifest.go lines 41-52) and the CLI wizard. If future code needs to validate a manifest without round-tripping through YAML, add `ValidateManifest` as a follow-up task.

**Verification:** `make test` passes. RoundTrip test is key correctness check.

**Dependencies:** None.

---

### A2: Refactor CLI `loadout_create.go` to use shared functions

**Description:** Replace local `loadoutCreateManifest` struct (lines 31-43) and inline YAML marshal/write with calls to `loadout.BuildManifest()` and `loadout.WriteManifest()`. Item selection logic stays -- it's CLI-specific UX.

**Files to modify:**
- `cli/cmd/syllago/loadout_create.go`

**Implementation steps:**
1. Add import `loadout` package
2. Delete local `loadoutCreateManifest` struct (lines 31-43)
3. Replace manifest construction (lines 94-100) with `itemsByType` map
4. Replace switch block (lines 158-171) to populate `itemsByType[ct]`
5. Build manifest: `manifest := loadout.BuildManifest(providerSlug, name, description, itemsByType)`
6. Replace YAML marshal + write (lines 199-212). **Important:** The current `outDir` at line 199 is `filepath.Join(root, "content", "loadouts", providerSlug, name)` which already includes the item name. Since `WriteManifest` appends `<m.Name>` internally, pass the **parent** directory instead:
   ```go
   parentDir := filepath.Join(root, "content", "loadouts", providerSlug)
   outPath, err := loadout.WriteManifest(manifest, parentDir)
   ```

**Verification:** `make test` passes. Existing tests confirm behavior preserved.

**Dependencies:** A1

---

### A3: Refactor TUI `app.go` `doCreateLoadoutFromScreen` to use shared functions

**Description:** Replace inline manifest construction (app.go lines 363-385) and YAML marshal/write (app.go lines 410-418) with calls to shared functions. Destination resolution logic (lines 387-403) stays unchanged.

**Files to modify:**
- `cli/internal/tui/app.go` (lines 351-422)

**Implementation steps:**
1. Replace manifest construction with `itemsByType` map + `loadout.BuildManifest()`
2. Keep destination resolution as-is
3. Replace marshal/write block with `loadout.WriteManifest(manifest, destDir)`
4. Remove `yaml` import if now unused in app.go

**Verification:** `make test` passes. Golden file tests unchanged.

**Dependencies:** A1, A2 (validates shared functions work before wiring TUI)

---

## Group B: Inspect Enrichment

### B1: Extract file analysis helpers into `cli/internal/catalog/`

**Description:** Extract `ReadFileContent` and `PrimaryFileName` logic from `cli/internal/tui/detail_fileviewer.go` (lines 67-110 for primary file selection, lines 135-161 for content reading) into `cli/internal/catalog/fileinfo.go`. These are pure logic helpers that the CLI `inspect` command needs.

**Note:** `buildFileTree()` (lines 24-63) stays in the TUI -- it returns `[]splitViewItem`, a TUI-specific type with `Disabled`, `Selected`, and rendering fields. The CLI's `--files` flag doesn't need a file tree; it just needs `ReadFileContent` to read individual files.

**Files to create:**
- `cli/internal/catalog/fileinfo.go`
- `cli/internal/catalog/fileinfo_test.go`

**New functions:**
```go
// PrimaryFileName returns the best default file to display for a content item.
// Skills: SKILL.md; Hooks: first .json/.yaml; MCP: first .json; etc.
// Returns empty string if no match found.
func PrimaryFileName(files []string, ct ContentType) string

// ReadFileContent reads a file at itemPath/relPath, capping at maxLines.
// Returns content string. If truncated, appends "(N more lines)" suffix.
func ReadFileContent(itemPath, relPath string, maxLines int) (string, error)
```

**Test cases:**
- `TestPrimaryFileName_Skill` -- finds SKILL.md
- `TestPrimaryFileName_Hook` -- finds first .json file
- `TestPrimaryFileName_MCP` -- finds first .json file
- `TestPrimaryFileName_NoMatch` -- returns empty string
- `TestPrimaryFileName_Empty` -- nil/empty files returns empty string
- `TestReadFileContent_Basic` -- reads file content
- `TestReadFileContent_Truncated` -- caps at maxLines, appends N more lines
- `TestReadFileContent_NotFound` -- returns error

**Verification:** `make test` passes. TUI is not modified -- no golden file impact.

**Dependencies:** None within Group B.

---

### B2: Add `--files` flag to `inspect` command

**Description:** Add `--files` flag to `cli/cmd/syllago/inspect.go` showing file contents. Uses `catalog.ReadFileContent()` from B1.

**Files to modify:**
- `cli/cmd/syllago/inspect.go`
- `cli/cmd/syllago/inspect_test.go`

**New JSON field:** `FileContents map[string]string json:"file_contents,omitempty"`

**Plain text format:**
```
--- SKILL.md ---
<file content>

--- README.md ---
<file content>
```

**Test cases:**
- `TestInspectFiles_ShowsContent` -- prints file contents
- `TestInspectFiles_JSON` -- includes file_contents map
- `TestInspectFiles_MultipleFiles` -- shows headers
- `TestInspectFiles_MissingFile` -- graceful error inline

**Dependencies:** B1

---

### B3: Add `--compatibility` flag to `inspect` command

**Description:** Add `--compatibility` flag showing per-provider hook compatibility matrix. Uses `converter.LoadHookData()` and `converter.AnalyzeHookCompat()` (already exported). Only meaningful for hooks; prints "not applicable" for other types.

**Files to modify:**
- `cli/cmd/syllago/inspect.go`
- `cli/cmd/syllago/inspect_test.go`

**New JSON field:**
```go
type compatResult struct {
    Provider string `json:"provider"`
    Level    string `json:"level"`
    Notes    string `json:"notes,omitempty"`
}
Compatibility []compatResult `json:"compatibility,omitempty"`
```

**Plain text format:**
```
Compatibility:
  checkmark claude-code    full
  ~ cursor         degraded   (matcher syntax differs)
  x copilot-cli    broken     (no PostToolUse support)
```

**Test cases:**
- `TestInspectCompat_Hook` -- shows provider matrix
- `TestInspectCompat_NonHook` -- prints "not applicable"
- `TestInspectCompat_JSON` -- includes compatibility array

**Dependencies:** B2 (establishes flag registration pattern)

---

### B4: Add `--risk` flag to `inspect` command

**Description:** Add `--risk` flag with detailed MCP configuration inspection (command, args, env vars) and hook command details. Uses `installer.ParseMCPConfig()` (already exported).

**Files to modify:**
- `cli/cmd/syllago/inspect.go`
- `cli/cmd/syllago/inspect_test.go`

**New JSON field:** `DetailedRisks []riskDetail json:"detailed_risks,omitempty"`

**Plain text format:**
```
Detailed risks:
  warning  Runs commands
      command: npx
      args: -y, @modelcontextprotocol/server-github
      env: GITHUB_TOKEN (required)
```

**Test cases:**
- `TestInspectRisk_MCP` -- shows command/args/env
- `TestInspectRisk_Skill` -- shows Bash tool details
- `TestInspectRisk_NoRisk` -- clean output
- `TestInspectRisk_JSON` -- includes detailed_risks array

**Dependencies:** B2

---

## Group C: Structured Error System

### C1: Create `StructuredError` type and error code registry

**Description:** Add `StructuredError` type to `output.go` and create `errors.go` with error code constants and constructors. Additive -- no existing functions change.

**Key context:**
- Existing `ErrorResponse` struct in output.go has Code (int), Message, Suggestion
- 189 `fmt.Errorf` calls across 27 command files
- 11 `output.PrintError()` calls using numeric codes

**Files to create:**
- `cli/internal/output/errors.go`
- `cli/internal/output/errors_test.go`

**Files to modify:**
- `cli/internal/output/output.go` -- add StructuredError + PrintStructuredError

**StructuredError type:**
```go
type StructuredError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    Suggestion string `json:"suggestion,omitempty"`
    DocsURL    string `json:"docs_url,omitempty"`
    Details    string `json:"details,omitempty"`
}

func (e StructuredError) Error() string  // implements error interface for errors.As()
```

**Error code registry (errors.go):**
```go
const (
    ErrCatalogNotFound     = "CATALOG_001"
    ErrCatalogScanFailed   = "CATALOG_002"
    ErrRegistryClone       = "REGISTRY_001"
    ErrRegistryNotFound    = "REGISTRY_002"
    ErrRegistryNotAllowed  = "REGISTRY_003"
    ErrProviderNotFound    = "PROVIDER_001"
    ErrProviderNotDetected = "PROVIDER_002"
    ErrInstallNotWritable  = "INSTALL_001"
    ErrInstallItemNotFound = "INSTALL_002"
    ErrConvertNotSupported = "CONVERT_001"
    ErrConvertParseFailed  = "CONVERT_002"
    ErrImportCloneFailed   = "IMPORT_001"
    ErrImportConflict      = "IMPORT_002"
    ErrExportNotSupported  = "EXPORT_001"
    ErrConfigInvalid       = "CONFIG_001"
    ErrConfigNotFound      = "CONFIG_002"
)

func NewStructuredError(code, message, suggestion string) StructuredError
func NewStructuredErrorDetail(code, message, suggestion, details string) StructuredError
```

**DocsURL auto-generation:** `CATALOG_001` -> `https://openscribbler.github.io/syllago-docs/errors/catalog-001/`

**Test cases:**
- `TestDocsURL_Format` -- code to URL mapping
- `TestNewStructuredError_PopulatesDocsURL` -- auto-generates URL
- `TestPrintStructuredError_PlainText` -- prints code, message, suggestion, URL
- `TestPrintStructuredError_JSON` -- valid JSON with all fields
- `TestPrintStructuredError_NoSuggestion` -- omits suggestion line
- `TestAllErrorCodes_UniqueValues` -- no duplicate codes
- `TestAllErrorCodes_Format` -- all match CATEGORY_NNN pattern

**Dependencies:** None.

---

### C2: Apply structured errors to high-traffic CLI commands

**Description:** Replace `fmt.Errorf` + `output.PrintError(int, ...)` calls with `output.NewStructuredError()` + `output.PrintStructuredError()` in the most-used commands.

**Files to modify:**
- `cli/cmd/syllago/install_cmd.go`
- `cli/cmd/syllago/import.go`
- `cli/cmd/syllago/add_cmd.go`
- `cli/cmd/syllago/sync_and_export.go`

**Pattern:**
```go
// Before:
output.PrintError(1, "unknown provider: "+toSlug, "Available: "+strings.Join(slugs, ", "))
return output.SilentError(fmt.Errorf("unknown provider: %s", toSlug))

// After:
se := output.NewStructuredError(
    output.ErrProviderNotFound,
    "unknown provider: "+toSlug,
    "Available: "+strings.Join(slugs, ", "),
)
output.PrintStructuredError(se)
return output.SilentError(se)  // wrap StructuredError itself, NOT a new fmt.Errorf
```

**Critical:** The `SilentError` must wrap the `StructuredError` value (not a plain `fmt.Errorf`). This allows `errors.As()` in C4's `printExecuteError` to unwrap through `silentError.Unwrap()` and find the `StructuredError`. If you wrap a `fmt.Errorf` instead, the `errors.As` branch becomes inert.

**Note:** Check existing test assertions for error output format changes (`Error [CODE]:` prefix).

**Dependencies:** C1

---

### C3: Apply structured errors to remaining CLI commands

**Description:** Apply structured error pattern to remaining commands: `loadout_apply.go`, `loadout_create.go`, `loadout_remove.go`, `list.go`, `convert_cmd.go`, `registry_cmd.go`, `config_cmd.go`, `inspect.go`, `uninstall_cmd.go`, `info.go`.

**Heuristic:** Only convert errors where a meaningful code can be assigned. Leave ambiguous/internal errors as `fmt.Errorf`.

**Dependencies:** C2 (establishes pattern)

---

### C4: Wire structured errors into `main.go` JSON error output

**Description:** Update `printExecuteError` in `main.go` (lines 194-198) so `--json` mode outputs structured JSON for all errors, including those not yet converted to `StructuredError`.

**Files to modify:**
- `cli/cmd/syllago/main.go` (lines 194-198)

**New logic:**
```go
func printExecuteError(err error) {
    if output.IsSilentError(err) { return }
    var se output.StructuredError
    if errors.As(err, &se) {
        output.PrintStructuredError(se)
        return
    }
    if output.JSON {
        output.PrintStructuredError(output.StructuredError{
            Code: "UNKNOWN_001", Message: err.Error(),
        })
        return
    }
    fmt.Fprintln(output.ErrWriter, err)
}
```

**Requires:** `StructuredError` implements `error` interface (added in C1).

**Dependencies:** C3

---

## Group D: Provider Detection with Custom Paths

### D1: Add `DetectProvidersWithResolver()` to `detect.go`

**Description:** Add resolver-aware detection that checks custom paths in addition to defaults. Keep `DetectProviders()` as zero-config wrapper.

**Key context:**
- Current `DetectProviders()` in `detect.go` lines 7-24
- Each provider's `Detect` field: `func(homeDir string) bool`
- Checks only hardcoded paths (e.g., `~/.claude`, `~/.cursor`)
- PathResolver in `config/resolver.go` has `BaseDir(slug string) string`

**Import cycle resolution:** `provider` cannot import `config` (cycle). Define interface:
```go
type ProviderPathLookup interface {
    BaseDir(slug string) string
}

func DetectProvidersWithResolver(lookup ProviderPathLookup) []Provider
```

`config.PathResolver` already satisfies this interface structurally.

**Files to modify:**
- `cli/internal/provider/detect.go`

**Files to create:**
- `cli/internal/provider/detect_test.go`

**Test cases:**
- `TestDetectProviders_NoConfig` -- standard detection
- `TestDetectProvidersWithResolver_NilResolver` -- same as DetectProviders
- `TestDetectProvidersWithResolver_CustomBaseDir_Exists` -- detected via custom path
- `TestDetectProvidersWithResolver_CustomBaseDir_Missing` -- not detected
- `TestDetectProvidersWithResolver_StandardAndCustom` -- standard detection unaffected

**Implementation detail for custom path detection:** The `lookup.BaseDir(slug)` returns the configured base directory for a provider. Check if it exists with `os.Stat()`. Do NOT call `p.Detect(customBaseDir)` -- the per-provider `Detect` functions check hardcoded subdirectories relative to home (e.g., `filepath.Join(homeDir, ".claude")`), and passing a custom base dir would check the wrong path. Instead, a non-empty `BaseDir` that exists on disk is sufficient evidence of provider presence.

**Verification:** `make test` passes. No import cycles (`go build ./...`).

**Dependencies:** None.

---

### D2: Update call sites to use resolver-aware detection

**Description:** Update `main.go:284` and `init.go:77` to use `DetectProvidersWithResolver()`.

**Files to modify:**
- `cli/cmd/syllago/main.go` (line 284)
- `cli/cmd/syllago/init.go` (line 77)

**Implementation:**
- `main.go`: Build resolver from loaded config, pass to detection
- `init.go`: Pass nil (no config exists during init wizard)

**Verification:** `make build` + `make test` passes.

**Dependencies:** D1

---

## Potential Challenges

| Challenge | Resolution |
|-----------|-----------|
| Import cycle in D1 (provider -> config -> provider) | Define `ProviderPathLookup` interface in detect.go |
| Existing test assertions on error format (C2/C3) | First-pass audit of affected assertions before implementation |
| TUI golden file regeneration (B1) | Changes are behavioral only, no visual output changes |
| `StructuredError.Error()` timing | Must be added in C1, not C4 -- needed for `errors.As()` |
