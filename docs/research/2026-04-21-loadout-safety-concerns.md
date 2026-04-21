# Loadout Safety, Try/Keep Semantics, and Multi-Loadout Architecture

**Date:** 2026-04-21
**Status:** Research — for discussion
**Related:** [2026-04-20-mod-organizer-patterns-research.md](./2026-04-20-mod-organizer-patterns-research.md) (the MO2 comparison that surfaced these findings)
**Scope:** Internal syllago audit. No external comparison — this is about what the current `loadout` + `installer` + `snapshot` implementation does, what could go wrong, and what would unlock multi-loadout-per-provider.

---

## Why this doc exists

Three concerns came out of the MO2 comparison that aren't about borrowing patterns — they're concrete things I can see in syllago's current code:

1. Users can silently lose hand-edits to `settings.json` / MCP configs.
2. The `--try` vs `--keep` mental model is blurred by shared remove semantics.
3. Only one loadout can be active per provider, and that's an architectural property of the snapshot model, not a TUI limitation.

This doc lays out each, grounded in file references, with fix direction and scope honesty.

---

## Part 1: Hook / MCP config safety concerns

### 🔴 Concern A — User edits to `settings.json` between apply and remove are silently reverted

**Evidence.**

- `loadout/remove.go:59` calls `snapshot.Restore(snapshotDir, manifest)` which wholesale restores backed-up files.
- The snapshot was taken *before* apply at `loadout/apply.go:113`.
- Therefore remove reverts `settings.json` to its pre-apply state, regardless of anything the user changed while the loadout was active.

**User-visible failure modes.**

- User applies loadout `dev-tools` (adds 2 hooks). User later adds their own `PreToolUse` hook manually. User runs `syllago loadout remove`. Their hook is gone, no warning.
- User applies loadout. User manually changes `mcpServers.github.env.GITHUB_TOKEN` to update a rotated token. Remove → token reverts to whatever was there before apply. Authentication quietly breaks.
- User applies, edits `includeCoAuthoredBy: false` in settings. Remove → flipped back.

**Why the fix is cheap.** Syllago already has the pieces:

- `installer/installed.go:18` — each `InstalledHook` carries a `GroupHash` (SHA256 of the matcher-group JSON). This unambiguously identifies "our" entries.
- `installer/orphans.go:37-43` — already uses this hash to distinguish managed from unmanaged entries.
- `Source: "loadout:<name>"` tagging lets us scope removal to just one loadout's contributions.

**Fix direction.** Replace wholesale snapshot-restore with surgical removal for `--keep` loadouts:

1. On remove, load current `settings.json` (not the snapshot).
2. For each tracked `InstalledHook` with `Source == "loadout:<name>"`, find the matching entry in `settings.json.hooks.<event>[]` by `GroupHash` and splice it out.
3. For each tracked `InstalledMCP`, delete `mcpServers.<serverKey>` only if content hash matches (don't clobber user-modified servers).
4. Keep the pre-apply snapshot purely as a rollback net for *apply failure* (Step 5 in the Apply lifecycle) — not as the remove mechanism.

`--try` mode keeps its wholesale-restore behavior — that's a feature of try (see Part 2).

**Scope estimate.** 2-3 days. Touches `loadout/remove.go`, adds a `surgicalRemove` helper, extends tests in `loadout/remove_test.go` and `integration_test.go`. The content-hash check for MCP is new; hooks already have `GroupHash`.

**Severity: high.** Silent data loss on what users reasonably assume is a reversible operation.

---

### 🟡 Concern B — JSONC / comments in `settings.json` cause hard-stop errors

**Evidence.** `loadout/apply.go:464-476`:

```go
func readJSONFileOrEmpty(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) { return []byte("{}"), nil }
    if err != nil { return nil, err }
    if !json.Valid(data) {
        return nil, fmt.Errorf("%s contains invalid JSON; fix or delete the file before applying a loadout", filepath.Base(path))
    }
    return data, nil
}
```

**Failure mode.** Claude Code accepts JSONC in `settings.json` (inline `//` comments, trailing commas). A user who has ever added a comment like `// permissions for work machine` cannot apply a loadout — they hit a hard-stop error that blames their file.

**Frequency.** Hard to estimate without telemetry. Claude Code docs don't prominently advertise JSONC support, but power users do use it. Non-zero and unpleasant when it fires.

**Fix direction.** Either:

- **Strip comments + trailing commas before `json.Valid`.** Use a lightweight JSONC sanitizer (there are small Go libraries; we could also hand-roll ~30 lines since sjson output is pure JSON so we only need to handle input). Preserves user comments on re-write? Probably not; comments get lost. Acceptable trade-off.
- **Use a JSONC-aware parser for the validate step**, write output in strict JSON on rewrite, and emit a one-time warning: "comments in `settings.json` were removed during loadout apply."

**Scope estimate.** 1 day including tests + a warning message UX.

**Severity: medium.** Doesn't corrupt data, but blocks usage with a confusing error.

---

### 🟡 Concern C — No concurrent-run guard on apply/remove

**Evidence.**

- `writeJSONFileAtomic` (`loadout/apply.go:479-498`) is atomic *per call* via temp-file + rename.
- The read→sjson-mutate→write sequence in `applyHook` (`apply.go:228`) and `applyMCP` (`apply.go:297`) is *not* locked.
- `installer.SaveInstalled` (`installer/installed.go:76`) has the same pattern.

**Failure mode.** Two concurrent `syllago loadout apply` runs (or one apply racing one remove) can interleave. Process X reads settings.json at t=0, Process Y reads at t=1 (sees X's mutations? maybe), X writes at t=2, Y writes at t=3 using its stale base state → X's changes are lost.

**Likely triggers in practice.**

- `--try` SessionEnd hook fires while a user is mid-session running `syllago loadout remove` by hand.
- A TUI-driven apply overlapping with a CLI-driven apply from another terminal.
- Future: CI-driven applies from multiple jobs.

**Fix direction.** A lock file at `.syllago/.apply.lock` created with `os.O_EXCL`, held for the duration of apply/remove. On conflict, exit with "another syllago operation is in progress at PID N." Stale-lock handling via PID check (if PID N doesn't exist, reclaim).

**Scope estimate.** 1 day including tests for the stale-lock reclaim path.

**Severity: medium-low.** Low probability today, rises with automation.

---

### 🟢 Already well-handled

Worth noting so we don't re-litigate these:

- Pre-existing user hooks are preserved on apply. `appendHookEntry` (`apply.go:277-294`) appends via `sjson.SetRawBytes` at `hooks.<event>.-1` — existing array entries aren't touched.
- Atomic single-file writes prevent partial-write corruption (`writeJSONFileAtomic`, random hex suffix, then rename).
- Apply-failure rollback is implemented (`apply.go:122-134`) — snapshot restore + partial-symlink cleanup on any error after snapshot creation.
- Orphan detection for hooks/MCP already exists (`installer/orphans.go:24`) even if unsurfaced in UI.

---

## Part 2: `--try` vs `--keep` — the murkiness, clarified

### What the modes actually do

Reading `loadout/apply.go:42-47` and the injection in `apply.go:136-141`:

| Mode | Apply-time behavior | Remove-time behavior (today) | Auto-revert |
|---|---|---|---|
| `--preview` | Computes `[]PlannedAction`, returns without touching disk. | N/A | N/A |
| `--try` | Applies all changes **and injects a `SessionEnd` hook** that runs `syllago loadout remove --auto`. | Wholesale snapshot-restore. | Yes — on session end. |
| `--keep` | Applies all changes, no SessionEnd hook. | Wholesale snapshot-restore (same mechanism as try — **this is Concern A**). | No — user-driven remove only. |

### Why the distinction feels slippery

The visible difference today is only **when** remove fires (SessionEnd vs. user command). The **remove mechanic** is identical: wholesale snapshot-restore. So users reasonably ask "what's the difference — it all ends up restored either way?" The distinction lives in user intent, not in behavior.

### What the distinction should be

After fixing Concern A, try and keep diverge meaningfully:

- **`--try`** = **wholesale snapshot-restore on revert.** The promise is "put everything back exactly how it was." Correct for a demo-and-forget flow. One try active at a time (SessionEnd hook doesn't compose).
- **`--keep`** = **surgical removal on revert.** The promise is "this loadout is no longer contributing. Anything else you or other loadouts did stays." Correct for long-lived installs.

### Suggested user-facing framing

- **try** = "I'm demoing this. End my session → it's gone, guaranteed."
- **keep** = "Install this. I'll remove it later. My other customizations stay put."

The mental model gets crisp once the remove semantics stop being identical.

---

## Part 3: Multiple active loadouts per provider

### What blocks it today

`loadout/remove.go:46` calls `snapshot.Load(opts.ProjectRoot)` which returns **the** most recent snapshot — singular. The entire remove flow assumes one active loadout at a time.

Concretely:

```
apply A → snapshotA captures pre-A state
apply B → snapshotB captures post-A-pre-B state
remove A now?  snapshot.Load returns snapshotB.
               Restoring snapshotB reverts B, not A.
               The "pre-A" state exists in snapshotA's dir but Load doesn't know about it.
```

**Loadouts are not composable under snapshot-restore.**

### What's already in place that helps

You already have ~80% of the required infrastructure:

- **Per-item lineage:** every `InstalledHook`, `InstalledMCP`, `InstalledSymlink` carries `Source: "loadout:<name>"` (`installer/installed.go:15-48`).
- **Unambiguous entry identity:** `GroupHash` on hooks, content-hash fields on MCP and symlinks.
- **Additive data model:** `Installed{Hooks, MCP, Symlinks}` are slices — appending another loadout's entries doesn't conflict with earlier entries.

What's *not* yet in place:

1. **An "active loadouts" set** instead of an implicit "most recent snapshot." Today: one snapshot, implicitly = one active loadout. Needed: an explicit ledger, e.g., `.syllago/active-loadouts.json` = `[{name, appliedAt, mode, snapshotDir}, …]`.
2. **Per-loadout snapshots stored by name, not just timestamp.** `.syllago/snapshots/by-name/<loadout-name>/` with the existing timestamp layout as a secondary axis. Apply creates snapshot-per-loadout; remove consults the specific one.
3. **Conflict resolution when two loadouts contribute the same item.** Simplest viable rule: *explicit error on apply* — "loadout B wants to install skill `foo`, but loadout A already installed it. Remove A first, or rename one item." Matches the current `error-conflict` treatment in `preview.go:96-130`.
4. **Try-mode composition rule.** SessionEnd-based auto-revert doesn't compose naturally (two tries would race on the session-end hook). Practical rule: one `--try` active at a time, stacked on top of any number of `--keep`s. End of session → revert the try layer only, keeps untouched.

### Solution sketch

`★ State model change.` Treat `installed.json` + the per-loadout snapshots as the authoritative state. The "active state on disk" is the composition:

```
[unmanaged user state]
  + layer(keep-A)
  + layer(keep-B)
  + layer(try-X)   ← at most one
```

- **Apply** = compute what this layer contributes, verify no collisions against active layers, append to `Installed`, create a named snapshot for apply-failure rollback only.
- **Remove** = locate the layer in the active set, surgically remove its contributions from `Installed` + the on-disk state (symlinks deleted, hook/MCP entries spliced by hash), delete its snapshot. Other layers are untouched.
- **Try auto-revert** on SessionEnd = remove the try layer specifically.

This is effectively what Concern A's fix implements, generalized from 1 layer to N.

### Scope honesty

This is **not** a one-afternoon change. It touches:

- `loadout/apply.go` — active-loadouts ledger, per-name snapshots, collision check against active set.
- `loadout/remove.go` — surgical layer-removal (shared with Concern A fix).
- `snapshot/snapshot.go` — storage layout changes, or additional indexing.
- `loadout/stale.go` — "stale" semantics may need to be per-loadout.
- TUI — active-loadout surface becomes active-loadouts (plural).
- Tests — integration tests for layered scenarios (A only, B only, A+B, A+B remove A, A+B+try X, etc.).

Estimate: **~1 week of focused work**, assuming Concern A's surgical remove lands first (it's a strict subset).

### Should we do it?

The honest answer is *depends on whether users want it*. Users might want it for:

- Stacking a personal loadout + a team-provided baseline loadout.
- Keeping an "always-on" security loadout active while trying experimental ones.
- Per-project + per-global layering.

These are all real workflows, but none have been explicitly requested that I've seen. Worth gauging before committing a week.

**If we do commit to this:** the order is Concern A → multi-loadout, because A is a strict prerequisite. A standalone makes sense even if multi-loadout is deferred.

---

## Consolidated recommendation (priority order)

| # | Item | Scope | Value | Dependency |
|---|---|---|---|---|
| 1 | Fix Concern B (JSONC tolerance) | 1 day | Unblocks users with commented settings files | None |
| 2 | Fix Concern A (surgical remove for `--keep`) | 2-3 days | Eliminates silent user-edit loss; clarifies try/keep distinction | None |
| 3 | Fix Concern C (concurrency lock) | 1 day | Paranoia insurance, rises in value with automation | None |
| 4 | Multi-loadout architecture | ~1 week | Resolves "one loadout per provider" limitation | #2 must land first |

`★ Key insight ─────────────────────────────────`
#2 and #4 share the same underlying change: **stop using snapshot-restore as the remove mechanism.** The snapshot stays valuable for *apply-failure* rollback, but not for user-initiated remove. Fixing #2 lays the foundation; #4 generalizes it from one layer to many.
`─────────────────────────────────────────────────`

---

## Discussion questions for tomorrow

1. **Is Concern A worth a hotfix-track release, or does it wait in a normal sprint?** The severity (silent data loss) argues for faster; the low probability (users who hand-edit between apply and remove) argues slower.
2. **Is multi-loadout-per-provider on the roadmap or out of scope?** If not on the roadmap, #2 is still worth doing standalone for the safety fix. If on the roadmap, let's do #2 with #4's shape in mind so it's strictly additive.
3. **JSONC: strip silently, strip with warning, or fail with a better error?** I'd vote strip-with-warning ("comments in your settings file were removed; check diff").
4. **Concurrent-run lock: add now or wait for a real race report?** My vote is now — it's cheap and the race *will* happen once automation/TUI multi-process usage grows.

---

## Appendix: files read for this audit

- `cli/internal/loadout/apply.go` — full apply lifecycle, JSONC rejection, atomic write, hook/MCP merge
- `cli/internal/loadout/remove.go` — snapshot-based wholesale restore; `cleanInstalledEntries` already scoped by source
- `cli/internal/loadout/preview.go` — `error-conflict` treatment for existing targets
- `cli/internal/loadout/stale.go` — try-mode 24h staleness
- `cli/internal/installer/installed.go` — `Installed{}` layer ledger, `GroupHash`, `Source`
- `cli/internal/installer/orphans.go` — existing orphan detection pattern (hooks/MCP only)
- `cli/internal/installer/conflict.go` — cross-provider path-collision (distinct from loadout-loadout conflicts)
- `cli/internal/snapshot/snapshot.go` — timestamp-indexed snapshot storage
