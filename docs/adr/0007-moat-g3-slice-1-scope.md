---
id: "0007"
title: MOAT G-3 Slice-1 Scope and Trusted Root Strategy
status: accepted
date: 2026-04-18
enforcement: strict
files:
  - "cli/internal/moat/verify.go"
  - "cli/internal/moat/manifest_verify.go"
  - "cli/internal/moat/sigstore_spike_test.go"
  - "cli/internal/moat/trusted_root.json"
  - "cli/internal/moat/trusted_root_loader.go"
  - "cli/internal/config/config.go"
  - "cli/cmd/syllago/moat_cmd.go"
tags: [moat, sigstore, trust, verification, slice-scope]
---

# ADR 0007: MOAT G-3 Slice-1 Scope and Trusted Root Strategy

## Status

Accepted — 2026-04-18, after a three-round expert panel (Remy, Karpathy, Valsorda,
enterprise-security, spec-purist) reached `NO FURTHER OBJECTIONS` on all decisions.
Panel transcript: `.scratch/panel/moat-trusted-root/` (bus.jsonl + consensus.md).

## Context

G-3 (`syllago-pwojm`) is the largest MOAT gap — manifest signing and verification
with Sigstore keyless OIDC. The implementation question is **how much** lands in
the first shippable slice. Two coupled decisions drove the panel:

**D1 — Trusted root source:**
- (A) bundle Sigstore's public-good `trusted_root.json` at build time via `go:embed`
- (B) fetch via TUF at runtime from `tuf-repo-cdn.sigstore.dev`
- (C) hybrid: bundled default with optional TUF refresh

Syllago is a CLI distributed as a standalone binary. TUF bootstrapping adds a
second trust root (the TUF repository metadata signer), a persistent filesystem
cache, network dependency at first verify, and ~700 LOC of TUF client state.
Bundle-at-build-time makes every release immutable but creates a staleness
problem: the Sigstore public-good instance rotates its Fulcio CA and Rekor key
every 6–12 months, so any bundled root has a shelf life.

**D2 — First implementation slice scope:**
- (A) primitive only — `VerifyManifest` function callable from tests
- (B) primitive + `config.Registry` schema fields populated but not enforced
- (C) full end-to-end — primitive + `registry add` TOFU flow + install-time
  verification + trust-tier UX

The existing `cli/internal/moat/sigstore_verify.go` is a spike (`syllago-9jzgr`).
Its `BuildBundle` function exists because the spike operated on raw Rekor API
responses. Production publishers (the MOAT Publisher Action) emit a `.sigstore`
bundle alongside each artifact, which sigstore-go can load directly by
constructing a `*sgbundle.Bundle` and calling `UnmarshalJSON` on the bytes.
`BuildBundle` is architectural dead code that survived because nobody had
committed to the primitive's input shape.

**What's uniquely at stake here:** the signing profile is TOFU-pinned at
`registry add` time. The GitHub OIDC SAN is a string derived from owner/repo
names that the original owner can surrender by transferring the repository —
the transferee inherits the same SAN and can produce validly-signed artifacts
that match the pinned profile. GitHub emits `repository_id` and `repository_owner_id`
OIDC extensions specifically to prevent this. Any slice that captures a signing
profile without those numeric IDs creates TOFU profiles that are
vulnerable-by-construction — a state that cannot be retrofitted after an
attacker has used the transfer window.

## Decision

### D1: Bundle at build time, with staleness guardrails.

- `cli/internal/moat/trusted_root.json` is committed as a versioned asset and
  loaded at runtime via `go:embed`.
- `cli/internal/moat/trusted_root_issued_at.go` carries the bundled root's
  issuance date as a Go constant written when the asset is refreshed.
- Staleness policy is enforced on every MOAT verification path:
  - **0–89 days** since `issued_at`: silent.
  - **90–179 days**: single-line `stderr` warn on every `install` / `moat verify`.
  - **180–364 days**: multi-line `stderr` warn every invocation. Shows the
    bundled-root fingerprint, the specific hard-fail date (`YYYY-MM-DD`),
    and the upgrade path (`syllago self-update`).
  - **365+ days**: hard-fail with `MOAT_TRUSTED_ROOT_STALE`. Error names the
    `--trusted-root <path>` escape hatch and directs the operator to upgrade.
- The cliff fires on `min(issued_at + 365d, cert_validity_end)`. The bundled
  root's own validity window is authoritative — if Sigstore rotates something
  with a shorter window, we honor the shorter window.
- `syllago moat trust status` is exit-coded:
  - `0` — fresh (< 90 days).
  - `1` — warn (90–364 days).
  - `2` — expired (365+ days) OR trusted root missing OR corrupted.
- The subcommand also emits the acquisition mode line
  (`moat.trusted_root=bundled` or `moat.trusted_root_path=/etc/corp/fulcio-root.json`)
  so auditors can see which root is in effect.
- Every `VerifyManifest` call consuming a non-bundled trusted root emits an
  info-level line naming the source. Silent override is the attack surface;
  loud override is the defense.

**Rejected alternatives:**

- **Runtime TUF (B)** rejected on operational cost: TUF-as-sole-mechanism adds
  a second trust root, network dependency at first verify, and a persistent
  filesystem cache. TUF refresh is a valuable slice-2 addition but is not the
  right mechanism to introduce trust-root acquisition in slice 1.
- **Hybrid (C)** rejected for slice 1 because hybrid = bundled-defaults + TUF
  refresh; the refresh path is slice 2+. Shipping hybrid now would be A with
  TUF-as-stub, which commits to UX before the TUF design is ready.

### D2: Primitive only, plus minimal forward-compat schema.

The primitive:

```go
func VerifyManifest(
    manifestBytes    []byte,
    bundleBytes      []byte,           // .sigstore bundle
    pinnedProfile    *SigningProfile,
    trustedRootJSON  []byte,           // bundled default or operator override
) (VerificationResult, error)
```

- Consumes `.sigstore` bundles directly via `sgbundle.LoadJSONFromReader`.
  **`BuildBundle` is retired from production code and preserved only in
  test-only helpers** for the spike fixture. Production publishers emit
  `.sigstore` bundles; converting raw Rekor JSON to sigstore bundles is
  architectural dead-weight.
- Returns a structured `VerificationResult`:
  ```go
  type VerificationResult struct {
      SignatureValid        bool
      CertificateChainValid bool
      RekorProofValid       bool
      IdentityMatches       bool
      TrustedRootFresh      bool
      RevocationChecked     bool   // always false in slice 1
      TrustedRootSource     string // "bundled" or "path:<file>"
      TrustedRootIssuedAt   time.Time
      TrustedRootAgeDays    int
  }
  ```
  Callers cannot collapse to "verified" without explicitly ignoring
  `RevocationChecked: false`. This prevents the single-reader-assumption bug
  when revocation lands in a later slice.
- No live Rekor calls. Offline inclusion-proof verification against the Rekor
  public key embedded in the trusted root.
- No tombstone consultation. No `registry add` CLI flow. No TUF refresh.

The schema additions below land in slice 1, **unused by the verifier**, to
avoid a disk-migration when slice 2+ wires them through:

```go
// cli/internal/config/config.go
type Registry struct {
    // ... existing fields ...

    // MOAT slice-1 forward-compat (reserved; not yet consumed by verifier).
    TrustedRoot string `json:"trusted_root,omitempty"` // path; empty = bundled
}

type SigningProfile struct {
    // Existing (retained for back-compat):
    Issuer  string `json:"issuer"`
    Subject string `json:"subject"`

    // MOAT slice-1 additions:
    ProfileVersion    int    `json:"profile_version"`              // 1
    SubjectRegex      string `json:"subject_regex,omitempty"`
    IssuerRegex       string `json:"issuer_regex,omitempty"`
    RepositoryID      string `json:"repository_id,omitempty"`      // GitHub OIDC numeric
    RepositoryOwnerID string `json:"repository_owner_id,omitempty"`// GitHub OIDC numeric
}
```

Rule that drove the inclusion: **schema that persists to user disk has
irreversibility cost absent now → add it. Behavior whose absence creates an
exploit window → add it. Behavior whose absence creates only a missing
feature → defer.**

**The GitHub OIDC numeric-ID binding is slice-1 non-negotiable.** When
`Issuer == "https://token.actions.githubusercontent.com"` and the cert carries
the OIDC extensions `1.3.6.1.4.1.57264.1.15` (`sourceRepositoryIdentifier`,
the immutable numeric repo ID) and `1.3.6.1.4.1.57264.1.17`
(`sourceRepositoryOwnerIdentifier`), `VerifyManifest` MUST match both — in
addition to the existing SAN match. Mismatch on any dimension is a hard-fail
with `MOAT_IDENTITY_MISMATCH`. `ProfileVersion` is bumped from 1 to 2+ when
GitLab/Buildkite/etc. issuers add equivalent fields. Missing `ProfileVersion`
= v1 for back-compat on profiles captured before versioning.

> **OID correction:** the panel transcript cited `.1.12`/`.1.13` as the
> numeric-ID extensions. Those OIDs are actually `sourceRepositoryURI` and
> `sourceRepositoryDigest` — a URL string and a git commit SHA, both of which
> move with a repo transfer. The truly immutable numeric IDs sit at `.1.15`
> and `.1.17` per the sigstore/fulcio OID registry. This ADR fixes the
> reference at the authoritative source so the implementation doesn't
> inherit the transcript's error.

**Three-state trust output:** slice 1 labels content `signed` / `unsigned` /
`invalid`. The word `verified` is reserved for when revocation checking lands.
Error codes: `MOAT_SIGNED`, `MOAT_UNSIGNED`, `MOAT_INVALID`,
`MOAT_IDENTITY_MISMATCH`, `MOAT_IDENTITY_UNPINNED`, `MOAT_TRUSTED_ROOT_STALE`.
`MOAT_REVOKED` reserved for the revocation slice.

**Rejected alternatives:**

- **Full end-to-end (C)** rejected as slice creep. `registry add` TOFU UX,
  install-time verification wiring, and the full operator flow are slice 2+.
  Slice 1's correctness only requires the primitive and its test vectors.
- **Primitive alone without schema (A-strict)** rejected because
  `config.Registry` is a user-visible on-disk schema. Adding fields in slice 2
  forces a migration on every registry entry written between slices — a cost
  that dwarfs the ~20 lines of schema.

### Explicitly deferred to slice 2+ (with ADR follow-ups required)

- Revocation trust model (tombstones vs. log-event design).
- Signing-identity UX at `registry add` time (shipped allowlist vs.
  `--signing-identity` CLI flag vs. TTY-guarded interactive confirm; the panel
  did not converge — see `consensus.md` "Open item for user arbitration").
- TUF refresh (manual `syllago moat trust refresh` and/or automatic).
- Per-registry `TrustedRoot` wired through to the verifier.
- Invocation-time `--trusted-root <path>` CLI override.
- Non-GitHub OIDC issuers (GitLab, Buildkite, etc.).
- Full error taxonomy beyond the six slice-1 codes.

## Consequences

**What becomes easier:**

- Tests for `VerifyManifest` can run against a hermetic `.sigstore` bundle
  fixture without mocking Rekor API responses.
- Slice 2 adds revocation as a new bit on `VerificationResult`, not a new
  output schema — consumers who read `RevocationChecked` keep working.
- The repo-transfer forgery vector is closed for every pinned GitHub registry,
  starting day one. Operators adding registries in slice 2 capture numeric IDs
  automatically from the first cert's OIDC extensions.

**What becomes harder:**

- The bundled trusted root is an ongoing operational artifact. It must be
  refreshed at least every 9 months to stay out of the 180-day warning band.
  This requires a release-process discipline that syllago does not currently
  practice. Tracked by `syllago-eafuv` (refresh runbook + cadence mechanism).
- `BuildBundle`'s retirement to test-only means the spike fixtures (raw Rekor
  API JSON) stay in `testdata/` and need their own migration when the test
  vectors get regenerated. The TODO lives in the spike bead's closing notes.
- `syllago moat trust status` is a new user-facing surface. Its exit-code
  contract (0/1/2) is a compatibility commitment — CI pipelines will grep on
  it, so reshaping the codes in slice 2 is a breaking change.

**Parallel MOAT spec PR required.** Per spec-purist's constraint, syllago
commits to contributing a MOAT spec PR that formalizes:

1. Trusted-root acquisition modes (bundled, per-registry path, invocation
   override) as normative options for conforming clients.
2. GitHub OIDC numeric-ID binding requirement when issuer is
   `token.actions.githubusercontent.com`.
3. Error-code vocabulary for slice-1 trust states.

Without the spec PR, syllago's slice-1 shipping behavior is a unilateral
extension rather than a conforming interpretation. The panel reserved the
right to reopen if the spec PR is not drafted within a reasonable window.
Tracked by `syllago-9cv8m`.

## Implementation checklist

Sourced from the panel consensus (Remy's slice-1 list, adopted as authoritative):

1. `VerifyManifest(manifestBytes, bundleBytes, pinnedProfile, trustedRoot) (VerificationResult, error)` —
   consumes `.sigstore` bundles by constructing `*sgbundle.Bundle` and calling
   its `UnmarshalJSON` method.
2. `VerificationResult` struct exposing `RevocationChecked: false` explicitly.
3. `SigningProfile` struct gains `ProfileVersion`, `SubjectRegex`, `IssuerRegex`,
   `RepositoryID`, `RepositoryOwnerID`. GitHub issuer requires numeric-ID match.
4. `config.Registry.TrustedRoot` optional string (schema only; bundled default
   when empty).
5. Numeric-ID-aware `SigningProfile` on `config.Registry` (replaces the
   current two-field struct; `ProfileVersion: 1` on all new captures).
6. Embedded `trusted_root.json` + `issued_at` constant; 90/180/365 staleness
   policy; `moat trust status` exit-coded 0/1/2; acquisition-mode line;
   actionable cliff error.
7. Tests against real meta-registry `.sigstore` bundle including rotated-key
   and stale-root cases.
8. `BuildBundle` retired to `sigstore_spike_test.go` — the `_test.go` filename
   suffix makes it test-only by Go convention, removing `protorekor`/`rekor`/`tle`
   from the production dependency graph without needing a build tag.
9. ADR (this file) linked from updated G-3 plan section; draft MOAT spec PR
   referenced in ADR.

## References

- Panel transcript: `.scratch/panel/moat-trusted-root/bus.jsonl`
- Panel consensus summary: `.scratch/panel/moat-trusted-root/consensus.md`
- Gap analysis: `docs/plans/2026-04-12-moat-conformance-gap-analysis.md` (G-3 section)
- MOAT specification: `../moat/moat-spec.md` v0.6.0
- Fulcio OID registry: `1.3.6.1.4.1.57264.1.*` (GitHub OIDC extensions)
- sigstore-go: `github.com/sigstore/sigstore-go/pkg/{bundle,root,verify}`
- Follow-up beads: `syllago-9cv8m` (upstream MOAT spec PR),
  `syllago-eafuv` (trusted_root.json refresh runbook + cadence)
