//! Dual-Attested verification for an item whose `signing_profile` is in
//! key-fingerprint form (moat-spec.md §Author-Key Profile, Client
//! verification): the registry's per-item attestation and the Publisher's
//! `rekord` entry for the same Content Hash.
//!
//! The reference refuses key-fingerprint profiles outright and has no
//! `rekord` code, so this follows the spec's steps directly. The log's
//! evidence for the `rekord` entry (SET, inclusion proof and checkpoint) is
//! all required, and the checkpoint's tree size must match the proof's.

use std::time::SystemTime;

use sha2::{Digest, Sha256};

use crate::registry::{RegistrySigningProfile, verify_registry_attestation};
use crate::rekor::{self, RekorLog};
use crate::trust::{Cause, Identity, TrustFailure};
use crate::trust_root::TrustRoot;
use crate::{ContentHash, VerifiedSigner, item_message, verify_item};

/// The manifest fields a Dual-Attested item with a key-fingerprint
/// `signing_profile` carries.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ManifestEntry {
    /// `content[].rekor_log_index`: the registry's per-item entry.
    pub rekor_log_index: u64,
    /// `content[].signing_profile.key_fingerprint`.
    pub key_fingerprint: String,
}

/// Both attestations of a Dual-Attested item, verified.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DualAttested {
    /// The registry signer.
    pub registry: Identity,
    /// The registry's per-item entry.
    pub registry_log_index: u64,
    /// The Author Key whose item signature verified.
    pub author: VerifiedSigner,
    /// The Publisher's `rekord` entry.
    pub author_log_index: u64,
}

/// Verifies both attestations of an item whose `signing_profile` is in
/// key-fingerprint form.
///
/// `item` must be the Content Hash the client computed over the bytes it
/// installs, `registry_profile` the manifest's verified
/// `registry_signing_profile`, and `trusted_list` the trusted
/// allowed-signers list for the item's `source_uri`. The registry entry at
/// `entry.rekor_log_index` must verify as [`verify_registry_attestation`]
/// requires. Then, among the `rekord` entries the log holds for the item
/// message's SHA-256, those whose key's fingerprint is
/// `entry.key_fingerprint` are selected; zero is a failure. The first
/// selected entry whose public key has no comment, whose log evidence
/// verifies, and whose SSHSIG verifies over the item message by a key in
/// `trusted_list` is accepted. When none is, the first selected entry's
/// refusal is returned.
#[allow(clippy::too_many_arguments)]
pub fn verify_dual_attested(
    item: &ContentHash,
    entry: &ManifestEntry,
    registry_profile: &RegistrySigningProfile,
    trusted_list: &[u8],
    log: &dyn RekorLog,
    root: &TrustRoot,
    now: SystemTime,
) -> Result<DualAttested, TrustFailure> {
    root.check_fresh(now)?;
    let registry = verify_registry_attestation(
        item,
        entry.rekor_log_index,
        registry_profile,
        log,
        root,
        now,
    )?;

    let now = jiff::Timestamp::try_from(now)
        .map_err(|_| Cause::SetInvalid("the current time is out of range".into()))?;
    let message = item_message(item);
    let message_hash: [u8; 32] = Sha256::digest(&message).into();
    let candidates = log
        .search_by_data_hash(&message_hash)
        .map_err(Cause::from)?;
    let mut first_refusal = None;
    for candidate in &candidates {
        let Ok(rekord) = rekor::parse_rekord(candidate.body()) else {
            continue;
        };
        if rekord.fingerprint.as_str() != entry.key_fingerprint {
            continue;
        }
        let checked = (|| {
            if rekord.has_comment {
                return Err(Cause::RekorPubkeyComment);
            }
            if rekord.data_hash != message_hash {
                return Err(Cause::RekorPayloadMismatch);
            }
            rekor::verify_log_evidence(candidate, &root.rekor_keys, now)?;
            let author =
                verify_item(item, &rekord.signature, trusted_list).map_err(Cause::AuthorKey)?;
            if author.fingerprint != rekord.fingerprint {
                return Err(Cause::RekorEntryMalformed(
                    "the signature's key is not the entry's public key".into(),
                ));
            }
            Ok(author)
        })();
        match checked {
            Ok(author) => {
                return Ok(DualAttested {
                    registry,
                    registry_log_index: entry.rekor_log_index,
                    author,
                    author_log_index: candidate.log_index(),
                });
            }
            Err(cause) => {
                first_refusal.get_or_insert(cause);
            }
        }
    }
    Err(first_refusal
        .unwrap_or_else(|| {
            Cause::RekorEntryMissing(format!(
                "no rekord entry by {} records this item",
                entry.key_fingerprint
            ))
        })
        .into())
}
