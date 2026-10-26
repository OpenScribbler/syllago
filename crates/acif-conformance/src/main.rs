//! The ACIF conformance-runner adapter (adapter protocol 2). It decodes one
//! JSON request per stdin line, dispatches it to the `acif` crate, and writes
//! one JSON response line per request. All ACIF logic lives in the crate.

use std::io::{self, BufRead, Write};
use std::path::Path;

use acif::{
    Body, Error, Install, Item, Kind, Map, PACK_NAMESPACE, RuleTarget, Value, canonical_json,
    derive_pack_id, derived_capabilities, evaluate_requires, ingest_body,
    ingest_rule_activation_source, ingest_sidecar, render_rule, resolve_pack,
    resolve_skill_hook_ref, rule_activation_projection, validate_registry_record,
};
use serde_json::json;

fn main() -> io::Result<()> {
    let stdout = io::stdout();
    let mut out = stdout.lock();
    for line in io::stdin().lock().lines() {
        let line = line?;
        let line = line.trim_end_matches('\r');
        if line.trim().is_empty() {
            continue;
        }
        let response = match serde_json::from_str::<Value>(line) {
            Ok(request) => respond(&request),
            Err(error) => {
                json!({"ok": false, "error": format!("adapter: request is not JSON: {error}")})
            }
        };
        writeln!(out, "{response}")?;
        out.flush()?;
    }
    Ok(())
}

fn respond(request: &Value) -> Value {
    if request["op"] == "hello" {
        return json!({"ok": true, "result": {
            "implementation": "acif (Rust)",
            "version": env!("CARGO_PKG_VERSION"),
            "adapter_protocol": 2,
            "scopes": ["core", "skill", "rule", "registry"],
        }});
    }
    let input = request.get("input").and_then(Value::as_object);
    let result = match (request["op"].as_str(), input) {
        (Some(op), Some(input)) => dispatch(op, input),
        _ => Err(Error::Unsupported("unknown request".into())),
    };
    match result {
        Ok(result) => json!({"ok": true, "result": result}),
        Err(Error::Reject(id)) => json!({"ok": false, "error": id, "diagnostics": []}),
        Err(Error::Nonconformant(reason)) => json!({"ok": true, "result": {
            "conformant": false,
            "reason": reason.id,
            "params": reason.params,
        }}),
        Err(Error::Unsupported(_)) => json!({"unsupported": true}),
        Err(error @ (Error::Malformed(_) | Error::Io(_))) => {
            json!({"ok": false, "error": format!("adapter: {error}")})
        }
    }
}

fn dispatch(op: &str, input: &Map<String, Value>) -> Result<Value, Error> {
    match op {
        "ingest" => ingest(input),
        "derive_pack_id" => {
            if input.get("namespace").and_then(Value::as_str) != Some(PACK_NAMESPACE) {
                return Err(Error::Unsupported("unknown pack-id namespace".into()));
            }
            let id = derive_pack_id(
                string(input, "repository_url")?,
                string(input, "display_name")?,
            );
            Ok(json!({"inferred_pack_id": id}))
        }
        "resolve_pack" => {
            let known: Vec<&str> = array(input, "known_packs")?
                .iter()
                .filter_map(|pack| pack.as_str().or_else(|| pack.get("id")?.as_str()))
                .collect();
            let resolution = resolve_pack(object(input, "item")?, &known);
            let mut result = json!({"install": resolution.install().as_str()});
            if let Some(member_of) = resolution.member_of() {
                result["member_of"] = Value::from(member_of);
            }
            if let Some(name) = resolution.as_str() {
                result["pack_resolution"] = Value::from(name);
            }
            Ok(result)
        }
        "evaluate_requires" => {
            let recognizes = strings(array(input, "consumer_recognizes")?)?;
            let (evaluation, install) =
                evaluate_requires(object(input, "item_requires")?, &recognizes)?;
            Ok(json!({"evaluation": evaluation.as_str(), "install": install.as_str()}))
        }
        "project" => {
            let item = object(input, "item")?;
            match string(input, "projection")? {
                "derived_capabilities" => {
                    Ok(json!({"derived_capabilities": derived_capabilities(item)?}))
                }
                "rule_activation" => Ok(json!({"projection": rule_activation_projection(item)?})),
                other => Err(Error::Unsupported(format!("projection {other}"))),
            }
        }
        "render" => {
            let target = match string(input, "target")? {
                "native-rule-format" => RuleTarget::NativeRuleFormat,
                "no-declaration-surface-provider" => RuleTarget::NoDeclarationSurface,
                other => return Err(Error::Unsupported(format!("render target {other}"))),
            };
            let rendered = render_rule(object(input, "canonical")?, target)?;
            let diagnostics: Vec<Value> = rendered
                .diagnostics
                .iter()
                .map(|diagnostic| json!({"id": diagnostic.id, "params": diagnostic.params}))
                .collect();
            Ok(json!({"output": rendered.output, "diagnostics": diagnostics, "lossy": []}))
        }
        "resolve_reference" => {
            let known = strings(array(object(input, "registry_state")?, "known_hooks")?)?;
            let reference = resolve_skill_hook_ref(object(input, "item")?, &known)?;
            let entry = json!({
                "source_path": reference.source_path,
                "target_kind": reference.target_kind.as_str(),
                "resolution": reference.resolution.as_str(),
            });
            let diagnostics = match reference.resolution {
                acif::Resolution::Resolved => vec![],
                acif::Resolution::Unresolved => vec![json!({
                    "id": "acif.registry.reference_unresolved",
                    "params": {"declared_name": reference.declared_name},
                })],
            };
            Ok(
                json!({"cross_reference": entry, "reciprocal_entries": [entry], "diagnostics": diagnostics}),
            )
        }
        _ => Err(Error::Unsupported(format!("op {op}"))),
    }
}

fn ingest(input: &Map<String, Value>) -> Result<Value, Error> {
    let kind_name = string(input, "kind")?;
    let kind = Kind::parse(kind_name);
    let context = input.get("context");
    if context.and_then(|context| context.get("validation_surface"))
        == Some(&Value::from("registry_emit"))
    {
        let kind = kind.ok_or_else(|| Error::Unsupported(format!("kind {kind_name}")))?;
        validate_registry_record(kind, object(input, "sidecar")?)?;
        return Ok(json!({"conformant": true}));
    }
    if let Some(config) = input.get("provider_config") {
        if kind == Some(Kind::Rule) && config["provider"] == "rule-activation-source" {
            return Ok(item_result(ingest_rule_activation_source(
                config.get("content"),
            )?));
        }
        return Err(Error::Unsupported("provider_config".into()));
    }
    if let Some(root) = input.get("body_root") {
        let kind = kind.ok_or_else(|| Error::Unsupported(format!("kind {kind_name}")))?;
        let root = root
            .as_str()
            .ok_or_else(|| adapter_error("body_root is not a string"))?;
        let body = Body::read(Path::new(root))?;
        return Ok(item_result(ingest_body(
            kind,
            &body,
            string(input, "entry_file")?,
        )?));
    }
    if input.contains_key("sidecar") {
        let sidecar = object(input, "sidecar")?;
        let mut result = item_result(ingest_sidecar(kind, sidecar)?);
        if sidecar.contains_key("publisher_section") || sidecar.contains_key("registry_section") {
            let installable = resolve_pack(sidecar, &[]).install() == Install::Proceed;
            result["installable"] = Value::from(installable);
        }
        return Ok(result);
    }
    Err(Error::Unsupported(
        "ingest without a body or sidecar".into(),
    ))
}

fn item_result(item: Item) -> Value {
    let mut result = json!({
        "conformant": true,
        "canonical": item.canonical,
        "publisher_section": item.publisher_section,
        "diagnostics": [],
    });
    if let Some(body_hash) = &item.body_hash {
        result["body_hash"] = Value::from(body_hash.value.as_str());
    }
    if let Some(classification) = item.classification {
        result["classification"] = Value::from(classification.as_str());
    }
    if let Some(section) = &item.publisher_section {
        result["metadata_hash"] = Value::from(item.metadata_hash().map(|hash| hash.value));
        result["canonical_bytes"] = Value::from(canonical_json(&Value::Object(section.clone())));
    }
    result
}

fn adapter_error(message: &str) -> Error {
    Error::Malformed(message.to_string())
}

fn string<'a>(input: &'a Map<String, Value>, name: &str) -> Result<&'a str, Error> {
    input
        .get(name)
        .and_then(Value::as_str)
        .ok_or_else(|| adapter_error(&format!("{name} is not a string")))
}

fn object<'a>(input: &'a Map<String, Value>, name: &str) -> Result<&'a Map<String, Value>, Error> {
    input
        .get(name)
        .and_then(Value::as_object)
        .ok_or_else(|| adapter_error(&format!("{name} is not an object")))
}

fn array<'a>(input: &'a Map<String, Value>, name: &str) -> Result<&'a Vec<Value>, Error> {
    input
        .get(name)
        .and_then(Value::as_array)
        .ok_or_else(|| adapter_error(&format!("{name} is not an array")))
}

fn strings(values: &[Value]) -> Result<Vec<&str>, Error> {
    values
        .iter()
        .map(|value| {
            value
                .as_str()
                .ok_or_else(|| adapter_error("expected an array of strings"))
        })
        .collect()
}
