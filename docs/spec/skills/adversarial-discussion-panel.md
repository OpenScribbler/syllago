# ACP Specification v0.1.0-draft — Adversarial Discussion Panel

**Date:** 2026-04-01
**Spec version reviewed:** v0.1.0-draft (commit 794b90c)
**Panel composition:** 5 independent agents with distinct review lenses
**Method:** Parallel adversarial review — each agent received the full spec with no shared context

---

## Panel Composition

| Panelist | Lens | Focus |
|----------|------|-------|
| Security Researcher | Cryptographic protocols, supply chain attacks | Vulnerabilities, attack vectors, trust model gaps |
| Standards Expert | IETF/W3C/OASIS conformance | Normative language, ambiguity, interoperability |
| Implementer | Go + TypeScript implementation | Algorithm clarity, edge cases, cross-platform |
| Red Team Operator | Attacker perspective | Exploitation scenarios, bypass techniques |
| Technical Auditor | Mechanical consistency | Cross-references, field names, hash values |

---

## Consensus Finding: YAML-to-JSON Type Mapping

**All 5 panelists independently flagged this as the #1 issue.**

Section 8 doesn't specify how YAML fields map to JSON types before JCS serialization. Different YAML parsers handle implicit typing differently (booleans, timestamps, integers vs floats, null vs absent), and these produce different JCS output, which produces different meta hashes.

Specific failure modes identified:
- YAML 1.2 boolean coercion (`yes`, `no`, `on`, `off` → `true`/`false`)
- Timestamp fields parsed as datetime objects vs strings (parser-dependent)
- `version: 1.0` as float vs integer (different JCS serialization)
- Absent optional fields: key omitted vs key present with `null` value
- Multi-line YAML strings (folded/literal block scalars) with varying trailing whitespace
- YAML anchors/aliases must be resolved before serialization

**Recommended fix:** Add a normative type mapping table specifying the expected JSON type for every field. Alternatively, require schema-aware YAML parsing or mandate that all ambiguous values be YAML-quoted.

---

## Structural Issues

### Blocking (interoperability-breaking)

**`published_at` timestamp format under-constrained (Section 6.3.10)**
ISO 8601 permits many representations. `2026-04-01T14:32Z`, `2026-04-01T14:32:00Z`, and `2026-04-01T14:32:00+00:00` are all valid but produce different JCS outputs and different meta hashes. The spec should constrain to a specific ISO 8601 profile (e.g., RFC 3339 with mandatory UTC `Z` suffix and seconds precision).
— *Standards Expert*

**Unknown/extra fields in meta.yaml handling undefined (Section 8)**
Section 8 says "Extract all fields... EXCEPT `meta_hash` and `signature`." This implies unknown fields are included in meta hash computation. But this is never stated explicitly, creating ambiguity: does a consumer that strips unknown fields before hashing conform? Does a publisher adding a future extension field change the meta hash? Forward compatibility depends on this answer.
— *Standards Expert, Implementer, Red Team*

**No signing input test vector (Section 9.1)**
The signing input — the most security-critical byte sequence in the spec — has no test vector. The format (`content_hash_value \n meta_hash_value`) leaves questions: does "value" include the `sha256:` prefix? Is there a trailing newline? A byte-level example would eliminate ambiguity.
— *Standards Expert, Implementer*

### High

**Distribution scope not machine-determinable (Section 6.4)**
Publishers MUST include fields for "the applicable distribution scope" but scope is never encoded in `meta.yaml`. A consumer cannot determine which scope applies or enforce scope-dependent requirements.
— *Standards Expert*

**Symlink resolution needs precision (Section 7.3 step 1)**
"Within the content directory" is imprecise. Questions: relative symlinks through `..` that resolve back inside? Symlinks to directories (recursive enumeration)? Transitive chains (A → B → C)? The spec should define: "the fully resolved absolute path of the symlink target MUST be a descendant of the content directory's absolute path."
— *Standards Expert, Implementer*

### Medium

**`meta.yaml` case sensitivity on case-insensitive filesystems (Section 6.1)**
On macOS/Windows, `META.YAML` and `meta.yaml` are the same file. The spec says filename "MUST be exactly `meta.yaml`" but doesn't specify case-sensitive matching. Publishers on case-insensitive systems could create files that fail on case-sensitive systems.
— *Standards Expert*

**`.git/` included if content directory = repository root (Section 7.5)**
"All files in the content directory are included, including hidden files. No exclusion list is applied." If the content directory is a repo root, `.git/objects/...` would be included. Almost certainly unintended.
— *Standards Expert*

**"content directory" vs "content root" inconsistency (Section 3 vs 7.3)**
Section 3 defines "content directory." Section 7.3 step 2 introduces "content root" without definition. Should be the same term throughout.
— *Standards Expert*

### Low

**Zero-length files unaddressed (Section 7.3)**
Empty dirs are excluded, but zero-length files have a well-defined SHA-256. The spec should state whether they're included (they likely should be — e.g., `__init__.py`).
— *Implementer*

**`source_commit` normative strength inconsistent (Section 6.3.9)**
Heading says "OPTIONAL, RECOMMENDED for public scope." Body says "SHOULD be included." OPTIONAL and SHOULD have different BCP 14 meanings.
— *Standards Expert*

---

## Security Findings

### Critical

**Unsigned content indistinguishable from verified in conformance requirements (Sections 5.2, 9.4)**
Without a signature, hashes only prove content hasn't been tampered with *after publication* — they don't authenticate who published it. But conformance requirements use SHOULD (not MUST) for distinguishing signed from unsigned content. A consumer displaying "Provenance verified" for unsigned content would be conformant.
— *Security Researcher*

**No revocation model for either signing method (Section 11.6)**
For SSH: zero revocation mechanism. A compromised key means unlimited forgery forever. For Sigstore: short-lived certificates help for future signatures, but no mechanism to mark past signatures as suspect during a compromise window.
— *Security Researcher*

### High

**Sidecar stripping as downgrade attack (Section 11.4)**
Attacker strips `meta.yaml`, modifies content, redistributes. The spec allows consumers to accept content without provenance (SHOULD NOT, not MUST NOT). The entire integrity layer is optional.
— *Security Researcher, Red Team*

**False lineage via `derived_from` with real source hashes (Section 6.3.14)**
Attacker claims `derived_from` a trusted skill, includes the real `source_hash` (obtained by hashing the actual source). The `source_hash` verifies, making the false claim more convincing. No mechanism for upstream authors to deny derivation claims.
— *Red Team*

**Authorship impersonation on unsigned content**
Attacker sets `authors: ["linus <torvalds@linux-foundation.org>"]` with valid hashes. Everything verifies. No signature needed.
— *Red Team*

**No identity trust policy for Sigstore (Section 9.2)**
Verification proves "this was signed by someone with a valid OIDC token" but not "this was signed by the *right* person." No mechanism for consumers to specify which identities they trust for which content.
— *Security Researcher, Red Team*

**SSH key trust has no defined bootstrap (Section 9.3)**
"Obtaining the signer's public key via platform API, pinned key store, or other trust mechanism" — the trust mechanism is undefined. An attacker can sign with any key and helpfully provide the public key.
— *Red Team*

**Content-type mismatch attack (Section 6.3.2)**
`type: rule` but content is executable hook code. The `type` field is not validated against actual content. Provenance attests to integrity, not behavioral classification.
— *Red Team*

**Extra fields become "verified metadata" (Section 8)**
Attacker adds `verified_by: "NIST"`, `security_audit: "passed"`, `trust_score: 99`. These get covered by meta_hash (and signature if present), appearing officially attested.
— *Red Team*

### Medium

**Name squatting / namespace confusion (Section 6.3.3)**
No publisher namespacing model. Two publishers can publish different content with the same name, version, and type. Identical to npm/PyPI squatting attacks.
— *Red Team*

**Version rollback attack (Section 6.3.4)**
Old versions with valid hashes/signatures can be redistributed. No freshness mechanism, no "latest version" authority. Consumers receive outdated content with known vulnerabilities.
— *Red Team*

**Unicode homoglyph paths (Section 7.3)**
Files with visually identical names using different Unicode codepoints (e.g., Latin L vs Roman numeral U+216C). NFC normalization doesn't collapse homoglyphs. Both files hash and verify normally.
— *Red Team*

**Windows CRLF line endings break verification (Section 7.5)**
Git's `core.autocrlf` converts LF to CRLF on Windows checkout. Content then fails hash verification. The spec mandates LF and forbids normalization, creating a correctness cliff on Windows.
— *Red Team*

**TOCTOU gap between verification and runtime use (Sections 5.2, 7)**
Content verified at install time but read from disk at runtime. An attacker with filesystem access can modify content after verification passes.
— *Security Researcher*

**Rekor verification underspecified (Section 9.2)**
No inclusion proof / SET verification required. Consumer that fetches by `log_index` and trusts the response is vulnerable to a compromised Rekor instance.
— *Security Researcher*

**Sigstore identity confusion (Section 9.2)**
Attacker creates `github.com/alice-security/code-review` (their own repo). Signs with valid OIDC. Identity looks like "alice" to casual inspection. No requirement that signer identity match claimed author.
— *Red Team*

**Symlink TOCTOU race condition (Section 7.3)**
Between checking a symlink target is internal (step 1) and reading content (step 4), attacker swaps symlink target to external file.
— *Red Team*

### Low

**No downgrade protection in algorithm migration path (Sections 6.3.1, 11.5)**
When `meta_version` 2 arrives, can consumers accept both 1 and 2? If so, attacker publishes with weaker version 1.
— *Security Researcher*

**File permissions excluded from hash (Section 7.5)**
Attacker can make scripts executable or change permissions without affecting hashes.
— *Security Researcher*

**Composition bombing via `derived_from` (Section 6.3.14)**
Attacker adds hundreds of `derived_from` entries pointing to real trusted skills. Visual noise overwhelms reviewers.
— *Red Team*

---

## Test Vector Issues

### From Auditor

**TV-15 per-file hash prefix inconsistency**
The per-file SHA-256 on line 688 uses `sha256:` prefix, but Section 7.2 step 3 specifies output as "64 lowercase hexadecimal characters" (no prefix). All other per-file hashes in TV-01 through TV-14 are bare. TV-15 is informative, but the presentation is inconsistent.

**Schema example placeholder `source_hash`**
Line 131: `sha256:x9y8z7w6...` contains non-hex characters. Other truncated hashes in the example use valid hex prefixes.

### Missing Test Vectors (from Implementer + Standards)

| Missing Vector | Why It Matters |
|---------------|----------------|
| Signing input (Section 9.1) | Most security-critical byte sequence, zero coverage |
| YAML boolean/timestamp edge cases | Would pin down type mapping behavior |
| Absent vs null optional fields | Different JCS output, different meta hash |
| Multiple `derived_from` entries (composition) | Only single-entry tested |
| Path with spaces | Confirms space handling in manifest |
| Unicode beyond BMP (emoji in filename) | Tests multi-byte UTF-8 sort |
| CRLF line endings | Proves different hash from LF |
| Minimal meta.yaml (local scope, only required fields) | Confirms absent fields excluded |
| Symlink to directory | Only file symlinks tested |
| Single file in directory | 7.3 always used, but no single-file directory TV |
| SSH/Sigstore signature verification | No signature TVs at all |
| Content hash format validation (uppercase hex) | Input validation edge case |

---

## What All Panelists Agreed Is Done Well

- Hash binding model (Section 8: content_hash included in meta_hash)
- Fail-closed on unknown `meta_version`
- No override on signature verification failure
- Single algorithm (7.3) eliminating publisher/consumer ambiguity
- NFC normalization with collision detection
- Symlink cycle detection requirement
- Content hash test vector coverage for edge cases
- Honest, non-defensive security considerations section
- Coherent provenance chain across TV-MH and TV-MH2

---

## Triage: What to Fix When

### Before v0.1.0-draft tag (mechanical fixes)

- [ ] TV-15 per-file hash: remove `sha256:` prefix for consistency
- [ ] Schema example `source_hash`: use real hex placeholder

### Before advancing beyond Draft (interoperability requirements)

- [ ] YAML-to-JSON type mapping table (normative)
- [ ] `published_at` format constraint (RFC 3339 profile)
- [ ] Unknown fields handling rule (include in meta_hash or strip)
- [ ] Signing input test vector
- [ ] Symlink resolution precision ("descendant of content directory absolute path")

### Document as known limitations (add to Security Considerations)

- [ ] Unsigned content trust confusion (consider upgrading SHOULD to MUST)
- [ ] Sidecar stripping downgrade attack
- [ ] Self-reported claims exploitability (`authors`, `derived_from`, `type`)
- [ ] Platform-specific verification failures (Windows CRLF)
- [ ] Extra fields as "verified metadata" injection vector

### Track as future work (post-v0.1.0)

- [ ] Revocation model (both SSH and Sigstore)
- [ ] Identity trust policy framework
- [ ] Content-type validation
- [ ] Namespace/name-squatting prevention
- [ ] Version freshness / rollback prevention
- [ ] Registry minimum security requirements
- [ ] Homoglyph detection
- [ ] TOCTOU mitigations

---

## Key Insight from Red Team

> None of the 16 attacks identified require breaking a signature. Every single attack works against unsigned content, and most work even against signed content. The signature model is sound in isolation, but the spec's trust model has a massive gap between "hashes verify" and "content is trustworthy."

This is by design — ACP is a provenance spec, not a trust spec. But the gap must be clearly communicated to prevent consumers from conflating hash verification with trust.
