# TUI Confirm-Bucket Rendering — Design Document

**Goal:** Surface content-signal confidence tiers in the TUI — both as persistent metadata on library items (install wizard) and as an interactive triage step in the add wizard for confirm-bucket items.

**Decision Date:** 2026-04-02

**Related:** `docs/plans/2026-04-01-content-based-classification-design.md` (parent feature), bead `syllago-6fubw`

---

## Problem Statement

The content-signal detector classifies items into Auto (high confidence) and Confirm (needs user approval) buckets. The CLI `manifest generate` command outputs tier labels as text, but the TUI has no way to:

1. Show confidence tier information on library items that originated from content-signal detection
2. Let users triage confirm-bucket items during the "add" flow (provider, registry, local, or git sources)

The analyzer runs on all discovery sources — providers, registries, local paths, and git URLs — because any source could contain repos with non-standard content layouts that pattern-based detection misses.

## Proposed Solution

Two surfaces, implemented sequentially:

**Surface 1 — Metadata persistence + tier badge:** Store `confidence` and `detection_source` on `.syllago.yaml` metadata. The install wizard review step shows a tier badge for content-signal items.

**Surface 2 — Triage step in add wizard:** New conditional step between Discovery and Review. Two-column triage view (item list + file preview). High-confidence and user-asserted items pre-checked; medium/low unchecked. Included items merge into Review alongside auto/pattern items.

## Architecture

### Surface 1: Metadata Persistence + Tier Badge

#### Schema Change

Three new optional fields on `metadata.Meta`:

```go
type Meta struct {
    // ... existing fields ...
    Confidence      float64 `yaml:"confidence,omitempty"`
    DetectionSource string  `yaml:"detection_source,omitempty"`
    DetectionMethod string  `yaml:"detection_method,omitempty"` // "automatic" or "user-directed"
}
```

`.syllago.yaml` example:
```yaml
name: redteam-agent
description: Red team exercise agent
confidence: 0.65
detection_source: content-signal
detection_method: automatic
```

Values:
- `confidence`: 0.0 = not from analyzer (pattern-detected or manually added). 0.55–0.85 = analyzer score.
- `detection_source`: `"content-signal"` (automatic fallback), `"user-directed"` (--scan-as), `""` (pattern/library).
- `detection_method`: `"automatic"` (content-signal fallback), `"user-directed"` (explicit --scan-as), `""` (pattern/library). Avoids floating-point equality checks for tier determination.

#### Data Flow

1. `add.AddItems()` writes an item to the library
2. If the source `DiscoveryItem` carries confidence/source info, those fields are written to `.syllago.yaml`
3. `catalog.Scan()` reads them back into `ContentItem.Meta.Confidence` / `.DetectionSource`
4. TUI components access via `item.Meta.Confidence` — no new fields on `ContentItem` itself

**Security constraint — allowlist approach:** All three analyzer-produced fields (`confidence`, `detection_source`, `detection_method`) are scanner-computed, never read from incoming package YAML. During `add.AddItems()`, the importer must strip these fields from any externally-sourced `.syllago.yaml` metadata and set them based on the actual discovery source. Use an allowlist of safe metadata fields rather than blacklisting individual analyzer fields — this prevents future analyzer metadata from being trusted by default. A malicious package shipping `confidence: 0.85` or `detection_method: user-directed` must not influence tier badges, pre-check behavior, or display.

#### TUI Tier Function

New function in `analyzer` package to avoid passing `*DetectedItem` to the TUI:

```go
// TierForMeta applies the same tier logic as TierForItem but from metadata fields.
// Uses detection_method instead of floating-point equality to identify user-directed items.
func TierForMeta(confidence float64, method string) ConfidenceTier {
    if method == "user-directed" {
        return TierUser
    }
    switch {
    case confidence < 0.60:
        return TierLow
    case confidence < 0.70:
        return TierMedium
    default:
        return TierHigh
    }
}
```

#### Install Wizard Review Step

In `viewReview()`, after Target/Scope/Method lines, render a Detection line when applicable:

```
Installing "redteam" to Claude Code
Target:   ~/.claude/rules/
Scope:    Global
Method:   Symlink
Detection: ● Medium confidence (content-signal)
```

Only shown when `item.Meta != nil && item.Meta.DetectionSource != ""`.

#### Color Mapping

| Tier | Color Variable | Dot |
|------|---------------|-----|
| Low | `warningColor` (Flexoki orange) | `●` |
| Medium | `primaryColor` (Flexoki cyan) | `●` |
| High | `successColor` (Flexoki green) | `●` |
| User-asserted | `accentColor` (Flexoki purple) | `●` |

User-asserted uses purple (not yellow like Low) because it has opposite semantics: the user explicitly identified this directory, vs. Low which means weak/uncertain signals. Sharing a color would create false visual equivalence.

### Surface 2: Triage Step in Add Wizard

#### Step Enum

```go
const (
    addStepSource   addStep = iota
    addStepType
    addStepDiscovery
    addStepTriage     // NEW — conditional
    addStepReview
    addStepExecute
)
```

This shifts `addStepReview` (3→4) and `addStepExecute` (4→5). All existing code references these by name, not value, so the shift is safe.

#### Conditional Step Logic

The Triage step is included when `hasTriageStep == true` (set when the analyzer returns confirm-bucket items). The pattern matches the existing `preFilterType` skip for the Type step.

Shell labels adjust dynamically:
- With Triage + Type: `["Source", "Type", "Discovery", "Triage", "Review", "Execute"]`
- With Triage, no Type: `["Source", "Discovery", "Triage", "Review", "Execute"]`
- No Triage + Type: `["Source", "Type", "Discovery", "Review", "Execute"]`
- No Triage, no Type: `["Source", "Discovery", "Review", "Execute"]`

`shellIndexForStep` handles all four permutations by skipping inactive steps.

**Shell label rebuild timing:** `wizardShell.SetSteps()` must be called in two places:
1. In `handleDiscoveryDone` — when `hasTriageStep` is determined (adds or omits "Triage" label)
2. In `clearTriageState()` — when triage state is cleared on back-navigation (removes "Triage" label)

Without explicit `SetSteps()` calls, the breadcrumb shows wrong step names after `hasTriageStep` changes.

#### State Cleanup: `clearTriageState()`

A single method called from both `goBackFromDiscovery()` and `advanceFromSource()` to prevent stale triage data from surviving source changes or back-navigation:

```go
func (m *addWizardModel) clearTriageState() {
    m.hasTriageStep = false
    m.confirmItems = nil
    m.confirmSelected = nil
    m.confirmCursor = 0
    m.confirmOffset = 0
    m.confirmPreview = previewModel{} // nil-equivalent; reconstructed on re-entry
    m.confirmFocus = triageZoneItems
    // Reset maxStep to Discovery — steps beyond Discovery are invalidated when
    // triage state changes. Without this, stale maxStep allows clicking into
    // out-of-bounds breadcrumb steps (panic).
    m.maxStep = addStepDiscovery
    // Rebuild shell labels without Triage step and sync breadcrumb state
    m.shell.SetSteps(m.buildShellLabels())
    m.updateMaxStep()
}
```

Nil-ing `confirmPreview` also solves WindowSizeMsg propagation: the preview model is reconstructed with current dimensions when the user re-enters the triage step, so there's no stale-dimension issue on resize.

**`buildShellLabels()` contract:** This method inspects `preFilterType` and `hasTriageStep` to produce the correct label permutation. Must be called from three sites: `openAddWizard()` (initial), `handleDiscoveryDone` (when `hasTriageStep` determined), and `clearTriageState()` (when triage removed). Returns `[]string` matching one of the 4 permutations documented in the Conditional Step Logic section.

#### Data Model Additions

```go
type addWizardModel struct {
    // ... existing fields ...

    // Triage step
    hasTriageStep  bool
    confirmItems    []addConfirmItem
    confirmSelected map[int]bool
    confirmCursor   int
    confirmOffset   int
    confirmPreview  previewModel
    confirmFocus    triageFocusZone
}

type addConfirmItem struct {
    detected    *analyzer.DetectedItem
    tier        analyzer.ConfidenceTier
    displayName string   // from DetectedItem.DisplayName or .Name
    itemType    catalog.ContentType
    path        string   // primary file path (relative to source root)
    sourceDir   string   // directory containing the item
}

type triageFocusZone int
const (
    triageZoneItems   triageFocusZone = iota
    triageZonePreview
    triageZoneButtons
)
```

On `addDiscoveryItem` (existing struct), two new fields for merged confirm items:
```go
type addDiscoveryItem struct {
    // ... existing fields ...
    confidence      float64
    detectionSource string
    tier            analyzer.ConfidenceTier
}
```

#### Discovery Backend Changes

All four discovery backends gain a parallel analyzer pass. Return signatures change to include confirm items:

```go
func discoverFromLocalPath(dir string, types []catalog.ContentType, contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, error)

func discoverFromGitURL(url string, types []catalog.ContentType, contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, string, error)

func discoverFromProvider(prov provider.Provider, projectRoot string, cfg *config.Config,
    contentRoot string, types []catalog.ContentType,
) ([]addDiscoveryItem, []addConfirmItem, error)

func discoverFromRegistry(reg catalog.RegistrySource, types []catalog.ContentType,
    contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, error)
```

Each backend:
1. Runs existing pattern-based discovery (unchanged)
2. Runs `analyzer.Analyze()` on the source directory
3. Merges `AnalysisResult.Auto` items into the discovery list (dedup by path). Merged Auto items carry `detectionSource: "content-signal"` on their `addDiscoveryItem` so the discovery step view can show a provenance indicator (e.g., `(detected)` suffix or colored dot) distinguishing them from pattern-matched items.
4. Returns `AnalysisResult.Confirm` items as `[]addConfirmItem`

`addDiscoveryDoneMsg` gains a new field:
```go
type addDiscoveryDoneMsg struct {
    // ... existing fields ...
    confirmItems []addConfirmItem
}
```

#### Path Containment in Preview

**Security constraint:** `catalog.ReadFileContent(basePath, relPath, maxLines)` performs a bare `filepath.Join` with no validation that the result stays under `basePath`. The triage preview calls this function with `relPath` from `DetectedItem.Path`. A crafted repo could include items with `relPath` values like `../../.ssh/id_rsa`, causing the preview to read arbitrary files.

**Required fix:** `ReadFileContent` must validate path containment before reading:
```go
resolved := filepath.Clean(filepath.Join(basePath, relPath))
if !strings.HasPrefix(resolved, filepath.Clean(basePath)+string(filepath.Separator)) {
    return "", fmt.Errorf("path traversal: %s escapes %s", relPath, basePath)
}
```

This is a pre-existing gap that the triage feature makes newly exploitable (content-signal items come from untrusted repos with attacker-controlled paths). Fix applies to ALL callers of `ReadFileContent`, not just the triage step.

#### Type Filtering for Analyzer Results

When `selectedTypes` is set (either via `preFilterType` or the Type step checkboxes), each backend must filter both `AnalysisResult.Auto` and `AnalysisResult.Confirm` items by type before returning. The analyzer's `Analyze()` method scans for all types — it has no type filter parameter. Without backend-side filtering, a "Add Skills" flow would surface agents, hooks, and rules in both the discovery list and triage step.

#### Provider Directory Trust Boundary

When the source is `addSourceProvider`, the analyzer runs on provider home directories (`~/.claude/`, `~/.gemini/`, etc.). The content-signal fallback could surface user config files as triage candidates — a privacy risk if those items are later exported to a shared registry.

**Mitigation:** Add a skip-list of sensitive directory patterns that the analyzer should not enter during provider-source discovery:
- `*.json` settings files in provider config roots (already handled by pattern detectors)
- Files outside known content subdirectories (`rules/`, `skills/`, `agents/`, etc.)
- Any file matching `*secret*`, `*credential*`, `*token*`, `*.env` patterns

Alternatively, disable the content-signal fallback entirely for provider sources (pattern detection is sufficient for standard provider directory layouts). The design recommends the latter as the simpler approach — the analyzer fallback was designed for unknown repo layouts, not well-structured provider directories.

#### Symlink Escape Guard

All path resolution from untrusted content must use a `SafeResolve(baseDir, untrustedPath)` utility that:
1. Joins and cleans the path: `filepath.Clean(filepath.Join(baseDir, untrustedPath))`
2. Resolves symlinks: `filepath.EvalSymlinks(resolved)`
3. Validates containment: result must be a prefix of `filepath.Clean(baseDir)`
4. Returns error if path escapes

This applies to:
- `buildCatalogItemFromDetected` — must receive `sourceDir` (absolute base path from the discovery backend), not derive paths solely from `DetectedItem.Path` (which is relative to repo root). If `sourceDir` is wrong or empty, the containment check fails safe.
- `copySupportingFiles` — must resolve each source file through `SafeResolve` before copying. Must explicitly exclude `.syllago.yaml` from the copy set (metadata.Save overwrites it, but a race between copy and save could preserve attacker-controlled metadata if save fails).
- `ReadFileContent` — path containment check (B6 fix) should use the same utility.

#### Deduplication

When merging analyzer Auto items into pattern-detected items, dedup by file path. Pattern-detected items win (higher confidence, more specific classification). The content-signal detector already avoids files matched by pattern detectors during `Analyze()`, but the add wizard's Phase 1 uses different detection code (`catalog.Scan`, `add.DiscoverFromProvider`) than the analyzer's pattern detectors. Overlap is possible.

Dedup strategy: build a set of canonicalized paths (`filepath.Clean` + `filepath.ToSlash`) from Phase 1 results. Skip any analyzer Auto item whose canonicalized path is already in the set. Canonicalization prevents bypass via `./rules/foo` vs `rules/foo` vs `rules//foo` variations.

For provider sources, dedup must also resolve symlinks (`filepath.EvalSymlinks`) before comparison — installed content is often symlinked from `~/.syllago/content/` into provider dirs like `~/.claude/rules/`. Without symlink resolution, the same item appears as both a pattern-detected provider item and a content-signal detection, creating duplicates in the triage step.

#### Triage Step View

Two-column layout with bordered frame, reusing `previewModel`:

```
  Triage: 3 items detected by content analysis
  Include items to add them, or skip (skipped items reappear next time you add from this source).

  ╭─ Items ──────────────┬─ Preview ─────────────────────╮
  │ ✓ agents/redteam     │ # Red Team Agent              │
  │ > skills/deploy      │                               │
  │   rules/lint         │ A specialized agent for red   │
  │                      │ team exercises...             │
  │                      │                               │
  │                      │ ## Allowed Tools              │
  │                      │ - shell                       │
  │                      │ - file_read                   │
  ├──────────────────────┴───────────────────────────────┤
  │  [Cancel]  [Back]  [Next]                            │
  ╰──────────────────────────────────────────────────────╯
```

**Left pane — single-line items with type + right-aligned tier:**

```
  ✓ agents/redteam       Agent    ● High
  > skills/deploy        Skill    ● Medium
    rules/lint           Rule     ● Low
    prompts/very-long... Prompt   ● User
```

Each row:
- Left prefix: `✓` (included), `>` (cursor), ` ` (excluded)
- Item name, truncated with `…` to fit available width
- Content type label (fixed-width column, matches discovery step format)
- Right-aligned: colored dot + one-word tier label

Cursor row is bold with accent color. Included items use primary text. Excluded items use muted text. Content type column maintains visual consistency with the adjacent Discovery step.

**Responsive column collapse:** At 80-wide (minimum TUI size), the left pane is ~23 chars — too narrow for all three columns (prefix + name + type + tier consumes ~16 chars of overhead, leaving ~7 for the name). When the left pane is below 30 chars, collapse the tier word label and show only the colored dot:

```
  ✓ agents/redteam   Agent  ●    (wide: 30+ char left pane)
  ✓ agents/redteam   ●          (narrow: <30 char, drop type too)
```

Breakpoints:
- Left pane ≥ 30 chars: Name + Type + `● Label` (full: `● High`)
- Left pane 24–29 chars: Name + `● Label` (drop Type column)
- Left pane < 24 chars: Name + `● X` (abbreviated: `● H`, `● M`, `● L`, `● U`)

Never use dot-only mode — even at the narrowest width, a single letter preserves tier context. Type is always visible in the preview pane header.

**Right pane:** `previewModel` showing the focused item's primary file content. Updates on cursor change.

**Pane sizing:** 30% left / 70% right, same ratio as install wizard review. Left pane minimum 18 chars.

#### Keyboard

| Key | Action |
|-----|--------|
| `↑/↓` | Move cursor |
| `space` | Toggle include/skip (sole toggle key — consistent with Discovery checkboxes) |
| `a` | Include all |
| `n` | Exclude all |
| `tab` | Cycle focus: items → preview → buttons |
| `enter` | Advance to next step (from any zone). No drill-in — preview pane is sufficient. |
| `esc` | Back to Discovery |

**Enter/space consistency:** `space` is the sole toggle key for include/skip — this matches the existing checkbox pattern in both Discovery and Type steps. `enter` from the items zone advances to the next step (not toggle, not drill-in). The triage preview pane already shows file content for the focused item — a separate drill-in view is unnecessary and would require complex state management (parallel drill-in fields that risk collision with the review step's drill-in state). Deep file inspection belongs in the library after install, not during triage.

#### Back-Navigation

Esc from Review must respect `hasTriageStep`:
- If `hasTriageStep == true`: Esc from Review → Triage (not Discovery)
- If `hasTriageStep == false`: Esc from Review → Discovery (existing behavior)

**Breadcrumb reverse mapping (shell index → step):** The mouse click handler in `updateMouse` maps shell breadcrumb indices back to step enums. This reverse mapping must handle all 4 permutations:

| Permutation | Shell labels | Index→Step mapping |
|-------------|-------------|-------------------|
| +Type +Triage | Source(0) Type(1) Discovery(2) Triage(3) Review(4) Execute(5) | direct |
| +Type -Triage | Source(0) Type(1) Discovery(2) Review(3) Execute(4) | 3→Review, 4→Execute |
| -Type +Triage | Source(0) Discovery(1) Triage(2) Review(3) Execute(4) | 1→Discovery, 2→Triage, etc. |
| -Type -Triage | Source(0) Discovery(1) Review(2) Execute(3) | 1→Discovery, 2→Review, etc. |

Implementation: add a `stepForShellIndex(idx int) addStep` method (inverse of `shellIndexForStep`).

Implementation notes:
- `updateKeyReview` (keyboard Esc) currently hardcodes `m.step = addStepDiscovery` — must change to respect `hasTriageStep`
- `updateMouseReview` (`add-back` zone handler) also hardcodes `m.step = addStepDiscovery` — same fix needed
- Both keyboard and mouse handlers must stay in sync

#### Mouse

Zone-mark every item row (`triage-item-{i}`), preview pane (`triage-preview`), and buttons (`triage-cancel`, `triage-back`, `triage-next`). Click item to focus + toggle. Click buttons to activate.

**Mouse handler requirements** (per `tui-wizard-patterns.md` checklist and `tui-elm.md` rule 7):
- Add `case addStepTriage:` branch in `updateMouse` for per-step click routing
- Add `case addStepTriage:` branch in `updateMouseWheel` for preview scrolling
- Every zone-marked element in the View must have a corresponding `zone.Get(id).InBounds(msg)` check

#### Default Selection

| Tier | Pre-checked | Rationale |
|------|-------------|-----------|
| High | ✓ Yes | Analyzer is confident |
| User-asserted | ✓ Yes | User explicitly identified the directory |
| Medium | ✗ No | Uncertain — user should review |
| Low | ✗ No | Weak signals — likely noise |

#### Triage → Review Merge

When advancing from Triage to Review, included confirm-bucket items are converted to `addDiscoveryItem` and appended to the wizard's item list:

```go
func (m *addWizardModel) mergeConfirmIntoDiscovery() {
    // IDEMPOTENCY: strip any previously-merged confirm items before re-merging.
    // This prevents duplicates when the user navigates Back from Review to Triage
    // and then advances again with different selections.
    m.discoveredItems = m.discoveredItems[:m.actionableCount+m.installedCount]

    for i, item := range m.confirmItems {
        if !m.confirmSelected[i] { continue }
        di := addDiscoveryItem{
            name:            item.displayName,
            itemType:        item.itemType,
            path:            item.path,
            sourceDir:       item.sourceDir,
            status:          add.StatusNew,
            confidence:      item.detected.Confidence,
            detectionSource: "content-signal", // always use literal, not item.detected.Provider
            tier:            item.tier,
        }
        // Build catalogItem for drill-in preview
        ci := buildCatalogItemFromDetected(item.detected)
        di.catalogItem = &ci
        di.risks = catalog.RiskIndicators(ci)
        m.discoveredItems = append(m.discoveredItems, di)
    }
    // Rebuild discovery list to include merged items
    m.discoveryList = m.buildDiscoveryList()
    // Auto-select the merged items
    for i := m.actionableCount; i < len(m.discoveredItems); i++ {
        m.discoveryList.selected[i] = true
    }
}
```

**Idempotency guarantee:** The truncation `m.discoveredItems[:m.actionableCount+m.installedCount]` removes any previously-merged confirm items before re-appending. This makes the function safe to call multiple times (Back → Triage → change selections → Next).

The merged items appear in the Review step alongside pattern-detected items. Their tier badge is visible in the review item list for context.

#### validateStep

```go
case addStepTriage:
    if !m.hasTriageStep {
        panic("wizard invariant: addStepTriage entered without confirm items")
    }
    if len(m.confirmItems) == 0 {
        panic("wizard invariant: addStepTriage entered with empty confirm items")
    }
```

#### stepHints

```go
case addStepTriage:
    return append([]string{"↑/↓ navigate", "space toggle", "a all", "n none",
        "tab switch panes", "enter next", "esc back"}, base...)
```

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Metadata persistence | Store on `.syllago.yaml` | Survives across sessions; visible in install wizard and library |
| Analyzer scope | All four sources | Uniform behavior; any source could contain non-standard layouts |
| Triage step placement | Between Discovery and Review | Keeps auto/confirm concepts visually separated |
| Left pane format | Single-line items, right-aligned tier | Compact, consistent with discovery list pattern |
| Default selection | High + User pre-checked | Balances friction reduction with safety |
| Tier label format | One word (High/Medium/Low/User) | Compact for right-alignment; color carries the meaning |
| User-asserted color | Purple (accentColor), not yellow | Opposite semantics from Low — must be visually distinct (panel R3) |
| Triage item columns | Name + Type + Tier (responsive) | Matches discovery step format; collapses at narrow widths (panel R2, I1) |
| User-directed detection | Explicit `detection_method` field | Avoids fragile float equality on confidence=0.60 (panel R5) |
| Dedup strategy | Path-based, pattern wins | Pattern detection is higher confidence than content-signal |
| Triage step conditional | Skip when no confirm items | Doesn't clutter the flow for clean repos |
| TierForMeta function | Separate from TierForItem | Avoids TUI dependency on `*DetectedItem`; works from metadata fields |

## Error Handling

| Scenario | Handling |
|----------|----------|
| Analyzer fails during discovery | Handled INSIDE each backend — catch `analyzer.Analyze()` error, return pattern-detected items with empty confirm slice. Never propagate as backend return error (which would trigger error screen and discard valid pattern items). Push a toast: "Content analysis unavailable — showing pattern-detected items only." |
| All confirm items excluded | Allow advancing to Review with zero confirm items selected. No error — user deliberately skipped them. |
| Triage step entered with stale data | Seq number check (existing pattern) rejects stale `addDiscoveryDoneMsg`. `clearTriageState()` prevents ghost items. |
| Metadata fields missing on old items | `confidence: 0` and `detection_source: ""` — tier badge not shown. Backwards compatible. |
| Path traversal in preview relPath | `ReadFileContent` validates containment before reading. Returns error for paths escaping basePath. |

## Testing Strategy

### Surface 1
- **Unit:** `TierForMeta()` with same test cases as `TierForItem()`
- **Integration:** Add item with confidence metadata → load from library → verify Meta fields populated
- **TUI golden:** Install wizard review step with content-signal item shows tier badge
- **TUI golden:** Install wizard review step without content-signal item shows no tier line

### Surface 2
- **Wizard invariant tests:** Forward-path through all 6 steps (with triage). Esc/back from triage returns to discovery. Skip triage when no confirm-bucket items.
- **Triage step tests:** Pre-check behavior (High=checked, Low=unchecked). Toggle include/skip. Select all / none. Cursor navigation.
- **Discovery backend tests:** Verify analyzer runs alongside pattern detection. Dedup correctness.
- **Merge tests:** Confirm items properly converted to addDiscoveryItem. Appear in Review step. Idempotency: Back→Triage→Next does not duplicate items.
- **Parallel array tests:** `confirmItems` and `confirmSelected` stay in sync (required by wizard pattern checklist).
- **Golden files:** Triage step at 80x20, 80x30, 120x40 with varied item counts and tier distributions. (TUI minimum is 80x20 — below that, `renderTooSmall()` fires. No need to test below the enforced floor.)
- **Mouse tests:** Zone-marked items, buttons, preview pane.

## Success Criteria

1. Library items that originated from content-signal detection show tier badges in the install wizard
2. Add wizard surfaces confirm-bucket items in a dedicated triage step
3. Users can include/exclude confirm items with space/a/n keys and mouse clicks
4. High-confidence and user-asserted items are pre-checked; medium/low are not
5. Triage step is skipped entirely when no confirm items exist
6. Included confirm items appear in the Review step for final approval
7. Metadata persists across sessions — tier info visible on subsequent TUI launches
8. All four discovery sources (provider, registry, local, git) run the analyzer

## Panel Review Findings

### First Panel (2026-04-02)

Four-persona discussion-style review. Three rounds, consensus on all points. No blocking issues.

| # | Change | Origin |
|---|--------|--------|
| R1 | Back-navigation from Review respects `hasTriageStep` | TUI Architect |
| R2 | Content type column added to triage item rows | Product Designer, Developer |
| R3 | User-asserted tier uses `accentColor` (purple) instead of `warningColor` (orange) | Product Designer, Developer |
| R4 | Golden tests at 80x20 (minimum enforced size) | TUI Architect |
| R5 | `detection_method` field replaces float equality for user-directed tier detection | Security Engineer |

### Second Panel (2026-04-02)

Same four personas, separate parallel agents, 2 rounds cross-discussion. Found 3 blocking issues.

**Blocking (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| B1 | `detection_method` in `.syllago.yaml` is attacker-writable — malicious package can set `user-directed` for pre-check + badge | Scanner-computed only; strip from incoming YAML | Security |
| B2 | `mergeConfirmIntoDiscovery` not idempotent — Back→Triage→Next duplicates items | Truncate to pre-merge length before re-appending | Developer + Security |
| B3 | Breadcrumb reverse mapping (shell index → step) missing for click navigation across 4 permutations | `stepForShellIndex()` method + truth table; fix both keyboard and mouse handlers | TUI Architect |

**Important (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| I1 | Left pane too cramped at 80x20 for name+type+tier | Responsive column collapse with breakpoints | Developer + Product Designer |
| I2 | Missing `addStepTriage` mouse handler branches | Add `case addStepTriage:` in updateMouse and updateMouseWheel | TUI Architect |
| I3 | "Confirm" name collides with "Review" semantically | Renamed to "Triage" throughout | Product Designer |
| I4 | Parallel array tests missing for confirmItems/confirmSelected | Added to testing strategy | TUI Architect |

**Minor (noted, not all applied):**

- Path dedup should use `filepath.Clean` + canonicalization (applied)
- Symlink resolution needed for provider source dedup (applied)
- `warningColor` doc label said "yellow", actually Flexoki orange (fixed)
- "Detection" label on install wizard vague — consider "Source" or "Found via"
- Show "(0 selected)" when all triage items excluded before advancing
- Existing `shell.SetActive(N)` test calls need updating after enum shift

### Third Panel (2026-04-03)

Same four personas, separate parallel agents, 2 rounds cross-discussion. Found 2 blocking and 4 important issues.

**Blocking (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| B4 | Stale triage state survives back-navigation and source changes — ghost items, wrong shell labels, stale security metadata | `clearTriageState()` method called from `goBackFromDiscovery()` and `advanceFromSource()`; nil-and-reconstruct `confirmPreview` | All four personas |
| B5 | `confidence` and `detection_source` also attacker-writable in `.syllago.yaml` — extends B1 to all analyzer fields | Allowlist approach: strip all analyzer-produced fields at import boundary, not just `detection_method` | Security |

**Important (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| I5 | Triage step title gives no context for WHY items need review | Added subtitle: "These were flagged for review — include or skip each one" | Product Designer |
| I6 | Auto items merged into Discovery have no visual provenance indicator | `detectionSource` field on merged Auto items; discovery view shows `(detected)` suffix or dot | Developer + Product Designer |
| I7 | Dot-only mode at <24 chars removes too much context for triage decisions | Use abbreviated tier labels `H`/`M`/`L`/`U` instead of dot-only | Product Designer, supported by all |
| I8 | WindowSizeMsg propagation to triage sub-models | Folded into B4 via nil-and-reconstruct pattern on `confirmPreview` | Developer + TUI Architect |

**Minor (noted):**

- `SignalEntry.Signal` sanitization for audit log defense-in-depth
- `TierForItem()` float equality deprecation in favor of `TierForMeta` pattern
- Primary file selection for triage preview undefined (first-file fallback acceptable)

### Fourth Panel (2026-04-03)

Same four personas, separate parallel agents, 2 rounds cross-discussion. Found 2 blocking and 5 important issues.

**Blocking (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| B6 | `ReadFileContent` lacks path containment — crafted `relPath` with `../` reads arbitrary files through triage preview | `filepath.Clean` + prefix validation against basePath before reading | Security |
| B7 | `maxStep` not reset in `clearTriageState()` — stale value allows clicking out-of-bounds breadcrumb steps (panic) | Reset `maxStep` to `addStepDiscovery` in `clearTriageState()`, call `updateMaxStep()` | TUI Architect |

**Important (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| I9 | Analyzer ignores `preFilterType` — "Add Skills" surfaces all types in triage | Backends filter Auto/Confirm items by `selectedTypes` before returning | Developer + TUI Architect |
| I10 | Analyzer failure surfaces as backend error, discarding valid pattern items | Catch inside backends; return pattern items + empty confirm slice + toast | Developer + Product Designer |
| I11 | Provider home dir trust boundary — analyzer could surface user config files | Disable content-signal fallback for provider sources (pattern detection sufficient) | Security |
| I12 | Enter/space inconsistency between Discovery (drill-in) and Triage (toggle) | `enter` = advance (no drill-in — preview pane sufficient), `space` = sole toggle key | Product Designer + TUI Architect |
| I13 | "Include"/"skip" semantics unexplained — non-persistence invisible to user | Subtitle communicates skip behavior with scoped language | Product Designer |

**Minor (noted):**

- No analyzer timeout/cancellation for large repos
- Metadata allowlist lacks compile-time enforcement mechanism
- Single-item triage could auto-skip (like single-provider in install wizard)
- Temp dir race during triage back-nav with in-flight git discovery

### Fifth Panel (2026-04-03)

Same four personas, separate parallel agents, 2 rounds cross-discussion. **0 blocking issues found.**

**Important (applied to design above):**

| # | Issue | Fix | Origin |
|---|-------|-----|--------|
| I14 | Triage drill-in undefined and risks state collision with review drill-in | Removed from scope — triage preview pane is sufficient. No drill-in state fields needed. | All four personas (convergent) |
| I15 | Symlink escape guard needed for `buildCatalogItemFromDetected` and `copySupportingFiles` | `SafeResolve(baseDir, untrustedPath)` utility. Exclude `.syllago.yaml` from copy set. | Security |
| I16 | Analyzer scope contradiction — doc says "all four sources" but I11 disables for providers | Corrected: three sources + pattern-only for providers | Developer |
| I17 | Missing `case addStepTriage:` in `updateKey` keyboard routing (I2 only covered mouse) | Add keyboard case alongside mouse case | TUI Architect |
| I18 | Skip semantics wording — "future scans" implies background scanning | Reworded: "next time you add from this source" | Product Designer |

**Minor (noted):**

- `clearTriageState` unreachable from Execute (not a bug, document why)
- `updateMaxStep` sync in `clearTriageState` — next Update cycle handles it
- Wizard step count in `tui-wizard-patterns.md` stale (5→6)
- Auto-item provenance indicator column — post-launch enhancement
- Surface 1 tier badge near-zero visibility after I11 — future-proofing, acknowledged

### Accepted As-Is (all five panels)

- Conditional step pattern (follows `preFilterType` precedent)
- Step enum shift (named constants, not raw integers — safe)
- Default selection (High+User pre-checked)
- Analyzer on three sources (registry, local, git) + pattern-only for providers
- Metadata persistence on `.syllago.yaml`
- `TierForMeta` as separate function from `TierForItem`
- Triage-to-review merge via `mergeConfirmIntoDiscovery` (with idempotency fix)
- TUI minimum 80x20 enforced — no sub-minimum testing needed
- Seq-based stale message rejection covers new confirm items correctly
- `enterReview()` naturally works with merged triage items via `selectedItems()`
- Triage uses preview-only (no drill-in) — deep inspection belongs in library post-install
- Preview scroll position loss on triage re-entry (consistent with existing patterns)

## Open Questions

None — all design decisions resolved during brainstorm and five panel reviews.

---

## Next Steps

Ready for implementation planning with the Plan skill.
