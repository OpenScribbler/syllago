# ACP Specification Changelog

All notable changes to the ACP specification are documented in this file.

## [0.2.0] — 2026-04-03

Trust anchor integration: source binding, delegated publishing, and ecosystem positioning.

### Added

- **Source binding verification** (Section 9.2 steps 6–7) — normative `source_repo` binding via Fulcio OID extension `1.3.6.1.4.1.57264.1.12`, with exact string equality matching. Closes the gap between "signature verified" and "signature verified against someone I trust."
- **`publisher_identity` field** (Section 6.3.15) — REQUIRED when signing identity differs from `source_repo` owner (delegated/platform publishing). Verifiers MUST surface the distinction.
- **`repository_owner_id` field** (Section 6.3.16) — RECOMMENDED numeric platform identifier for account resurrection protection.
- **`sigstore_trust_root` field** (Section 6.3.17) — OPTIONAL TUF root reference for enterprise/private Sigstore deployments. Excluded from `meta_hash` (distribution metadata, not content provenance).
- **Section 2 rewrite** — platform-first framing (Section 2.1), trust chain narrative (Section 2.2), specification overview (Section 2.3)
- **First-publish trust (TOFU)** (Section 11.19) — formally acknowledges TOFU semantics with documented limitations and registry policy delegation
- **Source binding residual risks** (Section 11.17) — repo takeover, transfer, org multi-committer, workflow manipulation, self-hosted OIDC trust
- **Version rollback WARNING** (Section 11.18) — source binding amplifies rollback convincingness; registries encouraged to maintain latest-version manifests
- **Appendix D** — Provider OIDC certificate extension values (D.1), enterprise self-hosted Sigstore (D.2), sigstore-a2a related work (D.3)
- **TV-MH4** — test vector for `publisher_identity` and `repository_owner_id` in meta hash computation

### Changed

- `publisher_identity` and `repository_owner_id` added to Section 8.1 hashed fields allowlist (meta_version 1)
- Section 8.2 type mapping table updated with new fields
- Section 6.4 distribution scope table updated with new field requirements — added `sigstore_trust_root` to Public scope
- Appendix C Forgejo entry corrected — cannot participate in Sigstore keyless signing (no OIDC `id_tokens` support, no Fulcio issuer type)
- Section 9.2 Forgejo removed from supported platforms list

### Fixed (Panel Review — 13 consensus fixes)

**Consensus (5-0 agreement):**
- `publisher_identity` (6.3.15, 9.2 step 7): added normative text that field is self-reported, MUST NOT be treated as verified identity
- `sigstore_trust_root` (6.3.17, 9.2 step 8): added normative verification behavior (SHOULD use when present) and integrity warning
- Section 9.2 step 5: demoted identity verification from MUST to SHOULD; strict consumers MUST document their algorithm
- TV-MH4: fixed `source_repo` to 3-segment path (`github.com/syllago/community-skills`), recomputed hash
- Section 6.3.15: moved "differs" definition from informative Note to normative text body
- Section 6.4: added `sigstore_trust_root` row to distribution scope table
- Section 11.18: downgraded latest-version manifest from SHOULD to non-normative recommendation
- Section 11.19 + 5.3: demoted TOFU MUST to SHOULD; added first-publish policy requirement to registry conformance

**Additional (3+ votes):**
- Section 9.2 step 4: added Fulcio certificate expiry guidance (verify against Rekor timestamp, not current time)
- Section 9.2 step 7: clarified "surface to user or audit log" for headless CI/CD environments
- Section 9.2 step 6: added normative behavior when OID 1.3.6.1.4.1.57264.1.12 is absent from certificate
- Section 5.2.2: added strict consumer source binding requirement (mirrors 9.2 step 6)
- Appendix D.3: removed RFC 2119 SHOULD applied to third-party spec, replaced with plain prose

## [0.1.0] — 2026-04-02

Initial draft release.

### Added

- Sidecar format (`meta.yaml`) with 12 metadata fields across 3 distribution scopes (local, team, public)
- Content hash algorithm (Section 7) — directory tree hashing with SHA-256, NFC path normalization, symlink resolution
- Meta hash algorithm (Section 8) — explicit field allowlist per `meta_version`, JCS canonicalization, normative YAML-to-JSON type mapping
- Cryptographic signatures (Section 9) — Sigstore and SSH methods with `ACP-V1:` domain separator
- Lineage model (Section 10) — `derived_from` with fork/convert/adapt relations and version reset
- Conformance classes (Section 5) — publishers, consumers (strict/permissive), and registries
- Security considerations (Section 11) — 16 subsections covering trust model through ecosystem security
- 22 test vectors (Appendix B) — content hash, meta hash, signing input, error cases, VCS exclusion
- VCS directory exclusions (`.git/`, `.svn/`, `_svn/`, `.hg/`, `CVS/`)
- Implementation note on CRLF/cross-platform verification (Section 7.6)
