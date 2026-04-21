# Mod Organizer 2 → Syllago: Transferable Patterns

**Date:** 2026-04-20
**Status:** Research — for discussion
**Revision:** 2 (corrected after reading syllago internals in `cli/internal/{loadout,installer,catalog,snapshot}/`)
**Source:** Deep dive on Mod Organizer 2 codebase at `~/.local/src/modorganizer` (355 source files), cross-referenced against syllago's actual implementation.

---

## Why this comparison is useful

The surface domains are unrelated, but the core problem is the same:

> **Manage a large, user-curated collection of third-party content that must land in specific locations on some other app's disk, support multiple named configurations, and make conflicts between items visible and resolvable.**

MO2 has been solving this since ~2014 for Bethesda games. Several things syllago is skirting today (or hasn't yet hit at scale) are solved problems there.

**Important correction from Revision 1.** My first pass made recommendations that assumed syllago hadn't solved problems it had already solved. This revision is grounded in the actual code.

---

## What syllago already has (the table I wish I'd built first)

| MO2 pattern | Syllago status | Evidence |
|---|---|---|
| Manifest → reconcile split (compute actions, then apply) | ✅ **Already exists** | `loadout/apply.go:52` — `Apply` runs Resolve → Validate → **Preview** → Snapshot → apply. `Mode = "preview"` returns actions without touching disk. |
| Atomic rollback on partial failure | ✅ **Already exists** | `loadout/apply.go:122-134` — on apply error, `snapshot.Restore` + symlink cleanup + snapshot.Delete. All-or-nothing. |
| Dry-run / preview mode | ✅ **Already exists** | Same as above; `ApplyOptions.Mode = "preview"` returns the full `[]PlannedAction` list. |
| Source lineage tracking | ✅ **Already exists** | `installer/installed.go:15-48` — every `InstalledHook`/`InstalledMCP`/`InstalledSymlink` carries `Source: "loadout:<name>"` or `"export"`. |
| Cross-provider conflict detection | ✅ **Already exists, narrow** | `installer/conflict.go:74` — catches when one provider's `InstallDir` collides with another's `GlobalSharedReadPaths`. Three resolutions: shared-only, own-dirs-only, all. |
| Target-already-exists conflict | ✅ **Already exists** | `loadout/preview.go:51` — `previewSymlink` returns `error-conflict` if target exists and isn't our symlink. |
| Overridden items tracked by precedence | ✅ **Already exists** | `catalog/precedence.go:23` — `applyPrecedence` dedups by (name, type), keeps winner in `Items`, losers in `Overridden`. `OverridesFor(name, type)` surfaces them. |
| Stale snapshot detection (auto-cleanup) | ✅ **Already exists** | `loadout/stale.go:29` — flags `--try` snapshots older than 24h. |
| Orphan detection (state present on disk but not in installed.json) | ⚠️ **Partial** | `installer/orphans.go:24` — covers hooks + MCP in settings.json. **No equivalent for symlinked content types.** |
| User-controllable priority per loadout | ❌ **Missing by design** | Syllago uses a fixed precedence hierarchy (Library 0 > Shared 1 > Registry 2 > Built-in 3, in `precedence.go:6-17`), not MO2-style per-loadout integer priority. |
| Visible "shadowed" items in UI | ❌ **Data tracked, not shown** | `Catalog.Overridden` exists; no TUI surface. |
| Promote-orphan-to-item flow | ❌ **Missing** | Orphans are detected (for hooks/MCP) but not actionable in TUI. |
| Separator / divider items for organizing lists | ❌ **Missing** | No equivalent of `ModInfoSeparator`. |

**Consequence.** The two recommendations in my first pass that I was most excited about (the "manifest/reconcile split" and atomic rollback) were already done. The Overwrite pattern was partly done — and the gap there is the real, useful recommendation.

---

## Revised recommendations

### 1. Extend orphan detection to symlinked content, and surface orphans in the TUI

**🟢 Strongest recommendation — a small, concrete gap.**

**What exists.** `CheckOrphanedMerges` (`installer/orphans.go:24-111`) reads each detected provider's `settings.json` and flags hook entries and MCP server entries that aren't recorded in `installed.json`. This is exactly MO2's "Overwrite" concept: state present in the managed directory that syllago didn't place there, so it must be user- or tool-generated.

**What's missing.**

1. **No orphan detection for symlinked content types** (Skills, Agents, Rules, Commands). If a user or another tool drops a skill into `.claude/skills/my-skill/`, syllago has no record of it. `Installed.Symlinks` tracks what we created; nothing walks the install dir to find what we *didn't* create.
2. **No TUI surface** for orphans — neither the existing hook/MCP orphan detection nor the (hypothetical) symlink version shows up in the Library or metapanel.
3. **No adoption action.** Orphans today are reportable but not actionable. MO2's Overwrite has a clear workflow: promote-to-mod, ignore, or discard.

**Why this matters for syllago.** The publisher-warn install gate (commit `adb6b69`) already establishes that syllago cares about pre-existing unmanaged content at install time. Post-install drift — hand-edits to an installed skill, files dropped by a teammate's tool, AI-generated configs, `.claude/settings.local.json` — is invisible today. Given MOAT's direction and the general trust-surfacing arc, closing this gap compounds with work already in flight.

**Scope sketch.**

1. **Detection:** add `CheckOrphanedSymlinks(projectRoot, providers)` paralleling `CheckOrphanedMerges`. For each detected provider, walk `InstallDir(home, ct)` for each universal + provider-specific type, and flag anything not present as an entry in `Installed.Symlinks` (matched by path).
2. **Catalog surface:** extend `Catalog` with an `Orphans []OrphanItem` field, populated by a scan step. Show them as a virtual source in Library (alongside Library / Shared / Registry). Reuse the metapanel.
3. **Adoption actions:** `[a] Adopt` creates a minimal `.syllago.yaml` and moves/copies the item into the Library. `[i] Ignore` records a path in config to suppress future detection. `[d] Delete` removes the file.

**Non-goal.** Don't treat orphans as "conflicts" in the installer sense — they're pre-existing state, not competing installs.

---

### 2. Surface `Catalog.Overridden` in the Library UI

**🟢 Low cost, pure UI work — the data is already tracked.**

**What exists.** `applyPrecedence` (`catalog/precedence.go:23-61`) dedups same-name same-type items and keeps shadowed ones in `Catalog.Overridden`. `OverridesFor(name, type)` retrieves them. The precedence hierarchy is:

```
0 Library    (your ~/.syllago/content/)
1 Shared     (repo-level content, no registry)
2 Registry   (git-cloned content)
3 Built-in   (tagged "builtin")
```

**What's missing.** The TUI doesn't display this. A user with a `security-rules` item in their Library and a same-named item from a registry has no way to see that the registry version is being suppressed. Fine when it's the user's choice; confusing when they forgot it exists.

**MO2 inspiration.** The "redundant" conflict type in MO2 (`modinfowithconflictinfo.h`) says "this mod is entirely covered by higher-priority mods." Syllago already has the same concept — it just doesn't show it.

**Scope sketch.**

- In the metapanel for any Library item: a "Shadows" line — `Shadows: 2 items → registry-a/security-rules, registry-b/security-rules`.
- For items in `Overridden`, a reverse badge — shown only when a user filters to "show overridden" or inspects conflicts. Default hidden (don't clutter the main list).
- Optional: a `[o]` key to toggle "show overridden items" in the Library filter row.

**Why this is separate from §1.** Orphans (§1) are "state we didn't put here." Overrides (§2) are "items we know about but suppressed." Different concepts, worth surfacing differently.

---

### 3. Separator item type for organizing long loadouts

**🟡 Nice-to-have, trivial.**

**What exists.** Loadouts today are flat lists: `rules: [...], skills: [...], hooks: [...]`, etc. As loadouts grow past ~15 items, readability suffers.

**MO2 inspiration.** `ModInfoSeparator` is a zero-content entry that acts purely as a section header in the mod list (`--- Graphics mods ---`). No files, no activity, just structure.

**Scope sketch.**

- Extend `ItemRef` (`loadout/manifest.go:14`) to support a `separator: "Security rules"` variant via the existing `UnmarshalYAML` polymorphism.
- Skip separators in resolution (they produce no `ResolvedRef`).
- Render them as dividers in loadout preview output and the TUI loadout view.

**Why bother.** Curated enterprise loadouts with 30+ items will exist. Pre-building the affordance is cheap; retrofitting it later is awkward.

---

### 4. User-reorderable priority — **not recommended, retracted**

**🔴 Do not pursue.**

My original §1.2 framed this as an obvious win. After reading `catalog/precedence.go`: syllago's precedence is a **fixed hierarchy by source type**, not per-item integer priority. This is a deliberate design — the arbitration is predictable without user intervention.

MO2's per-profile integer priority solves a problem syllago doesn't have: thousands of mods where conflicts are the norm and there's no principled tiebreaker. Syllago's source-type hierarchy is a principled tiebreaker. The gap isn't the arbitration rule; it's the visibility of its outcome (§2).

Changing to user-reorderable priority would be a large refactor that solves a problem we don't appear to have. Don't do it. Instead do §2.

---

### 5. Manifest/reconcile split and atomic rollback — **retracted, already done**

My original §1.4 recommended this as the architectural refactor worth doing. It's already the architecture. `loadout/apply.go:40` has the comment:

```
// The sequence is: Resolve -> Validate -> Preview -> Snapshot -> Apply items -> Record.
// If any step after snapshot creation fails, the snapshot is restored (all-or-nothing).
```

Previewing before applying, snapshotting before mutating, restoring on error — all present. I was recommending work we'd already done.

---

## Smaller patterns — reassessed against the real code

| Pattern | Status | Verdict |
|---|---|---|
| Sidecar metadata (`meta.ini` ↔ `.syllago.yaml`) | ✅ Already done | MO2 confirms the instinct. Nothing to do. |
| Explicit refresh, no fs watchers | ✅ Already done | `R` key in TUI. Don't add watchers. |
| Atomic single-file JSON writes | ✅ Already done | `writeJSONFileAtomic` in `apply.go:479` uses random-suffix temp + rename. |
| Batched writes across an apply (debounce) | ⚠️ Not done | Each hook/MCP merge in `applyActions` rewrites settings.json separately. Could batch into one read-modify-write per file. **Small, real perf win.** |
| Pluggable content-package formats (FOMOD-style) | ❌ Not relevant yet | Don't pre-build. Revisit when the second format request lands. |
| `syllago://install?…` protocol handler | ❌ Absent | Becomes natural once MOAT meta-registry is live and we have a web surface to link from. |
| Pre-install veto hooks (enterprise policy) | ❌ Absent | Revisit when enterprise deployments materialize. |

---

## Discussion questions for tomorrow

In rough "decide this first" order:

1. **Orphan detection for symlinked content + TUI surface (§1).** Biggest new surface. Scope options:
   - (a) Detection only, CLI `syllago status --orphans` output
   - (b) Detection + TUI badges in Library
   - (c) Full (a) + (b) + adoption flow

2. **Surface `Overridden` in the Library metapanel (§2).** Pure UI, no data-model risk. In or out?

3. **Separator item type (§3).** Now, later, or never?

4. **Batch writes across an apply (§smaller).** Currently each merge rewrites settings.json. Fix opportunistically or wait for a perf complaint?

5. **What else did I miss?** I read `cli/internal/{loadout,installer,catalog,snapshot}/` closely. If there are invariants in `metadata`, `provider`, `moat`, or `tui/` that change the picture, flag them before I go further.

---

## One-line recommendation (revised)

> The single recommendation that survives a close read of the actual code is **§1: extend orphan detection to symlinked content and make orphans first-class citizens in the Library TUI.** It extends an already-present pattern (`installer/orphans.go`) to where it's missing, and it compounds with the MOAT trust-surfacing work already in flight. Everything else from my first pass was either already solved (manifest/reconcile, rollback, preview) or had to be reframed around syllago's precedence model instead of MO2's priority model (§2), or is a minor nicety (§3, batch writes).

---

## Appendix: what I read in syllago to write this revision

- `cli/internal/loadout/manifest.go` — `Manifest`, `ItemRef`, the flat-per-type content structure.
- `cli/internal/loadout/apply.go` — the six-step apply lifecycle with snapshot-based rollback.
- `cli/internal/loadout/preview.go` — `previewSymlink`, `previewHook`, `previewMCP`; encodes conflicts as actions, not errors.
- `cli/internal/loadout/remove.go` — snapshot-restore + targeted `installed.json` cleanup by source.
- `cli/internal/loadout/stale.go` — 24h stale-snapshot detection for `--try` mode.
- `cli/internal/installer/conflict.go` — cross-provider install-path collision detection.
- `cli/internal/installer/installed.go` — `Installed{Hooks,MCP,Symlinks}` with `Source` field.
- `cli/internal/installer/orphans.go` — **the existing Overwrite-analog, scoped to JSON merges only.**
- `cli/internal/catalog/types.go` — `Catalog` with `Items` + `Overridden`; `ContentItem` with MOAT trust fields.
- `cli/internal/catalog/precedence.go` — fixed-hierarchy precedence; `Overridden` tracked but unsurfaced.
- `cli/internal/snapshot/snapshot.go` — snapshot manifest + symlink records + backed-up files.
