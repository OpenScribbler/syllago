//! Signing: the emulated security-key signer against the MOAT vectors, the
//! signing checks, the `rekord` entry, and the security-key policy. The
//! ssh-keygen signer's own test is in ssh_keygen.rs.

mod common;

use std::path::Path;

use moat::testing::EmulatedSkSigner;
use moat::{KeyType, NonSkKeys, Os, SignError, Signer, SkKeyType, SshKeygenSigner, SshPublicKey};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};

/// The reference generator's `_seed`: SHA-256 of `moat-test-key-<label>`.
fn seed(label: &str) -> [u8; 32] {
    Sha256::digest(format!("moat-test-key-{label}")).into()
}

fn vector<'a>(doc: &'a Value, id: &str) -> &'a Value {
    doc["author_key"]["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap()
}

/// A vector key's OpenSSH public-key form, from its allowed-signers line.
fn public_key(doc: &Value, label: &str) -> SshPublicKey {
    let line = doc["author_key"]["keys"][label]["line"].as_str().unwrap();
    SshPublicKey::parse(line.split_once(' ').unwrap().1).unwrap()
}

#[test]
fn emulated_signer_reproduces_tv_ak_06_07_08() {
    let doc = common::vectors();
    let cases = [
        ("TV-AK-06", "D", EmulatedSkSigner::ed25519(seed("D"))),
        ("TV-AK-07", "E", EmulatedSkSigner::ecdsa_p256(seed("E"))),
        (
            "TV-AK-08",
            "D",
            EmulatedSkSigner::ed25519(seed("D")).with_flags(0x00),
        ),
    ];
    for (id, label, signer) in cases {
        let v = vector(&doc, id);
        assert_eq!(v["signer"], label, "{id}");
        assert_eq!(signer.public_key(), &public_key(&doc, label), "{id}");
        let signature = signer
            .sign(v["message"].as_str().unwrap().as_bytes())
            .unwrap();
        assert_eq!(signature, v["signature"].as_str().unwrap(), "{id}");
    }
}

#[test]
fn sign_item_output_verifies() {
    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    for (label, list, signer) in [
        ("D", "LD", EmulatedSkSigner::ed25519(seed("D"))),
        ("E", "LE", EmulatedSkSigner::ecdsa_p256(seed("E"))),
    ] {
        let signed = moat::sign_item(&signer, &hash).unwrap();
        let verified = moat::verify_item(&hash, &signed.signature, common::list(&doc, list));
        assert_eq!(
            verified.unwrap().fingerprint.as_str(),
            doc["author_key"]["keys"][label]["fingerprint"],
            "{label}"
        );
    }
}

/// TV-AK-08's signer: its output lacks the user-presence flag, so the
/// signing check refuses it as the verifier would.
#[test]
fn sign_item_refuses_a_signature_without_user_presence() {
    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    let signer = EmulatedSkSigner::ed25519(seed("D")).with_flags(0x00);
    let err = moat::sign_item(&signer, &hash).unwrap_err();
    assert_eq!(err, SignError::SignatureUnverified);
    assert_eq!(err.id(), "syllago.moat.signature_unverified");
}

/// A signer whose output is by another key than the one it reports.
struct WrongKey {
    reported: SshPublicKey,
    actual: EmulatedSkSigner,
}

impl Signer for WrongKey {
    fn public_key(&self) -> &SshPublicKey {
        &self.reported
    }

    fn sign(&self, message: &[u8]) -> Result<String, SignError> {
        self.actual.sign(message)
    }
}

#[test]
fn sign_message_refuses_a_signature_by_another_key() {
    let doc = common::vectors();
    let signer = WrongKey {
        reported: public_key(&doc, "E"),
        actual: EmulatedSkSigner::ed25519(seed("D")),
    };
    assert_eq!(
        moat::sign_message(&signer, b"message").unwrap_err(),
        SignError::SignatureUnverified
    );
}

/// The proposed entry carries exactly what `rekor-cli upload --type rekord
/// --pki-format ssh` sends, in JCS key order, and its public key is the
/// comment-free key with a trailing LF, as Rekor stores an SSH key.
#[test]
fn rekord_entry_bytes() {
    let doc = common::vectors();
    let v = vector(&doc, "TV-AK-06");
    let message = v["message"].as_str().unwrap();
    let signature = v["signature"].as_str().unwrap();
    let line = doc["author_key"]["keys"]["D"]["line"].as_str().unwrap();
    let openssh = line.split_once(' ').unwrap().1;
    let key = SshPublicKey::parse(&format!("{openssh} alice@example.com\n")).unwrap();

    let entry = moat::rekord_entry(message.as_bytes(), signature, &key);
    // Keys written in sorted order serialize sorted whether or not
    // serde_json preserves insertion order.
    let expected = json!({
        "apiVersion": "0.0.1",
        "kind": "rekord",
        "spec": {
            "data": {"content": common::base64(message.as_bytes())},
            "signature": {
                "content": common::base64(signature.as_bytes()),
                "format": "ssh",
                "publicKey": {"content": common::base64(format!("{openssh}\n").as_bytes())},
            },
        },
    });
    assert_eq!(
        String::from_utf8(entry).unwrap(),
        serde_json::to_string(&expected).unwrap()
    );
}

#[test]
fn signed_item_carries_its_rekord_entry() {
    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    let signer = EmulatedSkSigner::ecdsa_p256(seed("E"));
    let signed = moat::sign_item(&signer, &hash).unwrap();
    assert_eq!(
        signed.rekord_entry,
        moat::rekord_entry(
            &moat::item_message(&hash),
            &signed.signature,
            signer.public_key()
        )
    );
}

#[test]
fn ssh_keygen_signer_refuses_a_plain_key_unless_allowed() {
    let doc = common::vectors();
    let plain = public_key(&doc, "A");
    let err = SshKeygenSigner::new("key", plain.clone(), NonSkKeys::Refuse).unwrap_err();
    assert_eq!(err, SignError::KeyNotSecurityKey(KeyType::Ed25519));
    assert_eq!(err.id(), "syllago.moat.signing_key_not_sk");
    assert!(err.to_string().contains("security-key (sk-) key"));
    assert!(SshKeygenSigner::new("key", plain, NonSkKeys::Allow).is_ok());
    for label in ["D", "E"] {
        let sk = public_key(&doc, label);
        assert!(SshKeygenSigner::new("key", sk, NonSkKeys::Refuse).is_ok());
    }
}

#[test]
fn public_key_parse_refusals() {
    let doc = common::vectors();
    let line = doc["author_key"]["keys"]["A"]["line"].as_str().unwrap();
    let blob = line.rsplit_once(' ').unwrap().1;
    let rsa = "AAAAB3NzaC1yc2EAAAADAQABAAAAgQC7";
    for text in [
        "",
        "ssh-ed25519",
        &format!("sk-ssh-ed25519@openssh.com {blob}"),
        &format!("ssh-ed25519 {blob}x"),
        &format!("ssh-rsa {rsa}"),
        "ssh-ed25519 AAAA",
    ] {
        let err = SshPublicKey::parse(text).unwrap_err();
        assert_eq!(err.id(), "syllago.moat.signing_key_invalid", "{text:?}");
    }
    let key = SshPublicKey::parse(&format!("ssh-ed25519 {blob}")).unwrap();
    assert_eq!(key.key_type(), KeyType::Ed25519);
    assert_eq!(
        key.fingerprint().as_str(),
        doc["author_key"]["keys"]["A"]["fingerprint"]
    );
}

#[test]
fn default_sk_key_type_per_os() {
    assert_eq!(moat::default_sk_key_type(Os::Linux), SkKeyType::Ed25519Sk);
    assert_eq!(moat::default_sk_key_type(Os::MacOs), SkKeyType::EcdsaSk);
    assert_eq!(moat::default_sk_key_type(Os::Windows), SkKeyType::EcdsaSk);
    assert_eq!(SkKeyType::Ed25519Sk.key_type(), KeyType::SkEd25519);
    assert_eq!(SkKeyType::EcdsaSk.key_type(), KeyType::SkEcdsaP256);
}

#[test]
fn keygen_args_for_each_type() {
    let path = Path::new("keys/id_moat");
    assert_eq!(
        moat::keygen_args(SkKeyType::Ed25519Sk, path),
        ["-t", "ed25519-sk", "-C", "", "-f", "keys/id_moat"]
    );
    assert_eq!(
        moat::keygen_args(SkKeyType::EcdsaSk, path),
        ["-t", "ecdsa-sk", "-C", "", "-f", "keys/id_moat"]
    );
}
