# 0011. collect cache-hit counts via ProviderStatus counter vs. per-entry collection outside runStage1Fetch

Date: 2026-05-07
Status: Proposed
Feature: capmon-fetch-subcommand

## Context

`runStage1Fetch` already owns the complete fetch loop including SSRF validation, jitter, chromedp branching, healing, and meta-patching. Duplicating that loop in `capmon_cmd.go` would create two sources of truth for Stage 1 behavior — any future change to fetch mechanics would need to be applied in both places. The right answer is to surface `SourcesCacheHit` as a first-class field on `ProviderStatus`, which is the struct already designed to carry all per-provider Stage 1 outcomes (`types.go:62–78`). The Stage 1 loop increments it at the same site as `SourcesFetched` (`pipeline.go:211`), making both counters consistent.

## Decision

Chose **Add `SourcesCacheHit int` to `ProviderStatus` and increment in the Stage 1 loop (`cli/internal/capmon/types.go:62`)** over **Call `FetchSource`/`FetchChromedp` directly in the command's `RunE`, duplicating the source-iteration loop (`cli/internal/capmon/pipeline.go:162–213`)**.

## Consequences

`ProviderStatus` gains one field. The JSON serialization of `last-run.json` gains `"sources_cache_hit"` in provider entries (omitempty, so existing logs that predate this change are unaffected). The `capmon fetch` command reads this counter for `--verbose` output; `runStage1Fetch` becomes the single authoritative Stage 1 implementation as intended.

---
