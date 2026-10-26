//! SSH wire reading, base64, and the Python text rules the MOAT reference
//! implementation (`reference/moat_author_key.py`) applies. Each function
//! here reproduces the reference's accept and reject behavior, so a list or
//! signature the reference refuses is refused here and the reverse.

/// Reads SSH wire data (RFC 4251 §5). Every read fails on truncation.
pub(crate) struct Reader<'a> {
    data: &'a [u8],
}

impl<'a> Reader<'a> {
    pub(crate) fn new(data: &'a [u8]) -> Reader<'a> {
        Reader { data }
    }

    pub(crate) fn take(&mut self, n: usize) -> Option<&'a [u8]> {
        if n > self.data.len() {
            return None;
        }
        let (head, tail) = self.data.split_at(n);
        self.data = tail;
        Some(head)
    }

    pub(crate) fn u32(&mut self) -> Option<u32> {
        let bytes = self.take(4)?;
        Some(u32::from_be_bytes(bytes.try_into().ok()?))
    }

    pub(crate) fn string(&mut self) -> Option<&'a [u8]> {
        let len = usize::try_from(self.u32()?).ok()?;
        self.take(len)
    }

    pub(crate) fn done(&self) -> bool {
        self.data.is_empty()
    }
}

/// Appends `bytes` as an SSH `string`.
pub(crate) fn put_string(out: &mut Vec<u8>, bytes: &[u8]) {
    let len = u32::try_from(bytes.len()).expect("SSH string longer than 4 GiB");
    out.extend_from_slice(&len.to_be_bytes());
    out.extend_from_slice(bytes);
}

const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

/// Standard base64 without padding, the form an OpenSSH fingerprint uses.
pub(crate) fn base64_unpadded(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len().div_ceil(3) * 4);
    for chunk in bytes.chunks(3) {
        let b = [
            chunk[0],
            chunk.get(1).copied().unwrap_or(0),
            chunk.get(2).copied().unwrap_or(0),
        ];
        let n = u32::from(b[0]) << 16 | u32::from(b[1]) << 8 | u32::from(b[2]);
        for i in 0..=chunk.len() {
            out.push(char::from(ALPHABET[(n >> (18 - 6 * i) & 63) as usize]));
        }
    }
    out
}

/// Decodes standard base64 exactly as `base64.b64decode(s, validate=True)`
/// does in Python 3.11 or later: the length is a multiple of four, only the
/// alphabet appears before at most two final `=`, and a final group of `X===`
/// is refused. Like Python, it ignores the unused low bits of the last
/// character, so `QR==` decodes as `QQ==` does.
pub(crate) fn base64_decode(text: &str) -> Option<Vec<u8>> {
    let bytes = text.as_bytes();
    if !bytes.len().is_multiple_of(4) {
        return None;
    }
    let pad = bytes.iter().rev().take_while(|&&b| b == b'=').count();
    if pad > 2 {
        return None;
    }
    let data = &bytes[..bytes.len() - pad];
    let mut out = Vec::with_capacity(data.len() * 3 / 4);
    let mut acc = 0u32;
    let mut bits = 0;
    for &c in data {
        let value = ALPHABET.iter().position(|&a| a == c)?;
        acc = acc << 6 | value as u32;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((acc >> bits) as u8);
            acc &= (1 << bits) - 1;
        }
    }
    Some(out)
}

/// Python's `str.isspace()`, which is also what `\s` matches in a `str`
/// regular expression.
pub(crate) fn py_isspace(c: char) -> bool {
    matches!(
        c,
        '\t' | '\n'
            | '\u{0b}'
            | '\u{0c}'
            | '\r'
            | '\u{1c}'..='\u{1f}'
            | ' '
            | '\u{85}'
            | '\u{a0}'
            | '\u{1680}'
            | '\u{2000}'..='\u{200a}'
            | '\u{2028}'
            | '\u{2029}'
            | '\u{202f}'
            | '\u{205f}'
            | '\u{3000}'
    )
}

/// Python's `str.strip()` with no argument.
pub(crate) fn py_strip(text: &str) -> &str {
    text.trim_matches(py_isspace)
}

/// Python's `str.splitlines()`: splits at every line boundary Python
/// recognizes, treats CRLF as one boundary, and yields no empty last line
/// for a trailing boundary.
pub(crate) fn py_splitlines(text: &str) -> Vec<&str> {
    let mut lines = Vec::new();
    let mut start = 0;
    let mut chars = text.char_indices().peekable();
    while let Some((i, c)) = chars.next() {
        let boundary = matches!(
            c,
            '\n' | '\r'
                | '\u{0b}'
                | '\u{0c}'
                | '\u{1c}'
                | '\u{1d}'
                | '\u{1e}'
                | '\u{85}'
                | '\u{2028}'
                | '\u{2029}'
        );
        if boundary {
            lines.push(&text[start..i]);
            let mut end = i + c.len_utf8();
            if c == '\r' && chars.peek().is_some_and(|&(_, next)| next == '\n') {
                chars.next();
                end += 1;
            }
            start = end;
        }
    }
    if start < text.len() {
        lines.push(&text[start..]);
    }
    lines
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn base64_matches_python_validate() {
        // Each case's expected result is what Python 3.13's
        // base64.b64decode(s, validate=True) returns.
        let cases: &[(&str, Option<&[u8]>)] = &[
            ("", Some(b"")),
            ("QQ==", Some(b"A")),
            ("QR==", Some(b"A")),
            ("QUI=", Some(b"AB")),
            ("QUJ=", Some(b"AB")),
            ("QUJD", Some(b"ABC")),
            ("QQ", None),
            ("QQ=", None),
            ("QQ===", None),
            ("Q===", None),
            ("QU JD", None),
            ("=QUJD", None),
            ("QUJD=", None),
            ("QQ=A", None),
            ("Q=Q=", None),
            ("QQ==QQ==", None),
            ("QQ==\n", None),
            ("QUJ-", None),
        ];
        for (text, want) in cases {
            assert_eq!(base64_decode(text).as_deref(), *want, "{text:?}");
        }
    }

    #[test]
    fn base64_round_trips() {
        for len in 0..10 {
            let bytes: Vec<u8> = (0..len).map(|i| (i * 37 + 11) as u8).collect();
            let mut text = base64_unpadded(&bytes);
            while !text.len().is_multiple_of(4) {
                text.push('=');
            }
            assert_eq!(base64_decode(&text).unwrap(), bytes);
        }
    }

    #[test]
    fn splitlines_matches_python() {
        assert_eq!(py_splitlines(""), Vec::<&str>::new());
        assert_eq!(
            py_splitlines("a\r\nb\rc\nd\u{2028}e\n"),
            ["a", "b", "c", "d", "e"]
        );
        assert_eq!(py_splitlines("a\n\nb"), ["a", "", "b"]);
        assert_eq!(py_strip("\u{a0} x \u{3000}\n"), "x");
    }
}
