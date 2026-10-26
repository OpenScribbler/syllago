use serde_json::{Map, Value};
use uuid::Uuid;

use crate::Install;

/// The namespace of inferred pack ids for inference algorithm v0.1
/// (publisher §9.4).
pub const PACK_NAMESPACE: &str = "93516344-00e5-419b-a230-6e8b1d02f87d";

/// Derives an inferred pack id (publisher §9.4): UUIDv5 under
/// [`PACK_NAMESPACE`] over the canonical repository URL and the canonical
/// display name, joined by LF.
pub fn derive_pack_id(repository_url: &str, display_name: &str) -> String {
    let namespace = Uuid::parse_str(PACK_NAMESPACE).expect("PACK_NAMESPACE is a UUID");
    Uuid::new_v5(
        &namespace,
        format!("{repository_url}\n{display_name}").as_bytes(),
    )
    .to_string()
}

/// How an item's pack membership resolves (publisher §8.3).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PackResolution {
    /// The item declares a `pack_id` that names a known pack.
    Declared(String),
    /// The item declares no `pack_id` and the registry inferred one.
    Inferred(String),
    /// The item declares a `pack_id` that names no known pack.
    Unresolved(String),
    /// The item belongs to no pack.
    None,
}

impl PackResolution {
    /// The pack the item is a member of, when membership resolves.
    pub fn member_of(&self) -> Option<&str> {
        match self {
            PackResolution::Declared(id) | PackResolution::Inferred(id) => Some(id),
            PackResolution::Unresolved(_) | PackResolution::None => None,
        }
    }

    /// The spec spelling of the resolution, or `None` for a pack-less item.
    pub fn as_str(&self) -> Option<&'static str> {
        match self {
            PackResolution::Declared(_) => Some("declared"),
            PackResolution::Inferred(_) => Some("inferred"),
            PackResolution::Unresolved(_) => Some("unresolved"),
            PackResolution::None => None,
        }
    }

    /// An unresolved declared pack is refused without operator opt-in.
    pub fn install(&self) -> Install {
        match self {
            PackResolution::Unresolved(_) => Install::RefuseUnlessOperatorOptIn,
            _ => Install::Proceed,
        }
    }
}

/// Resolves the pack membership of an item record: a declared
/// `publisher_section.pack_id` wins over `registry_section.inferred_pack_id`.
pub fn resolve_pack(record: &Map<String, Value>, known_packs: &[&str]) -> PackResolution {
    let field = |section: &str, name: &str| {
        record
            .get(section)
            .and_then(|section| section.get(name))
            .and_then(Value::as_str)
            .map(str::to_string)
    };
    if let Some(declared) = field("publisher_section", "pack_id") {
        return if known_packs.contains(&declared.as_str()) {
            PackResolution::Declared(declared)
        } else {
            PackResolution::Unresolved(declared)
        };
    }
    match field("registry_section", "inferred_pack_id") {
        Some(inferred) => PackResolution::Inferred(inferred),
        None => PackResolution::None,
    }
}
