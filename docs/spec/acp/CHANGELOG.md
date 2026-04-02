# ACP Specification Changelog

All notable changes to the ACP specification are documented in this file.

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
