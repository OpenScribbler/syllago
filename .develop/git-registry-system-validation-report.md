# Git Registry System — Design/Plan Parity Validation

**Design doc:** `docs/plans/2026-02-23-git-registry-system-design.md`
**Implementation plan:** `docs/plans/2026-02-23-git-registry-system-implementation.md`
**Validated:** 2026-02-23

---

## Validation Report

### Covered (22 requirements → 18 tasks)

**Design Steps → Tasks:**
- Step 1: Config schema (`Registry` struct, `Registries []Registry`, `omitempty`) → Task 1
- Step 2: Registry package (`CacheDir`, `CloneDir`, `Clone`, `Sync`, `SyncAll`, `Remove`, `IsCloned`, `NameFromURL`, `checkGit`) → Task 2
- Step 2 (tests): Pure function unit tests for `NameFromURL` → Task 3
- Step 3: CLI `registry add/remove/list/sync` subcommands + security warnings → Task 8
- Step 4: `ContentItem.Registry` field, `RegistrySource` struct, `ByRegistry`, `CountRegistry` → Task 4
- Step 4 (scanner): `ScanWithRegistries` function → Task 5
- Step 4 (scanner): `ScanRegistriesOnly` function → Task 16 (added to scanner.go)
- Step 5: Wire `ScanWithRegistries` into `runTUI` + auto-sync + pass `regSources` to App → Task 7
- Step 6a: Sidebar "Registries" entry + count + `isRegistriesSelected()` + `totalItems()` update → Task 11
- Step 6b: Landing page Registries card + `welcomeConfigMap` entry → Task 12
- Step 6c: `registriesModel` screen (list, navigate, empty state, CLI hints) → Task 13
- Step 6d: Wire `screenRegistries` into App navigation (enter, esc, mouse, window resize, view, breadcrumb) → Task 14
- Step 7a: `[registry-name]` tag in items view → Task 9
- Step 7b: `[registry-name]` tag in detail view breadcrumb + `Registry:` metadata field → Task 10
- Step 8: `nesco registry items [name] [--type] [--json]` subcommand → Task 16
- Step 9: Installer `CheckStatus` extended with `registryPaths...` variadic + `IsSymlinkedToAny` → Task 6
- Step 10: `registryAutoSync` preference in Settings screen (toggle + description) → Task 15
- Step 11: README security disclaimer → Task 18 (added by this validation)

**Key Decisions → Tasks:**
- Per-project registries in `.nesco/config.json` → Task 1 (Registry struct + Registries field)
- Global cache at `~/.nesco/registries/<name>/` → Task 2 (CacheDir/CloneDir)
- Shell out to `git` CLI → Task 2 (exec.Command, checkGit)
- `ScanWithRegistries()` opt-in → Tasks 5, 7
- Registry items excluded from export → Task 7 (explicitly noted as NOT wired into export.go)
- Name derivation from URL → Task 2 (NameFromURL), Task 8 (add command)
- Security warnings on add → Task 8 (box on first, brief on subsequent)
- Install status detection extended → Task 6 (IsSymlinkedToAny, variadic CheckStatus)
- Name collisions allowed, `[registry-name]` tag → Tasks 9, 10 (display differentiation)
- Auto-sync explicit by default, `registryAutoSync` config, 5s timeout → Tasks 7, 15
- Registry items mixed into normal content views → Tasks 5, 7, 11 (sidebar counts include registry items)
- Sidebar counts include registry items → Task 11 (registryCount field, count displayed)
- Private repos handled by git transparently → Task 2 (no special auth code; git handles it)

**Error handling table → Tasks:**
- `git` not on PATH → Task 2 (`checkGit()` in registry package)
- Clone fails → Task 2 (`Clone()` cleans up partial clone, returns error; Task 8 does not save to config on error)
- Pull fails (dirty tree) → Task 2 (`Sync()` returns error with hint); fixed hint wording in this validation
- Registry has no content dirs → Task 8 (warning on add, still saves)
- Duplicate registry name → Task 8 (error before clone)
- Config missing `registries` key → Task 1 (`omitempty`, nil slice on load)

**Design success criteria → Tasks:**
1. `nesco registry add` clones and saves → Task 8
2. `nesco registry list` shows registries with sync status → Task 8
3. `nesco registry sync` pulls all → Task 8
4. `nesco registry items <name>` lists items → Task 16
5. `nesco registry remove` removes config + deletes clone → Task 8
6. TUI shows Registries in sidebar + landing page → Tasks 11, 12
7. TUI registries screen lists registries with item counts → Task 13
8. Entering a registry shows its items → Task 14
9. Registry items show `[registry-name]` tag → Tasks 9, 10
10. Existing configs without `registries` load without error → Task 1
11. `nesco export` unaffected → Task 7 (explicitly excluded)

**TBD/TODO/mock data check:** None found.

---

### Gaps Found (3 issues — all fixed)

**1. Missing task: Step 11 (README security disclaimer) had no implementing task.**

The design doc explicitly lists a "Security" section in the README as Step 11, and the Key Decisions table documents the security model. No task in the original plan covered this.

Fix: Added Task 18 — "Add security disclaimer to README" with the exact content specified in the design. Added to Execution Order as a dependency-free task.

**2. Misleading error hint: `Sync()` hinted at a `--force` flag that was never defined.**

The `registry.Sync()` function in Task 2 contained:
```
(Hint: try nesco registry sync --force %s to reset)
```
But `registrySyncCmd` in Task 8 defines no `--force` flag and no force logic. The design's error handling table describes this behavior but the design did not explicitly require a `--force` flag — the hint was an over-reach. Pointing users at a nonexistent flag is worse than no hint.

Fix: Replaced the hint with a concrete recoverable action: delete the clone directory and re-run `nesco registry add`.

**3. Implementation confusion: Duplicate `init()` function across Tasks 8 and 16.**

Task 8 ends with an `init()` that registers 4 subcommands. Task 16 provides a replacement `init()` that registers 5 subcommands (adding `registryItemsCmd`). Since both code blocks target the same file (`registry_cmd.go`), an implementer following the plan in order would create a compile error (duplicate `init()` in the same package file, which is actually legal in Go — multiple `init()` functions are allowed, but registering `rootCmd.AddCommand(registryCmd)` twice would panic at runtime).

Fix: Added an explicit note at the end of Task 8 instructing implementers to delete Task 8's `init()` block when implementing Task 16.

---

### Action Required

All 3 gaps have been fixed in the implementation plan. No further action required before implementation begins.

Fixes applied to `docs/plans/2026-02-23-git-registry-system-implementation.md`:
1. Task 18 added (lines inserted before Task 17)
2. `Sync()` error hint corrected (Task 2, line ~168)
3. Task 8 note added about `init()` replacement in Task 16

Attempt 1/5

---

✅ PASSED
