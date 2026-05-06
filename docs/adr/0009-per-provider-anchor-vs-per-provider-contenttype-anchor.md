# 0009. per-provider anchor vs per-(provider, contentType) anchor

Date: 2026-05-06
Status: Accepted
Feature: capmon-provider-batching

## Context

The design concept establishes that one issue per Provider Slug is the target state. Using the provider-only anchor as the dedup key means `FindOpenCapmonIssue` needs one `gh issue list` call regardless of how many Content Types changed in a run. Using the existing per-(provider, contentType) anchor would require either multiple find-or-create calls (replicating the race window the design explicitly rejects) or a compound lookup that scans all content types, which is more complex with no benefit. The new anchor is a narrower key that correctly represents the new batching granularity.

## Decision

Chose **Per-provider anchor `<!-- capmon-check: <slug> -->` (new)** over **Per-(provider, contentType) anchor `<!-- capmon-check: <slug>/<contentType> -->` (`cli/internal/capmon/report.go`)**.

## Consequences

`FindOpenCapmonIssue` and `CreateCapmonChangeIssue` in report.go currently take `contentType string` as a parameter and embed it in the anchor. The new per-provider variants omit `contentType` from the anchor and from their signatures. The existing `(provider, contentType)`-scoped functions remain unchanged because `onboard.go` calls `CreateCapmonChangeIssue` directly at the existing granularity and is explicitly out of scope for this change. The new `FindOpenCapmonProviderIssue` and `CreateCapmonProviderIssue` functions are additive, not replacements.

---
