//! Verify MOAT Author-Key Profile signatures (moat-spec.md §Author-Key
//! Profile) over ACIF content items.
//!
//! An Author Key signs two kinds of MOAT message: the item message, over an
//! item's Content Hash, and the list-change message, over a change to a
//! source's allowed-signers list. [`verify_item`] and
//! [`verify_list_change`] check them with the accept and reject behavior of
//! MOAT's reference implementation, `reference/moat_author_key.py`.
//! [`verify_message`] checks any other message an Author Key signs under the
//! same scheme, such as syllago's loadout message. As in
//! the reference, Ed25519 verification is strict, so a signature by a
//! small-order key, or with a small-order R, is refused, and an armored
//! signature longer than 16 KiB is refused before it is parsed.
//!
//! [`sign_item`] signs an item message with an Author Key through any
//! [`Signer`] (`SshKeygenSigner` goes through `ssh-keygen`) and builds the
//! `rekord` entry that records it.
//!
//! The Sigstore side ([`verify_manifest`], [`verify_registry_attestation`],
//! [`verify_dual_attested`]) reports a [`TrustFailure`] carrying a MOAT Trust
//! State. Rekor is reached only through the caller's [`RekorLog`].
//!
//! The crate reads bytes and returns facts. Apart from signing through
//! `ssh-keygen`, which takes the message as a temporary file, it does no
//! network or file I/O, keeps no state, and never
//! prompts; trust on first use and the lockfile belong to its caller.

mod allowed_signers;
mod crypto_provider;
mod dual;
mod registry;
mod rekor;
mod sign;
mod sshsig;
#[cfg(feature = "testing")]
pub mod testing;
mod trust;
mod trust_root;
mod wire;

use std::fmt;

use sha2::{Digest, Sha256};

pub use allowed_signers::{AllowedSigners, ListDigest, canonical_text};
pub use dual::{DualAttested, ManifestEntry, verify_dual_attested};
pub use registry::{RegistrySigningProfile, verify_manifest, verify_registry_attestation};
pub use rekor::{LogEntry, RekorError, RekorLog, rekord_body};
pub use sign::{
    NonSkKeys, Os, SignError, SignedItem, Signer, SkKeyType, SshKeygenSigner, SshPublicKey,
    default_sk_key_type, keygen_args, rekord_entry, sign_item, sign_message,
};
pub use trust::{Cause, Identity, TrustFailure, TrustState};
pub use trust_root::{BUNDLED_ROOT_MAX_AGE, TrustRoot, TrustRootOrigin};

const ITEM_PREFIX: &[u8] = b"moat-author-item-v1\n";
const LIST_CHANGE_PREFIX: &[u8] = b"moat-allowed-signers-change-v1\n";

/// An item's Content Hash (moat-spec.md §Content Hashing), `sha256:<hex>`.
///
/// Build one only from an item ingested through the `acif` crate from the
/// bytes the client fetched. The type cannot tell a Content Hash copied from
/// a record apart from one computed here, so that obligation is the
/// caller's.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ContentHash(acif::Hash);

impl ContentHash {
    /// The Content Hash of an ACIF content item: the ACIF attestation input
    /// digest over the item's `body_hash` and `metadata_hash`. The item must
    /// have been ingested from its body.
    pub fn of_acif(item: &acif::Item) -> Result<ContentHash, ContentHashError> {
        let body_hash = item
            .body_hash
            .as_ref()
            .ok_or(ContentHashError::NoBodyHash)?;
        Ok(ContentHash(acif::attestation_input_digest(
            body_hash,
            item.metadata_hash().as_ref(),
        )))
    }
}

impl fmt::Display for ContentHash {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// Why a Content Hash could not be computed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContentHashError {
    /// The item was not ingested from a body, so it has no `body_hash`.
    NoBodyHash,
}

impl ContentHashError {
    /// The stable identifier for this error.
    pub fn id(&self) -> &'static str {
        match self {
            ContentHashError::NoBodyHash => "syllago.moat.item_has_no_body",
        }
    }
}

impl fmt::Display for ContentHashError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ContentHashError::NoBodyHash => {
                write!(f, "{}: the item has no body_hash", self.id())
            }
        }
    }
}

impl std::error::Error for ContentHashError {}

/// An Author Key's fingerprint: `SHA256:` and the unpadded base64 SHA-256
/// of its SSH public-key blob, as `ssh-keygen -l` prints it.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Fingerprint(String);

impl Fingerprint {
    /// The fingerprint of an SSH public-key blob.
    pub fn of_key_blob(blob: &[u8]) -> Fingerprint {
        Fingerprint(format!(
            "SHA256:{}",
            wire::base64_unpadded(&Sha256::digest(blob))
        ))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for Fingerprint {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

/// A key type the profile accepts.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyType {
    /// `ssh-ed25519`
    Ed25519,
    /// `sk-ssh-ed25519@openssh.com`
    SkEd25519,
    /// `sk-ecdsa-sha2-nistp256@openssh.com`
    SkEcdsaP256,
}

impl KeyType {
    /// The key type's SSH name.
    pub fn as_str(self) -> &'static str {
        match self {
            KeyType::Ed25519 => "ssh-ed25519",
            KeyType::SkEd25519 => "sk-ssh-ed25519@openssh.com",
            KeyType::SkEcdsaP256 => "sk-ecdsa-sha2-nistp256@openssh.com",
        }
    }

    /// Whether this is a security-key (`sk-`) type. A security-key type
    /// does not show that the private key is held in hardware.
    pub fn is_security_key(self) -> bool {
        self != KeyType::Ed25519
    }

    fn from_wire(name: &[u8]) -> Option<KeyType> {
        [KeyType::Ed25519, KeyType::SkEd25519, KeyType::SkEcdsaP256]
            .into_iter()
            .find(|key_type| key_type.as_str().as_bytes() == name)
    }
}

/// The key whose signature verified.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct VerifiedSigner {
    pub fingerprint: Fingerprint,
    pub key_type: KeyType,
}

/// Why an item signature or a list change was refused. The first four are
/// MOAT's outcomes, and their identifiers are MOAT's strings.
///
/// `SourceUriInvalid` is syllago's own, with the id
/// `syllago.moat.source_uri_invalid`: where this crate refuses, the
/// reference raises an exception. [`ContentHashError`]'s
/// `syllago.moat.item_has_no_body` is syllago's too, because the reference
/// takes a Content Hash as given and never computes one.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Refusal {
    /// The signature does not verify over the expected message under the
    /// profile's scheme.
    SignatureInvalid,
    /// The signature verifies, but its key is not in the trusted list.
    SignerNotAllowed,
    /// An allowed-signers list is malformed and is refused as a whole.
    AllowedSignersMalformed(String),
    /// A new list's sequence is not greater than the trusted list's.
    SequenceNotGreater { recorded: u64, new: u64 },
    /// The `source_uri` is empty or contains a line break, so no
    /// list-change message can be built for it.
    SourceUriInvalid,
}

impl Refusal {
    /// The stable identifier for this refusal. A detail never changes it.
    pub fn id(&self) -> &'static str {
        match self {
            Refusal::SignatureInvalid => "signature_invalid",
            Refusal::SignerNotAllowed => "signer_not_allowed",
            Refusal::AllowedSignersMalformed(_) => "allowed_signers_malformed",
            Refusal::SequenceNotGreater { .. } => "sequence_not_greater",
            Refusal::SourceUriInvalid => "syllago.moat.source_uri_invalid",
        }
    }
}

impl fmt::Display for Refusal {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            Refusal::SignatureInvalid => write!(f, "{id}: the signature does not verify"),
            Refusal::SignerNotAllowed => {
                write!(f, "{id}: the signing key is not in the trusted list")
            }
            Refusal::AllowedSignersMalformed(detail) => write!(f, "{id}: {detail}"),
            Refusal::SequenceNotGreater { recorded, new } => write!(
                f,
                "{id}: the new list's sequence {new} is not greater than the trusted list's \
                 {recorded}; if the publisher's key was compromised, remove this source's \
                 trusted list to recover"
            ),
            Refusal::SourceUriInvalid => {
                write!(f, "{id}: the source_uri is empty or contains a line break")
            }
        }
    }
}

impl std::error::Error for Refusal {}

/// The canonical Attestation Payload for a Content Hash
/// (moat-spec.md §Attestation Payload).
///
/// The payload is formatted rather than JSON-encoded. That is valid JSON
/// only because a [`ContentHash`] always displays as `sha256:<hex>`, which
/// holds no character JSON would escape.
pub fn attestation_payload(content_hash: &ContentHash) -> Vec<u8> {
    format!(r#"{{"_version":1,"content_hash":"{content_hash}"}}"#).into_bytes()
}

/// The item message: `moat-author-item-v1` LF, then the Attestation
/// Payload.
pub fn item_message(content_hash: &ContentHash) -> Vec<u8> {
    [ITEM_PREFIX, &attestation_payload(content_hash)].concat()
}

/// The list-change message: `moat-allowed-signers-change-v1` LF, then the
/// `source_uri`, the new list's sequence, the previous list's digest and the
/// new list's digest, each followed by LF.
pub fn list_change_message(
    source_uri: &str,
    previous: &AllowedSigners,
    new: &AllowedSigners,
) -> Result<Vec<u8>, Refusal> {
    if source_uri.is_empty() || source_uri.contains(['\n', '\r']) {
        return Err(Refusal::SourceUriInvalid);
    }
    let text = format!(
        "{source_uri}\n{}\n{}\n{}\n",
        new.sequence(),
        previous.digest(),
        new.digest()
    );
    Ok([LIST_CHANGE_PREFIX, text.as_bytes()].concat())
}

/// Verifies an item's Author-Key signature. `content_hash` must be the one
/// the client computed over the bytes it installs. The item is accepted only
/// when `list` is well formed, `signature` verifies over the item message,
/// and the signing key is in `list`.
pub fn verify_item(
    content_hash: &ContentHash,
    signature: &str,
    list: &[u8],
) -> Result<VerifiedSigner, Refusal> {
    verify_signed(&item_message(content_hash), signature, list)
}

/// Verifies an Author-Key signature over a message of a kind outside MOAT's
/// profile. The message must open with its own separator line, distinct
/// from MOAT's `moat-author-item-v1` and `moat-allowed-signers-change-v1`,
/// so a signature over it never verifies as a MOAT item or list change; a
/// message that opens with either is refused as `signature_invalid`. It is
/// accepted only when `list` is well formed, `signature` verifies over
/// `message`, and the signing key is in `list`.
pub fn verify_message(
    message: &[u8],
    signature: &str,
    list: &[u8],
) -> Result<VerifiedSigner, Refusal> {
    if message.starts_with(ITEM_PREFIX) || message.starts_with(LIST_CHANGE_PREFIX) {
        return Err(Refusal::SignatureInvalid);
    }
    verify_signed(message, signature, list)
}

/// [`verify_message`] without the separator check, for MOAT's own kinds.
fn verify_signed(message: &[u8], signature: &str, list: &[u8]) -> Result<VerifiedSigner, Refusal> {
    let allowed = AllowedSigners::parse(list)?;
    let signer = sshsig::verify(signature, message).ok_or(Refusal::SignatureInvalid)?;
    if !allowed.contains(&signer.fingerprint) {
        return Err(Refusal::SignerNotAllowed);
    }
    Ok(signer)
}

/// Verifies a signed change from the `recorded` (trusted) list to `new` for
/// `source_uri`. The change is accepted only when both lists are well
/// formed, the new sequence is greater than the recorded one, and
/// `signature` verifies over the list-change message by a key the recorded
/// list allows.
pub fn verify_list_change(
    source_uri: &str,
    recorded: &[u8],
    new: &[u8],
    signature: &str,
) -> Result<VerifiedSigner, Refusal> {
    let recorded = AllowedSigners::parse(recorded)?;
    let new = AllowedSigners::parse(new)?;
    if new.sequence() <= recorded.sequence() {
        return Err(Refusal::SequenceNotGreater {
            recorded: recorded.sequence(),
            new: new.sequence(),
        });
    }
    let message = list_change_message(source_uri, &recorded, &new)?;
    let signer = sshsig::verify(signature, &message).ok_or(Refusal::SignatureInvalid)?;
    if !recorded.contains(&signer.fingerprint) {
        return Err(Refusal::SignerNotAllowed);
    }
    Ok(signer)
}

#[cfg(test)]
mod tests {
    use super::*;

    const VECTORS: &str = include_str!("../tests/vectors/moat-vectors.json");

    // The Attestation Payload vector names a Content Hash with no item
    // behind it, so it is built directly, which only this crate can do.
    #[test]
    fn attestation_payload_vector() {
        let doc: serde_json::Value = serde_json::from_str(VECTORS).unwrap();
        let vector = &doc["attestation_payload"];
        let (algorithm, value) = vector["content_hash"]
            .as_str()
            .unwrap()
            .split_once(':')
            .unwrap();
        let hash = ContentHash(acif::Hash {
            algorithm: algorithm.into(),
            value: value.into(),
        });
        let payload = attestation_payload(&hash);
        assert_eq!(payload, vector["payload"].as_str().unwrap().as_bytes());
        assert_eq!(
            allowed_signers::hex(&Sha256::digest(&payload)),
            vector["payload_sha256"].as_str().unwrap()
        );
    }

    #[test]
    fn refusal_ids_are_moat_strings() {
        assert_eq!(Refusal::SignatureInvalid.id(), "signature_invalid");
        assert_eq!(Refusal::SignerNotAllowed.id(), "signer_not_allowed");
        assert_eq!(
            Refusal::AllowedSignersMalformed("x".into()).id(),
            "allowed_signers_malformed"
        );
        let refusal = Refusal::SequenceNotGreater {
            recorded: 2,
            new: 1,
        };
        assert_eq!(refusal.id(), "sequence_not_greater");
        assert!(
            refusal
                .to_string()
                .contains("remove this source's trusted list")
        );
    }
}
