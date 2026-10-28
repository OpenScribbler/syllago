use serde_json::{Map, Value};

use crate::activation::{
    kind_block, rule_activation, rule_activation_from_source, skill_activation,
};
use crate::body::{hash_body, split_frontmatter};
use crate::envelope::{check_envelope, check_forbidden_fields, check_requires};
use crate::{
    Body, Classification, Error, Hash, Install, Kind, frontmatter, malformed, metadata_hash,
    resolve_pack,
};

/// An ingested item.
#[derive(Debug, Clone, PartialEq)]
pub struct Item {
    pub kind: Kind,
    /// The `body_hash`, when the item was ingested from a body.
    pub body_hash: Option<Hash>,
    /// The body classification, when the item was ingested from a body.
    pub classification: Option<Classification>,
    /// What the publisher declared, observed faithfully (publisher §5).
    /// `None` when the publisher declared nothing.
    pub publisher_section: Option<Map<String, Value>>,
    /// The canonical record: the declared fields plus `kind`, the
    /// materialized activation defaults, and for a body its `classification`
    /// (and a rule's prose as `rule.body`). It enters no hash.
    pub canonical: Map<String, Value>,
    /// For an item record, its install disposition when no pack record is
    /// known: a pack-less item installs, and a declared `pack_id` is
    /// `unresolved` (publisher §8.3).
    pub install: Option<Install>,
}

impl Item {
    /// The `metadata_hash`, present exactly when a `publisher_section` is.
    pub fn metadata_hash(&self) -> Option<Hash> {
        self.publisher_section.as_ref().map(metadata_hash)
    }
}

/// Ingests an item from its body. `entry_file` is the body-relative POSIX
/// path of the entry file, whose frontmatter is the item's
/// `publisher_section`. Keys outside the envelope go into the block of the
/// kind the frontmatter declares, else of `kind`.
pub fn ingest_body(kind: Kind, body: &Body, entry_file: &str) -> Result<Item, Error> {
    let hashed = hash_body(body, entry_file)?;
    let (block, prose) = split_frontmatter(&hashed.entry_text);
    let publisher_section = match block {
        Some(block) => frontmatter::publisher_section(kind, block)?,
        None => None,
    };
    let mut item = canonicalize(Some(kind), publisher_section)?;
    item.canonical.insert(
        "classification".into(),
        Value::from(hashed.classification.as_str()),
    );
    if item.kind == Kind::Rule {
        let prose =
            String::from_utf8(prose.to_vec()).map_err(|_| malformed("rule body is not UTF-8"))?;
        if let Some(Value::Object(rule)) = item.canonical.get_mut("rule") {
            rule.insert("body".into(), Value::String(prose));
        }
    }
    item.body_hash = Some(hashed.hash);
    item.classification = Some(hashed.classification);
    Ok(item)
}

/// Ingests an item from a sidecar. A sidecar with a `publisher_section` or
/// `registry_section` key is an item record; any other sidecar is the
/// `publisher_section` itself. A `kind` the section declares wins over
/// the `kind` argument.
pub fn ingest_sidecar(kind: Option<Kind>, sidecar: &Map<String, Value>) -> Result<Item, Error> {
    if !(sidecar.contains_key("publisher_section") || sidecar.contains_key("registry_section")) {
        return canonicalize(kind, Some(sidecar.clone()));
    }
    let publisher_section = record_section(sidecar, "publisher_section")?;
    if let Some(registry_section) = record_section(sidecar, "registry_section")? {
        check_forbidden_fields(registry_section)?;
    }
    let mut item = canonicalize(kind, publisher_section.cloned())?;
    item.install = Some(resolve_pack(sidecar, &[]).install());
    Ok(item)
}

fn record_section<'a>(
    record: &'a Map<String, Value>,
    name: &str,
) -> Result<Option<&'a Map<String, Value>>, Error> {
    match record.get(name) {
        None => Ok(None),
        Some(Value::Object(section)) => Ok(Some(section)),
        Some(_) => Err(malformed(format!("{name} is not an object"))),
    }
}

/// Ingests a rule whose activation is declared through a pre-abstracted
/// provider configuration (rule §10). The item has no body and declares no
/// `publisher_section`.
pub fn ingest_rule_activation_source(content: Option<&Value>) -> Result<Item, Error> {
    let activation = rule_activation_from_source(content)?;
    let mut rule = Map::new();
    rule.insert("activation".into(), Value::Object(activation));
    let mut canonical = Map::new();
    canonical.insert("kind".into(), Value::from(Kind::Rule.as_str()));
    canonical.insert("rule".into(), Value::Object(rule));
    Ok(Item {
        kind: Kind::Rule,
        body_hash: None,
        classification: None,
        publisher_section: None,
        canonical,
        install: None,
    })
}

/// Validates a `publisher_section` and builds the canonical record from it.
/// Only skill and rule extension blocks are implemented: an item of another
/// kind has its envelope validated, and a kind block it declares is
/// unsupported.
fn canonicalize(
    kind: Option<Kind>,
    publisher_section: Option<Map<String, Value>>,
) -> Result<Item, Error> {
    let empty = Map::new();
    let section = publisher_section.as_ref().unwrap_or(&empty);
    // The forbidden fields are checked at the section's top level and as
    // direct members of its kind block (core §5.2), before `kind` itself
    // (publisher §5.6 steps 4 and 5). The kind block is the declared kind's
    // when that kind is valid, else the caller's, as frontmatter places it.
    check_forbidden_fields(section)?;
    let block_kind = section
        .get("kind")
        .and_then(Value::as_str)
        .and_then(Kind::parse)
        .or(kind);
    if let Some(Value::Object(block)) = block_kind.and_then(|kind| section.get(kind.as_str())) {
        check_forbidden_fields(block)?;
    }
    let kind = check_envelope(section)?
        .or(kind)
        .ok_or_else(|| Error::Unsupported("the item's kind is unknown".into()))?;
    let mut canonical = section.clone();
    canonical.insert("kind".into(), Value::from(kind.as_str()));
    let block = kind_block(section, kind)?;
    match kind {
        Kind::Skill | Kind::Rule => {
            if let Some(block) = block {
                check_requires(block)?;
            }
            let declared = block.and_then(|block| block.get("activation"));
            let activation = match kind {
                Kind::Skill => skill_activation(declared)?,
                _ => rule_activation(declared)?,
            };
            let mut block = block.cloned().unwrap_or_default();
            block.insert("activation".into(), Value::Object(activation));
            canonical.insert(kind.as_str().into(), Value::Object(block));
        }
        _ if block.is_some() => {
            return Err(Error::Unsupported(format!(
                "the {} extension block",
                kind.as_str()
            )));
        }
        _ => {}
    }
    Ok(Item {
        kind,
        body_hash: None,
        classification: None,
        publisher_section,
        canonical,
        install: None,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn skill_from(frontmatter: &str) -> Result<Item, Error> {
        let text = format!("---\n{frontmatter}---\nUse this skill.\n");
        ingest_body(
            Kind::Skill,
            &Body::from_files(&[("SKILL.md", text.as_bytes())]),
            "SKILL.md",
        )
    }

    fn rejects_with(result: Result<Item, Error>, id: &str, field: Option<&str>) {
        match result {
            Err(Error::Nonconformant(diagnostic)) => {
                assert_eq!(diagnostic.id, id);
                if let Some(field) = field {
                    assert_eq!(diagnostic.params["field"], field);
                }
            }
            other => panic!("expected {id}, got {other:?}"),
        }
    }

    // A top-level frontmatter key outside the envelope is a direct member of
    // the kind block, so a forbidden name there rejects (publisher §5.6 step
    // 4), ahead of the step 5 `kind` check.
    #[test]
    fn forbidden_frontmatter_key_rejects_before_kind() {
        rejects_with(
            skill_from("display_name: Demo\nresolved_version: 1.0.0\n"),
            "acif.envelope.forbidden_field",
            Some("resolved_version"),
        );
        rejects_with(
            skill_from("kind: widget\nderived_version: 1.0.0\n"),
            "acif.envelope.forbidden_field",
            Some("derived_version"),
        );
    }

    // The forbidden names are checked in the sections, not among the
    // record's own keys (core §5.2).
    #[test]
    fn record_level_forbidden_name_is_not_checked() {
        let record = serde_json::json!({
            "publisher_section": {"kind": "skill", "display_name": "Demo"},
            "resolved_version": "1.0.0",
        });
        assert!(ingest_sidecar(Some(Kind::Skill), record.as_object().unwrap()).is_ok());
    }

    // A plain sidecar is the `publisher_section`, so a forbidden name as a
    // direct member of its kind block rejects (core §5.2).
    #[test]
    fn sidecar_kind_block_forbidden_name_rejects() {
        let sidecar = serde_json::json!({
            "kind": "skill",
            "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
            "display_name": "Demo",
            "skill": {"resolved_version": "1.0.0"},
        });
        rejects_with(
            ingest_sidecar(Some(Kind::Skill), sidecar.as_object().unwrap()),
            "acif.envelope.forbidden_field",
            Some("resolved_version"),
        );
    }

    // A null-valued forbidden key is absent, so it does not reject (publisher
    // §5.6 step 3).
    #[test]
    fn null_forbidden_frontmatter_key_is_absent() {
        let item = skill_from("display_name: Demo\nresolved_version:\n").unwrap();
        assert_eq!(
            item.publisher_section,
            Some(
                serde_json::json!({"display_name": "Demo"})
                    .as_object()
                    .unwrap()
                    .clone()
            )
        );
    }

    // Only the section's top level and the kind block's direct members are
    // checked (core §5.2); a forbidden name nested deeper is extension data.
    #[test]
    fn nested_forbidden_name_is_extension_data() {
        let item = skill_from(
            "display_name: Demo\nskill:\n  resolved_version: 1.0.0\nmetadata:\n  resolved_version: 1.0.0\n",
        )
        .unwrap();
        assert_eq!(
            item.publisher_section.unwrap()["skill"],
            serde_json::json!({
                "skill": {"resolved_version": "1.0.0"},
                "metadata": {"resolved_version": "1.0.0"},
            })
        );
    }

    // `id` is lowercase hex only (core §5.1), from frontmatter as from a
    // sidecar.
    #[test]
    fn uppercase_hex_id_is_invalid() {
        for id in [
            "F47AC10B-58CC-4372-A567-0E02B2C3D479",
            "f47ac10b-58cc-4372-A567-0e02b2c3d479",
        ] {
            rejects_with(
                skill_from(&format!("id: {id}\ndisplay_name: Demo\n")),
                "acif.envelope.id_invalid",
                None,
            );
        }
        assert!(skill_from("id: f47ac10b-58cc-4372-a567-0e02b2c3d479\n").is_ok());
    }

    // On an item record the forbidden names are checked at the top level of
    // `registry_section` too, and nowhere deeper.
    #[test]
    fn record_checks_forbidden_names_per_section() {
        let record = |value: serde_json::Value| value.as_object().unwrap().clone();
        rejects_with(
            ingest_sidecar(
                Some(Kind::Skill),
                &record(serde_json::json!({
                    "publisher_section": {"display_name": "Demo"},
                    "registry_section": {"resolved_version": "1.0.0"},
                })),
            ),
            "acif.envelope.forbidden_field",
            Some("resolved_version"),
        );
        assert!(
            ingest_sidecar(
                Some(Kind::Skill),
                &record(serde_json::json!({
                    "publisher_section": {"display_name": "Demo"},
                    "registry_section": {"metadata": {"resolved_version": "1.0.0"}},
                })),
            )
            .is_ok()
        );
    }

    // Agent Skills declares `license` as a string. It is not the envelope's
    // license object, so it stays in the skill block as written.
    #[test]
    fn string_license_is_kept_as_declared() {
        let body = Body::from_files(&[("SKILL.md", b"---\nname: x\nlicense: MIT\n---\nbody\n")]);
        let item = ingest_body(Kind::Skill, &body, "SKILL.md").unwrap();
        let section = item.publisher_section.unwrap();
        assert_eq!(
            section["skill"],
            serde_json::json!({"name": "x", "license": "MIT"})
        );
        assert!(!section.contains_key("license"));
    }

    // A declared kind wins over the caller's, so the extension keys belong
    // to the declared kind's block.
    #[test]
    fn declared_kind_owns_extension_keys() {
        let body = Body::from_files(&[(
            "SKILL.md",
            b"---\nkind: rule\nactivation: {mode: manual}\n---\nRule prose\n",
        )]);
        let item = ingest_body(Kind::Skill, &body, "SKILL.md").unwrap();
        assert_eq!(item.kind, Kind::Rule);
        assert_eq!(item.canonical["rule"]["activation"]["mode"], "manual");
        assert!(!item.canonical.contains_key("skill"));
    }
}
