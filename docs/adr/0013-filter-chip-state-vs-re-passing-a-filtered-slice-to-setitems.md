# 0013. Filter chip state vs. re-passing a filtered slice to SetItems

Date: 2026-05-02
Status: Accepted
Feature: library-unified-view
Enforcement: advisory
Scope: `cli/internal/tui/library.go`

## Context

The Library tab needed filter chips (All / In Library / Not in Library / Project / Global) to let
users narrow the unified item list without triggering a catalog rescan. Two approaches were considered:

**A. Store filter state in the model; apply dynamically over the full item set.**
`libraryModel` holds `allItems []catalog.ContentItem` (the complete, unfiltered slice received from
the catalog) and a `filter libraryFilter` enum. `SetItems` stores the full slice and immediately
calls `applyFilter()`, which recomputes the visible subset. Switching chips calls `setFilter()`,
which updates the enum and re-calls `applyFilter()` — no catalog round-trip.

**B. Pass a pre-filtered slice to SetItems.**
Callers (App) would apply filter predicates before calling `SetItems`, passing only matching items.
Switching chips would require `App` to re-invoke `SetItems` with a freshly filtered slice each time.

## Decision

Chose **A** (store state, apply dynamically).

## Consequences

- Filter chips switch instantly with no I/O or catalog rescan.
- `SetItems` never resets the active filter; refreshContent can be called freely (e.g., after
  install/remove) without losing the user's current filter selection.
- `allItems` must always hold the complete, unfiltered catalog slice — callers must not pre-filter
  before passing to `SetItems`.
- `applyFilter()` is the single source of truth for what items are visible; any new filter chip
  adds a case there and nowhere else.
