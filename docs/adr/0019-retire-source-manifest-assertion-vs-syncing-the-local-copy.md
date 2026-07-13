# 0019. Retire the source-manifest coverage assertion vs syncing the local copy

Date: 2026-07-13
Status: Accepted
Feature: provider-sources-retirement

## Context

`CheckCoverage` assertion 1 (`go-vs-source-manifest`) compared Go `SupportsType`
declarations against static copies of provider source manifests in
`docs/provider-sources/`. Since the capmon extraction, the authoritative
manifests live in the capmon repo and syllago's copies were frozen (last
touched ~2026-04): the zed rules URL had moved, two agent sources were added
upstream, and the amp and copilot-cli hooks URLs had changed. A stale mirror
validates against yesterday's claims — the assertion could pass while the real
manifests disagreed, which is worse than no check. Assertion 5
(`go-vs-capability-feed`) already validates the same Go declarations against
the attestation-verified Capability Feed mirror that the daily `capmon-pull`
workflow keeps current.

## Decision

Chose **deleting assertion 1 and `docs/provider-sources/` outright, leaving
assertion 5 as the sole external-truth check** over **keeping the local
manifest copies and adding sync machinery to keep them current**.

## Consequences

`CheckCoverage` validates against three axes: Go internal consistency
(assertions 3, 4), format YAMLs (assertion 2), and the Capability Feed mirror
(assertion 5). External support claims now enter the repo through exactly one
door — the verified, automatically refreshed feed — so there is no second copy
to drift. Anyone needing a provider's source manifest reads it in the capmon
repo (`docs/provider-sources/<slug>.yaml` there). Re-adding a local manifest
consumer would require re-importing the data; nothing in syllago may assume
`docs/provider-sources/` exists.
