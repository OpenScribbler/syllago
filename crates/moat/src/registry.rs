//! Registry-side Sigstore verification (moat-spec.md §Signature Envelope and
//! §Per-item attestation): the manifest's bundle, and the registry's
//! per-item `hashedrekord` entry rebuilt into a bundle as the reference's
//! `_build_bundle` does.
//!
//! Both check the Fulcio certificate chain, its SCT, the Rekor entry's
//! inclusion proof, checkpoint and SET, and the signature, through
//! `sigstore-verify`, then match the certificate's identity against the
//! registry signing profile. The identity must be a SAN URI, since the spec
//! defines the subject as the certificate's SAN URI; the reference compares
//! the SAN's text whatever its kind.

use std::time::SystemTime;

use sha2::{Digest, Sha256};
use sigstore_verify::rekor::RekorEntryBody;
use sigstore_verify::types::{
    Bundle, CertificateContent, DigestBytes, HashAlgorithm, KindVersion, MediaType, MessageDigest,
    MessageSignature, SignatureContent, VerificationMaterial, VerificationMaterialContent,
};
use sigstore_verify::{SubjectAltName, VerificationPolicy, VerificationResult};

use crate::allowed_signers::hex;
use crate::rekor::{LogEntry, RekorLog};
use crate::trust::{Cause, Identity, TrustFailure};
use crate::trust_root::TrustRoot;
use crate::{ContentHash, attestation_payload};

/// A registry's signing identity, as its manifest's
/// `registry_signing_profile` declares it and as the client pins it.
pub type RegistrySigningProfile = Identity;

/// Verifies a registry manifest against its Sigstore bundle and its
/// declared `registry_signing_profile`, then compares that profile with the
/// client's pin.
///
/// Returns `Ok` (`MOAT_SIGNED`) only when the bundle verifies over
/// `manifest`, its certificate's identity is the declared profile, and the
/// declared profile equals `pinned`. With no pin it returns
/// `MOAT_IDENTITY_UNPINNED` carrying the declared profile, for the caller's
/// trust-on-first-use decision; a declared profile that differs from the pin
/// is `MOAT_IDENTITY_MISMATCH`. A stale bundled root refuses before any
/// verification runs.
pub fn verify_manifest(
    manifest: &[u8],
    bundle: &[u8],
    pinned: Option<&RegistrySigningProfile>,
    root: &TrustRoot,
    now: SystemTime,
) -> Result<(), TrustFailure> {
    root.check_fresh(now)?;
    let declared = declared_profile(manifest)?;
    let bundle = std::str::from_utf8(bundle)
        .ok()
        .and_then(|text| Bundle::from_json(text).ok())
        .ok_or_else(|| Cause::BundleMalformed("the bundle is not a Sigstore bundle".into()))?;
    let result = verify_bundle(manifest, &bundle, root)?;
    require_identity(&result, &declared)?;
    match pinned {
        None => Err(Cause::ProfileUnpinned { declared }.into()),
        Some(pinned) if *pinned != declared => Err(Cause::ProfileChanged {
            pinned: pinned.clone(),
            declared,
        }
        .into()),
        Some(_) => Ok(()),
    }
}

/// Verifies the registry's per-item Rekor entry at `log_index` for `item`:
/// a `hashedrekord` 0.0.1 entry over the item's Attestation Payload, signed
/// by `profile` (the manifest's verified `registry_signing_profile`).
/// Returns the signer.
pub fn verify_registry_attestation(
    item: &ContentHash,
    log_index: u64,
    profile: &RegistrySigningProfile,
    log: &dyn RekorLog,
    root: &TrustRoot,
    now: SystemTime,
) -> Result<Identity, TrustFailure> {
    root.check_fresh(now)?;
    let entry = log.entry_by_index(log_index).map_err(Cause::from)?;
    if entry.log_index() != log_index {
        return Err(Cause::RekorEntryMalformed(format!(
            "asked the log for entry {log_index} and it returned entry {}",
            entry.log_index()
        ))
        .into());
    }
    let payload = attestation_payload(item);
    let bundle = rebuild_bundle(&entry, &payload)?;
    let result = verify_bundle(&payload, &bundle, root)?;
    require_identity(&result, profile)?;
    Ok(profile.clone())
}

/// Reads `registry_signing_profile` from a manifest. Both fields are
/// REQUIRED by the spec; an empty or missing one is refused, where the
/// reference skips the comparison for it.
fn declared_profile(manifest: &[u8]) -> Result<Identity, TrustFailure> {
    let malformed = |detail: &str| Cause::SigningProfileMalformed(detail.to_string());
    let manifest: serde_json::Value =
        serde_json::from_slice(manifest).map_err(|_| malformed("the manifest is not JSON"))?;
    let profile = &manifest["registry_signing_profile"];
    let field = |name: &str| {
        profile[name]
            .as_str()
            .filter(|value| !value.is_empty())
            .map(str::to_string)
            .ok_or_else(|| {
                malformed(&format!(
                    "registry_signing_profile.{name} is missing or empty"
                ))
            })
    };
    Ok(Identity {
        issuer: field("issuer")?,
        subject: field("subject")?,
    })
}

/// Rebuilds a v0.3 bundle from a registry `hashedrekord` entry, as the
/// reference's `_build_bundle` does: the certificate from the entry's public
/// key, the signature and message digest from its body, and the entry
/// itself with its SET and inclusion proof. The entry's data hash must be
/// the SHA-256 of `payload`.
fn rebuild_bundle(entry: &LogEntry, payload: &[u8]) -> Result<Bundle, TrustFailure> {
    let malformed = |detail: String| TrustFailure::from(Cause::RekorEntryMalformed(detail));
    let value: serde_json::Value = serde_json::from_slice(entry.body())
        .map_err(|_| malformed("the body is not JSON".into()))?;
    if value["kind"] != "hashedrekord" || value["apiVersion"] != "0.0.1" {
        return Err(malformed(
            "the registry entry is not a hashedrekord 0.0.1 entry".into(),
        ));
    }
    let RekorEntryBody::HashedRekordV001(body) =
        RekorEntryBody::from_json(entry.body(), "hashedrekord", "0.0.1")
            .map_err(|e| malformed(e.to_string()))?
    else {
        return Err(malformed(
            "the body is not a hashedrekord 0.0.1 body".into(),
        ));
    };
    let hash = &body.spec.data.hash;
    let expected = Sha256::digest(payload);
    if hash.algorithm != HashAlgorithm::Sha2256 || hash.value.as_str() != hex(&expected) {
        return Err(Cause::RekorPayloadMismatch.into());
    }
    let certificate = body
        .spec
        .signature
        .public_key
        .parse_certificate()
        .map_err(|e| malformed(e.to_string()))?;
    let tlog_entry = entry
        .0
        .to_bundle_entry(KindVersion::HashedRekordV001)
        .map_err(|e| malformed(e.to_string()))?;
    let material = VerificationMaterial::new(VerificationMaterialContent::Certificate(
        CertificateContent::new(certificate),
    ))
    .with_tlog_entries(vec![tlog_entry]);
    let signature = MessageSignature::new(body.spec.signature.content.clone()).with_message_digest(
        MessageDigest::new(HashAlgorithm::Sha2256, DigestBytes::new(expected.to_vec())),
    );
    Ok(Bundle::new(
        MediaType::Bundle0_3,
        material,
        SignatureContent::MessageSignature(signature),
    ))
}

/// Runs `sigstore-verify` over `artifact` with no identity policy; the
/// caller matches the identity itself so it can tell a wrong signer from a
/// bad signature.
fn verify_bundle(
    artifact: &[u8],
    bundle: &Bundle,
    root: &TrustRoot,
) -> Result<VerificationResult, TrustFailure> {
    root.verifier
        .verify(artifact, bundle, &VerificationPolicy::any_identity())
        .map_err(|e| Cause::SigstoreVerification(e.to_string()).into())
}

/// Requires the verified certificate's SAN URI and issuer to equal
/// `expected`.
fn require_identity(result: &VerificationResult, expected: &Identity) -> Result<(), TrustFailure> {
    let found = Identity {
        subject: result
            .identity()
            .map(|san| san.as_str().to_string())
            .unwrap_or_default(),
        issuer: result.issuer().unwrap_or_default().to_string(),
    };
    let is_uri = matches!(result.identity(), Some(SubjectAltName::Uri(_)));
    if !is_uri || found != *expected {
        return Err(Cause::IdentityMismatch {
            expected: expected.clone(),
            found,
        }
        .into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::trust_root::TrustRootOrigin;

    /// A public-good bundle and its trusted root; provenance in
    /// tests/fixtures/public-good/README.md.
    const CAPTURED_BUNDLE: &str =
        include_str!("../tests/fixtures/public-good/cosign-v3-blob.sigstore.json");
    const CAPTURED_ARTIFACT: &[u8] =
        include_bytes!("../tests/fixtures/public-good/cosign-v3-blob.txt");
    const CAPTURED_ROOT: &[u8] = include_bytes!("../tests/fixtures/public-good/trusted_root.json");

    fn captured_roots() -> [TrustRoot; 2] {
        let origin = TrustRootOrigin::Invocation {
            path: "trusted_root.json".into(),
            registry: "test".into(),
        };
        [
            TrustRoot::from_override(CAPTURED_ROOT, origin).unwrap(),
            TrustRoot::bundled(),
        ]
    }

    #[test]
    fn captured_public_good_bundle_verifies() {
        let bundle = Bundle::from_json(CAPTURED_BUNDLE).unwrap();
        for root in captured_roots() {
            let result = verify_bundle(CAPTURED_ARTIFACT, &bundle, &root).unwrap();
            assert_eq!(
                result.identity(),
                Some(&SubjectAltName::Email("w.vollprecht@gmail.com".into()))
            );
            assert_eq!(result.issuer(), Some("https://github.com/login/oauth"));
            // An email SAN is never a registry signing profile's subject,
            // even with matching text.
            let as_profile = Identity {
                subject: "w.vollprecht@gmail.com".into(),
                issuer: "https://github.com/login/oauth".into(),
            };
            let err = require_identity(&result, &as_profile).unwrap_err();
            assert_eq!(err.id(), "MOAT_IDENTITY_MISMATCH");
        }
    }

    #[test]
    fn captured_bundle_over_other_bytes_is_invalid() {
        let bundle = Bundle::from_json(CAPTURED_BUNDLE).unwrap();
        for root in captured_roots() {
            let err = verify_bundle(b"test content for cosign!\n", &bundle, &root).unwrap_err();
            assert_eq!(err.id(), "MOAT_INVALID");
            assert_eq!(err.cause.id(), "syllago.moat.sigstore_verification_failed");
        }
    }

    #[test]
    fn declared_profile_requires_both_fields() {
        let ok = br#"{"registry_signing_profile":{"issuer":"https://i","subject":"https://s"}}"#;
        assert_eq!(
            declared_profile(ok).unwrap(),
            Identity {
                subject: "https://s".into(),
                issuer: "https://i".into()
            }
        );
        for bad in [
            &b"not json"[..],
            br#"{}"#,
            br#"{"registry_signing_profile":{"issuer":"https://i"}}"#,
            br#"{"registry_signing_profile":{"issuer":"","subject":"https://s"}}"#,
            br#"{"registry_signing_profile":{"issuer":"https://i","subject":5}}"#,
        ] {
            let err = declared_profile(bad).unwrap_err();
            assert_eq!(err.id(), "MOAT_INVALID");
            assert_eq!(err.cause.id(), "syllago.moat.signing_profile_malformed");
        }
    }
}
