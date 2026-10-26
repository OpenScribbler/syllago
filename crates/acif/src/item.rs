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
    check_forbidden_fields(sidecar)?;
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
    check_forbidden_fields(section)?;
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
