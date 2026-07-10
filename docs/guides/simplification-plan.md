# Simplification Execution Plan (Tiers 1, 3, 4)

**Status:** Tier 0 and Tier 2 are executed (see commit history on this branch).
Companion to [`simplification-audit.md`](simplification-audit.md). This document
turns the approved-but-not-yet-executed tiers into concrete, ordered work items.

**Approvals on record:** all tiers approved; capmon extraction to a separate
repo with **zero functionality loss**; capmon cadence **every 4 days** (already
applied — `capmon.yml` cron is `0 6 */4 * *`, staleness threshold bumped
36h → 108h to match).

## Corrections to the audit discovered during execution

Verification during Tier 2 disproved four audit claims. Recorded here so they
don't get re-attempted:

1. **`converter/frontmatter_registry.go` is NOT dead.** `genproviders.go`
   calls `converter.FrontmatterFieldsFor()` to enrich `providers.json`
   (33 frontmatter entries consumed by syllago-docs). It stays.
2. **`releases/*.md` are NOT changelog duplication.** `release.yml` passes
   `releases/<tag>.md` to `gh release create --notes-file`. Both stay.
3. **`kitchen_sink_coverage_test.go` and the catalog `*_extra_test.go` files
   are real tests** (frontmatter field-drift guards and behavior tests), not
   coverage theater. They stay.
4. **TUI rules-doc consolidation is deferred** — five agent hooks reference
   the individual `tui-*.md` files by path; consolidation must update the
   hooks in the same change. (The commit-blocking gate itself was demoted to
   advisory in Tier 0.)

Known pre-existing test issue (not from this work):
`TestInstall_ModifiedState_NonInteractiveErrors/unreadable` is flaky when
tests run as root (`chmod 0` has no effect for root). Consider a
`os.Geteuid() == 0` skip.

---

## Tier 1 — Extract capmon to its own repository

**Constraint (owner decision):** no functionality may be lost. The full
pipeline — fetch (incl. chromedp), all extractors (incl. tree-sitter),
recognize, derive, diff, healing (auto-PR/issue) — moves intact. Nothing is
dropped; it just stops living inside the product module.

### Target shape

- **New repo:** `OpenScribbler/capmon` (suggested), a standalone Go module
  with `cmd/capmon/` (the former `syllago capmon *` subcommands become
  `capmon *`) and the pipeline packages from `cli/internal/capmon/`.
- **Data stays in syllago.** `docs/provider-sources/`,
  `docs/provider-formats/`, `docs/provider-capabilities/`, and
  `docs/spec/canonical-keys.yaml` remain here — they are consumed by
  `syllago info` (reads `docs/provider-formats/`), `gencapabilities`, and
  syllago-docs. The capmon repo's scheduled workflow checks out syllago,
  runs the pipeline against those dirs, and opens PRs/issues **on syllago**
  exactly as the healing layer does today.
- **provmon moves too** (decision revised during execution): provmon is
  consumed only by `cmd/provider-monitor` (a standalone dev binary, never
  wired into CI or the product) and by capmon itself
  (`fetch_validity.go`), so the whole monitoring family moves together.
  This removes the need for any `capshared` shim in syllago and lets the
  product shed chromedp completely.

### What changes for the user/dev (flagged, not silent)

- The `syllago capmon <sub>` command tree disappears from the product
  binary; the standalone `capmon <sub>` binary replaces it 1:1. This is
  dev/CI tooling, not an end-user feature — README does not advertise it.
- The syllago build sheds chromedp, go-tree-sitter (CGO!), goquery,
  goldmark, and BurntSushi/toml (verify each with `go mod tidy` — toml may
  have other consumers). CI stops needing the headless-shell container.
- `capmon.yml` moves to the new repo (same 4-day cron); it needs a PAT or
  GitHub App token with `contents:write` + `issues:write` + `pull-requests:
  write` on syllago for the healing PRs — the default `GITHUB_TOKEN` no
  longer reaches across repos. **Owner setup step.**

### Steps

1. Create `OpenScribbler/capmon` (owner action — this session cannot create
   repos). Empty, default branch `main`.
2. Preserve history: `git subtree split -P cli/internal/capmon -b
   capmon-split` in a syllago clone; push to the new repo; then layer
   `cmd/capmon/` on top by adapting `cli/cmd/syllago/capmon_*.go` (cobra
   root instead of subcommand attachment; `--providers-json` gains a
   `--syllago-dir` style flag or keeps taking explicit paths — they already
   do via `--formats-dir`/`--sources-dir`/`--cache-root`).
3. Stand up the new repo's CI: build, `go test ./...` (the ~18k test lines
   move with the code), plus the migrated `capmon.yml` (checkout syllago +
   run + PR). Run one manual `workflow_dispatch` end-to-end before step 4.
4. In syllago: introduce `internal/capshared` with provmon's four symbols;
   point provmon at it; delete `cli/internal/capmon/`, `cli/cmd/syllago/
   capmon_*.go`, `.github/workflows/capmon.yml`; `go mod tidy`; regenerate
   `commands.json`/`telemetry.json` (capmon telemetry enrichments leave the
   catalog); update ARCHITECTURE.md, `.claude/rules/capmon-drift-detection.md`
   (points to the new repo), and `docs/runbooks/` as applicable.
5. Acceptance: (a) new repo's scheduled run produces the same artifacts and
   PRs as the last in-repo run; (b) syllago `make build && make test` green;
   (c) `ldd`-level check that CGO is no longer required (`CGO_ENABLED=0 go
   build ./...` passes); (d) binary size before/after recorded.

Do **not** start Tier 3 item 3c (capmon flag helpers) — it becomes the new
repo's concern after extraction.

---

## Tier 3 — Mechanical dedup batches

One PR per batch; each is behavior-preserving and guarded by the existing
suite. Order chosen so nothing conflicts with the Tier 1 move.

> **Status (2026-07-09, bead syllago-o0e):** 3a/3b/3d/3e/3h/3i/3k executed.
> 3j was evaluated and **skipped on the merits**: the "twins" are one-line
> forwarders (e.g. `DetectProviders()` → `DetectProvidersWithResolver(nil)`)
> with ~21 call sites using the short form — removing them would make call
> sites noisier to save ~3 lines per pair. During 3a,
> `catalog.ParseFrontmatterWithBody` was found to be genuinely divergent
> (accepts a bare `---` closing fence at EOF) and stays as-is. Remaining:
> 3f, 3g (need a golden-regeneration pass), 3l (test reorganization).

| # | Batch | Scope | Guardrail |
|---|-------|-------|-----------|
| 3a | `parse.SplitFrontmatter(content) (yaml []byte, body string, ok bool)` | Replace the 16 identical fence-cut blocks in converter/{rules,skills,agents,commands}.go, catalog/frontmatter.go, parse/cursor.go. Leave analyzer's and metadata's divergent hand-rolls for a follow-up — they have subtly different semantics (line-based scanning) that must be diffed first. | converter kitchen-sink + roundtrip tests |
| 3b | `loadCatalog()` / `loadCatalogMOAT()` in cmd helpers.go | Replace the ~15 find-root→scan→StructuredError blocks; keep error codes/messages byte-identical. | cmd integration tests |
| 3d | TUI `overlay` interface | `Active()/Update()/View()/SetSize()` + ordered slice in App; collapses the 4 hand-wired modal chains in app_update.go/app.go/app_view.go. No visual change. | TUI goldens unchanged |
| 3e | TUI `padCell`/`framePanel` helpers in styles.go | Replace 9 inline `wrapLine` closures + shared T-junction frame assembly. | goldens unchanged |
| 3f | TUI shared `detailPane` | Unify library/explorer/wizard-review drill-in frames (~300-400 lines). The one batch expected to need `-update-golden`; review the diff image-by-image. | goldens regenerated + eyeballed |
| 3g | TUI shared `searchInput` sub-model | Unify library/explorer/gallery search state machines + the two `renderSearchBar` copies. | goldens + key-handling tests |
| 3h | Converter render-tail + plain-markdown renderer cluster | One helper + descriptor table for the 6 "plain markdown, maybe scope note" renderers; keep windsurf/kiro/cursor bespoke. | kitchen-sink roundtrip |
| 3i | `ProviderCapabilities` data table | Replace the 9 adapters' constant-returning `Capabilities()` methods with one map; codecs untouched. | hook conversion tests |
| 3j | Collapse `*WithResolver`/`*WithBase` twins | installer, provider. Keep exported names that tests/rules reference; deprecate rather than break if anything external uses them. | installer tests |
| 3k | Small: gen* shared JSON-emit helper; `newTelemetryToggleCmd(bool)`; snapshot-load helper; `findItemByName` | ~80 lines total. | existing tests |
| 3l | Test reorganization | Split the remaining ~1,100 lines of real tests out of `coverage_extras_test.go` into `moat_sign_cmd_test.go`, `sync_and_export_test.go`, `registry_cmd_test.go`, `explain_cmd_test.go`, `update_cmd_test.go`, `install_conflict_test.go`; reconcile `uninstall_cmd_test.go` vs `uninstall_cmd_monolithic_test.go` overlap. No net line change. | full suite green |

---

## Tier 4 — Drift-hazard fixes (one at a time, in this order)

**4a. Unify the catalog scan paths.** Migrate `main.go:runTUI` from its
hand-rolled `regSources` + `moat.ScanAndEnrich` mirror to
`moat.LoadAndScan` (the comment in main.go documents they must stay in
sync; drift already caused bug syllago-scgjl). Verify: TUI first load shows
identical items/trust badges for (no registries / registries / MOAT-pinned
registries); app golden tests; manual `syllago` launch.

**4b. One settings.json writer.** Extract installer's hook/MCP JSON-merge
writer into an exported seam; replace `loadout/apply.go`'s duplicated
`applyHook`/`applyMCP`/`appendHookEntry`/`injectSessionEndHook` with calls
to it. The loadout integration roundtrip (apply → verify → remove →
restore) is the acceptance test; diff the written settings.json bytes
before/after on the loadout fixtures.

**4c. Single provider-capability source.** Map the three consumers
(provider.HookTypes → install gating; converter.ProviderCapabilities →
conversion warnings; converter/compat.go → TUI compat display). Introduce
one table (natural home: the provider registry) and migrate consumers one
per commit. Done after 3i, which already turns the adapter copies into data.

**4d. MOAT item verification through the delegating verifier.** Rebuild
`VerifyAttestationItem`'s internals on `BuildBundle` → `verify.NewVerifier`
(the bridge `moat sign` already uses), sharing one numeric-ID OID matcher
with manifest_verify.go. Constraints: keep the exported signature (it is
moatinstall's stub seam); the ADR 0007 Addenda 1–2 quirks (SET
regeneration, inclusion-proof LogIndex) must be re-proven; all
item_verify/shard_index fixture tests must pass unchanged. Highest-care
item; do last.

---

## Suggested sequence

1. **Tier 1 step 1** (owner creates repo) can start immediately; steps 2–5
   follow.
2. Tier 3 batches 3a/3b/3k are independent of Tier 1 — start any time.
3. TUI batches 3d→3e→3f→3g in that order (each builds on the previous).
4. Converter batches 3h→3i, then Tier 4c.
5. Tier 4a and 4b independent of the above; 4d last.
