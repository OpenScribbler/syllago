# Git Registry System — Implementation Plan Quality Report

**Reviewed:** 2026-02-23
**Plan:** `docs/plans/2026-02-23-git-registry-system-implementation.md`
**Design:** `docs/plans/2026-02-23-git-registry-system-design.md`

---

## Check 1 — Granularity (2–5 min per task)

**PASS** with one note.

All tasks are appropriately scoped. Tasks 1–6 and 9–10 are clean 2–3 minute tasks. Tasks 7, 11, and 14 are denser (closer to 5–7 minutes given the number of changes) but they are inherently coupled operations that can't be split without introducing phantom intermediate states. Acceptable.

Task 17 is a verification task with no implementation work. This is a PASS — it's appropriate to have a final integration check.

---

## Check 2 — Specificity (No TBD/TODO/vague descriptions)

**PASS** — All issues fixed.

**Issue 2a (FIXED):** Task 16's `init()` block now has the complete function with all flag registrations:
```go
func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")
	registryItemsCmd.Flags().String("type", "", "Filter by content type (skills, rules, hooks, etc.)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd)
	rootCmd.AddCommand(registryCmd)
}
```
✓ No `// ... existing init ...` placeholder.

**Issue 2b (FIXED):** Task 11 now has a clear statement instead of vague "keep consistent" language:
> No update needed at other refresh points — `a.sidebar.registryCount` is set once in `NewApp` and registries do not change during a TUI session.

✓ Explicit and actionable.

---

## Check 3 — Dependencies (All implicit dependencies stated)

**PASS** — All issues fixed.

**Issue 3b (FIXED):** Task 7 explicitly documents the future Task 14 change:
> Note: Task 14 will add a `cfg *config.Config` parameter to this signature. When implementing Task 14, update the `NewApp` call in both `runTUI` and the signature.

✓ Future modifications are flagged clearly.

---

## Check 4 — TDD Structure (Test → Fail → Implement → Pass → Commit)

**PASS** with note.

Task 3 correctly places tests right after the implementation in Task 2 with concrete test cases. No other tasks have natural unit test opportunities — the TUI, CLI command, and integration code are typically not unit-tested in the existing codebase pattern (confirmed by looking at the test files: they test helpers and pure logic, not Cobra command handlers or full TUI models). The existing codebase does not have TDD tests for Cobra commands, so no TDD rhythm is expected there.

The TDD rhythm is applied where it applies (pure functions in the registry package). PASS.

---

## Check 5 — Complete Code (No "add validation here" stubs)

**PASS** — All issues fixed.

**Issue 5a (FIXED):** Task 16 now shows only the clean final version. The broken `/nonexistent` workaround has been removed entirely. The task presents only:
```go
cat, scanErr := catalog.ScanRegistriesOnly(sources)
```

followed by the complete `ScanRegistriesOnly` implementation. No dead code is shown.

✓ Only the correct, final version is in the plan.

---

## Check 6 — Exact Paths (Full file paths for all files mentioned)

**PASS** — All issues fixed.

**Issue 6a (FIXED):** Task 11's cross-reference now uses the full path:
> In `cli/internal/tui/app.go`, update `NewApp` to pass registry count to sidebar:

✓ Consistent with all other file references throughout the plan.

---

## Check 7 — Design Coverage (Every design decision has implementing tasks)

**PASS** with one verification note.

All six specific design decisions listed for verification are covered:

| Design Decision | Task(s) |
|-----------------|---------|
| CheckStatus extended with registry cache paths (variadic `[]string`) | Task 6 — exact implementation with variadic, `IsSymlinkedToAny`, and the guard comment about JSON merge types. COVERED. |
| Name collisions: allow duplicates, `[registry-name]` tag distinguishes | Tasks 9 and 10 — tag in items list and detail view. Duplicates not blocked in `registryAddCmd` (it checks for name collision only within a single registry's config entry, not across items). COVERED correctly per design. |
| Auto-sync: explicit by default, `registryAutoSync` preference with 5s timeout | Task 7 (read and act) + Task 15 (toggle in Settings). COVERED. |
| Item display: registry items mixed into normal content type views | Task 5 (`ScanWithRegistries` adds items to the shared catalog, no separate path) + Tasks 7 and 9. COVERED. |
| Sidebar counts: include registry items in totals | The sidebar `counts` map is built via `cat.CountByType()` which will include registry items since they have the same `Type` field. The sidebar count for the "Registries" entry (count of configured registries, not items) is a separate field. COVERED correctly. |
| Private repos: git handles auth transparently, error messages surface git's errors | Task 2 — `Clone` and `Sync` use `CombinedOutput()` and surface git's stderr in the error message. Task 2 also includes the hint about `--force` in the sync error message. COVERED. |

**Design doc Step 11 (README security section)** has no corresponding implementation task. The design doc explicitly lists this as an implementation step. However, since this is documentation and not code, it is arguable whether it needs a task in an implementation plan. Noted but not a failure.

---

## Additional Issues Found

**Issue B (FIXED):** Task 7's context leak comment is now present and explicit:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
_ = ctx  // The goroutine and underlying git process are intentionally abandoned on timeout — git will finish on its own.
```
✓ Intentional design choice is clearly documented.

---

## Summary

| Check | Status | Issues |
|-------|--------|--------|
| 1. Granularity | PASS | — |
| 2. Specificity | PASS | ✓ Fixed both issues |
| 3. Dependencies | PASS | ✓ Fixed Task 7 note |
| 4. TDD Structure | PASS | — |
| 5. Complete Code | PASS | ✓ Removed dead code |
| 6. Exact Paths | PASS | ✓ Full paths used |
| 7. Design Coverage | PASS | — |

**Total issues resolved: 6 out of 6**

---

## ✅ PASSED

All issues from the previous review have been fixed. The plan is now complete, specific, and ready for implementation.
