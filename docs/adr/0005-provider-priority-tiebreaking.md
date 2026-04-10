---
id: "0005"
title: Provider Priority Tiebreaking in Deduplication
status: accepted
date: 2026-04-01
enforcement: advisory
files: ["cli/internal/analyzer/dedup.go"]
tags: [dedup, priority, detectors, content-discovery]
---

# ADR 0005: Provider Priority Tiebreaking in Deduplication

## Status

Accepted

## Context

Multiple detectors can claim the same content item. A skill at `skills/my-skill/SKILL.md` matches the syllago canonical detector (0.95), the Claude Code detector (if under `.claude/skills/`), and the top-level detector (0.85). When these produce `DetectedItem` values with the same `(Type, Name)` key and the same content hash, the dedup engine must pick one winner and record the others as aliases.

The primary tiebreaker is confidence score (higher wins). But what happens when confidence scores are equal? Without a secondary tiebreaker, the winner is determined by Go map iteration order — non-deterministic and platform-dependent.

## Decision

When two items have the same `(Type, Name)` key and the same content hash, the dedup engine applies two tiebreakers in order:

1. **Higher confidence wins** (primary)
2. **Provider priority wins** (secondary, when confidence is equal)

Provider priority is a three-tier ranking:
- **Tier 0 (highest)**: `"syllago"` — canonical format always wins
- **Tier 1**: All named providers (`"claude-code"`, `"cursor"`, `"copilot"`, etc.)
- **Tier 2 (lowest)**: `"top-level"` — generic agnostic detection

This means: if a skill exists at both `skills/my-skill/SKILL.md` (syllago canonical, 0.95) and `.claude/skills/my-skill/SKILL.md` (claude-code, 0.90), the syllago canonical version wins on confidence alone. But if both are at 0.90, syllago still wins on priority.

When two items have the same key but *different* content hashes, they are recorded as a conflict pair (neither wins) for the user to resolve.

The loser's path is recorded in the winner's `Providers` slice, preserving the information that the content was detected in multiple locations.

## Consequences

**What becomes easier:**
- Deterministic dedup output regardless of detector registration order or map iteration.
- Syllago canonical format is always preferred, incentivizing registry authors to adopt it.
- The `Providers` alias list enables future UI features like "also found at: ..." without re-running detection.

**What becomes harder:**
- Adding a new provider requires deciding its tier. Currently all non-syllago, non-top-level providers share tier 1. If a need arises to rank providers against each other (e.g., "cursor rules are more authoritative than windsurf rules"), the `providerPriority` function needs refinement.
