//! Build-provenance attestations: a v0.3 Sigstore bundle whose DSSE envelope
//! signs an in-toto statement naming an artifact by digest, as GitHub's
//! `actions/attest-build-provenance` produces. Syllago verifies its
//! capability feed this way; MOAT's own manifests are message signatures and
//! never go through this path (`verify_manifest` still refuses a DSSE
//! bundle).
//!
//! The certificate chain, SCT, transparency-log entry and envelope signature
//! go through the same `sigstore-verify` core as the registry checks, so
//! there is one Sigstore verifier.

use std::time::SystemTime;

use sha2::{Digest, Sha256};
use sigstore_verify::types::{Bundle, MediaType, SignatureContent};

use crate::allowed_signers::hex;
use crate::registry::{require_identity, verify_bundle};
use crate::trust::{Cause, Identity, TrustFailure};
use crate::trust_root::TrustRoot;

const IN_TOTO_PAYLOAD_TYPE: &str = "application/vnd.in-toto+json";
const STATEMENT_V1: &str = "https://in-toto.io/Statement/v1";

/// What a verified attestation's certificate says about the build that
/// signed it, for a caller whose policy goes beyond the SAN and issuer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Attested {
    /// Fulcio's Source Repository URI extension (1.3.6.1.4.1.57264.1.12),
    /// when the certificate carries one.
    pub source_repository_uri: Option<String>,
}

/// Verifies that `bundle` attests `artifact`: the bundle is a v0.3 DSSE
/// bundle over an in-toto v1 statement whose `predicateType` is
/// `predicate_type` and one of whose subjects carries the SHA-256 of
/// `artifact`, it verifies under `root`, and its certificate's SAN URI and
/// issuer are `signer`. Returns the certificate's build claims, which
/// this function does not check.
///
/// The statement is read before the signature is checked only so a refusal
/// can name the subject or predicate that differs; nothing in it is trusted
/// until the whole check passes. A stale bundled root refuses before
/// anything else.
pub fn verify_attestation(
    artifact: &[u8],
    bundle: &[u8],
    predicate_type: &str,
    signer: &Identity,
    root: &TrustRoot,
    now: SystemTime,
) -> Result<Attested, TrustFailure> {
    root.check_fresh(now)?;
    let bundle = std::str::from_utf8(bundle)
        .ok()
        .and_then(|text| Bundle::from_json(text).ok())
        .ok_or_else(|| Cause::BundleMalformed("the bundle is not a Sigstore bundle".into()))?;
    if bundle.media_type != MediaType::Bundle0_3 {
        return Err(Cause::BundleMalformed(format!(
            "the bundle is {}, not a v0.3 bundle",
            bundle.media_type
        ))
        .into());
    }
    let SignatureContent::DsseEnvelope(envelope) = &bundle.content else {
        return Err(Cause::BundleMalformed(
            "the bundle holds a message signature, not a DSSE attestation".into(),
        )
        .into());
    };
    if envelope.payload_type != IN_TOTO_PAYLOAD_TYPE {
        return Err(Cause::StatementMalformed(format!(
            "the envelope's payload type is {:?}, not {IN_TOTO_PAYLOAD_TYPE}",
            envelope.payload_type
        ))
        .into());
    }
    let statement: serde_json::Value = serde_json::from_slice(envelope.payload.as_bytes())
        .map_err(|_| Cause::StatementMalformed("the statement is not JSON".into()))?;
    if statement["_type"] != STATEMENT_V1 {
        return Err(Cause::StatementMalformed(format!(
            "the statement's _type is not {STATEMENT_V1}"
        ))
        .into());
    }
    let found = statement["predicateType"].as_str().unwrap_or_default();
    if found != predicate_type {
        return Err(Cause::PredicateTypeMismatch {
            expected: predicate_type.to_string(),
            found: found.to_string(),
        }
        .into());
    }
    let digest = hex(&Sha256::digest(artifact));
    let subjects = statement["subject"]
        .as_array()
        .ok_or_else(|| Cause::StatementMalformed("the statement has no subject list".into()))?;
    if !subjects
        .iter()
        .any(|subject| subject["digest"]["sha256"] == digest.as_str())
    {
        return Err(Cause::SubjectDigestMismatch { digest }.into());
    }
    let result = verify_bundle(artifact, &bundle, root)?;
    require_identity(&result, signer)?;
    Ok(Attested {
        source_repository_uri: result
            .certificate()
            .and_then(|c| c.ci_claims.source_repository_uri.clone()),
    })
}
