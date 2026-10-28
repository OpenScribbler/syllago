use std::borrow::Cow;

use saphyr::{LoadableYamlNode, Scalar, Yaml};
use saphyr_parser::{Event, Parser, ScalarStyle};
use serde_json::{Map, Number, Value};

use crate::envelope::check_forbidden_fields;
use crate::{Error, Kind};

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

/// A block that fails the core §2 parse rejects with
/// `acif.frontmatter.malformed` (publisher §5.6 step 1). `_reason` names the
/// condition for the reader; the identifier carries no detail.
fn malformed(_reason: impl Into<String>) -> Error {
    Error::Reject("acif.frontmatter.malformed")
}

/// The largest integer magnitude frontmatter may hold (core §2): I-JSON
/// bounds integers to the doubles that hold them exactly.
const MAX_SAFE_INTEGER: u64 = (1 << 53) - 1;

/// The deepest frontmatter nesting accepted. The tree build and its
/// conversion recurse once per level, so unbounded nesting overflows the
/// stack.
const MAX_NESTING: usize = 64;

/// Reads a YAML 1.2 frontmatter block into the `publisher_section` it
/// declares, or `None` when the block declares nothing (publisher §5.6).
/// Keys outside the envelope go into the block of `kind`, the kind the
/// entry file assigns.
pub(crate) fn publisher_section(
    kind: Kind,
    block: &[u8],
) -> Result<Option<Map<String, Value>>, Error> {
    let text = std::str::from_utf8(block).map_err(|_| malformed("frontmatter is not UTF-8"))?;
    // Raw characters outside YAML 1.2 c-printable, and the raw line
    // separators YAML 1.1 parsers break lines at, reject anywhere in the
    // block; their escapes in a double-quoted scalar stay legal (core §2).
    if text.chars().any(|c| {
        !matches!(c, '\t' | '\n' | '\r' | ' '..='~' | '\u{A0}'..='\u{D7FF}' | '\u{E000}'..='\u{FFFD}' | '\u{10000}'..)
            || matches!(c, '\u{2028}' | '\u{2029}')
    }) {
        return Err(malformed("frontmatter holds a raw character outside c-printable"));
    }
    let text = separate_tabs(text);
    scan_events(&text)?;
    let documents =
        Yaml::load_from_str(&text).map_err(|error| malformed(format!("frontmatter: {error}")))?;
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
    // Every top-level key is checked, since a forbidden name is never an
    // envelope field and so lands in the kind block (step 4). A valid
    // declared kind other than the assigned one rejects rather than moving
    // the extension keys to its block; an invalid one is left for the
    // envelope check (step 5).
    check_forbidden_fields(&fields)?;
    if let Some(declared) = fields
        .get("kind")
        .and_then(Value::as_str)
        .and_then(Kind::parse)
        && declared != kind
    {
        return Err(Error::Reject("acif.publisher.frontmatter_kind_mismatch"));
    }
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

/// Rejects anchors, aliases, explicit tags and nesting deeper than
/// [`MAX_NESTING`] from the parser's events, before the loader builds a
/// tree: the loader expands every alias, so nested aliases grow
/// exponentially, and it recurses once per nesting level.
fn scan_events(text: &str) -> Result<(), Error> {
    let mut depth = 0;
    for event in Parser::new_from_str(text) {
        let (event, _) = event.map_err(|error| malformed(format!("frontmatter: {error}")))?;
        let (anchored, tagged) = match event {
            Event::Alias(_) => (true, false),
            Event::Scalar(value, style, anchor, tag) => {
                if style == ScalarStyle::Plain && !int_in_range(&value) {
                    return Err(malformed("frontmatter holds an integer beyond 2^53-1"));
                }
                (anchor != 0, tag.is_some())
            }
            Event::SequenceStart(anchor, tag) | Event::MappingStart(anchor, tag) => {
                depth += 1;
                if depth > MAX_NESTING {
                    return Err(malformed(format!(
                        "frontmatter nests deeper than {MAX_NESTING} levels"
                    )));
                }
                (anchor != 0, tag.is_some())
            }
            Event::SequenceEnd | Event::MappingEnd => {
                depth -= 1;
                (false, false)
            }
            _ => (false, false),
        };
        if anchored {
            return Err(malformed("frontmatter holds an anchor or alias"));
        }
        if tagged {
            return Err(malformed("frontmatter holds an explicit tag"));
        }
    }
    Ok(())
}

/// The text with each `:` TAB the scanner would refuse made `:` SPACE.
/// YAML 1.2 separates with a tab as with a space, but saphyr's scanner
/// refuses a tab after a value indicator's colon. Outside a quoted or block
/// scalar or a comment, `:` followed by whitespace is always an indicator,
/// so the swap changes no value there; inside a scalar the tab is content,
/// so a first parse of the fully swapped text finds those scalars and
/// their tabs are put back. A text the swap leaves unparseable is returned
/// swapped, for the event scan to reject. Where the scalars cannot be
/// placed, the text is returned as it was, and the event scan rejects the
/// first refused tab.
fn separate_tabs(text: &str) -> Cow<'_, str> {
    if !text.contains(":\t") {
        return Cow::Borrowed(text);
    }
    let swapped = text.replace(":\t", ": ");
    // The scanner counts a directive's parameters in bytes and everything
    // else in characters. Directive lines, which can only come before the
    // first `---`, are made comments of the same length for this parse, so
    // every marker counts characters.
    let mut located = swapped.clone().into_bytes();
    let mut at = 0;
    for line in swapped.split_inclusive('\n') {
        if line.starts_with("---") {
            break;
        }
        if line.starts_with('%') {
            located[at] = b'#';
        }
        at += line.len();
    }
    let located = String::from_utf8(located).expect("only ASCII bytes were swapped");
    let byte_at: Vec<usize> = located
        .char_indices()
        .map(|(at, _)| at)
        .chain([located.len()])
        .collect();
    let mut content = Vec::new();
    for event in Parser::new_from_str(&located) {
        match event {
            Ok((Event::Scalar(_, style, ..), span)) if style != ScalarStyle::Plain => {
                let (Some(&start), Some(&end)) = (
                    byte_at.get(span.start.index()),
                    byte_at.get(span.end.index()),
                ) else {
                    return Cow::Borrowed(text);
                };
                let quote = match style {
                    ScalarStyle::DoubleQuoted => Some(b'"'),
                    ScalarStyle::SingleQuoted => Some(b'\''),
                    _ => None,
                };
                if quote.is_some_and(|quote| located.as_bytes().get(start) != Some(&quote)) {
                    return Cow::Borrowed(text);
                }
                content.push(start..end);
            }
            Ok(_) => {}
            Err(_) => return Cow::Owned(swapped),
        }
    }
    let mut bytes = swapped.into_bytes();
    for range in content {
        for at in range.clone().skip(1) {
            if text.as_bytes().get(at - 1..=at) == Some(b":\t") {
                bytes[at] = b'\t';
            }
        }
    }
    Cow::Owned(String::from_utf8(bytes).expect("only ASCII bytes were swapped"))
}

fn to_json(node: &Yaml) -> Result<Value, Error> {
    Ok(match node {
        Yaml::Value(Scalar::Null) => Value::Null,
        Yaml::Value(Scalar::Boolean(value)) => Value::Bool(*value),
        // The loader also reads signed hex and octal and a doubled sign as
        // ints, which the source-text check does not see.
        Yaml::Value(Scalar::Integer(value)) if value.unsigned_abs() <= MAX_SAFE_INTEGER => {
            Value::from(*value)
        }
        Yaml::Value(Scalar::Integer(_)) => {
            return Err(malformed("frontmatter holds an integer beyond 2^53-1"));
        }
        Yaml::Value(Scalar::FloatingPoint(value)) => Number::from_f64(value.into_inner())
            .map(Value::Number)
            .ok_or_else(|| malformed("frontmatter holds a non-finite number"))?,
        Yaml::Value(Scalar::String(value)) => Value::String(checked_string(value)?),
        Yaml::Sequence(items) => Value::Array(items.iter().map(to_json).collect::<Result<_, _>>()?),
        Yaml::Mapping(entries) => {
            let mut object = Map::new();
            for (key, value) in entries {
                let Yaml::Value(Scalar::String(key)) = key else {
                    return Err(malformed("frontmatter has a non-string key"));
                };
                object.insert(checked_string(key)?, to_json(value)?);
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

/// False for a plain scalar in a core-schema int form (decimal, `0o`
/// octal, `0x` hex) whose magnitude passes 2^53-1. The loader reads a
/// decimal past the i64 range as a float, so the limit is applied to the
/// source text.
fn int_in_range(plain: &str) -> bool {
    let (digits, radix) = if let Some(octal) = plain.strip_prefix("0o") {
        (octal, 8)
    } else if let Some(hex) = plain.strip_prefix("0x") {
        (hex, 16)
    } else {
        (plain.strip_prefix(['-', '+']).unwrap_or(plain), 10)
    };
    if digits.is_empty() || !digits.chars().all(|c| c.is_digit(radix)) {
        return true;
    }
    u64::from_str_radix(digits, radix).is_ok_and(|value| value <= MAX_SAFE_INTEGER)
}

/// A key or string value, rejected when it holds a Unicode noncharacter,
/// raw or escaped (core §2).
fn checked_string(value: &str) -> Result<String, Error> {
    let noncharacter =
        |c: char| matches!(c, '\u{FDD0}'..='\u{FDEF}') || (c as u32) & 0xFFFE == 0xFFFE;
    if value.chars().any(noncharacter) {
        return Err(malformed("frontmatter holds a noncharacter"));
    }
    Ok(value.to_string())
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
            assert!(
                matches!(result, Err(Error::Reject("acif.frontmatter.malformed"))),
                "{document}"
            );
        }
    }

    // The parser bounds flow nesting only. Block nesting this deep overflows
    // the stack in the recursive tree build unless the event scan stops it.
    #[test]
    fn deep_block_nesting_is_malformed() {
        let document = format!("x:\n{}0\n", "- ".repeat(3000));
        let result = publisher_section(Kind::Skill, document.as_bytes());
        assert!(matches!(
            result,
            Err(Error::Reject("acif.frontmatter.malformed"))
        ));
        let limit = format!("x:\n{}0\n", "- ".repeat(MAX_NESTING - 1));
        assert!(publisher_section(Kind::Skill, limit.as_bytes()).is_ok());
    }

    fn reject(document: &str) -> Option<&'static str> {
        match publisher_section(Kind::Skill, document.as_bytes()) {
            Err(Error::Reject(id)) => Some(id),
            _ => None,
        }
    }

    // A tab after a value indicator separates as a space does (YAML 1.2),
    // though the scanner refuses it; a tab inside a quoted or block scalar
    // stays, after multibyte text too.
    #[test]
    fn tab_separator_is_whitespace() {
        let section = |document: &str| publisher_section(Kind::Skill, document.as_bytes()).unwrap();
        assert_eq!(
            section("display_name:\tDemo\n"),
            section("display_name: Demo\n")
        );
        assert_eq!(
            section("description: Café — x\ndisplay_name:\t\tDemo\n"),
            section("description: Café — x\ndisplay_name: Demo\n")
        );
        for padding in ["", "# pad\n"] {
            let document = format!("%FOO ééé\n--- \nx: \"a:\tb\"\ny:\t1\n{padding}");
            assert_eq!(
                section(&document),
                section("x: \"a:\tb\"\ny: 1\n"),
                "{document:?}"
            );
        }
        assert_eq!(
            section("x: \"éééé:\tb\"\n").unwrap()["skill"]["x"],
            "éééé:\tb"
        );
        assert_eq!(
            section("é:\t'a:\tb'\nc: |\n  d:\te\nf:\t[g:\th]\n"),
            Some(
                serde_json::json!({"skill": {"é": "a:\tb", "c": "d:\te\n", "f": [{"g": "h"}]}})
                    .as_object()
                    .unwrap()
                    .clone()
            )
        );
        assert_eq!(
            section("display_name:\t\"a:\tb\"\nx:\t{y:\t1}\n"),
            Some(
                serde_json::json!({"display_name": "a:\tb", "skill": {"x": {"y": 1}}})
                    .as_object()
                    .unwrap()
                    .clone()
            )
        );
    }

    // A valid declared kind other than the entry file's rejects instead of
    // moving the extension keys, after the forbidden-field check (publisher
    // §5.6 steps 4 and 5); an invalid one is left to the envelope check.
    #[test]
    fn declared_kind_must_match_entry_file() {
        assert_eq!(
            reject("kind: rule\ndisplay_name: Demo\n"),
            Some("acif.publisher.frontmatter_kind_mismatch")
        );
        assert!(matches!(
            publisher_section(Kind::Skill, b"kind: rule\nresolved_version: 1.0.0\n"),
            Err(Error::Nonconformant(diagnostic)) if diagnostic.id == "acif.envelope.forbidden_field"
        ));
        assert!(publisher_section(Kind::Skill, b"kind: skill\nx: 1\n").is_ok());
        assert!(publisher_section(Kind::Skill, b"kind: widget\nx: 1\n").is_ok());
    }

    // Core §2 conditions the parser accepts on its own.
    #[test]
    fn core_schema_limits_are_malformed() {
        for document in [
            "a: !!str x\n",
            "a: ! x\n",
            "a: !local x\n",
            "a: !<tag:yaml.org,2002:str> x\n",
            "a: \"\u{1B}[1m\"\n",
            "a: one\u{2028}two\n",
            "a: \"\\uFDD0\"\n",
            "\u{FDD0}: x\n",
            "a: 9007199254740992\n",
            "a: -9007199254740992\n",
            "a: 99999999999999999999\n",
            "a: 0x20000000000000\n",
            "a: 0x-20000000000000\n",
            "a: ++9007199254740992\n",
            "a: 0o+777777777777777777\n",
        ] {
            assert_eq!(
                reject(document),
                Some("acif.frontmatter.malformed"),
                "{document:?}"
            );
        }
        for document in [
            "a: 9007199254740991\n",
            "a: -9007199254740991\n",
            "a: 1e20\n",
            "a: \"\\e[1m\"\n",
            "a: \"99999999999999999999\"\n",
        ] {
            assert!(
                publisher_section(Kind::Skill, document.as_bytes()).is_ok(),
                "{document:?}"
            );
        }
    }
}
