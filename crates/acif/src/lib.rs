//! Canonicalize, hash and render items in the Agent Content Interchange
//! Format (ACIF).
//!
//! This crate implements the ACIF core, skill and rule specifications far
//! enough to pass their conformance vectors, plus the attestation input
//! digest of the registry specification. It reads bytes and JSON values; it
//! knows nothing about any provider's filesystem layout, git, signing or the
//! network.
//!
//! Items, sections and canonical records are JSON objects
//! ([`serde_json::Map`]), because that is the shape the specifications define
//! them in.

mod activation;
mod body;
mod envelope;
mod frontmatter;
mod hash;
mod item;
mod jcs;
mod pack;
mod registry;

use std::fmt;

pub use serde_json::{Map, Value};

pub use activation::{
    CrossReference, HookRefResolution, Rendered, Resolution, RuleTarget, derived_capabilities,
    render_rule, resolve_skill_hook_ref, rule_activation_projection,
};
pub use body::{Body, Classification};
pub use envelope::{Install, RequiresEvaluation, evaluate_requires};
pub use hash::{Hash, attestation_input_digest};
pub use item::{Item, ingest_body, ingest_rule_activation_source, ingest_sidecar};
pub use jcs::{canonical_json, metadata_hash};
pub use pack::{PACK_NAMESPACE, PackResolution, derive_pack_id, resolve_pack};
pub use registry::validate_registry_record;

/// An ACIF content type (core §5.1).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Kind {
    Hook,
    Skill,
    Rule,
    Command,
    Agent,
    McpConfig,
    Pack,
}

impl Kind {
    /// Parses a `kind` value by exact byte comparison.
    pub fn parse(value: &str) -> Option<Kind> {
        Some(match value {
            "hook" => Kind::Hook,
            "skill" => Kind::Skill,
            "rule" => Kind::Rule,
            "command" => Kind::Command,
            "agent" => Kind::Agent,
            "mcp_config" => Kind::McpConfig,
            "pack" => Kind::Pack,
            _ => return None,
        })
    }

    /// The `kind` value, which is also the name of the kind's extension block.
    pub fn as_str(self) -> &'static str {
        match self {
            Kind::Hook => "hook",
            Kind::Skill => "skill",
            Kind::Rule => "rule",
            Kind::Command => "command",
            Kind::Agent => "agent",
            Kind::McpConfig => "mcp_config",
            Kind::Pack => "pack",
        }
    }
}

/// A spec-minted identifier with its parameters, used both for diagnostics
/// and for the reason of a non-conformant verdict.
#[derive(Debug, Clone, PartialEq)]
pub struct Diagnostic {
    /// The identifier, such as `acif.requires.orphan_key`.
    pub id: &'static str,
    /// The identifier's parameters, such as `{"key": "handler_types"}`.
    pub params: Map<String, Value>,
}

impl Diagnostic {
    pub(crate) fn new(id: &'static str, params: &[(&str, Value)]) -> Diagnostic {
        let params = params
            .iter()
            .map(|(name, value)| (name.to_string(), value.clone()))
            .collect();
        Diagnostic { id, params }
    }
}

/// Why an operation did not produce a result.
#[derive(Debug)]
pub enum Error {
    /// The input is rejected with a spec-minted identifier, such as
    /// `acif.body.symlink`.
    Reject(&'static str),
    /// The input is non-conformant. The diagnostic names the verdict reason.
    Nonconformant(Diagnostic),
    /// The input is malformed in a way no spec identifier covers.
    Malformed(String),
    /// The input asks for something this crate does not implement.
    Unsupported(String),
    /// Reading a body from the filesystem failed.
    Io(std::io::Error),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Reject(id) => write!(f, "rejected: {id}"),
            Error::Nonconformant(diagnostic) => write!(f, "non-conformant: {}", diagnostic.id),
            Error::Malformed(message) => write!(f, "malformed: {message}"),
            Error::Unsupported(message) => write!(f, "unsupported: {message}"),
            Error::Io(error) => write!(f, "i/o error: {error}"),
        }
    }
}

impl std::error::Error for Error {}

impl From<std::io::Error> for Error {
    fn from(error: std::io::Error) -> Error {
        Error::Io(error)
    }
}

pub(crate) fn malformed(message: impl Into<String>) -> Error {
    Error::Malformed(message.into())
}

pub(crate) fn verdict(id: &'static str, params: &[(&str, Value)]) -> Error {
    Error::Nonconformant(Diagnostic::new(id, params))
}
