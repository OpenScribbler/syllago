#![allow(dead_code)]

use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU32, Ordering};

use moat::ContentHash;
use serde_json::Value;

/// The committed MOAT vectors.
pub fn vectors() -> Value {
    serde_json::from_str(include_str!("../vectors/moat-vectors.json")).unwrap()
}

/// A directory of this test's own under the system temp dir, removed on
/// drop. Nothing outside it is ever removed.
pub struct TempDir(PathBuf);

impl TempDir {
    pub fn new(label: &str) -> TempDir {
        static NEXT: AtomicU32 = AtomicU32::new(0);
        let n = NEXT.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("moat-test-{}-{n}-{label}", std::process::id()));
        std::fs::create_dir(&path).unwrap();
        TempDir(path)
    }

    pub fn path(&self) -> &Path {
        &self.0
    }
}

impl Drop for TempDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// Writes `files` (relative POSIX path to text) into a fresh directory and
/// ingests it as an ACIF skill whose entry file is `SKILL.md`.
pub fn acif_item(files: &serde_json::Map<String, Value>) -> acif::Item {
    let dir = TempDir::new("acif");
    for (path, text) in files {
        let path = dir.path().join(path);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, text.as_str().unwrap()).unwrap();
    }
    let body = acif::Body::read(dir.path()).unwrap();
    acif::ingest_body(acif::Kind::Skill, &body, "SKILL.md").unwrap()
}

/// The Content Hash of an item made of `files`.
pub fn content_hash(files: &serde_json::Map<String, Value>) -> ContentHash {
    ContentHash::of_acif(&acif_item(files)).unwrap()
}

/// The text of a named list from the vectors.
pub fn list<'a>(doc: &'a Value, name: &str) -> &'a [u8] {
    doc["author_key"]["lists"][name]["text"]
        .as_str()
        .unwrap()
        .as_bytes()
}

/// Standard base64 with padding.
pub fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let b = [
            chunk[0],
            *chunk.get(1).unwrap_or(&0),
            *chunk.get(2).unwrap_or(&0),
        ];
        let n = u32::from(b[0]) << 16 | u32::from(b[1]) << 8 | u32::from(b[2]);
        for i in 0..4 {
            if i <= chunk.len() {
                out.push(char::from(ALPHABET[(n >> (18 - 6 * i) & 63) as usize]));
            } else {
                out.push('=');
            }
        }
    }
    out
}

/// Appends `bytes` as an SSH `string`.
pub fn put_string(out: &mut Vec<u8>, bytes: &[u8]) {
    out.extend_from_slice(&u32::try_from(bytes.len()).unwrap().to_be_bytes());
    out.extend_from_slice(bytes);
}

/// The `ssh-ed25519` public-key blob for a raw 32-byte key.
pub fn ed25519_key_blob(public: &[u8]) -> Vec<u8> {
    let mut blob = Vec::new();
    put_string(&mut blob, b"ssh-ed25519");
    put_string(&mut blob, public);
    blob
}

/// The allowed-signers line for a key blob: fingerprint, type, base64 blob.
pub fn signers_line(blob: &[u8]) -> String {
    let fingerprint = moat::Fingerprint::of_key_blob(blob);
    let len = u32::from_be_bytes(blob[..4].try_into().unwrap()) as usize;
    let key_type = std::str::from_utf8(&blob[4..4 + len]).unwrap();
    format!("{fingerprint} {key_type} {}", base64(blob))
}
