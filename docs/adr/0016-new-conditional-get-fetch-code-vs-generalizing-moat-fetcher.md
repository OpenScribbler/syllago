# 0016. New conditional-GET fetch code vs generalizing moat.Fetcher

Date: 2026-07-12
Status: Accepted
Feature: capmon-pull

## Context

`moat.Fetcher` is registry-manifest-specific in its parse step — a 200 response is parsed by `ParseManifest` and returned as `FetchResult.Manifest` (`fetch.go:123-133`), so the pull tool cannot use it as-is. Generalizing it means refactoring a security-relevant, user-facing verification path to serve a maintainer tool, and the signed-off concept pins `cli/internal/moat/` to "reuse patterns, no behavior change." The pattern itself (headers, 304 handling, size cap, injectable client, test seams) is ~100 lines whose duplication cost is trivial against the risk of touching MOAT's sync path.

## Decision

Chose **New fetch code in the pull tool's core package, following the `moat.Fetcher` pattern verbatim (`cli/internal/moat/fetch.go:80-133`)** over **Extending `moat.Fetcher` with a general "fetch bytes" API, as contemplated by the comment at `cli/internal/moat/sync.go:325`**.

## Consequences

Two conditional-GET implementations exist in the module — one manifest-typed in `moat`, one bytes-typed in the pull tool. If a third consumer ever appears, that is the moment to extract the general API `sync.go:325` anticipated; until then MOAT's fetch/sync code and its tests are untouched, and the pull tool's fetcher can diverge freely (e.g., binary-safe Accept headers, per-file size caps sized to the feed).
