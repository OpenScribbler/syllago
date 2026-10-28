//! Signed loadout manifests.
//!
//! A loadout manifest is a JSON file `<name>.json` in the sync repository,
//! and its armored SSHSIG sits beside it at `<name>.json.sig`. The manifest
//! lists Content Items by ACIF `id`, pins each to its `attestation_hash`,
//! and targets one or more providers. An Author Key signs the loadout
//! message over the repository's `source_uri`, the loadout's name, its
//! sequence and the manifest digest, under MOAT's author-key scheme. The
//! message opens with its own separator line, `syllago-loadout-v1`, so a
//! loadout signature never verifies as a MOAT item or list change, and
//! neither verifies as a loadout.
//!
//! `tests/vectors/README.md` states the format for other clients.

use std::collections::BTreeSet;
use std::fmt;

use moat::{Signer, VerifiedSigner};
use serde::{Deserialize, Deserializer};
use sha2::{Digest, Sha256};

const MESSAGE_PREFIX: &[u8] = b"syllago-loadout-v1\n";
/// The extension a manifest's file name carries. The loadout's name is the
/// file name without it.
pub const EXTENSION: &str = ".json";
/// The largest sequence a manifest may carry, 2^53 - 1.
const MAX_SEQUENCE: u64 = (1 << 53) - 1;
/// The largest manifest read, in bytes. A real manifest, a few providers and
/// a few dozen items, is a few KiB; without the cap, an unsigned manifest of
/// any size pushed with a stolen token is copied and parsed before it is
/// refused.
pub const MAX_MANIFEST_BYTES: usize = 64 * 1024;

/// A well-formed loadout manifest, parsed from its canonical text.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Manifest {
    name: String,
    sequence: u64,
    providers: Vec<String>,
    items: Vec<PinnedItem>,
    digest: String,
}

/// A Content Item a loadout names, by ACIF `id`, and the `attestation_hash`
/// it is pinned to.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PinnedItem {
    pub id: String,
    pub attestation_hash: String,
}

// The derived visitors refuse an unknown field, a field of the wrong type and
// a repeated key, so no two readers of one manifest see different fields.
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct RawManifest {
    #[serde(default, deserialize_with = "present")]
    name: Option<String>,
    sequence: String,
    providers: Vec<String>,
    items: Vec<RawItem>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct RawItem {
    id: String,
    attestation_hash: String,
}

/// A `name` that is present must be a string; `null` is not an absent name.
fn present<'de, D: Deserializer<'de>>(deserializer: D) -> Result<Option<String>, D::Error> {
    String::deserialize(deserializer).map(Some)
}

impl Manifest {
    /// Parses the manifest in `bytes`, read from a file named `file_name`
    /// (the base name, no directory). The name is `file_name` without
    /// [`EXTENSION`]. The manifest is parsed from its canonical text, the
    /// bytes its digest covers, never from `bytes` as given.
    ///
    /// Refused as malformed: a file name without the extension, a name that
    /// is not one or more of `a`-`z`, `0`-`9` and `-`, a `name` field that
    /// differs from the file name, a `sequence` that is not a string in
    /// canonical decimal from 0 to 2^53 - 1, no providers or a repeated one,
    /// an item `id` that is not a lowercase UUID version 4 or is repeated,
    /// an `attestation_hash` that is not `sha256:` and 64 lowercase hex
    /// digits, a manifest or item that is not a JSON object, any other
    /// field, and a manifest over [`MAX_MANIFEST_BYTES`].
    pub fn parse(file_name: &str, bytes: &[u8]) -> Result<Manifest, Refusal> {
        let malformed = |loadout: &str, detail: &str| Refusal {
            loadout: loadout.to_string(),
            reason: Reason::Malformed(detail.to_string()),
        };
        let name = file_name
            .strip_suffix(EXTENSION)
            .ok_or_else(|| malformed(file_name, "the file name does not end in .json"))?;
        if !is_name(name) {
            return Err(malformed(
                file_name,
                "the name is not one or more of a-z, 0-9 and -",
            ));
        }
        if bytes.len() > MAX_MANIFEST_BYTES {
            return Err(malformed(
                name,
                &format!("the manifest is larger than {MAX_MANIFEST_BYTES} bytes"),
            ));
        }
        let canonical = moat::canonical_text(bytes);
        let digest = format!("sha256:{}", hex(&Sha256::digest(&canonical)));
        let invalid =
            |e: serde_json::Error| malformed(name, &format!("the manifest is not valid: {e}"));
        // The derived visitors also accept a struct written as a JSON array,
        // so the shape is checked on a Value first. The Value keeps the last
        // of a repeated key, so the fields come from the derived parse.
        let value: serde_json::Value = serde_json::from_slice(&canonical).map_err(invalid)?;
        let items_are_objects = match value.get("items") {
            Some(serde_json::Value::Array(items)) => items.iter().all(|item| item.is_object()),
            _ => true,
        };
        if !value.is_object() || !items_are_objects {
            return Err(malformed(
                name,
                "the manifest and each of its items must be a JSON object",
            ));
        }
        let raw: RawManifest = serde_json::from_slice(&canonical).map_err(invalid)?;
        if raw.name.as_ref().is_some_and(|field| field != name) {
            return Err(malformed(name, "the name field differs from the file name"));
        }
        let sequence = parse_sequence(&raw.sequence).ok_or_else(|| {
            malformed(
                name,
                "the sequence is not canonical decimal from 0 to 2^53 - 1",
            )
        })?;
        if raw.providers.is_empty() {
            return Err(malformed(name, "the manifest targets no provider"));
        }
        let mut providers = BTreeSet::new();
        for provider in &raw.providers {
            if provider.is_empty() || !providers.insert(provider) {
                return Err(malformed(
                    name,
                    &format!("provider {provider:?} is empty or repeated"),
                ));
            }
        }
        let mut ids = BTreeSet::new();
        for item in &raw.items {
            if !is_uuid_v4(&item.id) || !ids.insert(&item.id) {
                return Err(malformed(
                    name,
                    &format!(
                        "item id {:?} is not a lowercase UUID v4 or is repeated",
                        item.id
                    ),
                ));
            }
            if !is_sha256_digest(&item.attestation_hash) {
                return Err(malformed(
                    name,
                    &format!(
                        "attestation_hash {:?} is not sha256: and 64 lowercase hex digits",
                        item.attestation_hash
                    ),
                ));
            }
        }
        Ok(Manifest {
            name: name.to_string(),
            sequence,
            providers: raw.providers,
            items: raw
                .items
                .into_iter()
                .map(|item| PinnedItem {
                    id: item.id,
                    attestation_hash: item.attestation_hash,
                })
                .collect(),
            digest,
        })
    }

    /// The loadout's name: the manifest's file name without the extension.
    pub fn name(&self) -> &str {
        &self.name
    }

    pub fn sequence(&self) -> u64 {
        self.sequence
    }

    /// The providers the loadout targets, in manifest order.
    pub fn providers(&self) -> &[String] {
        &self.providers
    }

    /// The items the loadout pins, in manifest order.
    pub fn items(&self) -> &[PinnedItem] {
        &self.items
    }

    /// The manifest digest: `sha256:` and the lowercase hex SHA-256 of the
    /// manifest's canonical text.
    pub fn digest(&self) -> &str {
        &self.digest
    }
}

/// Parses every manifest in a repository, each as `(file name, bytes)`, and
/// refuses the first one [`Manifest::parse`] refuses or whose name another
/// manifest already has.
pub fn parse_all(files: &[(&str, &[u8])]) -> Result<Vec<Manifest>, Refusal> {
    let mut names = BTreeSet::new();
    let mut manifests = Vec::with_capacity(files.len());
    for (file_name, bytes) in files {
        let manifest = Manifest::parse(file_name, bytes)?;
        if !names.insert(manifest.name.clone()) {
            return Err(Refusal {
                loadout: manifest.name,
                reason: Reason::Malformed("another manifest has the same name".into()),
            });
        }
        manifests.push(manifest);
    }
    Ok(manifests)
}

/// One or more of `a`-`z`, `0`-`9` and `-`.
fn is_name(name: &str) -> bool {
    !name.is_empty()
        && name
            .bytes()
            .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || b == b'-')
}

/// A decimal integer with no sign or leading zero, at most 2^53 - 1.
fn parse_sequence(digits: &str) -> Option<u64> {
    let well_formed = !digits.is_empty()
        && digits.len() <= 16
        && digits.bytes().all(|b| b.is_ascii_digit())
        && (digits == "0" || !digits.starts_with('0'));
    let sequence: u64 = digits.parse().ok().filter(|_| well_formed)?;
    (sequence <= MAX_SEQUENCE).then_some(sequence)
}

/// A UUID version 4 in its hyphenated form with lowercase hex digits, as
/// ACIF writes an item `id`.
fn is_uuid_v4(id: &str) -> bool {
    let bytes = id.as_bytes();
    bytes.len() == 36
        && bytes.iter().enumerate().all(|(i, &b)| match i {
            8 | 13 | 18 | 23 => b == b'-',
            _ => is_lower_hex(b),
        })
        && bytes[14] == b'4'
        && matches!(bytes[19], b'8' | b'9' | b'a' | b'b')
}

/// `sha256:` and 64 lowercase hex digits.
fn is_sha256_digest(value: &str) -> bool {
    value
        .strip_prefix("sha256:")
        .is_some_and(|hex| hex.len() == 64 && hex.bytes().all(is_lower_hex))
}

fn is_lower_hex(b: u8) -> bool {
    b.is_ascii_digit() || (b'a'..=b'f').contains(&b)
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// The loadout message: `syllago-loadout-v1` LF, then the sync repository's
/// `source_uri`, the loadout name, the sequence and the manifest digest,
/// each followed by LF.
pub fn loadout_message(source_uri: &str, manifest: &Manifest) -> Result<Vec<u8>, Refusal> {
    if source_uri.is_empty() || source_uri.contains(['\n', '\r']) {
        return Err(Refusal {
            loadout: manifest.name.clone(),
            reason: Reason::SourceUriInvalid,
        });
    }
    let text = format!(
        "{source_uri}\n{}\n{}\n{}\n",
        manifest.name, manifest.sequence, manifest.digest
    );
    Ok([MESSAGE_PREFIX, text.as_bytes()].concat())
}

/// Signs the loadout message for `manifest` in the sync repository at
/// `source_uri`, with [`moat::sign_message`]'s checks. The result is the
/// armored SSHSIG to commit at the manifest's path plus `.sig`.
pub fn sign_loadout(
    signer: &dyn Signer,
    source_uri: &str,
    manifest: &Manifest,
) -> Result<String, SignError> {
    let message = loadout_message(source_uri, manifest).map_err(|_| SignError::SourceUriInvalid)?;
    moat::sign_message(signer, &message).map_err(SignError::Moat)
}

/// Verifies a loadout's signature, `None` when its `.sig` file is missing,
/// against the trusted allowed-signers `list` for `source_uri`. It is
/// accepted only when the signature verifies over the loadout message and
/// its key is in `list`.
pub fn verify_loadout(
    source_uri: &str,
    manifest: &Manifest,
    signature: Option<&str>,
    list: &[u8],
) -> Result<VerifiedSigner, Refusal> {
    let refuse = |reason| Refusal {
        loadout: manifest.name.clone(),
        reason,
    };
    let message = loadout_message(source_uri, manifest)?;
    let signature = signature.ok_or_else(|| refuse(Reason::SignatureMissing))?;
    moat::verify_message(&message, signature, list).map_err(|e| refuse(Reason::Moat(e)))
}

/// What this machine recorded for one loadout of one `source_uri`: the
/// sequence and manifest digest it last accepted, and whether the loadout is
/// active here.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Record {
    pub sequence: u64,
    pub digest: String,
    pub active: bool,
}

/// What sync does with a loadout that passed verification.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Decision {
    /// The loadout is active here: apply it, and once the apply commits,
    /// record its sequence and digest.
    Apply,
    /// The machine has no record of the loadout: report it and apply
    /// nothing until the End User activates it.
    ReportNoRecord,
    /// The loadout is recorded as inactive: report it and apply nothing,
    /// even when its digest equals the record.
    ReportInactive,
}

impl Decision {
    /// The stable identifier for this decision.
    pub fn id(self) -> &'static str {
        match self {
            Decision::Apply => "apply",
            Decision::ReportNoRecord => "report_no_record",
            Decision::ReportInactive => "report_inactive",
        }
    }
}

/// Decides what sync does with `manifest` from the sync repository at
/// `source_uri`. The signature must verify under the trusted `list` first.
/// Then a manifest whose digest differs from `record` must carry a sequence
/// greater than the recorded one, whether or not the loadout is active; a
/// digest equal to the record is unchanged. Only a loadout recorded as
/// active is applied.
pub fn decide(
    source_uri: &str,
    manifest: &Manifest,
    signature: Option<&str>,
    list: &[u8],
    record: Option<&Record>,
) -> Result<Decision, Refusal> {
    verify_loadout(source_uri, manifest, signature, list)?;
    let Some(record) = record else {
        return Ok(Decision::ReportNoRecord);
    };
    if record.digest != manifest.digest && manifest.sequence <= record.sequence {
        return Err(Refusal {
            loadout: manifest.name.clone(),
            reason: Reason::SequenceNotGreater {
                recorded: record.sequence,
                new: manifest.sequence,
            },
        });
    }
    Ok(if record.active {
        Decision::Apply
    } else {
        Decision::ReportInactive
    })
}

/// Why sync refused a loadout. It names the loadout: its name, or for a file
/// name that yields no name, the file name.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Refusal {
    pub loadout: String,
    pub reason: Reason,
}

/// The reason a loadout was refused.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Reason {
    /// The manifest or its file name breaks the loadout grammar.
    Malformed(String),
    /// The `source_uri` is empty or contains a line break, so no loadout
    /// message can be built for it.
    SourceUriInvalid,
    /// The manifest has no `.sig` file beside it.
    SignatureMissing,
    /// The signature check refused: `signature_invalid`,
    /// `signer_not_allowed`, or `allowed_signers_malformed` for the trusted
    /// list.
    Moat(moat::Refusal),
    /// The manifest changed without a sequence greater than the recorded
    /// one.
    SequenceNotGreater { recorded: u64, new: u64 },
}

impl Refusal {
    /// The stable identifier for this refusal. A detail never changes it.
    /// The signature refusals carry MOAT's strings, and a missing `.sig` is
    /// `signature_invalid`.
    pub fn id(&self) -> &'static str {
        match &self.reason {
            Reason::Malformed(_) => "syllago.loadout.malformed",
            Reason::SourceUriInvalid => "syllago.loadout.source_uri_invalid",
            Reason::SignatureMissing => "signature_invalid",
            Reason::Moat(refusal) => refusal.id(),
            Reason::SequenceNotGreater { .. } => "sequence_not_greater",
        }
    }
}

impl fmt::Display for Refusal {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        write!(f, "loadout {:?}: ", self.loadout)?;
        match &self.reason {
            Reason::Malformed(detail) => write!(f, "{id}: {detail}"),
            Reason::SourceUriInvalid => {
                write!(f, "{id}: the source_uri is empty or contains a line break")
            }
            Reason::SignatureMissing => {
                write!(f, "{id}: the manifest has no .sig file beside it")
            }
            Reason::Moat(refusal) => write!(f, "{refusal}"),
            Reason::SequenceNotGreater { recorded, new } => write!(
                f,
                "{id}: the manifest changed, but its sequence {new} is not greater than the \
                 recorded {recorded}. If the author recreated this loadout on a machine that \
                 could not see the higher sequence, re-sign it at a sequence above {recorded}. \
                 If an author key was stolen, remove this repository's trusted list to recover"
            ),
        }
    }
}

impl std::error::Error for Refusal {}

/// Why a loadout could not be signed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SignError {
    /// The `source_uri` is empty or contains a line break.
    SourceUriInvalid,
    /// Signing, or the check of the signature it produced, failed.
    Moat(moat::SignError),
}

impl SignError {
    /// The stable identifier for this error. A detail never changes it.
    pub fn id(&self) -> &'static str {
        match self {
            SignError::SourceUriInvalid => "syllago.loadout.source_uri_invalid",
            SignError::Moat(error) => error.id(),
        }
    }
}

impl fmt::Display for SignError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SignError::SourceUriInvalid => write!(
                f,
                "{}: the source_uri is empty or contains a line break",
                self.id()
            ),
            SignError::Moat(error) => write!(f, "{error}"),
        }
    }
}

impl std::error::Error for SignError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn names() {
        for good in ["a", "work", "my-loadout-2", "-", "0"] {
            assert!(is_name(good), "{good:?}");
        }
        for bad in ["", "Work", "my_loadout", "a.b", "a b", "a\n", "é"] {
            assert!(!is_name(bad), "{bad:?}");
        }
    }

    #[test]
    fn sequences() {
        assert_eq!(parse_sequence("0"), Some(0));
        assert_eq!(parse_sequence("17"), Some(17));
        assert_eq!(parse_sequence("9007199254740991"), Some(MAX_SEQUENCE));
        for bad in [
            "",
            "00",
            "01",
            "+1",
            "-1",
            "-0",
            "1.0",
            "1e3",
            " 1",
            "1 ",
            "9007199254740992",
            "99999999999999999999",
            "\u{661}",
        ] {
            assert_eq!(parse_sequence(bad), None, "{bad:?}");
        }
    }

    #[test]
    fn uuids_and_digests() {
        assert!(is_uuid_v4("0f8fad5b-d9cb-469f-a165-70867728950e"));
        for bad in [
            "0F8FAD5B-D9CB-469F-A165-70867728950E",
            "0f8fad5b-d9cb-569f-a165-70867728950e",
            "0f8fad5b-d9cb-469f-c165-70867728950e",
            "0f8fad5bd9cb469fa16570867728950e",
            "0f8fad5b-d9cb-469f-a165-70867728950",
        ] {
            assert!(!is_uuid_v4(bad), "{bad:?}");
        }
        let hash = format!("sha256:{}", "ab".repeat(32));
        assert!(is_sha256_digest(&hash));
        assert!(!is_sha256_digest(&hash.to_uppercase()));
        assert!(!is_sha256_digest(&hash[..70]));
        assert!(!is_sha256_digest(&hash.replace("sha256", "sha512")));
    }
}
