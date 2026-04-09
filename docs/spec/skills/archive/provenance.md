# Provenance and Attestation Specification

**Date:** 2026-04-01
**Status:** Design — iterating on core decisions
**Scope:** All syllago content types (skills, agents, hooks, rules, MCP configs, commands, loadouts)

## Design Principle

**One model, all content types. Facts recorded, integrity verified, claims signed.**

Provenance records who made something, where it came from, what version it is, whether it's been tampered with, and what it was derived from. Every content type gets the same provenance model — no tiers, no exceptions. Syllago is pioneering cross-provider sharing for all these types, not just skills. The provenance model should work for the world we're building, not the world that exists today.

## Core Decisions (Established)

### 1. Universal provenance — no content-type tiers

All 7 content types get the same provenance model. The distinction between "executable" and "instructional" content is a false boundary — an agent definition that says "always run `curl | bash`" is as dangerous as a hook script that does it. The LLM is the execution engine; anything loaded into context can cause execution.

Content type determines **scan depth** (install safety's concern), not **provenance depth**.

### 2. Separate content and metadata hashes

Two independent hashes, each covering a distinct concern:

- **`content_hash`** — SHA-256 of the content files only. Answers: "has the actual content been modified?" Changing a description or author email doesn't break it. This is the integrity anchor.
- **`meta_hash`** — SHA-256 of the provenance metadata (serialized as canonical JSON via RFC 8785/JCS). Answers: "have the provenance claims been modified?"

The SSH signature signs both hashes, binding content integrity and provenance claims to a cryptographic identity.

**Why separate, not combined:**
- Authors edit metadata frequently (descriptions, emails, repo URLs). A combined hash would break on every metadata tweak, creating friction during authoring.
- Content integrity and metadata integrity are different questions with different audiences. Consumers who just want "is this the same skill?" only need `content_hash`. Auditors who want "are the claims intact?" also check `meta_hash`.
- The signature covers both hashes, so tamper-evidence is preserved — changing either one invalidates the signature.

### 3. Universal provenance sidecar — `meta.yaml`

Provenance lives in a single known filename: **`meta.yaml`**, placed next to the content it describes. Same name for all content types — the `type` field inside the file identifies what kind of content it is.

**Why one name, not type-specific:**
- Implementers check for one known path, not a lookup table or glob pattern
- The `type` field already carries the content type — encoding it in the filename is redundant
- Simpler filesystem scanning, fewer edge cases
- Not syllago-branded — other tools can adopt this convention

**The sidecar travels with the content.** When content is exported, published, or shared, meta.yaml goes with it. This is the portable provenance layer.

### 4. SSH signing from day one

Signing is included in v0.1, not deferred to v0.2. The signature field is optional but verified when present:

- **No signature:** Hash-only verification. Content integrity confirmed, provenance claims are best-effort.
- **Signature present:** Hash + signature verified. Provenance claims are cryptographically bound to an identity.
- **Signature fails:** Hard reject. No override.

Three signing methods, platform-agnostic by design:

| Method | Identity | Key Management | Best For |
|--------|----------|---------------|----------|
| **Sigstore (OIDC)** | Repo URL / CI pipeline identity | None — ephemeral keys, transparency log | CI/CD: GitHub Actions, GitLab CI, CircleCI, Buildkite, Forgejo v15+ |
| **SSH** | Key fingerprint | TOFU with pinning | Local publishing, self-hosted git without OIDC |
| **Sigstore (email)** | Google/Microsoft/GitHub email via Dex | None — interactive browser flow | Individual publishers without CI/CD |

**Why Sigstore is primary, not SSH-only:**

Sigstore's trust anchor is the OIDC protocol, not any single platform. Fulcio (the certificate authority) accepts OIDC tokens from any compliant issuer — GitHub, GitLab, CircleCI, Buildkite, self-hosted Forgejo, or custom Keycloak. The identity in the certificate is whatever the OIDC token claims: a repo URL, a CI pipeline, an email. If any single platform disappears, every other OIDC provider works identically.

SSH signing is the fallback for environments without OIDC (self-hosted git, local publishing). It works everywhere, with TOFU (trust-on-first-use) key pinning for verification.

**The Let's Encrypt model for adoption:**

Syllago ships CI/CD workflows (GitHub Action, GitLab CI template) that automatically sign all meta.yaml files on push. Zero friction for publishers — add the workflow, done. The secure thing is the easy thing. For platforms without pre-built workflows, `syllago publish --sign` handles local signing via SSH or interactive Sigstore (email).

**Verification:**

| Method | How to verify |
|--------|--------------|
| Sigstore (OIDC) | Check Rekor transparency log. Identity = repo URL. Publicly verifiable, no keys to distribute. |
| Sigstore (email) | Check Rekor transparency log. Identity = email. Publicly verifiable. |
| SSH | Verify against pinned key or fetch from platform (e.g., `github.com/<user>.keys`). TOFU model. |

**Platform support status (as of 2026-04):**

| Platform | Sigstore OIDC | Status |
|----------|--------------|--------|
| GitHub Actions | Native | Full |
| GitLab CI/CD | Native | Full |
| CircleCI | Supported by Fulcio | Full |
| Buildkite | Supported by Fulcio | Full |
| Forgejo/Codeberg | OIDC merged Jan 2026, full support in v15 | Almost there |
| Self-hosted (any OIDC) | Custom issuer via Fulcio `--oidc-issuer` | Works |
| No CI / no OIDC | N/A — use SSH fallback | SSH always works |

### 5. Single manifest for syllago bookkeeping

Syllago's internal tracking (when you installed something, from where, the UUID, source format) is NOT provenance. It's local bookkeeping that never leaves your machine.

Instead of a `.syllago.yaml` file next to every content item (clutter), one manifest file per scope:

```
~/.syllago/manifest.yaml           # global library tracking
<project>/.syllago/manifest.yaml   # project-level tracking
```

| File | Where | Purpose | Travels? |
|------|-------|---------|----------|
| `meta.yaml` | Next to content | Provenance (portable) | Yes |
| `.syllago/manifest.yaml` | Project root or ~/.syllago/ | Syllago bookkeeping | No |

### 6. Layered trust model

Each layer adds guarantees without changing the layers below:

| Layer | What it proves | Mechanism |
|-------|---------------|-----------|
| Content hash | Content files haven't been modified | SHA-256 of content (mathematical) |
| Meta hash | Provenance claims haven't been modified | SHA-256 of metadata via JCS (mathematical) |
| Signature | A specific identity vouches for both | Sigstore (OIDC) or SSH signature over both hashes |
| Git history | Who changed what and when | Git log / blame (auditable) |

---

## Meta.yaml Schema (Draft)

```yaml
# meta.yaml (same structure for all content types)
meta_version: 1
type: skill                         # skill | agent | hook | rule | mcp | command | loadout
name: code-review
version: 3
description: Reviews code for quality and security issues.

authors:
  - alice <alice@example.com>

source_repo: github.com/alice/code-review
source_commit: abc123def456789
published_at: 2026-04-01T14:32:00Z

content_hash: "sha256:a1b2c3d4..."  # SHA-256 of content files only
meta_hash: "sha256:e5f6a7b8..."     # SHA-256 of metadata (JCS-serialized)

signature:                           # optional — verified when present
  method: sigstore                   # sigstore | ssh
  identity: "github.com/alice/code-review@refs/heads/main"  # OIDC identity (sigstore)
  log_index: 12345678                # Rekor transparency log entry (sigstore)
  value: "AAAAB3NzaC1..."           # base64 signature over content_hash + meta_hash
  # For SSH method, replace identity/log_index with:
  #   algorithm: ssh-ed25519
  #   key_id: "SHA256:nThbg6k..."   # public key fingerprint

derived_from:                        # optional — for forked/converted content
  source: github.com/bob/code-helper
  relation: adapt                    # fork | convert | adapt | compose
  source_hash: "sha256:x9y8z7..."
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `meta_version` | yes | Schema version (integer). Currently `1`. |
| `type` | yes | Content type: `skill`, `agent`, `hook`, `rule`, `mcp`, `command`, `loadout` |
| `name` | yes | Human-readable name |
| `version` | yes | Integer version. Incremented on each publish. |
| `description` | no | Short description of what this content does |
| `authors` | yes | List of authors. Format: `Name <email>` or just `Name` |
| `source_repo` | yes* | Canonical source repository. *Required for published content. |
| `source_commit` | no | Git commit hash at publish time. Verifiable by fetching. |
| `published_at` | yes* | ISO 8601 timestamp. *Required for published content. Computed by tooling. |
| `content_hash` | yes | SHA-256 of content files only. See hash algorithm. |
| `meta_hash` | yes | SHA-256 of metadata fields (JCS-serialized). See hash algorithm. |
| `signature` | no | Signature over both hashes. Verified when present. |
| `signature.method` | yes† | `sigstore` or `ssh`. †Required when signature present. |
| `signature.identity` | yes† (sigstore) | OIDC identity (repo URL / email). |
| `signature.log_index` | yes† (sigstore) | Rekor transparency log entry index. |
| `signature.algorithm` | yes† (ssh) | SSH algorithm (e.g., `ssh-ed25519`). |
| `signature.key_id` | yes† (ssh) | Public key fingerprint. |
| `signature.value` | yes† | Base64-encoded signature. |
| `derived_from` | no | Lineage for forked/converted content. |
| `derived_from.source` | yes‡ | Source content identifier. ‡Required when derived_from present. |
| `derived_from.relation` | yes‡ | Relationship type: `fork`, `convert`, `adapt`, `compose` |
| `derived_from.source_hash` | yes‡ | Content hash of the source at derivation time. |

### Versioning

Integer versions (`1`, `2`, `3`), not semver. Incremented on each publish.

Rationale: "What's a breaking change to a markdown file?" (Prior panel, Aisha Williams). The only question users ask is "am I on the latest?" Integer versions answer that. Semver adds complexity without meaning for most content types.

### Required vs Optional by Distribution Scope

| Scope | Required Fields |
|-------|----------------|
| Local (personal use) | `meta_version`, `type`, `name`, `version`, `content_hash` |
| Team (shared internally) | Above + `authors` |
| Public (published to registry) | Above + `source_repo`, `published_at`, `signature` recommended |

---

## Syllago Manifest Schema (Draft)

```yaml
# .syllago/manifest.yaml
manifest_version: 1
entries:
  skills/code-review:
    id: f10a0a40-186b-4dab-8034-491d86114b76
    source_type: git
    source_format: md
    source_provider: claude-code
    source_url: github.com/alice/code-review
    source_registry: community
    source_visibility: public
    source_scope: global
    added_at: 2026-04-01T14:32:00Z
    added_by: syllago v0.5.0
    has_source: true

  hooks/pre-commit-lint:
    id: 8b2c3d4e-5f6a-7b8c-9d0e-1f2a3b4c5d6e
    source_type: registry
    source_format: sh
    source_provider: claude-code
    source_registry: community
    added_at: 2026-04-02T09:15:00Z
    added_by: syllago v0.5.0
```

---

## Hash Algorithms

Two separate hashes with distinct purposes.

### Content Hash (`content_hash`)

SHA-256 of content files only. Does not include meta.yaml.

#### Input Classification

Determine the content storage type:
- **Single file** (Rules): Step A
- **Directory tree** (Skills, Agents, Commands, Loadouts): Step B
- **JSON fragment** (Hooks, MCP configs): Step C

#### Step A: Single File

1. Read file as raw bytes.
2. SHA-256. Output as lowercase hex.

Verify with: `sha256sum <file>` produces the same result.

#### Step B: Directory Tree

1. **Enumerate files.** Walk content directory recursively. Exclude empty directories. Resolve symlinks whose target is within the content directory (use resolved content under the symlink's path). Exclude symlinks with external targets. Exclude `meta.yaml` (sidecar, not content).
2. **Compute relative paths.** Relative to content root. Forward slashes only. No leading `./`. No trailing `/`.
3. **NFC-normalize paths.** Apply Unicode NFC normalization (UAX #15) to each path string. If two paths collide after NFC normalization, reject as error — content is unpublishable.
4. **JCS-canonicalize JSON files.** For any file ending in `.json`, apply RFC 8785 JCS canonicalization before hashing. Non-JSON files are hashed as raw bytes.
5. **Compute per-file hashes.** SHA-256 of each file's (possibly canonicalized) content. Output as 64-character lowercase hex.
6. **Sort.** Sort entries by NFC-normalized path using raw UTF-8 byte ordering (C locale equivalent).
7. **Concatenate.** For each entry: `{path}\x00{hash}\n` (path = UTF-8 bytes, `\x00` = null byte separator, hash = 64 lowercase hex chars, `\n` = 0x0A). Every entry including the last ends with `\n`.
8. **Final hash.** SHA-256 of the concatenated byte string. Output as lowercase hex.

#### Step C: JSON Fragment

1. Parse as strict JSON (RFC 8259). Reject JSONC or non-conforming input.
2. Apply JCS canonicalization (RFC 8785).
3. SHA-256 of the canonical form. Output as lowercase hex.

If the content type uses multiple JSON files in a directory, apply Step B instead (which includes per-file JCS in sub-step 4).

#### Rules

- **No CRLF normalization.** Mandate LF in published content. CRLF vs LF = different hash = real portability issue.
- **Symlinks:** Resolve if target within content directory, exclude if external. If resolved path collides after NFC normalization, error.
- **Binary files:** Hashed as-is, no line-ending treatment.
- **File permissions:** Excluded from hash. Not content. (Windows has no POSIX mode bits; permissions change on checkout.)
- **Hidden files:** All included. No exclusion list. Publish-time tooling may warn on OS junk (`.DS_Store`). An exclusion list would be an attack surface — excluded files could carry injected content without affecting the hash.
- **Empty directories:** Excluded.
- **meta.yaml:** Excluded from content hash (it's a sidecar, not content).
- **Single file vs directory:** A single file hashed via Step A MUST produce a different hash than a directory containing that same file via Step B. This is inherent — Step B includes the filename in the hash input.
- **Reference implementation + 15 test vectors** required before any tool can claim conformance.

#### Required Test Vectors

| # | Test Case | Validates |
|---|-----------|-----------|
| 1 | Single file, ASCII content | Step A baseline. Matches `sha256sum`. |
| 2 | Single file, UTF-8 content with BOM | BOM preserved in raw bytes. |
| 3 | Directory with 3 ASCII-path files | Step B baseline: sort, concatenate, final hash. |
| 4 | NFC/NFD variant filenames that collide | Must error (unpublishable). |
| 5 | macOS-style NFD path, no collision | NFC normalization produces correct hash matching Linux. |
| 6 | Nested subdirectories (`a/b/c.txt`) | Forward-slash paths, sort is depth-agnostic. |
| 7 | Directory with hidden file (`.env.example`) | Hidden file included in hash. |
| 8 | Directory with empty subdirectory | Empty dir excluded. |
| 9 | Internal symlink | Resolved: symlink's path, target's content. |
| 10 | External symlink | Excluded from hash. |
| 11 | Directory containing `.json` file | JCS-canonicalized before per-file hash. |
| 12 | JSON fragment, different key ordering | Same hash after JCS. |
| 13 | Binary file (PNG) | Raw bytes, no normalization. |
| 14 | Sort edge cases (`a-b` vs `a.b` vs `a/b`) | Raw byte sort determinism. |
| 15 | Same content: single file (A) vs directory with one file (B) | Hashes MUST differ. |

### Meta Hash (`meta_hash`)

SHA-256 of provenance metadata, serialized as canonical JSON per **RFC 8785 (JSON Canonicalization Scheme / JCS)**.

**Steps:**

1. Extract all meta.yaml fields EXCEPT `meta_hash` and `signature`. This INCLUDES `content_hash` — binding metadata to specific content (prevents mix-and-match attack where legitimate metadata is paired with different content).
2. Serialize to canonical JSON using JCS (RFC 8785): deterministic key ordering, no whitespace, UTF-8 NFC normalization
3. SHA-256 the resulting JSON bytes
4. Encode as `sha256:<hex digest>`

**Why JCS, not canonical YAML:** YAML has no standard canonical form — key ordering, quoting styles, whitespace, Unicode normalization, and alias handling all vary across parsers. JSON canonicalization is a solved problem with an RFC. The file stays YAML for human authoring; only the hash input is JSON. One extra serialization step, zero ambiguity across implementations.

### Signature

The signature covers the concatenation of both hashes: `content_hash + "\n" + meta_hash`. This binds content integrity and provenance claims to the signer's identity in a single signature operation.

**Signing:**
- **Sigstore:** CI/CD runner obtains OIDC token → requests ephemeral key from Fulcio → signs hash concatenation → records in Rekor transparency log → stores `identity`, `log_index`, and `value` in meta.yaml.
- **SSH:** `ssh-keygen -Y sign` over the hash concatenation → stores `algorithm`, `key_id`, and `value` in meta.yaml.

**Verification:**
- **Sigstore:** Recompute both hashes, fetch certificate from Rekor by `log_index`, verify signature and OIDC identity match.
- **SSH:** Recompute both hashes, verify signature against pinned key or platform-fetched public key.

---

## Derived-From (Lineage Tracking)

When content is forked, converted, or adapted, `derived_from` records the chain:

| Relation | Meaning | Example |
|----------|---------|---------|
| `fork` | Copied and modified independently | Forking someone's skill to customize |
| `convert` | Mechanical format transformation | Hub-and-spoke conversion (Claude Code → Cursor) |
| `adapt` | Rewritten for different domain/purpose | Narrowing a general skill to a specific framework |
| `compose` | Combined from multiple sources | Merging several skills into one |

The `source_hash` in `derived_from` creates a verifiable chain: you can confirm the source existed and what it looked like at derivation time.

### Cross-Provider Conversion

When syllago converts a skill from Claude Code format to Cursor format via hub-and-spoke:

1. Source skill has `content_hash: sha256:AAA`
2. Converted skill gets a NEW `content_hash: sha256:BBB` (content changed)
3. Converted skill records `derived_from: { source_hash: sha256:AAA, relation: convert }`

The lineage chain is preserved across format boundaries.

---

## Discussion Points

### ~~D1: Hash algorithm — metadata canonicalization~~ RESOLVED

**Decision:** JCS (RFC 8785) for metadata hashing. YAML for authoring, JSON for hashing. See Hash Algorithms section.

### ~~D2: Hash algorithm — file tree ordering~~ RESOLVED

**Decision:** Sorted path:hash pairs. Format: `{path}\x00{sha256_hex}\n`. Raw UTF-8 byte sort (C locale). NFC-normalized paths. JCS-canonicalized JSON files. All hidden files included, no exclusion list. File permissions excluded. Empty dirs excluded. Full algorithm specified in Hash Algorithms section above with 15 test vectors.

### ~~D3: Signature verification — where do public keys come from?~~ RESOLVED

**Decision:** Two methods, platform-agnostic by design.

- **Sigstore (primary):** Identity = OIDC claim (repo URL, pipeline, or email). Verified via Rekor transparency log. No key distribution needed. Works with GitHub Actions, GitLab CI, CircleCI, Buildkite, Forgejo v15+, and any custom OIDC issuer.
- **SSH (fallback):** Identity = key fingerprint. TOFU (trust-on-first-use) with pinning. Keys discoverable via platform APIs (e.g., `github.com/<user>.keys`) or pinned locally.

Syllago ships CI/CD workflows (GitHub Action, GitLab CI template) that auto-sign on push — the Let's Encrypt model for adoption. Local signing via `syllago publish --sign` supports both methods.

### ~~D4: Meta.yaml for JSON-merge types~~ RESOLVED

**Decision:** meta.yaml lives in the content directory in syllago's library, same as all other types. The "JSON merge" is an installation mechanism, not a storage model — in the library, hooks and MCP configs are stored as directories (`hook.json` + `meta.yaml` + optional scripts), same structure as skills.

When installed into a provider's settings file (JSON merge), the meta.yaml stays in the library. The manifest tracks what's installed where.

**Deferred: collection-level provenance.** When a repository contains many items (20 hooks, 5 MCP configs), each currently gets its own per-item meta.yaml. A collection-level meta.yaml that covers all items in one signed file is a better publisher experience but introduces design questions about how items travel independently. Deferred to a future version.

### ~~D5: Loadout provenance — bundle vs contents~~ RESOLVED

**Decision:** Manifest only. The loadout's `content_hash` covers the loadout manifest file — the authored document listing items with pinned versions and expected hashes. Each referenced item carries its own meta.yaml with its own provenance independently.

This is the `package.json` model: the manifest is the authored artifact. Each item's `expected_hash` in the manifest is a pin — at install time, syllago verifies each item's actual content_hash matches the pin. Mismatch = install failure.

Updating an item means editing the manifest (new pin), which changes the loadout's content_hash. The loadout's signature covers the manifest, which transitively covers all pinned hashes.

### ~~D6: `authors` field — what about agents?~~ RESOLVED

**Decision:** Authors are always human. Agents are tools, not authors. Optional `generated_by` field for disclosing AI involvement.

```yaml
authors:
  - alice <alice@example.com>
generated_by: claude-code/4.0    # optional — the agent is a tool, not an author
```

### ~~D7: Provenance fingerprint in content files~~ DEFERRED

**Decision:** Not in v0.1. Added to roadmap. A 12-character fingerprint in SKILL.md frontmatter would help reconnect orphaned content to its provenance, but it adds a third metadata location (frontmatter + meta.yaml + manifest). The scenario it solves (content separated from its sidecar) is an edge case better addressed by D9/D10 in v0.1. Revisit when orphaned content becomes a real problem.

### ~~D8: Revocation records~~ OUT OF SCOPE

**Decision:** Revocation is a distribution/registry concern, not a provenance concern. The provenance spec provides the fields that revocation references (`content_hash`, `version`, signing infrastructure), but the revocation mechanism itself (`revocations.json` in registry index, client enforcement, `syllago audit`) belongs in syllago's registry/distribution spec.

Panel research and design documented separately for implementation in the registry layer. See panel notes: signed `revocations.json` in registry index, install-time checks, `syllago audit` with CI exit codes, shrink detection, enterprise multi-source support.

### ~~D9: Zero-provenance behavior~~ RESOLVED

**Decision:** Spec-level: "Content without a meta.yaml sidecar has no provenance. Consumers SHOULD surface this visibly to users and SHOULD NOT treat it identically to content with provenance."

Specific UX behavior (warnings on import/install, audit flags, UI treatment) is a tooling/registry concern, not a spec concern. Added to roadmap for syllago implementation.

### ~~D10: Sidecar stripping attack~~ RESOLVED

**Decision:** Registry/tooling concern, not spec. Defenses (registry-level enforcement requiring meta.yaml for listing, manifest pinning detecting missing/changed sidecars, import warnings) belong in syllago's implementation. The spec's role is making provenance presence/absence unambiguous — which it is: meta.yaml either exists or it doesn't.

### ~~D11: Manifest `added_by` identity~~ OUT OF SCOPE

**Decision:** The manifest is syllago's internal bookkeeping, not part of the provenance spec. Human identity tracking in `added_by` (git identity, system user, or explicit config) is a syllago implementation concern. Added to roadmap.

### ~~D12: Version semantics on fork/adapt~~ RESOLVED

**Decision:** Always start at 1. A fork is a new lineage. `derived_from` records the source and its version/hash, so history isn't lost, but the fork's version number is independent.

---

## Relationship to Other Specs

| Concern | Owner | Interaction |
|---------|-------|-------------|
| Install safety (quarantine, scanning, health signals) | Separate spec: `18-install-safety-strategy.md` | Install safety USES content_hash from provenance. Provenance does not depend on install safety. |
| Activation/triggers (mode, globs, commands) | SKILL.md frontmatter | Triggers are content, not provenance. Covered by content_hash but not provenance fields. |
| Content format (SKILL.md structure) | Agent Skills spec | Provenance is a sidecar, not embedded. No spec changes needed. |
| Distribution (install, update, version pin) | Syllago registry system | Provenance travels with content through distribution. |

## Roadmap (Post-v0.1)

### Revocation System (Registry Layer)

Signed `revocations.json` in registry index repos. Panel-designed, ready for implementation. Key features:

- **Record format:** content_hash, version, severity (critical/high/medium/low), reason (vulnerability/malicious/buggy/deprecated), superseded_by, signed by revoker
- **Client enforcement:** `syllago install` checks revocations before installing (hard block on critical/high). `syllago audit` scans installed content, non-zero exit for CI.
- **Shrink detection:** Client caches revocations hash; warns if file gets smaller between fetches (tampering signal).
- **Trust model:** Author self-revocation (same key as content signature), registry maintainer (registry key), enterprise third-party (key in consumer's trust config).
- **Enterprise support:** Multiple revocation sources via config, org-level overrides, offline/air-gapped via mirrored files.
- **CLI:** `syllago revoke <content> <version> --reason --severity` generates signed entry, optionally opens PR against registry index.

### Collection-Level Provenance

Single meta.yaml covering all items in a repository. Better publisher experience for repos with many items. Design questions around how items travel independently when extracted from a collection.

### Provenance Fingerprint

12-character hash in content file frontmatter (e.g., SKILL.md) for orphan reconnection. Deferred because it adds a third metadata location.

### Key-Level Revocation

Revoke a signing key → flag all content signed by it. Requires key-to-content index.

### Push-Based Revocation Notifications

Webhook/pubsub for revocation propagation without polling.

## Prior Research

These documents contain the full adversarial review and research context:

- `docs/spec/skills/archive/15-provenance-deep-dive.md` — 6-persona adversarial review of provenance design
- `docs/spec/skills/archive/14-adversarial-review-panel.md` — 6-persona review of full metadata convention
- `docs/research/agent-skills-spec/18-install-safety-strategy.md` — install safety strategy (separate concern, uses content_hash)
- `docs/research/agent-skills-spec/07-synthesis.md` — community consensus on what must change
