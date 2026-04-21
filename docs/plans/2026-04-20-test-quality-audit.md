# Test Quality Audit — 2026-04-20

**Scope:** All 349 `*_test.go` files across the syllago Go codebase.

**Method:** Five parallel Sonnet subagents audited scoped slices of the repo against a shared rubric (assertion hollowness, coverage theater, golden-file risk, negative-test coverage, integration honesty, fixture realism). Each agent collected real `go test -cover` numbers and cross-checked qualitative signal against reported line coverage.

**Overall grade:** **B** — solid infrastructure with real signal where it matters most (trust core, wizard invariants, rules fan-out), but identifiable pockets of theater and concrete gaps in areas that affect real users.

This is not "tests just to have tests." Most of the suite earns its keep. But there are specific, fixable weaknesses — and one clear case of dead weight.

---

## Reported Coverage Summary

| Area | Coverage | Signal quality |
|------|----------|---------------|
| Trust core (moat) | 88.4% | A− — adversarial tests, real fixtures, byte-exact pins |
| Registry / catalog / snapshot / updater | 77–85% | B — strong per-package, gaps at integration boundaries |
| Content flow (provider, converter, loadout) | 77–95% per pkg | B+ — fan-out for rules/hooks, gaps in skills/MCP |
| Capmon extractors (9 formats) | 82–97% | B− — high coverage, hand-crafted fixtures only |
| CLI commands | 75.8% | B− — mix of strong integration and hollow stubs |
| TUI (v3/active) | 56.4% | B− — strong wizard/modal, weak gallery/triage goldens |
| TUI v1 (legacy) | 68.0% | C — dead weight, 75 pure-golden files |

**Aggregate signal coverage:** ~75–78% (vs. ~80% stated line coverage — modest gap).

---

## Findings by Priority

Beads issues have been filed for every item. Priority maps to beads `--priority`:
- **P0 (critical)** — `--priority=0`
- **P1 (major)** — `--priority=1`
- **P2 (minor)** — `--priority=2`
- **P3 (nice-to-have)** — `--priority=3`

Each section below lists the specific finding, its location in the code, and the corresponding beads issue ID.

---

## P0 — Critical

These are tests that silently permit known-bad states. Either a regression would not be caught, or a broken operation would leave the user's filesystem in an unknown state.

### P0.1 — sigstore-go shard-index bug regression test is absent

**Finding.** The production workaround at `cli/internal/moat/item_verify.go:buildTransparencyLogEntry` exists because `sigstore-go v1.1.4` `tlog.NewEntry()` overwrites the shard-local `InclusionProof.LogIndex` with the global cross-shard logIndex (see project memory). The test suite's happy-path tests (`TestVerifyItemSigstore_HappyPath`, `TestVerifyAttestationItem_HappyPath`) pass *coincidentally* on the current fixture but would continue passing if `buildTransparencyLogEntry` were replaced with `tle.NewEntry()`. No test pins the shard-local `LogIndex` value from the TLE or verifies the inclusion proof uses the shard-local index rather than the global one.

**Impact.** A maintainer can introduce a verification-breaking regression with a green test suite.

### P0.2 — Loadout partial-failure rollback is untested

**Finding.** `cli/internal/loadout/integration_test.go` covers happy-path apply+remove and one conflict-abort. Line 398–400 contains a comment `// or rollback should clean up)` followed by a conditional log that does not assert state. There is no test where one item in a bundle installs successfully and a later item fails mid-apply, verifying the already-installed items are cleaned up.

**Impact.** Users applying a broken loadout get silent filesystem corruption with no regression test guarding against it.

**Beads:** [`syllago-f4pp9`] (P0.1), [`syllago-irxch`] (P0.2). Parent epic: [`syllago-t5k5g`].

---

## P1 — Major

These items either close a demonstrable user-facing gap, or remove ongoing CI cost with zero regression value.

### P1.1 — Retire `cli/internal/tui_v1` (dead UI code)

**Finding.** `tui_v1` is a legacy UI package that the product no longer ships. It has 35 test files, 75 golden files, 68% coverage, and costs ~4.5s of CI time per run. The current v3 UI lives in `cli/internal/tui`. Every CI run invests compute testing a UI that doesn't exist in the product.

**Options.**
1. Delete the package (preferred if truly unused).
2. Exclude from CI with build tags if there is some retention reason.

### P1.2 — Skills conversion fan-out test (6 providers missing)

**Finding.** `TestKitchenSinkSkillRoundTrip` covers 6 targets (Claude Code, Gemini, OpenCode, Kiro, Cursor, Windsurf). Missing: Amp, Cline, RooCode, Zed, Copilot, Codex. There is no `TestFieldPreservation_Skills_FanOut` equivalent to the rules fan-out test.

**Model to copy:** `cli/internal/converter/field_preservation_test.go:TestFieldPreservation_RulesScoped` fans out a canonical rule to all 11 render targets with format-specific assertions. This is the strongest single test in the codebase. Skills deserves the same.

### P1.3 — MCP render tests for Cursor and Windsurf

**Finding.** `cli/internal/converter/mcp_test.go` covers Zed, Cline, OpenCode, Kiro, Codex, Amp. Cursor and Windsurf both support MCP via JSON-merge install paths, but have no MCP render tests. A broken MCP render for Cursor would install silently-wrong configs.

### P1.4 — Canonicalize-from-markdown source path untested for Gemini and OpenCode

**Finding.** Rules from Gemini CLI and OpenCode fall through to `canonicalizeMarkdownRule`. No test sources content *from* these providers into canonical format. If a provider-specific parsing edge case exists, it won't be caught.

### P1.5 — catalog → moat enrichment integration has no negative tests

**Finding.** `cli/internal/catalog/trust_test.go` covers display/badge logic only (string formatting, enum mapping). No test connects `TrustTier` fields on a `ContentItem` to any actual verification outcome. A bug where `enrich.go` silently sets every item to `TrustTierSigned` would not be caught.

### P1.6 — Updater: assert conservative default when `SigningPublicKey == ""`

**Finding.** In production today, `verifyChecksumSignature` is gated by `SigningPublicKey == ""` — the "no signing key configured" path emits a warning and succeeds. Tests cover invalid-signature and tampered-checksum paths, but no test asserts the production default behavior is actually conservative (i.e., that an update is NOT installed when the signing key is empty and the registry is untrusted).

### P1.7 — Snapshot partial-restore and corruption tests

**Finding.** `cli/internal/snapshot/snapshot_test.go:TestCreate_BacksUpFiles` checks only that a manifest file and a `files/` directory exist — it does not read the manifest to verify contents, does not verify backup bytes are correct, and does not test that the backup is usable for restore. There are no tests for: a partial restore (process interrupted mid-restore), a corrupt backup file, or a situation where the target path has been replaced with a symlink between snapshot and restore. The package's security surface is protecting against rollback-corruption, but only happy-path round-trip is covered.

**Beads:** [`syllago-fqi6l`] P1.1, [`syllago-o4l63`] P1.2, [`syllago-14usr`] P1.3, [`syllago-v4x2f`] P1.4, [`syllago-f27t4`] P1.5, [`syllago-aijou`] P1.6, [`syllago-8dtyq`] P1.7.

---

## P2 — Minor

These are hollow assertions, theater tests, and parity gaps that don't represent immediate user risk but erode test suite trustworthiness over time.

### P2.1 — Delete hollow stubs in `cmd/syllago/coverage_test.go`

**Finding.** Four tests provide zero behavioral signal:
- `TestIsInteractiveImpl` (line 575): `result := isInteractiveImpl(); _ = result` — output discarded.
- `TestCheckAndWarnStaleSnapshot_NoSnapshot` (line 496): calls function, no assertion.
- `TestRunLoadoutApply_NoArgs_NoCatalog` (line 1072): uses `t.Log` instead of `t.Error`, passes regardless of outcome.
- `TestRunLoadoutList_NoLoadouts`: same `t.Log` soft-assertion pattern.

These existed to hit coverage lines. Delete them. The `runAddFromShared` block in the same file is genuinely strong and should stay.

### P2.2 — Pair pure-golden TUI tests with semantic assertions

**Finding.** Several tests in `cli/internal/tui` rely solely on golden file comparison with no semantic pair:
- `golden_triage_test.go`: `TestGolden_Triage_80x30`, `TestGolden_Triage_120x40`
- `gallery_test.go`: 3 gallery golden tests
- Responsive size tests (if present in tui, not just tui_v1)

A rendering regression + one `-update-golden` run silently blesses broken output. Pair each with assertions like "cursor on item 0", "checked count is 2", "focus indicator present on active card".

### P2.3 — Fix hollow assertions in capmon extractors

**Finding.** Specific locations where `len(result.Fields) > 0` is the only check:
- `extract_typescript:TestTypeScriptExtractor_EnumWithNumbers` — no assertion that `Status.OK == "200"`.
- `extract_test.go:TestFixtures_WindsurfLLMSTxt` — only checks `len(result.Fields) > 0` and landmark presence; no field-value assertions.

Replace with field-value assertions on the extracted output.

### P2.4 — Expand markdown extractor tests

**Finding.** `extract_markdown` has only 3 test functions — less than half the typical 7-10 of other extractors. No adversarial tests (malformed tables, missing pipes, uneven columns, documents without headings, unicode in cells, mismatched `Primary` heading level). Markdown is the most human-authored format in real AI-tool docs and has the weakest test suite.

### P2.5 — Replace synthetic capmon fixtures with real provider snippets

**Finding.** No `testdata/` fixtures come from actual AI coding tool source repos. The `hooks-docs.html`, `hooks.rs`, `hooks.ts`, and `windsurf-llms.txt` fixtures are all hand-authored 20-40 line stubs written to exercise specific parser paths. Real production inputs would likely surface format variations the synthetic stubs don't.

### P2.6 — Improve `updater` `TestUpdate_FullFlow` assertion

**Finding.** `cli/internal/updater/updater_test.go:TestUpdate_FullFlow` accepts either success OR a "replacing binary" rename error as proof of "successful download + checksum". This conditional acceptance means the test can pass without the checksum step ever being fully exercised. Replace with a positive assertion that the checksum was verified.

### P2.7 — Add `signing` package real behavioral tests (or remove compile-check stubs)

**Finding.** `cli/internal/signing/signing_test.go` contains only interface compile-checks (`var _ Signer = nil`) and constant assignments. Go already enforces interface well-formedness at compile time, so these tests provide zero behavioral signal. Either add real implementation tests or remove the stubs. If an implementation ever lands here, these stubs will give false confidence.

### P2.8 — Test telemetry `Enrich()` payload integration

**Finding.** Telemetry at 74.1% coverage. The `Enrich()` function — the integration point all CLI commands use — has no test asserting that properties set via `Enrich()` actually appear in the outgoing event payload. The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` catches mismatched property keys in the catalog but not payload integration.

### P2.9 — Test `cline.go:ClineMCPSettingsPath` alternate branch (50% coverage)

**Finding.** The function selects between per-project and global settings paths; only happy path tested, alternate branch untested. Single-provider parity gap.

### P2.10 — Replace `t.Log` soft-assertions with real assertions

**Finding.** Audit all test files for `t.Log` used in a conditional where `t.Error` would be more appropriate. The `t.Log` soft-assertion pattern means tests are always green regardless of SUT behavior. Examples: `TestRunLoadoutApply_NoArgs_NoCatalog`, `TestRunLoadoutList_NoLoadouts`, the loadout `/ or rollback should clean up)` block.

### P2.11 — Reassess cobra wiring-check tests

**Finding.** Tests that only assert `cmd.Flags().Lookup("flag-name") != nil` (e.g., `TestInstallFlagsRegistered`, `TestSyncAndExportFlagsDefined`) verify wiring but not behavior. If a flag is registered but the handler ignores it, these tests still pass. Keep as one-time sanity checks, or delete if behavioral tests cover the same ground.

**Beads:** [`syllago-1547y`] P2.1, [`syllago-jdady`] P2.2, [`syllago-bk3o6`] P2.3, [`syllago-ey2zu`] P2.4, [`syllago-0vo31`] P2.5, [`syllago-yc8n6`] P2.6, [`syllago-so7gb`] P2.7, [`syllago-6sc5x`] P2.8, [`syllago-7gf47`] P2.9, [`syllago-74q7h`] P2.10, [`syllago-tmiwv`] P2.11.

---

## P3 — Nice-to-have

These are polish items. None represents a concrete risk, but each would lift the suite's overall quality floor.

### P3.1 — Rename `coverage_test.go` files that contain real invariants

**Finding.** The naming convention is misleading. Files named `coverage_test.go` in `provider/`, `add/`, `loadout/`, and `sandbox/` contain real invariant checks (e.g., `TestCoverageInternalGoConsistency` validates that `ConfigLocations[ct]` and `InstallDir(ct)` agree for every provider). The name suggests coverage padding; the content is genuine. A future reader may dismiss these files as theater without reading them.

Rename to `invariant_test.go` or similar. `cmd/syllago/coverage_test.go` is a mixed case — after P2.1 deletes the hollow stubs, the remaining strong tests could also be renamed.

### P3.2 — Add freshness check for `trusted-root-public-good.json` fixture

**Finding.** `cli/internal/moat/testdata/trusted-root-public-good.json` is a real Sigstore public-good trusted root captured from sigstore-go v1.1.4. If Fulcio/Rekor keys rotate, verification tests would silently break — the comment in `sigstore_verify_test.go:loadTrustedRoot` acknowledges this.

Add an automated freshness check (e.g., a weekly CI job or a test that warns if the fixture is older than N days) so key rotation doesn't cause silent test failures with no indication of why.

### P3.3 — Add analyzer confidence-score specific-value tests

**Finding.** `cli/internal/analyzer/confidence_test.go` exists but there is no test that verifies a specific confidence score is computed for a specific input signal combination. If weight constants shift, the bucket (Auto vs. Confirm) could flip silently.

Add tests pinning specific numeric scores for known input combinations.

### P3.4 — Add audit logger error-path tests

**Finding.** `cli/internal/audit` has no error-path tests for: `NewLogger` with an unwritable path, handling a `Log()` call after `Close()`, handling disk-full during write.

### P3.5 — Implement or remove `provmon` content-hash detection method

**Finding.** `TestCheckVersion_ContentHash` returns nil immediately because the `content-hash` detection method is explicitly unimplemented. The test confirms the path exits gracefully, not that detection works. Either implement and test properly, or remove the placeholder.

### P3.6 — Add mouse coverage for tui install wizard, add wizard, gallery navigation

**Finding.** Project rule: every TUI interactive element MUST have mouse support (see `.claude/rules/tui-wizard-patterns.md`). Current gaps: `cli/internal/tui` has zero mouse tests for the install wizard, add wizard, and gallery navigation. Only confirm modal, registry-add modal, and toast have mouse tests.

### P3.7 — Add responsive-truncation assertions to TUI size-variant tests

**Finding.** Size-variant golden tests in `tui` (60x20, 80x30, 120x40) check that rendering doesn't crash; none verify that truncation, column count, or layout breakpoints are correct. A `MaxWidth()` → `Width()` regression (word-wrap vs truncate, per project memory) passes unless a human inspects the diff.

Add assertions like "at 60 width, content row length <= 58" and "at 120 width, gallery shows N columns".

### P3.8 — Add TUI cross-model transition tests

**Finding.** Transitions into wizard mode only cover the open path. No test verifies that wizard close restores the correct tab or that `refreshContent()` fires with the right items after wizard completion (see `.claude/rules/tui-items-rebuild.md` for the correct pattern being guarded).

**Beads:** [`syllago-bww28`] P3.1, [`syllago-fu0i5`] P3.2, [`syllago-5idx1`] P3.3, [`syllago-2rwwn`] P3.4, [`syllago-p0phh`] P3.5, [`syllago-mnrsx`] P3.6, [`syllago-y94ni`] P3.7, [`syllago-f083j`] P3.8.

---

## What's Genuinely Strong (keep these as models)

These tests are the ones to copy when writing new tests — they verify the right invariants at the right level of paranoia.

- **MOAT canonical payload byte-exactness** — `cli/internal/moat/canonical_payload_test.go` pins exact byte sequence, SHA-256 digest, field ordering, and builder consistency. A switch to `json.Marshal` would break multiple tests immediately.
- **Adversarial item verification** — `cli/internal/moat/item_verify_test.go` — every rejection path has its own test with both error AND specific `VerifyError.Code` asserted.
- **Revocation fail-closed** — `cli/internal/moat/revocation_test.go:TestRevocationSet_UnknownSourceFailsClosed` prevents future `"future_source"` values from accidentally downgrading safety.
- **Rules fan-out conversion** — `cli/internal/converter/field_preservation_test.go:TestFieldPreservation_RulesScoped` routes one canonical rule to 11 render targets with format-specific assertions. Single bug breaks fan-out everywhere.
- **Wizard invariants** — `cli/internal/tui/wizard_invariant_test.go` (23 tests) pins panic messages, merge idempotency, step-machine transitions. Survives a golden-file purge.
- **Atomicity under concurrent access** — `cli/internal/installer/jsonmerge_test.go:TestWriteJSONFile_NoPartialWrites` uses a goroutine to poll file during write and fails on empty/partial content.
- **Full-flow lifecycle** — `cli/internal/loadout/integration_test.go:TestTryRoundTrip_ApplyAndAutoRevert` creates symlinks + JSON-merges hooks, verifies merge, triggers auto-revert, confirms full cleanup.

## Summary Table of Beads

| ID | Priority | Title |
|----|----------|-------|
| `syllago-t5k5g` | P1 (epic) | Test quality audit — follow-up work |
| `syllago-f4pp9` | P0 | Add sigstore-go shard-index bug regression test |
| `syllago-irxch` | P0 | Add loadout partial-failure rollback tests |
| `syllago-fqi6l` | P1 | Retire cli/internal/tui_v1 legacy UI package |
| `syllago-o4l63` | P1 | Add skills conversion fan-out test (6 providers) |
| `syllago-14usr` | P1 | Add MCP render tests for Cursor and Windsurf |
| `syllago-v4x2f` | P1 | Add canonicalize-from-markdown source tests for Gemini CLI and OpenCode |
| `syllago-f27t4` | P1 | Add catalog → moat enrichment negative integration tests |
| `syllago-aijou` | P1 | Assert conservative default in updater when SigningPublicKey is empty |
| `syllago-8dtyq` | P1 | Add snapshot partial-restore and corruption tests |
| `syllago-1547y` | P2 | Delete hollow test stubs in cmd/syllago/coverage_test.go |
| `syllago-jdady` | P2 | Pair pure-golden TUI tests with semantic assertions |
| `syllago-bk3o6` | P2 | Fix hollow assertions in capmon extractor tests |
| `syllago-ey2zu` | P2 | Expand capmon markdown extractor tests |
| `syllago-0vo31` | P2 | Replace synthetic capmon fixtures with real provider snippets |
| `syllago-yc8n6` | P2 | Fix conditional-success assertion in updater TestUpdate_FullFlow |
| `syllago-so7gb` | P2 | Add real behavioral tests to signing package or remove stubs |
| `syllago-6sc5x` | P2 | Test telemetry Enrich() payload integration |
| `syllago-7gf47` | P2 | Test Cline MCPSettingsPath alternate branch |
| `syllago-74q7h` | P2 | Audit and fix t.Log soft-assertions across CLI test suite |
| `syllago-tmiwv` | P2 | Reassess cobra wiring-check tests for signal value |
| `syllago-bww28` | P3 | Rename coverage_test.go files that contain real invariants |
| `syllago-fu0i5` | P3 | Add freshness check for trusted-root-public-good.json fixture |
| `syllago-5idx1` | P3 | Add analyzer confidence-score specific-value tests |
| `syllago-2rwwn` | P3 | Add audit logger error-path tests |
| `syllago-p0phh` | P3 | Implement or remove provmon content-hash detection method |
| `syllago-mnrsx` | P3 | Add mouse coverage for tui install wizard, add wizard, gallery |
| `syllago-y94ni` | P3 | Add responsive-truncation assertions to tui size-variant tests |
| `syllago-f083j` | P3 | Add tui cross-model state transition tests |

**Total:** 1 epic + 2 P0 + 7 P1 + 11 P2 + 8 P3 = 28 tracked follow-up items.

---

## Notes on Methodology

Each subagent was given:
- A specific scope (5–15 packages per agent) with no overlap.
- A shared rubric explicitly listing assertion-hollowness, coverage theater, golden-file vulnerability, negative-test coverage, integration honesty, and fixture realism.
- Read-only constraints (no Write/Edit, but `go test -cover` allowed for real numbers).
- A fixed output format for consistency across agents.

Per-package coverage numbers cited here are real (`go test -cover` output captured by each agent, not estimates). "Signal coverage" estimates are qualitative assessments — they're judgment calls, not measurements, but they were made consistently across all five agents using the same rubric.

One finding worth noting separately: **naming conventions matter.** The `coverage_test.go` naming across multiple packages led the evaluation to initially suspect padding. On inspection, most such files contained real invariant checks — but the misleading name is itself a maintenance hazard. P3.1 tracks the rename.
