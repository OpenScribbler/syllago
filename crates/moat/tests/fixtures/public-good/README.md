# Public-good Sigstore fixture

A bundle captured from the Sigstore public-good instance and the trusted root
it verifies under. `src/registry.rs` tests use it to check that verification
works on real Fulcio, CT and Rekor output, not only on the instance
`tests/pki` generates.

| File | SHA-256 |
|---|---|
| `cosign-v3-blob.sigstore.json` | `f7afd1c87706839b6ac19974ce23e47c4988c22807791e8257734202308923ec` |
| `cosign-v3-blob.txt` | `e194b99df6639501325c5f44da79fe3a8efae6f239bd79443a19787e06fb1764` |
| `trusted_root.json` | `6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66` |

## Provenance

Copied unchanged from the `sigstore-verify` 0.15.1 crate as published on
crates.io, whose `.cargo_vcs_info.json` names commit
`fc308a2f0bcef92f10c199c6d75fa72441b329f7` of
<https://github.com/sigstore/sigstore-rust>, path `crates/sigstore-verify`:

- `test_data/bundles/cosign-v3-blob.sigstore.json`
- `test_data/bundles/cosign-v3-blob.txt`

`trusted_root.json` is copied unchanged from the `sigstore-trust-root` 0.15.1
crate, same commit, path `crates/sigstore-trust-root`, file
`src/trusted_root.json` (the production root that crate embeds). The bundle
carries an RFC 3161 timestamp, and the smaller `public-good.json` root in
`sigstore-verify`'s test data has no timestamp authority that signed it.

The files are Apache-2.0, as are both crates.

The bundle is a v0.3 bundle made by cosign over `cosign-v3-blob.txt`. Its
certificate's SAN is the email `w.vollprecht@gmail.com` with issuer
`https://github.com/login/oauth`, and its `hashedrekord` entry is Rekor log
index 738312748, integrated at 1764787003 (2025-12-03T18:36:43Z). An email SAN
can never be a registry signing profile's subject, so the tests use it for
the cryptographic checks and for that refusal.
