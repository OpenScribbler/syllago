use saphyr::{LoadableYamlNode, Scalar, Yaml};
use saphyr_parser::{Event, Parser};
use serde_json::{Map, Number, Value};

use crate::{Error, Kind, malformed};

/// Envelope fields (core §5.1). Every other frontmatter key belongs to the
/// kind's extension block.
const ENVELOPE_FIELDS: [&str; 7] = [
    "kind",
    "id",
    "display_name",
    "version",
    "description",
    "license",
    "pack_id",
];

/// The deepest frontmatter nesting accepted. The tree build and its
/// conversion recurse once per level, so unbounded nesting overflows the
/// stack.
const MAX_NESTING: usize = 64;

/// Reads a YAML 1.2 frontmatter block into the `publisher_section` it
/// declares, or `None` when the block declares nothing. Keys outside the
/// envelope go into the block of the kind the frontmatter declares, else of
/// `kind`.
pub(crate) fn publisher_section(
    kind: Kind,
    block: &[u8],
) -> Result<Option<Map<String, Value>>, Error> {
    let text = std::str::from_utf8(block).map_err(|_| malformed("frontmatter is not UTF-8"))?;
    scan_events(text)?;
    let documents =
        Yaml::load_from_str(text).map_err(|error| malformed(format!("frontmatter: {error}")))?;
    let mut fields = match documents.as_slice() {
        [] => return Ok(None),
        [document] => match to_json(document)? {
            Value::Null => return Ok(None),
            Value::Object(fields) => fields,
            _ => return Err(malformed("frontmatter is not a mapping")),
        },
        _ => return Err(malformed("frontmatter holds more than one YAML document")),
    };
    // A null-valued top-level key is absent, dropped before every later step;
    // a block left with no key, `{}` included, declares nothing (publisher
    // §5.6 steps 2 and 3).
    fields.retain(|_, value| !value.is_null());
    if fields.is_empty() {
        return Ok(None);
    }
    let kind = fields
        .get("kind")
        .and_then(Value::as_str)
        .and_then(Kind::parse)
        .unwrap_or(kind);
    let mut section = Map::new();
    let mut block = Map::new();
    for (key, value) in fields {
        // A non-mapping `license` is the Agent Skills string form, not the
        // envelope's license object, so it stays in the kind block.
        let envelope_license = key != "license" || value.is_object();
        if ENVELOPE_FIELDS.contains(&key.as_str()) && envelope_license {
            section.insert(key, value);
        } else {
            block.insert(key, value);
        }
    }
    if !block.is_empty() {
        section.insert(kind.as_str().to_string(), Value::Object(block));
    }
    Ok(Some(section))
}

/// Rejects anchors, aliases and nesting deeper than [`MAX_NESTING`] from the
/// parser's events, before the loader builds a tree: the loader expands
/// every alias, so nested aliases grow exponentially, and it recurses once
/// per nesting level.
fn scan_events(text: &str) -> Result<(), Error> {
    let mut depth = 0;
    for event in Parser::new_from_str(text) {
        let (event, _) = event.map_err(|error| malformed(format!("frontmatter: {error}")))?;
        let anchored = match event {
            Event::Alias(_) => true,
            Event::Scalar(_, _, anchor, _) => anchor != 0,
            Event::SequenceStart(anchor, _) | Event::MappingStart(anchor, _) => {
                depth += 1;
                if depth > MAX_NESTING {
                    return Err(malformed(format!(
                        "frontmatter nests deeper than {MAX_NESTING} levels"
                    )));
                }
                anchor != 0
            }
            Event::SequenceEnd | Event::MappingEnd => {
                depth -= 1;
                false
            }
            _ => false,
        };
        if anchored {
            return Err(malformed("frontmatter holds an anchor or alias"));
        }
    }
    Ok(())
}

fn to_json(node: &Yaml) -> Result<Value, Error> {
    Ok(match node {
        Yaml::Value(Scalar::Null) => Value::Null,
        Yaml::Value(Scalar::Boolean(value)) => Value::Bool(*value),
        Yaml::Value(Scalar::Integer(value)) => Value::from(*value),
        Yaml::Value(Scalar::FloatingPoint(value)) => Number::from_f64(value.into_inner())
            .map(Value::Number)
            .ok_or_else(|| malformed("frontmatter holds a non-finite number"))?,
        Yaml::Value(Scalar::String(value)) => Value::String(value.to_string()),
        Yaml::Sequence(items) => Value::Array(items.iter().map(to_json).collect::<Result<_, _>>()?),
        Yaml::Mapping(entries) => {
            let mut object = Map::new();
            for (key, value) in entries {
                let Yaml::Value(Scalar::String(key)) = key else {
                    return Err(malformed("frontmatter has a non-string key"));
                };
                object.insert(key.to_string(), to_json(value)?);
            }
            Value::Object(object)
        }
        _ => {
            return Err(malformed(
                "frontmatter holds a tag, alias or value outside the YAML core schema",
            ));
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    // Null-valued top-level keys are dropped before the forbidden-field check
    // and placement; nested nulls stay. A block holding only nulls, or `{}`,
    // declares nothing (publisher §5.6 steps 2 and 3).
    #[test]
    fn null_top_level_keys_are_absent() {
        let section = |document: &str| publisher_section(Kind::Skill, document.as_bytes()).unwrap();
        assert_eq!(
            section("display_name: Demo\nresolved_version:\ndescription: ~\nallowed-tools: null\n"),
            Some(
                serde_json::json!({"display_name": "Demo"})
                    .as_object()
                    .unwrap()
                    .clone()
            )
        );
        assert_eq!(
            section("display_name: Demo\nmetadata:\n  x: null\n"),
            Some(
                serde_json::json!({"display_name": "Demo", "skill": {"metadata": {"x": null}}})
                    .as_object()
                    .unwrap()
                    .clone()
            )
        );
        assert_eq!(section("description:\n"), None);
        assert_eq!(section("{}\n"), None);
    }

    // Aliases are rejected before the loader can expand them: a few hundred
    // bytes of nested aliases otherwise expand to gigabytes. These documents
    // are small, so a loader that expands them still terminates.
    #[test]
    fn anchors_and_aliases_are_malformed() {
        for document in [
            "a: &x 1\n",
            "a: &x [1]\nb: *x\n",
            "a0: &a0 [0]\na1: &a1 [*a0, *a0]\na2: &a2 [*a1, *a1]\n",
        ] {
            let result = publisher_section(Kind::Skill, document.as_bytes());
            assert!(matches!(result, Err(Error::Malformed(_))), "{document}");
        }
    }

    // The parser bounds flow nesting only. Block nesting this deep overflows
    // the stack in the recursive tree build unless the event scan stops it.
    #[test]
    fn deep_block_nesting_is_malformed() {
        let document = format!("x:\n{}0\n", "- ".repeat(3000));
        let result = publisher_section(Kind::Skill, document.as_bytes());
        assert!(matches!(result, Err(Error::Malformed(_))));
        let limit = format!("x:\n{}0\n", "- ".repeat(MAX_NESTING - 1));
        assert!(publisher_section(Kind::Skill, limit.as_bytes()).is_ok());
    }
}
