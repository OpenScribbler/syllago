//! Signing a loadout, the refusal text, and the separation between loadout
//! signatures and MOAT's item and list-change signatures.

use std::path::PathBuf;
use std::sync::atomic::{AtomicU32, Ordering};

use library::loadout::{self, Manifest, Record, SignError};
use moat::Signer;
use moat::testing::EmulatedSkSigner;
use sha2::{Digest, Sha256};

const SOURCE_URI: &str = "https://git.example.com/alice/library.git";

fn signer(label: &str) -> EmulatedSkSigner {
    EmulatedSkSigner::ed25519(Sha256::digest(format!("loadout-sign-test-{label}")).into())
}

fn list(signer: &EmulatedSkSigner, sequence: u64) -> String {
    format!(
        "# moat-sequence: {sequence}\n{} {}\n",
        signer.public_key().fingerprint(),
        authorized(signer)
    )
}

/// The key's `<type> <base64 blob>` form, read back from a `rekord` entry,
/// which carries the key in OpenSSH form.
fn authorized(signer: &EmulatedSkSigner) -> String {
    let entry = moat::rekord_entry(b"", "", signer.public_key());
    let entry: serde_json::Value = serde_json::from_slice(&entry).unwrap();
    let encoded = entry["spec"]["signature"]["publicKey"]["content"]
        .as_str()
        .unwrap();
    let text = decode_base64(encoded);
    String::from_utf8(text).unwrap().trim_end().to_string()
}

fn decode_base64(text: &str) -> Vec<u8> {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut bits = 0u32;
    let mut count = 0;
    let mut out = Vec::new();
    for c in text.bytes().filter(|&c| c != b'=') {
        let value = ALPHABET.iter().position(|&a| a == c).unwrap() as u32;
        bits = bits << 6 | value;
        count += 6;
        if count >= 8 {
            count -= 8;
            out.push((bits >> count) as u8);
        }
    }
    out
}

fn manifest(sequence: u64) -> Manifest {
    let text = format!(
        "{{\"sequence\": \"{sequence}\", \"providers\": [\"codex\"], \"items\": \
         [{{\"id\": \"0f8fad5b-d9cb-469f-a165-70867728950e\", \"attestation_hash\": \
         \"sha256:{}\"}}]}}\n",
        "ab".repeat(32)
    );
    Manifest::parse("work.json", text.as_bytes()).unwrap()
}

#[test]
fn signed_loadout_verifies_and_applies() {
    let a = signer("A");
    let current = manifest(5);
    let signature = loadout::sign_loadout(&a, SOURCE_URI, &current).unwrap();
    let list = list(&a, 0);
    let verified =
        loadout::verify_loadout(SOURCE_URI, &current, Some(&signature), list.as_bytes()).unwrap();
    assert_eq!(verified.fingerprint, a.public_key().fingerprint());
    let record = Record {
        sequence: 4,
        digest: manifest(4).digest().into(),
        active: true,
    };
    let decision = loadout::decide(
        SOURCE_URI,
        &current,
        Some(&signature),
        list.as_bytes(),
        Some(&record),
    );
    assert_eq!(decision, Ok(loadout::Decision::Apply));
}

#[test]
fn sign_refuses_an_invalid_source_uri() {
    let a = signer("A");
    for uri in ["", "https://x\n", "https://x\rinjected"] {
        assert_eq!(
            loadout::sign_loadout(&a, uri, &manifest(1)),
            Err(SignError::SourceUriInvalid),
            "{uri:?}"
        );
        let refusal = loadout::loadout_message(uri, &manifest(1)).unwrap_err();
        assert_eq!(refusal.id(), "syllago.loadout.source_uri_invalid");
    }
}

#[test]
fn malformed_trusted_list_is_refused() {
    let a = signer("A");
    let manifest = manifest(1);
    let signature = loadout::sign_loadout(&a, SOURCE_URI, &manifest).unwrap();
    let refusal =
        loadout::verify_loadout(SOURCE_URI, &manifest, Some(&signature), b"no keys\n").unwrap_err();
    assert_eq!(refusal.id(), "allowed_signers_malformed");
}

#[test]
fn sequence_not_greater_names_the_loadout_and_both_recoveries() {
    let a = signer("A");
    let manifest = manifest(3);
    let signature = loadout::sign_loadout(&a, SOURCE_URI, &manifest).unwrap();
    let record = Record {
        sequence: 3,
        digest: format!("sha256:{}", "00".repeat(32)),
        active: true,
    };
    let refusal = loadout::decide(
        SOURCE_URI,
        &manifest,
        Some(&signature),
        list(&a, 0).as_bytes(),
        Some(&record),
    )
    .unwrap_err();
    let text = refusal.to_string();
    assert!(
        text.starts_with("loadout \"work\": sequence_not_greater: "),
        "{text}"
    );
    assert!(text.contains("re-sign it at a sequence above 3"), "{text}");
    assert!(
        text.contains("remove this repository's trusted list"),
        "{text}"
    );
}

#[test]
fn missing_signature_names_the_loadout() {
    let refusal = loadout::verify_loadout(
        SOURCE_URI,
        &manifest(1),
        None,
        list(&signer("A"), 0).as_bytes(),
    )
    .unwrap_err();
    assert_eq!(refusal.id(), "signature_invalid");
    assert_eq!(refusal.loadout(), "work");
    assert!(refusal.to_string().contains(".sig"));
}

/// A directory of this test's own under the system temp dir, removed on
/// drop, even after a panic. Nothing outside it is ever removed.
struct TempDir(PathBuf);

impl TempDir {
    fn new(label: &str) -> TempDir {
        static NEXT: AtomicU32 = AtomicU32::new(0);
        let n = NEXT.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("loadout-test-{}-{n}-{label}", std::process::id()));
        std::fs::create_dir(&path).unwrap();
        TempDir(path)
    }
}

impl Drop for TempDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// A content item ingested from a one-file skill in a directory of its own.
fn content_hash() -> moat::ContentHash {
    let dir = TempDir::new("skill");
    std::fs::write(dir.0.join("SKILL.md"), "Use this to demo loadouts.\n").unwrap();
    let body = acif::Body::read(&dir.0).unwrap();
    let item = acif::ingest_body(acif::Kind::Skill, &body, "SKILL.md").unwrap();
    moat::ContentHash::of_acif(&item).unwrap()
}

#[test]
fn loadout_and_moat_signatures_never_cross() {
    let a = signer("A");
    let previous = list(&a, 0);
    let new = list(&a, 1);
    let manifest = manifest(1);
    let hash = content_hash();

    let loadout_signature = loadout::sign_loadout(&a, SOURCE_URI, &manifest).unwrap();
    assert_eq!(
        moat::verify_item(&hash, &loadout_signature, previous.as_bytes()),
        Err(moat::Refusal::SignatureInvalid)
    );
    assert_eq!(
        moat::verify_list_change(
            SOURCE_URI,
            previous.as_bytes(),
            new.as_bytes(),
            &loadout_signature
        ),
        Err(moat::Refusal::SignatureInvalid)
    );

    let item_signature = moat::sign_item(&a, &hash).unwrap().signature;
    let change_message = moat::list_change_message(
        SOURCE_URI,
        &moat::AllowedSigners::parse(previous.as_bytes()).unwrap(),
        &moat::AllowedSigners::parse(new.as_bytes()).unwrap(),
    )
    .unwrap();
    let change_signature = moat::sign_message(&a, &change_message).unwrap();
    assert!(moat::verify_item(&hash, &item_signature, previous.as_bytes()).is_ok());
    for signature in [&item_signature, &change_signature] {
        let refusal =
            loadout::verify_loadout(SOURCE_URI, &manifest, Some(signature), previous.as_bytes())
                .unwrap_err();
        assert_eq!(refusal.id(), "signature_invalid");
    }
}

#[test]
fn oversized_manifest_is_malformed() {
    let valid = "{\"sequence\": \"1\", \"providers\": [\"codex\"], \"items\": []}\n";
    let at_limit = format!(
        "{valid}{}",
        " ".repeat(loadout::MAX_MANIFEST_BYTES - valid.len())
    );
    assert!(Manifest::parse("work.json", at_limit.as_bytes()).is_ok());
    let over = format!("{at_limit} ");
    let refusal = Manifest::parse("work.json", over.as_bytes()).unwrap_err();
    assert_eq!(refusal.id(), "syllago.loadout.malformed");
    assert!(
        refusal.to_string().contains("larger than 65536 bytes"),
        "{refusal}"
    );
}
