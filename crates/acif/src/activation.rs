use std::collections::BTreeMap;

use serde_json::{Map, Value, json};

use crate::{Diagnostic, Error, Kind, malformed};

/// The most globs a `rule_activation` projection samples (rule §12.1).
const GLOB_SAMPLE_CAP: usize = 16;

/// Returns the item's kind block (`skill`, `rule`, ...), if present.
pub(crate) fn kind_block(
    item: &Map<String, Value>,
    kind: Kind,
) -> Result<Option<&Map<String, Value>>, Error> {
    match item.get(kind.as_str()) {
        None => Ok(None),
        Some(Value::Object(block)) => Ok(Some(block)),
        Some(_) => Err(malformed(format!(
            "{} block is not an object",
            kind.as_str()
        ))),
    }
}

/// Validates a declared skill `activation` block and materializes its
/// defaults (skill §6.2, §7).
pub(crate) fn skill_activation(declared: Option<&Value>) -> Result<Map<String, Value>, Error> {
    let Some(declared) = declared else {
        return Ok(object(json!({"type": "auto", "user_invocable": true})));
    };
    let mut activation = declared
        .as_object()
        .cloned()
        .ok_or_else(|| malformed("skill.activation is not an object"))?;
    let activation_type = match activation.get("type") {
        None => return Err(Error::Reject("acif.skill.activation_type_missing")),
        Some(value) => match value.as_str() {
            Some(name @ ("auto" | "hook" | "manual")) => name,
            _ => return Err(Error::Reject("acif.skill.activation_type_invalid")),
        },
    };
    if let Some(hook_ref) = activation.get("hook_ref") {
        if activation_type != "hook" {
            return Err(Error::Reject("acif.skill.hook_ref_forbidden"));
        }
        let hook_ref = hook_ref
            .as_object()
            .ok_or_else(|| malformed("hook_ref is not an object"))?;
        match hook_ref.get("id") {
            None => return Err(Error::Reject("acif.skill.hook_ref_id_missing")),
            Some(Value::String(_)) => {}
            Some(_) => return Err(malformed("hook_ref.id is not a string")),
        }
    }
    match activation.get("user_invocable") {
        None => {
            activation.insert("user_invocable".into(), Value::Bool(true));
        }
        Some(Value::Bool(_)) => {}
        Some(_) => return Err(malformed("user_invocable is not a boolean")),
    }
    Ok(activation)
}

/// Validates a declared rule `activation` block and materializes its
/// default (rule §6.2, §7).
pub(crate) fn rule_activation(declared: Option<&Value>) -> Result<Map<String, Value>, Error> {
    let Some(declared) = declared else {
        return Ok(object(json!({"mode": "always"})));
    };
    let activation = declared
        .as_object()
        .cloned()
        .ok_or_else(|| malformed("rule.activation is not an object"))?;
    let mode = match activation.get("mode") {
        None => return Err(Error::Reject("acif.rule.activation_mode_missing")),
        Some(value) => match value.as_str() {
            Some(mode @ ("always" | "glob" | "manual" | "model_decision")) => mode,
            _ => return Err(Error::Reject("acif.rule.activation_mode_invalid")),
        },
    };
    match (mode, activation.get("globs")) {
        ("glob", None) => return Err(Error::Reject("acif.rule.glob_mode_without_globs")),
        ("glob", Some(Value::Array(globs))) => {
            if globs.is_empty() {
                return Err(Error::Reject("acif.rule.glob_mode_without_globs"));
            }
            if !globs
                .iter()
                .all(|glob| glob.as_str().is_some_and(|glob| !glob.is_empty()))
            {
                return Err(malformed("each glob must be a non-empty string"));
            }
        }
        ("glob", Some(_)) => return Err(malformed("globs is not an array")),
        (_, Some(_)) => return Err(Error::Reject("acif.rule.globs_without_glob_mode")),
        (_, None) => {}
    }
    Ok(activation)
}

/// Maps a pre-abstracted provider activation declaration to a canonical rule
/// `activation` block (rule §10, Appendix A.2). `content` carries the
/// source-mechanism token in `source_sub_mode` and the declaration's
/// normalized fields.
pub(crate) fn rule_activation_from_source(
    content: Option<&Value>,
) -> Result<Map<String, Value>, Error> {
    let missing = || Error::Reject("acif.rule.activation_mode_missing");
    let content = content.and_then(Value::as_object).ok_or_else(missing)?;
    let token = content
        .get("source_sub_mode")
        .and_then(Value::as_str)
        .ok_or_else(missing)?;
    let mode = match token {
        "always_on" => "always",
        "glob" | "frontmatter_globs" => "glob",
        "model_decision" => "model_decision",
        "manual" | "slash_command" => "manual",
        "legacy" => {
            let globs = content
                .get("globs")
                .filter(|globs| globs.as_array().is_some_and(|globs| !globs.is_empty()));
            return Ok(if content.get("always_apply") == Some(&Value::Bool(true)) {
                object(json!({"mode": "always"}))
            } else if let Some(globs) = globs {
                rule_activation(Some(&json!({"mode": "glob", "globs": globs})))?
            } else {
                object(json!({"mode": "model_decision"}))
            });
        }
        _ => return Err(Error::Reject("acif.rule.activation_mode_unmappable")),
    };
    let mut activation = object(json!({"mode": mode}));
    if mode == "glob"
        && let Some(globs) = content.get("globs")
    {
        activation.insert("globs".into(), globs.clone());
    }
    rule_activation(Some(&Value::Object(activation)))
}

/// Computes the DERIVABLE capability predicates of a skill or rule item over
/// its post-materialization form (skill §10.1, rule §9.1).
///
/// `skill_bundled_resources` is computed only when the item carries the
/// `classification` of an ingested body.
pub fn derived_capabilities(
    item: &Map<String, Value>,
) -> Result<BTreeMap<&'static str, bool>, Error> {
    let mut capabilities = BTreeMap::new();
    match item_kind(item)? {
        Kind::Skill => {
            let block = kind_block(item, Kind::Skill)?;
            let activation = skill_activation(block.and_then(|block| block.get("activation")))?;
            let activation_type = activation["type"].as_str();
            capabilities.insert("auto_invocable", activation_type == Some("auto"));
            capabilities.insert("disable_model_invocation", activation_type != Some("auto"));
            capabilities.insert(
                "user_invocable",
                activation["user_invocable"] == Value::Bool(false),
            );
            if let Some(classification) = item.get("classification") {
                capabilities.insert("skill_bundled_resources", classification == "multi-file");
            }
        }
        _ => {
            let activation = rule_item_activation(item)?;
            capabilities.insert("activation_mode", is_gated(&activation["mode"]));
        }
    }
    Ok(capabilities)
}

/// The `activation_mode` registry projection of a rule item (rule §12.1):
/// the §9.1 boolean, the canonical mode, and for `mode: glob` a bounded
/// sample of the globs.
pub fn rule_activation_projection(item: &Map<String, Value>) -> Result<Value, Error> {
    if item_kind(item)? != Kind::Rule {
        return Err(Error::Unsupported(
            "rule_activation projects rule items only".into(),
        ));
    }
    let activation = rule_item_activation(item)?;
    let mode = &activation["mode"];
    let mut projection = object(json!({"derivable": is_gated(mode), "mode": mode}));
    if let Some(Value::Array(globs)) = activation.get("globs") {
        let sample = &globs[..globs.len().min(GLOB_SAMPLE_CAP)];
        projection.insert(
            "globs".into(),
            json!({"present": true, "count": globs.len(), "sample": sample, "truncated": globs.len() > GLOB_SAMPLE_CAP}),
        );
    }
    Ok(Value::Object(projection))
}

/// A rule render target, classified by whether it can declare the canonical
/// activation mode (rule §11).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RuleTarget {
    /// The target's native rule format, which loads a bare rule file
    /// unconditionally. Only `mode: always` is rendered to it, as the bare
    /// body.
    NativeRuleFormat,
    /// A target with no activation declaration surface: every rule loads
    /// always-on there.
    NoDeclarationSurface,
}

/// A rendered rule and the diagnostics its rendering emitted.
#[derive(Debug, Clone, PartialEq)]
pub struct Rendered {
    pub output: String,
    pub diagnostics: Vec<Diagnostic>,
    /// The documented-lossy tokens of the rendering (core Appendix A.2).
    /// Rule rendering documents none.
    pub lossy: Vec<&'static str>,
}

/// Renders a canonical rule to `target` (rule §11). The body is emitted
/// byte-verbatim; a mode the target cannot declare degrades to always-on
/// with `acif.rule.activation_degraded`.
///
/// A canonical record without `rule.body` renders as an empty body.
pub fn render_rule(canonical: &Map<String, Value>, target: RuleTarget) -> Result<Rendered, Error> {
    let activation = rule_item_activation(canonical)?;
    let mode = activation["mode"].as_str().unwrap_or_default();
    let output = match kind_block(canonical, Kind::Rule)?.and_then(|block| block.get("body")) {
        None => String::new(),
        Some(Value::String(body)) => body.clone(),
        Some(_) => return Err(malformed("rule.body is not a string")),
    };
    let mut diagnostics = Vec::new();
    match target {
        RuleTarget::NativeRuleFormat if mode != "always" => {
            return Err(Error::Unsupported(format!(
                "rendering mode {mode} to native-rule-format"
            )));
        }
        RuleTarget::NoDeclarationSurface if mode != "always" => diagnostics.push(Diagnostic::new(
            "acif.rule.activation_degraded",
            &[
                ("mode_lost", Value::from(mode)),
                ("effective_behavior", Value::from("always-on")),
            ],
        )),
        _ => {}
    }
    Ok(Rendered {
        output,
        diagnostics,
        lossy: Vec::new(),
    })
}

/// How a cross-reference resolved (core §10).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Resolution {
    Resolved,
    Unresolved,
}

impl Resolution {
    /// The spec spelling: `resolved` or `unresolved`.
    pub fn as_str(self) -> &'static str {
        match self {
            Resolution::Resolved => "resolved",
            Resolution::Unresolved => "unresolved",
        }
    }
}

/// A resolved cross-reference entry (registry §9).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CrossReference {
    /// The JSON path of the reference site.
    pub source_path: &'static str,
    /// The expected kind of the target.
    pub target_kind: Kind,
    /// The declared reference as written: for a UUID-authored reference, the
    /// target's id.
    pub declared_name: String,
    pub resolution: Resolution,
}

/// What a registry emits on a skill's record for its `hook_ref`
/// (registry §9).
#[derive(Debug, Clone, PartialEq)]
pub struct HookRefResolution {
    pub cross_reference: CrossReference,
    /// The skill-side entries reciprocal to the hook-side `activation_target`
    /// entry (skill §13.2). A declared `hook_ref` is its own reciprocal entry.
    pub reciprocal_entries: Vec<CrossReference>,
    /// `acif.registry.reference_unresolved` when the reference does not
    /// resolve (registry §12).
    pub diagnostics: Vec<Diagnostic>,
}

/// Resolves a skill's `activation.hook_ref` against the ids of the hooks a
/// registry knows (skill §11, §13.2). The skill must declare `type: hook`
/// with a `hook_ref`.
pub fn resolve_skill_hook_ref(
    item: &Map<String, Value>,
    known_hooks: &[&str],
) -> Result<HookRefResolution, Error> {
    let block = kind_block(item, Kind::Skill)?;
    let activation = skill_activation(block.and_then(|block| block.get("activation")))?;
    let Some(id) = activation
        .get("hook_ref")
        .and_then(|hook_ref| hook_ref["id"].as_str())
    else {
        return Err(Error::Unsupported("the skill declares no hook_ref".into()));
    };
    let resolved = known_hooks.contains(&id);
    let cross_reference = CrossReference {
        source_path: "skill.activation.hook_ref",
        target_kind: Kind::Hook,
        declared_name: id.to_string(),
        resolution: if resolved {
            Resolution::Resolved
        } else {
            Resolution::Unresolved
        },
    };
    let diagnostics = if resolved {
        Vec::new()
    } else {
        vec![Diagnostic::new(
            "acif.registry.reference_unresolved",
            &[("declared_name", Value::from(id))],
        )]
    };
    Ok(HookRefResolution {
        reciprocal_entries: vec![cross_reference.clone()],
        cross_reference,
        diagnostics,
    })
}

/// The kind of an item given as a JSON object: its `kind`, else the one
/// kind block it carries.
fn item_kind(item: &Map<String, Value>) -> Result<Kind, Error> {
    let kind = match item.get("kind") {
        Some(kind) => kind.as_str().and_then(Kind::parse),
        None if item.contains_key("skill") => Some(Kind::Skill),
        None if item.contains_key("rule") => Some(Kind::Rule),
        None => None,
    };
    match kind {
        Some(kind @ (Kind::Skill | Kind::Rule)) => Ok(kind),
        _ => Err(Error::Unsupported(
            "only skill and rule items are implemented".into(),
        )),
    }
}

fn rule_item_activation(item: &Map<String, Value>) -> Result<Map<String, Value>, Error> {
    rule_activation(kind_block(item, Kind::Rule)?.and_then(|block| block.get("activation")))
}

/// The rule §9.1 predicate, in its positive-set form: a future mode is
/// derivable-false until it is added here.
fn is_gated(mode: &Value) -> bool {
    matches!(mode.as_str(), Some("glob" | "manual" | "model_decision"))
}

fn object(value: Value) -> Map<String, Value> {
    match value {
        Value::Object(map) => map,
        _ => unreachable!("object() takes a JSON object literal"),
    }
}
