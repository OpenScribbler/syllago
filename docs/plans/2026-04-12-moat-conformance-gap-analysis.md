# MOAT Conformance Gap Analysis — Syllago + syllago-meta-registry

**Date:** 2026-04-12
**MOAT Spec Version:** 0.5.3 (Draft)
**Syllago Version:** current main branch
**Scope:** Gap analysis for making Syllago a conforming MOAT client and syllago-meta-registry a conforming self-publishing registry

---

## Executive Summary

Syllago was built before MOAT existed. Its registry system uses git clones with YAML manifests (`registry.yaml`), per-file SHA-256 hashing, and mechanism-level install tracking (`installed.json`). MOAT defines a fundamentally different model: signed JSON manifests served at stable URLs, directory-level content hashing with text normalization, trust tiers backed by Sigstore/Rekor transparency logs, lockfiles with attestation bundles, and revocation enforcement.

The gap is **structural but not adversarial** — Syllago's architecture can be extended to support MOAT as a new registry type alongside existing git registries. No existing functionality needs to be torn out.

**Key numbers:**
- **66 normative client requirements** extracted from the MOAT spec (42 MUST, 9 MUST NOT, 7 SHOULD, 1 SHOULD NOT, 7 MAY)
- **12 major gaps** in Syllago's implementation
- **10 gaps** in syllago-meta-registry
- **3 spec inconsistencies** found in MOAT itself that need resolution
- **1 content type** (loadouts) excluded from MOAT attestation — Syllago-specific type

---

## Part 1: Spec Feedback — Issues Found in MOAT

These issues should be resolved in the MOAT spec before or alongside the Syllago implementation.

### SF-1: Test Vector Manifest Format Diverges from Normative Reference

**Severity:** Critical — test vectors produce different hashes than `moat_hash.py`

The normative reference (`reference/moat_hash.py` line 191) uses sha256sum format:
```python
manifest = "".join(f"{h}  {p}\n" for p, h in entries).encode("utf-8")
# Format: "<hash>  <path>\n"  (two spaces, hash first)
```

The test vector generator (`reference/generate_test_vectors.py` lines 50-53) uses a different format:
```python
concat += path.encode("utf-8") + b"\x00" + file_hash.encode("utf-8") + b"\n"
# Format: "<path>\x00<hash>\n"  (NUL-separated, path first)
```

These produce different final hashes for the same directory content. A conforming implementation cannot pass both — they contradict each other. The spec states "Conforming implementations in any language MUST produce identical output to this script" referring to `moat_hash.py`, making that the authoritative format. The test vectors need to be regenerated using the sha256sum format.

**Recommendation:** Update `generate_test_vectors.py` to use `"{h}  {p}\n"` format matching `moat_hash.py`, then regenerate all test vector expected values.

### SF-2: Symlink Handling Contradiction Between Reference and Test Vectors

**Severity:** Moderate — test vectors describe behavior the normative reference doesn't support

- `moat_hash.py` (lines 175-176): Rejects ALL symlinks with `ValueError` — no resolution, no exclusion, hard error.
- `moat-verify.md` (line 79): Confirms symlinks cause exit code 2 failure.
- `publisher-action.md` (line 15): Symlinks "skip the item with a logged warning" (item-level, not file-level).
- TV-09: Describes internal symlinks as "resolved: symlink's path with target's content" — this resolution behavior doesn't exist in `moat_hash.py`.
- TV-10: Describes external symlinks as "excluded" — `moat_hash.py` doesn't exclude, it errors.

The normative reference is clear (reject all symlinks), but the test vectors describe a resolve/exclude model that contradicts it.

**Recommendation:** Update TV-09 and TV-10 to test that symlinks produce errors, not resolved/excluded results. Or, if the spec intends to support resolved internal symlinks, update `moat_hash.py` to match.

### SF-3: Text Normalization Not Covered by Test Vectors

**Severity:** Low — coverage gap, not contradiction

The test vector generator operates on raw `dict[str, bytes]` — it never touches the filesystem, so text/binary classification, BOM stripping, and CRLF normalization are never exercised. A conforming implementation could pass all test vectors while getting text normalization wrong.

**Recommendation:** Add test vectors that explicitly test:
- A `.md` file with CRLF line endings (hash should match LF-only version)
- A `.py` file with UTF-8 BOM (hash should match BOM-stripped version)
- A `.gitignore` file (extensionless dotfile — should be treated as binary)
- A `.json` file with NUL byte in first 8KB (binary override despite text extension)

### SF-4: Loadouts Excluded from MOAT Attestation

**Decision:** Loadouts are a Syllago-specific content type (curated bundles for a specific provider). They will NOT be proposed as a MOAT content type.

Loadouts remain distributed via Syllago's own `registry.yaml` mechanism without MOAT attestation. The syllago-meta-registry's 12 loadout items will not appear in the MOAT manifest. This is correct — MOAT is a community protocol for universal content types; loadouts are a Syllago implementation detail.

**Impact:** The `moat.yml` content discovery override in the meta-registry only needs to map `agents/` items. Loadouts are not listed in `moat.yml` at all — the Publisher Action and Registry Action simply don't see them.

### SF-5: [RESOLVED] `subagent` vs `agent` Terminology

**Observation:** The MOAT spec uses `subagent` as the normative content type name and `subagents/` as the canonical directory. Syllago uses `agents`/`agents/` throughout.

**Resolution:** Keep MOAT's `subagent` terminology as-is. After researching industry terminology (12/14 AI coding tools self-identify as "agents"), the naming makes sense: the tools ARE agents ("AI Agent Runtimes" in MOAT terms), and the content type creates sub-agents within them. Both are agents — the qualifier distinguishes them.

**Actions:**
1. Strengthen "AI Agent Runtime" as a prominent defined term throughout the MOAT spec
2. Add clear terminology callout explaining the runtime/subagent distinction
3. Syllago uses boundary mapping at the MOAT manifest layer (`typemap.go`) — maps `agents` ↔ `subagent` at protocol boundary
4. Syllago's "provider" → something else rename is a separate future workstream (tracked as a bead)

---

## Part 2: Gap Inventory — Syllago as Conforming Client

### G-1: Content Hashing Algorithm [BREAKING — Must Implement from Scratch]

**MOAT requires:** Directory-level hash with text normalization (BOM strip, CRLF→LF), text/binary classification by extension + NUL-byte probe, VCS directory exclusions, symlink rejection, NFC Unicode path normalization, posix-style paths, global UTF-8 byte sort, sha256sum manifest format, `sha256:<hex>` output.

**Syllago has:** Per-file `HashFile()`/`HashBytes()` in `installer/integrity.go` — raw SHA-256 of file bytes, bare hex output. No text normalization, no directory-level hash, no NFC normalization, no symlink rejection.

**Gap:** Complete reimplementation needed. None of Syllago's existing hash functions produce MOAT-conforming hashes.

**Architecture:** New package `internal/moat/hash.go`. Port `moat_hash.py` to Go. Must pass all MOAT test vectors (once SF-1 is resolved). Existing per-file hashing in `installer/integrity.go` remains for its own purpose (drift detection).

**Key implementation details:**
- Go's `filepath.WalkDir` sorts per-directory, not globally — must collect all entries then sort
- Go has no stdlib NFC normalizer — need `golang.org/x/text/unicode/norm`
- Text normalization must handle CR at chunk boundaries (streaming impl)
- Must convert `filepath.Separator` to `/` (Windows)

### G-2: Registry Manifest Format [MAJOR — New Registry Type]

**MOAT requires:** Signed JSON document (`registry.json`) with `schema_version`, `manifest_uri`, `name`, `operator`, `updated_at`, `registry_signing_profile`, `content[]`, `revocations[]`. Served at a stable URL, not git-cloned.

**Syllago has:** YAML `registry.yaml` with `name`, `description`, `version`, `maintainers`, `items[]`. Distributed via git clone.

**Gap:** Completely different format, transport, and trust model.

**Architecture:** Dual registry type support:
- Existing git registries continue working via `registry.Clone()`/`registry.Sync()` with `registry.yaml`
- New MOAT registries use HTTP fetch of `registry.json` + `.sigstore` bundle
- Detection: config-level `type` field (`"git"` or `"moat"`) set at `registry add` time
- The `config.Registry` struct gains MOAT-specific fields (see G-4)

### G-3: Manifest Signing and Verification [MAJOR — No Implementation Exists]

**MOAT requires:** Verify registry manifest using cosign keyless OIDC signing. Fetch bundle at `{manifest_uri}.sigstore`. Verify bundle covers exact manifest bytes, OIDC issuer/subject match `registry_signing_profile`, Rekor transparency log entry is valid. Rekor unavailability is a hard failure.

**Syllago has:** Hidden stub commands `sign`/`verify` returning "not yet implemented." An unimplemented `signing` package with interface definitions.

**Gap:** No Sigstore/cosign integration. No Rekor client. No OIDC verification.

**Architecture:** Two options:
1. **Shell out to `cosign` CLI** — simpler, requires cosign installed. Good for MVP.
2. **Use `sigstore-go` library** — no external dependency, better for production.

Recommendation: Use `sigstore-go` from the start. It's the mature Go library for this. The existing `signing` package interfaces should be evaluated for compatibility but will likely need redesign (they're per-hook, not per-manifest/per-item).

### G-4: Registry Trust Bootstrap & Signing Profile Tracking [MAJOR]

**MOAT requires (6 requirements):**
- Explicit End User action to add trusted registry
- Track `registry_signing_profile` per registry; changes require re-approval
- TOFU for manually-added registries (trust signing profile on first fetch)
- Display label changes (`name`, `operator`) MUST NOT trigger re-approval
- Support for Registry Index discovery (configurable, not authoritative)
- Signing profile match for index-discovered registries

**Syllago has:** `config.Registry` with `Name`, `URL`, `Ref`, `Trust` (user label), `Visibility`, `VisibilityCheckedAt`. The `Trust` field is a static string, not a cryptographic identity.

**Gap:** No signing profile tracking, no TOFU, no re-approval flow, no index support.

**Config schema changes needed:**
```go
type Registry struct {
    Name                string          `json:"name"`
    URL                 string          `json:"url"`
    Ref                 string          `json:"ref,omitempty"`
    Type                string          `json:"type,omitempty"`                // "git" or "moat"
    Trust               string          `json:"trust,omitempty"`               // legacy user label
    Visibility          string          `json:"visibility,omitempty"`
    VisibilityCheckedAt *time.Time      `json:"visibility_checked_at,omitempty"`
    // New MOAT fields:
    ManifestURI         string          `json:"manifest_uri,omitempty"`
    SigningProfile      *SigningProfile  `json:"signing_profile,omitempty"`
    LastFetchedAt       *time.Time      `json:"last_fetched_at,omitempty"`     // staleness tracking
}

type SigningProfile struct {
    Issuer  string `json:"issuer"`
    Subject string `json:"subject"`
}
```

### G-5: Per-Item Rekor Attestation Verification [MAJOR]

**MOAT requires:** For Signed/Dual-Attested items, verify per-item Rekor entry on install. Reconstruct canonical payload `{"_version":1,"content_hash":"sha256:<hex>"}`, verify signature against certificate, confirm OIDC identity matches signing profile. Unknown `_version` values must fail verification.

**Syllago has:** Nothing — no per-item attestation concept.

**Architecture:** New verification pipeline in `internal/moat/verify.go`:
1. On install from MOAT registry: read `rekor_log_index` from manifest entry
2. Fetch Rekor entry at that index
3. Reconstruct canonical payload from `content_hash`
4. Verify signature covers payload
5. Confirm OIDC identity matches expected signing profile

### G-6: Trust Tier Determination and Display [MODERATE]

**MOAT requires:** Three normative tiers (Dual-Attested, Signed, Unsigned). Determined by manifest entry fields (`rekor_log_index` for Signed, `signing_profile` for Dual-Attested). Client must surface tier before install confirmation.

**Syllago has:** Static `Trust` string in registry config — user-assigned label, not computed from attestation data.

**Gap:** Trust is user-assigned, not attestation-derived. No per-item trust tier. No UI for displaying trust tier at install time.

**Architecture:**
- Add `TrustTier` field to `catalog.ContentItem` for MOAT-sourced content
- Compute tier from manifest entry fields during catalog scan
- Update install wizard TUI to display tier before confirmation
- CLI install flow to surface tier in non-interactive mode

### G-7: MOAT Lockfile [MAJOR — New File]

**MOAT requires:** Lockfile with `moat_lockfile_version`, `entries[]` (each with `name`, `type`, `registry`, `content_hash`, `trust_tier`, `attested_at`, `pinned_at`, `attestation_bundle`, `signed_payload`), and `revoked_hashes[]`. Must store full cosign bundle for offline re-verification. Must verify `sha256(signed_payload) == data.hash.value` in Rekor entry before writing. Lockfile must be interoperable across conforming clients.

**Syllago has:** `installed.json` with three separate arrays (`hooks`, `mcp`, `symlinks`) tracking mechanism-level state for uninstall. Different structure, different purpose.

**Gap:** `installed.json` cannot evolve into a MOAT lockfile — they serve fundamentally different purposes:
- `installed.json` = "what did Syllago put where, so it can undo it" (mechanism tracking)
- MOAT lockfile = "what content is installed with what trust properties" (trust ledger)

**Architecture:** New file `.syllago/moat-lockfile.json` coexisting with `installed.json`. Install flow writes to BOTH: `installed.json` for operational tracking, lockfile for trust state.

### G-8: Revocation Handling [MAJOR — No Mechanism Exists]

**MOAT requires (20 requirements — the largest category):**
- Registry revocations: hard-block (refuse to load/execute)
- Publisher revocations: warn-once-per-session, allow with explicit confirmation
- Check installed content against `revocations` array on every manifest sync
- Non-zero exit code when revoked content encountered
- Surface revocation source, reason, `details_url`, and issuing registry
- Unknown reason values accepted without error
- `revoked_hashes` in lockfile not silently removable

**Syllago has:** `RevocationEntry`/`RevocationList` type definitions in the unimplemented `signing` package — designed for per-hook revocation, not MOAT's per-content-hash model.

**Architecture:** New module `internal/moat/revocation.go`. Integration points:
1. **`registry sync`** — after fetching updated manifest, diff revocations against lockfile
2. **Install flow** — check `revoked_hashes` before installing
3. **`CheckStatus()`** — add `StatusRevoked` for TUI/CLI display
4. **Session-level warning tracking** — publisher revocations warn once per session

### G-9: Manifest Freshness / Staleness Threshold [MODERATE]

**MOAT requires:** 24-hour staleness threshold computed against client's last-fetch timestamp. Must sync manifest before revocation checks if stale. Should not allow threshold above 48 hours. If `expires_at` is present, reject manifests past that time.

**Syllago has:** No freshness tracking. `registry.Sync()` is user-triggered only.

**Gap:** No `last_fetched_at` timestamp per registry. No auto-sync behavior.

**Architecture:**
- Add `LastFetchedAt` to `config.Registry` (see G-4)
- Before any MOAT operation (install, status check): check staleness
- If stale: auto-fetch MOAT manifest (HTTP is fast). For git registries: prompt or auto-pull
- Check `expires_at` field when present

**Behavioral change:** Currently `syllago install` never touches the network for git registries. With MOAT staleness, MOAT-type registries may auto-fetch. This only affects MOAT registries — git registries retain current behavior.

### G-10: Private Content Isolation [SMALL — Syllago is Ahead]

**MOAT requires:** Must not auto-submit private content to public registries. Must surface visibility and require confirmation.

**Syllago has:** Multi-gate privacy system (G1-G4) with fail-closed visibility probing. Stronger than MOAT's requirements.

**Gap:** Minor — need per-item `private_repo` tracking when consuming MOAT manifests (currently tracked at registry level, not item level).

### G-11: Registry Index Support [MODERATE — New Feature]

**MOAT requires:** Support configurable registry index sources. Surface which indices are used. Index does not bypass per-registry trust. Users must be able to add registries by direct URL without an index.

**Syllago has:** `KnownAliases` map with hardcoded alias expansion. Not an index.

**Gap:** No index fetching, parsing, or signed index verification. Not blocking for initial conformance — Syllago can support direct-URL registry addition first and add index support later.

### G-12: Content Type Alignment [MODERATE — Rename `agents` to `subagent`]

**MOAT normative types:** `skill`, `subagent`, `rules`, `command`. Deferred: `hook`, `mcp`.

**Syllago types:** `skills`, `agents`, `rules`, `commands`, `hooks`, `mcp`, `loadouts`.

**Gaps:**
- `agents` → `subagent` (per decision: align to spec)
- `loadouts` is Syllago-specific — excluded from MOAT attestation entirely
- `hooks` and `mcp` are deferred in MOAT but active in Syllago
- Syllago uses plural forms, MOAT uses singular for the type string

**Architecture:** For MOAT manifest consumption, map between Syllago's internal type names and MOAT's type strings at the boundary. The `agents → subagent` rename is a separate, deeper refactor.

---

## Part 3: Gap Inventory — syllago-meta-registry

### MR-1: No GitHub Actions Workflows

**Needs:** Two workflow files:
- `.github/workflows/moat.yml` — Publisher Action (based on `reference/moat.yml`)
- `.github/workflows/moat-registry.yml` — Registry Action (based on `reference/moat-registry.yml`)

**Complexity:** Moderate — reference templates are drop-in with minimal customization.

### MR-2: No `registry.yml` (MOAT Registry Config)

**Needs:** MOAT registry config at repo root (NOT the existing `registry.yaml` which is Syllago's format):

```yaml
schema_version: 1
registry:
  name: syllago-meta-registry
  operator: OpenScribbler
  manifest_uri: https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/moat-registry/registry.json
sources:
  - uri: https://github.com/OpenScribbler/syllago-meta-registry
revocations: []
```

**Note:** `registry.yaml` (Syllago format) and `registry.yml` (MOAT format) coexist — different files, different purposes, different extensions.

### MR-3: No `moat.yml` (Content Discovery Override)

**Needs:** `moat.yml` at repo root for non-standard content mapping:

```yaml
items:
  # Map agents/ to MOAT's subagent type
  - name: syllago-author
    type: subagent
    path: agents/syllago-author
  # NOTE: loadouts are Syllago-specific and excluded from MOAT attestation.
  # They are NOT listed here — distributed via registry.yaml only.
```

**Why required:** The meta-registry uses `agents/` (not `subagents/`). Without `moat.yml`, the Publisher Action won't discover agent content. (Loadouts are excluded from MOAT attestation — Syllago-specific type.)

### MR-4: `agents/` Directory Name Mismatch

**Current:** `agents/syllago-author/`
**MOAT expects:** `subagents/syllago-author/`
**Fix:** Use `moat.yml` mapping (MR-3) or rename directory. Using `moat.yml` is cleaner since it doesn't require Syllago to understand `subagents/` yet.

### MR-5: Loadout Nested Structure

**Current:** `loadouts/<provider>/<name>/` (two levels deep)
**MOAT expects:** `<category>/<item_name>/` (one level deep)
**Not applicable:** Loadouts are excluded from MOAT — they remain Syllago-native content distributed via `registry.yaml`.

### MR-6: Self-Publishing OIDC Identity

**Needs:** Two distinct OIDC subjects (automatically satisfied by having two separate workflow files):
- Publisher: `.../.github/workflows/moat.yml@refs/heads/main`
- Registry: `.../.github/workflows/moat-registry.yml@refs/heads/main`

**Complexity:** Trivial — handled automatically by Sigstore.

### MR-7: `moat-attestation` and `moat-registry` Branches

**Needs:** Orphan branches created automatically on first workflow run.
**Complexity:** Trivial.

### MR-8: Skills Directory Alignment

**Current:** `skills/` with 5 items — matches MOAT's canonical `skills/` directory.
**Status:** Already aligned. No changes needed.

### MR-9: Empty Directories

**Current:** `commands/`, `rules/`, `hooks/`, `mcp/` exist but are empty.
**Impact:** Publisher Action will scan them and find nothing — no error, no items. Fine.

### MR-10: `self_published: true` Disclosure

**Needs:** Manifest must disclose self-publishing since publisher + registry are the same repo.
**Complexity:** Trivial — auto-detected by the Registry Action.

---

## Part 4: Architecture Decisions

### AD-1: Dual Registry Support Model

MOAT registries and git registries coexist as two `type` variants in `config.Registry`. The existing git clone/sync code path is untouched. A new code path handles MOAT manifest fetching, verification, and lockfile management.

**Detection heuristic at `registry add`:**
- User provides a manifest URL → MOAT registry
- User provides a git URL → git registry
- CLI flag `--moat` could force MOAT mode for ambiguous URLs

### AD-2: New `internal/moat/` Package

All MOAT-specific logic lives in a new package rather than modifying existing packages:

| File | Purpose |
|------|---------|
| `hash.go` | Content hashing algorithm (Go port of `moat_hash.py`) |
| `manifest.go` | MOAT manifest types, parsing, and verification |
| `verify.go` | Sigstore signature + Rekor entry verification |
| `lockfile.go` | MOAT lockfile management |
| `revocation.go` | Revocation checking and enforcement |
| `index.go` | Registry Index support (Phase 5) |

### AD-3: `sigstore-go` Over CLI Shelling

Use `sigstore/sigstore-go` library rather than shelling out to `cosign`. Rationale:
- No runtime dependency on cosign being installed
- Better error handling and integration
- Go-native, fits the codebase
- The library is mature and used by other Go tools

### AD-4: Lockfile Coexistence

`.syllago/moat-lockfile.json` is a new file alongside `installed.json`. The install flow writes to both. This avoids any migration complexity and lets each file serve its distinct purpose cleanly.

### AD-5: Content Type Boundary Mapping

**MOAT uses `subagent`; Syllago uses `agents`.** Both are correct in their own context. The MOAT spec defines protocol terms; implementations use their own internal terminology. Syllago maps between internal type names and MOAT type strings at the MOAT manifest boundary via `internal/moat/typemap.go`:

| Syllago Internal | MOAT Type | Notes |
|-----------------|-----------|-------|
| `skills` | `skill` | Plural → singular |
| `agents` | `subagent` | Different term entirely |
| `rules` | `rules` | Same |
| `commands` | `command` | Plural → singular |
| `hooks` | `hook` | Deferred in MOAT |
| `mcp` | `mcp` | Deferred in MOAT |
| `loadouts` | _(excluded)_ | Syllago-specific — not in MOAT, not mapped |

---

## Part 5: Syllago Implementation Details

### AD-6: `RegistryClient` Interface (Panel C8)

Prevent git/MOAT conditionals from spreading through the codebase. Every registry operation goes through a common interface:

```go
// internal/registry/client.go
type RegistryClient interface {
    // Sync updates the local state from the remote registry.
    // For git: git pull --ff-only. For MOAT: HTTP fetch manifest + verify.
    Sync(ctx context.Context) error

    // Items returns all content items from this registry.
    Items() []catalog.ContentItem

    // FetchContent downloads content for a specific item.
    // For git: already local (cloned). For MOAT: fetch from source_uri.
    FetchContent(ctx context.Context, item catalog.ContentItem, dest string) error

    // Type returns "git" or "moat".
    Type() string

    // Trust returns trust metadata for the registry.
    // For git: nil (no MOAT trust). For MOAT: signing profile, last verified, etc.
    Trust() *TrustMetadata
}
```

`GitClient` wraps existing `registry.Clone()`/`registry.Sync()` code. `MOATClient` is the new implementation. The existing `registry` package functions become internal to `GitClient`; the public API moves to the interface.

### AD-7: User-Facing Trust Display (Panel C9)

**Three-state collapsed model:**

| Internal State | User-Facing Label | Badge | When Shown |
|---------------|-------------------|-------|------------|
| Dual-Attested | Verified | green checkmark | MOAT registry, verification passed |
| Signed | Verified | green checkmark | MOAT registry, verification passed |
| Unsigned / git registry | _(no badge)_ | _(none)_ | Git registries show no trust badge — absence is not a negative signal |
| Revoked (registry) | Recalled | red X | Hard-blocked content, cannot install |
| Revoked (publisher) | Recalled | yellow warning | Warn-on-use, user can confirm to proceed |

**Detailed tier on drill-down:** When a user inspects an item (metadata panel, `syllago show`), display the full tier:
- "Verified (dual-attested by publisher and registry)"
- "Verified (registry-attested)"
- "Recalled by registry — [reason]"

**"MOAT" is invisible in normal use.** Users see "Verified" or "Recalled." The word "MOAT" appears only in: developer docs, `syllago doctor` diagnostic output, and the `moat verify` CLI command.

### AD-8: Error Message Templates

Every error a user can encounter needs to be actionable:

**Registry operations:**
- Manifest fetch failed: `Could not reach registry [name]. Check your network connection. (URL: [url], error: [code])`
- Signature verification failed: `Registry [name] signature could not be verified. The manifest may have been tampered with. Run 'syllago registry sync --force' to re-fetch, or remove the registry with 'syllago registry remove [name]'.`
- Signing profile changed: `Registry [name] has a new signing identity. This could indicate the registry has been transferred to a new owner. Approve the new identity? [Y/n] (Previous: [old], New: [new])`

**Install operations:**
- Content hash mismatch: `Content hash mismatch for [item] from [registry]. Expected [hash], got [computed]. The content may have been modified after attestation. Installation blocked.`
- Revoked (registry): `[item] has been recalled by [registry]. Reason: [reason]. Installation blocked. [details_url if present]`
- Revoked (publisher): `[item] has a publisher advisory from [registry]. Reason: [reason]. Install anyway? [Y/n] [details_url if present]`

**Cosign/Rekor errors wrapped — never raw output:**
- Rekor unreachable: `Could not verify [item] attestation — transparency log is unreachable. This is a temporary infrastructure issue. Try again later. (exit code 3)`
- NOT: raw cosign stderr like `error: posting to rekor: http status 503`

### AD-9: Non-Interactive Behavior

Every interactive prompt has a defined non-interactive fallback for CI/CD:

| Operation | Interactive | Non-Interactive (`--non-interactive` or no TTY) |
|-----------|------------|------------------------------------------------|
| TOFU signing profile | Prompt Y/n | Accept on first add (TOFU semantics); reject on change (exit 1) |
| Signing profile change | Prompt with old/new identity | Exit 1 with message: "signing identity changed, re-approve with `syllago registry approve [name]`" |
| Publisher revocation | Prompt Y/n per item | Exit 1 with revocation details on stderr |
| Staleness exceeded | Warning + auto-sync | Warning on stderr + auto-sync attempt; if network fails, exit 1 |

### AD-10: Security Contract Per Phase

What Syllago can honestly promise at each phase boundary:

**After Phase 0 (meta-registry bootstrap):**
- Promise: "The syllago-meta-registry produces signed MOAT manifests."
- NOT a promise: Anything about Syllago's client behavior.

**After Phase 1 (content hashing):**
- Promise: "Syllago can compute content hashes interoperable with the MOAT ecosystem."
- NOT a promise: Any claim about content authenticity, integrity, or trust.
- User-visible change: None. Hashing is internal plumbing.

**After Phase 2 (the big merge: manifest + signing + revocation + lockfile):**
- Promise: "Content from MOAT registries is cryptographically verified before installation. Recalled content is blocked. Trust tier is accurate."
- NOT a promise: Offline verification (requires lockfile maturity). Registry Index discovery.
- User-visible change: MOAT registries show Verified/Recalled badges. Git registries unchanged.

**After Phase 3 (polish + index):**
- Promise: Full MOAT conformance including SHOULD/MAY requirements.
- User-visible change: Registry Index discovery, offline verification, `self_published` surfacing.

---

## Part 6: Phased Implementation Plan (Post-Panel Revision)

### Phase 0: Spec Fixes + Meta-Registry Bootstrap

**Goal:** Fix MOAT spec blockers. Bootstrap meta-registry as the first self-publishing MOAT registry.

**Tasks:**
1. Fix SF-1 in MOAT spec — update `generate_test_vectors.py` to sha256sum format, regenerate all expected values
2. Fix SF-2 — update TV-09/TV-10 symlink test vectors to test error behavior
3. Add text normalization test vectors (SF-3)
4. Strengthen "AI Agent Runtime" terminology throughout MOAT spec; add terminology callout
5. Create `moat.yml` in meta-registry — content discovery mapping
6. Create `registry.yml` in meta-registry — MOAT registry config
7. Add `.github/workflows/moat.yml` — Publisher Action
8. Add `.github/workflows/moat-registry.yml` — Registry Action
9. Validate: `moat-attestation` branch, `moat-registry` branch, `self_published: true`, dual OIDC subjects

**Output:** Working MOAT registry producing signed manifests. Fixed spec with consistent test vectors.

### Phase 1: Content Hashing

**Goal:** Go implementation of MOAT `content_hash` that passes all test vectors.

**Tasks:**
1. Create `cli/internal/moat/hash.go` — port of `moat_hash.py`
2. Add `golang.org/x/text/unicode/norm` dependency
3. Create `cli/internal/moat/hash_test.go` — all test vectors as table-driven tests
4. Create `cli/internal/moat/testdata/` — filesystem fixtures for text normalization, symlinks, VCS dirs
5. Cross-language validation: hash fixtures with both Go implementation and `moat_hash.py`, assert identical output
6. Define and implement `RegistryClient` interface (AD-6) — wrap existing git code in `GitClient`

**Output:** `moat.ContentHash(dir string) (string, error)` producing `sha256:<hex>` identical to `moat_hash.py`. `RegistryClient` interface ready for `MOATClient`.

**Can run in parallel with Phase 0** — hash implementation validates against `moat_hash.py` directly, not against the meta-registry.

### Phase 2: Manifest + Signing + Revocation + Lockfile (The Big Merge)

**Goal:** Full conforming MOAT client. Trust tiers displayed ONLY after verification works.

This phase merges the original Phases 2-4 because the panel unanimously agreed (C2) that showing trust signals without verification behind them is worse than showing nothing.

**Week 1 — Sigstore spike:**
- Verify one real Rekor entry from the meta-registry's Phase 0 Publisher Action output using `sigstore-go`
- If this works, the integration path is validated
- If it doesn't, we learn before investing in the full implementation
- This is the highest-risk task in the entire plan — de-risk it first

**Manifest + config (after spike validates):**
1. Create `cli/internal/moat/manifest.go` — MOAT manifest types + JSON parsing
2. Create `cli/internal/moat/fetch.go` — HTTP fetch with ETag/If-None-Match (C7), User-Agent header
3. Extend `config.Registry` with MOAT fields (ManifestURI, SigningProfile, LastFetchedAt, Type)
4. Implement `MOATClient` conforming to `RegistryClient` interface
5. Add `registry add` flow for MOAT manifest URLs
6. Add `registry sync` flow for MOAT registries (manifest fetch + signature verify)
7. Content type boundary mapping (`typemap.go` — AD-5)

**Signature verification:**
8. Implement manifest signature verification using `sigstore-go`
9. Implement per-item Rekor entry verification
10. Implement TOFU signing profile tracking and storage
11. Implement re-approval flow when signing profile changes
12. Implement non-interactive fallbacks (AD-9)

**Revocation:**
13. Implement revocation checking against manifest `revocations[]` on sync
14. Implement hard-block for registry revocations
15. Implement warn-on-use for publisher revocations (once per session)
16. Implement `revoked_hashes` in lockfile (append-only without user action)

**Lockfile:**
17. Create `cli/internal/moat/lockfile.go` — types, read/write
18. Populate lockfile on install (all required fields)
19. Store attestation bundle + signed payload at install time
20. Implement `sha256(signed_payload)` → Rekor `data.hash.value` check before writing
21. Implement staleness threshold (24h + jitter per C7)

**Trust display (ONLY after verification works):**
22. Add trust tier to `catalog.ContentItem` for MOAT-sourced content
23. Update TUI metadata panel with Verified/Recalled badges (AD-7)
24. Update CLI output with trust indicators
25. Update install wizard to surface trust tier before confirmation

**Verification command:**
26. Replace `syllago verify` stub with real implementation (online mode: hash + manifest + Rekor)

**Testing:**
27. Fixture-based unit tests using real `.sigstore` bundles from meta-registry
28. Integration tests gated behind `SYLLAGO_TEST_SIGSTORE=1`
29. End-to-end test: `registry add` meta-registry → `install` a skill → verify trust tier → revocation check

**Output:** Syllago is a conforming MOAT client. Users see Verified/Recalled badges backed by real cryptographic verification.

### Phase 3: Polish + Index

**Goal:** Full spec coverage including SHOULD/MAY requirements.

**Tasks:**
1. Registry Index fetching, parsing, signed verification
2. Index configuration (add/remove/replace)
3. `expires_at` manifest rejection
4. Offline verification mode using lockfile
5. `self_published` surfacing in UI
6. `syllago verify --offline` (lockfile mode)
7. Name collision handling in install wizard (display `source_uri` as disambiguator)

---

## Part 7: Requirement Coverage Matrix (Revised Phasing)

| Req# | Category | Keyword | Covered By |
|------|----------|---------|------------|
| 1 | Registry Trust | MUST | Phase 2 (registry add flow) |
| 2-5 | Signing Profile | MUST | Phase 2 (TOFU + re-approval) |
| 6 | Signing Profile | SHOULD | Phase 3 (index) |
| 7-11 | Registry Index | MUST/SHOULD | Phase 3 (index) |
| 12-17 | Manifest Verification | MUST | Phase 2 (signature verification) |
| 18 | Freshness | MUST | Phase 3 (expires_at) |
| 19-21 | Manifest Fields | MUST NOT/MAY | Phase 2 (parsing) |
| 22-26 | Content Hashing | MUST | Phase 1 (hash algorithm) |
| 27-30 | Per-Item Rekor | MUST | Phase 2 (Rekor verification) |
| 31-38 | Lockfile | MUST | Phase 2 (lockfile) |
| 39-41 | Trust Tier | MUST/SHOULD | Phase 2 (display after verification) |
| 42-62 | Revocation | MUST | Phase 2 (basic revocation + lockfile) |
| 63-65 | Private Content | MUST NOT | Phase 2 (extend existing per-item) |
| 66-67 | Runtime Boundary | SHOULD/MAY | Phase 3 (polish) |
| 68-70 | Misc | MUST/MAY | Phase 1 (test vectors) + Phase 3 (index) |

**Phase 0-2 covers all 42 MUST and 9 MUST NOT requirements.** Phase 3 covers the 7 SHOULD, 1 SHOULD NOT, and 7 MAY requirements.

---

## Part 8: External Dependencies

| Dependency | Phase | Purpose |
|-----------|-------|---------|
| `golang.org/x/text/unicode/norm` | 1 | NFC Unicode normalization |
| `sigstore/sigstore-go` | 2 | Cosign bundle verification, Rekor client |
| HTTP client (stdlib) | 2 | Manifest fetching with ETag |

### Dependency on MOAT Spec Fixes

| Spec Issue | Blocking Phase | Impact if Not Fixed |
|-----------|---------------|---------------------|
| SF-1 (test vector format) | Phase 0 (ship-blocker) | Cannot validate against test vectors; no Go code until fixed |
| SF-2 (symlink handling) | Phase 0 | Implement `moat_hash.py` behavior (reject); contradictory test vectors confuse implementors |
| SF-3 (text normalization tests) | Phase 0 | Write our own tests; important for correctness but not blocking |
| SF-4 (loadout type) | N/A | Resolved: loadouts are Syllago-specific, excluded from MOAT |
| SF-5 (terminology) | Resolved | Keep `subagent` in MOAT; strengthen "AI Agent Runtime"; boundary mapping in Syllago |

---

## Part 9: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `sigstore-go` API complexity | Medium | High | Week 1 spike in Phase 2 — verify one real Rekor entry before committing to full implementation |
| Test vector discrepancy | Fixed | N/A | Resolved in Phase 0 |
| Rekor availability in tests | Medium | Medium | Fixture-based unit tests; integration tests behind `SYLLAGO_TEST_SIGSTORE=1` |
| Content type boundary mapping | Low | Low | `typemap.go` — maps `agents` ↔ `subagent` at MOAT boundary |
| Config schema migration | Low | Low | New fields are additive/optional — backwards compatible |
| Day-one adoption signal | Medium | High | Phase 0 bootstraps meta-registry BEFORE client ships — verified content exists on day one |

---

## Appendix A: Syllago Files Requiring Modification

**Existing files:**
- `cli/internal/config/config.go` — Registry struct extension
- `cli/internal/catalog/types.go` — ContentItem trust tier field
- `cli/internal/registry/registry.go` — MOAT manifest detection path
- `cli/cmd/syllago/registry_cmd.go` — MOAT sync flow
- `cli/cmd/syllago/sign_cmd.go` — Replace stubs with real impl
- `cli/internal/tui/install.go` — Trust tier display
- `cli/internal/installer/installer.go` — MOAT verification + lockfile write

**New files:**
- `cli/internal/moat/hash.go` + test
- `cli/internal/moat/manifest.go` + test
- `cli/internal/moat/verify.go` + test
- `cli/internal/moat/lockfile.go` + test
- `cli/internal/moat/revocation.go` + test
- `cli/internal/moat/fetch.go` + test
- `cli/internal/moat/index.go` + test (Phase 5)

## Appendix B: Meta-Registry Files to Create

- `.github/workflows/moat.yml` — Publisher Action
- `.github/workflows/moat-registry.yml` — Registry Action
- `registry.yml` — MOAT registry configuration
- `moat.yml` — Content discovery override

---

## Part 9: Panel Consensus (5-Agent Review)

A five-persona panel reviewed this design doc across 3 rounds of discussion (Remy — spec/adoption reviewer, Platform Vendor — engineering lead, Enterprise Security — security architect, Solo Publisher — independent content creator, Registry Operator — community registry ops).

### Unanimous Consensus (5/5 AGREE)

| # | Item | Rationale |
|---|------|-----------|
| C1 | **SF-1 is a ship-blocker** — fix test vector format before writing Go code | The normative reference (`moat_hash.py`) and test vector generator produce different hashes. No conformance testing is possible until resolved. |
| C2 | **No trust tier display before signature verification ships** — merge original Phases 2+3 | Displaying "Signed" or "Dual-Attested" badges without verification behind them is the browser certificate lock icon mistake. Enterprise Security: "Worse than showing nothing." All five panelists converged on this after Platform Vendor withdrew initial dissent. |
| C3 | **Use boundary mapping between Syllago's `agents` and MOAT's `subagent`** | [Post-panel resolution] Keep MOAT's `subagent` — it's semantically correct when the tools themselves are agents. Syllago maps at the protocol boundary via `typemap.go`. The "provider" rename in Syllago is a separate workstream. |
| C4 | **Basic revocation checking moves into the merged phase** | Checking installed hashes against a `revocations[]` array is ~10 lines of Go. Deferring it to a separate phase creates a window where compromised content has no recall mechanism. Hard-block on registry revocations, warn-on-use for publisher revocations. |
| C5 | **This is substantial work** | Panel's traditional estimate was 10-14+ weeks for a solo dev. AI-augmented timeline: 6-10 sessions across Phases 0-3. The thinking (design decisions, sigstore-go learning curve) compresses less than the typing. |
| C6 | **Meta-registry bootstrap is Phase 0** — prerequisite, not parallel | If 99% of content is "Unverified" on day one, users learn to ignore the trust signal. Having the meta-registry producing signed manifests BEFORE the client ships creates meaningful content to verify on first use. |
| C7 | **ETag + jitter on manifest fetches are non-optional** | Registry Operator: a registry with 1,000 clients each polling at the same 24h mark creates a thundering herd. ETag (`If-None-Match`) avoids re-downloading unchanged manifests. Random jitter (0-10% of staleness window) spreads load. |
| C8 | **`RegistryClient` interface to abstract git vs MOAT** | Platform Vendor: without this, every registry operation becomes a type-switch conditional. A `RegistryClient` interface with `Sync()`, `Items()`, `FetchContent()` methods prevents git/MOAT conditionals from spreading across the codebase. |
| C9 | **Three-state user-facing trust display: Verified / Unverified / Recalled** | Collapse MOAT's Signed + Dual-Attested into "Verified" for the default view. Detailed tier available on drill-down. Git-registry content shows no badge at all (absence ≠ negative signal). "Recalled" is better language than "Revoked" for non-technical users. |
| C10 | **Publisher Action should warn about undiscovered content** | Solo Publisher: the failure mode of silent omission ("your content exists but isn't attested because you forgot to add it to `moat.yml`") is the worst kind of failure for a trust system. The action should detect directories with content-like structure that weren't matched. |

### Remaining Dissent (Minor — Not Blocking)

| Topic | Position A | Position B |
|-------|-----------|-----------|
| **48h staleness hard-fail** | Enterprise Security: MUST hard-fail — degrading to a warning is a downgrade attack vector | Platform Vendor: hard-failing because a laptop was offline for a weekend will drive users to disable MOAT entirely |
| **Lockfile HMAC** | Enterprise Security: lockfile integrity via HMAC deserves a Phase 3 design decision | Platform Vendor: over-engineering — attacker with local write access can modify installed content directly |
| **Revocation archival** | Registry Operator: ever-growing `revocations[]` array needs an archival/pruning mechanism before registries go live | Others: acknowledged but deferred — spec concern, not Syllago concern |
| **Publisher workflow removal** | Solo Publisher: what happens when a publisher deletes their MOAT workflow? Stale attestations that never update are a real lifecycle event | Others: acknowledged — spec should address this |

### Design Doc Additions Required (One Per Panelist)

1. **Remy — Adoption Sequencing Plan:** The doc covers *what to build* but not *how to ensure the trust signal is meaningful on day one*. Without bootstrapped content and early-adopter publishers before launch, users habituate to ignoring "Unverified" — and that habituation is harder to undo than any engineering mistake.

2. **Platform Vendor — Non-Interactive Behavior Spec:** Every interactive prompt (TOFU approval, signing profile change, publisher revocation confirmation) needs a defined non-interactive fallback with specific exit codes. Without this, CI/CD pipelines and enterprise fleet management cannot use MOAT-enabled Syllago.

3. **Enterprise Security — Security Contract Per Phase:** Define what Syllago can honestly promise at each phase boundary — what IS verified, what is NOT verified, and what users see. Without it, every phase ships with ambiguous security semantics and no testable acceptance criteria.

4. **Solo Publisher — Publisher Adoption Gate:** Define WHEN to publicly recommend publisher adoption — not until the merged Phase ships and an end-to-end test passes against the meta-registry. Premature publisher adoption creates support obligations before the system is ready.

5. **Registry Operator — Namespace Conflict Resolution:** Define how `content[].name` collisions within a manifest are handled. Without a uniqueness constraint or qualification scheme, name collisions at scale are inevitable and the current ambiguity forces operators to make up policy ad hoc.

### Revised Phase Plan (Post-Panel, AI-Augmented Timeline)

The panel's traditional 10-15 week estimate assumes a solo human developer. Our working model (Holden + Maive) front-loads thinking and parallelizes execution. The phases below reflect realistic pacing for our partnership: thinking time stays the same, implementation compresses significantly.

| Phase | Scope | Estimate | Notes |
|-------|-------|----------|-------|
| **0: Spec Fixes + Meta-Registry Bootstrap** | Fix SF-1/SF-2/SF-3, propose SF-5, bootstrap meta-registry with Publisher + Registry Actions | 1 session | Spec fixes are text edits + regenerate. Meta-registry bootstrap is config files + reference workflow templates. |
| **1: Content Hashing + RegistryClient Interface** | Go port of `moat_hash.py`, full test vector compliance, `RegistryClient` interface, `GitClient` wrapper | 1-2 sessions | Mechanical port with good test vectors. NFC normalization and streaming CRLF handling need care. |
| **2: The Big Merge** | Manifest + signing + revocation + lockfile + trust display. Sigstore spike first. | 3-5 sessions | The thinking-heavy phase. sigstore-go spike first. Then manifest parsing, verification, lockfile, revocation, UX. Each subsystem is a session. |
| **3: Polish + Index** | Registry Index, offline verification, `expires_at`, `self_published` display | 1-2 sessions | Incremental on top of Phase 2. |

**What takes time:** Design decisions (DQ-1 through DQ-7 in the MOAT panel doc), sigstore-go learning curve, error message design, TUI integration. These compress less because they require back-and-forth thinking, not just typing.

**What compresses:** Writing Go code, test suites, config schema extensions, manifest parsing, CLI commands. These are well-specified and mechanical once the design is clear.

**Key invariants at each phase boundary:**
- **After Phase 0:** Meta-registry producing signed manifests. MOAT spec has no known contradictions.
- **After Phase 1:** Syllago can compute MOAT content hashes identical to the reference implementation. `RegistryClient` interface is ready.
- **After Phase 2:** Syllago is a conforming MOAT client. Users see Verified/Recalled signals backed by real cryptographic verification. Revoked content is blocked. Non-interactive mode works for CI.
- **After Phase 3:** Full spec coverage including SHOULD/MAY requirements. Registry Index discovery.
