//! The ownership record and the snapshot, as syllago writes them.

use std::path::{Path, PathBuf};

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use sha2::{Digest, Sha256};

/// The record's format version.
pub const VERSION: u32 = 1;

/// What syllago owns on this machine. Remove and drift detection read only
/// this. An entry names a native path, since the record never leaves the
/// machine it describes.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Record {
    pub version: u32,
    pub entries: Vec<Entry>,
}

impl Default for Record {
    fn default() -> Record {
        Record {
            version: VERSION,
            entries: Vec::new(),
        }
    }
}

/// One thing syllago owns. A settings key written into a provider's
/// settings file is a further kind, added beside these.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Entry {
    /// A file an apply placed whole.
    File {
        path: PathBuf,
        /// The item that placed it.
        owner: String,
        /// The lowercase hex SHA-256 of the bytes placed.
        sha256: String,
        executable: bool,
        /// The file that was there before, replaced with `--overwrite`.
        /// Remove puts it back.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        prior: Option<Prior>,
    },
    /// One item's contribution to a shared file, between the markers for
    /// its `key`.
    Contribution {
        path: PathBuf,
        owner: String,
        /// The contribution key: the item's content name (install-targets
        /// §10).
        key: String,
        /// The lowercase hex SHA-256 of the bytes between the markers.
        sha256: String,
        /// The bytes inserted before the opening marker to set the block off
        /// from what precedes it. Removing the block removes them too when
        /// they are still there.
        separator: String,
    },
    /// A shared file that did not exist before syllago contributed to it.
    /// It is deleted when the last contribution leaves it empty.
    CreatedFile { path: PathBuf },
    /// A directory an apply created. It is removed once it is empty.
    Dir { path: PathBuf },
}

impl Entry {
    pub fn path(&self) -> &Path {
        match self {
            Entry::File { path, .. }
            | Entry::Contribution { path, .. }
            | Entry::CreatedFile { path }
            | Entry::Dir { path } => path,
        }
    }

    pub fn owner(&self) -> Option<&str> {
        match self {
            Entry::File { owner, .. } | Entry::Contribution { owner, .. } => Some(owner.as_str()),
            Entry::CreatedFile { .. } | Entry::Dir { .. } => None,
        }
    }
}

/// A file's bytes and executable bit, as they were before a change.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Prior {
    #[serde(serialize_with = "to_hex", deserialize_with = "from_hex")]
    pub bytes: Vec<u8>,
    pub executable: bool,
}

/// Everything a run is about to change, written before its first change,
/// so that [`crate::Engine::recover`] can undo a run that stopped part way,
/// a crash included. It is deleted once the run or its rollback finishes.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub(crate) struct Snapshot {
    /// The run wrote its record and only deleting the snapshot was left.
    pub committed: bool,
    /// The ownership record before the run, with every original it holds;
    /// `None` when there was none.
    pub record: Option<Prior>,
    /// Each file the run writes or deletes, with what was there before
    /// (`None`: nothing, so undoing deletes it).
    pub files: Vec<(PathBuf, Option<Prior>)>,
    /// Directories the run creates, which undoing removes.
    pub created_dirs: Vec<PathBuf>,
    /// Existing empty-or-not directories the run may remove, which undoing
    /// recreates, shallowest first.
    pub prunable_dirs: Vec<PathBuf>,
}

pub(crate) fn sha256(bytes: &[u8]) -> String {
    hex(&Sha256::digest(bytes))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

fn to_hex<S: Serializer>(bytes: &[u8], serializer: S) -> Result<S::Ok, S::Error> {
    serializer.serialize_str(&hex(bytes))
}

fn from_hex<'de, D: Deserializer<'de>>(deserializer: D) -> Result<Vec<u8>, D::Error> {
    let text = String::deserialize(deserializer)?;
    let digit = |b: u8| match b {
        b'0'..=b'9' => Some(b - b'0'),
        b'a'..=b'f' => Some(b - b'a' + 10),
        _ => None,
    };
    if text.len() % 2 != 0 {
        return Err(serde::de::Error::custom("odd-length hex"));
    }
    text.as_bytes()
        .chunks(2)
        .map(|pair| Some(digit(pair[0])? << 4 | digit(pair[1])?))
        .collect::<Option<Vec<u8>>>()
        .ok_or_else(|| serde::de::Error::custom("not lowercase hex"))
}
