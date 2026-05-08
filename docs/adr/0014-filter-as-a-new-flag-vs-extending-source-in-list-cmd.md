# 0014. `--filter` as a new flag vs. extending `--source` in list_cmd

Date: 2026-05-02
Status: Accepted
Feature: library-unified-view
Enforcement: advisory
Scope: `cli/cmd/syllago/list.go`

## Context

`syllago list` already had a `--source` flag (values: `library`, `shared`, `registry`, `builtin`,
`all`) that filtered by WHERE content comes from. The library-unified-view feature required
filtering by item STATE (in-library, not-in-library, project, installed, not-installed).

Two approaches were considered:

**A. Add `--filter` as a new, independent flag.**
`--source` retains its current semantics (provenance filter). `--filter` accepts repeatable state
values and is applied additively after `--source`. Both flags are independently composable:
`syllago list --source registry --filter not-in-library` is unambiguous.

**B. Extend `--source` to accept state values.**
State values (in-library, not-in-library) would become valid `--source` values alongside the
existing provenance values. One flag, broader semantics.

## Decision

Chose **A** (`--filter` as a new flag).

## Consequences

- Orthogonal concerns remain orthogonal in the CLI surface: `--source` answers "where did this
  content come from?" and `--filter` answers "what is the item's current state?".
- Both flags compose cleanly without ambiguity. Mixing provenance + state values in a single flag
  would require semantic disambiguation logic and make help text harder to read.
- `filterBySource` and `filterByState` remain separate functions in `list.go`, each with a clear
  single responsibility.
- New state predicates (e.g., `installed`, `not-installed`) extend `--filter` without touching
  `--source`; new provenance sources extend `--source` without touching `--filter`.
