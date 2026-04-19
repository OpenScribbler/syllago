# MOAT Conformance Gap Analysis — Syllago + syllago-meta-registry

**Date:** 2026-04-12 (authored); **Audited:** 2026-04-17 against MOAT v0.6.0
**MOAT Spec Version:** 0.6.0 (Draft)
**Syllago Version:** current main branch (no MOAT code written yet; `cli/internal/moat/` does not exist)
**Scope:** Gap analysis for making Syllago a conforming MOAT client and syllago-meta-registry a conforming self-publishing registry

**2026-04-17 audit note:** MOAT advanced from 0.5.3 → 0.6.0 since this plan was authored. The upstream changes resolved SF-1/SF-2/SF-3 (Phase 0 spec fixes) and renamed `subagent` → `agent` (which aligns Syllago's internal `agents` type with MOAT directly). Additional v0.6.0 requirements are catalogued in Part 2b below. Drift items are annotated inline with **[v0.6.0 UPDATE]** callouts.

---

## Executive Summary

Syllago was built before MOAT existed. Its registry system uses git clones with YAML manifests (`registry.yaml`), per-file SHA-256 hashing, and mechanism-level install tracking (`installed.json`). MOAT defines a fundamentally different model: signed JSON manifests served at stable URLs, directory-level content hashing with text normalization, trust tiers backed by Sigstore/Rekor transparency logs, lockfiles with attestation bundles, and revocation enforcement.

The gap is **structural but not adversarial** — Syllago's architecture can be extended to support MOAT as a new registry type alongside existing git registries. No existing functionality needs to be torn out.

**Key numbers:**
- **66+ normative client requirements** extracted from the MOAT spec (42 MUST, 9 MUST NOT, 7 SHOULD, 1 SHOULD NOT, 7 MAY); v0.6.0 added ~12 more normative items (Version Transition, revocation archival, namespace uniqueness, algorithm allowlist, non-interactive table — see Part 2b)
- **12 major gaps** in Syllago's implementation (G-12 substantially reduced by upstream `subagent → agent` rename)
- **10 gaps** in syllago-meta-registry
- **3 spec inconsistencies** in MOAT 0.5.3 — all **RESOLVED** in v0.6.0
- **1 content type** (loadouts) excluded from MOAT attestation — Syllago-specific type

---

## Bead Chain — Progress Tracking

Work is tracked in beads (`bd` CLI). Check `bd ready` to see what's unblocked; `bd show <id>` for details.

**Phase epics:**

| Phase | Bead | Scope |
|-------|------|-------|
| 0 | `syllago-nxqft` | Meta-registry bootstrap |
| 1 | `syllago-zcgx5` | Content hashing |
| 2 | `syllago-dsqjz` | Manifest + signing + revocation + lockfile (big merge) |
| 3 | `syllago-m9pa9` | Polish + index |

**Phase dependencies:** Phase 2 depends on Phase 1. Phase 3 depends on Phase 2. Phase 0 runs in parallel with Phase 1.

**Gap beads** (tracked inline at each gap header below):

| Gap | Bead | Phase | Gap | Bead | Phase |
|-----|------|-------|-----|------|-------|
| G-1 | `syllago-nrmla` | 1 | G-13 | `syllago-sq1xk` | 2 |
| G-2 | `syllago-bg1gx` | 2 | G-14 | `syllago-ay3gb` | 2 |
| G-3 | `syllago-pwojm` | 2 | G-15 | `syllago-wouth` | 2 |
| G-4 | `syllago-25vib` | 2 | G-16 | `syllago-35ro4` | 3 |
| G-5 | `syllago-4xbft` | 2 | G-17 | `syllago-q2syn` | 2 |
| G-6 | `syllago-89zaa` | 2 | G-18 | `syllago-m3owd` | 2 |
| G-7 | `syllago-v2z92` | 2 | G-19 | `syllago-xtjpw` | 2 |
| G-8 | `syllago-42uow` | 2 | G-20 | `syllago-o3czr` | 2 |
| G-9 | `syllago-t4i67` | 2 | G-21 | `syllago-te64v` | 3 |
| G-10 | `syllago-33b1w` | 2 | G-22 | `syllago-2bhs9` | 3 |
| G-11 | `syllago-pbn0k` | 3 | G-23 | `syllago-kdfbq` | 3 |
| G-12 | `syllago-63gs7` | 2 | | | |

| Meta-registry | Bead | | Other | Bead |
|---------------|------|-|-------|------|
| MR-1 | `syllago-ycsh4` | | Sigstore spike | `syllago-9jzgr` |
| MR-2 | `syllago-dvel1` | | RegistryClient (AD-6) | `syllago-eb5sc` |
| MR-3 | `syllago-17u9f` | | | |
| MR-6 | `syllago-ivrbi` | | | |
| MR-7 | `syllago-c1wsw` | | | |
| MR-10 | `syllago-ryayh` | | | |
| MR-11 | `syllago-5fl0t` | | | |

**Skipped (resolved/N-A):** MR-4 (resolved in v0.6.0), MR-5 (loadouts excluded from MOAT), MR-8 (already aligned), MR-9 (no-op — empty dirs fine).

---

## Part 1: Spec Feedback — Issues Found in MOAT

> **All items in this section are resolved upstream (verified against MOAT v0.6.0 CHANGELOG on 2026-04-17).** SF-1, SF-2, SF-3 were fixed; SF-5 was overtaken by the `subagent → agent` rename; SF-4 was never a MOAT issue (loadouts are Syllago-specific and excluded from MOAT attestation). No Syllago client work is blocked by MOAT spec changes. This section is retained as historical record — items below show what was filed upstream and how each was resolved.

These issues were raised in the MOAT spec before or alongside the Syllago implementation.

### SF-1: [RESOLVED in v0.6.0] Test Vector Manifest Format Diverges from Normative Reference

**Resolution:** MOAT commit `7c5db96` "align test vectors to moat_hash.py sha256sum format (SB-1, SB-2, SB-3)" updated `generate_test_vectors.py` to emit `"{h}  {p}\n"` format. Test vectors now match `moat_hash.py`.

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

### SF-2: [RESOLVED in v0.6.0] Symlink Handling Contradiction Between Reference and Test Vectors

**Resolution:** Per CHANGELOG v0.6.0: "TV-09, TV-10 rewritten as error cases (symlink rejection)." All symlinks now consistently cause errors across spec, reference, and test vectors.

**Severity:** Moderate — test vectors describe behavior the normative reference doesn't support

- `moat_hash.py` (lines 175-176): Rejects ALL symlinks with `ValueError` — no resolution, no exclusion, hard error.
- `moat-verify.md` (line 79): Confirms symlinks cause exit code 2 failure.
- `publisher-action.md` (line 15): Symlinks "skip the item with a logged warning" (item-level, not file-level).
- TV-09: Describes internal symlinks as "resolved: symlink's path with target's content" — this resolution behavior doesn't exist in `moat_hash.py`.
- TV-10: Describes external symlinks as "excluded" — `moat_hash.py` doesn't exclude, it errors.

The normative reference is clear (reject all symlinks), but the test vectors describe a resolve/exclude model that contradicts it.

**Recommendation:** Update TV-09 and TV-10 to test that symlinks produce errors, not resolved/excluded results. Or, if the spec intends to support resolved internal symlinks, update `moat_hash.py` to match.

### SF-3: [RESOLVED in v0.6.0] Text Normalization Not Covered by Test Vectors

**Resolution:** v0.6.0 adds `reference/test_normalization.py` with TV-17 (CRLF normalization), TV-18 (UTF-8 BOM stripping), TV-19 (extensionless dotfile binary), TV-20 (NUL byte forces binary), TV-21 (CR at chunk boundary), TV-22 (lone CR at EOF). Syllago's Go implementation must pass all six.

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

### SF-5: [OVERTAKEN in v0.6.0] `subagent` vs `agent` Terminology

**v0.6.0 outcome:** MOAT reversed course and renamed `subagent` → `agent` as the normative content type, with `agents/` as the canonical directory (breaking change, CHANGELOG v0.6.0).

**Implication for Syllago:** Syllago's internal `agents` content type now maps to MOAT's `agent` directly. No boundary mapping is needed for this specific type. AD-5 updated below — the `agents ↔ subagent` row is deleted; plural→singular mapping (`skills → skill`, `commands → command`) still applies to other types.

**Implication for meta-registry:** `moat.yml` uses `type: agent` (not `type: subagent`). MR-3 and MR-4 below are updated.

**Historical context (informative):** The original SF-5 resolution argued for keeping `subagent` based on industry "AI Agent Runtime" framing. The v0.6.0 spec revision adopted the simpler `agent` term. Syllago's "provider" → something else rename remains a separate future workstream.

---

## Part 2: Gap Inventory — Syllago as Conforming Client

### G-1: Content Hashing Algorithm [BREAKING — Must Implement from Scratch] — `syllago-nrmla`

**MOAT requires:** Directory-level hash with text normalization (BOM strip, CRLF→LF), text/binary classification by extension + NUL-byte probe, VCS directory exclusions, symlink rejection, NFC Unicode path normalization, posix-style paths, global UTF-8 byte sort, sha256sum manifest format, `sha256:<hex>` output.

**Syllago has:** Per-file `HashFile()`/`HashBytes()` in `installer/integrity.go` — raw SHA-256 of file bytes, bare hex output. No text normalization, no directory-level hash, no NFC normalization, no symlink rejection.

**Gap:** Complete reimplementation needed. None of Syllago's existing hash functions produce MOAT-conforming hashes.

**Architecture:** New package `internal/moat/hash.go`. Port `moat_hash.py` to Go. Must pass all MOAT test vectors (once SF-1 is resolved). Existing per-file hashing in `installer/integrity.go` remains for its own purpose (drift detection).

**Key implementation details:**
- Go's `filepath.WalkDir` sorts per-directory, not globally — must collect all entries then sort
- Go has no stdlib NFC normalizer — need `golang.org/x/text/unicode/norm`
- Text normalization must handle CR at chunk boundaries (streaming impl)
- Must convert `filepath.Separator` to `/` (Windows)

### G-2: Registry Manifest Format [MAJOR — New Registry Type] — `syllago-bg1gx`

**MOAT requires:** Signed JSON document (`registry.json`) with `schema_version`, `manifest_uri`, `name`, `operator`, `updated_at`, `registry_signing_profile`, `content[]`, `revocations[]`. Served at a stable URL, not git-cloned.

**Syllago has:** YAML `registry.yaml` with `name`, `description`, `version`, `maintainers`, `items[]`. Distributed via git clone.

**Gap:** Completely different format, transport, and trust model.

**Architecture:** Dual registry type support:
- Existing git registries continue working via `registry.Clone()`/`registry.Sync()` with `registry.yaml`
- New MOAT registries use HTTP fetch of `registry.json` + `.sigstore` bundle
- Detection: config-level `type` field (`"git"` or `"moat"`) set at `registry add` time
- The `config.Registry` struct gains MOAT-specific fields (see G-4)

### G-3: Manifest Signing and Verification [MAJOR — No Implementation Exists] — `syllago-pwojm`

**See ADR 0007** (`docs/adr/0007-moat-g3-slice-1-scope.md`) for the architectural decisions that supersede the original options below. ADR 0007 is the authoritative source for slice-1 scope; the text here is retained for historical context.

**MOAT requires:** Verify registry manifest using cosign keyless OIDC signing. Fetch bundle at `{manifest_uri}.sigstore`. Verify bundle covers exact manifest bytes, OIDC issuer/subject match `registry_signing_profile`, Rekor transparency log entry is valid. Rekor unavailability is a hard failure.

**Syllago has:** Hidden stub commands `sign`/`verify` returning "not yet implemented." An unimplemented `signing` package with interface definitions. A spike (`syllago-9jzgr`) in `cli/internal/moat/` that verifies one real meta-registry Rekor entry end-to-end via sigstore-go, built around a `BuildBundle` helper that converts raw Rekor API JSON into a sigstore bundle.

**Gap:** No production-ready primitive. The spike's `BuildBundle` path is architectural dead-weight: MOAT Publisher Actions emit `.sigstore` bundles directly, and sigstore-go can load them via `sgbundle.LoadJSONFromReader`. The spike predates that decision.

**Architecture (decided in ADR 0007):** Ship a primitive-only slice 1 plus minimal forward-compat schema.

**Slice 1 consists of:**

1. `VerifyManifest(manifestBytes, bundleBytes, pinnedProfile, trustedRoot) (VerificationResult, error)` consuming `.sigstore` bundles by constructing `*sgbundle.Bundle` and calling its `UnmarshalJSON` method. No raw Rekor JSON in production. No live network calls.
2. Structured `VerificationResult` exposing `RevocationChecked: false` explicitly — callers cannot collapse to "verified" without ignoring the bit.
3. `SigningProfile` gains `ProfileVersion`, `SubjectRegex`, `IssuerRegex`, `RepositoryID`, `RepositoryOwnerID`. When issuer is `https://token.actions.githubusercontent.com`, verifier MUST match both numeric IDs (from Fulcio OIDC extensions `1.3.6.1.4.1.57264.1.15` (repo) and `1.3.6.1.4.1.57264.1.17` (owner)) — this closes the repo-transfer forgery vector. Note: `.12`/`.13` are URI/digest strings (mutable on transfer) and are NOT the correct OIDs for this binding.
4. `config.Registry.TrustedRoot` optional string (schema only; zero value = bundled default). Wired through to verification in slice 2+.
5. `trusted_root.json` bundled via `go:embed` alongside an `issued_at` constant. 90-day warn / 180-day escalation / 365-day hard-fail. `moat trust status` command exit-coded 0/1/2.
6. `BuildBundle` moves to test-only helpers.
7. Three-state trust output: `signed` / `unsigned` / `invalid`. The word `verified` is reserved for when revocation lands.

**Explicitly deferred to slice 2+:** `registry add` TOFU flow, install-time verification wiring, signing-identity UX (flag vs. allowlist vs. interactive), TUF refresh, revocation trust model, per-registry `TrustedRoot` enforcement, non-GitHub OIDC issuers.

**Parallel MOAT spec PR:** syllago commits to contributing a spec PR formalizing trusted-root acquisition modes, GitHub OIDC numeric-ID binding, and the slice-1 error code vocabulary.

### G-4: Registry Trust Bootstrap & Signing Profile Tracking [MAJOR] — `syllago-25vib`

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

### G-5: Per-Item Rekor Attestation Verification [MAJOR] — `syllago-4xbft`

**MOAT requires:** For Signed/Dual-Attested items, verify per-item Rekor entry on install. Reconstruct canonical payload `{"_version":1,"content_hash":"sha256:<hex>"}`, verify signature against certificate, confirm OIDC identity matches signing profile. Unknown `_version` values must fail verification.

**Syllago has:** Nothing — no per-item attestation concept.

**Architecture:** New verification pipeline in `internal/moat/verify.go`:
1. On install from MOAT registry: read `rekor_log_index` from manifest entry
2. Fetch Rekor entry at that index
3. Reconstruct canonical payload from `content_hash`
4. Verify signature covers payload
5. Confirm OIDC identity matches expected signing profile

### G-6: Trust Tier Determination and Display [MODERATE] — `syllago-89zaa`

**MOAT requires:** Three normative tiers (Dual-Attested, Signed, Unsigned). Determined by manifest entry fields (`rekor_log_index` for Signed, `signing_profile` for Dual-Attested). Client must surface tier before install confirmation.

**Syllago has:** Static `Trust` string in registry config — user-assigned label, not computed from attestation data.

**Gap:** Trust is user-assigned, not attestation-derived. No per-item trust tier. No UI for displaying trust tier at install time.

**Architecture:**
- Add `TrustTier` field to `catalog.ContentItem` for MOAT-sourced content
- Compute tier from manifest entry fields during catalog scan
- Update install wizard TUI to display tier before confirmation
- CLI install flow to surface tier in non-interactive mode

### G-7: MOAT Lockfile [MAJOR — New File] — `syllago-v2z92`

**MOAT requires (v0.6.0):** Lockfile with `moat_lockfile_version`, `registries` object keyed by manifest URL with `fetched_at` timestamps per registry, `entries[]` (each with `name`, `type`, `registry`, `content_hash`, `trust_tier`, `attested_at`, `pinned_at`, `attestation_bundle`, `signed_payload`), and `revoked_hashes[]`. Must store full cosign bundle for offline re-verification. Must verify `sha256(signed_payload) == data.hash.value` in Rekor entry before writing. Lockfile must be interoperable across conforming clients.

**[v0.6.0 UPDATE]** The `registries[url].fetched_at` object is new in v0.6.0. It records the client's last successful manifest fetch per registry — used for staleness enforcement (see G-9) and by `moat-verify` for staleness auditing. Upgrade path: if a client reads a lockfile without `registries`, it initializes the key and sets `fetched_at` on next successful fetch.

**Syllago has:** `installed.json` with three separate arrays (`hooks`, `mcp`, `symlinks`) tracking mechanism-level state for uninstall. Different structure, different purpose.

**Gap:** `installed.json` cannot evolve into a MOAT lockfile — they serve fundamentally different purposes:
- `installed.json` = "what did Syllago put where, so it can undo it" (mechanism tracking)
- MOAT lockfile = "what content is installed with what trust properties" (trust ledger)

**Architecture:** New file `.syllago/moat-lockfile.json` coexisting with `installed.json`. Install flow writes to BOTH: `installed.json` for operational tracking, lockfile for trust state.

### G-8: Revocation Handling [MAJOR — No Mechanism Exists] — `syllago-42uow`

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

### G-9: Manifest Freshness / Staleness Threshold [MODERATE] — `syllago-t4i67`

**MOAT requires (v0.6.0):** **72-hour** default staleness threshold (TUF-inspired model) computed against the client's `fetched_at` timestamp stored in lockfile `registries[url].fetched_at`. If manifest carries `expires` (RFC 3339 UTC), clients MUST NOT trust it after that time. Staleness is checked at install time, not continuously. A failed refresh MUST NOT reset the clock — it runs from the last *successful* fetch. Non-interactive clients MUST exit non-zero when staleness is exceeded and cannot refresh.

**[v0.6.0 UPDATE]** Plan previously specified 24-hour threshold; spec landed at 72 hours (survives the weekend: Friday 6pm to Monday 9am is 63 hours). The `expires_at` field was renamed to `expires`. The "48h hard-fail" panel dissent is resolved — 72h default with hard-fail on stale+offline is the normative behavior.

**Syllago has:** No freshness tracking. `registry.Sync()` is user-triggered only.

**Gap:** No `fetched_at` timestamp per registry. No auto-sync behavior. No `expires` enforcement.

**Architecture:**
- Add `LastFetchedAt` to `config.Registry` (see G-4); also record in lockfile's `registries[url].fetched_at` for cross-client interop
- Before any MOAT operation (install, status check): check staleness against 72h default or manifest's `expires`
- If stale: auto-fetch MOAT manifest (HTTP is fast). For git registries: prompt or auto-pull
- Check `expires` field when present (renamed from `expires_at`)
- Non-interactive mode: exit 1 with message if stale and refresh fails

**Behavioral change:** Currently `syllago install` never touches the network for git registries. With MOAT staleness, MOAT-type registries may auto-fetch. This only affects MOAT registries — git registries retain current behavior.

### G-10: Private Content Isolation [SMALL — Syllago is Ahead] — `syllago-33b1w`

**MOAT requires:** Must not auto-submit private content to public registries. Must surface visibility and require confirmation.

**Syllago has:** Multi-gate privacy system (G1-G4) with fail-closed visibility probing. Stronger than MOAT's requirements.

**Gap:** Minor — need per-item `private_repo` tracking when consuming MOAT manifests (currently tracked at registry level, not item level).

### G-11: Registry Index Support [MODERATE — New Feature] — `syllago-pbn0k`

**MOAT requires:** Support configurable registry index sources. Surface which indices are used. Index does not bypass per-registry trust. Users must be able to add registries by direct URL without an index.

**Syllago has:** `KnownAliases` map with hardcoded alias expansion. Not an index.

**Gap:** No index fetching, parsing, or signed index verification. Not blocking for initial conformance — Syllago can support direct-URL registry addition first and add index support later.

### G-12: Content Type Alignment [SMALL — Plural-to-Singular Only] — `syllago-63gs7`

**MOAT normative types (v0.6.0):** `skill`, `agent`, `rules`, `command`. Deferred: `hook`, `mcp`.

**Syllago types:** `skills`, `agents`, `rules`, `commands`, `hooks`, `mcp`, `loadouts`.

**[v0.6.0 UPDATE]** Upstream renamed `subagent` → `agent`. Syllago's `agents` type now aligns with MOAT's `agent` semantically; only the plural/singular surface differs. The deeper "rename agents" refactor previously implied by this gap is no longer needed.

**Remaining mismatch (boundary mapping only):**
- `skills` → `skill` (plural → singular at MOAT boundary)
- `agents` → `agent` (plural → singular at MOAT boundary)
- `commands` → `command` (plural → singular at MOAT boundary)
- `rules` → `rules` (already matches)
- `loadouts` is Syllago-specific — excluded from MOAT attestation entirely
- `hooks` and `mcp` are deferred in MOAT but active in Syllago — pass through internally, excluded from MOAT manifest emission

**Architecture:** `internal/moat/typemap.go` performs plural-to-singular mapping at the MOAT manifest boundary. No internal Syllago renames required.

---

## Part 2b: Additional v0.6.0 Spec Requirements (Added 2026-04-17)

These requirements were added or made normative between MOAT 0.5.3 and 0.6.0. None were covered in the original gap inventory.

### G-13: `attestation_hash_mismatch` Field [SMALL] — `syllago-sq1xk`

**Spec (v0.6.0):** §Registry Manifest, §Trust Tier Determination. When the Registry Action's computed `content_hash` differs from the publisher's attested hash in `moat-attestation.json`, the Registry Action:
1. Indexes the item at the registry's computed hash (authoritative)
2. Downgrades trust tier to `Signed` regardless of publisher Rekor verification
3. Sets `"attestation_hash_mismatch": true` on the manifest entry

**Client requirement:** Conforming clients SHOULD surface `attestation_hash_mismatch: true` to End Users. Clients MAY treat a newly-detected hash mismatch on previously-installed content as a re-approval event. Clients MUST NOT hard-block on `attestation_hash_mismatch` alone.

**Architecture:** Add to Phase 2 manifest-parsing + trust display (metadata panel badge or annotation).

### G-14: `_version` Grace Period for Attestation Payload [SMALL] — `syllago-ay3gb`

**Spec (v0.6.0):** §Version Transition. When a new `_version` value is introduced, conforming clients MUST accept both prior and new values for **6 months** after the bump. After the grace period, the prior value MUST be rejected.

**Verification ordering (load-bearing):** Clients MUST verify in this order:
1. `content_hash` matches locally computed hash
2. `_version` is a recognized schema version (in-grace or current)
3. Rekor certificate identity

Checking `_version` first creates a TOCTOU window where a verifier accepts an old-format attestation for different content. Content-hash-first is normative.

**Architecture:** Add to Phase 2 per-item Rekor verification logic. Current `_version` is `1`; no bump is pending, so the practical work is to implement the ordering correctly now so the grace-period logic is easy to drop in later.

### G-15: Revocation Archival — Lockfile Must Hold Pruned Revocations [SMALL] — `syllago-wouth`

**Spec (v0.6.0):** §Revocation. Registries MAY prune revocation entries after ≥180 days (recommended minimum). When a revocation is pruned from the manifest, the lockfile's `revoked_hashes` entry **MUST** persist. Clients MUST NOT silently remove a `revoked_hashes` entry because the manifest no longer carries the revocation.

**Plan status:** G-8 task #16 (append-only `revoked_hashes`) already captures this at the client level. This entry documents that the behavior is now normative rather than a local design choice.

**Meta-registry side:** Registry Action enforces `revocation-tombstones.json` in the `moat-registry` branch — a previously-revoked hash MUST NOT reappear in the `content` array after pruning. This adds a meta-registry concern (see MR-11 below).

### G-16: Namespace Uniqueness — `(name, type)` Compound Key [SMALL] — `syllago-35ro4`

**Spec (v0.6.0):** §Registry Manifest. `content[].name` + `content[].type` MUST be unique within a single manifest. Registry Action MUST exit non-zero on duplicate detection. Cross-registry collisions (same name+type in two different registries) are handled by the client, which SHOULD display `source_uri` for disambiguation.

**Client requirement:** When multiple registries surface the same `(name, type)`, the TUI and CLI MUST display `source_uri` alongside the item name. This is already covered by Phase 3 task #7 ("name collision handling in install wizard — display `source_uri` as disambiguator") — annotate with reference to the normative rule.

**Meta-registry side:** The Registry Action validates this automatically. No extra meta-registry work.

### G-17: `revocations[].source` Field (Read Explicitly) [SMALL] — `syllago-q2syn`

**Spec (v0.6.0):** §Registry Manifest. Each revocation entry has an OPTIONAL `source` field: `"registry"` or `"publisher"`. Absent defaults to `"registry"` (fail-closed). Determines client behavioral class:
- `registry` → hard-block
- `publisher` → warn-once-per-session, allow with explicit confirmation

**Plan status:** G-8 correctly splits the two behaviors but doesn't read the explicit field. Update Phase 2 tasks #13–#15 to branch on `revocations[].source` rather than inferring from context.

### G-18: Non-Interactive Behavior — Normative Table [SMALL] — `syllago-m3owd`

**Spec (v0.6.0):** §Revocation Mechanism. Non-interactive clients MUST exit non-zero on:
1. TOFU signing-profile acceptance (first registry add)
2. `registry_signing_profile` change detected
3. Publisher revocation encountered
4. Manifest staleness exceeded

**Plan status:** AD-9 covers the concept but predates the normative table. Update AD-9 to cite the spec's four conditions verbatim and map each to a Syllago exit code. Pre-approval mechanism is deferred upstream (Issue 11 in MOAT ROADMAP), so Syllago matches that — no pre-approval flow in this plan.

### G-19: Algorithm Allowlist [SMALL] — `syllago-xtjpw`

**Spec (v0.6.0):** §What the Spec Defines. `sha256` is REQUIRED; `sha512` is OPTIONAL; `sha1`, `md5`, and any algorithm with known practical collision attacks are FORBIDDEN. Clients MUST reject hashes using a forbidden algorithm as a **hard failure** (not a warning). Unrecognized algorithms MUST refuse to verify rather than silently pass.

**Plan status:** Plan assumes sha256 throughout but doesn't specify reject behavior. Add allowlist check to Phase 2 manifest validation — parse `<algorithm>:<hex>` from `content_hash` and `revocations[].content_hash`, reject on `sha1`/`md5`/unknown.

### G-20: Attestation Payload Canonical Test Vector [SMALL] — `syllago-o3czr`

**Spec (v0.6.0):** §Attestation Payload. Input hash `sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b` produces payload bytes `{"_version":1,"content_hash":"sha256:3a4b…}` with SHA-256 = `b7d70330da474c9d32efe29dd4e23c4a0901a7ca222e12bdbc84d17e4e5f69a4`.

**Plan status:** Use as a unit-test fixture in Phase 2 canonical-payload serialization. Prevents whitespace/key-order regressions in the Go implementation (Go's default JSON encoder adds no whitespace but key ordering is map-iteration-dependent — must use `sort_keys` equivalent).

### G-21: OIDC Subject Rename Attack [INFORMATIVE] — `syllago-te64v`

**Spec (v0.6.0):** §Publisher Signing Identity Model, informative risk note. GitHub Actions OIDC tokens encode the subject as `repo:owner/name:ref:refs/heads/branch` — the repo name is embedded in the Rekor certificate. When a publisher renames their repo, the subject string changes. An attacker who registers the old name produces Rekor entries whose subject matches the original `signing_profile`. Cosign's `--certificate-identity` flag matches against the subject string, not the immutable `repository_id` claim. No client-level mitigation with the current `cosign verify-blob` toolchain.

**Plan impact:** No plan change. Worth noting in AD-8 error-message templates so security operators understand the attack surface. If Syllago wants to close this gap later, it would require a custom Rekor verifier that reads the `repository_id` extension — out of scope for initial conformance.

### G-22: `self_published` Disclosure [SMALL] — `syllago-2bhs9`

**Spec (v0.6.0):** §Registry Manifest, Registry Action §Self-Publishing. When the Registry Action detects that the registry repo URI matches a source URI, it MUST set `self_published: true`. Conforming clients SHOULD surface this to End Users so they can make an informed trust decision.

**Plan status:** Phase 3 task #5 covers surfacing. No change needed — spec now makes the emission requirement explicit on the registry side.

### G-23: `publisher_workflow_ref` (Meta-Registry Side) [SMALL] — `syllago-kdfbq`

**Spec (v0.6.0):** Publisher Action §`moat-attestation.json`. OPTIONAL field derived from `GITHUB_WORKFLOW_REF` at signing time (e.g., `.github/workflows/moat.yml@refs/heads/main`). Registry Action reads this to construct the expected OIDC subject for publisher verification. Absent → Registry Action falls back to `.github/workflows/moat.yml@refs/heads/main`.

**Plan status:** Meta-registry side only. Using the latest reference workflow template satisfies this automatically. No Syllago client work.

---

## Part 3: Gap Inventory — syllago-meta-registry

### MR-1: No GitHub Actions Workflows — `syllago-ycsh4`

**Needs:** Two workflow files:
- `.github/workflows/moat.yml` — Publisher Action (based on `reference/moat.yml`)
- `.github/workflows/moat-registry.yml` — Registry Action (based on `reference/moat-registry.yml`)

**Complexity:** Moderate — reference templates are drop-in with minimal customization.

### MR-2: No `registry.yml` (MOAT Registry Config) — `syllago-dvel1`

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

### MR-3: `moat.yml` (Content Discovery Override) — Possibly Unneeded in v0.6.0 — `syllago-17u9f`

**[v0.6.0 UPDATE]** MOAT renamed `subagent` → `agent` and canonical directory to `agents/`. The meta-registry already uses `agents/`, so Tier-1 discovery now matches without any override. `moat.yml` is only needed if the meta-registry adds content with non-canonical layouts.

If `moat.yml` becomes necessary for any reason (e.g., non-canonical paths for future content), the content type is now `agent`, not `subagent`:

```yaml
items:
  - name: syllago-author
    type: agent
    path: agents/syllago-author
```

Loadouts remain excluded from MOAT attestation — Syllago-specific type, distributed via `registry.yaml` only.

### MR-4: [RESOLVED in v0.6.0] `agents/` Directory Name Mismatch

**Previously:** Meta-registry used `agents/` but MOAT expected `subagents/`.
**v0.6.0:** MOAT canonical directory is `agents/`. No rename or mapping needed. This item is resolved by the upstream spec change.

### MR-5: Loadout Nested Structure

**Current:** `loadouts/<provider>/<name>/` (two levels deep)
**MOAT expects:** `<category>/<item_name>/` (one level deep)
**Not applicable:** Loadouts are excluded from MOAT — they remain Syllago-native content distributed via `registry.yaml`.

### MR-6: Self-Publishing OIDC Identity — `syllago-ivrbi`

**Needs:** Two distinct OIDC subjects (automatically satisfied by having two separate workflow files):
- Publisher: `.../.github/workflows/moat.yml@refs/heads/main`
- Registry: `.../.github/workflows/moat-registry.yml@refs/heads/main`

**Complexity:** Trivial — handled automatically by Sigstore.

### MR-7: `moat-attestation` and `moat-registry` Branches — `syllago-c1wsw`

**Needs:** Orphan branches created automatically on first workflow run.
**Complexity:** Trivial.

### MR-8: Skills Directory Alignment

**Current:** `skills/` with 5 items — matches MOAT's canonical `skills/` directory.
**Status:** Already aligned. No changes needed.

### MR-9: Empty Directories

**Current:** `commands/`, `rules/`, `hooks/`, `mcp/` exist but are empty.
**Impact:** Publisher Action will scan them and find nothing — no error, no items. Fine.

### MR-10: `self_published: true` Disclosure — `syllago-ryayh`

**Needs:** Manifest must disclose self-publishing since publisher + registry are the same repo.
**Complexity:** Trivial — auto-detected by the Registry Action.

### MR-11: [v0.6.0] Revocation Tombstones — `syllago-5fl0t`

**Needs:** The Registry Action writes `revocation-tombstones.json` in the `moat-registry` branch alongside the manifest. Contains an array of content_hash strings that MUST never reappear in `content[]` after being revoked and pruned from `revocations[]`.

**Complexity:** Trivial — handled by the reference Registry Action template. Meta-registry just needs to use the latest version.

**Why it matters:** Closes the gap for clients who never witnessed a revocation — a previously-revoked hash reappearing with no revocation entry would bypass their lockfile guard on first install.

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

**[v0.6.0 UPDATE]** MOAT renamed `subagent` → `agent`, so the boundary mapping is now plural-to-singular only (same semantic meaning on both sides). Syllago maps between internal plural names and MOAT singular type strings at the MOAT manifest boundary via `internal/moat/typemap.go`:

| Syllago Internal | MOAT Type | Notes |
|-----------------|-----------|-------|
| `skills` | `skill` | Plural → singular |
| `agents` | `agent` | Plural → singular (was `subagent` before v0.6.0) |
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

### Phase 0: Meta-Registry Bootstrap — `syllago-nxqft` (epic)

**[v0.6.0 UPDATE]** Tasks 1–4 (spec fixes) are **DONE upstream**. Phase 0 is now meta-registry bootstrap only.

**Goal:** Bootstrap meta-registry as the first self-publishing MOAT registry.

**Tasks:**
1. ~~Fix SF-1 in MOAT spec~~ — **DONE** (commit `7c5db96`)
2. ~~Fix SF-2 symlink handling~~ — **DONE** (CHANGELOG v0.6.0)
3. ~~Add text normalization test vectors~~ — **DONE** (TV-17 through TV-22)
4. ~~Strengthen "AI Agent Runtime" terminology~~ — **SUPERSEDED** (MOAT renamed `subagent` → `agent` in v0.6.0)
5. Create `registry.yml` in meta-registry — MOAT registry config
6. (Optional) Create `moat.yml` in meta-registry — only if non-canonical paths are needed. v0.6.0's `agents/` canonical directory now matches the meta-registry layout, so `moat.yml` may not be required.
7. Add `.github/workflows/moat.yml` — Publisher Action (reference template, latest version — auto-emits `publisher_workflow_ref`)
8. Add `.github/workflows/moat-registry.yml` — Registry Action (reference template, latest version — auto-writes `revocation-tombstones.json`, enforces `(name, type)` uniqueness)
9. Validate on first workflow runs: `moat-attestation` branch, `moat-registry` branch with manifest + `.sigstore` + `revocation-tombstones.json`, `self_published: true`, dual OIDC subjects, no `(name, type)` duplicates

**Output:** Working MOAT registry producing signed v0.6.0-conformant manifests.

### Phase 1: Content Hashing — `syllago-zcgx5` (epic)

**Goal:** Go implementation of MOAT `content_hash` that passes all normative test vectors.

**[v0.6.0 UPDATE]** `moat_hash.py` is now INFORMATIVE reference; test vectors (TV-01 through TV-22) are NORMATIVE. Acceptance criterion changes from "matches `moat_hash.py` bit-for-bit" to "passes all TV-01…TV-22". A second conforming implementation in a different language advances the spec past Draft — Syllago's Go implementation qualifies if validated against test vectors independently.

**Tasks:**
1. Create `cli/internal/moat/hash.go` — Go implementation guided by `moat_hash.py` but validated against test vectors
2. Add `golang.org/x/text/unicode/norm` dependency
3. Create `cli/internal/moat/hash_test.go` — all test vectors as table-driven tests, including TV-17 (CRLF), TV-18 (BOM), TV-19 (dotfile binary), TV-20 (NUL forces binary), TV-21 (CR at chunk boundary), TV-22 (lone CR at EOF)
4. Create `cli/internal/moat/testdata/` — filesystem fixtures for text normalization, symlink rejection, VCS dirs
5. Cross-language sanity check: hash fixtures with both Go implementation and `moat_hash.py`, log any divergence — but the test-vector suite is authoritative
6. Define and implement `RegistryClient` interface (AD-6) — wrap existing git code in `GitClient` — `syllago-eb5sc`

**Output:** `moat.ContentHash(dir string) (string, error)` producing `sha256:<hex>` that passes all normative test vectors. `RegistryClient` interface ready for `MOATClient`.

**Can run in parallel with Phase 0** — hash implementation validates against test vectors, not against the meta-registry.

### Phase 2: Manifest + Signing + Revocation + Lockfile (The Big Merge) — `syllago-dsqjz` (epic)

**Goal:** Full conforming MOAT client. Trust tiers displayed ONLY after verification works.

This phase merges the original Phases 2-4 because the panel unanimously agreed (C2) that showing trust signals without verification behind them is worse than showing nothing.

**Week 1 — Sigstore spike — `syllago-9jzgr`:**
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
13. Implement revocation checking against manifest `revocations[]` on sync — **read `revocations[]. source` explicitly (v0.6.0 G-17); absent defaults to `"registry"` (fail-closed)**
14. Implement hard-block for `source: "registry"` revocations
15. Implement warn-on-use for `source: "publisher"` revocations (once per session)
16. Implement `revoked_hashes` in lockfile (append-only; MUST persist across manifest pruning per v0.6.0 G-15)

**Lockfile:**
17. Create `cli/internal/moat/lockfile.go` — types, read/write
18. Populate lockfile on install (all required fields, including `registries[url].fetched_at` object per v0.6.0 G-7)
19. Store attestation bundle + signed payload at install time
20. Implement `sha256(signed_payload)` → Rekor `data.hash.value` check before writing; install MUST abort on mismatch
21. Implement staleness threshold (**72h default per v0.6.0 G-9**, was 24h; jitter per C7). Lockfile upgrade path: if `registries` key absent on read, initialize on next successful fetch.

**v0.6.0 additions (Phase 2):**
22a. Parse and surface `attestation_hash_mismatch` on manifest entries (G-13) — metadata panel badge/annotation; do NOT hard-block on this flag alone
22b. Implement `_version` grace-period handling for Attestation Payload (G-14) — accept current `_version: 1` plus any declared grace values; verify `content_hash` BEFORE `_version` (TOCTOU defense)
22c. Algorithm allowlist (G-19) — parse `<algorithm>:<hex>` from `content_hash` and `revocations[].content_hash`; reject `sha1`/`md5`/unknown as hard failure
22d. Canonical payload test fixture (G-20) — unit test using the normative vector (SHA-256 `b7d70330…`) to guard against JSON serialization regressions
22e. Non-interactive behavior table (G-18) — map each of the four MUST-exit conditions to a distinct Syllago exit code

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

### Phase 3: Polish + Index — `syllago-m9pa9` (epic)

**Goal:** Full spec coverage including SHOULD/MAY requirements.

**Tasks:**
1. Registry Index fetching, parsing, signed verification
2. Index configuration (add/remove/replace)
3. **`expires` manifest rejection** (renamed from `expires_at` in v0.6.0)
4. Offline verification mode using lockfile
5. `self_published` surfacing in UI (G-22)
6. `syllago verify --offline` (lockfile mode) — implement the `moat-verify` offline protocol (see `../moat/specs/moat-verify.md`)
7. Name collision handling in install wizard — display `source_uri` as disambiguator (normative per v0.6.0 G-16 `(name, type)` uniqueness rule)

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

All SF items are now resolved upstream — no MOAT spec work remains before Phase 1 can begin.

| Spec Issue | Status | Notes |
|-----------|--------|-------|
| SF-1 (test vector format) | **RESOLVED in v0.6.0** | Commit `7c5db96`; test vectors now match `moat_hash.py` sha256sum format |
| SF-2 (symlink handling) | **RESOLVED in v0.6.0** | TV-09/TV-10 rewritten as error cases |
| SF-3 (text normalization tests) | **RESOLVED in v0.6.0** | TV-17 through TV-22 added |
| SF-4 (loadout type) | N/A | Loadouts are Syllago-specific, excluded from MOAT |
| SF-5 (terminology) | **OVERTAKEN in v0.6.0** | MOAT renamed `subagent` → `agent`; Syllago's `agents` aligns directly |

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

- `.github/workflows/moat.yml` — Publisher Action (latest reference template)
- `.github/workflows/moat-registry.yml` — Registry Action (latest reference template)
- `registry.yml` — MOAT registry configuration
- `moat.yml` — Content discovery override (optional in v0.6.0; only needed for non-canonical layouts)

**Auto-generated on `moat-registry` branch by Registry Action:**
- `registry.json` — signed manifest
- `registry.json.sigstore` — cosign bundle
- `revocation-tombstones.json` — tombstone list (v0.6.0 MR-11)

**Auto-generated on `moat-attestation` branch by Publisher Action:**
- `moat-attestation.json` — per-item attestations with `publisher_workflow_ref` field (v0.6.0 G-23)

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

| Topic | Position A | Position B | v0.6.0 resolution |
|-------|-----------|-----------|-------------------|
| **48h staleness hard-fail** | Enterprise Security: MUST hard-fail — degrading to a warning is a downgrade attack vector | Platform Vendor: hard-failing because a laptop was offline for a weekend will drive users to disable MOAT entirely | **RESOLVED:** Spec chose 72h default (survives a weekend) with hard-fail on stale+can't-refresh. Non-interactive MUST exit non-zero. |
| **Lockfile HMAC** | Enterprise Security: lockfile integrity via HMAC deserves a Phase 3 design decision | Platform Vendor: over-engineering — attacker with local write access can modify installed content directly | Spec §Lockfile Integrity sides with Platform Vendor: an attacker with local write can modify installed content directly; detection via `moat-verify` re-hashing is the stated approach. No HMAC. |
| **Revocation archival** | Registry Operator: ever-growing `revocations[]` array needs an archival/pruning mechanism before registries go live | Others: acknowledged but deferred — spec concern, not Syllago concern | **RESOLVED:** v0.6.0 adds 180-day retention + `revocation-tombstones.json` + lockfile-authoritative rule for pruned revocations. Covered by G-15 and MR-11. |
| **Publisher workflow removal** | Solo Publisher: what happens when a publisher deletes their MOAT workflow? Stale attestations that never update are a real lifecycle event | Others: acknowledged — spec should address this | Still open upstream; not covered in v0.6.0. Tracked as MOAT spec concern, not Syllago concern. |

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
| **0: Meta-Registry Bootstrap** | Bootstrap meta-registry with Publisher + Registry Actions (reference templates). Spec fixes already DONE upstream. | <1 session | Config files + latest reference workflow templates. |
| **1: Content Hashing + RegistryClient Interface** | Go implementation of MOAT hash validated against TV-01…TV-22, `RegistryClient` interface, `GitClient` wrapper | 1-2 sessions | NFC normalization and streaming CRLF handling need care. Test vectors are the authoritative acceptance criterion. |
| **2: The Big Merge** | Manifest + signing + revocation + lockfile + trust display + v0.6.0 additions (G-13 through G-20). Sigstore spike first. | 3-5 sessions | The thinking-heavy phase. sigstore-go spike first. Then manifest parsing, verification, lockfile, revocation, UX. Each subsystem is a session. |
| **3: Polish + Index** | Registry Index, offline verification, `expires` enforcement, `self_published` + `attestation_hash_mismatch` display | 1-2 sessions | Incremental on top of Phase 2. |

**What takes time:** Design decisions (DQ-1 through DQ-7 in the MOAT panel doc), sigstore-go learning curve, error message design, TUI integration. These compress less because they require back-and-forth thinking, not just typing.

**What compresses:** Writing Go code, test suites, config schema extensions, manifest parsing, CLI commands. These are well-specified and mechanical once the design is clear.

**Key invariants at each phase boundary:**
- **After Phase 0:** Meta-registry producing signed manifests. MOAT spec has no known contradictions.
- **After Phase 1:** Syllago can compute MOAT content hashes identical to the reference implementation. `RegistryClient` interface is ready.
- **After Phase 2:** Syllago is a conforming MOAT client. Users see Verified/Recalled signals backed by real cryptographic verification. Revoked content is blocked. Non-interactive mode works for CI.
- **After Phase 3:** Full spec coverage including SHOULD/MAY requirements. Registry Index discovery.
