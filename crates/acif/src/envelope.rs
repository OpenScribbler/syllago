use serde_json::{Map, Value};
use uuid::{Uuid, Variant};

use crate::{Error, Kind, malformed, verdict};

/// Version-inheritance field names reserved on item records (core §5.2).
const FORBIDDEN_FIELDS: [&str; 4] = [
    "effective_version",
    "derived_version",
    "pack_inherited_version",
    "resolved_version",
];

/// The outcome of evaluating `requires` keys (core §9.5).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RequiresEvaluation {
    Satisfied,
    Unknown,
}

impl RequiresEvaluation {
    /// The spec spelling: `satisfied` or `unknown`.
    pub fn as_str(self) -> &'static str {
        match self {
            RequiresEvaluation::Satisfied => "satisfied",
            RequiresEvaluation::Unknown => "unknown",
        }
    }
}

/// What an install tool does with an item (core §9.5, §10).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Install {
    Proceed,
    RefuseUnlessOperatorOptIn,
}

impl Install {
    /// The spec spelling: `proceed` or `refuse-unless-operator-opt-in`.
    pub fn as_str(self) -> &'static str {
        match self {
            Install::Proceed => "proceed",
            Install::RefuseUnlessOperatorOptIn => "refuse-unless-operator-opt-in",
        }
    }
}

/// Evaluates an item's `requires` keys against the keys a consumer
/// recognizes (core §9.5). An empty `requires` is satisfied; any key the
/// consumer does not recognize makes the result `unknown`, which an install
/// tool refuses without operator opt-in.
///
/// ACIF 0.1 defines no `requires` vocabulary, so a set of keys the consumer
/// recognizes in full cannot be evaluated and is `Unsupported`.
pub fn evaluate_requires(
    item_requires: &Map<String, Value>,
    consumer_recognizes: &[&str],
) -> Result<(RequiresEvaluation, Install), Error> {
    if item_requires.is_empty() {
        return Ok((RequiresEvaluation::Satisfied, Install::Proceed));
    }
    if item_requires
        .keys()
        .any(|key| !consumer_recognizes.contains(&key.as_str()))
    {
        return Ok((
            RequiresEvaluation::Unknown,
            Install::RefuseUnlessOperatorOptIn,
        ));
    }
    Err(Error::Unsupported(
        "no requires vocabulary is defined in ACIF 0.1".into(),
    ))
}

/// Rejects a reserved version-inheritance field at the top level of `section`
/// (core §5.2). A section's kind block is checked by passing it here too; a
/// key of the same name nested deeper is extension data.
pub(crate) fn check_forbidden_fields(section: &Map<String, Value>) -> Result<(), Error> {
    match FORBIDDEN_FIELDS
        .iter()
        .find(|field| section.contains_key(**field))
    {
        Some(field) => Err(verdict(
            "acif.envelope.forbidden_field",
            &[("field", Value::from(*field))],
        )),
        None => Ok(()),
    }
}

/// Validates the envelope fields `section` declares (core §5.1) and returns
/// its declared kind. Absent fields are not checked: a `publisher_section`
/// carries only what the publisher declared.
pub(crate) fn check_envelope(section: &Map<String, Value>) -> Result<Option<Kind>, Error> {
    let kind = match section.get("kind") {
        None => None,
        Some(value) => Some(
            value
                .as_str()
                .and_then(Kind::parse)
                .ok_or_else(|| verdict("acif.envelope.kind_invalid", &[]))?,
        ),
    };
    if let Some(id) = section.get("id")
        && !id.as_str().is_some_and(is_uuid_v4)
    {
        return Err(verdict("acif.envelope.id_invalid", &[]));
    }
    if let Some(version) = section.get("version")
        && !version
            .as_str()
            .is_some_and(|v| semver::Version::parse(v).is_ok())
    {
        return Err(verdict("acif.envelope.version_invalid", &[]));
    }
    if let Some(license) = section.get("license") {
        let license = license
            .as_object()
            .ok_or_else(|| malformed("license is not an object"))?;
        if let Some(spdx) = license.get("spdx")
            && !spdx.as_str().is_some_and(is_spdx_identifier)
        {
            return Err(verdict("acif.envelope.license_spdx_invalid", &[]));
        }
    }
    Ok(kind)
}

/// Rejects any key in the kind block's `requires`: ACIF 0.1 recognizes none
/// for skills or rules (core §9.4).
pub(crate) fn check_requires(block: &Map<String, Value>) -> Result<(), Error> {
    let Some(requires) = block.get("requires") else {
        return Ok(());
    };
    let requires = requires
        .as_object()
        .ok_or_else(|| malformed("requires is not an object"))?;
    match requires.keys().next() {
        Some(key) => Err(verdict(
            "acif.requires.orphan_key",
            &[("key", Value::from(key.as_str()))],
        )),
        None => Ok(()),
    }
}

/// A UUID version 4 in its 36-character hyphenated form ([RFC9562]) with
/// lowercase hex digits. Lowercase-only is ACIF's own restriction (core
/// §5.1): `id` is compared and hashed byte for byte.
fn is_uuid_v4(value: &str) -> bool {
    value.len() == 36
        && !value.bytes().any(|byte| byte.is_ascii_uppercase())
        && Uuid::try_parse(value)
            .is_ok_and(|uuid| uuid.get_version_num() == 4 && uuid.get_variant() == Variant::RFC4122)
}

/// An SPDX license identifier, matched exactly. A trailing `+` or a
/// compound expression is not an identifier.
fn is_spdx_identifier(value: &str) -> bool {
    spdx::license_id(value).is_some_and(|license| license.name == value)
}
