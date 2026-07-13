# Design Discussion: capmon-pull

## Summary

**Current state:** `docs/provider-capabilities/` holds 89 stale YAML files with no in-repo generator, no code references, and no connection to the Capability Feed that Capmon now publishes at https://openscribbler.github.io/capmon/ (`research.md` Q4, Q8).
**Desired state:** A maintainer tool plus daily cron — Capmon Pull — verifies the Capability Feed fail-closed and keeps `docs/provider-capabilities/` current as verbatim Capability Documents via a single rolling PR, with a non-required CI check surfacing Coverage Drift.
**End state (narrative):** When Capmon publishes a new `data_revision`, within a day a rolling PR appears (or force-updates in place) carrying the verified mirror diff, a provenance marker bump, and a body listing the revision and changed providers. Its CI shows a red, non-blocking Coverage Drift check when Go's `SupportsType` claims contradict the mirrored data. A tampered, unsigned, or stale (>48h) feed writes nothing and exits non-zero — a red workflow run is the only alarm. Holden can run the same tool locally on WSL, no `gh` binary required.

## Research questions answered

- Q1: What Sigstore verification entry points exist in `cli/internal/moat/` — which functions verify a bundle, check Fulcio signer identity, and verify Rekor inclusion; what are their signatures and inputs; and do any of them handle DSSE/in-toto attestation statements (SLSA provenance) as opposed to plain artifact signatures?
- Q2: What does `cli/internal/provider/coverage.go` define — the `CheckCoverage` and `CoverageDrift` shapes, each assertion it performs, the inputs it reads, and what callers or test files reference it anywhere in the module?
- Q3: Where and how do providers declare per-content-type support in Go (`SupportsType` or equivalent) — what is the data shape, where is it defined per provider, and which packages consume it?
- Q4: What is the current file inventory of `docs/provider-capabilities/` (YAML files, schema.json, README, by-content-type/), and what references any of those paths?
- Q5: What patterns do the existing `.github/workflows/` files use for cron schedules, actions/cache, creating/updating PRs from automation, GITHUB_TOKEN permissions, and action pinning?
- Q6: What HTTP client code exists for fetching remote resources — conditional GET/ETag, GitHub API auth, timeouts, and httptest patterns?
- Q7: What precedent exists for non-user-facing maintainer tools in this module — separate main packages or cmd directories outside the syllago CLI, and how are they built and invoked?
- Q8: What residual references remain to the deleted `cli/internal/capmon` package or the old `docs/provider-capabilities` generator — in the Makefile, CI workflows, docs, or Go code — as of the current HEAD?

## Patterns to Follow

### Pattern: sigstore-go bundle verifier construction

**Source:** `cli/internal/moat/manifest_verify.go:168-197`

**Snippet:**
```go
sev, err := verify.NewVerifier(tr,
    verify.WithTransparencyLog(1),
    verify.WithIntegratedTimestamps(1),
)
// ...
certID, err := verify.NewShortCertificateIdentity(
    pinnedProfile.Issuer,
    pinnedProfile.IssuerRegex,
    pinnedProfile.Subject,
    pinnedProfile.SubjectRegex,
)
// ...
policy := verify.NewPolicy(
    verify.WithArtifact(bytes.NewReader(manifestBytes)),
    verify.WithCertificateIdentity(certID),
)
sgResult, err := sev.Verify(bundle, policy)
```

**Why it applies here:** The new SLSA provenance verification for `v1/index.json` builds a verifier the same way — trusted root, transparency-log + integrated-timestamp requirements, pinned certificate identity — but with `verify.WithArtifactDigest("sha256", ...)` in the policy, because GitHub's attestations API returns DSSE/in-toto bundles whose subject is a digest, not raw artifact bytes. MOAT has no DSSE handling today (`cli/internal/moat/rekor.go:96-101` hard-rejects non-`hashedrekord` kinds), so this is new code following the pattern, not a call into `VerifyManifest`.

### Disambiguation: Bundle-based sigstore-go verifier vs manual hashedrekord verification chain

**Chosen:** Bundle-based verifier construction, following `VerifyManifest` (`cli/internal/moat/manifest_verify.go:168-197`)
**Considered:** Manual 8-step hashedrekord chain, following `VerifyAttestationItem` (`cli/internal/moat/item_verify.go:102-107`, steps documented at `item_verify.go:3-25`)
**Why:** GitHub's attestations API serves Sigstore v0.3-family bundles containing DSSE envelopes with in-toto SLSA provenance statements. sigstore-go parses and verifies these natively through the same `bundle.UnmarshalJSON` + `verify.NewVerifier` path `VerifyManifest` already uses (`manifest_verify.go:159-171`) — including DSSE signature verification, Rekor inclusion, and certificate identity in one call. The manual chain exists only because MOAT per-item verification has no bundle on the wire (it reconstructs from raw Rekor responses) and it is hardcoded to reject anything that is not `hashedrekord` v0.0.1 (`cli/internal/moat/rekor.go:96-101`); replicating eight hand-rolled steps for a bundle format sigstore-go handles natively would be new security-critical code with no offsetting benefit.
**Consequences:** The pull tool's verification is a thin policy declaration over sigstore-go rather than bespoke crypto plumbing, which keeps the fail-closed path small and auditable. The signer identity is pinned as a hardcoded constant (subject `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`, issuer `https://token.actions.githubusercontent.com`) rather than a TOFU Signing Profile — the publisher is known at compile time, so there is no first-contact problem. `cli/internal/moat/` gains no DSSE code and stays behavior-unchanged; if MOAT ever needs DSSE support, the pull tool's implementation is the in-repo reference.

### Pattern: Conditional GET with ETag

**Source:** `cli/internal/moat/fetch.go:80-133`

**Snippet:**
```go
req.Header.Set("User-Agent", f.userAgent())
req.Header.Set("Accept", "application/json")
if prevETag != "" {
    req.Header.Set("If-None-Match", prevETag)
}
// ...
switch resp.StatusCode {
case http.StatusNotModified:
    _, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
    return &FetchResult{ETag: resp.Header.Get("ETag"), NotModified: true, FetchedAt: fetchedAt}, nil
case http.StatusOK:
    body, err := io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
```

**Why it applies here:** Polling `v1/index.json` politely means exactly this shape — `If-None-Match` from the previous ETag, 304 mapped to a not-modified result, byte-size cap, injectable `*http.Client` with a default timeout (`fetch.go:62-67`), and httptest-based tests that pass `srv.URL` directly (`cli/internal/moat/fetch_test.go:19-36`).

### Disambiguation: New conditional-GET fetch code vs generalizing moat.Fetcher

**Chosen:** New fetch code in the pull tool's core package, following the `moat.Fetcher` pattern verbatim (`cli/internal/moat/fetch.go:80-133`)
**Considered:** Extending `moat.Fetcher` with a general "fetch bytes" API, as contemplated by the comment at `cli/internal/moat/sync.go:325`
**Why:** `moat.Fetcher` is registry-manifest-specific in its parse step — a 200 response is parsed by `ParseManifest` and returned as `FetchResult.Manifest` (`fetch.go:123-133`), so the pull tool cannot use it as-is. Generalizing it means refactoring a security-relevant, user-facing verification path to serve a maintainer tool, and the signed-off concept pins `cli/internal/moat/` to "reuse patterns, no behavior change." The pattern itself (headers, 304 handling, size cap, injectable client, test seams) is ~100 lines whose duplication cost is trivial against the risk of touching MOAT's sync path.
**Consequences:** Two conditional-GET implementations exist in the module — one manifest-typed in `moat`, one bytes-typed in the pull tool. If a third consumer ever appears, that is the moment to extract the general API `sync.go:325` anticipated; until then MOAT's fetch/sync code and its tests are untouched, and the pull tool's fetcher can diverge freely (e.g., binary-safe Accept headers, per-file size caps sized to the feed).

### Pattern: Maintainer tool as a separate main package

**Source:** `cli/cmd/syllago-sign/main.go:1-12`

**Snippet:**
```go
// Build-time Ed25519 signing utility for syllago release artifacts.
// Not shipped to end users — invoked from .github/workflows/release.yml via
// `go run ./cmd/syllago-sign`. ...
//
// Subcommands:
//
//	syllago-sign keygen        Print a fresh "private=<hex>\npublic=<hex>" pair.
//	syllago-sign sign <file>   Sign <file> with the seed in $SYLLAGO_SIGNING_PRIVATE_KEY
package main
```

**Why it applies here:** Capmon Pull is a maintainer tool in the same Go module, invoked from a workflow via `go run` (as `release.yml:107` does for syllago-sign) and runnable identically on WSL — never shipped in release artifacts or `build-all` (`research.md` Q7).

### Disambiguation: Separate main package vs hidden `_`-prefixed subcommand

**Chosen:** Separate main package under `cli/cmd/`, following syllago-sign (`cli/cmd/syllago-sign/main.go:1-5`)
**Considered:** Hidden cobra subcommand on the user-facing binary, following `_gencapabilities` (`cli/cmd/syllago/gencapabilities.go:198-206`)
**Why:** The signed-off concept explicitly chose a separate maintainer tool over re-adding a `syllago capmon` surface, which would partially reverse the `ef6fdd2b` extraction that removed the in-repo capmon family. The hidden-subcommand pattern is for pure generators that dump JSON from the binary's own in-memory state (`release.yml:58-66`); Capmon Pull carries network fetching, sigstore verification, and git-tree mutation — capabilities that do not belong compiled into the user-facing CLI even hidden, and that syllago-sign's "not shipped to end users, `go run` from a workflow" precedent fits exactly.
**Consequences:** The tool gets its own `main` + `main_test.go` like syllago-sign, shares `cli/internal/` packages (provider registry, moat trusted-root loader), and is invoked as `go run ./cmd/<tool>` from the cron workflow and locally. It never appears in `commands.json`, release artifacts, or the user-facing help. The cost is one more entry point to keep compiling, covered by the module-wide `go build ./...`/`go vet` CI.

### Pattern: Cron workflow hygiene and the single rolling `gh` idiom

**Source:** `.github/workflows/moat-trusted-root-check.yml:16-28` and `:88-108`

**Snippet:**
```yaml
on:
  schedule:
    - cron: '0 12 * * 1'
  workflow_dispatch: {}
concurrency:
  group: moat-trusted-root-check
  cancel-in-progress: false
permissions:
  contents: read
```
```bash
EXISTING="$(gh issue list --search "${ISSUE_TITLE} in:title" --state open \
  --json number --jq '.[0].number // empty')"
if [ -n "${EXISTING}" ]; then
  gh issue comment "${EXISTING}" --body-file "${RUNNER_TEMP}/issue-body.md"
else
  gh issue create --title "${ISSUE_TITLE}" --body-file "${RUNNER_TEMP}/issue-body.md"
fi
```

**Why it applies here:** The new daily pull workflow copies every element of the repo's only cron+`gh` precedent — cron paired with `workflow_dispatch`, a `concurrency` group with `cancel-in-progress: false`, top-level `contents: read` with per-job escalation (`moat-trusted-root-check.yml:27-36`), full-SHA action pins with version comments, `persist-credentials: false` on checkout (`:40-41`), dynamic content flowing through `--body-file` files rather than shell interpolation (security posture comment at `:9-14`) — and adapts the find-existing-then-update idiom from issues to the single rolling PR (`gh pr list --head <branch>` → `gh pr edit`/`gh pr create`).

### Pattern: Unauthenticated GitHub API fetch

**Source:** `cli/internal/updater/updater.go:23-24,64-71`

**Snippet:**
```go
// githubAPIURL is a var so tests can override it with an httptest server URL.
var githubAPIURL = "https://api.github.com/repos/OpenScribbler/syllago/releases/latest"
// ...
// GitHub requires a User-Agent header; requests without one get a 403.
req.Header.Set("User-Agent", "syllago-updater")
req.Header.Set("Accept", "application/vnd.github+json")
```

**Why it applies here:** The attestation bundle comes from GitHub's public attestations REST API (`/repos/OpenScribbler/capmon/attestations/sha256:<digest>`), which needs no auth for public repos — same as the updater's release check. The tool follows the same shape: mandatory User-Agent, `application/vnd.github+json` Accept, shared client with an explicit timeout, and a package-level base-URL var (or injected client) as the test seam, keeping the tool free of any `gh` binary or `GITHUB_TOKEN` dependency for local WSL runs (no Go code in the module reads `GITHUB_TOKEN` today, `research.md` Q6).

### Pattern: Coverage assertion constants and CoverageDrift findings

**Source:** `cli/internal/provider/coverage.go:26-33`

**Snippet:**
```go
// Assertion names used by CheckCoverage. Kept as exported constants so tests
// and telemetry can filter on them without string-matching.
const (
    AssertionGoVsSourceManifest     = "go-vs-source-manifest"
    AssertionGoVsFormatYAML         = "go-vs-format-yaml"
    AssertionConfigLocationsVsGo    = "configlocations-vs-supportstype"
    AssertionInstallDirVsSupportsGo = "installdir-vs-supportstype"
)
```

**Why it applies here:** The new Go-claims-vs-feed assertion is a fifth named constant producing `CoverageDrift` findings (`coverage.go:15-24`) over `CoverageContentTypes` (`coverage.go:38-45`), reading the committed Capability Documents — which `CheckCoverage` never reads today (`research.md` Q2). Because `CheckCoverage` returns all assertions' findings and the four pre-existing assertions must stay caller-less (concept out-of-scope), the new CI caller filters findings by the new assertion name — exactly what the constants exist for per the comment above.

## Design Questions

1. **How does the pull job push the rolling branch so that CI (including the Coverage Drift check) actually runs on the rolling PR?** Pushes and PRs created with `GITHUB_TOKEN` do not trigger `pull_request` workflows — GitHub's recursion guard — so a `GITHUB_TOKEN`-pushed rolling branch would show no checks at all, defeating the concept's "visible red on drift" requirement. The repo's only stronger-credential precedent is the Aembit-issued key used for the cross-repo homebrew-tap push (`release.yml:181-187`).
   - A) Aembit-issued credential via `Aembit/get-credentials` (per `release.yml:181-187`), scoped to syllago contents + pull-requests write; rolling PR gets normal CI like any human PR. Requires Holden to provision the credential once.
   - B) `GITHUB_TOKEN` for the push/PR, and the pull workflow itself runs the Coverage Drift check and reports it as a commit status on the rolling branch head — no extra credential, but the check logic runs on a second path outside normal PR CI, and other checks (build, tests) still never run on the rolling PR.
   - C) <free-form slot — user fills in>
   - **Recommended:** A — it is the repo's established pattern for pushes that need more than `GITHUB_TOKEN`, and it keeps the rolling PR on the exact same CI path as every other PR instead of maintaining a parallel status-reporting mechanism.
   - **RESOLVED (Holden, 2026-07-12): A.** Aembit-issued credential via `Aembit/get-credentials`, scoped to syllago contents + pull-requests write. Holden provisions the credential once; the workflow consumes it like `release.yml:181-187`.

2. **How much stale in-repo capmon documentation does the first mirror PR clean up?** Research Q8 found live references to deleted commands and paths: `CONTRIBUTING.md:133-166` (documents `.github/workflows/capmon.yml` and `syllago capmon run`, neither exists), `docs/guides/adding-a-provider.md:185-302` (capmon onboarding against `cli/internal/capmon/`, deleted in `ef6fdd2b`), and `docs/provider-capabilities/README.md:40-64` (documents `capmon run/seed/verify/generate` and a validator path that no longer exists). `commands.json:161-229` also lists the dead `syllago capmon` family but is a generated file that `_gendocs` regenerates at the next release (`release.yml:58-66`).
   - A) Rewrite everything that becomes actively wrong when the mirror lands: `docs/provider-capabilities/README.md` (mandatory — it describes the directory being replaced), plus the `CONTRIBUTING.md:133-166` and `adding-a-provider.md:185-302` sections, rewritten to describe Capmon Pull and the consume-only relationship. `commands.json` excluded — it self-heals at the next release.
   - B) Only `docs/provider-capabilities/README.md` (unavoidable); file a follow-up bead for `CONTRIBUTING.md` and `adding-a-provider.md`.
   - C) <free-form slot — user fills in>
   - **Recommended:** A — those sections document commands deleted four days ago and will directly contradict the new workflow; rewriting them alongside the mirror is cheap and avoids a misleading interregnum where docs describe machinery that no longer exists.
   - **RESOLVED (Holden, 2026-07-12): A.** Rewrite `docs/provider-capabilities/README.md`, `CONTRIBUTING.md:133-166`, and `docs/guides/adding-a-provider.md:185-302` to describe Capmon Pull and the consume-only relationship. `commands.json` excluded — self-heals at next release.

## Decisions made (not questions)

- **In-process sigstore-go verification, not `gh attestation verify`** — concept key decision; WSL has gh 2.45 (< 2.49 required) and an external binary does not belong in the fail-closed path.
- **Signer identity is a hardcoded constant** (subject `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`, issuer `https://token.actions.githubusercontent.com`) — the publisher is known at compile time; no TOFU Signing Profile machinery needed.
- **Reuse MOAT's bundled `trusted_root.json` via its existing loader** (`root.NewTrustedRootFromJSON`, loaded through `cli/internal/moat/trusted_root_loader.go`) — capmon's attestations live on the same Sigstore public-good instance, and the weekly staleness check (`moat-trusted-root-check.yml`) already keeps that root fresh; a second pinned root would be a second thing to rot.
- **Fail-closed ordering: verify, then trust the metadata.** Fetch `v1/index.json` → fetch attestation bundle → verify SLSA provenance (DSSE subject digest equals sha256 of the fetched index bytes, certificate identity matches the pinned signer) → only then read `generated_at` for the 48h staleness check (capmon ADR 0012 heartbeat semantics, per ticket) → fetch each listed file and verify its sha256 against the verified index → only then write. Any failure at any step: write nothing, leave any open rolling PR untouched, exit non-zero.
- **Change detection by `data_revision` against a committed provenance marker** (checked on the rolling branch if it exists, else main), with the ETag held best-effort in Actions cache — concept key decision. This is the repo's first direct `actions/cache` use (`research.md` Q5: none today); cache loss only costs one unconditional GET.
- **Verbatim mirror preserving feed-relative layout** — `capabilities/<slug>.json` and `by-content-type/*.json` committed byte-for-byte under `docs/provider-capabilities/` so every committed file still matches an attested per-file hash; the provenance marker records `data_revision` and `generated_at`. Exact tree layout is Structure's call.
- **Single rolling PR on a fixed branch, force-pushed per new `data_revision`**, body built via `--body-file` carrying the `data_revision` and changed-provider list — concept key decision, executed with the `gh` CLI find-existing-then-update idiom (`moat-trusted-root-check.yml:95-108`); no third-party PR actions (repo uses none today, and every action is SHA-pinned).
- **Tolerant reader via default `encoding/json` semantics** — unknown fields are ignored by default (never `DisallowUnknownFields`); enums decoded as open strings; `supported` decoded three-state so absent = unknown, per the published field-semantics spec. Unknown files in the feed index are mirrored (verbatim mirror), not rejected.
- **Coverage Drift check invocation follows the existing env-gated test idiom** (`cli/internal/provider/invariant_test.go:67-75`, gate at `:68-70`): a new gated Go test runs `CheckCoverage`, filters findings to the new feed assertion constant, and fails on any Coverage Drift — run by a new non-required CI job on every PR. This adds the smallest new surface and stays runnable locally with one env var; the four orphaned assertions remain gated behind `SYLLAGO_COVERAGE_STRICT=1` untouched.
- **`supported` absent = unknown = no finding** in the feed assertion — matches the format-YAML assertion's existing "empty status = not asserted" stance (`coverage.go:88-99`).
- **Failure visibility is the red workflow run + failure email, nothing else** — concept key decision; no auto-filed issues (alert machinery for an audience of one).
- **First mirror PR retires the 89 YAML baselines, the `by-content-type/` YAML views, and `schema.json`** — all have zero code references (`research.md` Q4) and no in-repo regeneration path since `ef6fdd2b`. `compatibility-matrix.md` is hand-maintained (`docs/provider-capabilities/README.md:14`) and stays.
- **80%+ test coverage on new core logic**, using the module's established seams: injectable `*http.Client`/URL args with httptest servers (`cli/internal/moat/fetch_test.go:19-36`) and package-level base-URL vars (`updater.go:23-24`).

## Out of Scope

- `advisories.json` — not mirrored, not acted on (concept).
- Rewiring `CheckCoverage`'s four pre-existing orphaned assertions or changing the `SYLLAGO_COVERAGE_STRICT` gate — only the new feed assertion gets a caller (concept; `invariant_test.go:67-75` stays as-is).
- Docs site (separate Astro repo) and RSS digest — decided 2026-07-12, closed (ticket non-goal).
- Any change to the capmon repo or the published feed contract — consume-only (ticket non-goal).
- Generalizing `moat.Fetcher` or any other `cli/internal/moat/` behavior change — the `sync.go:325` "fetch bytes API" idea stays unrealized (see Disambiguation above).
- Retiring `docs/provider-capabilities/compatibility-matrix.md` — hand-maintained, not part of the Capability Feed mirror; tempting during the directory sweep but explicitly kept.
- `commands.json` regeneration — the stale `syllago capmon` entries (`commands.json:161-229`) self-heal at the next release via `_gendocs` (`release.yml:58-66`).
- Historical capmon mentions in Go comments (`gencapabilities.go:262,273`, `genproviders.go:83`, `catalog/trust.go:102`) and in plan/changelog docs — harmless provenance notes, not misleading workflow docs.
- Making the Coverage Drift check required for merge — the feed legitimately outpaces Go (concept accepted risk).
- Review-rendering aids for JSON diffs — later, additive fix if diffs prove unpleasant (concept accepted risk).

## Interfaces affected (preview)

- New maintainer tool main package under `cli/cmd/` — new; the Capmon Pull entry point (`go run` from workflow and locally; Structure names it).
- New internal package(s) for feed fetch, verification, staleness, and mirror logic — new; the 80%-covered core (Structure lays them out).
- `cli/internal/provider/coverage.go` — modified; fifth assertion constant + feed assertion reading committed Capability Documents.
- `cli/internal/provider/` tests — new env-gated test invoking the feed assertion for CI.
- `cli/internal/moat/` — imported read-only (trusted-root loading); no behavior change.
- `.github/workflows/` — new daily Capmon Pull cron workflow; new non-required Coverage Drift check job.
- `docs/provider-capabilities/` — contents replaced by mirrored Capability Documents + provenance marker; README rewritten; YAML baselines, `by-content-type/` views, and `schema.json` retired.
- `CONTRIBUTING.md`, `docs/guides/adding-a-provider.md` — stale capmon sections rewritten (pending Design Question 2).
