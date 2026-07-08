# Codebase Simplification Audit

**Date:** 2026-07-08
**Scope:** Full repository — CLI command layer, TUI, content pipeline, capmon/telemetry, trust stack (MOAT/sandbox/registry), tests, CI, git hooks, and repo meta-systems.
**Goal:** Reduce maintenance burden for a solo maintainer without changing or breaking existing functionality.

---

## Executive summary

**Do not rewrite. Do not re-architect.** The core design — hub-and-spoke conversion through a canonical format, thin Cobra commands over `internal/` packages, a presentation-only TUI — is sound and correctly layered. A rewrite would burn scarce solo-dev time re-earning bugs that are already fixed.

The real burden comes from three accretions, in descending order of cost:

1. **A bolt-on subsystem larger than the product it serves.** Capmon (with tests and CLI commands) is ~32k lines — plus chromedp, tree-sitter/CGO, and two daily CI jobs — and has **zero coupling to the shipping product** (add/install/convert never import it). Its essential job (detect upstream doc drift) is already implemented in ~230 lines inside provmon.
2. **Process gates that tax every commit, push, and AI session.** The pre-push hook fully duplicates CI (lint + build + codegen freshness on every push). The `.wolf`/OpenWolf hook system fires Node scripts on every tool call. A commit-blocking TUI docs gate. A 95% coverage aspiration that has produced ~1,500 lines of coverage-theater tests.
3. **Mechanical repetition of small primitives.** The same frontmatter-fence parser is copy-pasted 16–18×; the same catalog-load boilerplate ~15×; the same TUI modal plumbing 11×; the same `wrapLine` closure 9×; 29 duplicate flag registrations in capmon commands.

Headline numbers: **~91k lines of non-test Go, ~139k lines of test Go, 837 files, 30+ internal packages, 12 CI workflows, 12 agent hooks, 78 TUI golden files, 15 ADRs.**

Estimated achievable reduction without behavior change: **~13–15k source lines and ~20k+ test lines leave the product repo** (mostly via capmon extraction), plus several minutes/day of gate overhead and two heavyweight build dependencies (chromedp, tree-sitter/CGO).

---

## Tier 0 — Process overhead (biggest time-per-week wins, zero code risk)

These change no product code at all.

### 0.1 Gut the pre-push hook — it fully duplicates CI
`.githooks/pre-push` (70 lines) runs full `golangci-lint run ./...`, `go build`, and two codegen-freshness diffs on **every push**. Every one of these already runs in `.github/workflows/ci.yml`. On a 91k-line codebase that is 20–60s per push, several times a day, for zero added safety.
**Action:** delete the lint and freshness blocks; if a local safety net is wanted, use `golangci-lint run --new-from-rev=origin/main`. Keep the cheap pre-commit gofmt hook.

### 0.2 Disable or delete the `.wolf` / OpenWolf meta-layer
Six of the 12 hook registrations in `.claude/settings.json` run Node scripts (`.wolf/hooks/*.js`, 1,604 lines) on **every** Read/Write/Edit/Bash/SessionStart/Stop, and `.claude/rules/openwolf.md` instructs the agent to read/update `.wolf/anatomy.md`, `memory.md`, `cerebrum.md`, and `buglog.json` around every file operation. This is the single largest per-AI-session latency source and generates constant repo churn (`suggestions.json`, `token-ledger.json`, `daemon.log`, …).
**Action:** remove the 6 `.wolf` hook entries and the openwolf rule unless actively relied on; at minimum gitignore the generated state files. Also pick **one** memory store — `.wolf/memory.md` vs `memory/project-context.md` currently duplicate each other.

### 0.3 Soften the coverage policy; delete coverage-theater tests
The 80%-min/95%-aspirational policy in `CLAUDE.md` has produced files that test stubs and print statements to hit a number:
- `cli/cmd/syllago/coverage_extras_test.go` — **1,205 lines**, including tests asserting that unimplemented stubs return "not yet implemented"
- `converter/kitchen_sink_coverage_test.go`, `catalog/types_extra_test.go`, `catalog/trust_extra_test.go`
Several packages sit at 2.5:1 test:source (installer 7,591/2,941; loadout 3,562/1,373; add 2,029/770) — a large green-keeping surface on every refactor.
**Action:** delete the stub/print tests (fold real cases into per-command files); drop the 95% aspiration and the "every new file needs a test file" rule; keep 80% on core logic packages and explicitly exempt thin command wiring.

### 0.4 Demote the TUI docs commit-block to a warning
`tui-docs-gate.sh` blocks `git commit` whenever a TUI Go file is staged without a doc file (escape hatch `--no-doc-update` shows it's already treated as noise). Keep `wizard-invariant-gate.sh` (fast, valuable).
**Action:** exit 0 with a message instead of exit 2. Also consolidate the 8 `tui-*.md` rules + tui-builder skill + `tui/CLAUDE.md` into one source of truth.

### 0.5 CI: reduce capmon cadence, drop the redundant test run
Two separate **daily** capmon workflows (`capmon.yml` 06:00, `capmon-check.yml` 07:00, the latter offset "to avoid contention") — reduce to weekly and merge into one workflow. In `ci.yml`, drop the plain `go test ./...` step (the `-race` run covers it) to roughly halve CI test time.

### 0.6 Doc/meta dedup
`.claude/rules/go-error-handling.md` restates `cli/CLAUDE.md`; `releases/` (132K per-version notes) duplicates `CHANGELOG.md`. Keep one of each.

---

## Tier 1 — Strategic: extract or shrink capmon

Capmon is the one place where the answer is structural, not mechanical.

**Facts established:**
- ~11.3k non-test + ~18.6k test lines, plus `cmd/syllago/capmon_*.go` — **~32k lines total**, larger than most of the shipping product.
- **Zero product coupling.** Nothing in add/install/convert/catalog/converter/installer imports it. The only external consumers are the `syllago capmon` subcommands, `capmon.yml` CI, and 4 small symbols used by `provmon/checker_source_hash.go`.
- It carries the repo's two heaviest non-MOAT dependencies: **chromedp** (needed by only ~15 of ~150 sources) and **tree-sitter + CGO** (needed by 20 sources that point at source code instead of docs).
- Its essential job — "tell me when provider docs change so I can update capability tables" — is already implemented as a **~230-line hash-diff checker** in provmon.

**Recommendation: extract capmon to its own repo** (or, if you're willing to hand-maintain capability tables, replace it with the provmon-style hash-diff and delete the recognize/healing layers, ~8–9k source lines). Either way the product build sheds chromedp, tree-sitter/CGO, goquery, and goldmark, and `go test ./...` stops paying for 18k+ test lines. Give provmon a small shared mini-lib (or copy) for the 4 symbols it uses.

If capmon must stay in-repo, the in-place trims are: delete the healing layer (−1,559), drop chromedp + tree-sitter extractors (−~500 + 2 heavy deps), merge the json/toml/yaml extractors — they're ~90% identical flatteners over `map[string]any` (−~200), delete the never-called `FetchGitHubFile` (−89).

---

## Tier 2 — Delete dead code (zero behavior change)

| What | Where | Lines | Evidence |
|---|---|---|---|
| `internal/signing` package | `internal/signing/signing.go` | 115 | Header says "PLANNED / UNIMPLEMENTED"; **zero importers** (confirmed by two independent audits); superseded by MOAT. Remove its row from `cli/CLAUDE.md`. |
| Frontmatter registry | `converter/frontmatter_registry.go` + 35 `RegisterFrontmatter` init calls | ~135 | Reflection-backed lookup table whose only reader is its own test. |
| Unused audit event types | `internal/audit/audit.go` | ~100 src + ~300 test | 10 `EventType` constants declared; only `EventContentInstall` is ever emitted. |
| Stub commands | `sign_cmd.go`, `refresh_cmd.go`, `export_cmd.go` + stub branches in `capmon_cmd.go` | ~150 | All return "not yet implemented"; `sign`/`verify` are misleading next to the real `moat sign`. Confirm roadmap status first; the hidden ones are risk-free. |
| MOAT spike verifier | `moat/verify.go` `VerifyItem`/`VerifyItemSigstore` | ~150 | Test-only dead weight; ADR 0007 checklist item 8 was only partially completed (`BuildBundle` stays — `moat sign` uses it). |
| Coverage-theater tests | see Tier 0.3 | ~1,500 test | — |
| Near-dead TUI constants, `key.Matches` doc drift | `tui/keys.go`, `tui/CLAUDE.md` | trivial | Doc mandates a pattern the code never uses; fix the doc. |

Total: **~650 source lines + ~1,800 test lines**, all safe.

---

## Tier 3 — Mechanical dedup (safe, behavior-preserving refactors)

Ordered by (lines saved × safety). None of these change observable behavior; the converter and TUI ones are guarded by existing kitchen-sink and golden tests.

### Content pipeline
1. **`SplitFrontmatter` helper.** The `normalize CRLF → check "---\n" → find closing fence → yaml.Unmarshal → trim body` idiom is copy-pasted **16×** (rules/skills/agents/commands/catalog/parse) plus 3 divergent hand-rolls in analyzer and metadata. One shared primitive removes ~150–200 lines and unifies subtly inconsistent fence/CRLF handling. **Highest-value single helper in the repo.**
2. **Shared render tail + collapse the "plain markdown" renderer cluster.** The `renderFrontmatter → write fm + body → Result{Filename}` tail recurs 18× in rules.go alone; `renderSingleFileRule`/`renderClaudeCodeRule`/`renderCopilotRule`/`renderOpenCodeRule`/`renderZedRule`/`renderRooCodeRule` differ only in filename + scope strategy → one helper + a small descriptor table. Windsurf/Kiro/Cursor keep their genuinely distinct functions.
3. **Capabilities as data.** All 9 hook adapters' `Capabilities()` methods return static struct literals — move to one `map[slug]ProviderCapabilities` table. Keep the Encode/Decode codecs (real logic) per file.
4. **Collapse `*WithResolver`/`*WithBase` twins** in installer/provider — thin default-forwarding wrappers.

### CLI command layer
5. **`loadCatalog()` helper.** The find-root → structured-error → scan block repeats ~15×; `ErrCatalogScanFailed` with near-identical text appears 23×. ~60–80 lines, purely mechanical.
6. **Capmon flag helpers** (if capmon stays): the same 5 flags registered **29×** with duplicated default strings; `--provider` + `SanitizeSlug` + enrich block 8×; five per-command test-override vars → one struct. ~50–70 lines.
7. **Small ones:** `gen*` shared JSON-emit/header helper (~40–50); `telemetry on|off` factory (~15); snapshot-load helper (~10); `findItemByName` (~15). The `gen*` codegen itself is *not* over-engineered — each artifact has a distinct downstream consumer; keep them separate.

### TUI (biggest package, 24k src + 28k test + 78 goldens)
The TUI is **under-abstracted, not over-abstracted** — same four primitives hand-reimplemented per screen. Recommended order:
8. **Overlay/modal interface.** 11 modal-likes each hand-wire `active` + size + the same ~9-line Update/resize/view blocks in **four places** (`app_update.go` key chain, mouse chain, WindowSize, and `resizeContent` — the last two already duplicate each other, a live drift risk). An `overlay` interface + ordered slice collapses ~150 lines. Highest ROI, no golden churn.
9. **`wrapLine`/`framePanel` helpers.** The identical pad-or-truncate closure is defined inline 9×; the manual bordered-frame assembly appears ~50× across 12 files. Pure extraction, no visual change.
10. **Shared `detailPane`.** `library.go` and `explorer.go` are near-mirrors; the drill-in detail frame (meta rows → tree|preview → borders) is built 4× (library, explorer, both wizards' review steps). ~300–400 lines, needs one careful `-update-golden` pass.
11. **Shared `searchInput` sub-model** — three parallel search state machines + two duplicate `renderSearchBar`s.
12. **Do NOT build a generic wizard framework.** The two wizards' step machines are genuinely different and the invariant tests encode their shape; extract only the shared review drill-in pane + focus-zone cycle. Likewise, do **not** split the App model — it's a large-but-organized coordinator, and findings 8–11 shrink it naturally (do collapse the six `pending*` fields into one struct).

### Test-file organization (no net lines, big navigability win)
13. Redistribute `coverage_extras_test.go`'s real tests to their source-matched files; reconcile `uninstall_cmd_test.go` vs `uninstall_cmd_monolithic_test.go` (likely overlapping). The `install_*`/`add_*` test splits were checked and are **not** redundant — leave them.

---

## Tier 4 — Correctness + simplification (behavioral care needed, highest payoff-per-line)

1. **Unify the dual catalog-scan paths.** `main.go:runTUI` hand-rebuilds registry sources and calls `moat.ScanAndEnrich` directly, with an in-code comment admitting it "mirrors moat.LoadAndScan; the two paths must stay in sync" — and it has **already caused one bug**. Migrate `runTUI` to `moat.LoadAndScan` and re-verify TUI first load. This is the single best correctness-and-simplification win in the repo.
2. **One settings.json writer.** `loadout/apply.go` self-documents that it duplicates `installer.installHook`'s hook/MCP JSON-merge logic; two independent code paths write the same provider settings files. Extract the shared merge-writer into installer and have loadout call it.
3. **One provider-capability model.** "What can provider X do with hooks" is declared three ways: `provider.HookTypes`, `converter.ProviderCapabilities` (×9 adapters), and `converter/compat.go`. Consolidate toward one source of truth feeding all three consumers (install gating, conversion warnings, TUI compat display).
4. **MOAT: route item verification through the delegating verifier.** `item_verify.go` (415 lines) hand-rolls chain/SET/inclusion verification that `manifest_verify.go` gets free from sigstore-go's `verify.NewVerifier`; the bridge (`BuildBundle`: raw Rekor JSON → bundle) already exists. Consolidating shaves ~300–500 lines and one of two duplicate OID extractors — but respect the sigstore quirks documented in ADR 0007 Addenda 1–2 and re-prove the install-time path.

---

## Explicitly keep as-is (checked, not worth touching)

- **Hub-and-spoke canonical conversion** — turns O(providers²) into O(providers); the problem is spoke boilerplate, not the design.
- **registry / registryops / moatinstall layering** — the split is forced by a real moat↔registry import cycle and fixed real CLI/TUI drift bugs.
- **sandbox** — complete, CLI+TUI-wired, README-documented, runtime-gated (compiles everywhere, no build bloat). Not an experiment.
- **promote/share** — complete, gated, tested.
- **doctor / updater / gitutil / output** — cohesive, all used. (Minor: `output` and `errordocs` duplicate the code→slug mapping with a 60-vs-58 parity gap — share one list.)
- **ADRs, beads, demos, smoke tests, security workflows** — cheap, functional signal.
- **MOAT's dependency tree** (41 sigstore/TUF/rekor modules + an active 3-CVE go-tuf pin) is the fixed cost of shipping MOAT; the only lever is MOAT itself. Keep the go-tuf pin removal on the sigstore-go upgrade checklist.

---

## Suggested sequencing

| Phase | Work | Effort | Payoff |
|---|---|---|---|
| 1 (an afternoon) | Tier 0: pre-push hook, .wolf hooks, docs-gate demotion, CI cadence | Hours | Minutes/day recovered, forever |
| 2 (a day) | Tier 2 deletions: signing, frontmatter registry, audit events, stubs, coverage-theater tests, MOAT spike | 1 day | ~650 src + ~1,800 test lines, zero risk |
| 3 (a weekend) | Tier 3 items 1, 5, 8, 9 (SplitFrontmatter, loadCatalog, overlay interface, wrapLine) | 1–2 days | ~500 lines + eliminates the two worst copy-paste families |
| 4 (a week, incremental) | Capmon extraction (Tier 1); remaining Tier 3 | Spread out | ~30k lines + 2 heavy deps leave the product repo |
| 5 (careful, one at a time) | Tier 4: scan-path unification first, then settings-writer, capability model, MOAT verifier | As time allows | Removes the three documented drift hazards |

Each phase is independently shippable; nothing depends on a later phase.
