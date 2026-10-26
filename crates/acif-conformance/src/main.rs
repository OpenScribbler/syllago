//! The ACIF conformance-runner adapter (adapter protocol 2). It decodes one
//! JSON request per stdin line, dispatches it to the `acif` crate, and writes
//! one JSON response line per request. All ACIF logic lives in the crate.

use std::io::{self, BufRead, Write};
use std::path::Path;

use acif::{
    Body, CrossReference, Diagnostic, Error, Install, Item, Kind, Map, PACK_NAMESPACE, RuleTarget,
    Value, canonical_json, derive_pack_id, derived_capabilities, evaluate_requires, ingest_body,
    ingest_rule_activation_source, ingest_sidecar, render_rule, resolve_pack,
    resolve_skill_hook_ref, rule_activation_projection, validate_registry_record,
};
use serde_json::json;

fn main() -> io::Result<()> {
    serve(io::stdin().lock(), io::stdout().lock())
}

fn serve(input: impl BufRead, mut out: impl Write) -> io::Result<()> {
    for line in input.split(b'\n') {
        let line = line?;
        let line = line.strip_suffix(b"\r").unwrap_or(&line);
        if line.trim_ascii().is_empty() {
            continue;
        }
        let response = match serde_json::from_slice::<Value>(line) {
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
            // Only TV-L3-e of the registry scope is implemented, but the
            // runner skips a vector outside every claimed scope, even under
            // --only.
            "scopes": ["core", "skill", "rule", "registry"],
        }});
    }
    let result = match (request["op"].as_str(), request["input"].as_object()) {
        (Some(op), Some(input)) => dispatch(op, input),
        (None, _) => Err(adapter_error("op is not a string")),
        (Some(_), None) => Err(adapter_error("input is not an object")),
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
            Ok(json!({
                "output": rendered.output,
                "diagnostics": diagnostics(&rendered.diagnostics),
                "lossy": rendered.lossy,
            }))
        }
        "resolve_reference" => {
            let known = strings(array(object(input, "registry_state")?, "known_hooks")?)?;
            let resolution = resolve_skill_hook_ref(object(input, "item")?, &known)?;
            let reciprocal_entries: Vec<Value> = resolution
                .reciprocal_entries
                .iter()
                .map(cross_reference)
                .collect();
            Ok(json!({
                "cross_reference": cross_reference(&resolution.cross_reference),
                "reciprocal_entries": reciprocal_entries,
                "diagnostics": diagnostics(&resolution.diagnostics),
            }))
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
        return Ok(item_result(ingest_sidecar(
            kind,
            object(input, "sidecar")?,
        )?));
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
    if let Some(install) = item.install {
        result["installable"] = Value::from(install == Install::Proceed);
    }
    if let Some(section) = &item.publisher_section {
        result["metadata_hash"] = Value::from(item.metadata_hash().map(|hash| hash.value));
        result["canonical_bytes"] = Value::from(canonical_json(&Value::Object(section.clone())));
    }
    result
}

fn cross_reference(reference: &CrossReference) -> Value {
    json!({
        "source_path": reference.source_path,
        "target_kind": reference.target_kind.as_str(),
        "resolution": reference.resolution.as_str(),
    })
}

fn diagnostics(diagnostics: &[Diagnostic]) -> Vec<Value> {
    diagnostics
        .iter()
        .map(|diagnostic| json!({"id": diagnostic.id, "params": diagnostic.params}))
        .collect()
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

#[cfg(test)]
mod tests {
    use super::*;

    fn responses(input: &[u8]) -> Vec<Value> {
        let mut out = Vec::new();
        serve(input, &mut out).unwrap();
        out.split(|&byte| byte == b'\n')
            .filter(|line| !line.is_empty())
            .map(|line| serde_json::from_slice(line).unwrap())
            .collect()
    }

    // A harness bug is an adapter error, not a coverage gap.
    #[test]
    fn request_without_input_is_an_adapter_error() {
        let [response] = &responses(b"{\"op\": \"ingest\"}\n")[..] else {
            panic!("expected one response");
        };
        assert_eq!(response["ok"], false);
        assert!(response.get("unsupported").is_none());
    }

    // One bad line answers with an error; the requests after it still run.
    #[test]
    fn non_utf8_line_does_not_stop_the_adapter() {
        let responses = responses(b"\xff\n{\"op\": \"hello\"}\n");
        assert_eq!(responses.len(), 2);
        assert_eq!(responses[0]["ok"], false);
        assert_eq!(responses[1]["ok"], true);
    }
}
