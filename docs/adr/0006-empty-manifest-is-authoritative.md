---
id: "0006"
title: Empty Manifest Is Authoritative
status: accepted
date: 2026-04-01
enforcement: strict
files: ["cli/internal/catalog/scanner.go"]
tags: [manifest, scanner, registry, empty-state]
---

# ADR 0006: Empty Manifest Is Authoritative

## Status

Accepted

## Context

A `registry.yaml` with `items: []` previously triggered the scanner's directory-walk fallback — identical behavior to "no manifest at all." This meant a registry author couldn't create an empty registry: even with an explicit empty items list, the scanner would walk directories and discover content the author chose not to list.

This matters for two scenarios:

1. **Staged rollout**: An author creates a registry.yaml with zero items as a first step, planning to add items incrementally. The scanner should not surprise them by auto-discovering content they haven't vetted yet.

2. **Explicit exclusion**: An author has content in their repo that they don't want in the registry (internal tools, experiments, deprecated items). Listing `items: []` should mean "nothing here" — not "look harder."

## Decision

When `registry.yaml` exists and parses successfully:
- If the `items` key is present (even as `[]`), the items list is authoritative. The scanner does NOT fall back to directory walking. Zero items means zero items.
- If the `items` key is absent (YAML produces `nil`, not `[]`), the scanner falls back to directory walking for backwards compatibility with legacy manifests that only declare metadata (name, description) without an items list.

This distinction relies on Go's nil-vs-empty-slice semantics after YAML unmarshaling:
- `items: []` → `[]manifestItem{}` (non-nil, length 0) → authoritative empty
- No `items` key → `nil` → legacy manifest, fall back to walk

## Consequences

**What becomes easier:**
- Registry authors have full control. `items: []` is a valid, intentional choice.
- The analyzer's `WriteGeneratedManifest` can produce registries with zero items when a repo has no detectable content, and the scanner won't override that by walking.

**What becomes harder:**
- The test `TestScanFromIndex_EmptyRegistryYamlNoItems` changed from "expects walk fallback" to "expects empty catalog." Any tooling that previously relied on empty manifests triggering directory walks must be updated.
- Debugging can be surprising: a user with `items: []` in their registry.yaml won't see content that directory walking would find. The error message from `syllago manifest generate` ("no AI content detected") should help surface this.
