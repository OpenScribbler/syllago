//! The extra checks in MOAT's reference/test_author_key.py, with signatures
//! made in-process so they run on every OS, plus the cases where this crate
//! deliberately refuses what the reference accepts.

mod common;

use curve25519_dalek::Scalar;
use ed25519_dalek::{Signer, SigningKey, Verifier};
use moat::{ContentHash, Refusal};
use sha2::{Digest, Sha256, Sha512};

const SOURCE: &str = "https://github.com/owner/repo";
const OTHER_SOURCE: &str = "https://github.com/owner/other-repo";

struct Key {
    signing: SigningKey,
    line: String,
}

impl Key {
    fn new(name: &str) -> Key {
        let signing = SigningKey::from_bytes(&Sha256::digest(name.as_bytes()).into());
        let blob = common::ed25519_key_blob(signing.verifying_key().as_bytes());
        Key {
            line: common::signers_line(&blob),
            signing,
        }
    }

    fn blob(&self) -> Vec<u8> {
        common::ed25519_key_blob(self.signing.verifying_key().as_bytes())
    }

    /// An armored SSHSIG over `message` under `namespace` and `hashalg`.
    fn sign_with(&self, message: &[u8], namespace: &str, hashalg: &str) -> String {
        let signed = signed_data(message, namespace, hashalg);
        let signature = self.signing.sign(&signed).to_bytes();
        armor(&self.blob(), namespace, hashalg, &signature)
    }

    fn sign(&self, message: &[u8]) -> String {
        self.sign_with(message, "file", "sha512")
    }
}

/// The bytes an SSHSIG signer signs for `message`.
fn signed_data(message: &[u8], namespace: &str, hashalg: &str) -> Vec<u8> {
    let hashed = match hashalg {
        "sha512" => Sha512::digest(message).to_vec(),
        "sha256" => Sha256::digest(message).to_vec(),
        _ => unreachable!(),
    };
    let mut signed = b"SSHSIG".to_vec();
    common::put_string(&mut signed, namespace.as_bytes());
    common::put_string(&mut signed, b"");
    common::put_string(&mut signed, hashalg.as_bytes());
    common::put_string(&mut signed, &hashed);
    signed
}

fn armor(key_blob: &[u8], namespace: &str, hashalg: &str, signature: &[u8]) -> String {
    let mut sig_field = Vec::new();
    common::put_string(&mut sig_field, b"ssh-ed25519");
    common::put_string(&mut sig_field, signature);
    let mut blob = b"SSHSIG".to_vec();
    blob.extend_from_slice(&1u32.to_be_bytes());
    common::put_string(&mut blob, key_blob);
    common::put_string(&mut blob, namespace.as_bytes());
    common::put_string(&mut blob, b"");
    common::put_string(&mut blob, hashalg.as_bytes());
    common::put_string(&mut blob, &sig_field);
    let encoded = common::base64(&blob);
    let lines: Vec<&str> = encoded
        .as_bytes()
        .chunks(70)
        .map(|c| std::str::from_utf8(c).unwrap())
        .collect();
    format!(
        "-----BEGIN SSH SIGNATURE-----\n{}\n-----END SSH SIGNATURE-----\n",
        lines.join("\n")
    )
}

fn content_hash() -> ContentHash {
    let doc = common::vectors();
    common::content_hash(&doc["acif"][3])
}

fn list(sequence: u64, lines: &[&str]) -> Vec<u8> {
    let mut text = format!("# moat-sequence: {sequence}\n");
    for line in lines {
        text.push_str(line);
        text.push('\n');
    }
    text.into_bytes()
}

fn item_outcome(signature: &str, list: &[u8]) -> &'static str {
    match moat::verify_item(&content_hash(), signature, list) {
        Ok(_) => "ok",
        Err(refusal) => refusal.id(),
    }
}

#[test]
fn signatures_outside_the_profile_are_invalid() {
    let key = Key::new("moat-test-key-A");
    let list = list(0, &[&key.line]);
    let message = moat::item_message(&content_hash());
    assert_eq!(item_outcome(&key.sign(&message), &list), "ok");
    assert_eq!(
        item_outcome(&key.sign_with(&message, "git", "sha512"), &list),
        "signature_invalid"
    );
    assert_eq!(
        item_outcome(&key.sign_with(&message, "file", "sha256"), &list),
        "signature_invalid"
    );
    let bare = moat::attestation_payload(&content_hash());
    assert_eq!(item_outcome(&key.sign(&bare), &list), "signature_invalid");
}

#[test]
fn list_field_and_sequence_line_whitespace() {
    let key = Key::new("moat-test-key-A");
    let signature = key.sign(&moat::item_message(&content_hash()));
    let good = list(0, &[&key.line]);
    let tabs = String::from_utf8(good.clone())
        .unwrap()
        .replace(' ', "\t")
        .replace("#\tmoat-sequence:\t0", "# moat-sequence: 0");
    assert_eq!(item_outcome(&signature, tabs.as_bytes()), "ok");

    let nbsp = list(0, &[&key.line.replacen(' ', "\u{a0}", 1)]);
    let leading_space = [b" ".as_slice(), &good].concat();
    let leading_tab = [b"\t".as_slice(), &good].concat();
    let trailing_space = String::from_utf8(good.clone())
        .unwrap()
        .replace(": 0\n", ": 0 \n");
    for (label, bad) in [
        ("no-break space", nbsp.as_slice()),
        ("leading space", &leading_space),
        ("leading tab", &leading_tab),
        ("trailing space", trailing_space.as_bytes()),
    ] {
        assert_eq!(
            item_outcome(&signature, bad),
            "allowed_signers_malformed",
            "{label}"
        );
    }
}

#[test]
fn list_changes_bind_source_and_sequence() {
    let key = Key::new("moat-test-key-A");
    let other = Key::new("moat-test-key-B");
    let r0 = list(0, &[&key.line]);
    let r1 = list(1, &[&key.line, &other.line]);
    let r2 = list(2, &[&key.line]);
    let parse = |bytes: &[u8]| moat::AllowedSigners::parse(bytes).unwrap();
    let add = key.sign(&moat::list_change_message(SOURCE, &parse(&r0), &parse(&r1)).unwrap());
    let remove = key.sign(&moat::list_change_message(SOURCE, &parse(&r1), &parse(&r2)).unwrap());

    let signer = moat::verify_list_change(SOURCE, &r0, &r1, &add).unwrap();
    assert_eq!(
        signer.fingerprint,
        moat::Fingerprint::of_key_blob(&key.blob())
    );
    assert!(moat::verify_list_change(SOURCE, &r1, &r2, &remove).is_ok());
    assert_eq!(
        moat::verify_list_change(OTHER_SOURCE, &r0, &r1, &add).unwrap_err(),
        Refusal::SignatureInvalid
    );
    let replay = moat::verify_list_change(SOURCE, &r2, &r1, &add).unwrap_err();
    assert_eq!(
        replay,
        Refusal::SequenceNotGreater {
            recorded: 2,
            new: 1
        }
    );
    assert!(
        replay
            .to_string()
            .contains("remove this source's trusted list")
    );
}

/// The reference raises on an empty `source_uri` or one with a line break,
/// after it has parsed both lists and compared their sequences.
#[test]
fn source_uri_is_checked_after_the_sequence() {
    let key = Key::new("moat-test-key-A");
    let r0 = list(0, &[&key.line]);
    let r1 = list(1, &[&key.line]);
    for source in ["", "https://x\n", "https://x\r"] {
        assert_eq!(
            moat::verify_list_change(source, &r0, &r1, "").unwrap_err(),
            Refusal::SourceUriInvalid,
            "{source:?}"
        );
        assert_eq!(
            moat::verify_list_change(source, &r1, &r0, "")
                .unwrap_err()
                .id(),
            "sequence_not_greater"
        );
        assert_eq!(
            moat::verify_list_change(source, b"", &r1, "")
                .unwrap_err()
                .id(),
            "allowed_signers_malformed"
        );
    }
}

/// Divergence: the reference verifies Ed25519 cofactorlessly and accepts a
/// small-order key, so the identity key "signs" every message with R = B
/// and s = 1. This crate verifies strictly and refuses it.
#[test]
fn small_order_key_forgery_is_refused() {
    let mut identity = [0u8; 32];
    identity[0] = 1;
    let mut signature = [0x66u8; 64];
    signature[0] = 0x58;
    signature[32..].fill(0);
    signature[32] = 1;
    let blob = common::ed25519_key_blob(&identity);
    let list = list(0, &[&common::signers_line(&blob)]);
    let forged = armor(&blob, "file", "sha512", &signature);
    assert_eq!(item_outcome(&forged, &list), "signature_invalid");
}

/// Divergence: the reference also accepts a real key's signature whose R is
/// the identity, a small-order point. With s = k·a, where k is the
/// challenge hash and a the secret scalar, [s]B - [k]A is the identity, so
/// cofactorless verification accepts it. This crate verifies strictly and
/// refuses it.
#[test]
fn small_order_r_from_a_real_key_is_refused() {
    let key = Key::new("moat-test-key-A");
    let list = list(0, &[&key.line]);
    let signed = signed_data(&moat::item_message(&content_hash()), "file", "sha512");
    let public = key.signing.verifying_key();
    let mut r = [0u8; 32];
    r[0] = 1;
    let challenge = Sha512::new()
        .chain_update(r)
        .chain_update(public.as_bytes())
        .chain_update(&signed)
        .finalize();
    let k = Scalar::from_bytes_mod_order_wide(&challenge.into());
    let s = k * key.signing.to_scalar();
    let signature = [r, s.to_bytes()].concat();

    let parsed = ed25519_dalek::Signature::from_slice(&signature).unwrap();
    assert!(public.verify(&signed, &parsed).is_ok());
    assert!(public.verify_strict(&signed, &parsed).is_err());
    let forged = armor(&key.blob(), "file", "sha512", &signature);
    assert_eq!(item_outcome(&forged, &list), "signature_invalid");
}

/// Divergence: an armored signature longer than 16 KiB is refused before
/// it is split into lines. Blank lines after BEGIN pad a good signature,
/// which the reference reads at any length.
#[test]
fn armor_over_16_kib_is_refused() {
    const CAP: usize = 16 * 1024;
    let key = Key::new("moat-test-key-A");
    let list = list(0, &[&key.line]);
    let good = key.sign(&moat::item_message(&content_hash()));
    let body_start = good.find('\n').unwrap() + 1;
    let padded = |len: usize| {
        let pad = "\n".repeat(len - good.len());
        format!("{}{pad}{}", &good[..body_start], &good[body_start..])
    };
    assert_eq!(item_outcome(&padded(CAP), &list), "ok");
    assert_eq!(item_outcome(&padded(CAP + 1), &list), "signature_invalid");
}

#[test]
fn signature_armor_and_envelope_errors() {
    let key = Key::new("moat-test-key-A");
    let list = list(0, &[&key.line]);
    let good = key.sign(&moat::item_message(&content_hash()));
    // Surrounding whitespace and CRLF line ends are tolerated, as in the
    // reference's unarmor.
    let crlf = format!("\n  {}  \n", good.replace('\n', "\r\n"));
    assert_eq!(item_outcome(&crlf, &list), "ok");
    // Corrupt the first character of the base64 body, the line after BEGIN.
    let body_start = good.find('\n').unwrap() + 1;
    let bad_base64 = format!("{}*{}", &good[..body_start], &good[body_start + 1..]);
    for bad in [
        String::new(),
        good.replace("-----BEGIN SSH SIGNATURE-----", "-----BEGIN SIGNATURE-----"),
        good.replace("-----END SSH SIGNATURE-----\n", ""),
        bad_base64,
    ] {
        assert_eq!(item_outcome(&bad, &list), "signature_invalid", "{bad:?}");
    }
}
