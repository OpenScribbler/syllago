use serde_json::{Map, Value};

use crate::{Error, Hash, Kind, attestation_input_digest};

/// Validates the record-emit conditions of an emitted registry record that
/// this crate implements (registry §6, §10.2): `source_uri` is present, and
/// `attestation_hash`, when present, equals the attestation input digest of
/// the record's own `body_hash` and `metadata_hash`. A pack record never
/// carries `attestation_hash`.
pub fn validate_registry_record(kind: Kind, record: &Map<String, Value>) -> Result<(), Error> {
    let section = record
        .get("registry_section")
        .and_then(Value::as_object)
        .filter(|section| section.contains_key("source_uri"))
        .ok_or(Error::Reject("acif.source_uri.missing"))?;
    let Some(attestation) = section.get("attestation_hash") else {
        return Ok(());
    };
    let expected = if kind == Kind::Pack {
        None
    } else {
        record_digest(section)
    };
    if expected.is_none() || hash_field(attestation) != expected {
        return Err(Error::Reject("acif.registry.attestation_hash_mismatch"));
    }
    Ok(())
}

/// The attestation input digest of the section's own hashes, or `None` when
/// `body_hash` is absent or either hash is not a `{algorithm, value}` object.
fn record_digest(section: &Map<String, Value>) -> Option<Hash> {
    let body_hash = hash_field(section.get("body_hash")?)?;
    let metadata_hash = match section.get("metadata_hash") {
        None => None,
        Some(value) => Some(hash_field(value)?),
    };
    Some(attestation_input_digest(&body_hash, metadata_hash.as_ref()))
}

fn hash_field(value: &Value) -> Option<Hash> {
    Some(Hash {
        algorithm: value.get("algorithm")?.as_str()?.to_string(),
        value: value.get("value")?.as_str()?.to_string(),
    })
}
