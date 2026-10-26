//! Rekor log entries: the lookup interface the caller implements, and the
//! `rekord` entry an Author Key's item signature is recorded in
//! (moat-spec.md §Author-Key Profile, Transparency log entry).
//!
//! The crate makes no network calls. A caller that talks to Rekor implements
//! [`RekorLog`] and hands this crate the entries it fetched; nothing an entry
//! says is trusted until its signed entry timestamp, inclusion proof and
//! checkpoint verify against the trusted root's log keys.

use std::fmt;

use sha2::{Digest, Sha256};
use sigstore_verify::crypto::{Checkpoint, Keyring};
use sigstore_verify::types::{Sha256Hash, SignatureBytes};

use crate::allowed_signers::hex;
use crate::trust::Cause;
use crate::wire::{Reader, base64, base64_decode};
use crate::{Fingerprint, KeyType, sshsig};

const SSHSIG_MAGIC: &[u8] = b"SSHSIG";
const PEM_LINE: usize = 64;

/// Read access to a Rekor v1 log. The caller implements it over HTTP, a
/// cache, or a fixture; this crate never reaches the network itself.
pub trait RekorLog {
    /// Every entry whose data hash is `sha256`: Rekor's
    /// `/api/v1/index/retrieve` search, with each returned UUID fetched.
    fn search_by_data_hash(&self, sha256: &[u8; 32]) -> Result<Vec<LogEntry>, RekorError>;

    /// The entry at `index`: Rekor's `/api/v1/log/entries?logIndex=`.
    fn entry_by_index(&self, index: u64) -> Result<LogEntry, RekorError>;
}

/// Why a [`RekorLog`] lookup produced no entry.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RekorError {
    /// The log could not be reached or answered with an error.
    Unavailable(String),
    /// The log has no entry for the lookup.
    NotFound,
    /// The log's answer does not parse as a log entry.
    Malformed(String),
}

impl RekorError {
    /// The stable identifier for this error.
    pub fn id(&self) -> &'static str {
        match self {
            RekorError::Unavailable(_) => "syllago.moat.rekor_unavailable",
            RekorError::NotFound => "syllago.moat.rekor_entry_missing",
            RekorError::Malformed(_) => "syllago.moat.rekor_entry_malformed",
        }
    }
}

impl fmt::Display for RekorError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            RekorError::Unavailable(detail) | RekorError::Malformed(detail) => {
                write!(f, "{id}: {detail}")
            }
            RekorError::NotFound => write!(f, "{id}: the log has no entry for the lookup"),
        }
    }
}

impl std::error::Error for RekorError {}

impl From<RekorError> for Cause {
    fn from(error: RekorError) -> Cause {
        match error {
            RekorError::Unavailable(detail) => Cause::RekorUnavailable(detail),
            RekorError::NotFound => Cause::RekorEntryMissing("the log has no such entry".into()),
            RekorError::Malformed(detail) => Cause::RekorEntryMalformed(detail),
        }
    }
}

/// One Rekor v1 log entry as the log's API returns it. Its contents are
/// unverified until a verification in this crate checks them.
#[derive(Debug, Clone)]
pub struct LogEntry(pub(crate) sigstore_verify::rekor::LogEntry);

impl LogEntry {
    /// Parses one entry from a Rekor v1 API response: a JSON object with a
    /// single key, the entry's UUID, whose value is the entry.
    pub fn from_rekor_json(json: &[u8]) -> Result<LogEntry, RekorError> {
        let malformed = |detail: String| RekorError::Malformed(detail);
        let value: serde_json::Value =
            serde_json::from_slice(json).map_err(|e| malformed(e.to_string()))?;
        let object = value
            .as_object()
            .filter(|object| object.len() == 1)
            .ok_or_else(|| malformed("expected an object with exactly one entry".into()))?;
        let (uuid, entry) = object.iter().next().expect("one entry");
        let mut entry: sigstore_verify::rekor::LogEntry =
            serde_json::from_value(entry.clone()).map_err(|e| malformed(e.to_string()))?;
        entry.uuid = sigstore_verify::types::EntryUuid::new(uuid.clone());
        Ok(LogEntry(entry))
    }

    /// The entry's index in the log, as the log reports it.
    pub fn log_index(&self) -> u64 {
        self.0.log_index.get()
    }

    /// The entry's canonicalized body.
    pub fn body(&self) -> &[u8] {
        self.0.body.as_bytes()
    }
}

/// The canonicalized body Rekor v1 stores for a `rekord` entry recording
/// `armored_signature` over `message`: data hash SHA-256, signature format
/// `ssh`, the signature re-armored as Rekor re-armors it, and the public key
/// taken from the signature in OpenSSH form with no comment. `None` when the
/// signature is not an SSHSIG or its key is not an accepted type.
///
/// The body is what Rekor's `/api/v1/log/entries` returns base64-encoded,
/// and the Merkle leaf the log commits to.
pub fn rekord_body(message: &[u8], armored_signature: &str) -> Option<Vec<u8>> {
    let blob = sshsig::unarmor(armored_signature)?;
    let key_blob = sshsig_key_blob(&blob)?;
    let key_type = KeyType::from_wire(Reader::new(key_blob).string()?)?;
    let public_key = format!("{} {}\n", key_type.as_str(), base64(key_blob));
    let body = format!(
        r#"{{"apiVersion":"0.0.1","kind":"rekord","spec":{{"data":{{"hash":{{"algorithm":"sha256","value":"{}"}}}},"signature":{{"content":"{}","format":"ssh","publicKey":{{"content":"{}"}}}}}}}}"#,
        hex(&Sha256::digest(message)),
        base64(armor(&blob).as_bytes()),
        base64(public_key.as_bytes()),
    );
    Some(body.into_bytes())
}

/// What a `rekord` entry records, read from its body.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Rekord {
    /// The SHA-256 of the data the signature is over.
    pub(crate) data_hash: [u8; 32],
    /// The armored SSHSIG, as the entry stores it.
    pub(crate) signature: String,
    /// The fingerprint of the entry's public key.
    pub(crate) fingerprint: Fingerprint,
    /// Whether the public key carries a comment, which the spec requires
    /// verifiers to reject. The entry is still read, so a verifier can tell
    /// whether it was the one the signing profile names.
    pub(crate) has_comment: bool,
}

/// Reads a `rekord` entry's body. Only `rekord` 0.0.1 with format `ssh`, a
/// SHA-256 data hash and an accepted key type is read.
pub(crate) fn parse_rekord(body: &[u8]) -> Result<Rekord, Cause> {
    let malformed = |detail: &str| Cause::RekorEntryMalformed(detail.to_string());
    let body: serde_json::Value =
        serde_json::from_slice(body).map_err(|_| malformed("the body is not JSON"))?;
    if body["kind"] != "rekord" || body["apiVersion"] != "0.0.1" {
        return Err(malformed("the entry is not a rekord 0.0.1 entry"));
    }
    let spec = &body["spec"];
    let hash = &spec["data"]["hash"];
    if hash["algorithm"] != "sha256" {
        return Err(malformed("the data hash is not SHA-256"));
    }
    let data_hash = hash["value"]
        .as_str()
        .and_then(parse_lower_hex_32)
        .ok_or_else(|| malformed("the data hash is not 64 lowercase hex digits"))?;
    let signature = &spec["signature"];
    if signature["format"] != "ssh" {
        return Err(malformed("the signature format is not ssh"));
    }
    let armored = decode_text(&signature["content"])
        .ok_or_else(|| malformed("the signature content is not base64 text"))?;
    let public_key = decode_text(&signature["publicKey"]["content"])
        .ok_or_else(|| malformed("the public key content is not base64 text"))?;
    let (key_blob, has_comment) = parse_public_key(&public_key)?;
    Ok(Rekord {
        data_hash,
        signature: armored,
        fingerprint: Fingerprint::of_key_blob(&key_blob),
        has_comment,
    })
}

/// Reads an OpenSSH public key line, `type SP base64`, with one optional
/// trailing LF, as Rekor's canonical form writes it, and returns the key
/// blob and whether a comment follows it. Anything else is malformed.
fn parse_public_key(text: &str) -> Result<(Vec<u8>, bool), Cause> {
    let malformed = || Cause::RekorEntryMalformed("the public key is not an OpenSSH key".into());
    let line = text.strip_suffix('\n').unwrap_or(text);
    let mut fields = line.splitn(3, ' ');
    let (Some(type_name), Some(blob)) = (fields.next(), fields.next()) else {
        return Err(malformed());
    };
    let has_comment = fields.next().is_some();
    let blob = base64_decode(blob).ok_or_else(malformed)?;
    let mut reader = Reader::new(&blob);
    let key_type = reader
        .string()
        .and_then(KeyType::from_wire)
        .ok_or_else(malformed)?;
    if key_type.as_str() != type_name {
        return Err(malformed());
    }
    Ok((blob, has_comment))
}

/// Verifies the log's evidence for `entry` against the trusted log keys:
/// the signed entry timestamp over the entry, the RFC 6962 inclusion proof
/// of its body, and the checkpoint the proof's root is signed in. All three
/// are required. `now` bounds the integrated time and selects the log keys
/// in effect.
pub(crate) fn verify_log_evidence(
    entry: &LogEntry,
    keys: &Keyring,
    now: jiff::Timestamp,
) -> Result<(), Cause> {
    let entry = &entry.0;
    let integrated_time = entry
        .integrated_time
        .ok_or_else(|| Cause::SetInvalid("the entry has no integrated time".into()))?;
    if integrated_time > now {
        return Err(Cause::SetInvalid(format!(
            "the integrated time {integrated_time} is in the future"
        )));
    }
    let verification = entry.verification.as_ref();

    let set = verification
        .and_then(|v| v.signed_entry_timestamp.as_ref())
        .ok_or_else(|| Cause::SetInvalid("the entry has no signed entry timestamp".into()))?;
    let log_id = entry
        .log_id
        .decode()
        .ok()
        .and_then(|id| Sha256Hash::try_from(id.as_slice()).ok())
        .ok_or_else(|| Cause::SetInvalid("the log ID is not a SHA-256 hash".into()))?;
    let key = keys.key_at(&log_id, integrated_time).ok_or_else(|| {
        Cause::SetInvalid(format!(
            "no trusted log key {} at {integrated_time}",
            log_id.to_hex()
        ))
    })?;
    // RFC 8785 form of the SET payload. Its keys are already in sorted
    // order and no value holds a character JSON escapes, so formatting
    // produces it exactly.
    let payload = format!(
        r#"{{"body":"{}","integratedTime":{},"logID":"{}","logIndex":{}}}"#,
        entry.body.to_base64(),
        integrated_time.as_second(),
        log_id.to_hex(),
        entry.log_index.as_i64(),
    );
    key.verify(payload, &SignatureBytes::new(set.as_bytes().to_vec()))
        .map_err(|_| Cause::SetInvalid("the signed entry timestamp does not verify".into()))?;

    let proof = verification
        .and_then(|v| v.inclusion_proof.as_ref())
        .ok_or_else(|| Cause::InclusionProofInvalid("the entry has no inclusion proof".into()))?;
    let leaf = sigstore_merkle::hash_leaf(entry.body.as_bytes());
    sigstore_merkle::verify_inclusion_proof(
        &leaf,
        proof.log_index.get(),
        proof.tree_size,
        &proof.hashes,
        &proof.root_hash,
    )
    .map_err(|e| Cause::InclusionProofInvalid(e.to_string()))?;

    let checkpoint = Checkpoint::from_text(&proof.checkpoint)
        .map_err(|e| Cause::CheckpointInvalid(format!("the checkpoint does not parse: {e}")))?;
    require_checkpoint_matches(&checkpoint, &proof.root_hash, proof.tree_size)?;
    let signed = checkpoint.signed_data();
    let verified = checkpoint.signatures().iter().any(|sig| {
        keys.keys_by_hint(&sig.key_id, now)
            .any(|key| key.verify(signed, &sig.signature).is_ok())
    });
    if !verified {
        return Err(Cause::CheckpointInvalid(
            "no trusted log key signed the checkpoint".into(),
        ));
    }
    Ok(())
}

/// Requires a Rekor v1 checkpoint to name the inclusion proof's root hash
/// and tree size. The proof's own root and size are unsigned; an index's
/// audit path can compute the same root at more than one tree size, so the
/// size is checked as well as the root.
pub(crate) fn require_checkpoint_matches(
    checkpoint: &Checkpoint,
    root_hash: &Sha256Hash,
    tree_size: u64,
) -> Result<(), Cause> {
    if checkpoint.root_hash() != root_hash || checkpoint.tree_size() != tree_size {
        return Err(Cause::CheckpointInvalid(
            "the checkpoint does not match the inclusion proof's tree".into(),
        ));
    }
    Ok(())
}

/// The key blob inside an SSHSIG blob.
fn sshsig_key_blob(blob: &[u8]) -> Option<&[u8]> {
    let mut reader = Reader::new(blob);
    if reader.take(SSHSIG_MAGIC.len())? != SSHSIG_MAGIC || reader.u32()? != 1 {
        return None;
    }
    reader.string()
}

/// Armors an SSHSIG blob as Go's `pem.EncodeToMemory` does, which is how
/// Rekor stores an `ssh` signature: 64-column base64 lines and a final LF.
fn armor(blob: &[u8]) -> String {
    let encoded = base64(blob);
    let mut out = String::from("-----BEGIN SSH SIGNATURE-----\n");
    for line in encoded.as_bytes().chunks(PEM_LINE) {
        out.push_str(std::str::from_utf8(line).expect("base64 is ASCII"));
        out.push('\n');
    }
    out.push_str("-----END SSH SIGNATURE-----\n");
    out
}

fn decode_text(value: &serde_json::Value) -> Option<String> {
    String::from_utf8(base64_decode(value.as_str()?)?).ok()
}

fn parse_lower_hex_32(text: &str) -> Option<[u8; 32]> {
    let bytes = text.as_bytes();
    if bytes.len() != 64 {
        return None;
    }
    let digit = |c: u8| match c {
        b'0'..=b'9' => Some(c - b'0'),
        b'a'..=b'f' => Some(c - b'a' + 10),
        _ => None,
    };
    let mut out = [0u8; 32];
    for (i, pair) in bytes.chunks(2).enumerate() {
        out[i] = digit(pair[0])? << 4 | digit(pair[1])?;
    }
    Some(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn key_line(fields: &str) -> String {
        // A valid ssh-ed25519 blob: type string, then a 32-byte key.
        let mut blob = Vec::new();
        crate::wire::put_string(&mut blob, b"ssh-ed25519");
        crate::wire::put_string(&mut blob, &[7u8; 32]);
        fields.replace("BLOB", &base64(&blob))
    }

    #[test]
    fn public_key_forms() {
        let plain = |line: &str| parse_public_key(&key_line(line)).map(|(_, comment)| comment);
        assert_eq!(plain("ssh-ed25519 BLOB\n"), Ok(false));
        assert_eq!(plain("ssh-ed25519 BLOB"), Ok(false));
        for comment in [
            "ssh-ed25519 BLOB me@example",
            "ssh-ed25519 BLOB a b\n",
            "ssh-ed25519 BLOB \n",
        ] {
            assert_eq!(plain(comment), Ok(true), "{comment:?}");
        }
        for bad in [
            "ssh-rsa BLOB\n",
            "ssh-ed25519  BLOB\n",
            "ssh-ed25519\tBLOB\n",
            "ssh-ed25519 BLOB\n\n",
            "BLOB\n",
            "ssh-ed25519 !!!!\n",
        ] {
            let err = plain(bad).unwrap_err();
            assert_eq!(err.id(), "syllago.moat.rekor_entry_malformed", "{bad:?}");
        }
    }

    #[test]
    fn armor_wraps_at_64_columns() {
        let text = armor(&[0u8; 100]);
        let lines: Vec<&str> = text.lines().collect();
        assert_eq!(lines.first(), Some(&"-----BEGIN SSH SIGNATURE-----"));
        assert_eq!(lines[1].len(), 64);
        // 100 bytes are 136 base64 characters: 64, 64, then 8.
        assert_eq!(lines[2].len(), 64);
        assert_eq!(lines[3].len(), 8);
        assert!(text.ends_with("-----END SSH SIGNATURE-----\n"));
        assert_eq!(sshsig::unarmor(&text), Some(vec![0u8; 100]));
    }

    #[test]
    fn lower_hex_only() {
        assert_eq!(parse_lower_hex_32(&"ab".repeat(32)), Some([0xab; 32]));
        assert_eq!(parse_lower_hex_32(&"AB".repeat(32)), None);
        assert_eq!(parse_lower_hex_32(&"ab".repeat(31)), None);
    }

    #[test]
    fn rekor_errors_map_to_causes() {
        for (error, id) in [
            (
                RekorError::Unavailable("down".into()),
                "syllago.moat.rekor_unavailable",
            ),
            (RekorError::NotFound, "syllago.moat.rekor_entry_missing"),
            (
                RekorError::Malformed("x".into()),
                "syllago.moat.rekor_entry_malformed",
            ),
        ] {
            assert_eq!(error.id(), id);
            assert_eq!(Cause::from(error).id(), id);
        }
    }

    #[test]
    fn from_rekor_json_wants_one_entry() {
        for bad in [
            &b"[]"[..],
            b"{}",
            b"{\"a\":{},\"b\":{}}",
            b"{\"a\":{}}",
            b"x",
        ] {
            assert!(matches!(
                LogEntry::from_rekor_json(bad),
                Err(RekorError::Malformed(_))
            ));
        }
    }
}
