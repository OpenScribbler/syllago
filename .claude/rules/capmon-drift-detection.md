---
paths:
  - "cli/internal/capmon/**/*.go"
  - "docs/provider-formats/*.yaml"
---

# Capmon Drift Detection Rule

**The whole point of capmon is to surface when provider docs are stale, deprecated, moved, or otherwise changed.** A 404, an empty cache, a thin landmark count, or "the cache only contains TypeScript source files" is **evidence of drift**, never justification for a won't-fix decision.

## Forbidden Behaviors

The following patterns are **not allowed** in any `recognize_*.go` file:

- A comment block that justifies a won't-fix on the basis that the cache is empty, thin, or contains the wrong kind of content (instance files, source files, 404 pages, etc.) **without** an accompanying live-URL verification line.
- Concluding "the provider does not expose this capability" purely from cache content. The cache may be reading a stale URL.
- Adding `recognizer silence is the right move` language without first probing the live docs site for moved/renamed pages.

## Required Workflow When a Cache Yields No Anchors

When you implement a recognizer fragment and the cache yields zero or thin landmarks, you **must** complete this checklist before writing any won't-fix logic:

1. **Probe the live docs site.** Use `WebSearch site:<provider-docs-host>` for the content type, plus `WebFetch` against the top result. Do not trust the format YAML's `sources` URL — that is exactly what may be stale.
2. **If the URL moved or 404'd**, update `docs/provider-formats/<provider>.yaml`:
   - Replace the stale URL in the relevant `content_types.<ct>.sources[].uri` field.
   - Bump `last_fetched_at`.
   - Clear the old `content_hash` (it will be regenerated on refetch).
3. **Refetch the cache** via the capmon fetch-extract pipeline for that provider/content-type.
4. **Verify the new cache** contains rich heading vocabulary (≥ 3 H1/H2 anchors for documentation-typed sources). If still thin, the docs page itself is genuinely sparse and you may proceed to step 5.
5. **Wire the recognizer normally** against the refetched cache.

## When a Won't-Fix Is Actually Appropriate

Won't-fix is allowed only when **all** of the following hold:

- The live docs URL has been verified (record the URL and a `verified_at: YYYY-MM-DD` timestamp in the comment block).
- The live docs page genuinely exists and was readable, but does not document the capability.
- The format YAML's curator entry agrees (e.g., `status: unsupported` or `confidence: inferred` with mechanism describing absence).

The comment block must include this attestation:

```go
// Verified live: https://provider-docs.example.com/feature on 2026-04-28.
// The page describes <X, Y, Z> but does not document <capability>.
// Format YAML status: unsupported (lines NN–NN of <provider>.yaml).
```

Any won't-fix block lacking the `Verified live:` line is a violation of this rule.

## Why This Rule Exists

In April 2026, a recognizer pass produced six won't-fix decisions across cursor, roo-code, factory-droid, and copilot-cli — each justified by cache content that turned out to come from stale or wrong source URLs. The live docs for every one of those capabilities were rich, current, and just at a different path than the format YAML pointed at. The system that was supposed to detect drift had drifted in itself, and there was no enforcement to prevent it.

This rule is enforced in three layers:

1. **This rule** — codifies the principle (you are reading it).
2. **`syllago capmon doctor` subcommand** (planned) — automated CI gate that fails on 4xx URLs, redirects to different hosts, and thin caches where format YAML claims `status: supported`. Tracked in its own bead.
3. **Buglog entry `bug-554`** — surfaces the incident pattern at session start via `.wolf/buglog.json` lookup, before any new won't-fix code is written.

## Quick Reference

| Cache observation | Allowed action |
|---|---|
| 404 / 4xx response | Probe live docs; update format YAML URL; refetch. Never skip-and-comment. |
| Empty content / 0 landmarks | Probe live docs; if genuinely empty there too, attest with `Verified live:` line. |
| TypeScript source files instead of docs | The curator pointed at source code instead of docs. Find the docs page; update format YAML. |
| Instance files instead of spec | Same — the curator pointed at example content. Find the spec/docs page; update format YAML. |
| Rich landmarks but no canonical-key match | Legitimate won't-fix candidate — but still requires `Verified live:` attestation. |
