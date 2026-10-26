use std::collections::HashSet;
use std::fs;
use std::path::Path;

use unicode_normalization::UnicodeNormalization;

use crate::{Error, Hash};

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
    /// Paths that are not valid UTF-8 (UTF-16 on Windows), as raw bytes
    /// (`OsStr::as_encoded_bytes`). Only classification, the §7.4 path scan
    /// and collision detection read them.
    not_utf8: Vec<Vec<u8>>,
}

impl Body {
    /// Reads the body under `root`. A symlink anywhere beneath `root` is
    /// rejected with `acif.body.symlink` and never followed. `root` itself is
    /// resolved as given, so a symlinked item directory reads its target.
    pub fn read(root: &Path) -> Result<Body, Error> {
        let mut body = Body {
            files: Vec::new(),
            not_utf8: Vec::new(),
        };
        read_dir(root, b"", &mut body)?;
        Ok(body)
    }

    #[cfg(test)]
    pub(crate) fn from_files(files: &[(&str, &[u8])]) -> Body {
        let files = files
            .iter()
            .map(|(path, bytes)| (path.to_string(), bytes.to_vec()))
            .collect();
        Body {
            files,
            not_utf8: Vec::new(),
        }
    }
}

fn read_dir(dir: &Path, prefix: &[u8], body: &mut Body) -> Result<(), Error> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let name = entry.file_name();
        if name.to_str().is_some_and(|name| VCS_NAMES.contains(&name)) {
            continue;
        }
        let path = [prefix, name.as_encoded_bytes()].concat();
        let file_type = entry.file_type()?;
        if file_type.is_symlink() {
            return Err(Error::Reject("acif.body.symlink"));
        }
        if file_type.is_dir() {
            read_dir(&entry.path(), &[&path[..], b"/"].concat(), body)?;
        } else if file_type.is_file() {
            match String::from_utf8(path) {
                Ok(path) => body.files.push((path, fs::read(entry.path())?)),
                Err(error) => body.not_utf8.push(error.into_bytes()),
            }
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
    // Classification compares raw paths, before NFC, as §7.2 names them,
    // and excludes the entry file at the body root only.
    let multi_file = body.files.iter().any(|(path, _)| {
        path != ROOT_SIDECAR
            && !excluded_from_classification(path)
            && !(path == entry_file && !path.contains('/'))
    }) || body
        .not_utf8
        .iter()
        // A path that is not UTF-8 is never the entry file, though its lossy
        // spelling can match one.
        .any(|path| !excluded_from_classification(&String::from_utf8_lossy(path)));
    // The §7.4 manifest, and so its step 3 path scans, runs only for a
    // multi-file body (§7.2). Each scan covers every path before the next.
    if multi_file {
        if !body.not_utf8.is_empty() {
            return Err(Error::Reject("acif.body.path_not_utf8"));
        }
        // U+0000–U+001F and U+007F; U+0080–U+009F are allowed.
        if body
            .files
            .iter()
            .any(|(path, _)| path.chars().any(|c| c.is_ascii_control()))
        {
            return Err(Error::Reject("acif.body.path_control_char"));
        }
    }
    // Collision detection covers every path, a single-file body's and a path
    // that is not UTF-8 included.
    let mut seen = HashSet::new();
    let raw_paths = body.files.iter().map(|(path, _)| path.as_bytes());
    for path in raw_paths.chain(body.not_utf8.iter().map(Vec::as_slice)) {
        if !seen.insert(collision_key(path)) {
            return Err(Error::Reject("acif.body.path_collision"));
        }
    }
    // The empty check and the entry lookup read raw paths too: NFC can move
    // a name out of a root README* or LICENSE* match. A missing entry file
    // is `acif.body.empty`, as in the reference.
    if body
        .files
        .iter()
        .all(|(path, _)| path == ROOT_SIDECAR || excluded_from_classification(path))
    {
        return Err(Error::Reject("acif.body.empty"));
    }
    let Some((_, entry_bytes)) = body.files.iter().find(|(path, _)| path == entry_file) else {
        return Err(Error::Reject("acif.body.empty"));
    };
    // Each file's NFC path, whether it is the entry file (matched raw), and
    // its bytes.
    let mut files: Vec<(String, bool, &[u8])> = body
        .files
        .iter()
        .filter(|(path, _)| path != ROOT_SIDECAR)
        .map(|(path, bytes)| (path.nfc().collect(), path == entry_file, bytes.as_slice()))
        .collect();
    let entry_text = canonical_text(entry_bytes);
    let entry_input = split_frontmatter(&entry_text).1;
    if !multi_file {
        return Ok(HashedBody {
            hash: Hash::sha256(entry_input),
            classification: Classification::SingleFile,
            entry_text: entry_text.clone(),
        });
    }
    files.sort_by(|a, b| a.0.cmp(&b.0));
    let mut manifest = String::new();
    for (path, entry, bytes) in &files {
        let hash = if *entry {
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

/// A path's key for NFC collision detection (core §7.4 step 3): its NFC
/// form, with each byte that is not UTF-8 kept as is. Such a byte acts as a
/// starter, as the lone surrogate it decodes to in the reference does, so
/// composition never crosses it, and two different bytes never collide.
fn collision_key(path: &[u8]) -> Vec<u8> {
    let mut key = Vec::with_capacity(path.len());
    for chunk in path.utf8_chunks() {
        let valid: String = chunk.valid().nfc().collect();
        key.extend_from_slice(valid.as_bytes());
        key.extend_from_slice(chunk.invalid());
    }
    key
}

/// `LICENSE*` and `README*` regular files at the body root, matched as a
/// case-insensitive prefix (core §7.2). The match uppercases with Unicode
/// full case mapping, as the reference's `str.upper()` does, so `lıcense`
/// and `licenſe` match too.
fn excluded_from_classification(path: &str) -> bool {
    let upper = path.to_uppercase();
    !path.contains('/') && (upper.starts_with("LICENSE") || upper.starts_with("README"))
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

    fn reject_of(body: &Body) -> &'static str {
        reject_at(body, "SKILL.md")
    }

    fn reject_at(body: &Body, entry_file: &str) -> &'static str {
        match hash_body(body, entry_file) {
            Err(Error::Reject(id)) => id,
            Err(error) => panic!("expected a reject, got {error}"),
            Ok(_) => panic!("expected a reject, got a hash"),
        }
    }

    #[test]
    fn control_char_scan_runs_before_collision_detection() {
        let body = Body::from_files(&[
            ("SKILL.md", b"x\n"),
            ("caf\u{e9}.md", b"a\n"),
            ("cafe\u{301}.md", b"b\n"),
            ("z\n.md", b"c\n"),
        ]);
        assert_eq!(reject_of(&body), "acif.body.path_control_char");
        let body = Body::from_files(&[("SKILL.md", b"x\n"), ("d\x7f/run.sh", b"c\n")]);
        assert_eq!(reject_of(&body), "acif.body.path_control_char");
    }

    // A rule using char::is_control would reject U+0080–U+009F too.
    #[test]
    fn c1_controls_are_allowed_in_paths() {
        let body = Body::from_files(&[("SKILL.md", b"x\n"), ("notes\u{80}.md", b"y\n")]);
        let hashed = hash_body(&body, "SKILL.md").unwrap();
        assert_eq!(hashed.classification, Classification::MultiFile);
    }

    // Classification and the entry lookup compare raw paths, as the
    // reference does. U+212A KELVIN SIGN normalizes to K, so its SKILL.md
    // spelling is NFC-equal to the entry file but is a content file.
    #[test]
    fn classification_compares_raw_paths_with_entry_file() {
        let body = Body::from_files(&[
            ("SKILL.md", b"x\n"),
            ("S\u{212a}ILL.md", b"y\n"),
            ("README\n.md", b"z\n"),
        ]);
        assert_eq!(reject_of(&body), "acif.body.path_control_char");
        // A missing entry file is acif.body.empty, as in the reference.
        let body = Body::from_files(&[("S\u{212a}ILL.md", b"x\n")]);
        assert_eq!(reject_of(&body), "acif.body.empty");
    }

    // The reference excludes the entry file from classification only at the
    // body root, so a nested entry file makes the body multi-file.
    #[test]
    fn nested_entry_file_is_a_content_file() {
        let body = Body::from_files(&[("d\n/RULE.md", b"x\n")]);
        assert_eq!(
            reject_at(&body, "d\n/RULE.md"),
            "acif.body.path_control_char"
        );
    }

    // The reference uppercases with str.upper(), so U+0131 DOTLESS I and
    // U+017F LONG S match the I and S of LICENSE.
    #[test]
    fn root_exclusions_use_unicode_uppercase() {
        for name in ["L\u{131}CENSE\n", "licen\u{17f}e\n"] {
            let body = Body::from_files(&[("SKILL.md", b"x\n"), (name, b"y\n")]);
            let hashed = hash_body(&body, "SKILL.md").unwrap();
            assert_eq!(hashed.classification, Classification::SingleFile);
        }
        // The empty check matches the raw name: NFC composes E and U+0301
        // into U+00C9, which would no longer start with README.
        let body = Body::from_files(&[("README\u{301}", b"x\n")]);
        assert_eq!(reject_at(&body, "README\u{301}"), "acif.body.empty");
    }

    /// A directory under the system temp dir, removed on drop. Nothing
    /// outside it is ever removed.
    #[cfg(any(target_os = "linux", windows))]
    struct TempDir(std::path::PathBuf);

    #[cfg(any(target_os = "linux", windows))]
    impl TempDir {
        /// Creates a fresh, empty directory.
        fn new(label: &str) -> TempDir {
            use std::sync::atomic::{AtomicU32, Ordering};
            static NEXT: AtomicU32 = AtomicU32::new(0);
            let n = NEXT.fetch_add(1, Ordering::Relaxed);
            let dir = TempDir(
                std::env::temp_dir().join(format!("acif-test-{}-{n}-{label}", std::process::id())),
            );
            fs::create_dir(&dir.0).unwrap();
            dir
        }

        /// Writes `files`, each a relative path in raw bytes, into a fresh
        /// directory.
        #[cfg(target_os = "linux")]
        fn with_files(label: &str, files: &[&[u8]]) -> TempDir {
            use std::os::unix::ffi::OsStrExt;
            let dir = TempDir::new(label);
            for path in files {
                let path = dir.0.join(std::ffi::OsStr::from_bytes(path));
                fs::create_dir_all(path.parent().unwrap()).unwrap();
                fs::write(path, b"x\n").unwrap();
            }
            dir
        }
    }

    #[cfg(any(target_os = "linux", windows))]
    impl Drop for TempDir {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    // Linux file systems hold any byte name and APFS refuses one that is not
    // UTF-8, so these run on Linux only. NTFS holds an unpaired UTF-16
    // surrogate, which the Windows test below builds.
    #[cfg(target_os = "linux")]
    #[test]
    fn not_utf8_scan_runs_before_control_char_scan() {
        let dir = TempDir::with_files("order", &[b"SKILL.md", b"a\n.md", b"z\xff.md"]);
        assert_eq!(
            reject_of(&Body::read(&dir.0).unwrap()),
            "acif.body.path_not_utf8"
        );
        let dir = TempDir::with_files("dir", &[b"SKILL.md", b"d\xff/run.sh"]);
        assert_eq!(
            reject_of(&Body::read(&dir.0).unwrap()),
            "acif.body.path_not_utf8"
        );
    }

    // The §7.4 manifest, and so its path scans, runs only for a multi-file
    // body (§7.2). Root README* files leave this body single-file.
    #[cfg(target_os = "linux")]
    #[test]
    fn single_file_body_paths_are_not_scanned() {
        let dir = TempDir::with_files("single", &[b"SKILL.md", b"README\n.md", b"README\xff"]);
        let hashed = hash_body(&Body::read(&dir.0).unwrap(), "SKILL.md").unwrap();
        assert_eq!(hashed.classification, Classification::SingleFile);
    }

    // The reference NFC-normalizes a path that is not UTF-8 too, with each
    // invalid byte a lone surrogate, and checks a single-file body's paths.
    #[cfg(target_os = "linux")]
    #[test]
    fn nfc_collision_covers_not_utf8_paths() {
        let dir = TempDir::with_files(
            "nfc",
            &[b"SKILL.md", b"README\xffe\xcc\x81", b"README\xff\xc3\xa9"],
        );
        assert_eq!(
            reject_of(&Body::read(&dir.0).unwrap()),
            "acif.body.path_collision"
        );
        // Both spell README\u{fffd} lossily, but their bytes differ.
        let dir = TempDir::with_files("bytes", &[b"SKILL.md", b"README\xfe", b"README\xff"]);
        let hashed = hash_body(&Body::read(&dir.0).unwrap(), "SKILL.md").unwrap();
        assert_eq!(hashed.classification, Classification::SingleFile);
    }

    // Its lossy spelling, rule\u{fffd}.md, is the entry file's name, but a
    // path that is not UTF-8 is never the entry file.
    #[cfg(target_os = "linux")]
    #[test]
    fn not_utf8_path_never_matches_entry_file() {
        let dir = TempDir::with_files("lossy", &[b"rule\xef\xbf\xbd.md", b"rule\xff.md"]);
        assert_eq!(
            reject_at(&Body::read(&dir.0).unwrap(), "rule\u{fffd}.md"),
            "acif.body.path_not_utf8"
        );
    }

    // A symlink rejects as the tree is read (§7.4 step 2), whichever of the
    // two the walk meets first.
    #[cfg(target_os = "linux")]
    #[test]
    fn symlink_beats_not_utf8_path() {
        let dir = TempDir::with_files("link", &[b"SKILL.md", b"z\xff.md"]);
        std::os::unix::fs::symlink("SKILL.md", dir.0.join("link.md")).unwrap();
        assert!(matches!(
            Body::read(&dir.0),
            Err(Error::Reject("acif.body.symlink"))
        ));
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn not_utf8_scan_runs_before_collision_detection() {
        let dir = TempDir::with_files(
            "nfc-order",
            &[
                b"SKILL.md",
                b"caf\xc3\xa9.md",
                b"cafe\xcc\x81.md",
                b"z\xff.md",
            ],
        );
        assert_eq!(
            reject_of(&Body::read(&dir.0).unwrap()),
            "acif.body.path_not_utf8"
        );
    }

    // NTFS holds an unpaired UTF-16 surrogate, which is not UTF-8 either.
    #[cfg(windows)]
    #[test]
    fn unpaired_surrogate_is_not_utf8() {
        use std::os::windows::ffi::OsStringExt;
        let dir = TempDir::new("surrogate");
        fs::write(dir.0.join("SKILL.md"), b"x\n").unwrap();
        let name: Vec<u16> = [0x7A, 0xD800]
            .into_iter()
            .chain(".md".encode_utf16())
            .collect();
        fs::write(dir.0.join(std::ffi::OsString::from_wide(&name)), b"y\n").unwrap();
        assert_eq!(
            reject_of(&Body::read(&dir.0).unwrap()),
            "acif.body.path_not_utf8"
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
