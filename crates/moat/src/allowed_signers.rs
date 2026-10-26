use std::collections::BTreeSet;
use std::fmt;

use sha2::{Digest, Sha256};

use crate::wire::{Reader, base64_decode, py_isspace};
use crate::{Fingerprint, KeyType, Refusal};

const SEQUENCE_PREFIX: &str = "# moat-sequence: ";
/// The largest sequence a list may carry, 2^53 - 1.
const MAX_SEQUENCE: u64 = (1 << 53) - 1;

/// A well-formed allowed-signers list (`.moat/allowed_signers`): its
/// sequence, its digest, and the fingerprints of the keys it allows.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AllowedSigners {
    sequence: u64,
    digest: ListDigest,
    keys: BTreeSet<Fingerprint>,
}

/// A list digest: `sha256:` and the lowercase hex SHA-256 of the list's
/// canonical text.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct ListDigest(String);

impl ListDigest {
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for ListDigest {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl AllowedSigners {
    /// Parses a list, refusing it as a whole with `allowed_signers_malformed`
    /// unless every line is blank, a comment, the one sequence line, or a
    /// key line `<fingerprint> <key type> <base64 key blob>` whose
    /// fingerprint and type match the blob, and the list has at least one
    /// key and exactly one sequence line.
    pub fn parse(bytes: &[u8]) -> Result<AllowedSigners, Refusal> {
        let canonical = canonical_text(bytes);
        let digest = ListDigest(format!("sha256:{}", hex(&Sha256::digest(&canonical))));
        let text = std::str::from_utf8(&canonical)
            .map_err(|_| Refusal::AllowedSignersMalformed("the list is not UTF-8".into()))?;
        let mut keys = BTreeSet::new();
        let mut sequences = Vec::new();
        for raw in text.split('\n') {
            let line = raw.trim_matches([' ', '\t']);
            if is_sequence_line_start(line) {
                let sequence = parse_sequence_line(raw).ok_or_else(|| {
                    Refusal::AllowedSignersMalformed(format!("malformed sequence line: {raw:?}"))
                })?;
                sequences.push(sequence);
                continue;
            }
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            keys.insert(parse_key_line(line)?);
        }
        if keys.is_empty() {
            return Err(Refusal::AllowedSignersMalformed(
                "the list has no keys".into(),
            ));
        }
        let [sequence] = sequences[..] else {
            return Err(Refusal::AllowedSignersMalformed(format!(
                "the list has {} sequence lines, not 1",
                sequences.len()
            )));
        };
        Ok(AllowedSigners {
            sequence,
            digest,
            keys,
        })
    }

    /// The list's `# moat-sequence: <n>` value.
    pub fn sequence(&self) -> u64 {
        self.sequence
    }

    /// The list digest, over the list's canonical text.
    pub fn digest(&self) -> &ListDigest {
        &self.digest
    }

    /// Whether the list allows the key with this fingerprint.
    pub fn contains(&self, fingerprint: &Fingerprint) -> bool {
        self.keys.contains(fingerprint)
    }
}

/// The list's bytes with a leading UTF-8 byte-order mark removed and each
/// CRLF and lone CR replaced by LF.
fn canonical_text(bytes: &[u8]) -> Vec<u8> {
    let bytes = bytes.strip_prefix(b"\xef\xbb\xbf").unwrap_or(bytes);
    let mut out = Vec::with_capacity(bytes.len());
    let mut iter = bytes.iter().peekable();
    while let Some(&b) = iter.next() {
        if b == b'\r' {
            iter.next_if_eq(&&b'\n');
            out.push(b'\n');
        } else {
            out.push(b);
        }
    }
    out
}

/// A comment whose text after `#` and optional whitespace begins with
/// `moat-sequence` claims to be the sequence line.
fn is_sequence_line_start(line: &str) -> bool {
    line.strip_prefix('#').is_some_and(|rest| {
        rest.trim_start_matches(py_isspace)
            .starts_with("moat-sequence")
    })
}

/// `# moat-sequence: <n>` with nothing before or after, `<n>` a decimal
/// integer with no sign or leading zero, at most 2^53 - 1.
fn parse_sequence_line(raw: &str) -> Option<u64> {
    let digits = raw.strip_prefix(SEQUENCE_PREFIX)?;
    let well_formed = !digits.is_empty()
        && digits.len() <= 16
        && digits.bytes().all(|b| b.is_ascii_digit())
        && (digits == "0" || !digits.starts_with('0'));
    let sequence: u64 = digits.parse().ok().filter(|_| well_formed)?;
    (sequence <= MAX_SEQUENCE).then_some(sequence)
}

fn parse_key_line(line: &str) -> Result<Fingerprint, Refusal> {
    // A fourth field is enough to refuse the line, so stop there rather
    // than collecting every field of an attacker-sized line.
    let fields: Vec<&str> = line
        .split([' ', '\t'])
        .filter(|f| !f.is_empty())
        .take(4)
        .collect();
    let [principal, key_type, encoded] = fields[..] else {
        return Err(Refusal::AllowedSignersMalformed(format!(
            "malformed allowed-signers line: {line:?}"
        )));
    };
    let blob = base64_decode(encoded).ok_or_else(|| {
        Refusal::AllowedSignersMalformed(format!("key blob is not base64: {line:?}"))
    })?;
    let accepted = KeyType::from_wire(key_type.as_bytes()).is_some();
    if !accepted || Reader::new(&blob).string() != Some(key_type.as_bytes()) {
        return Err(Refusal::AllowedSignersMalformed(format!(
            "unaccepted or inconsistent key type: {key_type}"
        )));
    }
    let fingerprint = Fingerprint::of_key_blob(&blob);
    if principal != fingerprint.as_str() {
        return Err(Refusal::AllowedSignersMalformed(format!(
            "principal is not the key fingerprint: {principal}"
        )));
    }
    Ok(fingerprint)
}

pub(crate) fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn canonical_text_strips_bom_and_normalizes_line_ends() {
        assert_eq!(
            canonical_text(b"\xef\xbb\xbfa\r\nb\rc\n\r\r\n"),
            b"a\nb\nc\n\n\n"
        );
        assert_eq!(canonical_text(b"a\xef\xbb\xbf"), b"a\xef\xbb\xbf");
    }

    #[test]
    fn sequence_line_forms() {
        assert_eq!(parse_sequence_line("# moat-sequence: 0"), Some(0));
        assert_eq!(parse_sequence_line("# moat-sequence: 17"), Some(17));
        assert_eq!(
            parse_sequence_line("# moat-sequence: 9007199254740991"),
            Some(MAX_SEQUENCE)
        );
        for bad in [
            "# moat-sequence: 9007199254740992",
            "# moat-sequence: 99999999999999999999",
            "# moat-sequence: 01",
            "# moat-sequence: +1",
            "# moat-sequence: ",
            "# moat-sequence: 1 ",
            "#moat-sequence: 1",
            "# moat-sequence:  1",
            " # moat-sequence: 1",
            "# moat-sequence: \u{661}",
        ] {
            assert_eq!(parse_sequence_line(bad), None, "{bad:?}");
        }
        assert!(is_sequence_line_start("#moat-sequence"));
        assert!(is_sequence_line_start("#\u{a0}\tmoat-sequence: 1"));
        assert!(is_sequence_line_start("# moat-sequences are fun"));
        assert!(!is_sequence_line_start("# the moat-sequence"));
    }
}
