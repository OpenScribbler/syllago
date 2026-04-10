# Release Readiness Phase 2: Architecture + Error System

*Design date: 2026-03-19*
*Status: Design Complete*
*Phase: 2 of 5*
*Dependencies: Benefits from Phase 1 (CI gates catch issues during refactoring)*

## Overview

Fix the two real CLI/TUI architecture gaps and introduce structured error codes that will surface in the documentation site. This phase ensures the codebase is well-structured and errors are actionable before we write documentation.

## Context

A CLI/TUI parity audit found that the architecture is mostly sound — CLI commands exist for most operations and shared packages (`add`, `catalog`, `config`, `registry`) handle core logic. Two gaps need fixing:

1. **Loadout creation is duplicated** — CLI and TUI have completely independent implementations with no shared core. YAML marshaling, validation, and manifest writing exist in both paths.
2. **`inspect` command is thin** — CLI shows basic metadata only. File viewing, compatibility checks, and detailed risk analysis are TUI-only.

Additionally, the 4-agent audit recommended structured error codes that can be surfaced on the documentation site.

---

## Work Items

### 1. Extract Shared Loadout Creation Core

**Problem:** `cli/cmd/syllago/loadout_create.go` (CLI) and `cli/internal/tui/loadout_create.go` (TUI) both implement loadout creation independently. Duplicated logic includes: manifest building, content validation, name validation, YAML marshaling, destination handling.

**Fix:** Extract shared functions into `cli/internal/loadout/` package:
- `loadout.BuildManifest(opts BuildOpts) (*Manifest, error)` — core manifest construction
- `loadout.ValidateManifest(m *Manifest) error` — validation rules
- `loadout.WriteManifest(m *Manifest, destPath string) error` — YAML marshaling + write

Both CLI and TUI call these shared functions. TUI adds interactive UX (wizard steps, split-view preview, security callouts) on top. CLI adds stdin prompts on top.

**Convention reinforced:** CLI commands contain (or call) the business logic. TUI calls the same logic and adds interactive chrome.

### 2. Enrich `inspect` Command

**Problem:** `cli/cmd/syllago/inspect.go` shows basic metadata (name, type, source, provider, path, description, files, risk indicators). The TUI detail view shows much more: file contents, per-provider compatibility, MCP config inspection, hook compatibility matrix.

**Fix:** Add capabilities to `inspect`:
- `--files` flag — show file contents (formatted for terminal, raw for `--json`)
- `--compatibility` flag — show per-provider compatibility matrix
- `--risk` flag — show detailed risk analysis (hook commands, MCP server configs)
- Default `inspect` remains concise; flags opt into detail
- All output available as `--json` for scripting and accessibility

**Implementation:** Extract the analysis logic from TUI detail view files (`detail_fileviewer.go`, `detail_provcheck.go`) into shared packages that both CLI and TUI consume.

### 3. Structured Error System

**Design:** Namespaced error codes with numeric suffixes. Each code maps to a documentation page.

**Code format:** `CATEGORY_NNN`
- `CATALOG_001` — No catalog found
- `REGISTRY_001` — Registry clone failed
- `PROVIDER_001` — Provider not detected
- `INSTALL_001` — Installation path not writable
- `CONVERT_001` — Content type not supported by target provider

**Implementation:**

**3a. Error type** in `cli/internal/output/`:
```go
type StructuredError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    Suggestion string `json:"suggestion,omitempty"`
    DocsURL    string `json:"docs_url,omitempty"`
    Details    string `json:"details,omitempty"`
}
```

**3b. Error code registry** — Go constants with descriptions:
```go
const (
    ErrCatalogNotFound  = "CATALOG_001"
    ErrRegistryClone    = "REGISTRY_001"
    ErrProviderNotFound = "PROVIDER_001"
    // ...
)
```

**3c. Apply across CLI commands** — Replace bare `fmt.Errorf` calls with structured errors in all user-facing CLI commands. Internal package errors remain as standard Go errors.

**3d. JSON output** — When `--json` flag is set, errors output as structured JSON instead of plain text.

**3e. Docs URL** — Each error includes a URL like `https://openscribbler.github.io/syllago-docs/errors/catalog-001/`. This is populated automatically from the error code.

### 4. Provider Detection Respects Custom Paths

**Problem:** If a user has their provider config in a non-standard location and has configured custom paths via `PathResolver`, the provider detection still checks only default paths. This causes false "no providers detected" warnings.

**Fix:** Update `provider.DetectProviders()` to:
1. Load config and check for custom path overrides
2. Check custom paths first, then fall back to defaults
3. This leverages the existing `PathResolver` infrastructure from the `InstallWithResolver` work

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Loadout creation approach | Extract shared core | Fixes duplication, reinforces CLI-as-logic convention |
| inspect enrichment | Opt-in flags | Keep default output concise, power users add flags |
| Error code format | CATEGORY_NNN | Namespaced for grep-ability, numeric for ordering, maps to doc pages |
| Error scope | CLI commands only | Internal packages use standard Go errors |
| Docs URL pattern | Auto-generated from code | Consistent, no manual mapping needed |

---

## Out of Scope

- Replicating TUI interactivity in CLI (file browser, visual grid) — that's what the TUI is for
- Full CLI parity for import wizard (shared `add` package already handles core logic)
- Error code documentation content (Phase 5 — docs site work)
