//! Build-provenance attestations (DSSE in-toto bundles) against an instance
//! generated at test time, and the MOAT refusals that must hold beside them.

use std::time::{Duration, SystemTime};

use moat::testing::pki::{self, ISSUER, Instance, LOGGED_AT, Log, San};
use moat::{Identity, TrustFailure, verify_attestation, verify_manifest};
use p256::ecdsa::SigningKey;
use serde_json::{Value, json};

const SLSA: &str = "https://slsa.dev/provenance/v1";
const WORKFLOW: &str =
    "https://github.com/owner/feed/.github/workflows/publish.yml@refs/heads/main";

const REPOSITORY: &str = "https://github.com/owner/feed";

fn now() -> SystemTime {
    SystemTime::UNIX_EPOCH + Duration::from_secs(u64::try_from(LOGGED_AT).unwrap() + 86_400)
}

fn workflow() -> Identity {
    Identity {
        subject: WORKFLOW.into(),
        issuer: ISSUER.into(),
    }
}

struct Signer {
    pki: Instance,
    key: SigningKey,
    cert: Vec<u8>,
    log: Log,
}

impl Signer {
    fn new(san: &str) -> Signer {
        let pki = Instance::new();
        let key = pki::key(20);
        let cert = pki.ci_leaf(&key, San::Uri(san), ISSUER, LOGGED_AT, Some(REPOSITORY));
        let log = Log::new(pki.rekor.clone());
        Signer {
            pki,
            key,
            cert,
            log,
        }
    }

    fn attest(&mut self, statement: &Value) -> Vec<u8> {
        self.attest_payload("application/vnd.in-toto+json", &statement.to_string())
    }

    fn attest_payload(&mut self, payload_type: &str, payload: &str) -> Vec<u8> {
        pki::dsse_statement_bundle(
            &mut self.log,
            payload_type,
            payload.as_bytes(),
            &self.key,
            &self.cert,
        )
        .to_string()
        .into_bytes()
    }
}

/// A provenance statement naming each of `artifacts`, as
/// `attest-build-provenance` names several subject paths.
fn statement(artifacts: &[&[u8]], predicate_type: &str) -> Value {
    let subject: Vec<Value> = artifacts
        .iter()
        .map(|a| json!({"name": "index.json", "digest": {"sha256": pki::hex(&pki::sha256(a))}}))
        .collect();
    json!({
        "_type": "https://in-toto.io/Statement/v1",
        "subject": subject,
        "predicateType": predicate_type,
        "predicate": {"buildDefinition": {}, "runDetails": {}},
    })
}

fn assert_failure<T: std::fmt::Debug>(result: Result<T, TrustFailure>, state: &str, cause: &str) {
    let failure = result.unwrap_err();
    assert_eq!(
        (failure.id(), failure.cause.id()),
        (state, cause),
        "{failure}"
    );
}

#[test]
fn attestation_naming_the_artifact_verifies() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"other", b"artifact", b"third"], SLSA));
    let attested = verify_attestation(b"artifact", &bundle, SLSA, &workflow(), &root, now());
    assert_eq!(
        attested.unwrap().source_repository_uri.as_deref(),
        Some(REPOSITORY)
    );
}

/// The build claims are reported, not checked: a certificate without the
/// Source Repository URI extension still verifies, and says so.
#[test]
fn certificate_without_a_repository_claim_reports_none() {
    let mut signer = Signer::new(WORKFLOW);
    signer.cert = signer
        .pki
        .leaf(&signer.key, San::Uri(WORKFLOW), ISSUER, LOGGED_AT);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"artifact"], SLSA));
    let attested = verify_attestation(b"artifact", &bundle, SLSA, &workflow(), &root, now());
    assert_eq!(attested.unwrap().source_repository_uri, None);
}

#[test]
fn artifact_outside_the_subjects_is_refused() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"artifact"], SLSA));
    let result = verify_attestation(b"tampered", &bundle, SLSA, &workflow(), &root, now());
    assert_failure(
        result,
        "MOAT_INVALID",
        "syllago.moat.subject_digest_mismatch",
    );
}

#[test]
fn another_signer_is_refused() {
    let other = "https://github.com/owner/feed/.github/workflows/other.yml@refs/heads/main";
    let mut signer = Signer::new(other);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"artifact"], SLSA));
    let result = verify_attestation(b"artifact", &bundle, SLSA, &workflow(), &root, now());
    assert_failure(
        result,
        "MOAT_IDENTITY_MISMATCH",
        "syllago.moat.identity_mismatch",
    );
}

#[test]
fn another_predicate_type_is_refused() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"artifact"], "https://spdx.dev/Document/v2.3"));
    let result = verify_attestation(b"artifact", &bundle, SLSA, &workflow(), &root, now());
    assert_failure(
        result,
        "MOAT_INVALID",
        "syllago.moat.predicate_type_mismatch",
    );
}

#[test]
fn statement_swapped_after_signing_is_refused() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let bundle = signer.attest(&statement(&[b"artifact"], SLSA));
    // Replace the signed statement with one naming other bytes; the
    // envelope signature no longer covers it.
    let mut value: Value = serde_json::from_slice(&bundle).unwrap();
    let forged = statement(&[b"tampered"], SLSA).to_string();
    value["dsseEnvelope"]["payload"] = pki::base64(forged.as_bytes()).into();
    let result = verify_attestation(
        b"tampered",
        value.to_string().as_bytes(),
        SLSA,
        &workflow(),
        &root,
        now(),
    );
    assert_failure(
        result,
        "MOAT_INVALID",
        "syllago.moat.sigstore_verification_failed",
    );
}

#[test]
fn non_in_toto_payloads_are_refused() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let text = statement(&[b"artifact"], SLSA).to_string();
    for (payload_type, payload) in [
        ("application/json", text.as_str()),
        ("application/vnd.in-toto+json", "not json"),
        (
            "application/vnd.in-toto+json",
            &text.replace("Statement/v1", "Statement/v0.1"),
        ),
        (
            "application/vnd.in-toto+json",
            &json!({"_type": "https://in-toto.io/Statement/v1", "predicateType": SLSA}).to_string(),
        ),
    ] {
        let bundle = signer.attest_payload(payload_type, payload);
        let result = verify_attestation(b"artifact", &bundle, SLSA, &workflow(), &root, now());
        assert_failure(result, "MOAT_INVALID", "syllago.moat.statement_malformed");
    }
}

#[test]
fn message_signature_and_malformed_bundles_are_refused() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let (key, cert) = (signer.key.clone(), signer.cert.clone());
    let message = pki::bundle(&mut signer.log, b"artifact", &key, &cert).to_string();
    for bundle in [message.as_bytes(), b"not a bundle", b""] {
        let result = verify_attestation(b"artifact", bundle, SLSA, &workflow(), &root, now());
        assert_failure(result, "MOAT_INVALID", "syllago.moat.bundle_malformed");
    }
}

#[test]
fn stale_bundled_root_refuses_first() {
    let root = moat::TrustRoot::bundled();
    let far = SystemTime::UNIX_EPOCH + Duration::from_secs(4_000_000_000);
    let result = verify_attestation(b"artifact", b"not a bundle", SLSA, &workflow(), &root, far);
    assert_failure(
        result,
        "MOAT_TRUSTED_ROOT_STALE",
        "syllago.moat.trusted_root_stale",
    );
}

/// Accepting DSSE attestations here leaves MOAT's manifest path as it was: a
/// DSSE bundle whose statement names the manifest, by the manifest's own
/// signing profile, is still refused.
#[test]
fn manifest_verification_still_refuses_dsse() {
    let mut signer = Signer::new(WORKFLOW);
    let root = signer.pki.trust_root();
    let manifest = json!({
        "schema_version": 1,
        "registry_signing_profile": {"issuer": ISSUER, "subject": WORKFLOW},
        "content": []
    })
    .to_string()
    .into_bytes();
    let bundle = signer.attest(&statement(&[&manifest], SLSA));
    verify_attestation(&manifest, &bundle, SLSA, &workflow(), &root, now()).unwrap();
    let result = verify_manifest(&manifest, &bundle, Some(&workflow()), &root, now());
    assert_failure(result, "MOAT_INVALID", "syllago.moat.bundle_malformed");
}
