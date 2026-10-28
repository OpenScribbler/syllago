//! Contributions to a markdown shared file (install-targets §10).
//!
//! Each contribution is a contiguous block between two marker lines that
//! carry its key, so re-applying an item replaces only its own block and
//! removing it takes out exactly that block, whatever another item wrote:
//!
//! ```text
//! <!-- syllago:begin style -->
//! Use tabs.
//! <!-- syllago:end style -->
//! ```
//!
//! An HTML comment renders as nothing in every markdown reader the
//! providers use. Everything outside the blocks is left byte for byte.

use std::ops::Range;

/// One block: the whole of it, markers included, and the bytes between the
/// markers.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Block {
    pub whole: Range<usize>,
    pub inner: Range<usize>,
}

fn begin(key: &str) -> String {
    format!("<!-- syllago:begin {key} -->")
}

fn end(key: &str) -> String {
    format!("<!-- syllago:end {key} -->")
}

/// Whether `key` can sit inside a marker line and be read back unchanged.
pub(crate) fn valid_key(key: &str) -> bool {
    !key.is_empty() && !key.contains("--") && !key.contains('>') && !key.contains(char::is_control)
}

/// The blocks for `key`, in file order. A marker line matches with LF or
/// CRLF, so a file an editor converted to CRLF still shows where the block
/// is; its bytes then differ, which reads as an edit.
pub(crate) fn find(text: &[u8], key: &str) -> Vec<Block> {
    let (begin, end) = (begin(key), end(key));
    let mut blocks = Vec::new();
    let mut open = None;
    let mut position = 0;
    for line in text.split_inclusive(|&b| b == b'\n') {
        let start = position;
        position += line.len();
        let content = line.strip_suffix(b"\n").unwrap_or(line);
        let content = content.strip_suffix(b"\r").unwrap_or(content);
        if content == begin.as_bytes() && line.ends_with(b"\n") {
            open = Some((start, position));
        } else if content == end.as_bytes()
            && let Some((whole, inner)) = open.take()
        {
            blocks.push(Block {
                whole: whole..position,
                inner: inner..start,
            });
        }
    }
    blocks
}

/// The bytes placed between the markers: the contribution, ending in LF so
/// the closing marker starts its own line.
pub(crate) fn inner_bytes(contribution: &[u8]) -> Vec<u8> {
    let mut inner = contribution.to_vec();
    if !inner.is_empty() && !inner.ends_with(b"\n") {
        inner.push(b'\n');
    }
    inner
}

/// `text` with `block`'s inner bytes replaced.
pub(crate) fn replace(text: &[u8], block: &Block, inner: &[u8]) -> Vec<u8> {
    [&text[..block.inner.start], inner, &text[block.inner.end..]].concat()
}

/// `text` with a new block for `key` appended, and the separator put before
/// it: a blank line, unless the file is empty or already ends in one.
pub(crate) fn append(text: &[u8], key: &str, inner: &[u8]) -> (Vec<u8>, String) {
    let separator = if text.is_empty() || text.ends_with(b"\n\n") {
        ""
    } else if text.ends_with(b"\n") {
        "\n"
    } else {
        "\n\n"
    };
    let mut out = text.to_vec();
    out.extend_from_slice(separator.as_bytes());
    out.extend_from_slice(begin(key).as_bytes());
    out.push(b'\n');
    out.extend_from_slice(inner);
    out.extend_from_slice(end(key).as_bytes());
    out.push(b'\n');
    (out, separator.to_string())
}

/// `text` without `block`, and without `separator` when it still sits
/// right before the block, so a file holding nothing else returns to its
/// bytes before the block went in.
pub(crate) fn remove(text: &[u8], block: &Block, separator: &str) -> Vec<u8> {
    let mut start = block.whole.start;
    if text[..start].ends_with(separator.as_bytes()) {
        start -= separator.len();
    }
    [&text[..start], &text[block.whole.end..]].concat()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn append_then_remove_restores_the_bytes() {
        for before in [&b""[..], b"mine", b"mine\n", b"mine\n\n", b"a\r\nb\r\n"] {
            let (one, sep_a) = append(before, "a", b"A\n");
            let (two, sep_b) = append(&one, "b", b"A\n");
            // Either removal order gives back what was there.
            let block = &find(&two, "a")[0];
            let without_a = remove(&two, block, &sep_a);
            let block = &find(&without_a, "b")[0];
            assert_eq!(remove(&without_a, block, &sep_b), before);
            let block = &find(&two, "b")[0];
            let without_b = remove(&two, block, &sep_b);
            assert_eq!(without_b, one);
            let block = &find(&without_b, "a")[0];
            assert_eq!(remove(&without_b, block, &sep_a), before);
        }
    }

    #[test]
    fn find_reads_markers_by_key_and_line() {
        let (text, _) = append(b"x\n", "style", b"Use tabs.\n");
        let blocks = find(&text, "style");
        assert_eq!(blocks.len(), 1);
        assert_eq!(&text[blocks[0].inner.clone()], b"Use tabs.\n");
        assert!(find(&text, "styl").is_empty());
        // A marker mentioned inside a line is not a marker.
        assert!(
            find(
                b"see <!-- syllago:begin s -->\n<!-- syllago:end s -->\n",
                "s"
            )
            .is_empty()
        );
        // An unclosed block is not found; CRLF markers are.
        assert!(find(b"<!-- syllago:begin s -->\nx\n", "s").is_empty());
        let crlf = b"<!-- syllago:begin s -->\r\nx\r\n<!-- syllago:end s -->\r\n";
        assert_eq!(&crlf[find(crlf, "s")[0].inner.clone()], b"x\r\n");
    }

    #[test]
    fn keys_that_would_break_a_marker_are_invalid() {
        assert!(valid_key("style"));
        for key in ["", "a--b", "a>b", "a\nb"] {
            assert!(!valid_key(key), "{key:?}");
        }
    }
}
