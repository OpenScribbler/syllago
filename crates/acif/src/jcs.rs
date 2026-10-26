use serde_json::{Map, Value};

use crate::Hash;

/// Serializes `value` with the JSON Canonicalization Scheme (RFC 8785):
/// object keys sorted by UTF-16 code units, no whitespace, and numbers in
/// ECMAScript form.
pub fn canonical_json(value: &Value) -> String {
    let mut out = String::new();
    write_value(value, &mut out);
    out
}

/// The `metadata_hash` of a `publisher_section` (publisher §6): SHA-256 over
/// its JCS serialization followed by one LF.
pub fn metadata_hash(publisher_section: &Map<String, Value>) -> Hash {
    let mut preimage = String::new();
    write_object(publisher_section, &mut preimage);
    preimage.push('\n');
    Hash::sha256(preimage.as_bytes())
}

/// Numbers are written as IEEE 754 doubles, as RFC 8785 requires, so an
/// integer past 2^53 hashes as its nearest double.
fn write_value(value: &Value, out: &mut String) {
    match value {
        Value::Null => out.push_str("null"),
        Value::Bool(value) => out.push_str(if *value { "true" } else { "false" }),
        Value::Number(number) => out.push_str(&es_number(number.as_f64().unwrap_or(f64::NAN))),
        Value::String(string) => write_string(string, out),
        Value::Array(items) => {
            out.push('[');
            for (index, item) in items.iter().enumerate() {
                if index > 0 {
                    out.push(',');
                }
                write_value(item, out);
            }
            out.push(']');
        }
        Value::Object(object) => write_object(object, out),
    }
}

fn write_object(object: &Map<String, Value>, out: &mut String) {
    let mut keys: Vec<&String> = object.keys().collect();
    keys.sort_by(|a, b| a.encode_utf16().cmp(b.encode_utf16()));
    out.push('{');
    for (index, key) in keys.into_iter().enumerate() {
        if index > 0 {
            out.push(',');
        }
        write_string(key, out);
        out.push(':');
        write_value(&object[key], out);
    }
    out.push('}');
}

/// serde_json escapes exactly what RFC 8785 requires: `"`, `\`, and control
/// characters, using `\b \f \n \r \t` and lowercase `\u00xx` otherwise.
fn write_string(string: &str, out: &mut String) {
    out.push_str(&Value::from(string).to_string());
}

/// Formats a finite double the way ECMAScript `Number.prototype.toString`
/// does, which RFC 8785 §3.2.2.3 requires.
fn es_number(value: f64) -> String {
    ryu_js::Buffer::new().format(value).to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    // The vectors hold no floating-point metadata, so they never exercise the
    // ECMAScript number layout. Expected strings are what JavaScript prints.
    #[test]
    fn numbers_use_ecmascript_form() {
        let cases = [
            (0.0, "0"),
            (-0.0, "0"),
            (1.0, "1"),
            (-1.5, "-1.5"),
            (123.0, "123"),
            (0.1, "0.1"),
            (0.000001, "0.000001"),
            (1e-7, "1e-7"),
            (1.5e-7, "1.5e-7"),
            (1e20, "100000000000000000000"),
            (1e21, "1e+21"),
            (1.2345e25, "1.2345e+25"),
            (9007199254740993.0, "9007199254740992"),
            (5e-324, "5e-324"),
            (1.7976931348623157e308, "1.7976931348623157e+308"),
        ];
        for (value, expected) in cases {
            assert_eq!(es_number(value), expected, "{value:e}");
        }
    }

    // RFC 8785 Appendix B, as IEEE 754 bit patterns. The last is a tie that
    // ECMAScript breaks toward the even digit.
    #[test]
    fn numbers_match_rfc8785_appendix_b() {
        let cases = [
            (0x8000_0000_0000_0001, "-5e-324"),
            (0x4430_0000_0000_0000, "295147905179352830000"),
            (0x44b5_2d02_c7e1_4af5, "9.999999999999997e+22"),
            (0x44b5_2d02_c7e1_4af6, "1e+23"),
            (0x44b5_2d02_c7e1_4af7, "1.0000000000000001e+23"),
            (0x444b_1ae4_d6e2_ef4e, "999999999999999700000"),
            (0x444b_1ae4_d6e2_ef50, "1e+21"),
            (0x3eb0_c6f7_a0b5_ed8c, "9.999999999999997e-7"),
            (0x41b3_de43_5555_5554, "333333333.33333325"),
            (0x41b3_de43_5555_5557, "333333333.33333343"),
            (0xbecb_f647_612f_3696, "-0.0000033333333333333333"),
            (0x4314_3ff3_c1cb_0959, "1424953923781206.2"),
        ];
        for (bits, expected) in cases {
            assert_eq!(es_number(f64::from_bits(bits)), expected, "{bits:#x}");
        }
    }

    #[test]
    fn keys_sort_by_utf16_code_units() {
        // U+FB01 sorts before U+1F600 in UTF-8 but after it in UTF-16.
        let value = serde_json::json!({"\u{1F600}": 1, "\u{FB01}": 2, "a": [true, null, "\u{1}"]});
        assert_eq!(
            canonical_json(&value),
            "{\"a\":[true,null,\"\\u0001\"],\"\u{1F600}\":1,\"\u{FB01}\":2}"
        );
    }
}
