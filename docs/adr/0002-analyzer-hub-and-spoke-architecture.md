---
id: "0002"
title: Analyzer Hub-and-Spoke Architecture
status: accepted
date: 2026-04-01
enforcement: strict
files: ["cli/internal/analyzer/*", "cli/internal/catalog/scanner.go"]
tags: [analyzer, architecture, content-discovery, detectors]
---

# ADR 0002: Analyzer Hub-and-Spoke Architecture

## Status

Accepted

## Context

Syllago needed to discover AI content in any repository, regardless of how it's organized. The existing scanner (`cli/internal/catalog/scanner.go`) only understood syllago's own canonical layout (`skills/*/SKILL.md`, `agents/*/AGENT.md`, etc.) and provider-native directory structures. Repos organized differently — like `.cursor/rules/`, `.github/copilot-instructions.md`, or `.claude/settings.json` with hook wiring — were invisible.

Two approaches were considered:

1. **Extend the scanner** with provider-specific detection logic inline. This would bloat a file already at ~1000 lines with 10+ providers' detection patterns, pattern matching, dedup, and confidence scoring — mixing "read a manifest" concerns with "intelligently analyze a repository" concerns.

2. **New analyzer package** with a hub-and-spoke model: a central orchestrator (`Analyzer`) dispatches to pluggable `ContentDetector` implementations (one per provider), each declaring their own glob patterns and classification logic. The scanner stays focused on reading manifests.

## Decision

Create `cli/internal/analyzer/` as a new package with a hub-and-spoke architecture:

- **Hub**: `Analyzer` struct orchestrates the pipeline: Walk → Match → Classify → Dedup → References → Partition
- **Spokes**: 11 `ContentDetector` implementations, each owning their provider's patterns and classification logic
- **Scanner preserved**: `cli/internal/catalog/scanner.go` remains the manifest reader. When a `registry.yaml` exists, the scanner reads it. When one doesn't exist, the analyzer generates one.

The pipeline stages are separate, composable units:
- `walk.go` — filesystem traversal with exclusions and limits
- `match.go` — glob pattern matching against file paths
- `detector_*.go` — per-provider classification (11 files)
- `dedup.go` — same-name deduplication with priority tiebreaking
- `references.go` — markdown link and backtick path resolution
- `manifest.go` — conversion to ManifestItem and registry.yaml writing
- `validate.go` — cross-validation of authored manifests
- `reanalysis.go` — hash-based change detection for sync

## Consequences

**What becomes easier:**
- Adding a new provider detector is a single file implementing the `ContentDetector` interface (3 methods: `ProviderSlug`, `Patterns`, `Classify`). No changes to the orchestrator or other detectors.
- Each detector is independently testable with its own test file and fixtures.
- The pipeline is deterministic: Walk produces paths, MatchPatterns filters them, Classify inspects content, Dedup resolves duplicates, Partition sorts by confidence. Each stage's output is the next stage's input.

**What becomes harder:**
- Two code paths discover content: the scanner (manifest-based) and the analyzer (detection-based). They must stay in sync — the analyzer's output format (`DetectedItem`) maps to the scanner's input format (`ManifestItem`) via `ToManifestItem()`.
- The `catalog` ↔ `registry` import cycle prevents the analyzer from directly using `registry.ManifestItem` in scanner code. A `manifestItem` mirror struct in `scanner.go` must be kept in sync manually (marked with `KEEP IN SYNC` comment).

**What's deferred:**
- Deprecating the directory-walk fallback in the scanner (Decision #12). The walk path still exists for backwards compatibility but is bypassed when a manifest exists.
