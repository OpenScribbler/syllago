# MOAT Trusted-Root Refresh Runbook

**Owner:** MOAT maintainers (current reviewers of `cli/internal/moat/**`).
**Cadence:** Every 9 months, or sooner if `.github/workflows/moat-trusted-root-check.yml` opens a refresh issue.
**Related:** [ADR 0007](../adr/0007-moat-g3-slice-1-scope.md) (staleness policy).

---

## Why this runbook exists

Syllago ships a Sigstore trusted root (Fulcio CA + Rekor public keys + timestamp authorities) as a bundled asset (`cli/internal/moat/trusted_root.json`). `VerifyManifest` uses those bytes for every signed-registry verify.

The Sigstore public-good instance rotates keys every 6–12 months. Once the bundled root falls behind a rotation, verification against newly-signed manifests fails silently. ADR 0007 D1 therefore encodes a calendar-age policy:

| Age (days) | Status      | Exit | CLI behavior                  |
|------------|-------------|------|-------------------------------|
| 0–89       | `fresh`     | 0    | silent                        |
| 90–179     | `warn`      | 1    | one-line stderr warning       |
| 180–364    | `escalated` | 1    | multi-line stderr warning     |
| 365+       | `expired`   | 2    | verification refuses to run   |

If the bundled root reaches 365 days without a release, every end-user hits a hard-fail the moment they next try to add/sync a signed registry. This runbook exists so that never happens.

---

## Cadence: how the team gets the signal

`.github/workflows/moat-trusted-root-check.yml` runs weekly (Mondays, 12:00 UTC) and on-demand via `workflow_dispatch`. It:

1. Builds syllago.
2. Runs `./syllago moat trust status --json`.
3. On non-zero exit (status ≠ `fresh`), opens or updates a GitHub issue titled **"MOAT trusted root refresh due"** with the current age and cliff date.

The issue is the single source of truth — dedup is by title, so a rolling set of warns/escalated updates the same issue rather than spawning new ones.

**When the issue appears, follow the procedure below.**

---

## Refresh procedure

### 1. Fetch the new trusted root

Sigstore publishes the trusted root via TUF at `tuf-repo-cdn.sigstore.dev`. The easiest verified path is the `sigstore-go` companion repo, which commits a TUF-fetched `trusted_root.json` alongside each release tag.

```bash
# Option A: grab the file committed to sigstore-go at a specific tag
git -C /tmp clone --depth 1 --branch v1.1.4 https://github.com/sigstore/sigstore-go.git
cp /tmp/sigstore-go/examples/trusted-root/trusted_root.json /tmp/new-trusted-root.json

# Option B: TUF fetch with go-tuf (more direct; no third-party repo in the chain)
go run github.com/sigstore/sigstore-go/cmd/sigstore-go-fetch-trust-root@latest \
  > /tmp/new-trusted-root.json
```

Pick whichever method your release-process policy allows. Option A defers trust to whatever tag `sigstore-go` signed; option B defers trust to whatever the TUF client validates against the Sigstore root.

### 2. Sanity-check the file

```bash
# JSON parses
jq . /tmp/new-trusted-root.json > /dev/null

# Has the expected shape
jq -r '.mediaType' /tmp/new-trusted-root.json
# → application/vnd.dev.sigstore.trustedroot+json;version=0.1

# Fulcio CA chain is present
jq '.certificateAuthorities | length' /tmp/new-trusted-root.json
# → non-zero
```

### 3. Replace the bundled asset and the issued-at constant

```bash
cp /tmp/new-trusted-root.json cli/internal/moat/trusted_root.json
```

Then edit `cli/internal/moat/trusted_root_loader.go` and update the constant to today's UTC date:

```go
const TrustedRootIssuedAtISO = "YYYY-MM-DD"
```

The constant and the file **must** be updated in the same commit. The staleness math reads the constant, not the file's mtime (mtime is not preserved through git), so drift between them is a process bug that will mis-classify freshness.

### 4. Run the test suite

```bash
cd cli && make test
```

Key tests to watch for:

- `TestBundledTrustedRoot_*` — asserts staleness math returns `fresh` with the refreshed constant.
- `TestVerifyManifest_*` — exercises the full verify path against the fixture bundles. If Sigstore rotated Fulcio CAs since the fixtures were captured, these will fail and the fixtures need regenerating (separate workflow, not usually needed on a routine refresh).

### 5. Verify the new root outlasts the 365-day cliff

Open the new `trusted_root.json` and locate the Fulcio CA with the latest `validFor.end`. The cliff at `issued_at + 365d` must fall before that date — otherwise the staleness policy would allow operators to use a cert chain past its own validity. In practice the Sigstore Fulcio CAs are issued for ~10 years so this almost always passes, but confirm:

```bash
jq -r '.certificateAuthorities[].validFor.end' cli/internal/moat/trusted_root.json | sort | head -1
# Compare against issued_at + 365 days.
```

If a CA expires within the 365-day window, shorten the staleness cliff in `cli/internal/moat/trusted_root_loader.go` (`TrustedRootEscalatedDays`) and update ADR 0007.

### 6. Commit, PR, merge

```bash
git checkout -b refresh/moat-trusted-root-$(date -u +%Y-%m-%d)
git add cli/internal/moat/trusted_root.json cli/internal/moat/trusted_root_loader.go
git commit -m "chore(moat): refresh bundled Sigstore trusted root ($(date -u +%Y-%m-%d))"
gh pr create --fill --label "moat,trust-root-refresh"
```

PR checklist:

- [ ] `trusted_root.json` replaced.
- [ ] `TrustedRootIssuedAtISO` bumped.
- [ ] `cli && make test` passes locally.
- [ ] CI green.

### 7. Cut a patch release

The refreshed root only reaches end-users when a new binary ships. Bump the patch version and cut a release per the existing release workflow.

Release-notes template line:

> **MOAT:** Refreshed the bundled Sigstore trusted root (issued YYYY-MM-DD). Existing signed registries continue to verify; operators on older binaries should upgrade before YYYY+1-MM-DD to avoid the 365-day cliff.

### 8. Close the refresh-due issue

```bash
gh issue close <n> --reason completed --comment "Refreshed in <release-tag>."
```

The next weekly workflow run will confirm status is back to `fresh`.

---

## Escape hatches

**The Sigstore TUF repo is unavailable.** The refresh can be deferred as long as the bundled root is still within the 365-day window. Escalated status (180–364 days) means verification still runs with a louder warning. Operators with an internal Sigstore deployment can always supply their own root via `syllago add --trusted-root <path>` or the per-registry `trusted_root` config key — see [MOAT registry signing-identity docs](https://openscribbler.github.io/syllago-docs/moat/registry-add-signing-identity/).

**We missed the cliff.** End-users will see `MOAT_005 — trusted root expired`. Remediation is the same as a routine refresh, just with urgency. Communicate via the syllago release channel that upgrading unblocks verification.

**A refreshed root breaks verification for an existing registry.** The Fulcio CA chain rotated but a publisher's signing cert was issued under the previous CA. This is expected during a rotation window — publishers re-sign their manifests and republish. Document this in the release notes so operators know why a recently-working registry went `invalid`.

---

## Anti-patterns — things NOT to do

- **Don't update only the constant without refreshing the file** (or vice versa). The staleness math will lie.
- **Don't bypass the 365-day cliff by raising the constant.** The cliff protects against silent verification failures after a key rotation; extending it without a real fresh root trades security for convenience.
- **Don't fetch `trusted_root.json` from a third-party mirror.** Use the Sigstore TUF repo or a project (`sigstore-go`) that does.
- **Don't `.gitignore` the staleness check workflow.** The refresh-due issue is our memory — no issue = no reminder = missed cliff.
