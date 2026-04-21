# MOAT Phase 2c — TUI Trust Surfacing (Spec)

**Bead:** syllago-nyhgy (spec + expert panel) → syllago-lqas0 (plumbing) → syllago-hf5am (TUI) → syllago-0kbbx (validation)
**ADR:** 0007 (MOAT v0.6.0 integration)
**Date:** 2026-04-20
**Panel review:** see `2026-04-20-moat-tui-trust-surfacing-findings.md` (applied below)
**Symbol-accuracy pass:** after the panel review, a second sonnet-model reviewer found 11 MUST-FIX symbol/signature errors. All corrected below — every referenced symbol has been grepped against the actual code.

---

## 1. Overview

Surface MOAT trust state in the syllago TUI for library-installed and registry-browse content. Four user-visible additions:

1. **Row badges** on library and content tab list rows — AD-7 Panel C9 three-state collapse. Glyphs: `✓` (Verified, green), `R` (Recalled, red), space (Unsigned/Unknown).
2. **Metapanel drill-down** — expanded tier text (DUAL-ATTESTED vs SIGNED distinction), revocation banner with reason/issuer/details_url, private-repo indicator.
3. **Publisher-warn install modal** — when the install flow detects a publisher-source revocation, confirm before proceeding. Wired to the existing `installer.PreInstallCheck` gate decision (not to a catalog field) to avoid drift from the CLI path.
4. **Trust-details modal** — reusable drill-in modal for Recalled items that exposes the full `details_url` with `[y]` clipboard yank. Foundation for future attestation-mismatch surfacing.

Scope also includes the plumbing that feeds the TUI: new `ContentItem` fields, sanitization at enrich boundary, and a producer call site that invokes `moat.EnrichCatalog` from production catalog scan.

## 2. In scope / Out of scope

### In scope
- 4 new fields on `catalog.ContentItem` with zero-value semantics for non-MOAT items (`PrivateRepo`, `RecallSource`, `RecallDetailsURL`, `RecallIssuer`).
- Extension of `moat.EnrichCatalog` to populate all new fields. Sanitization at the enrich boundary (including existing `RecallReason` which is currently unsanitized) so consumers never see raw publisher-controlled strings.
- Update `catalog.TrustBadge.Glyph()` to return `"R"` for `TrustBadgeRecalled` (replacing `"✗"`). Keeps CLI `syllago show` and TUI display in sync.
- Producer wiring: `moat.EnrichFromMOATManifests` (package: `moat`, not `catalog`), called inside a `tea.Cmd` from catalog-scan pipeline. Migrate `rescanCatalog()` from synchronous mutation to async `catalogReadyMsg` atomic-swap pattern.
- Enumerate call sites: `grep -rn 'catalog\.Scan\|catalog\.ScanWithGlobal' cli/cmd/syllago/ cli/internal/tui/` and migrate all to `moat.ScanAndEnrich` factory.
- Row badge rendering in library + content tabs with 3-char prefix.
- Metapanel drill-down (Trust chip + revocation banner + private-repo chip).
- New `internal/tui/modals/` package with `publisher_warn.go` AND `trust_details.go` AND `messages.go`.
- Install-wizard integration via new `installStepPublisherWarn` step (with `validateStep()` entry-precondition + mouse zone marks + wizard-invariant tests per `.claude/rules/tui-wizard-patterns.md`).
- Golden file tests at 60x20, 80x30, 120x40 (trimmed per §12).
- Unit tests on new catalog fields + modal routing + sanitization helpers + truth table of trust states.
- Human smoke test against real MOAT content from `syllago-meta-registry`.

### Out of scope
- Changes to normative moat package primitives that break existing callers (`TrustTier`, `Revocation`, `PreInstallCheck`, `Session`) — frozen.
- **Reason-change invalidates publisher-warn confirmation.** `moat.Session.MarkConfirmed(registryURL, contentHash)` keys only on `(registry, hash)`. Adding a reason-aware key requires new `Session` API — deferred to follow-up bead (`syllago-2h86l`).
- Changes to AD-7 Panel C9's three-state collapse *rule* — rule stays. This spec changes one glyph (`✗` → `R`) and adds drill-down text suffix `(publisher)`/`(registry)` that doesn't violate collapse.
- `SYLLAGO_TUI_ACCESSIBLE` env var for screen-reader bracketed tokens (follow-up bead `syllago-ym3pl`).
- Nested `MOATState` struct refactor on `ContentItem` (follow-up bead `syllago-sn97y`).
- Enrich-time re-verification of manifest `signature.bundle` (follow-up bead `syllago-dwjcy`). Trust model: "we trust user's filesystem under `cacheDir`"; sync-time verification is authoritative.
- Staleness-warn-band visible degraded badge in TUI — dropped entirely (see §6). Binary Fresh vs Stale/Expired. Uses existing `moat.StalenessFresh`/`StalenessStale`/`StalenessExpired` trichotomy; `StalenessStale` and `StalenessExpired` both skip enrichment.
- Lockfile-driven trust for already-installed MOAT content outside the catalog-scan pipeline (future bead).

### Threat model

**Protects against:**
- User installing a malicious package that the registry revoked after initial publish (registry-source revocation = hard-block per `MOATGateHardBlock`).
- User installing a package the publisher revoked (publisher-source revocation = confirm-once session-scoped per `MOATGatePublisherWarn`).
- ANSI escapes, bidi overrides, or C0/C1 controls smuggled via publisher-controlled strings (`RecallReason`, `item.Name`, `RecallIssuer`, `RecallDetailsURL`) that could rewrite terminal state or invert glyph meanings.
- Accidentally stale-Verified badge (fail-closed at 72h per `moat.DefaultStalenessThreshold` + expiry-wins-over-freshness).

**Does NOT protect against:**
- Same-user compromise (attacker with write access to `~/.cache/syllago/`).
- Offline attacks where the user runs syllago without ever having synced a manifest (`TrustTier=Unknown` for everyone, no trust decisions possible).
- Registry operator compromise (the registry's signature is trusted; chain of trust rooted at sigstore).

**Invariants on `ContentItem` MOAT fields:**
- All publisher-controlled strings pass through `moat.SanitizeForDisplay` before write.
- Write authorized only from `moat.EnrichCatalog` via `moat.EnrichFromMOATManifests`, inside a `tea.Cmd`. Never from `Update()` or `View()`.
- Atomic catalog swap on rescan: new catalog constructed off-tree, then emitted via `catalogReadyMsg{cat}` for `Update()` to swap in.

## 3. Design decisions

| Decision | Rationale |
|----------|-----------|
| Row glyphs: `✓` (green) verified, `R` (red, bare) recalled, space otherwise | Shape-distinct for color-blind users; bare `R` avoids confusion with `[key]` hotkey brackets; `✓` kept because universally recognized. |
| `TrustBadge.Glyph()` also updates `✗` → `R` | Keeps CLI `syllago show` and TUI in sync; avoids two glyphs for one state. |
| Full tier text (DUAL-ATTESTED vs SIGNED) only appears in drill-down | Casual browsing stays uncluttered; users who care drill in. |
| Publisher-warn source distinction visible in drill-down as text suffix (not amber `!` prefix) | Text `RECALLED (publisher) — …` or `RECALLED (registry) — …` is self-describing and color-independent; removes amber/red two-color glyph collision. |
| Publisher-warn modal reads from `installer.PreInstallCheck` gate decision, not `ContentItem.RecallSource` | Avoids parallel decision paths that drift from CLI. `PreInstallCheck` is authoritative. Gate re-evaluated fresh at install commit, not cached from wizard entry. |
| Session-scoped publisher-warn confirmation uses existing `(registryURL, contentHash)` key | Existing `moat.Session` API. Reason-change re-triggers is deferred (API extension needed — see follow-up bead). |
| Private-repo row glyph: bare `P`, muted style | Matches bare-`R` asymmetry (single-char in row column). Drill-down spells out "Private" in a `Visibility` chip. |
| `EnrichCatalog` + `EnrichFromMOATManifests` + `ScanAndEnrich` all live in `moat` package | `moat` already imports `catalog`; reverse direction would cycle. |
| Producer opt-in per `config.Registry.Type == "moat"` | Non-MOAT (git) registries don't synthesize fake trust state. |
| Fail-closed on `StalenessStale` or `StalenessExpired` or missing bundle | Items stay `TrustTier=Unknown`. No degraded-visible warn band. |
| Staleness threshold = 72h (from `moat.DefaultStalenessThreshold`) — not re-declared in spec or code | Single source of truth; the moat primitive is spec-normative. |
| Sanitization at enrich boundary, not at render | Single chokepoint; consumers can't forget. Covers existing `RecallReason` which is currently unsanitized. |
| Trust-details modal used both for Recalled URL recovery and future attestation-mismatch drill-down | Reusable modal infra avoids rebuilding per-surface. Land foundation in 2c. |
| App gets two new concrete modal fields (`publisherWarn`, `trustDetails`), NOT a polymorphic `activeModal` interface | Consistent with existing pattern (`modal editModal`, `confirm confirmModal`, `remove removeModal`, etc.). No refactor of modal dispatch. |

## 4. New `ContentItem` fields

Extend `cli/internal/catalog/types.go`:

```go
type ContentItem struct {
    // ... existing fields ...

    // MOAT trust state (ADR 0007). All zero-valued for non-MOAT items.
    TrustTier    TrustTier  // existing
    Recalled     bool       // existing
    RecallReason string     // existing. IMPORTANT: currently unsanitized;
                            // lqas0 adds SanitizeForDisplay at the enrich
                            // boundary in EnrichCatalog.

    // NEW in 2c — all publisher-controlled strings are pre-sanitized at
    // enrich time via moat.SanitizeForDisplay. Consumers treat values as
    // trusted for display.

    // PrivateRepo mirrors ContentEntry.PrivateRepo (G-10). Independent of
    // registry-level Visibility probe — per-item publisher declaration.
    // Field name matches the source field, avoiding method-name shadow
    // (ContentEntry has an IsPrivate() method; ContentItem does not).
    PrivateRepo bool

    // RecallSource is Revocation.EffectiveSource() — "registry" or
    // "publisher" when Recalled; empty otherwise. Drives drill-down
    // banner text suffix. NOT read by the install wizard (which uses
    // PreInstallCheck instead).
    RecallSource string

    // RecallDetailsURL is Revocation.DetailsURL when Recalled. Empty for
    // publisher-source revocations where DetailsURL is optional per spec.
    // Displayed verbatim in the revocation banner (middle-truncated if
    // too wide) AND in full in the trust-details modal.
    RecallDetailsURL string

    // RecallIssuer is the identity of the revoker:
    //   - registry-source: Manifest.Operator (falls back to
    //     Manifest.RegistrySigningProfile.Subject if Operator empty;
    //     validate() guarantees that fallback is non-empty).
    //   - publisher-source: ContentEntry.SigningProfile.Subject if
    //     present, else literal "(publisher — identity not provided)".
    // Empty when not Recalled.
    RecallIssuer string
}
```

**Why separate fields instead of a nested struct:** keeps existing `ContentItem` consumers simple, and Go's zero-value semantics give us the "not MOAT-sourced" default for free. A nested `MOATState` refactor is follow-up bead `syllago-sn97y`.

**Field naming convention:** `PrivateRepo` (matching source field), not `IsPrivate`. The `Is`-prefix is reserved for methods in Go. `ContentEntry.IsPrivate()` is a method; `ContentItem.PrivateRepo` is a field.

**Zero-value collision note (documented, accepted):** `TrustTier == TrustTierUnknown` conflates "not MOAT-sourced" with "MOAT-sourced but cache stale." Both render identically (no badge). Distinguishing them is out-of-scope for this phase; `syllago trust-status` CLI disambiguates for operators.

**`AttestationHashMismatch` NOT added to `ContentItem`.** `moat.ContentEntry.TrustTier()` already downgrades the tier defensively. Carrying a display-only second signal invites drift. If a future drill-down needs the distinction, derive at render time.

## 5. `EnrichCatalog` extension

Modify `cli/internal/moat/enrich.go` + new `cli/internal/moat/sanitize.go`:

```go
// In sanitize.go:

// SanitizeForDisplay strips ANSI CSI/OSC sequences, C0/C1 control chars
// (except \t), DEL, bidi overrides (U+202A–U+202E, U+2066–U+2069), and
// line separators (U+2028, U+2029). Multi-line content collapses to
// single line with newlines → space. Empty input returns "".
func SanitizeForDisplay(s string) string { ... }

// In enrich.go — modified EnrichCatalog:

func EnrichCatalog(cat *catalog.Catalog, registryName string, m *Manifest) {
    if cat == nil || m == nil {
        return
    }

    revByHash := make(map[string]*Revocation, len(m.Revocations))
    for i := range m.Revocations {
        h := m.Revocations[i].ContentHash
        if _, ok := revByHash[h]; !ok {
            revByHash[h] = &m.Revocations[i]
        }
    }

    for i := range cat.Items {
        item := &cat.Items[i]
        if item.Registry != registryName {
            continue
        }
        entry, ok := FindContentEntry(m, item.Name)
        if !ok {
            continue
        }

        item.TrustTier = moatTierToCatalogTier(entry.TrustTier())
        item.PrivateRepo = entry.PrivateRepo

        if rev, ok := revByHash[entry.ContentHash]; ok {
            item.Recalled = true
            item.RecallReason = SanitizeForDisplay(rev.Reason) // NEW sanitization
            item.RecallSource = rev.EffectiveSource()
            item.RecallDetailsURL = SanitizeForDisplay(rev.DetailsURL)
            item.RecallIssuer = resolveRecallIssuer(rev, m, entry)
        }
    }
}

// resolveRecallIssuer uses Manifest.Operator (value field, no nil guard
// needed). RegistrySigningProfile is ALSO a value field — Manifest.validate()
// enforces its Subject is non-empty at parse time, so the fallback is safe.
func resolveRecallIssuer(rev *Revocation, m *Manifest, entry *ContentEntry) string {
    switch rev.EffectiveSource() {
    case RevocationSourceRegistry:
        if m.Operator != "" {
            return SanitizeForDisplay(m.Operator)
        }
        // Subject is guaranteed non-empty by validate().
        return SanitizeForDisplay(m.RegistrySigningProfile.Subject)
    case RevocationSourcePublisher:
        if entry.SigningProfile != nil && entry.SigningProfile.Subject != "" {
            return SanitizeForDisplay(entry.SigningProfile.Subject)
        }
        return "(publisher — identity not provided)"
    }
    return ""
}
```

`moatTierToCatalogTier` already exists at `enrich.go:54`; no change.

## 6. Producer wiring

New function `moat.EnrichFromMOATManifests` (new file `cli/internal/moat/producer.go`):

```go
// EnrichFromMOATManifests iterates MOAT-type registries in cfg, loads each
// registry's cached manifest + signature.bundle, checks staleness via
// moat.CheckRegistry (which keys on lockfile's per-registry FetchedAt),
// and calls EnrichCatalog on cat. Fail-closed: StalenessStale,
// StalenessExpired, missing cache, or missing bundle → items from that
// registry stay at TrustTier=Unknown.
//
// Per-registry parse/read failures append to cat.Warnings and enrichment
// continues. Returns error only on programmer error (nil cat or nil cfg).
//
// MUST be called inside a tea.Cmd, never from Update() or View().
// Enforced by convention + tui-elm.md rule #3.
func EnrichFromMOATManifests(
    cat *catalog.Catalog,
    cfg *config.Config,
    lf *Lockfile,
    cacheDir string,
    now time.Time,
) error
```

**Cache layout (committed):**
- `filepath.Join(cacheDir, "moat", "registries", <sanitized-name>, "manifest.json")`
- `filepath.Join(cacheDir, "moat", "registries", <sanitized-name>, "signature.bundle")`

**Registry name sanitization:**
- Regex `^[a-z0-9][a-z0-9._-]{0,63}$` enforced at config-load AND cache-path construction (defense in depth).
- After path construction, verify `filepath.Rel(cacheDir, resolvedPath)` stays under `cacheDir`.
- Test case: registry named `../escape` must be rejected at config-load with a clear error.

**Staleness check (uses existing primitives, no re-implementation):**
- Call `moat.CheckRegistry(lf, registryURL, manifest, now)`.
- Result `StalenessFresh` → enrich.
- Result `StalenessStale` OR `StalenessExpired` → skip enrichment. Items stay `TrustTier=Unknown`. Append `cat.Warnings` entry: `"MOAT cache <status> for registry <name>; trust decisions disabled"` where `<status>` is the `StalenessStatus.String()`.

The 72h threshold is `moat.DefaultStalenessThreshold` — not re-declared in spec. Spec depends on whatever value the primitive carries.

**No live network sync in this producer.** Live sync is a separate flow (`syllago registry sync`). `EnrichFromMOATManifests` is pure cache-read.

**Bundle verification is sync-time, not enrich-time.** `syllago registry sync` verifies `manifest.json` against `signature.bundle` and only persists both on success. Enrich-time reads trust the filesystem. Enrich-time re-verification is follow-up bead `syllago-dwjcy`.

**Factory pattern — `moat.ScanAndEnrich`:**

```go
// ScanAndEnrich is the production pipeline used by the TUI rescan Cmd
// and all CLI commands that build a live catalog. Lives in moat package
// so it can call both catalog.Scan* and EnrichFromMOATManifests.
//
// Returns a fully-populated catalog ready for atomic swap. Never mutates
// an existing catalog.
func ScanAndEnrich(
    cfg *config.Config,
    root, projectRoot string,
    regSources []catalog.RegistrySource,
    lf *Lockfile,
    cacheDir string,
    now time.Time,
) (*catalog.Catalog, error)
```

Signature mirrors the existing `catalog.ScanWithGlobalAndRegistries(root, projectRoot, regSources)` call used at `cli/internal/tui/app.go:228` plus the additional MOAT-specific inputs.

**Enumerated call sites (all migrated in `lqas0`):**
- `cli/internal/tui/app.go#rescanCatalog` at line 228 — replace `catalog.ScanWithGlobalAndRegistries` call. Also migrate the function itself to return `tea.Cmd` that emits `catalogReadyMsg` (see Concurrency rule below).
- `cli/cmd/syllago/main.go:302` and `main.go:314` — `catalog.ScanWithGlobalAndRegistries` (full catalog with registries) → migrate to `moat.ScanAndEnrich`.
- `cli/cmd/syllago/init.go`, `add_cmd.go`, `loadout_apply.go`, `loadout_list.go`, `loadout_create.go`, `sync_and_export.go` — each construct their own catalog via direct `catalog.Scan*` calls. Migrate.
- **Explicitly NOT migrated:**
  - `cli/cmd/syllago/main.go:138` — `catalog.Scan(root, projectRoot)`, narrow pre-TUI scan before registry sources are resolved. No registry content to enrich.
  - `cli/cmd/syllago/install_cmd.go:233` and `install_cmd.go:473` — `catalog.Scan(globalDir, globalDir)`, global-library-only scans (no registry content). No enrichment needed.
- Full enumeration via `grep -rn 'catalog\.Scan\|catalog\.ScanWithGlobal' cli/cmd/syllago/ cli/internal/tui/` during `lqas0` execution; cross-check against the migrate / NOT-migrate lists above.

**Concurrency rule — `rescanCatalog()` migration:**

Current `rescanCatalog()` at `app.go:200-236` is synchronous and directly mutates `a.catalog = cat` inside the function body. This violates `tui-elm.md` rule #3 ("no state mutation outside Update()").

Migration (part of `lqas0` scope):
1. `rescanCatalog()` continues to return `tea.Cmd` (same return type — no caller changes needed) but its body no longer mutates `a.catalog`. Instead it returns a `tea.Cmd` that calls `moat.ScanAndEnrich` off-tree.
2. On completion, Cmd emits new message type `catalogReadyMsg{cat *catalog.Catalog, err error}`.
3. `app_update.go` handles `catalogReadyMsg` by swapping `a.catalog = msg.cat` and calling `a.refreshContent() + a.updateNavState()`.
4. Failure path: toast warning, leave existing catalog in place (no partial update).

**Pre-existing bug to fix as part of the migration:**
- `app_update.go:408` calls `a.rescanCatalog()` and discards the returned `tea.Cmd` (currently a no-op because the scan is synchronous; after migration the scan would be silently dropped). Capture the returned cmd and thread it through the `Update()` return (e.g., `cmd := a.rescanCatalog(); return a, cmd`).
- Other call sites already capture correctly: `actions.go:288,300,372,400,412,601,617` (as `cmd2`) and `app_update.go:334,398` (as `cmd`).

New message type: `catalogReadyMsg` (defined in `app_update.go` or `app.go`).

## 7. Row badge rendering

Consumer: `cli/internal/tui/library*.go` row renderer + content tab equivalents.

Critical note: the spec changes `TrustBadge.Glyph()` to return `"R"` for `TrustBadgeRecalled` (was `"✗"`). Row renderer still builds styled glyph inline because the glyph needs to be wrapped in a color style that `Glyph()` alone cannot express.

```go
import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// In the row-render helper:
badge := catalog.UserFacingBadge(item.TrustTier, item.Recalled)
var trustGlyph string
var trustStyle lipgloss.Style
switch badge {
case catalog.TrustBadgeVerified:
    trustGlyph = "✓"
    trustStyle = trustVerifiedStyle       // successColor, bold
case catalog.TrustBadgeRecalled:
    trustGlyph = "R"                       // matches updated Glyph()
    trustStyle = trustRecalledStyle       // dangerColor, bold
case catalog.TrustBadgeNone:
    trustGlyph = " "
}

privateGlyph := " "
if item.PrivateRepo {
    privateGlyph = "P"  // privateIndicatorStyle — faintColor
}

// 3-char prefix: <trust><private><space>
prefix := trustStyle.Render(trustGlyph) + privateIndicatorStyle.Render(privateGlyph) + " "
```

**Column layout:** Fixed 3-char prefix: `<trust-glyph><private-glyph><space>`. Both inner slots fall back to space when absent.

**Name-column budget:** Existing name-column max-width in `library.go` and content tab renderers decreases by 3 characters. Full audit during `lqas0`.

**Narrow-terminal precedence (60x20):**
1. Trust glyph (safety signal) — never drops.
2. Item name — truncates with middle-ellipsis if needed.
3. Private `P` glyph — drops first when space tight.
4. Registry column — drops next.

**Named styles (in `styles.go`):**
- `trustVerifiedStyle`: `successColor`, bold.
- `trustRecalledStyle`: `dangerColor`, bold.
- `privateIndicatorStyle`: `mutedColor`, not bold.
- `revocationBannerStyle`: `dangerColor`, bold.

No inline `lipgloss.NewStyle()` at render sites.

## 8. Metapanel drill-down

Extend `cli/internal/tui/metapanel.go` using the existing `chip(key, val, width)` helper at metapanel.go:60-63 (which uses `len(key)` byte-count — this is fine because chip keys here are all ASCII; values with multi-byte glyphs pass through rune-aware `truncate`).

Layout diagram:

```
Line 1: chip("Name", name, 40) + chip("Type", type, 14) + chip("Files", n, 9) + chip("Origin", origin, 19) + chip("Installed", providers, 0)
Line 2: chip("Scope", scope, 16) + chip("Registry", reg, 28) + chip("Path", path, 0)
Line 3: [type-specific detail]                           [i] Install  [e] Edit
Line 4: chip("Trust", glyph + " " + description, 60) + chip("Visibility", visibilityText, 15)   ← NEW
Line 5: [Recalled banner, if Recalled]                                                           ← NEW, conditional
```

**Line 4 composition (chip pattern):**
- Trust chip value: `<glyph> <description>` where:
  - glyph = `✓` (verified) / `R` (recalled) / ` ` (unknown or unsigned)
  - description from `catalog.TrustDescription(tier, recalled, recallReason)`. Exact strings from `catalog/trust.go:113-129`:
    - DualAttested: `"Verified (dual-attested by publisher and registry)"` (46 chars)
    - Signed: `"Verified (registry-attested)"` (28 chars)
    - Unsigned: `"Unsigned (registry declares no attestation)"` (42 chars)
    - Unknown: `""` (empty)
    - Recalled with reason: `"Recalled — <reason>"`
    - Recalled no reason: `"Recalled"`
- Visibility chip (appended): `"Private"` when `PrivateRepo`, omitted when public.
- Whole Line 4 suppressed when `TrustTier == TrustTierUnknown && !PrivateRepo && !Recalled`. (Unsigned items DO render Line 4 — the text "Unsigned (registry declares no attestation)" is substantive information. Only Unknown / non-MOAT items suppress.)

**Line 5 (revocation banner):** Only when `Recalled`. Text-only source distinction:
```
│ RECALLED (registry) — <reason>. Issued by <issuer>. <details_url>
```
or:
```
│ RECALLED (publisher) — <reason>. Issued by <issuer>. <details_url>
```

- Rendered via `revocationBannerStyle` (`dangerColor`, `Bold(true)`).
- Left border `│` (U+2502, light) — NOT `┃` (heavy). Heavy box-drawing renders at 1.5× in some terminals.
- `(registry)` / `(publisher)` is TEXT, not a glyph. Replaces the amber `!` prefix originally proposed.
- `details_url` rendered verbatim; truncated with middle-ellipsis if exceeds available width; user presses `[d]` to open `TrustDetailsModel` with full URL + `[y]` clipboard yank.
- Conditional segments: if `RecallDetailsURL` empty, omit its segment entirely (no trailing space or double-period). Golden covers empty-URL rendering.

**Layout at narrow heights (60x20 collapse rule):**
- **Verified item at 60x20:** Hide Line 4 entirely (row badge already shows `✓`). Saves 1 row.
- **Recalled item at 60x20:** Merge Lines 4 + 5 into single line: `RECALLED (pub) — <reason>`. Drops `Issued by` and URL segments; user opens trust-details modal for full detail.
- **80x30 and above:** Full 5-line layout.

**Width discipline:** `MaxWidth`, explicit truncation via existing `truncate` helper. `lipgloss.Width()` for dimension checks only, never for rendering (per `.claude/rules/tui-layout.md` rule #3).

## 9. Private-repo indicator — committed

**Row glyph:** bare `P` (1 char), muted style (`privateIndicatorStyle` = `mutedColor`).

**Drill-down chip:** `chip("Visibility", "Private", 15)` — spelled out, not bracketed.

**Label note:** The value `PrivateRepo` reflects the publisher's self-declaration in `ContentEntry.PrivateRepo`. It is a claim, not a syllago-verified fact. Richer label "Publisher-declared private" under consideration for follow-up bead `syllago-2h86l`; 2c ships with "Private".

No emoji. No `🔒`.

## 10. Publisher-warn + trust-details modals

**New directory:** `cli/internal/tui/modals/` (doesn't exist yet — created in `hf5am` bead).
**New files:**
- `cli/internal/tui/modals/publisher_warn.go` — confirmation modal
- `cli/internal/tui/modals/trust_details.go` — URL-recovery drill-in modal
- `cli/internal/tui/modals/messages.go` — typed message types for both

### 10.1 PublisherWarnModel

```go
package modals

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/OpenScribbler/syllago/cli/internal/installer"
)

type PublisherWarnModel struct {
    itemName     string
    registryURL  string
    contentHash  string
    reason       string
    issuer       string
    detailsURL   string
    focus        int // 0 = Cancel (default), 1 = Confirm
    width        int
    height       int
}

// NewPublisherWarnModel is constructed from the GateBlock returned by
// installer.PreInstallCheck. The caller supplies itemName (ContentItem.Name)
// since it's not carried on GateBlock.Revocation.
func NewPublisherWarnModel(block installer.GateBlock, itemName string) *PublisherWarnModel

func (m *PublisherWarnModel) Init() tea.Cmd
func (m *PublisherWarnModel) Update(msg tea.Msg) (*PublisherWarnModel, tea.Cmd)
func (m *PublisherWarnModel) View() string
```

**Pointer receivers.** Consistent with existing wizard models.

**No `Confirmed()` getter.** Messages drive outcomes — state-getters invite stale reads.

### 10.2 TrustDetailsModel (reusable URL-recovery + future drill-down)

```go
type TrustDetailsModel struct {
    itemName     string
    trustTier    string // sanitized label e.g. "DUAL-ATTESTED", "SIGNED"
    source       string // "registry" | "publisher" | "" when not recalled
    reason       string // sanitized
    issuer       string // sanitized
    detailsURL   string // sanitized; rendered full-width, no truncation
    width        int
    height       int
}

func NewTrustDetailsModel(item catalog.ContentItem) *TrustDetailsModel
func (m *TrustDetailsModel) Init() tea.Cmd
func (m *TrustDetailsModel) Update(msg tea.Msg) (*TrustDetailsModel, tea.Cmd)
func (m *TrustDetailsModel) View() string
```

**Entry points:**
- User presses `[d]` on a Recalled row (library or content tab). App opens `TrustDetailsModel`.
- From HardBlock banner (see §10.8).
- Future: attestation-mismatch state (covered by separate bead).

**Clipboard yank (`[y]` key):**

BubbleTea does NOT have a built-in OSC 52 clipboard. Reuse the existing pattern from `cli/internal/tui/toast.go:copyAndDismiss` (toast.go:158-171) which writes OSC 52 bytes directly to `os.Stdout` from a `tea.Cmd`. `tea.Printf` is swallowed in alt-screen mode — that's why the direct stdout write is required.

Pre-yank extra sanitization: strip `\n`, `\r`, and shell metacharacters beyond what `SanitizeForDisplay` already handled. Implement as a helper `sanitizeForClipboard(s string) string` in the modals package.

**Keys inside modal:**
- `[y]` — yank. Emits toast on success/failure.
- `[esc]` or `[enter]` — dismiss.
- Mouse zone `trust-details-yank` wraps the `[y] Yank URL` footer button for mouse parity per `.claude/rules/tui-elm.md` rule #7.

### 10.3 Message types (in `modals/messages.go`)

```go
type PublisherWarnConfirmedMsg struct {
    ItemName    string
    RegistryURL string
    ContentHash string
}

type PublisherWarnCancelledMsg struct {
    ItemName string
}

type TrustDetailsDismissedMsg struct{}

type TrustDetailsYankedMsg struct {
    URL string
    Err error
}
```

`PublisherWarnConfirmedMsg` carries `RegistryURL` and `ContentHash` so the install wizard can call `installer.MarkPublisherConfirmed(session, registryURL, contentHash)` directly.

### 10.4 Message routing while modal active

| Message type | Goes to modal | Goes to background sub-models |
|--------------|:-------------:|:-----------------------------:|
| `tea.KeyMsg` | ✓ | ✗ (modal consumes) |
| `tea.MouseMsg` | ✓ | ✗ |
| `tea.WindowSizeMsg` | ✓ | ✓ (both need current dimensions) |
| `tickMsg` / `toastExpireMsg` | ✗ | ✓ |
| async result messages | ✗ | ✓ |

App adds two new concrete modal fields (alongside existing `modal editModal`, `confirm confirmModal`, `remove removeModal`, etc. at `app.go:45-50`):

```go
publisherWarn modals.PublisherWarnModel
trustDetails  modals.TrustDetailsModel
```

Each carries an `active bool` field following the existing modal convention. App's `Update()` dispatches to whichever is active. No polymorphic interface refactor.

### 10.5 Visual structure

**Title (mandated):** First rendered line in `warningColor`: `"Publisher revoked this content"`.
**Orientation line:** Second line: `"Installing <item> from <registry> will proceed despite publisher revocation."`
**Reason block:** Third segment — reason, issuer, details_url.
**Footer buttons:** Rendered via `renderModalButtons(focusIdx, usableW, pad, []string{"Confirm"}, buttons...)` from `cli/internal/tui/buttons.go:22`. The `[]string{"Confirm"}` `dangerLabels` argument renders the Confirm button in danger red (when focused), consistent with `removeModal.go:500` which uses `[]string{"Remove"}`. Cancel remains accent-styled (purple) since it is the safe default. Do NOT pass `nil` for `dangerLabels` — the Confirm action proceeds despite a publisher revocation and must render as a dangerous action, not a routine confirm.

**Default focus = Cancel.** Focus resets to Cancel on `WindowSizeMsg`.

**Key bindings:**
- `Tab` / `Shift+Tab` — cycle focus.
- `Left` / `Right` — cycle focus.
- `Enter` — activate focused button.
- `Esc` — equivalent to Cancel, dismisses with `PublisherWarnCancelledMsg`.
- No letter hotkeys.

**Width fallback:** Fixed width 56 when `width >= 58`. When terminal width 50–57, render at `width - 2`. Below 50, render inline error banner: `"Terminal too narrow for confirmation — resize to at least 50 columns"`.

**Manual box-drawing borders** (`╭─╮│╰─╯`) not `lipgloss.Border()` — per `.claude/rules/tui-modals.md` §2.
**Zone-marked buttons** for mouse parity per `.claude/rules/tui-elm.md` rule #7.

### 10.6 Install-wizard integration — `installStepPublisherWarn`

Existing wizard step enum at `cli/internal/tui/install.go:20-26`:
```go
const (
    installStepProvider installStep = iota
    installStepLocation
    installStepMethod
    installStepReview
    installStepConflict
)
```

Add new step AFTER `installStepReview` and BEFORE `installStepConflict`:
```go
const (
    installStepProvider installStep = iota
    installStepLocation
    installStepMethod
    installStepReview
    installStepPublisherWarn  // NEW
    installStepConflict
)
```

**Entry-precondition** (enforced by `validateStep()`):
- `m.gateBlock != nil && m.gateBlock.Decision == installer.MOATGatePublisherWarn`

**Entered from:** `installStepReview` when gate evaluates to `MOATGatePublisherWarn`.
**Exit paths:**
- Confirm → calls `installer.MarkPublisherConfirmed(session, registryURL, contentHash)` → transitions to `installStepConflict` (or skips to actual install if no conflict).
- Cancel / Esc → returns to `installStepReview`.

**HardBlock path:** When gate returns `MOATGateHardBlock`, wizard does NOT enter `installStepPublisherWarn`. Instead displays error banner at `installStepReview` (see §10.8) with no modal.

**Private-prompt path (`MOATGatePrivatePrompt`):** out of scope for this bead; existing `private_repo` prompt remains in CLI only. Future bead surfaces this in TUI.

**Tier-below-policy path (`MOATGateTierBelowPolicy`):** out of scope; CLI already exits with structured error.

**Wizard-invariant tests** (in `wizard_invariant_test.go`):
- Forward path: `installStepReview → installStepPublisherWarn → installStepConflict` with gate = `MOATGatePublisherWarn`.
- Skip path: `installStepReview → installStepConflict` with gate = `MOATGateProceed`.
- HardBlock path: banner at `installStepReview`, no modal.
- Back-path: `installStepPublisherWarn → installStepReview` on Esc.
- Zone-mark parity: every interactive element in `installStepPublisherWarn` view has matching `updateMouse` handler.

### 10.7 Session-scoped confirmation — existing API

Uses existing `installer.MarkPublisherConfirmed(session *moat.Session, registryURL, contentHash string)` which calls `moat.Session.MarkConfirmed(registryURL, contentHash)`. Key: `(registryURL, contentHash)`.

**Reason-change invalidation is NOT implemented in this bead.** Adding a reason-aware key requires extending `moat.Session`. Deferred to follow-up bead `syllago-2h86l` (repurposed from "publisher identity display refinement" — will add reason-aware session variant).

**Audit log:** Each Confirm writes to `cli/internal/audit` package: `{timestamp, item_name, content_hash, reason, issuer, user}`.

### 10.8 HardBlock banner

When gate returns `MOATGateHardBlock` at `installStepReview`:
- Banner text: `"Install blocked: registry revoked this content. Reason: <reason>. See [d] for details."`
- Position: top of review step, `dangerColor` background.
- Keys accepted:
  - `Esc` / `Enter` — close wizard.
  - `[d]` — open `TrustDetailsModel` for the item.
- Banner zone-marked; clicking opens `TrustDetailsModel`.
- Visible `[esc] Back` hint in footer.

### 10.9 Batch-aggregation rule (bulk install)

When user selects N items and multiple return `MOATGatePublisherWarn`, render ONE aggregated `PublisherWarnModel` listing all N items (scrollable list inside modal). One Confirm covers all; one Cancel aborts all.

Key bindings inside aggregated modal:
- `[Space]` — toggle individual item selection.
- `[a]` — select all.
- `[n]` — select none.
- `Enter` — confirm selected subset; `Esc` — cancel all.

On confirm: iterate selected items and call `installer.MarkPublisherConfirmed` for each.

Golden: 3-item batch publisher-warn modal at 80x30.

## 11. Error + edge cases

| Case | Behavior |
|------|----------|
| Catalog built without `EnrichFromMOATManifests` ever called | Items have zero-value fields → no badges, no drill-down additions. Non-regression. |
| MOAT manifest cache missing | `CheckRegistry` returns `StalenessStale` (fail-closed for nil lockfile entry) → skip enrich. No badges. |
| Manifest cache present but `ParseManifest` fails | Treat as no cache (log to `cat.Warnings`); skip enrich. |
| `signature.bundle` missing | Treat as no cache. Sync-time verification prevented this case from being the expected path. |
| `signature.bundle` fails verification at sync time | Manifest not persisted; cache left at previous valid state. Enrich-time read sees no new cache. |
| Item's `Registry` doesn't match any MOAT registry | Skipped by existing `registryName` filter. |
| Revocation `details_url` longer than panel width | Truncate with middle-ellipsis in Line 5. User presses `[d]` for full URL in `TrustDetailsModel`. |
| `RecallDetailsURL` empty | Banner omits URL segment. Trust-details modal shows `URL: (not provided)`. |
| Terminal width < 60 cols | Trust glyph column survives; name middle-ellipsis-truncates; `P` glyph drops; registry column drops. |
| Terminal width < 50 cols with publisher-warn modal pending | Inline error banner: `"Terminal too narrow — resize to ≥50 cols"`. |
| Terminal height < 20 rows | Metapanel collapses per §8. |
| `RecallIssuer` resolves empty | Fallback `"(publisher — identity not provided)"` or (unreachable per validate()) registry-side fallback. |
| Publisher-source rev with malicious literal `Subject = "publisher"` | Parenthetical fallback `"(publisher — identity not provided)"` disambiguates only when Subject is empty. A literal "publisher" Subject would display as "publisher" — acceptable edge case (attacker gains only self-identification, no privilege). |
| Stale-then-sync race (user presses `R` mid-sync) | Rescan reads on-disk state at that instant. If sync mid-write, `EnrichFromMOATManifests` appends warning and skips that registry until next rescan. |
| OSC 52 clipboard unsupported | `[y]` emits `TrustDetailsYankedMsg{Err: ...}`; toast shows `"Clipboard unavailable in this terminal"`. URL still visible in modal. |
| `TrustTier == TrustTierUnsigned` | Line 4 renders with description `"Unsigned (registry declares no attestation)"`. Row badge is `TrustBadgeNone` (no glyph) per AD-7 collapse. |
| `MOATGatePrivatePrompt` or `MOATGateTierBelowPolicy` returned by gate | Out of scope for 2c — wizard falls through to existing CLI-style error display (not polished). |

## 12. Testing strategy

### Unit tests
- `moat/sanitize_test.go`: one case per attack class (ANSI CSI, ANSI OSC, C0, C1, DEL, bidi overrides U+202A–U+202E and U+2066–U+2069, line separators U+2028/U+2029, tab preservation, newline collapse).
- `moat/enrich_test.go`: one test per new field (`PrivateRepo`, `RecallSource`, `RecallDetailsURL`, `RecallIssuer`), mixed-manifest test, sanitization-at-boundary test (manifest with ANSI escapes → assert sanitized on `ContentItem`), `RecallIssuer` fallback chain (Operator set; Operator empty + RegistrySigningProfile.Subject set; publisher with SigningProfile.Subject; publisher without SigningProfile).
- `moat/producer_test.go`: stale cache skipped, expired cache skipped, missing bundle skipped, path-traversal registry name rejected at config-load, factory pattern returns fresh catalog.
- `catalog/types_test.go`: zero-value `ContentItem` renders no trust surface (non-regression).
- `catalog/trust_test.go`: `TrustBadge.Glyph()` returns `"R"` for `TrustBadgeRecalled` (updated from `"✗"`).
- `tui/metapanel_test.go`: render truth table (Unknown public, Unsigned public, Signed public, DualAttested public, Recalled-registry public, Recalled-publisher public, DualAttested private, Recalled-registry private — 8 representative permutations).
- `tui/modals/publisher_warn_test.go`: modal routing (only on `MOATGatePublisherWarn`); dismiss paths (Esc, click-away, Cancel); key-bindings enumerated; width fallback (50–57 → shrink, <50 → inline banner); default focus = Cancel preserved across WindowSizeMsg.
- `tui/modals/trust_details_test.go`: `[y]` yank emits correct message; sanitization assertions; dismiss paths; width fallback.
- `tui/install_test.go`: wizard step enum includes `installStepPublisherWarn` between `installStepReview` and `installStepConflict`; `validateStep()` rejects entry without `MOATGatePublisherWarn` gate block.

### Integration tests
- `tui/moat_enrich_integration_test.go` (new): build real catalog from fixture registries (one MOAT with revocations, one git), call `moat.EnrichFromMOATManifests`, assert trust state per-item.
- `tui/install_integration_test.go` (existing): extend with publisher-warn modal path — inject fixture manifest with publisher-source revocation, verify wizard routes through `installStepPublisherWarn`.
- `tui/install_integration_test.go`: extend with HardBlock path — registry-source revocation, verify wizard shows error banner without modal.
- `tui/rescan_persistence_test.go` (new): press `R`, verify trust state survives across rescan via `catalogReadyMsg` atomic swap.

### Golden tests (bead 0kbbx)

Target: **11–13 goldens**. Canonical size: **80x30** for variants; full-app Library uses 3 sizes.

- **Full-app Library** @ 60x20, 80x30, 120x40 — mixed trust states row (3 goldens).
- **Metapanel variants @ 80x30:** Unknown, Unsigned, Verified-dual-attested, Verified-signed, Recalled-registry, Recalled-publisher, Recalled+Private combo (most crowded row) (7 goldens).
- **Publisher-warn modal @ 80x30:** default focus, focus-moved, batch-aggregation 3 items (3 goldens).
- **Trust-details modal @ 80x30:** Recalled-with-URL, Recalled-empty-URL (2 goldens).
- **Security regression @ 80x30:** sanitization golden — `ContentItem` with ANSI-laden reason + bidi issuer, assert render shows visible text only (1 golden).

### Security tests
- ANSI injection: item name, reason, issuer, URL each laden with CSI/OSC/bidi — assert rendered as visible text.
- Path traversal: `../escape` registry name rejected at config-load.
- Stale cache (25h old with 72h threshold) → `StalenessFresh` (within window). 73h old → `StalenessStale`. Verify via `CheckStaleness`.
- Expired manifest (past `Expires`) → `StalenessExpired` regardless of fetched_at.
- Bundle missing at enrich time → skipped, warning emitted.

## 13. Open questions — closed

| Question | Resolution |
|----------|-----------|
| Default focus on publisher-warn modal | Cancel (§10.5). |
| Narrow terminal drop-precedence | Trust glyph never drops; `P` and registry column drop first (§7). |
| Row glyph for Recalled | Bare `R` in red (§7); `TrustBadge.Glyph()` updated (§2). |
| Modal receivers | Pointer (§10.1). |
| Install wizard intercept point | New `installStepPublisherWarn` between `installStepReview` and `installStepConflict` (§10.6). |
| Modals package import cycle | `modals` imports `installer`; `tui` imports `modals`; no cycle. |
| Revocation `details_url` sanitization | `SanitizeForDisplay` at enrich boundary + shell-metachar strip on `[y]` yank (§10.2). |
| `RecallReason` sanitization | Added in `EnrichCatalog` (new behavior — currently unsanitized) (§5). |
| Private-repo glyph final | Bare `P` in row, `"Private"` in drill-down chip (§7, §8, §9). |
| Amber `!` vs red clash | Removed — text suffix `(registry)`/`(publisher)` (§8 Line 5). |
| Revocation banner border | `│` U+2502 light (§8). |
| Manifest cache location | `<cacheDir>/moat/registries/<sanitized-name>/{manifest.json,signature.bundle}` (§6). |
| Staleness threshold + formula | Uses existing `moat.DefaultStalenessThreshold` (72h) and `moat.CheckRegistry(lf, url, m, now)` primitive. No re-declaration. (§6). |
| `EnrichFromMOATManifests` call sites | Enumerated in §6; full grep during `lqas0`. |
| URL recovery for truncated `details_url` | `TrustDetailsModel` with `[y]` clipboard yank (§10.2). |
| Gate symbol names | `installer.PreInstallCheck`, `installer.GateBlock`, `.Decision`, `MOATGateProceed`/`MOATGateHardBlock`/`MOATGatePublisherWarn`/`MOATGatePrivatePrompt`/`MOATGateTierBelowPolicy` (§10). |
| Modal dispatch | Two new concrete modal fields on App, matching existing pattern (§10.4). |
| Rescan migration | `rescanCatalog()` returns `tea.Cmd` → `catalogReadyMsg` atomic swap (§6). |
| OSC 52 clipboard | Reuse `toast.go:copyAndDismiss` pattern (§10.2). |

**Remaining open question (deferred to follow-up bead `syllago-dwjcy`):** enrich-time re-verification of `signature.bundle`. Current posture: sync-time verification authoritative, enrich trusts filesystem.

---

## Summary of non-obvious calls

1. **Sanitization at enrich boundary, not render.** One chokepoint; consumers treat values as trusted for display. Existing `RecallReason` is currently unsanitized — 2c adds sanitization.
2. **AD-7 row glyph changed `✗` → bare `R` in red, AND `TrustBadge.Glyph()` updated.** CLI and TUI stay in sync.
3. **Publisher-warn modal reads from `installer.PreInstallCheck` gate decision (`GateBlock.Decision == MOATGatePublisherWarn`).** Prevents CLI/TUI drift.
4. **Staleness uses existing moat primitive.** `moat.CheckRegistry(lf, url, m, now)` with 72h threshold from `DefaultStalenessThreshold`. Spec doesn't re-declare.
5. **Default modal focus = Cancel.** Dangerous-action guard.
6. **Trust-details modal reusable infrastructure.** Lands in 2c for Recalled URL recovery; future beads reuse for attestation-mismatch.
7. **Session-scoped confirmation uses existing `(registryURL, contentHash)` key.** Reason-change invalidation deferred to follow-up bead (requires `moat.Session` API extension).
8. **Producer in `moat` package, not `catalog`.** `moat` already imports `catalog`; reverse would cycle.
9. **New install wizard step `installStepPublisherWarn`** inserted between `installStepReview` and `installStepConflict` — matches actual existing step names.
10. **Two new concrete modal fields on App** (`publisherWarn`, `trustDetails`), not a polymorphic interface — consistent with existing `modal editModal`, `confirm confirmModal`, etc.
11. **Field name is `PrivateRepo` not `IsPrivate`** — avoids method-name shadow with `ContentEntry.IsPrivate()`.
12. **`rescanCatalog()` migrates to `tea.Cmd → catalogReadyMsg`** atomic swap — fixes existing `tui-elm.md` rule #3 violation.
