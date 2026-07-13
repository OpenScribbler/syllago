# 0015. Bundle-based sigstore-go verifier vs manual hashedrekord verification chain

Date: 2026-07-12
Status: Accepted
Feature: capmon-pull

## Context

GitHub's attestations API serves Sigstore v0.3-family bundles containing DSSE envelopes with in-toto SLSA provenance statements. sigstore-go parses and verifies these natively through the same `bundle.UnmarshalJSON` + `verify.NewVerifier` path `VerifyManifest` already uses (`manifest_verify.go:159-171`) — including DSSE signature verification, Rekor inclusion, and certificate identity in one call. The manual chain exists only because MOAT per-item verification has no bundle on the wire (it reconstructs from raw Rekor responses) and it is hardcoded to reject anything that is not `hashedrekord` v0.0.1 (`cli/internal/moat/rekor.go:96-101`); replicating eight hand-rolled steps for a bundle format sigstore-go handles natively would be new security-critical code with no offsetting benefit.

## Decision

Chose **Bundle-based verifier construction, following `VerifyManifest` (`cli/internal/moat/manifest_verify.go:168-197`)** over **Manual 8-step hashedrekord chain, following `VerifyAttestationItem` (`cli/internal/moat/item_verify.go:102-107`, steps documented at `item_verify.go:3-25`)**.

## Consequences

The pull tool's verification is a thin policy declaration over sigstore-go rather than bespoke crypto plumbing, which keeps the fail-closed path small and auditable. The signer identity is pinned as a hardcoded constant (subject `OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`, issuer `https://token.actions.githubusercontent.com`) rather than a TOFU Signing Profile — the publisher is known at compile time, so there is no first-contact problem. `cli/internal/moat/` gains no DSSE code and stays behavior-unchanged; if MOAT ever needs DSSE support, the pull tool's implementation is the in-repo reference.
