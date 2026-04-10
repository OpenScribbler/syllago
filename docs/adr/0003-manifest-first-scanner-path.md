---
id: "0003"
title: Manifest-First Scanner Path
status: accepted
date: 2026-04-01
enforcement: strict
files: ["cli/internal/catalog/scanner.go", "cli/internal/analyzer/manifest.go"]
tags: [scanner, manifest, registry, content-discovery]
---

# ADR 0003: Manifest-First Scanner Path

## Status

Accepted

## Context

The scanner's `scanRoot` function had a single code path: check for `registry.yaml`, and if present with items, use them. Otherwise, walk directories looking for content in known layouts. This created two problems:

1. **Repos without registry.yaml were invisible** unless they happened to use syllago's exact directory structure. A repo with `.cursor/rules/` or `.claude/agents/` would be treated as empty.

2. **Empty manifests triggered directory walking.** A `registry.yaml` with `items: []` was treated identically to "no manifest" — the scanner fell through to directory walking, potentially discovering content the author explicitly chose not to list.

The distinction between "no manifest file exists" and "manifest exists but declares zero items" is semantically meaningful: the first means "I haven't created a manifest yet" (analyze me), the second means "this registry intentionally has no items" (respect my choice).

## Decision

The scanner follows a manifest-first path:

1. **`registry.yaml` with items** → use them (existing behavior, unchanged)
2. **`registry.yaml` with `items: []`** → return empty catalog, no fallback (new: Decision #41)
3. **`registry.yaml` without items key** → fall back to directory walk (legacy support)
4. **No `registry.yaml`** → fall back to directory walk (legacy support)

The implementation distinguishes cases 2 and 3 using Go's nil-vs-empty-slice semantics: YAML `items: []` unmarshals to `[]manifestItem{}` (non-nil, empty), while a missing `items` key unmarshals to `nil`.

Additionally, `loadManifestItems` was refactored to return `([]manifestItem, bool, error)` where the bool indicates whether `registry.yaml` was found at all. This enables future use but the current check uses `items != nil` for the authoritative-vs-fallback decision.

Manifest-provided `DisplayName` and `Description` fields flow through to `ContentItem` without requiring the item's primary file to exist on disk. This enables manifests to describe items whose content lives elsewhere or hasn't been fetched yet.

## Consequences

**What becomes easier:**
- Registry authors can create an empty `registry.yaml` (`items: []`) to explicitly declare "no content here" without the scanner overriding their choice.
- The analyzer can generate manifests for any repo structure, and the scanner will read them — no scanner changes needed per provider.
- Manifest metadata (display names, descriptions) can be authored once and flow through without requiring disk reads of every content file.

**What becomes harder:**
- The existing test `TestScanFromIndex_EmptyRegistryYamlNoItems` had to change behavior: it previously expected fallback to directory walk, now expects zero items. Any code relying on empty manifests triggering walks will break.

**What's deferred:**
- Full deprecation of the directory-walk fallback path (Decision #12). Cases 3 and 4 above still trigger walks for backwards compatibility.
