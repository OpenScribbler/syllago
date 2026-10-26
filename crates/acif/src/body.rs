use std::collections::HashSet;
use std::fs;
use std::path::Path;

use unicode_normalization::UnicodeNormalization;

use crate::{Error, Hash, malformed};

/// Version-control metadata names excluded from the body everywhere (core §7.4).
const VCS_NAMES: [&str; 6] = [".git", ".svn", ".hg", ".bzr", "_darcs", ".fossil"];

/// The registry-generated sidecar, excluded at the body root only (core §7.5).
const ROOT_SIDECAR: &str = "acif-sidecar.yaml";

/// Final extensions of text files (core §7.3).
const TEXT_EXTENSIONS: [&str; 38] = [
    ".md", ".txt", ".rst", ".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf", ".html",
    ".htm", ".xml", ".svg", ".css", ".scss", ".less", ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs",
    ".py", ".rb", ".lua", ".rs", ".go", ".sh", ".bash", ".zsh", ".fish", ".csv", ".tsv", ".sql",
    ".lock", ".sum", ".mod",
];

/// Whether a body is its entry file alone or carries co-located content
/// files (core §7.2).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Classification {
    SingleFile,
    MultiFile,
}

impl Classification {
    /// The spec spelling: `single-file` or `multi-file`.
    pub fn as_str(self) -> &'static str {
        match self {
            Classification::SingleFile => "single-file",
            Classification::MultiFile => "multi-file",
        }
    }
}

/// The regular files under an item's body root, each with its relative path
/// in POSIX form. Version-control metadata is left out.
#[derive(Debug, Clone)]
pub struct Body {
    files: Vec<(String, Vec<u8>)>,
}

impl Body {
    /// Reads the body under `root`. A symlink anywhere beneath `root` is
    /// rejected with `acif.body.symlink` and never followed. `root` itself is
    /// resolved as given, so a symlinked item directory reads its target.
    pub fn read(root: &Path) -> Result<Body, Error> {
        let mut files = Vec::new();
        read_dir(root, "", &mut files)?;
        Ok(Body { files })
    }

    #[cfg(test)]
    pub(crate) fn from_files(files: &[(&str, &[u8])]) -> Body {
        let files = files
            .iter()
            .map(|(path, bytes)| (path.to_string(), bytes.to_vec()))
            .collect();
        Body { files }
    }
}

fn read_dir(dir: &Path, prefix: &str, files: &mut Vec<(String, Vec<u8>)>) -> Result<(), Error> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let name = entry
            .file_name()
            .into_string()
            .map_err(|name| malformed(format!("file name {name:?} is not UTF-8")))?;
        if VCS_NAMES.contains(&name.as_str()) {
            continue;
        }
        let path = format!("{prefix}{name}");
        let file_type = entry.file_type()?;
        if file_type.is_symlink() {
            return Err(Error::Reject("acif.body.symlink"));
        }
        if file_type.is_dir() {
            read_dir(&entry.path(), &format!("{path}/"), files)?;
        } else if file_type.is_file() {
            files.push((path, fs::read(entry.path())?));
        }
    }
    Ok(())
}

/// A hashed body: its `body_hash`, its classification, and the canonical
/// text of its entry file, frontmatter included.
pub(crate) struct HashedBody {
    pub hash: Hash,
    pub classification: Classification,
    pub entry_text: Vec<u8>,
}

/// Classifies and hashes `body`, whose entry file is `entry_file` (core §7).
pub(crate) fn hash_body(body: &Body, entry_file: &str) -> Result<HashedBody, Error> {
    let mut seen = HashSet::new();
    let mut files: Vec<(String, &[u8])> = Vec::new();
    for (path, bytes) in &body.files {
        let path: String = path.nfc().collect();
        if !seen.insert(path.clone()) {
            return Err(Error::Reject("acif.body.path_collision"));
        }
        if path != ROOT_SIDECAR {
            files.push((path, bytes));
        }
    }
    if files
        .iter()
        .all(|(path, _)| excluded_from_classification(path))
    {
        return Err(Error::Reject("acif.body.empty"));
    }
    let entry: String = entry_file.nfc().collect();
    let Some((_, entry_bytes)) = files.iter().find(|(path, _)| *path == entry) else {
        return Err(malformed(format!(
            "entry file {entry_file:?} is not in the body"
        )));
    };
    let entry_text = canonical_text(entry_bytes);
    let entry_input = split_frontmatter(&entry_text).1;
    let multi_file = files
        .iter()
        .any(|(path, _)| *path != entry && !excluded_from_classification(path));
    if !multi_file {
        return Ok(HashedBody {
            hash: Hash::sha256(entry_input),
            classification: Classification::SingleFile,
            entry_text: entry_text.clone(),
        });
    }
    files.sort_by(|a, b| a.0.cmp(&b.0));
    let mut manifest = String::new();
    for (path, bytes) in &files {
        let hash = if *path == entry {
            Hash::sha256(entry_input)
        } else if is_text(path, bytes) {
            Hash::sha256(&canonical_text(bytes))
        } else {
            Hash::sha256(bytes)
        };
        manifest.push_str(&format!("{}  {path}\n", hash.value));
    }
    Ok(HashedBody {
        hash: Hash::sha256(manifest.as_bytes()),
        classification: Classification::MultiFile,
        entry_text,
    })
}

/// `LICENSE*` and `README*` regular files at the body root, matched as a
/// case-insensitive prefix (core §7.2).
fn excluded_from_classification(path: &str) -> bool {
    let prefixed = |prefix: &str| {
        path.as_bytes()
            .get(..prefix.len())
            .is_some_and(|start| start.eq_ignore_ascii_case(prefix.as_bytes()))
    };
    !path.contains('/') && (prefixed("license") || prefixed("readme"))
}

fn is_text(path: &str, bytes: &[u8]) -> bool {
    let name = path.rsplit('/').next().unwrap_or(path);
    let extension = match name.rfind('.') {
        None | Some(0) => String::new(),
        Some(dot) => name[dot..].to_lowercase(),
    };
    TEXT_EXTENSIONS.contains(&extension.as_str()) && !bytes[..bytes.len().min(8192)].contains(&0)
}

/// The canonical text form (core §7.3): a leading UTF-8 BOM removed, then
/// CRLF and lone CR turned into LF in one pass.
pub(crate) fn canonical_text(bytes: &[u8]) -> Vec<u8> {
    let bytes = bytes.strip_prefix(b"\xEF\xBB\xBF").unwrap_or(bytes);
    let mut text = Vec::with_capacity(bytes.len());
    let mut iter = bytes.iter().peekable();
    while let Some(&byte) = iter.next() {
        if byte == b'\r' {
            iter.next_if_eq(&&b'\n');
            text.push(b'\n');
        } else {
            text.push(byte);
        }
    }
    text
}

/// Splits canonical text into its frontmatter block, if any, and the rest
/// (core §7.3). The block opens with a first line of `---` and closes at the
/// next line of `---`, which ends with LF or at end of input.
pub(crate) fn split_frontmatter(text: &[u8]) -> (Option<&[u8]>, &[u8]) {
    if !text.starts_with(b"---\n") {
        return (None, text);
    }
    let block = |close: usize| {
        if close > 4 {
            &text[4..close]
        } else {
            &text[..0]
        }
    };
    if let Some(offset) = text[3..].windows(5).position(|window| window == b"\n---\n") {
        let close = offset + 3;
        return (Some(block(close)), &text[close + 5..]);
    }
    if text.len() >= 7 && text.ends_with(b"\n---") {
        return (Some(block(text.len() - 4)), &text[text.len()..]);
    }
    (None, text)
}

#[cfg(test)]
mod tests {
    use super::*;

    // No conformance vector feeds CRLF, lone CR or a BOM, because the runner
    // writes fixtures with LF. These prove a Windows checkout of a body
    // hashes the same as its LF form.
    #[test]
    fn line_endings_and_bom_do_not_move_body_hash() {
        let lf = Body::from_files(&[("SKILL.md", b"---\nd: x\n---\none\ntwo\n")]);
        let crlf = Body::from_files(&[(
            "SKILL.md",
            b"\xEF\xBB\xBF---\r\nd: x\r\n---\r\none\rtwo\r\n",
        )]);
        assert_eq!(
            hash_body(&lf, "SKILL.md").unwrap().hash,
            hash_body(&crlf, "SKILL.md").unwrap().hash
        );
    }

    #[test]
    fn manifest_order_ignores_read_order() {
        let a = Body::from_files(&[("SKILL.md", b"x\n"), ("b.sh", b"b\n"), ("a/c.sh", b"c\n")]);
        let b = Body::from_files(&[("a/c.sh", b"c\n"), ("b.sh", b"b\r\n"), ("SKILL.md", b"x\n")]);
        assert_eq!(
            hash_body(&a, "SKILL.md").unwrap().hash,
            hash_body(&b, "SKILL.md").unwrap().hash
        );
    }

    #[test]
    fn frontmatter_split() {
        assert_eq!(
            split_frontmatter(b"---\n---\nbody"),
            (Some(&b""[..]), &b"body"[..])
        );
        assert_eq!(
            split_frontmatter(b"---\na: 1\n---"),
            (Some(&b"a: 1"[..]), &b""[..])
        );
        assert_eq!(
            split_frontmatter(b"---\na: 1\n"),
            (None, &b"---\na: 1\n"[..])
        );
    }
}
