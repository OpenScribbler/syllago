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
/// A null `metadata_hash` is absent (registry §6).
fn record_digest(section: &Map<String, Value>) -> Option<Hash> {
    let body_hash = hash_field(section.get("body_hash")?)?;
    let metadata_hash = match section.get("metadata_hash") {
        None | Some(Value::Null) => None,
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

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    // Registry §6: the tuple's `metadata_hash` is null or absent exactly when
    // the item has no `publisher_section`, so null hashes as `MH = null`.
    #[test]
    fn null_metadata_hash_is_absent() {
        let body_hash = Hash::sha256(b"body");
        let digest = attestation_input_digest(&body_hash, None);
        let record = json!({"registry_section": {
            "source_uri": "https://example.com/item",
            "body_hash": {"algorithm": "sha256", "value": body_hash.value},
            "metadata_hash": null,
            "attestation_hash": {"algorithm": "sha256", "value": digest.value},
        }});
        let record = record.as_object().unwrap();
        assert!(validate_registry_record(Kind::Skill, record).is_ok());
    }
}
