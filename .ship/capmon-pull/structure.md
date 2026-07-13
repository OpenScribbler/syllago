# Structure Outline: capmon-pull

## Current / Desired / End State

**Current:** `docs/provider-capabilities/` holds 89 stale YAML files with no in-repo generator, no code references, and no connection to the Capability Feed that Capmon now publishes at https://openscribbler.github.io/capmon/ (`research.md` Q4, Q8). Nothing in the module compares Go `SupportsType` against that directory, and `CONTRIBUTING.md`, `docs/guides/adding-a-provider.md`, and the directory's own README document capmon commands deleted in `ef6fdd2b`.

**Desired:** A maintainer tool plus daily cron — Capmon Pull — verifies the Capability Feed fail-closed and keeps `docs/provider-capabilities/` current as verbatim Capability Documents via a single rolling PR, with a non-required CI check surfacing Coverage Drift.

**End state:** When Capmon publishes a new `data_revision`, within a day a rolling PR on the fixed branch `automation/capmon-pull` appears (or force-updates in place) carrying the verified mirror diff, a provenance marker bump, and a body listing the revision and changed providers. Its CI shows a red, non-blocking Coverage Drift check when Go's `SupportsType` claims contradict the mirrored data. A tampered, unsigned, or stale (>48h) feed writes nothing and exits non-zero — a red workflow run is the only alarm. Holden can run the same tool locally on WSL via `go run ./cmd/capmon-pull`, no `gh` binary required.

## Patterns to Follow

### Pattern: sigstore-go bundle verifier construction

**Source:** `cli/internal/moat/manifest_verify.go:168-197`

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

### Pattern: Conditional GET with ETag

**Source:** `cli/internal/moat/fetch.go:80-133`

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

### Pattern: Maintainer tool as a separate main package

**Source:** `cli/cmd/syllago-sign/main.go:1-12`

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

### Pattern: Cron workflow hygiene and the single rolling `gh` idiom

**Source:** `.github/workflows/moat-trusted-root-check.yml:16-28` and `:88-108`

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

## Design Summary

Everything lives in one new core package, `cli/internal/capfeed` (the 80%-covered logic: fetch, tolerant index read, DSSE/SLSA verification, freshness, mirror write, change detection), plus one new main package, `cli/cmd/capmon-pull`, following the syllago-sign precedent (design "Disambiguation: Separate main package", ADR 0017). Verification is a bundle-based sigstore-go policy over the GitHub attestations API response, with the signer identity a hardcoded constant and the trusted root reused from MOAT's bundled `trusted_root.json` via `moat.BundledTrustedRoot` — MOAT itself is imported read-only, no behavior change (design "Disambiguation: Bundle-based sigstore-go verifier", ADR 0015; "Decisions made": trusted-root reuse, hardcoded signer). The fetch code is new, copying `moat.Fetcher`'s shape rather than generalizing it (ADR 0016). Fail-closed ordering is absolute: verify provenance, then read `generated_at` for the 48h staleness gate, then fetch and sha256-verify every listed file, and only then write — any failure writes nothing and exits non-zero (design "Decisions made": fail-closed ordering). The mirror is verbatim and authoritative over its subtree: feed-relative layout under `docs/provider-capabilities/`, a `provenance.json` marker recording `data_revision`/`generated_at`, and a sweep that retires anything unmanaged except a keep-list (`README.md`, `compatibility-matrix.md`) — which is how the first rolling PR retires the 89 YAML baselines, `by-content-type/` views, and `schema.json` (design "Decisions made": verbatim mirror, first-PR retirement). Change detection is `data_revision` against the committed marker with the ETag held best-effort in Actions cache; the rolling PR is created/updated by the cron workflow using the Aembit-issued credential (design DQ1: A) and the `gh` find-existing-then-update idiom. Coverage Drift is a fifth assertion constant in `cli/internal/provider/coverage.go` with an env-gated test invoked by a new non-required CI job (design "Decisions made": coverage idiom). Stale capmon docs are rewritten alongside (design DQ2: A).

---

## Slices

### Slice 1: Polite Capability Feed polling with tolerant index read

**Observable outcome:** `go run ./cmd/capmon-pull -check -feed-url <url>` fetches `v1/index.json` with a conditional GET, tolerantly decodes it, and prints the feed's `data_revision` and `generated_at`; a malformed or unreachable index exits non-zero. Tests observe the polite-consumer contract (User-Agent, `If-None-Match`, 304 handling, size cap) server-side via httptest.

**Interfaces introduced or modified:**

- `capfeed.Fetcher` — `type Fetcher struct { Client *http.Client; UserAgent string }`; `func (f *Fetcher) Fetch(ctx context.Context, url, prevETag string) (*FetchResult, error)` with `FetchResult{Body []byte, ETag string, NotModified bool, FetchedAt time.Time}` — **Deps:** `local-substitutable` (URL is an argument; tests pass `httptest.NewServer` URLs, per `moat/fetch_test.go:19-36`)
  - **Hides:** Header construction, 304 mapping, `io.LimitReader` size cap, default 30s timeout when `Client` is nil, context wiring
  - **Exposes:** Raw bytes; the server ETag for persistence; a not-modified flag; a fetch timestamp. No parsing — this is the bytes-typed sibling of `moat.Fetcher` (ADR 0016), reused later for per-file fetches and the attestations API
- `capfeed.ParseIndex` — `func ParseIndex(body []byte) (*Index, error)` with `Index{DataRevision string, GeneratedAt time.Time, Files []IndexFile}`, `IndexFile{Path string, SHA256 string}` — **Deps:** `in-process`
  - **Hides:** Tolerant-reader decode (default `encoding/json`, never `DisallowUnknownFields`; unknown fields and unknown file entries retained/ignored per `v1/spec/field-semantics.md`), timestamp parsing, required-field validation
  - **Exposes:** `data_revision` for change detection; `generated_at` for the staleness gate; the attested file list (path + sha256) for mirroring. Missing `data_revision`, `generated_at`, or `files` is an error — the tool cannot change-detect or verify without them (fail-closed)
- `cmd/capmon-pull` — `package main`; flags `-feed-url` (default `https://openscribbler.github.io/capmon/v1/index.json`), `-check` (fetch+parse+print only, no writes) — **Deps:** `local-substitutable` (flags point at httptest servers in tests)
  - **Hides:** Flag parsing, exit-code mapping, stdout formatting
  - **Exposes:** The syllago-sign-style `go run ./cmd/capmon-pull` entry point, runnable identically in the workflow and on WSL; doc comment declares "not shipped to end users" (ADR 0017); never appears in `build-all`, release artifacts, or `commands.json`

**Files:**

- `cli/internal/capfeed/fetch.go` — bytes-typed conditional-GET fetcher copying `moat/fetch.go:80-133` shape
- `cli/internal/capfeed/fetch_test.go` — httptest tests, table-driven
- `cli/internal/capfeed/index.go` — `Index`/`IndexFile` types + `ParseIndex`
- `cli/internal/capfeed/index_test.go` — tolerant-reader tables
- `cli/cmd/capmon-pull/main.go` — main package, `-check` path
- `cli/cmd/capmon-pull/main_test.go` — end-to-end `-check` run against httptest
- `.ship/capmon-pull/checks/slice1.sh` — checkpoint script (Go internal tests wrapped per ship-run-test Go-repo convention)

**Test cases:**

- Unit: `TestFetch_ConditionalGET` (`fetch_test.go`) — server asserts `If-None-Match` echoes the previous ETag; 304 returns `NotModified: true` with the server ETag and no body
- Unit: `TestFetch_SetsUserAgentAndAccept` (`fetch_test.go`) — server-side header assertions; nil `Client` gets the default timeout
- Unit: `TestFetch_SizeCapAndBadStatus` (`fetch_test.go`) — oversized body and non-200/304 statuses return errors, table-driven
- Unit: `TestParseIndex_TolerantUnknownFields` (`index_test.go`) — index JSON with extra top-level keys, unknown per-file keys, and unrecognized file paths decodes cleanly and retains all file entries
- Unit: `TestParseIndex_MissingRequiredFields` (`index_test.go`) — table of missing `data_revision` / `generated_at` / `files` → error
- Integration: `TestMain_CheckPrintsRevision` (`main_test.go`) — `-check -feed-url srv.URL` prints `data_revision` and `generated_at`, exits zero; malformed index exits non-zero
- Integration: `.ship/capmon-pull/checks/slice1.sh` — wraps `cd cli && go test ./internal/capfeed/... ./cmd/capmon-pull/...`; exit zero = green, non-zero = red

**Checkpoint:** `bash .ship/capmon-pull/checks/slice1.sh`

---

### Slice 2: Fail-closed provenance verification and staleness gate

**Observable outcome:** `capmon-pull -check` now refuses to trust the index until SLSA provenance verifies: it fetches the attestation bundle for `sha256(index bytes)` from GitHub's attestations API, verifies DSSE/in-toto provenance against MOAT's bundled trusted root with the pinned capmon publisher identity, and only then reads `generated_at` for the 48h freshness gate. Tampered index bytes, a wrong signer, a missing attestation, or a stale feed each exit non-zero before anything else happens.

**Interfaces introduced or modified:**

- `capfeed.FetchAttestationBundle` — `func FetchAttestationBundle(ctx context.Context, client *http.Client, sha256Hex string) ([][]byte, error)`; package-level `var attestationsAPIBaseURL = "https://api.github.com/repos/OpenScribbler/capmon/attestations/"` as the test seam (per `updater.go:23-24`) — **Deps:** `local-substitutable` (httptest server behind the base-URL var)
  - **Hides:** GitHub REST shape (`{"attestations":[{"bundle":{...}}]}`), mandatory User-Agent, `application/vnd.github+json` Accept, no-auth policy, response size cap
  - **Exposes:** The raw Sigstore bundle JSON blobs recorded for the digest (possibly several); an error when the API returns none, 404s, or serves malformed JSON — callers never see a partially decoded response
- `capfeed.VerifyFeedProvenance` — `func VerifyFeedProvenance(indexBytes []byte, bundles [][]byte, trustedRootJSON []byte) error`; pinned constants `feedSignerSubject = "OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main"` (exact Fulcio SAN form confirmed against a live bundle at implementation time) and `feedSignerIssuer = "https://token.actions.githubusercontent.com"`; internal `verifyWithIdentity(indexBytes, bundle, trustedRootJSON []byte, subject, issuer string) error` seam so tests exercise identity mismatch without mocks — **Deps:** `in-process` (pure over bytes; sigstore-go `bundle.UnmarshalJSON` + `verify.NewVerifier` + `verify.WithArtifactDigest("sha256", ...)` + `verify.WithCertificateIdentity`, per ADR 0015)
  - **Hides:** Bundle parsing, DSSE envelope handling, Rekor inclusion + integrated-timestamp requirements, certificate identity matching, multi-bundle iteration (first verifying bundle wins; none → error)
  - **Exposes:** A single nil/error verdict — the whole fail-closed gate in one call; the pinned publisher identity constants; no `VerificationResult` plumbing, keeping the security surface small and auditable
- `capfeed.CheckFreshness` — `func CheckFreshness(generatedAt, now time.Time) error` (error when `now - generatedAt > 48h`, capmon ADR 0012 heartbeat semantics) — **Deps:** `in-process`
  - **Hides:** The 48h constant and boundary arithmetic
  - **Exposes:** The staleness verdict, independently table-testable with an injected clock. Deliberately shallow (depth heuristic accepted): it is the one policy point the design orders *after* verification and *before* any file fetch, and folding it into `VerifyFeedProvenance` would blur that ordering
- `cmd/capmon-pull` — `-check` path reordered: fetch index → fetch attestation → `VerifyFeedProvenance` → `CheckFreshness` → print; trusted root obtained via `moat.BundledTrustedRoot(time.Now()).Bytes` (read-only import, `trusted_root_loader.go:123`) — **Deps:** `in-process` (moat import is a byte source only; no MOAT behavior change)
  - **Hides:** Fail-closed sequencing, trusted-root plumbing
  - **Exposes:** Exit non-zero on any verification/staleness failure, before any metadata is trusted; nothing printed from an unverified feed

**Files:**

- `cli/internal/capfeed/attest.go` — attestations-API fetch + `VerifyFeedProvenance` + pinned identity constants
- `cli/internal/capfeed/attest_test.go` — fixture-driven verification tests + httptest API tests
- `cli/internal/capfeed/freshness.go` — `CheckFreshness`
- `cli/internal/capfeed/freshness_test.go` — clock-injected table tests
- `cli/internal/capfeed/testdata/feedsnapshot` — one-time captured live snapshot: real `index.json` bytes + its real attestation bundle JSON (captured via `curl` from the live feed and attestations API; small, committed)
- `cli/cmd/capmon-pull/main.go` — verification-gated `-check` sequencing
- `cli/cmd/capmon-pull/main_test.go` — tamper integration test
- `.ship/capmon-pull/checks/slice2.sh` — checkpoint script

**Test cases:**

- Unit: `TestVerifyFeedProvenance_ValidSnapshot` (`attest_test.go`) — the recorded `cli/internal/capfeed/testdata/feedsnapshot` index bytes + bundle + `moat.BundledTrustedRoot` bytes verify successfully
- Unit: `TestVerifyFeedProvenance_TamperedIndex` (`attest_test.go`) — flip one byte of the `feedsnapshot` index → digest-mismatch error, table-driven over tamper positions
- Unit: `TestVerifyFeedProvenance_WrongSignerIdentity` (`attest_test.go`) — `verifyWithIdentity` with a different pinned subject/issuer against the valid `feedsnapshot` bundle → identity error
- Unit: `TestVerifyFeedProvenance_NoUsableBundle` (`attest_test.go`) — empty slice and garbage bundle JSON → error
- Unit: `TestFetchAttestationBundle_RequestShape` (`attest_test.go`) — httptest server asserts path `sha256:<digest>`, User-Agent, Accept, no Authorization header; 404 and malformed body → error
- Unit: `TestCheckFreshness_Boundaries` (`freshness_test.go`) — table: 47h59m OK, exactly 48h OK, 48h01m error, future `generated_at` OK
- Integration: `TestMain_CheckFailsClosedOnTamper` (`main_test.go`) — httptest feed serving tampered index + real bundle behind the base-URL var → `-check` exits non-zero and prints nothing trusted
- Integration: `.ship/capmon-pull/checks/slice2.sh` — wraps `cd cli && go test ./internal/capfeed/... ./cmd/capmon-pull/...`; exit zero = green, non-zero = red

**Checkpoint:** `bash .ship/capmon-pull/checks/slice2.sh`

---

### Slice 3: Verified verbatim mirror of Capability Documents

**Observable outcome:** A full `capmon-pull -repo-root <dir>` run (no `-check`) fetches every file listed in the verified index, verifies each against its attested sha256, and writes the feed byte-for-byte under `docs/provider-capabilities/` (feed-relative layout: `capabilities/<slug>.json`, `by-content-type/*.json`, plus any unknown listed files) with a `provenance.json` marker; unmanaged files — the 89 stale YAML baselines, the YAML `by-content-type/` views, `schema.json` — are swept away, while `README.md` and `compatibility-matrix.md` survive. One bad per-file hash means nothing is written at all.

**Interfaces introduced or modified:**

- `capfeed.FetchFeedFiles` — `func FetchFeedFiles(ctx context.Context, f *Fetcher, feedBaseURL string, files []IndexFile) (map[string][]byte, error)`; resolves each `IndexFile.Path` against the feed base, verifies `sha256` per file *before* returning; any mismatch or fetch failure returns an error and no map — **Deps:** `local-substitutable` (httptest feed server)
  - **Hides:** URL resolution against the feed base, per-file size caps, hash comparison against the verified index, partial-failure cleanup
  - **Exposes:** A complete verified `path → bytes` map, keyed by feed-relative path; or a single error naming the offending file. Callers never see partial results (fail-closed: all fetch+verify precedes all writes)
- `capfeed.WriteMirror` — `func WriteMirror(capDir string, idx *Index, files map[string][]byte) (*MirrorResult, error)` with `MirrorResult{ChangedProviders []string, Written, Removed []string}`; keep-list constant `mirrorKeepList = ["README.md", "compatibility-matrix.md"]`; writes `provenance.json` `{data_revision, generated_at}` — **Deps:** `in-process` (filesystem via the `capDir` argument; `t.TempDir()` in tests)
  - **Hides:** Directory walking, sweep-vs-keep decisions, verbatim byte writes, subdirectory creation, changed-provider diffing (old vs new `capabilities/<slug>.json` bytes)
  - **Exposes:** The authoritative-mirror contract (after a successful call the subtree equals feed ∪ marker ∪ keep-list); the changed-provider list for the PR body; the written/removed paths for logging. This sweep is the mechanism by which the *first* rolling PR retires the stale YAML inventory (design decision, `research.md` Q4)
- `cmd/capmon-pull` — full-pull path: verify (Slice 2) → `FetchFeedFiles` → `WriteMirror`; `-repo-root` flag locates `docs/provider-capabilities/` — **Deps:** `in-process`
  - **Hides:** Path assembly, fail-closed ordering, flag defaults
  - **Exposes:** Files on disk only after every gate passes; exit non-zero writes nothing; identical behavior in the workflow and on WSL

**Files:**

- `cli/internal/capfeed/files.go` — `FetchFeedFiles`
- `cli/internal/capfeed/files_test.go` — hash-mismatch and completeness tests
- `cli/internal/capfeed/mirror.go` — `WriteMirror`, keep-list, provenance marker, changed-provider diff
- `cli/internal/capfeed/mirror_test.go` — sweep/keep/verbatim/marker tests
- `cli/internal/capfeed/testdata/feedsnapshot` — extended with the snapshot's listed per-provider files (byte-exact, matching the recorded index hashes)
- `cli/cmd/capmon-pull/main.go` — full-pull wiring
- `cli/cmd/capmon-pull/main_test.go` — end-to-end pull integration test
- `.ship/capmon-pull/checks/slice3.sh` — checkpoint script

**Test cases:**

- Unit: `TestFetchFeedFiles_HashMismatchAborts` (`files_test.go`) — serve one file with corrupted bytes among valid ones → error, nil map (table over which file is corrupt)
- Unit: `TestFetchFeedFiles_AllVerified` (`files_test.go`) — every listed file fetched and byte-identical to what the server sent
- Unit: `TestWriteMirror_VerbatimIncludingUnknownFiles` (`mirror_test.go`) — an index listing an unrecognized path (e.g. `extras/new-thing.json`) is mirrored byte-for-byte (tolerant reader: unknown files mirrored, not rejected)
- Unit: `TestWriteMirror_SweepRetiresUnmanagedKeepsKeepList` (`mirror_test.go`) — pre-populate `capDir` with stale `claude-code.yaml`, `schema.json`, `by-content-type/skills.yaml`, `README.md`, `compatibility-matrix.md`; after the call only keep-list + mirrored files + `provenance.json` remain, `MirrorResult.Removed` lists the retired paths
- Unit: `TestWriteMirror_ProvenanceMarkerAndChangedProviders` (`mirror_test.go`) — marker records `data_revision`/`generated_at`; when only one provider's bytes differ from the prior on-disk copy, `ChangedProviders` lists exactly that slug
- Integration: `TestMain_PullWritesVerifiedMirror` (`main_test.go`) — httptest serving the full `cli/internal/capfeed/testdata/feedsnapshot` (index + files + attestation) → run against a temp repo root → mirrored tree present with marker; then corrupt one served file → second run exits non-zero and the temp tree is unchanged
- Integration: `.ship/capmon-pull/checks/slice3.sh` — wraps `cd cli && go test ./internal/capfeed/... ./cmd/capmon-pull/...`; exit zero = green, non-zero = red

**Checkpoint:** `bash .ship/capmon-pull/checks/slice3.sh`

---

### Slice 4: Change detection, ETag persistence, and automation summary

**Observable outcome:** Running `capmon-pull` twice against an unchanged feed is a polite no-op: the second run either short-circuits on HTTP 304 (via the persisted ETag file) or on `data_revision` matching the committed `provenance.json` marker, writes nothing, and its machine-readable summary says `"changed": false`. When the revision differs, the summary carries `data_revision`, `generated_at`, and the changed-provider list — everything the workflow needs to build the rolling PR body.

**Interfaces introduced or modified:**

- `capfeed.Run` — `func Run(ctx context.Context, opts Options) (*Summary, error)` with `Options{FeedURL, RepoRoot, ETagFile, SummaryFile string, CheckOnly bool, Now func() time.Time}` and `Summary{Changed bool, DataRevision string, GeneratedAt time.Time, ChangedProviders []string}`; the single orchestrator: fetch (with prev ETag) → 304 short-circuit → verify → freshness → marker compare (`data_revision` short-circuit, *after* verification so no unverified metadata is ever trusted) → `FetchFeedFiles` → `WriteMirror` → persist ETag → write summary JSON — **Deps:** `local-substitutable` (all network behind injectable URLs/client; clock injectable)
  - **Hides:** The complete fail-closed sequencing, both short-circuit paths, ETag file round-tripping, summary serialization
  - **Exposes:** One `Summary` + error as the entire tool contract; the summary JSON file consumed by the workflow; deterministic no-op semantics for unchanged feeds. `main.go` becomes a thin flag-to-`Options` shim (thin wiring, coverage-exempt per repo testing rules)
- `capfeed.ReadMarker` — `func ReadMarker(path string) (*Marker, error)` with `Marker{DataRevision string, GeneratedAt time.Time}`; missing file → zero-value marker, nil error (first run) — **Deps:** `in-process`
  - **Hides:** Tolerant JSON decode of `provenance.json`, missing-file handling
  - **Exposes:** The last-known-good `data_revision` for change detection; the recorded `generated_at` for logging. The workflow arranges *which* ref's marker is on disk — rolling branch if it exists, else main (Slice 6 owns that overlay)
- `cmd/capmon-pull` — final flag surface: `-feed-url`, `-repo-root`, `-etag-file`, `-summary-file`, `-check`; delegates to `capfeed.Run` — **Deps:** `in-process`
  - **Hides:** Flag/exit-code mapping, `Options` assembly
  - **Exposes:** Exit 0 with `"changed": false|true` on success; non-zero on any failure; the stable CLI contract the cron workflow scripts against

**Files:**

- `cli/internal/capfeed/run.go` — `Run`, `Options`, `Summary`, ETag file read/write, summary JSON write
- `cli/internal/capfeed/run_test.go` — orchestration tests with request-counting httptest servers
- `cli/internal/capfeed/marker.go` — `Marker`, `ReadMarker`
- `cli/internal/capfeed/marker_test.go` — first-run and tolerant-decode tables
- `cli/cmd/capmon-pull/main.go` — thin shim over `capfeed.Run`
- `cli/cmd/capmon-pull/main_test.go` — two-consecutive-runs integration test
- `.ship/capmon-pull/checks/slice4.sh` — checkpoint script

**Test cases:**

- Unit: `TestRun_RevisionMatchShortCircuits` (`run_test.go`) — marker matches the verified index's `data_revision` → `Changed: false`, zero per-file requests hit the server (request counter), no writes
- Unit: `TestRun_NotModified304ShortCircuits` (`run_test.go`) — prev ETag in the ETag file → server returns 304 → `Changed: false`, no attestation fetch, no writes
- Unit: `TestRun_ETagRoundTrip` (`run_test.go`) — first run persists the server ETag to `-etag-file`; second run sends it as `If-None-Match`
- Unit: `TestRun_VerifyPrecedesMarkerCompare` (`run_test.go`) — tampered index whose `data_revision` matches the marker still fails (verification runs before the short-circuit is trusted)
- Unit: `TestRun_SummaryJSON` (`run_test.go`) — summary file parses; changed run carries `data_revision`, `generated_at`, `changed_providers`
- Unit: `TestReadMarker_MissingAndTolerant` (`marker_test.go`) — absent file → zero marker; extra JSON keys ignored
- Integration: `TestMain_SecondRunIsNoOp` (`main_test.go`) — full pull, then re-run against the same feed: exit 0, `"changed": false`, mirror bytes untouched
- Integration: `.ship/capmon-pull/checks/slice4.sh` — wraps `cd cli && go test ./internal/capfeed/... ./cmd/capmon-pull/...` plus the package coverage gate: `go test ./internal/capfeed/ -coverprofile` must report ≥80% total; exit zero = green, non-zero = red

**Checkpoint:** `bash .ship/capmon-pull/checks/slice4.sh`

---

### Slice 5: Coverage Drift findings from mirrored Capability Documents

**Observable outcome:** `CheckCoverage` gains a fifth assertion: when a committed Capability Document's `supported` contradicts Go's `SupportsType` for a provider × content type, a `CoverageDrift` finding with assertion `go-vs-capability-feed` is returned; `supported` absent (or no Capability Document for the provider) yields no finding. A new env-gated test fails on any such finding, and a new non-required CI job runs it on every PR — locally reproducible with one env var.

**Interfaces introduced or modified:**

- `provider.AssertionGoVsCapabilityFeed` — `const AssertionGoVsCapabilityFeed = "go-vs-capability-feed"` added to the constants block at `coverage.go:26-33` — **Deps:** `in-process`
  - **Hides:** Nothing (a filterable name, per the block comment)
  - **Exposes:** The string the CI caller filters `CheckCoverage` findings on; the boundary that lets the four pre-existing assertions stay caller-less (concept out-of-scope); a stable name for telemetry/tests per the existing constants-block comment
- `provider.checkFeedCoverage` — `func checkFeedCoverage(repoRoot string, p Provider) ([]CoverageDrift, error)` in a new file (repo Go-edit pattern: new import + new code → new file), invoked from `CheckCoverage` per provider; reads `docs/provider-capabilities/capabilities/<slug>.json`, decodes tolerantly with three-state `supported` (`*bool`: absent = unknown = no finding, matching the format-YAML assertion's stance at `coverage.go:88-99`), and reports a finding on contradiction in either direction over `CoverageContentTypes` — **Deps:** `in-process`
  - **Hides:** Capability Document JSON shape, three-state decode, missing-file tolerance (no doc → no findings; malformed JSON → error so CI surfaces corruption), content-type key mapping
  - **Exposes:** `CoverageDrift` findings in the exact existing shape (`coverage.go:15-24`), tagged with the new assertion constant, merged into `CheckCoverage`'s combined result alongside the four existing assertions
- `provider.TestCoverageFeedDrift` — new test gated on `SYLLAGO_COVERAGE_FEED=1` (mirroring the `SYLLAGO_COVERAGE_STRICT` idiom at `invariant_test.go:67-75`, which stays untouched): runs `CheckCoverage(FindRepoRoot(...))`, filters to `AssertionGoVsCapabilityFeed`, fails listing each Coverage Drift — **Deps:** `in-process`
  - **Hides:** Finding filtering, drift rendering via `CoverageDrift.String()`
  - **Exposes:** The red/green signal the CI job and local runs consume; one env var to reproduce CI locally
- `.github/workflows/ci.yml` — new non-required job `coverage-drift`: pinned checkout + setup-go, `SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift` — **Deps:** `true-external` (GitHub Actions; exercised by real PR runs, structurally asserted by the checkpoint script)
  - **Hides:** Runner setup, Go toolchain caching
  - **Exposes:** The visible red-but-non-blocking Coverage Drift check on every PR, including the rolling PR; never added to required checks (the feed legitimately outpaces Go)

**Files:**

- `cli/internal/provider/coverage_feed.go` — `checkFeedCoverage` + Capability Document decode
- `cli/internal/provider/coverage.go` — add `AssertionGoVsCapabilityFeed` constant; call `checkFeedCoverage` from `CheckCoverage`
- `cli/internal/provider/coverage_feed_test.go` — table-driven unit tests + the `SYLLAGO_COVERAGE_FEED`-gated `TestCoverageFeedDrift`
- `.github/workflows/ci.yml` — non-required `coverage-drift` job
- `.ship/capmon-pull/checks/slice5.sh` — checkpoint script

**Test cases:**

- Unit: `TestCheckFeedCoverage_Contradictions` (`coverage_feed_test.go`) — table over a fixture repo root (`t.TempDir()` with fake `docs/` + `cli/` layout): feed `supported: true` vs Go `false` → finding; feed `false` vs Go `true` → finding; `supported` absent → none; no Capability Document for the provider → none; unknown JSON fields ignored; malformed JSON → error
- Unit: `TestCheckCoverage_FeedAssertionIntegrated` (exercises the `coverage.go` call site) — `CheckCoverage` over a fixture root returns feed findings tagged `go-vs-capability-feed` alongside the existing assertions' findings; existing `TestCoverageInternalGoConsistency` stays green unmodified
- Unit: `TestCoverageFeedDrift` (`coverage_feed_test.go`) — skips without `SYLLAGO_COVERAGE_FEED=1`; with it, passes against current HEAD (no `capabilities/*.json` committed yet → zero findings)
- Integration: `.ship/capmon-pull/checks/slice5.sh` — wraps `cd cli && go test ./internal/provider/...`, then `SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift`, then structural greps of `.github/workflows/ci.yml` for the `coverage-drift` job, the `SYLLAGO_COVERAGE_FEED=1` env, and full-SHA pins on any new `uses:` lines; exit zero = green, non-zero = red

**Checkpoint:** `bash .ship/capmon-pull/checks/slice5.sh`

---

### Slice 6: Daily Capmon Pull cron with rolling PR and consume-only docs

**Observable outcome:** A daily cron (and `workflow_dispatch`) runs Capmon Pull with the Aembit-issued credential: when the summary says `changed`, it commits the mirror on the fixed branch `automation/capmon-pull`, force-pushes, and creates-or-updates the single rolling PR whose body (via `--body-file`) lists the `data_revision` and changed providers — so the rolling PR gets normal CI, including the Coverage Drift check (design DQ1: A). Any tool failure leaves the branch and PR untouched and the run red. The three stale capmon doc sections now describe Capmon Pull and the consume-only relationship (design DQ2: A).

**Interfaces introduced or modified:**

- `.github/workflows/capmon-pull.yml` — new workflow: `schedule: cron '0 13 * * *'` + `workflow_dispatch: {}`; `concurrency: {group: capmon-pull, cancel-in-progress: false}`; top-level `permissions: contents: read`; checkout with `persist-credentials: false`; pinned `actions/setup-go` with `cache-dependency-path: cli/go.sum`; `actions/cache` (repo's first direct use — loss costs one unconditional GET) keyed for the ETag file; marker overlay step (`git fetch origin automation/capmon-pull` and, if it exists, `git show origin/automation/capmon-pull:docs/provider-capabilities/provenance.json` over the working copy — marker checked on the rolling branch if it exists, else main); `go run ./cmd/capmon-pull -repo-root . -etag-file ... -summary-file "$RUNNER_TEMP/summary.json"`; on `changed`: `Aembit/get-credentials` (per `release.yml:181-187`), commit as `github-actions[bot]`, force-push `automation/capmon-pull` via `x-access-token` URL, build PR body from summary JSON into `"$RUNNER_TEMP/pr-body.md"`, then `gh pr list --head automation/capmon-pull` → `gh pr edit`/`gh pr create` (adapting `moat-trusted-root-check.yml:95-108`) — **Deps:** `true-external` (GitHub Actions, Aembit, `gh`; verified by a manual `workflow_dispatch` run, structurally asserted by the checkpoint script)
  - **Hides:** Credential exchange, git plumbing, PR find-or-create, marker-ref selection; all dynamic content flows through files, never shell interpolation
  - **Exposes:** The rolling PR as the sole write path from feed to repo; at-most-daily polling cadence; normal PR CI on the rolling branch; a red run + failure email as the only alarm (no auto-filed issues)
- `docs/provider-capabilities/README.md` — rewritten: directory is a verbatim, attestation-verified mirror of the Capability Feed maintained by Capmon Pull; documents `provenance.json`, the keep-list, the local `go run ./cmd/capmon-pull` invocation, and Capability Document review before mappings graduate to a Provider Format Document — **Deps:** `in-process`
  - **Hides:** n/a (prose)
  - **Exposes:** Accurate maintainer-facing description replacing the `capmon run/seed/verify/generate` table (`README.md:40-64`); the review path from Capability Document to Provider Format Document; the consume-only contract with the external Capmon project
- `CONTRIBUTING.md` + `docs/guides/adding-a-provider.md` — the stale capmon sections (`CONTRIBUTING.md:133-166`, `adding-a-provider.md:185-302`) rewritten to describe the external Capmon project, the consume-only relationship, Capmon Pull, and Coverage Drift — **Deps:** `in-process`
  - **Hides:** n/a (prose)
  - **Exposes:** Contributor-facing workflow docs matching reality; no remaining references to deleted commands, `capmon.yml`, `.capmon-pause`, or `cli/internal/capmon/` paths

**Files:**

- `.github/workflows/capmon-pull.yml` — the daily pull workflow
- `docs/provider-capabilities/README.md` — rewritten mirror README
- `CONTRIBUTING.md` — capmon section rewrite
- `docs/guides/adding-a-provider.md` — capmon onboarding section rewrite
- `.ship/capmon-pull/checks/slice6.sh` — checkpoint script

**Test cases:**

- Integration: `.ship/capmon-pull/checks/slice6.sh` structural assertions over `.github/workflows/capmon-pull.yml` — cron + `workflow_dispatch` present; `concurrency` group with `cancel-in-progress: false`; top-level `contents: read`; every `uses:` pinned to a full SHA with version comment; `persist-credentials: false` on checkout; `--body-file` used and no `${{ }}` interpolation of feed-derived content inside `run:` bodies; `actions/cache` step present for the ETag file
- Integration: `.ship/capmon-pull/checks/slice6.sh` stale-docs assertions — greps `CONTRIBUTING.md`, `docs/guides/adding-a-provider.md`, and `docs/provider-capabilities/README.md` for banned stale strings (`syllago capmon`, `cli/internal/capmon/`, `capmon.yml`, `.capmon-pause`) and fails on any hit; then runs `cd cli && go build ./... && go vet ./...` to prove the module compiles with all new entry points
- Manual: end-to-end rolling PR verification
  1. Provision the Aembit credential (syllago contents + pull-requests write) and wire it into `Aembit/get-credentials` in `capmon-pull.yml` (design DQ1)
  2. Trigger `workflow_dispatch`; confirm the run verifies the live feed and opens the rolling PR on `automation/capmon-pull` carrying the first mirror (JSON Capability Documents + `provenance.json`, stale YAML/`schema.json`/`by-content-type` YAML retired, `README.md` + `compatibility-matrix.md` kept)
  3. Confirm normal PR CI runs on the rolling PR, including the non-required `coverage-drift` check
  4. Re-trigger the workflow with no feed change; confirm no-op (green run, `changed: false`, PR untouched, no duplicate PR)
  5. Confirm a forced failure (e.g. temporarily wrong pinned identity on a throwaway branch) produces a red run with nothing pushed

**Checkpoint:** `bash .ship/capmon-pull/checks/slice6.sh`

---

## Acceptance

- Polls `v1/index.json` at most daily with conditional GET; change detection via `data_revision` — Slices 1 & 4 tests (`TestFetch_ConditionalGET`, `TestRun_RevisionMatchShortCircuits`, `TestRun_NotModified304ShortCircuits`), cron in Slice 6
- Fail-closed verification: SLSA provenance on `v1/index.json` (pinned signer `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`), then per-file sha256, before acting; any mismatch aborts with nothing written — Slice 2 & 3 tests (`TestVerifyFeedProvenance_*`, `TestFetchFeedFiles_HashMismatchAborts`, `TestMain_PullWritesVerifiedMirror`)
- Stale feed (`generated_at` > 48h) keeps last-known-good and exits non-zero — Slice 2 (`TestCheckFreshness_Boundaries`, ordering after verification per design)
- Tolerant reader: unknown fields/files/keys ignored, open enums, `supported` absent = unknown — Slices 1, 3, 5 (`TestParseIndex_TolerantUnknownFields`, `TestWriteMirror_VerbatimIncludingUnknownFiles`, `TestCheckFeedCoverage_Contradictions`)
- Capability changes produce a single rolling syllago PR with normal CI via the Aembit credential — Slice 6 manual verification
- Go-claims-vs-data Coverage Drift check runs as a non-required PR check and locally via `SYLLAGO_COVERAGE_FEED=1` — Slice 5 checkpoint
- 80%+ test coverage on `cli/internal/capfeed` — Slice 4 checkpoint coverage gate; `cli/internal/moat/` byte-identical (imported read-only)
- Repo conventions honored: httptest for HTTP, no mocking libraries, table-driven tests, `cd cli && make fmt` before commits, `make build` after Go changes

## Out of Scope

- `advisories.json` — not mirrored, not acted on (concept).
- Rewiring `CheckCoverage`'s four pre-existing orphaned assertions or changing the `SYLLAGO_COVERAGE_STRICT` gate — only the new feed assertion gets a caller (concept; `invariant_test.go:67-75` stays as-is).
- Docs site (separate Astro repo) and RSS digest — decided 2026-07-12, closed (ticket non-goal).
- Any change to the capmon repo or the published feed contract — consume-only (ticket non-goal).
- Generalizing `moat.Fetcher` or any other `cli/internal/moat/` behavior change — the `sync.go:325` "fetch bytes API" idea stays unrealized.
- Retiring `docs/provider-capabilities/compatibility-matrix.md` — hand-maintained, not part of the Capability Feed mirror; tempting during the directory sweep but explicitly kept.
- `commands.json` regeneration — the stale `syllago capmon` entries (`commands.json:161-229`) self-heal at the next release via `_gendocs` (`release.yml:58-66`).
- Historical capmon mentions in Go comments (`gencapabilities.go:262,273`, `genproviders.go:83`, `catalog/trust.go:102`) and in plan/changelog docs — harmless provenance notes, not misleading workflow docs.
- Making the Coverage Drift check required for merge — the feed legitimately outpaces Go (concept accepted risk).
- Review-rendering aids for JSON diffs — later, additive fix if diffs prove unpleasant (concept accepted risk).
