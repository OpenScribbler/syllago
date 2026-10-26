//! Sigstore verification against an instance generated at test time: the
//! manifest bundle, the registry's per-item `hashedrekord` entry, and the
//! Publisher's `rekord` entry for Dual-Attested items.

mod common;
mod pki;

use std::cell::Cell;
use std::collections::BTreeMap;
use std::time::{Duration, SystemTime};

use moat::{
    Cause, ContentHash, Identity, LogEntry, ManifestEntry, RekorError, RekorLog, TrustFailure,
    TrustRoot, TrustState, verify_dual_attested, verify_manifest, verify_registry_attestation,
};
use p256::ecdsa::SigningKey;
use pki::{ISSUER, Instance, LOGGED_AT, Log, SUBJECT, San};
use serde_json::{Value, json};

/// A day after every test entry was logged.
fn now() -> SystemTime {
    SystemTime::UNIX_EPOCH + Duration::from_secs(u64::try_from(LOGGED_AT).unwrap() + 86_400)
}

fn profile() -> Identity {
    Identity {
        subject: SUBJECT.into(),
        issuer: ISSUER.into(),
    }
}

fn item() -> ContentHash {
    common::content_hash(&common::vectors()["acif"][3])
}

fn author_vector(id: &str) -> Value {
    let doc = common::vectors();
    doc["author_key"]["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap()
        .clone()
}

fn key_fingerprint(name: &str) -> String {
    common::vectors()["author_key"]["keys"][name]["fingerprint"]
        .as_str()
        .unwrap()
        .to_string()
}

fn l1() -> Vec<u8> {
    common::list(&common::vectors(), "L1").to_vec()
}

/// The entry inside a Rekor v1 API response.
fn inner(entry: &mut Value) -> &mut Value {
    entry.as_object_mut().unwrap().values_mut().next().unwrap()
}

fn base64_decode(text: &str) -> Vec<u8> {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut bits = 0u32;
    let mut count = 0;
    let mut out = Vec::new();
    for c in text.bytes().filter(|c| *c != b'=') {
        bits = bits << 6 | ALPHABET.iter().position(|a| *a == c).unwrap() as u32;
        count += 6;
        if count >= 8 {
            count -= 8;
            out.push((bits >> count) as u8);
        }
    }
    out
}

/// A Rekor log served from memory.
#[derive(Default)]
struct StubLog {
    by_index: BTreeMap<u64, Value>,
    search: Vec<Value>,
    down: bool,
    searched: Cell<Option<[u8; 32]>>,
}

impl RekorLog for StubLog {
    fn search_by_data_hash(&self, sha256: &[u8; 32]) -> Result<Vec<LogEntry>, RekorError> {
        if self.down {
            return Err(RekorError::Unavailable("connection refused".into()));
        }
        self.searched.set(Some(*sha256));
        self.search
            .iter()
            .map(|entry| LogEntry::from_rekor_json(entry.to_string().as_bytes()))
            .collect()
    }

    fn entry_by_index(&self, index: u64) -> Result<LogEntry, RekorError> {
        if self.down {
            return Err(RekorError::Unavailable("connection refused".into()));
        }
        let entry = self.by_index.get(&index).ok_or(RekorError::NotFound)?;
        LogEntry::from_rekor_json(entry.to_string().as_bytes())
    }
}

/// A test instance with the registry signer's key and certificate.
struct Registry {
    pki: Instance,
    signer: SigningKey,
    cert: Vec<u8>,
    log: Log,
}

impl Registry {
    fn new() -> Registry {
        Registry::with_cert(San::Uri(SUBJECT), ISSUER)
    }

    fn with_cert(san: San<'_>, issuer: &str) -> Registry {
        let pki = Instance::new();
        let signer = pki::key(10);
        let cert = pki.leaf(&signer, san, issuer, LOGGED_AT);
        let log = Log::new(pki.rekor.clone());
        Registry {
            pki,
            signer,
            cert,
            log,
        }
    }

    /// Logs the registry's per-item attestation for `item`.
    fn attest(&mut self, item: &ContentHash) -> u64 {
        let payload = moat::attestation_payload(item);
        let signature = pki::sign(&self.signer, &payload);
        self.log
            .append(pki::hashedrekord_body(&payload, &signature, &self.cert))
    }

    /// Logs a Publisher's `rekord` entry with this body.
    fn publish(&mut self, body: Vec<u8>) -> u64 {
        self.log.append(body)
    }
}

fn manifest() -> Vec<u8> {
    json!({
        "schema_version": 1,
        "registry_signing_profile": {"issuer": ISSUER, "subject": SUBJECT},
        "content": []
    })
    .to_string()
    .into_bytes()
}

fn signed_manifest(registry: &mut Registry, manifest: &[u8]) -> Vec<u8> {
    let signer = registry.signer.clone();
    let cert = registry.cert.clone();
    pki::bundle(&mut registry.log, manifest, &signer, &cert)
        .to_string()
        .into_bytes()
}

fn assert_failure(result: Result<impl std::fmt::Debug, TrustFailure>, state: &str, cause: &str) {
    let failure = result.unwrap_err();
    assert_eq!(
        (failure.id(), failure.cause.id()),
        (state, cause),
        "{failure}"
    );
}

// The manifest bundle.

#[test]
fn manifest_verifies_against_the_pinned_profile() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    let root = registry.pki.trust_root();
    verify_manifest(&manifest, &bundle, Some(&profile()), &root, now()).unwrap();
}

#[test]
fn manifest_without_a_pin_is_unpinned() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    let failure =
        verify_manifest(&manifest, &bundle, None, &registry.pki.trust_root(), now()).unwrap_err();
    assert_eq!(failure.state, TrustState::IdentityUnpinned);
    assert_eq!(
        failure.cause,
        Cause::ProfileUnpinned {
            declared: profile()
        }
    );
}

#[test]
fn manifest_whose_profile_changed_is_a_mismatch() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    let pinned = Identity {
        subject: "https://github.com/owner/old/.github/workflows/moat-registry.yml@refs/heads/main"
            .into(),
        issuer: ISSUER.into(),
    };
    assert_failure(
        verify_manifest(
            &manifest,
            &bundle,
            Some(&pinned),
            &registry.pki.trust_root(),
            now(),
        ),
        "MOAT_IDENTITY_MISMATCH",
        "syllago.moat.signing_profile_changed",
    );
}

#[test]
fn manifest_signed_by_another_identity_is_a_mismatch() {
    let other_subject =
        "https://github.com/attacker/registry/.github/workflows/moat-registry.yml@refs/heads/main";
    for (san, issuer) in [
        (San::Uri(other_subject), ISSUER),
        (San::Uri(SUBJECT), "https://accounts.google.com"),
        // The spec's subject is a SAN URI; an email SAN never matches,
        // even one whose text equals the profile's subject.
        (San::Email(SUBJECT), ISSUER),
    ] {
        let mut registry = Registry::with_cert(san, issuer);
        let manifest = manifest();
        let bundle = signed_manifest(&mut registry, &manifest);
        let failure = verify_manifest(
            &manifest,
            &bundle,
            Some(&profile()),
            &registry.pki.trust_root(),
            now(),
        )
        .unwrap_err();
        assert_eq!(failure.state, TrustState::IdentityMismatch, "{failure}");
        let Cause::IdentityMismatch { expected, .. } = failure.cause else {
            panic!("{failure}");
        };
        assert_eq!(expected, profile());
    }
}

#[test]
fn tampered_manifest_is_invalid() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    let mut tampered = manifest.clone();
    tampered.extend_from_slice(b" ");
    assert_failure(
        verify_manifest(
            &tampered,
            &bundle,
            Some(&profile()),
            &registry.pki.trust_root(),
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.sigstore_verification_failed",
    );
}

#[test]
fn manifest_bundle_with_a_tampered_set_is_invalid() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let mut bundle: Value =
        serde_json::from_slice(&signed_manifest(&mut registry, &manifest)).unwrap();
    let forged = pki::sign(&registry.pki.rekor, b"something else");
    bundle["verificationMaterial"]["tlogEntries"][0]["inclusionPromise"]["signedEntryTimestamp"] =
        common::base64(&forged).into();
    assert_failure(
        verify_manifest(
            &manifest,
            bundle.to_string().as_bytes(),
            Some(&profile()),
            &registry.pki.trust_root(),
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.sigstore_verification_failed",
    );
}

#[test]
fn manifest_signed_under_another_root_is_invalid() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    assert_failure(
        verify_manifest(
            &manifest,
            &bundle,
            Some(&profile()),
            &TrustRoot::bundled(),
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.sigstore_verification_failed",
    );
}

#[test]
fn malformed_bundle_and_profile_are_invalid() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let bundle = signed_manifest(&mut registry, &manifest);
    let root = registry.pki.trust_root();
    assert_failure(
        verify_manifest(
            &manifest,
            b"{\"not\":\"a bundle\"}",
            Some(&profile()),
            &root,
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.bundle_malformed",
    );
    let no_profile = br#"{"schema_version":1,"content":[]}"#;
    assert_failure(
        verify_manifest(no_profile, &bundle, Some(&profile()), &root, now()),
        "MOAT_INVALID",
        "syllago.moat.signing_profile_malformed",
    );
}

#[test]
fn manifest_bundle_must_be_a_v0_3_message_signature() {
    let mut registry = Registry::new();
    let manifest = manifest();
    let root = registry.pki.trust_root();
    // A DSSE envelope by the profile's identity whose in-toto statement
    // names sha256(manifest): it signs the statement, not the manifest.
    let signer = registry.signer.clone();
    let cert = registry.cert.clone();
    let dsse = pki::dsse_bundle(&mut registry.log, &manifest, &signer, &cert);
    assert_failure(
        verify_manifest(
            &manifest,
            dsse.to_string().as_bytes(),
            Some(&profile()),
            &root,
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.bundle_malformed",
    );
    // Older bundle layouts.
    let bundle: Value = serde_json::from_slice(&signed_manifest(&mut registry, &manifest)).unwrap();
    for version in ["0.1", "0.2"] {
        let mut older = bundle.clone();
        older["mediaType"] =
            format!("application/vnd.dev.sigstore.bundle+json;version={version}").into();
        assert_failure(
            verify_manifest(
                &manifest,
                older.to_string().as_bytes(),
                Some(&profile()),
                &root,
                now(),
            ),
            "MOAT_INVALID",
            "syllago.moat.bundle_malformed",
        );
    }
}

#[test]
fn manifest_proof_tree_size_must_match_the_checkpoint() {
    // The manifest's entry at index 5 of a 7-entry tree. An index-5 proof
    // computes the same root at tree size 8, so only the checkpoint's
    // size can catch the inconsistent proof.
    let mut registry = Registry::new();
    registry.publish(b"filler 3".to_vec());
    registry.publish(b"filler 4".to_vec());
    let manifest = manifest();
    let mut bundle: Value =
        serde_json::from_slice(&signed_manifest(&mut registry, &manifest)).unwrap();
    registry.publish(b"filler 6".to_vec());
    let mut entry = registry.log.bundle_entry(5);
    assert_eq!(entry["inclusionProof"]["treeSize"], "7");
    entry["inclusionProof"]["treeSize"] = "8".into();
    bundle["verificationMaterial"]["tlogEntries"][0] = entry;
    assert_failure(
        verify_manifest(
            &manifest,
            bundle.to_string().as_bytes(),
            Some(&profile()),
            &registry.pki.trust_root(),
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.rekor_checkpoint_invalid",
    );
}

#[test]
fn stale_bundled_root_refuses_before_any_verification() {
    let far = SystemTime::UNIX_EPOCH + Duration::from_secs(4_000_000_000);
    let root = TrustRoot::bundled();
    assert_failure(
        verify_manifest(b"not json", b"not a bundle", None, &root, far),
        "MOAT_TRUSTED_ROOT_STALE",
        "syllago.moat.trusted_root_stale",
    );
    let log = StubLog::default();
    assert_failure(
        verify_registry_attestation(&item(), 0, &profile(), &log, &root, far),
        "MOAT_TRUSTED_ROOT_STALE",
        "syllago.moat.trusted_root_stale",
    );
    let entry = ManifestEntry {
        rekor_log_index: 0,
        key_fingerprint: key_fingerprint("A"),
    };
    assert_failure(
        verify_dual_attested(&item(), &entry, &profile(), &l1(), &log, &root, far),
        "MOAT_TRUSTED_ROOT_STALE",
        "syllago.moat.trusted_root_stale",
    );
}

// The registry's per-item attestation.

#[test]
fn registry_attestation_verifies() {
    let mut registry = Registry::new();
    let item = item();
    let index = registry.attest(&item);
    let log = StubLog {
        by_index: [(index, registry.log.api_entry(index))].into(),
        ..StubLog::default()
    };
    let signer = verify_registry_attestation(
        &item,
        index,
        &profile(),
        &log,
        &registry.pki.trust_root(),
        now(),
    )
    .unwrap();
    assert_eq!(signer, profile());
}

#[test]
fn registry_proof_tree_size_must_match_the_checkpoint() {
    // As for the manifest: entry 5 of 7, its proof claiming tree size 8.
    let mut registry = Registry::new();
    registry.publish(b"filler 3".to_vec());
    registry.publish(b"filler 4".to_vec());
    let item = item();
    let index = registry.attest(&item);
    assert_eq!(index, 5);
    registry.publish(b"filler 6".to_vec());
    let mut entry = registry.log.api_entry(index);
    assert_eq!(
        inner(&mut entry)["verification"]["inclusionProof"]["treeSize"],
        7
    );
    inner(&mut entry)["verification"]["inclusionProof"]["treeSize"] = 8.into();
    let log = StubLog {
        by_index: [(index, entry)].into(),
        ..StubLog::default()
    };
    assert_failure(
        verify_registry_attestation(
            &item,
            index,
            &profile(),
            &log,
            &registry.pki.trust_root(),
            now(),
        ),
        "MOAT_INVALID",
        "syllago.moat.rekor_checkpoint_invalid",
    );
}

#[test]
fn registry_attestation_refusals() {
    let mut registry = Registry::new();
    let item = item();
    let index = registry.attest(&item);
    let other_item = common::content_hash(&common::vectors()["acif"][0]);
    let other_index = registry.attest(&other_item);
    let rekord = registry.publish(
        moat::rekord_body(
            &moat::item_message(&item),
            author_vector("TV-AK-01")["signature"].as_str().unwrap(),
        )
        .unwrap(),
    );
    let root = registry.pki.trust_root();
    let entry = |i| registry.log.api_entry(i);
    let check = |log: &StubLog, index: u64, state: &str, cause: &str| {
        assert_failure(
            verify_registry_attestation(&item, index, &profile(), log, &root, now()),
            state,
            cause,
        );
    };

    // The entry at the index attests another item.
    let log = StubLog {
        by_index: [(other_index, entry(other_index))].into(),
        ..StubLog::default()
    };
    check(
        &log,
        other_index,
        "MOAT_INVALID",
        "syllago.moat.rekor_payload_mismatch",
    );

    // The log answers with an entry at another index.
    let log = StubLog {
        by_index: [(other_index, entry(index))].into(),
        ..StubLog::default()
    };
    check(
        &log,
        other_index,
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_malformed",
    );

    // A rekord entry where a hashedrekord one belongs.
    let log = StubLog {
        by_index: [(rekord, entry(rekord))].into(),
        ..StubLog::default()
    };
    check(
        &log,
        rekord,
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_malformed",
    );

    // No entry, and no log.
    check(
        &StubLog::default(),
        index,
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_missing",
    );
    let down = StubLog {
        down: true,
        ..StubLog::default()
    };
    check(
        &down,
        index,
        "MOAT_INVALID",
        "syllago.moat.rekor_unavailable",
    );

    // A forged SET.
    let mut forged = entry(index);
    inner(&mut forged)["verification"]["signedEntryTimestamp"] =
        common::base64(&pki::sign(&registry.pki.rekor, b"other")).into();
    let log = StubLog {
        by_index: [(index, forged)].into(),
        ..StubLog::default()
    };
    check(
        &log,
        index,
        "MOAT_INVALID",
        "syllago.moat.sigstore_verification_failed",
    );

    // Signed by another identity.
    let log = StubLog {
        by_index: [(index, entry(index))].into(),
        ..StubLog::default()
    };
    let other = Identity {
        subject: SUBJECT.into(),
        issuer: "https://accounts.google.com".into(),
    };
    assert_failure(
        verify_registry_attestation(&item, index, &other, &log, &root, now()),
        "MOAT_IDENTITY_MISMATCH",
        "syllago.moat.identity_mismatch",
    );
}

// Dual attestation.

/// A Dual-Attested item: the registry's attestation and these `rekord`
/// bodies, all logged, with the log serving the bodies to a search.
struct Dual {
    registry: Registry,
    item: ContentHash,
    registry_index: u64,
    rekords: Vec<u64>,
}

impl Dual {
    fn new(rekord_bodies: Vec<Vec<u8>>) -> Dual {
        let mut registry = Registry::new();
        let item = item();
        let registry_index = registry.attest(&item);
        let rekords = rekord_bodies
            .into_iter()
            .map(|body| registry.publish(body))
            .collect();
        Dual {
            registry,
            item,
            registry_index,
            rekords,
        }
    }

    fn log(&self) -> StubLog {
        StubLog {
            by_index: [(
                self.registry.log.global_index(self.registry_index),
                self.registry.log.api_entry(self.registry_index),
            )]
            .into(),
            search: self
                .rekords
                .iter()
                .map(|i| self.registry.log.api_entry(*i))
                .collect(),
            ..StubLog::default()
        }
    }

    fn entry(&self, key: &str) -> ManifestEntry {
        ManifestEntry {
            rekor_log_index: self.registry.log.global_index(self.registry_index),
            key_fingerprint: key_fingerprint(key),
        }
    }

    fn verify(
        &self,
        key: &str,
        list: &[u8],
        log: &StubLog,
    ) -> Result<moat::DualAttested, TrustFailure> {
        verify_dual_attested(
            &self.item,
            &self.entry(key),
            &profile(),
            list,
            log,
            &self.registry.pki.trust_root(),
            now(),
        )
    }
}

fn rekord(vector: &str) -> Vec<u8> {
    let message = moat::item_message(&item());
    moat::rekord_body(
        &message,
        author_vector(vector)["signature"].as_str().unwrap(),
    )
    .unwrap()
}

/// A `rekord` body with its public key text replaced.
fn with_public_key(body: &[u8], key_text: &str) -> Vec<u8> {
    let mut value: Value = serde_json::from_slice(body).unwrap();
    value["spec"]["signature"]["publicKey"]["content"] = common::base64(key_text.as_bytes()).into();
    value.to_string().into_bytes()
}

fn public_key_text(body: &[u8]) -> String {
    let value: Value = serde_json::from_slice(body).unwrap();
    let encoded = value["spec"]["signature"]["publicKey"]["content"]
        .as_str()
        .unwrap();
    String::from_utf8(base64_decode(encoded)).unwrap()
}

#[test]
fn dual_attested_item_verifies() {
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    let log = dual.log();
    let verified = dual.verify("A", &l1(), &log).unwrap();
    assert_eq!(verified.registry, profile());
    assert_eq!(verified.registry_log_index, dual.registry_index);
    assert_eq!(verified.author.fingerprint.as_str(), key_fingerprint("A"));
    assert_eq!(verified.author_log_index, dual.rekords[0]);
    // The search is by the SHA-256 of the item message.
    assert_eq!(
        log.searched.get(),
        Some(pki::sha256(&moat::item_message(&dual.item)))
    );
}

#[test]
fn entry_index_is_global_and_proof_index_is_shard_local() {
    // On a sharded log the entry's logIndex (which the SET covers) is
    // global, and its inclusion proof's logIndex is local to the shard.
    let mut dual = Dual::new(vec![rekord("TV-AK-01")]);
    dual.registry.log.shard_offset = 1_000;
    let verified = dual.verify("A", &l1(), &dual.log()).unwrap();
    assert_eq!(verified.registry_log_index, dual.registry_index + 1_000);
    assert_eq!(verified.author_log_index, dual.rekords[0] + 1_000);
}

#[test]
fn rekord_body_is_canonical() {
    let body = rekord("TV-AK-01");
    let text = String::from_utf8(body.clone()).unwrap();
    assert!(text.starts_with(r#"{"apiVersion":"0.0.1","kind":"rekord","spec":{"data":{"hash":{"algorithm":"sha256","value":""#));
    let key = public_key_text(&body);
    assert!(
        key.starts_with("ssh-ed25519 AAAA") && key.ends_with('\n'),
        "{key:?}"
    );
    assert_eq!(key.matches(' ').count(), 1);
    assert_eq!(moat::rekord_body(b"m", "not a signature"), None);
}

#[test]
fn a_later_entry_by_the_key_can_verify() {
    // The first entry by key A carries a comment; the second is clean.
    let clean = rekord("TV-AK-01");
    let commented = with_public_key(&clean, &public_key_text(&clean).replace('\n', " a@b\n"));
    let dual = Dual::new(vec![commented, clean]);
    let verified = dual.verify("A", &l1(), &dual.log()).unwrap();
    assert_eq!(verified.author_log_index, dual.rekords[1]);
}

#[test]
fn rekord_public_key_with_a_comment_is_refused() {
    let clean = rekord("TV-AK-01");
    let commented = with_public_key(&clean, &public_key_text(&clean).replace('\n', " a@b\n"));
    let dual = Dual::new(vec![commented]);
    assert_failure(
        dual.verify("A", &l1(), &dual.log()),
        "MOAT_INVALID",
        "syllago.moat.rekor_pubkey_comment",
    );
}

#[test]
fn no_entry_by_the_profile_key_is_missing() {
    // The log holds key A's entry; the manifest names key B.
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    assert_failure(
        dual.verify("B", &l1(), &dual.log()),
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_missing",
    );
    let dual = Dual::new(vec![]);
    assert_failure(
        dual.verify("A", &l1(), &dual.log()),
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_missing",
    );
}

#[test]
fn signer_outside_the_trusted_list_is_refused() {
    let dual = Dual::new(vec![rekord("TV-AK-02")]);
    assert_failure(
        dual.verify("B", &l1(), &dual.log()),
        "MOAT_INVALID",
        "signer_not_allowed",
    );
}

#[test]
fn entry_key_must_be_the_signature_key() {
    // Key A's public key on key B's signature, with both keys trusted.
    let a = rekord("TV-AK-01");
    let b = rekord("TV-AK-02");
    let mixed = with_public_key(&b, &public_key_text(&a));
    let dual = Dual::new(vec![mixed]);
    let list = common::list(&common::vectors(), "L1+B").to_vec();
    assert_failure(
        dual.verify("A", &list, &dual.log()),
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_malformed",
    );
}

#[test]
fn rekord_for_another_message_is_refused() {
    // A search may return an entry by the right key over another message.
    let body = rekord("TV-AK-01");
    let mut value: Value = serde_json::from_slice(&body).unwrap();
    value["spec"]["data"]["hash"]["value"] = pki::hex(&pki::sha256(b"other")).into();
    let dual = Dual::new(vec![value.to_string().into_bytes()]);
    assert_failure(
        dual.verify("A", &l1(), &dual.log()),
        "MOAT_INVALID",
        "syllago.moat.rekor_payload_mismatch",
    );
}

#[test]
fn rekord_log_evidence_tamper_is_refused() {
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    let rekord_index = dual.rekords[0];
    let evidence = dual.registry.log.evidence(rekord_index, LOGGED_AT);
    let tamper = |edit: &dyn Fn(&mut Value)| {
        let mut log = dual.log();
        edit(inner(&mut log.search[0]));
        log
    };
    let cases: Vec<(&str, StubLog)> = vec![
        (
            "syllago.moat.rekor_set_invalid",
            tamper(&|e| {
                e["verification"]["signedEntryTimestamp"] =
                    common::base64(&pki::sign(&dual.registry.pki.rekor, b"other")).into();
            }),
        ),
        (
            "syllago.moat.rekor_set_invalid",
            tamper(&|e| e["integratedTime"] = (LOGGED_AT + 1).into()),
        ),
        (
            "syllago.moat.rekor_set_invalid",
            tamper(&|e| {
                e["verification"]
                    .as_object_mut()
                    .unwrap()
                    .remove("signedEntryTimestamp");
            }),
        ),
        (
            "syllago.moat.rekor_inclusion_proof_invalid",
            tamper(&|e| {
                e["verification"]["inclusionProof"]["hashes"][0] = pki::hex(&[0; 32]).into()
            }),
        ),
        (
            "syllago.moat.rekor_inclusion_proof_invalid",
            tamper(&|e| {
                e["verification"]
                    .as_object_mut()
                    .unwrap()
                    .remove("inclusionProof");
            }),
        ),
        (
            // Signed by a key the trusted root does not hold.
            "syllago.moat.rekor_checkpoint_invalid",
            tamper(&|e| {
                e["verification"]["inclusionProof"]["checkpoint"] =
                    Log::checkpoint(&pki::key(99), evidence.tree_size, &evidence.root).into();
            }),
        ),
        (
            // Signed by the log, over another tree size.
            "syllago.moat.rekor_checkpoint_invalid",
            tamper(&|e| {
                e["verification"]["inclusionProof"]["checkpoint"] = Log::checkpoint(
                    &dual.registry.pki.rekor,
                    evidence.tree_size + 1,
                    &evidence.root,
                )
                .into();
            }),
        ),
    ];
    for (cause, log) in cases {
        assert_failure(dual.verify("A", &l1(), &log), "MOAT_INVALID", cause);
    }
}

#[test]
fn rekord_logged_in_the_future_is_refused() {
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    let before =
        SystemTime::UNIX_EPOCH + Duration::from_secs(u64::try_from(LOGGED_AT).unwrap() - 1);
    // sigstore-verify bounds the registry entry's integrated time by the
    // real clock, so only the rekord check sees `before`.
    assert_failure(
        verify_dual_attested(
            &dual.item,
            &dual.entry("A"),
            &profile(),
            &l1(),
            &dual.log(),
            &dual.registry.pki.trust_root(),
            before,
        ),
        "MOAT_INVALID",
        "syllago.moat.rekor_set_invalid",
    );
}

#[test]
fn dual_refuses_when_the_registry_attestation_fails() {
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    let mut log = dual.log();
    log.by_index.clear();
    assert_failure(
        dual.verify("A", &l1(), &log),
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_missing",
    );
    let down = StubLog {
        down: true,
        ..StubLog::default()
    };
    assert_failure(
        dual.verify("A", &l1(), &down),
        "MOAT_INVALID",
        "syllago.moat.rekor_unavailable",
    );
}

#[test]
fn unparseable_log_answers_are_malformed() {
    assert_eq!(
        LogEntry::from_rekor_json(b"[]").unwrap_err().id(),
        "syllago.moat.rekor_entry_malformed"
    );
    let dual = Dual::new(vec![rekord("TV-AK-01")]);
    let mut log = dual.log();
    log.search.push(json!({"uuid": {"body": 5}}));
    assert_failure(
        dual.verify("A", &l1(), &log),
        "MOAT_INVALID",
        "syllago.moat.rekor_entry_malformed",
    );
}
