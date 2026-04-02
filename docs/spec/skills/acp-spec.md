# Agent Content Provenance (ACP) Specification

**Version:** 0.1.0 (Draft)
**Status:** Draft
**Date:** 2026-04-01

---

## 1. Abstract

This specification defines a portable sidecar format (`meta.yaml`) for recording provenance of AI coding tool content. ACP provides a uniform model for expressing authorship, integrity, source binding, lineage, and cryptographic signatures across all content types used by AI coding agents — including skills, agents, hooks, rules, MCP configurations, and commands. The specification is tool-agnostic and designed for adoption by any content management or distribution system.

## 2. Introduction

AI coding tool content — skills, rules, hooks, agents, MCP configurations, commands, and other artifacts — is increasingly authored, shared, and installed across tools and organizations. As this ecosystem grows, consumers need answers to basic provenance questions: Who made this? Has it been modified? Where did it come from? What was it derived from?

ACP addresses these questions with a single provenance model applied uniformly to all content types. The specification defines:

- A sidecar file format (`meta.yaml`) placed alongside content
- Two integrity hashes: one for content, one for metadata
- Optional cryptographic signatures via Sigstore or SSH
- A lineage model for tracking forks, conversions, and adaptations

ACP is designed to be tool-agnostic. While syllago is the reference implementation, any content management system can produce and consume ACP metadata.

## 3. Terminology

**content** — Any artifact used by an AI coding agent: a skill, agent definition, hook, rule, MCP configuration, or command.

**content directory** — The directory containing a content item's files (e.g., a skill's `SKILL.md` and supporting files).

**sidecar** — A metadata file placed alongside the content it describes, rather than embedded within it.

**publisher** — An entity that computes hashes, optionally signs, and distributes content with ACP metadata.

**consumer** — An entity that reads, verifies, and acts on ACP metadata when installing or auditing content.

**registry** — A system that indexes and distributes content. Registries MAY impose additional requirements beyond this specification.

**content hash** — A SHA-256 digest of a content item's files, computed according to the algorithm in Section 7.

**meta hash** — A SHA-256 digest of a content item's provenance metadata (including the content hash), serialized as canonical JSON per [RFC8785], computed according to the algorithm in Section 8. The inclusion of the content hash binds metadata to specific content.

**signature** — A cryptographic signature over both hashes, binding content integrity and provenance claims to a verifiable identity.

## 4. Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in
BCP 14 [RFC2119] [RFC8174] when, and only when, they appear in all
capitals, as shown here.

## 5. Conformance

This specification defines three conformance classes:

### 5.1 Publishers

A conforming publisher is any tool or process that generates `meta.yaml` files. A conforming publisher:

- MUST produce `meta.yaml` files conforming to the schema in Section 6.
- MUST compute `content_hash` according to Section 7.
- MUST compute `meta_hash` according to Section 8.
- MUST include all required fields for the applicable distribution scope.
- SHOULD compute signatures according to Section 9 for publicly distributed content.
- MUST use LF line endings in all published content files.

### 5.2 Consumers

A conforming consumer is any tool or process that reads and verifies `meta.yaml` files. A conforming consumer:

- MUST verify `content_hash` by recomputing it according to Section 7 and comparing.
- MUST verify `meta_hash` by recomputing it according to Section 8 and comparing.
- MUST reject content where `content_hash` or `meta_hash` verification fails.
- MUST verify the `signature` field when present, according to Section 9.
- MUST reject content where signature verification fails. There is no override.
- SHOULD surface the absence of `meta.yaml` visibly to users.
- SHOULD NOT treat content without `meta.yaml` identically to content with provenance.

### 5.3 Registries

A conforming registry is any system that indexes or distributes content with ACP metadata. A conforming registry:

- MUST verify `content_hash` and `meta_hash` before accepting content for listing.
- SHOULD require `meta.yaml` for listed content.
- MAY impose additional requirements (e.g., mandatory signatures, minimum fields).

## 6. The meta.yaml Format

### 6.1 File Placement

ACP metadata is stored in a file named `meta.yaml`, placed in the content directory alongside the content it describes. The filename MUST be exactly `meta.yaml` for all content types.

When content is exported, published, or shared, `meta.yaml` MUST travel with the content.

Content without a `meta.yaml` sidecar has no provenance.

### 6.2 Schema

The file MUST be valid YAML ([YAML 1.2]). All fields use lowercase snake_case keys.

```yaml
meta_version: 1
type: skill
name: code-review
version: 3
description: Reviews code for quality and security issues.

authors:
  - alice <alice@example.com>

generated_by: claude-code/4.0

source_repo: github.com/alice/code-review
source_commit: abc123def456789
published_at: 2026-04-01T14:32:00Z

content_hash: "sha256:9c9c3591..."
meta_hash: "sha256:ff9c3684..."

signature:
  method: sigstore
  identity: "github.com/alice/code-review@refs/heads/main"
  log_index: 12345678
  value: "AAAAB3NzaC1..."

derived_from:
  - source: github.com/bob/skill-collection/skills/code-helper
    relation: adapt
    source_hash: "sha256:x9y8z7w6..."
```

### 6.3 Field Definitions

#### 6.3.1 meta_version (REQUIRED)

Integer. The version of the ACP schema. Currently `1`.

Consumers MUST reject `meta.yaml` files with an unrecognized `meta_version`. Fail-closed.

#### 6.3.2 type (REQUIRED)

String. The content type. MUST be one of: `skill`, `agent`, `hook`, `rule`, `mcp`, `command`.

#### 6.3.3 name (REQUIRED)

String. Human-readable name of the content.

> **Note (Informative):** The `name` field alone does not guarantee uniqueness across publishers. Registries that index content from multiple publishers SHOULD implement a namespacing scheme (e.g., `publisher/name`) to prevent collisions. The namespacing model is a registry concern outside the scope of this specification.

#### 6.3.4 version (REQUIRED)

Integer. The content version, incremented on each publish. Versions start at `1`. On derivation (fork, conversion, adaptation, or composition), the version resets to `1` (see Section 10.2).

#### 6.3.5 description (OPTIONAL)

String. A short description of what the content does.

#### 6.3.6 authors (REQUIRED for team and public scope)

List of strings. Each entry identifies a human author. Format: `Name <email>` or `Name`.

> **Note (Informative):** The `authors` field is intended for human attribution. AI agents that generate content are tools, not authors — use the `generated_by` field to record agent involvement.

#### 6.3.7 generated_by (OPTIONAL)

String. Identifies the AI agent or tool used to generate the content. Format: `tool-name/version` (e.g., `claude-code/4.0`).

This field is informational. It does not affect provenance verification.

#### 6.3.8 source_repo (REQUIRED for public scope)

String. The canonical source repository identifier. MUST be a URL without scheme, in the format `host/owner/repo` (e.g., `github.com/alice/code-review`). Implementations MAY prepend `https://` when constructing clickable links.

> **Note (Informative):** `source_repo` identifies the repository containing this content. The `derived_from.source` field (Section 6.3.14) may optionally append a path to identify a specific content item within a repository.

#### 6.3.9 source_commit (OPTIONAL, RECOMMENDED for public scope)

String. The git commit hash at publish time. SHOULD be included for publicly distributed content to support reproducibility and auditing.

> **Note (Informative):** `source_commit` is a self-reported claim. It is verifiable by fetching the commit from the source repository but is not cryptographically bound to the content unless a signature is present.

#### 6.3.10 published_at (REQUIRED for public scope)

String. ISO 8601 timestamp of publication. MUST be computed by tooling at publish time.

#### 6.3.11 content_hash (REQUIRED)

String. SHA-256 digest of the content files, computed according to the Directory Tree algorithm in Section 7.3. Format: `sha256:<64 lowercase hex characters>`.

#### 6.3.12 meta_hash (REQUIRED)

String. SHA-256 digest of the provenance metadata, computed according to Section 8. Format: `sha256:<64 lowercase hex characters>`.

#### 6.3.13 signature (OPTIONAL)

Object. A cryptographic signature over both hashes. When present, consumers MUST verify it. Verification failure MUST result in rejection.

The `signature` object contains:

**signature.method** (REQUIRED when signature present) — String. `sigstore` or `ssh`.

**For `sigstore` method:**
- **signature.identity** (REQUIRED) — String. The OIDC identity (repo URL, pipeline identity, or email).
- **signature.log_index** (REQUIRED) — Integer. The Rekor transparency log entry index.
- **signature.value** (REQUIRED) — String. Base64-encoded signature.

**For `ssh` method:**
- **signature.algorithm** (REQUIRED) — String. The SSH algorithm (e.g., `ssh-ed25519`).
- **signature.key_id** (REQUIRED) — String. The public key fingerprint.
- **signature.value** (REQUIRED) — String. Base64-encoded signature.

#### 6.3.14 derived_from (OPTIONAL)

List of objects. Records lineage when content is created from existing content. Each entry represents one source and carries its own `relation` describing how that source was used.

When the list contains a single entry, the relation describes the operation (`fork`, `convert`, or `adapt`). When the list contains two or more entries, composition is implied — the output was created by combining multiple sources. Each entry's `relation` describes how that specific source contributed (e.g., one source adapted, another forked).

Each entry contains:

- **source** (REQUIRED) — String. Human-readable identifier of the source content item, in the format `host/owner/repo` or `host/owner/repo/path` (e.g., `github.com/alice/code-review` or `github.com/bob/skill-collection/skills/code-helper`). The `source` field is a hint for humans and tooling — it is NOT a machine-resolvable identifier, since consumers cannot determine from the string alone whether a path component is present. The `source_hash` field is the authoritative cross-reference anchor for matching source content.
- **relation** (REQUIRED) — String. MUST be one of: `fork`, `convert`, `adapt`. Describes how this specific source was used in derivation. Composition is not a per-source relationship — it is implied by the presence of multiple entries (see Section 10.1).
- **source_hash** (REQUIRED) — String. The content hash of the source at derivation time. Format: `sha256:<64 lowercase hex characters>`.

> **Note (Informative):** `derived_from` fields are self-reported claims. The `source_hash` is verifiable if the source content is accessible, but is not independently authenticated without the source's own signature.

### 6.4 Distribution Scope Requirements

| Scope | REQUIRED Fields |
|-------|----------------|
| Local (personal use) | `meta_version`, `type`, `name`, `version`, `content_hash`, `meta_hash` |
| Team (shared internally) | Above + `authors` |
| Public (published to registry) | Above + `source_repo`, `published_at`. `signature` RECOMMENDED. |

## 7. Content Hash Algorithm

The content hash is a SHA-256 digest of the content files. The `meta.yaml` sidecar is excluded.

### 7.1 Algorithm Selection

Both publishers and consumers MUST use the Directory Tree algorithm (Section 7.3) to compute `content_hash`. All provenance-tracked content resides in a content directory alongside its `meta.yaml` sidecar (Section 6.1), so both parties always operate on a directory — even when the directory contains a single content file.

> **Note (Informative):** Section 7.2 defines the per-file hash used as a sub-procedure within 7.3 (step 4). It is not a standalone algorithm for computing `content_hash`. Earlier drafts allowed publishers to choose between 7.2 and 7.3 based on content structure, but this created an ambiguity: consumers had no way to determine which algorithm the publisher used, and the two algorithms produce different hashes for the same content (see TV-15).

### 7.2 Single-File Hash (Sub-procedure)

Computes the SHA-256 digest of a single file's raw bytes. This is used in Section 7.3 step 4, not directly for `content_hash`.

1. Read the file as raw bytes.
2. Compute SHA-256.
3. Output as 64 lowercase hexadecimal characters.

### 7.3 Directory Tree (Content Hash Algorithm)

1. **Enumerate files.** Walk the content directory recursively. Exclude empty directories. Exclude `meta.yaml`. Resolve symlinks whose target is within the content directory — use the resolved content under the symlink's path. Exclude symlinks with external targets. Implementations MUST detect symlink cycles and reject the content as unpublishable.

2. **Compute relative paths.** Each path MUST be relative to the content root. Implementations MUST use forward slashes (`/`) as the path separator. Paths MUST NOT begin with `./` or end with `/`.

3. **NFC-normalize paths.** Apply Unicode NFC normalization ([UAX15]) to each path string. If two distinct filesystem paths normalize to the same NFC string, the content MUST be rejected as unpublishable.

4. **Compute per-file hashes.** SHA-256 of each file's raw bytes. Output as 64 lowercase hexadecimal characters. All files, including JSON, are hashed as raw bytes with no transformation.

5. **Sort.** Sort entries by NFC-normalized path using raw UTF-8 byte ordering.

6. **Concatenate.** For each entry, produce: `{path}\x00{hash}\n` where `\x00` is the null byte (0x00) and `\n` is the newline byte (0x0A). The path is encoded as UTF-8. The hash is 64 lowercase hexadecimal characters. Every entry, including the last, MUST end with `\n`.

7. **Final hash.** SHA-256 of the concatenated byte string. Output as `sha256:` followed by 64 lowercase hexadecimal characters.

### 7.4 JSON Fragment (Informative)

JSON content files (e.g., hook configurations, MCP settings) are hashed as raw bytes with no canonicalization — the same as any other file in a content directory. No special handling is required; Section 7.3 applies uniformly. Publisher tooling SHOULD enforce consistent JSON formatting as a pre-publish check to avoid hash instability from whitespace or key-order changes.

### 7.5 Content Normalization Rules

- Published content MUST use LF (0x0A) line endings. No CRLF normalization is performed; CRLF content produces a different hash than LF content.
- Binary files are hashed as raw bytes with no transformation.
- File permissions are excluded from the hash.
- All files in the content directory are included, including hidden files. No exclusion list is applied.
- Empty directories are excluded.
- If enumeration produces zero files (e.g., content directory contains only `meta.yaml`), the content MUST be rejected as unpublishable. A content_hash requires at least one content file.

## 8. Meta Hash Algorithm

The meta hash is a SHA-256 digest of provenance metadata bound to the content it describes.

1. Extract all fields from `meta.yaml` EXCEPT `meta_hash` and `signature`.
2. This explicitly INCLUDES `content_hash` — binding the metadata to specific content.
3. Serialize to canonical JSON using JCS ([RFC8785]). The input MUST be a valid JSON object ([RFC8259]).
4. Compute SHA-256 of the resulting bytes.
5. Output as `sha256:` followed by 64 lowercase hexadecimal characters.

Including `content_hash` in the meta hash input ensures that provenance metadata cannot be separated from its content and paired with different content. Without this binding, an attacker could take a legitimate `meta.yaml` from a trusted author and pair it with malicious content — both hashes would verify independently. By including `content_hash` in the meta hash, any such substitution invalidates `meta_hash`.

## 9. Signature

### 9.1 Signing Input

The signature covers the concatenation of both hashes: the `content_hash` value, a newline character (0x0A), and the `meta_hash` value. All three components are UTF-8 encoded strings.

### 9.2 Sigstore Method

1. The CI/CD runner or interactive client obtains an OIDC token from the identity provider.
2. The OIDC token is presented to Fulcio, which issues an ephemeral signing certificate.
3. The signing input (Section 9.1) is signed with the ephemeral key.
4. The signature is recorded in the Rekor transparency log.
5. The `identity`, `log_index`, and `value` fields are stored in `meta.yaml`.

Verification:
1. Recompute both hashes per Sections 7 and 8.
2. Construct the signing input per Section 9.1.
3. Fetch the certificate from Rekor by `log_index`.
4. Verify the signature against the certificate.
5. Verify the OIDC identity in the certificate matches `signature.identity`.

Sigstore signing is supported by any OIDC-compliant CI/CD platform, including GitHub Actions, GitLab CI/CD, CircleCI, Buildkite, and Forgejo (v15+). Custom OIDC issuers are supported via Fulcio configuration.

### 9.3 SSH Method

1. The publisher signs the signing input (Section 9.1) using `ssh-keygen -Y sign` or equivalent.
2. The `algorithm`, `key_id`, and `value` fields are stored in `meta.yaml`.

Verification:
1. Recompute both hashes per Sections 7 and 8.
2. Construct the signing input per Section 9.1.
3. Obtain the signer's public key (via platform API, pinned key store, or other trust mechanism).
4. Verify the signature against the public key.

### 9.4 Verification Requirements

- When `signature` is present, consumers MUST verify it.
- Verification failure MUST result in rejection. There is no override mechanism.
- When `signature` is absent, consumers MUST rely on hash verification only. Consumers SHOULD distinguish unsigned content from signed content in user-facing displays.

## 10. Derived-From (Lineage)

The `derived_from` field records the provenance chain when content is created from existing content.

### 10.1 Relation Types

| Relation | Meaning |
|----------|---------|
| `fork` | Copied and modified independently |
| `convert` | Mechanical format transformation (e.g., cross-provider conversion) |
| `adapt` | Rewritten for a different domain or purpose |

These relations describe how a single source was used. Composition (combining multiple sources) is expressed structurally: a `derived_from` list with two or more entries is a composition. Each entry's `relation` describes how that individual source contributed.

### 10.2 Version Reset

When content is derived from existing content — whether by fork, conversion, adaptation, or composition — the `version` field MUST start at `1`. Derived content is a new lineage. The source versions and hashes are preserved in the `derived_from` entries.

### 10.3 Cross-Format Conversion

When content is mechanically converted between provider formats (e.g., via hub-and-spoke conversion):

1. The converted content receives a new `content_hash` (the content has changed).
2. The converted content records `derived_from` with `relation: convert` and the `source_hash` of the original.

## 11. Security Considerations

### 11.1 Trust Model

ACP provides a layered trust model:

1. **Content hash** provides content integrity (mathematical).
2. **Meta hash** provides metadata integrity (mathematical).
3. **Signature** binds both to a verifiable identity (cryptographic).
4. **Git history** provides an audit trail (operational).

Without a signature, provenance metadata is a set of self-reported claims. Consumers SHOULD treat unsigned provenance as informational, not authoritative.

### 11.2 Hash Binding

The `meta_hash` includes `content_hash` as an input (Section 8). This prevents a "mix-and-match" attack where an attacker pairs legitimate provenance metadata from a trusted author with different (potentially malicious) content. Both `content_hash` and `meta_hash` may verify independently, but `meta_hash` will fail if paired with content it was not computed against.

Without a signature, an attacker can still recompute both hashes for forged content and metadata. The signature (Section 9) is the definitive binding of both to a verifiable identity.

### 11.3 Self-Reported Claims

The following fields are self-reported and not independently verifiable without additional mechanisms:

- `authors` — anyone can claim any authorship
- `source_repo` — anyone can claim any source
- `source_commit` — verifiable by fetching, but not bound to content without a signature
- `derived_from` — source hash is verifiable if the source is accessible, but the claim of derivation itself is self-reported

When a signature is present, these claims are bound to the signer's identity. The claims are as trustworthy as the signer.

### 11.4 Sidecar Removal

An attacker can remove `meta.yaml` from content. The content will still function without provenance. Consumers SHOULD surface the absence of provenance visibly. Registries SHOULD require `meta.yaml` for listed content. Defenses against sidecar removal (manifest pinning, registry enforcement) are implementation concerns outside this specification.

### 11.5 Hash Algorithm Collisions

SHA-256 is used for both content and meta hashes. No practical collision attacks against SHA-256 are known as of this specification's publication date. If SHA-256 is compromised in the future, a new `meta_version` with an updated algorithm would be required.

### 11.6 Signing Key Compromise

If a signing key is compromised, all content signed by that key should be considered suspect. Key-level revocation is outside the scope of this specification. Sigstore's short-lived certificates mitigate this risk by limiting the validity window of each signing event.

### 11.7 Content Execution Risk

All content types covered by this specification can potentially cause code execution when loaded by an AI coding agent. ACP does not assess or attest to the safety of content. Provenance records facts about origin and integrity, not safety judgments. Install safety mechanisms (quarantine, behavioral scanning, health signals) are separate concerns.

## 12. References

### 12.1 Normative References

- **[RFC2119]** Bradner, S., "Key words for use in RFCs to Indicate Requirement Levels", BCP 14, RFC 2119, March 1997. https://www.rfc-editor.org/rfc/rfc2119
- **[RFC8174]** Leiba, B., "Ambiguity of Uppercase vs Lowercase in RFC 2119 Key Words", BCP 14, RFC 8174, May 2017. https://www.rfc-editor.org/rfc/rfc8174
- **[RFC8259]** Bray, T., Ed., "The JavaScript Object Notation (JSON) Data Interchange Format", STD 90, RFC 8259, December 2017. https://www.rfc-editor.org/rfc/rfc8259
- **[RFC8785]** Rundgren, A., Jordan, B., and S. Erdtman, "JSON Canonicalization Scheme (JCS)", RFC 8785, June 2020. https://www.rfc-editor.org/rfc/rfc8785
- **[UAX15]** Davis, M. and K. Whistler, "Unicode Normalization Forms", Unicode Standard Annex #15. https://unicode.org/reports/tr15/
- **[YAML 1.2]** Ben-Kiki, O., Evans, C., and I. döt Net, "YAML Ain't Markup Language Version 1.2", October 2009. https://yaml.org/spec/1.2.2/

### 12.2 Informative References

- **[SIGSTORE]** Sigstore Project, "Sigstore: Software Signing for Everyone". https://docs.sigstore.dev/
- **[REKOR]** Sigstore Project, "Rekor Transparency Log". https://docs.sigstore.dev/logging/overview/
- **[FULCIO]** Sigstore Project, "Fulcio Certificate Authority". https://docs.sigstore.dev/certificate_authority/overview/

---

## Appendix A: Derivation Relation Types (Informative)

| Relation | When to use | Example |
|----------|-------------|---------|
| `fork` | Copying content to modify independently | Forking a code review skill to add team-specific rules |
| `convert` | Mechanical transformation between formats | Converting a Claude Code skill to Cursor format |
| `adapt` | Rewriting for a different domain or purpose | Narrowing a general testing skill to React testing |

**Composition** is expressed by having two or more entries in `derived_from`. Example: merging a linting skill (`relation: adapt`) and a formatting skill (`relation: fork`) into one combined skill.

## Appendix B: Test Vectors (Normative)

Conforming implementations MUST produce the exact hashes listed below for the given inputs. All content uses LF (0x0A) line endings. Hex values are lowercase.

> **Verification status:** These vectors were generated by a Python reference implementation and independently verified using shell-based SHA-256 computation (`sha256sum`). Before this specification advances beyond Draft status, vectors MUST be confirmed by at least two independent implementations in different languages.

### TV-01: Per-file hash, ASCII content (Section 7.2)

Tests the per-file hash sub-procedure used in Section 7.3 step 4.

Input: 12 bytes — `Hello, ACP!\n`

```
Hex: 48656c6c6f2c20414350210a
```

```
Per-file SHA-256: 621ce4cf6cca465755bc79d96598ad73afe693dfa387b063a79710ee6fa2d7fe
```

Validates: Section 7.2 baseline. Result MUST match `sha256sum` of the raw bytes.

### TV-02: Per-file hash, UTF-8 content with BOM (Section 7.2)

Tests the per-file hash sub-procedure with a BOM prefix.

Input: 12 bytes — UTF-8 BOM (EF BB BF) followed by `résumé\n`

```
Hex: efbbbf72c3a973756dc3a90a
```

```
Per-file SHA-256: c08423edeb854b637067004a3f998a7ce42cd0c71828ba9ce7f655bf409f2a3a
```

Validates: BOM is preserved in raw bytes — not stripped before hashing.

### TV-03: Directory with 3 ASCII-path files (Section 7.3)

Input: Three files in a content directory.

| Path | Content (UTF-8) | Per-file SHA-256 |
|------|-----------------|------------------|
| `SKILL.md` | `# Code Review\n` | `ddf68b0b231ead960e6d7bd9ee31cbc961c43435f25fb73958d6f4d3d3150159` |
| `config.yaml` | `timeout: 30\n` | `6e4206a968e12622b1c42b272ee6faa88501e8522f3391be751753ff7d03e008` |
| `lib/helpers.py` | `def greet():\n    return 'hello'\n` | `389066a9ee7ffa69396e2afd59a9ae9d4b34c59aace84751b7a13197a94f271e` |

Hash manifest (sorted by path, format: `{path}\x00{hash}\n`):

```
SKILL.md\x00ddf68b0b231ead960e6d7bd9ee31cbc961c43435f25fb73958d6f4d3d3150159\n
config.yaml\x006e4206a968e12622b1c42b272ee6faa88501e8522f3391be751753ff7d03e008\n
lib/helpers.py\x00389066a9ee7ffa69396e2afd59a9ae9d4b34c59aace84751b7a13197a94f271e\n
```

```
content_hash: sha256:9c9c3591140eae4e0f047060470af98da00629b668f152ac6d4846e64ff91d40
```

Validates: Section 7.3 baseline — enumeration, per-file hashing, sorting, concatenation, final hash.

### TV-04: NFC/NFD collision — MUST error (Section 7.3)

Input: Two files whose paths collide after NFC normalization.

| Path | Unicode | NFC Form |
|------|---------|----------|
| `café.md` | U+00E9 (precomposed é) | `café.md` |
| `café.md` | U+0065 U+0301 (e + combining acute) | `café.md` |

```
Expected: ERROR — content MUST be rejected as unpublishable
```

Validates: NFC collision detection per Section 7.3 step 3. Implementations MUST NOT produce a hash.

### TV-05: macOS-style NFD path, no collision (Section 7.3)

Input: Single file with NFD-encoded path (common on macOS HFS+/APFS).

| Path (filesystem) | Path (NFC, in manifest) | Content | Per-file SHA-256 |
|-------------------|------------------------|---------|------------------|
| `café.md` (e + U+0301) | `café.md` (U+00E9) | `coffee recipes\n` | `7e23cf888d4a88ac968b7dea16d8f93fa80fd1577c298417351d3705fd4370a7` |

```
content_hash: sha256:d0aa645389ad410fcc08150efd784805dafcc8542aeccb422bf7bea189e89b8e
```

Validates: NFC normalization applied to paths before hashing. The manifest uses the NFC form.

### TV-06: Nested subdirectories (Section 7.3)

Input: Three files at different directory depths.

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `a.txt` | `top\n` | `f7de2947c64cb6435e15fb2bef359d1ed5f6356b2aebb7b20535e3772904e6db` |
| `a/b.txt` | `mid\n` | `e9c2db5a0883fdd31b5eec1fe5bd1162b59cf9893ddd36d62a817e195966c075` |
| `a/b/c.txt` | `deep\n` | `64896f89fd11190013b70103e603a1c5826e56b7fb7d2197ab279b0690043599` |

Sort order: `a.txt` < `a/b.txt` < `a/b/c.txt` (forward-slash paths, raw UTF-8 byte ordering).

```
content_hash: sha256:9ea0a30f2b9a2ad72eb7cacff1870916ee64e64ade8c4b8aca4603c5aad0bc43
```

Validates: Forward-slash path separators, depth-agnostic sort by raw bytes.

### TV-07: Directory with hidden file (Section 7.3)

Input: Two files, one hidden (dot-prefixed).

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `.env.example` | `API_KEY=changeme\n` | `4f5b8dfc17ccfa4955df8242895db5119b78ca6fc9d9a0921cd5492da6f28d47` |
| `SKILL.md` | `# My Skill\n` | `e7cf9539b2a8d8ac10a0f09bb6174601bf2a75fd5481fbfebf320bce9489413b` |

Sort order: `.env.example` (0x2E) < `SKILL.md` (0x53).

```
content_hash: sha256:eef6bcd31816fa29b85f5637072f6e289f4e62ef682b96d927e7f32e0b022aa0
```

Validates: Hidden files are included in the hash. All files in the content directory participate.

### TV-08: Directory with empty subdirectory (Section 7.3)

Input: One file `README.md` and one empty directory `empty_dir/`.

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `README.md` | `# Hello\n` | `90f8ec5669cd34183b9b0fdf8b94f5efb4c3672876330f4aa76088c2b4ad17be` |

The empty directory `empty_dir/` is excluded from enumeration.

```
content_hash: sha256:032ef9ad13d8da0ccbc4f1762c995fb03ad689e03bd4dbaa3e734d2d465f2bc7
```

Validates: Empty directories are invisible to the hash algorithm. Result is identical to a directory containing only `README.md`.

### TV-09: Internal symlink (Section 7.3)

Input: `real.txt` is a regular file. `target/link.txt` is a symlink pointing to `real.txt` (target is within the content directory).

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `real.txt` | `I am the real file\n` | `7d1d8026dea16a96a6c93e79e31a824268b254224feee6ad3cb3ef9db03a7572` |
| `target/link.txt` | `I am the real file\n` | `7d1d8026dea16a96a6c93e79e31a824268b254224feee6ad3cb3ef9db03a7572` |

The symlink is resolved: the manifest uses the symlink's path (`target/link.txt`) with the target's content.

```
content_hash: sha256:a3089f05632d131d91dfcefbcb112ba835496f77c9032c023aa8ccca77afae74
```

Validates: Internal symlinks are resolved — symlink's path, target's content. Both entries have the same per-file hash.

### TV-10: External symlink (Section 7.3)

Input: `real.txt` is a regular file. `external.txt` is a symlink pointing to `/etc/passwd` (target is outside the content directory).

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `real.txt` | `only real file\n` | `3637aa48472032288e7c83e04580b2cb5ae1f649359c97a0fe910022c222f202` |

The external symlink `external.txt` is excluded.

```
content_hash: sha256:872c2c1128768d126b814fe0e7c439b608a61a70c9b3ec2ea70ab900ddf4dab1
```

Validates: External symlinks are excluded. Result is identical to a directory containing only `real.txt`.

### TV-11: Directory containing .json file (Section 7.3)

Input: Two files, one JSON.

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `SKILL.md` | `# Linter\n` | `ac8ea4ffc605d33848950df8fb8ceb294701b1af08ad0e0c2e52f0199cc59a92` |
| `config.json` | `{\n  "rules": ["no-eval"],\n  "severity": "error"\n}\n` | `5e374d9402ecd3184acd6d90ee2b10640015e664551284deb9a1f0b81c448de0` |

```
content_hash: sha256:7dd408b6bf5c729c405d030f5522727626d4ae3047a66812c5c21f13a9d5b2ef
```

Validates: JSON files in directories are hashed as raw bytes — whitespace and key order are preserved, no JCS canonicalization.

### TV-12: JSON file in directory — no canonicalization (Section 7.3)

Input: A content directory containing a single JSON file `hooks.json`.

| Path | Content | Per-file SHA-256 |
|------|---------|------------------|
| `hooks.json` | `{"hooks":{"pre_tool_execute":{"command":"echo guard"}}}\n` | `7ed3c77d9a647ce06b5494f59f4628fd254f2da425c7da58fa6abc5e5da6f013` |

```
content_hash: sha256:02157afbf7a9c768b64a92cfcadef2437bc6f449f901d5fbd1c1eb5499571dc7
```

Validates: JSON files are hashed as raw bytes with no canonicalization, like any other file in a content directory.

### TV-13: Binary file — PNG (Section 7.3)

Input: A content directory containing a single binary file `icon.png` — a minimal 1×1 white PNG, 67 bytes.

| Path | Content (hex) | Per-file SHA-256 |
|------|--------------|------------------|
| `icon.png` | (see below) | `08d2521273459ce20781173ac9cbf8f162880a70420b236ab5641a795bb02b54` |

```
icon.png hex (67 bytes):
89504e470d0a1a0a0000000d494844520000000100000001080200000090
7753de0000000c49444154789c63f80f00000101000518d84e000000004945
4e44ae426082
```

> **Note:** Hex is wrapped for readability. Concatenate all lines (removing whitespace) to obtain the 134-character hex string (67 bytes).

```
content_hash: sha256:05e48cad587f4475e3f8eb560f7796f5a2ad36177b93e280b75683ddced98198
```

Validates: Binary files are hashed as raw bytes with no transformation. The content_hash uses Section 7.3 (directory tree) like all provenance-tracked content.

### TV-14: Sort edge cases — `a-b` vs `a.b` vs `a/b` (Section 7.3)

Input: Three files testing byte-level sort determinism.

| Path | Separator byte | Content | Per-file SHA-256 |
|------|---------------|---------|------------------|
| `a-b` | 0x2D (hyphen) | `hyphen\n` | `52f940a6400df8280d40229bfd2c2bd2f2656b0c0a11795a91021641d586eff2` |
| `a.b` | 0x2E (period) | `dot\n` | `5ddbce254c08372e429a250112c6f4593868687ab01e9a126193e5a83560362b` |
| `a/b` | 0x2F (slash) | `slash\n` | `8578a26bad9cf662e6e0cd91540eea63fb2ed5b5b2cebc471364c137b12931e6` |

Sort order: `a-b` (0x2D) < `a.b` (0x2E) < `a/b` (0x2F).

```
content_hash: sha256:9cbe8cc88b731de27a04aebe24fc425034fddc1ceec32f5f968a2e41a675ada4
```

Validates: Paths are sorted by raw UTF-8 byte values, not by locale or filesystem conventions. The slash in `a/b` is a path component, not a separator for this sort.

### TV-15: Per-file hash vs content_hash — domain separation (Informative)

This vector demonstrates why Section 7.2 (per-file hash) cannot be used as a `content_hash` — the two algorithms produce different results for the same content.

Input: Identical bytes `identical content\n` (18 bytes, hex `6964656e746963616c20636f6e74656e740a`).

**Per-file SHA-256 (Section 7.2 sub-procedure):**
```
sha256:ac106884df28663de086413bc3063ea439cca415a191ffe30b73e23ebc5d32a4
```

**content_hash via Section 7.3 (directory containing `file.txt`):**
```
sha256:1cf7969a15d127e9ba099eb8cebb2fa2636ef720e6c044f32be5a28314befc68
```

The hashes differ because Section 7.3 hashes the manifest string (`file.txt\x00{hash}\n`), not the raw content bytes. This domain separation is why `content_hash` always uses Section 7.3 — if publishers could choose either algorithm, consumers would have no way to determine which was used.

### TV-16: Symlink cycle — MUST error (Section 7.3)

Input: Three files where two symlinks form a cycle.

| Path | Type | Target |
|------|------|--------|
| `real.txt` | regular file | — |
| `link-a.txt` | symlink | `link-b.txt` |
| `link-b.txt` | symlink | `link-a.txt` |

Both symlinks have internal targets (within the content directory), but resolving either leads to an infinite cycle.

```
Expected: ERROR — content MUST be rejected as unpublishable
```

Validates: Symlink cycle detection per Section 7.3 step 1. Implementations MUST detect cycles and MUST NOT produce a hash or enter an infinite loop.

### TV-MH: Meta hash computation (Section 8)

Input: All `meta.yaml` fields except `meta_hash` and `signature`. The `content_hash` is TV-03's real directory-tree hash.

```yaml
meta_version: 1
type: skill
name: code-review
version: 3
description: Reviews code for quality and security issues.
authors:
  - alice <alice@example.com>
generated_by: claude-code/4.0
source_repo: github.com/alice/code-review
source_commit: abc123def456789
published_at: 2026-04-01T14:32:00Z
content_hash: "sha256:9c9c3591140eae4e0f047060470af98da00629b668f152ac6d4846e64ff91d40"
```

JCS ([RFC8785]) canonical JSON:

```
{"authors":["alice <alice@example.com>"],"content_hash":"sha256:9c9c3591140eae4e0f047060470af98da00629b668f152ac6d4846e64ff91d40","description":"Reviews code for quality and security issues.","generated_by":"claude-code/4.0","meta_version":1,"name":"code-review","published_at":"2026-04-01T14:32:00Z","source_commit":"abc123def456789","source_repo":"github.com/alice/code-review","type":"skill","version":3}
```

```
meta_hash: sha256:ff9c3684845115f9d204b1cef0a8a1b43d7648663ccdea503331bf5623d097d9
```

Validates: Section 8 — JCS serialization of metadata fields (excluding `meta_hash` and `signature`), with `content_hash` included as input. The `content_hash` is TV-03's real directory-tree hash, demonstrating an end-to-end state where both content and metadata integrity are verifiable.

### TV-MH2: Meta hash with derived_from (Section 8)

Input: Metadata including `derived_from` — the most structurally complex field (a list of objects). Bob adapted Alice's code-review skill (TV-MH) into a React testing skill. The `source_hash` chains back to Alice's `content_hash` from TV-MH (`9c9c3591...`), forming a complete provenance chain across the two meta hash vectors.

```yaml
meta_version: 1
type: skill
name: react-testing
version: 1
description: Testing utilities for React components.
authors:
  - bob <bob@example.com>
source_repo: github.com/bob/react-testing
published_at: 2026-04-01T16:00:00Z
content_hash: "sha256:9ea0a30f2b9a2ad72eb7cacff1870916ee64e64ade8c4b8aca4603c5aad0bc43"
derived_from:
  - source: github.com/alice/code-review
    relation: adapt
    source_hash: "sha256:9c9c3591140eae4e0f047060470af98da00629b668f152ac6d4846e64ff91d40"
```

JCS ([RFC8785]) canonical JSON:

```
{"authors":["bob <bob@example.com>"],"content_hash":"sha256:9ea0a30f2b9a2ad72eb7cacff1870916ee64e64ade8c4b8aca4603c5aad0bc43","derived_from":[{"relation":"adapt","source":"github.com/alice/code-review","source_hash":"sha256:9c9c3591140eae4e0f047060470af98da00629b668f152ac6d4846e64ff91d40"}],"description":"Testing utilities for React components.","meta_version":1,"name":"react-testing","published_at":"2026-04-01T16:00:00Z","source_repo":"github.com/bob/react-testing","type":"skill","version":1}
```

```
meta_hash: sha256:cc2faf328de4c45bd5d9816c3709e29fffdbcb9438dcf076213079f6be7aedc9
```

Validates: JCS serialization of nested structures (`derived_from` list of objects). Keys within nested objects are sorted alphabetically per RFC 8785. The `source_hash` matches TV-MH's `content_hash`, forming a traceable lineage chain. The `version: 1` reflects the reset rule from Section 10.2 (adaptation creates a new lineage). Bob's `content_hash` (TV-06) differs from Alice's (TV-03) because the adapted content has different files.

## Appendix C: Platform Signing Support (Informative)

Sigstore OIDC signing is supported by the following platforms as of April 2026:

| Platform | Status |
|----------|--------|
| GitHub Actions | Full support |
| GitLab CI/CD | Full support |
| CircleCI | Supported via Fulcio |
| Buildkite | Supported via Fulcio |
| Forgejo/Codeberg | OIDC support merged January 2026, full support expected in v15 |
| Custom OIDC issuers | Supported via Fulcio `--oidc-issuer` configuration |

SSH signing is available on all platforms.

## Acknowledgements

This specification was developed through iterative design and adversarial review with multi-persona panels covering standards, security, open-source implementation, enterprise compliance, and daily authoring perspectives.
