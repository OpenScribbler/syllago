//! Contributions to a markdown shared file (install-targets §10).
//!
//! Each contribution is a contiguous block between two marker lines that
//! carry its key and a nonce, so re-applying an item replaces only its own
//! block and removing it takes out exactly that block, whatever another
//! item wrote:
//!
//! ```text
//! <!-- syllago:begin style 3f2a9c0d81b7e645 -->
//! Use tabs.
//! <!-- syllago:end style 3f2a9c0d81b7e645 -->
//! ```
//!
//! The nonce is the start of the SHA-256 of the contribution, so no line
//! of the contribution can be its own closing marker. The key is the
//! content name with `%`, `>`, whitespace, control characters and the
//! second of two hyphens percent-encoded, so any content name fits in an
//! HTML comment. An HTML comment renders as nothing in every markdown
//! reader the providers use. A block is written in the file's dominant
//! line ending, and its bytes are compared with CRLF read as LF, so an
//! editor normalizing line endings is not an edit. Everything outside the
//! blocks is left byte for byte.

use std::ops::Range;

use crate::record::sha256;

/// One block: the whole of it, markers included, and the bytes between the
/// markers.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Block {
    pub whole: Range<usize>,
    pub inner: Range<usize>,
}

/// What a file holds for one key.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Scan {
    /// Complete blocks, in file order.
    pub blocks: Vec<Block>,
    /// A marker line for the key that closes or opens no block: a block
    /// whose other marker was edited or deleted.
    pub damaged: bool,
}

const BEGIN: &str = "<!-- syllago:begin ";
const END: &str = "<!-- syllago:end ";
const CLOSE: &str = " -->";

/// `key` as it appears in a marker.
pub(crate) fn encode_key(key: &str) -> String {
    let mut out = String::new();
    let mut previous = None;
    for c in key.chars() {
        let escape = matches!(c, '%' | '>')
            || c.is_whitespace()
            || c.is_control()
            || (c == '-' && previous == Some('-'));
        if escape {
            let mut buf = [0; 4];
            for b in c.encode_utf8(&mut buf).bytes() {
                out.push_str(&format!("%{b:02X}"));
            }
        } else {
            out.push(c);
        }
        previous = Some(c);
    }
    out
}

/// `bytes` with each CRLF read as LF.
pub(crate) fn fold(bytes: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(bytes.len());
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'\r' && bytes.get(i + 1) == Some(&b'\n') {
            i += 1;
            continue;
        }
        out.push(bytes[i]);
        i += 1;
    }
    out
}

/// The file's dominant line ending: CRLF when more of its lines end in
/// CRLF than in a bare LF.
pub(crate) fn eol(text: &[u8]) -> &'static str {
    let lines = text.iter().filter(|&&b| b == b'\n').count();
    let crlf = text.windows(2).filter(|w| w == b"\r\n").count();
    if crlf * 2 > lines { "\r\n" } else { "\n" }
}

/// The contribution as it goes between the markers: LF line endings, and
/// ending in LF so the closing marker starts its own line.
pub(crate) fn inner_bytes(contribution: &[u8]) -> Vec<u8> {
    let mut inner = fold(contribution);
    if !inner.is_empty() && !inner.ends_with(b"\n") {
        inner.push(b'\n');
    }
    inner
}

/// The marker line content `prefix key nonce -->`, split into key and
/// nonce.
fn marker<'t>(line: &'t [u8], prefix: &str) -> Option<(&'t [u8], &'t [u8])> {
    let middle = line
        .strip_prefix(prefix.as_bytes())?
        .strip_suffix(CLOSE.as_bytes())?;
    let space = middle.iter().position(|&b| b == b' ')?;
    let (key, nonce) = (&middle[..space], &middle[space + 1..]);
    (!nonce.is_empty() && !nonce.contains(&b' ')).then_some((key, nonce))
}

fn content(line: &[u8]) -> &[u8] {
    let line = line.strip_suffix(b"\n").unwrap_or(line);
    line.strip_suffix(b"\r").unwrap_or(line)
}

/// The blocks for `key`. A closing marker pairs with the latest opening
/// marker carrying the same nonce; marker lines inside a block are its
/// content, and a block inside another is part of it.
pub(crate) fn scan(text: &[u8], key: &str) -> Scan {
    let key = encode_key(key);
    let mut blocks: Vec<Block> = Vec::new();
    let mut open: Vec<(&[u8], usize, usize)> = Vec::new();
    // Marker lines that open or close no block, by where they start.
    let mut stray = Vec::new();
    let mut position = 0;
    for line in text.split_inclusive(|&b| b == b'\n') {
        let start = position;
        position += line.len();
        let content = content(line);
        if let Some((k, nonce)) = marker(content, BEGIN)
            && k == key.as_bytes()
            && line.ends_with(b"\n")
        {
            open.push((nonce, start, position));
        } else if let Some((k, nonce)) = marker(content, END)
            && k == key.as_bytes()
        {
            match open.iter().rposition(|(n, _, _)| *n == nonce) {
                Some(index) => {
                    let (_, whole, inner) = open[index];
                    open.truncate(index);
                    blocks.push(Block {
                        whole: whole..position,
                        inner: inner..start,
                    });
                }
                None => stray.push(start),
            }
        }
    }
    stray.extend(open.iter().map(|(_, start, _)| *start));
    // A block within another is that block's content.
    let all = blocks.clone();
    blocks.retain(|b| {
        !all.iter()
            .any(|o| o != b && o.whole.start <= b.whole.start && b.whole.end <= o.whole.end)
    });
    blocks.sort_by_key(|b| b.whole.start);
    // One inside a block is that block's content, not damage.
    let damaged = stray
        .iter()
        .any(|&at| !blocks.iter().any(|b| b.whole.contains(&at)));
    Scan { blocks, damaged }
}

/// The block for `key` holding `inner` (LF), in line ending `eol`.
fn render(key: &str, inner: &[u8], eol: &str) -> Vec<u8> {
    let key = encode_key(key);
    let nonce = &sha256(inner)[..16];
    let lf = [
        format!("{BEGIN}{key} {nonce}{CLOSE}\n").as_bytes(),
        inner,
        format!("{END}{key} {nonce}{CLOSE}\n").as_bytes(),
    ]
    .concat();
    if eol == "\n" {
        return lf;
    }
    let mut out = Vec::with_capacity(lf.len() + lf.len() / 16);
    for b in lf {
        if b == b'\n' {
            out.push(b'\r');
        }
        out.push(b);
    }
    out
}

/// The separator a block appended to `text` gets: a blank line, unless the
/// file is empty or already ends in one.
fn separator_for(text: &[u8], eol: &str) -> String {
    if text.is_empty() || text.ends_with(b"\n\n") || text.ends_with(b"\n\r\n") {
        String::new()
    } else if text.ends_with(b"\n") {
        eol.to_string()
    } else {
        eol.repeat(2)
    }
}

/// `text` with `block` replaced by a block for `key` holding `inner`.
pub(crate) fn replace(text: &[u8], block: &Block, key: &str, inner: &[u8]) -> Vec<u8> {
    let block_bytes = render(key, inner, eol(text));
    [
        &text[..block.whole.start],
        &block_bytes,
        &text[block.whole.end..],
    ]
    .concat()
}

/// `text` with a block for `key` holding `inner` appended, and the
/// separator put before it.
pub(crate) fn append(text: &[u8], key: &str, inner: &[u8]) -> (Vec<u8>, String) {
    let eol = eol(text);
    let separator = separator_for(text, eol);
    let out = [text, separator.as_bytes(), &render(key, inner, eol)].concat();
    (out, separator)
}

/// What removing a block did to the block right after it, when there is
/// one: its encoded key and the separator it has now.
pub(crate) type Handover = Option<(String, String)>;

/// `text` without `block`. Its `separator` goes too, but only when those
/// bytes are still the ones it was given: what precedes them would get the
/// same separator now, so a line the user wrote beside the block is never
/// joined to another. A block of ours right after it, set off by one line
/// ending, gets the separator its new predecessor calls for, so removing
/// every block in any order gives back the file's bytes.
pub(crate) fn remove(text: &[u8], block: &Block, separator: &str) -> (Vec<u8>, Handover) {
    let eol = eol(text);
    let mut start = block.whole.start;
    let before = &text[..start];
    if !separator.is_empty()
        && before.ends_with(separator.as_bytes())
        && separator_for(&before[..before.len() - separator.len()], eol) == separator
    {
        start -= separator.len();
    }
    let rest = &text[block.whole.end..];
    if let Some(next) = rest.strip_prefix(eol.as_bytes())
        && !next.starts_with(b"\n")
        && !next.starts_with(b"\r\n")
    {
        let line = next.split_inclusive(|&b| b == b'\n').next().unwrap_or(next);
        if let Some((key, _)) = marker(content(line), BEGIN) {
            let separator = separator_for(&text[..start], eol);
            let out = [&text[..start], separator.as_bytes(), next].concat();
            let key = String::from_utf8_lossy(key).into_owned();
            return (out, Some((key, separator)));
        }
    }
    ([&text[..start], rest].concat(), None)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Removes `key`'s block from `text`, keeping `separators` up to date.
    fn take(text: &[u8], key: &str, separators: &mut Vec<(String, String)>) -> Vec<u8> {
        let scan = scan(text, key);
        assert_eq!(scan.blocks.len(), 1, "{key}");
        let index = separators.iter().position(|(k, _)| k == key).unwrap();
        let (_, separator) = separators.remove(index);
        let (out, handover) = remove(text, &scan.blocks[0], &separator);
        if let Some((next, separator)) = handover {
            let entry = separators
                .iter_mut()
                .find(|(k, _)| encode_key(k) == next)
                .unwrap();
            entry.1 = separator;
        }
        out
    }

    #[test]
    fn append_then_remove_in_any_order_restores_the_bytes() {
        for before in [&b""[..], b"mine", b"mine\n", b"mine\n\n", b"a\r\nb\r\n"] {
            for order in [
                ["a", "b", "c"],
                ["b", "a", "c"],
                ["c", "b", "a"],
                ["a", "c", "b"],
            ] {
                let mut text = before.to_vec();
                let mut separators = Vec::new();
                for key in ["a", "b", "c"] {
                    let (out, separator) = append(&text, key, b"A\n");
                    text = out;
                    separators.push((key.to_string(), separator));
                }
                for key in order {
                    text = take(&text, key, &mut separators);
                }
                assert_eq!(text, before, "{order:?}");
            }
        }
    }

    // A line the user wrote right before or after a block keeps its line
    // ending when the block goes.
    #[test]
    fn removal_never_joins_the_users_lines() {
        let (text, separator) = append(b"# Mine\n", "style", b"S\n");
        let block = &scan(&text, "style").blocks[0];
        let edited = [
            &text[..block.whole.start],
            b"Before\n",
            &text[block.whole.clone()],
            b"After\n",
        ]
        .concat();
        let block = &scan(&edited, "style").blocks[0];
        let (out, _) = remove(&edited, block, &separator);
        assert_eq!(out, b"# Mine\n\nBefore\nAfter\n");
    }

    // A contribution that quotes a closing marker, even its own key's, is
    // still one block.
    #[test]
    fn a_contribution_cannot_close_its_own_block() {
        let inner = b"Example:\n<!-- syllago:end style -->\n<!-- syllago:end style 0000 -->\n\
                      <!-- syllago:begin style 1111 -->\n";
        let (text, _) = append(b"", "style", inner);
        let scan = scan(&text, "style");
        assert!(!scan.damaged);
        assert_eq!(scan.blocks.len(), 1);
        assert_eq!(&text[scan.blocks[0].inner.clone()], inner);
    }

    #[test]
    fn any_content_name_is_a_key() {
        for key in ["style--strict", "a>b", "50% off", "a-b"] {
            let (text, _) = append(b"x\n", key, b"K\n");
            assert_eq!(scan(&text, key).blocks.len(), 1, "{key}");
            // Only the comment's own "<!--" and "-->" hold two hyphens.
            let markers = String::from_utf8(text).unwrap();
            assert!(
                !markers
                    .replace("<!--", "")
                    .replace("-->", "")
                    .contains("--")
            );
        }
        assert_eq!(encode_key("style--strict"), "style-%2Dstrict");
        assert_eq!(encode_key("a>b c"), "a%3Eb%20c");
    }

    #[test]
    fn crlf_files_get_crlf_blocks_and_fold_for_comparison() {
        let (text, separator) = append(b"a\r\nb\r\n", "s", b"x\ny\n");
        assert_eq!(separator, "\r\n");
        let end = format!(
            "x\r\ny\r\n<!-- syllago:end s {} -->\r\n",
            &sha256(b"x\ny\n")[..16]
        );
        assert!(text.ends_with(end.as_bytes()));
        let block = &scan(&text, "s").blocks[0];
        assert_eq!(fold(&text[block.inner.clone()]), b"x\ny\n");
    }

    #[test]
    fn damaged_markers_are_reported() {
        let (text, _) = append(b"", "s", b"x\n");
        let text = String::from_utf8(text).unwrap();
        let no_end = text.replace("syllago:end", "syllago:ended");
        let scan_no_end = scan(no_end.as_bytes(), "s");
        assert!(scan_no_end.blocks.is_empty() && scan_no_end.damaged);
        let no_begin = text.replace("syllago:begin", "syllago:began");
        assert!(scan(no_begin.as_bytes(), "s").damaged);
        // A block of another key, or none at all, is not damage.
        assert_eq!(
            scan(text.as_bytes(), "t"),
            Scan {
                blocks: vec![],
                damaged: false
            }
        );
    }
}
