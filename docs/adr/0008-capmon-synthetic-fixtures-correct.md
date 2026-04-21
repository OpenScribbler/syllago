---
id: "0008"
title: Synthetic Fixtures Are Correct for capmon Extractor Tests
status: accepted
date: 2026-04-21
enforcement: advisory
files: ["cli/internal/capmon/extract_*/**", "cli/internal/capmon/testdata/**", "cli/internal/capmon/extract_test.go"]
tags: [testing, capmon, fixtures, architecture]
---

# ADR 0008: Synthetic Fixtures Are Correct for capmon Extractor Tests

## Status

Accepted

## Context

The 2026-04-20 test-quality audit flagged finding P2.5: "Replace synthetic capmon fixtures with real provider snippets." The finding observed that every checked-in `cli/internal/capmon/testdata/fixtures/` file is a hand-authored 20–40 line stub (`hooks-docs.html`, `hooks.rs`, `hooks.ts`, `windsurf/llms-full.txt`) and speculated that "real production inputs would likely surface format variations the synthetic stubs don't."

Follow-up investigation (bead `syllago-0vo31`) found the finding's premise doesn't map onto capmon's actual test architecture. The package has two distinct test layers with different scope and fixture needs:

**Layer 1 — Extractor tests** (`cli/internal/capmon/extract_*/*_test.go` + the top-level `extract_test.go`). These verify format-parsing mechanics: "does the Go extractor pull exported string consts correctly? Does the TOML extractor handle nested tables? Does the HTML extractor respect the `Primary` selector?" Inputs are focused, single-purpose, and paired with tight field-value assertions.

**Layer 2 — Recognize tests** (`cli/internal/capmon/recognize_*_test.go`). These verify end-to-end provider recognition using **real upstream data**, inlined as `[]string` landmark snapshots captured from live provider docs. Example from `recognize_gemini_cli_test.go`:

```go
// realGeminiCliHooksLandmarks is a snapshot of the merged headings from
// .capmon-cache/gemini-cli/hooks.{2,3}/extracted.json as of 2026-04-16.
var realGeminiCliHooksLandmarks = []string{
    "Gemini CLI hooks", "What are hooks?", "Hook events",
    "BeforeTool", "AfterTool", "BeforeAgent", ...
}
```

The recognize layer satisfies the integration-honesty principle the audit was after — 15+ recognize tests cover amp, cline, claude-code, codex, copilot-cli, cursor, factory-droid, gemini-cli, kiro, roo-code, windsurf, and others using real captured data, refreshed by dating the snapshot and updating when upstream drifts.

A second investigation pass found that 5 of the 9 extractors (`go`, `json`, `json_schema`, `markdown`, `toml`) don't use fixture files at all — they use inline `[]byte` literals in each test. All 9 extractors are tested synthetically; 4 via checked-in fixture files, 5 via inline byte literals. Both conventions are synthetic; the inline form has tighter input-to-assertion coupling.

## Decision

**Synthetic fixtures are the correct strategy for extractor-layer tests.** The audit finding P2.5 is rejected as written. The reasoning:

1. **Real-world integration is already tested at the recognize layer.** Adding real-world fixtures at the extractor layer duplicates signal that `recognize_*_test.go` already provides.

2. **Extractor tests are parser-mechanics tests, not integration tests.** At this layer, synthetic fixtures produce stronger tests: inputs are minimal, assertions are tight (`Status.OK == 200`, not `len(fields) > 50`), and edge cases (unicode, empty values, nested scopes) can be constructed deliberately.

3. **The upstream parsers (`go/parser`, `goquery`, `encoding/json`, `BurntSushi/toml`, etc.) are already battle-tested.** Our extractors wrap these libraries — validating that they handle real-world format variations is not syllago's test burden.

4. **Adding real fixtures would either produce hollow assertions or duplicate synthetic work.** Asserting `len(fields) > N` is the anti-pattern audit P2.3 flagged. Asserting specific field values from a real file requires hand-curating expected values — equivalent work to writing synthetic fixtures, for a larger check-in.

5. **The inline-bytes vs fixture-files inconsistency is acceptable.** Both forms are synthetic. Fixture files help when a single input is shared across multiple tests (`extract_rust_test.go` reads `hooks.rs` from 4 tests). Inline bytes help when each test has a distinct, focused input (`extract_go_test.go` has 10 tests with 10 different Go snippets). Choose per-test, not per-extractor.

## Consequences

**What becomes easier:**
- Contributors know synthetic fixtures are intentional; no pressure to "upgrade" them to real-world.
- Extractor-layer test authors can pick inline bytes or fixture files based on test shape, not convention policing.
- The two-layer test contract is explicit: mechanics below, integration above.

**What becomes harder:**
- If upstream parsers ever have format-handling bugs we need to regress against, we'd need to either file upstream bugs or introduce targeted real-world fixtures for *that specific* regression — case-by-case, not blanket.
- Contributors arriving from the audit finding may need to be pointed at this ADR to understand why P2.5 wasn't implemented.

**What's deferred:**
- If a future audit finds that recognize-layer tests are themselves underpowered (e.g., only cover landmarks, not field-value extraction end-to-end), that's a separate concern tracked independently.
- If the `testdata/fixtures/` tree grows past its current 4-file size, a README describing what each fixture represents may be worth adding — not required today.

## Revisit this decision if

- A production incident traces to a parser-mechanics bug that a real-world fixture would have caught but synthetic didn't.
- The upstream libraries (`goquery`, `BurntSushi/toml`, etc.) are replaced with custom parsers — at that point, real-world format variation testing becomes syllago's responsibility.
- The two-layer architecture is collapsed into one layer (e.g., extractor and recognize tests merge).

## References

- Audit plan: `docs/plans/2026-04-20-test-quality-audit.md` (finding P2.5)
- Bead: `syllago-0vo31` (closed with rationale pointing to this ADR)
- Parent epic: `syllago-t5k5g`
