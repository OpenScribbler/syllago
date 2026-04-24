# Provider-Flow Auto-Detect Split — Design

**Bead:** syllago-50wq4
**Date:** 2026-04-24
**Status:** design

## Intent

User directive: remove the separate "Monolithic rule files" Source option; picking a Provider (or later a Local Path) should auto-detect monolithic rule files and offer the splitter inline.

## Current state

Add wizard Source step shows 5 options; option 5 ("Monolithic rule files") is the only path that reaches the splitter. Provider discovery already finds `CLAUDE.md` but imports it as a single rule named "CLAUDE". The Heuristic step is monolithic-only (it appears in the breadcrumb only when `source == addSourceMonolithic`).

## Target shape

1. Source step shows 4 options (Provider / Registry / Local Path / Git URL).
2. Provider discovery flags rules whose source matches a monolithic pattern AND passes the splitter pre-check (`splitter.CanSplit` — ≥30 lines + ≥3 H2 headings) as `splittable = true`.
3. When any discovered rule is `splittable`, the wizard inserts the Heuristic step into the flow between Discovery and Review.
4. The Heuristic step's UI is per-splittable-rule: choose "Install whole" or "Split by H2" per rule. Default is Split if the file exceeds threshold.
5. Execute:
   - Non-splittable or "Install whole" → existing install path.
   - "Split" → invoke `splitter.Split`, write sections via `rulestore`.

## Phasing

| Phase | Scope | Files touched |
|-------|-------|---------------|
| P1 | Remove option 5 from Source view; drop Source-step routing to monolithic. Keep `addSourceMonolithic` enum + internal flow + CLI `add --from <file>`. | `add_wizard_view.go`, `add_wizard_update.go`, golden files, a handful of tests that entered monolithic via Source. |
| P2 | Detect splittable rules in provider discovery. Add `splittable` + `splitSource` + `chosenHeuristic` fields to discovery candidate. | `cli/internal/add/` (discovery), `add_wizard.go` candidate struct. |
| P3 | Insert Heuristic step into Provider flow when any splittable rule exists. Reuse `viewHeuristic` but extend it to per-rule choice. | `add_wizard.go` (step routing, breadcrumb), `add_wizard_view.go`, `add_wizard_update.go`. |
| P4 | Execute branches: splittable+chosen-heuristic invokes splitter+rulestore for that rule; non-splittable uses standard install. | `add_wizard_execute_test.go`, whatever `runExecute` currently dispatches. |
| P5 | Golden files, wizard invariants, mouse zones, tests. | `testdata/`, `wizard_invariant_test.go`. |

## Non-goals

- Local Path single-file support (bead out if needed).
- Splitter heuristics beyond H2 (D7 deferred in V1).
- Registry/Git auto-detect (out of scope; those paths already go through standard install).

## Risks

- The Heuristic step currently assumes monolithic-only state (`heuristicCursor`, `chosenHeuristic`, `markerLiteral`). Extending to per-rule means either a new step or a repurposed Heuristic step with a rule selector. P3 picks the minimal: convert Heuristic to per-rule choice where each splittable candidate gets a row.
- Wizard invariants will need adjustment for a Heuristic step that can exist in non-monolithic flows.
- Golden test churn is significant.
