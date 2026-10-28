//! Syllago's apply engine. It takes the adapters' placements for every
//! targeted provider and writes them as one change: it plans every file and
//! shared-file contribution, snapshots everything it will touch, writes, and
//! records what it owns. Any failure rolls back everything the run did, byte
//! for byte. Remove undoes what the record says an item placed, and nothing
//! else; drift detection compares what is on disk with the record.
//!
//! Rendered files are always written, never symlinked: providers rewrite
//! their own files, and symlinks break on Windows. A contribution to a
//! shared file sits between marker lines carrying its key ([`shared`]).
//!
//! The engine keeps two files in a state directory the caller chooses: the
//! ownership record, `ownership.json`, written atomically only once a run's
//! changes are all made, and the snapshot, `snapshot.json`, which holds
//! everything a run is about to change, the previous record included. It
//! outlives a run only when the run crashed or its rollback could not
//! finish; [`Engine::recover`] then replays it, and every run refuses to
//! start until it has.

mod fs;
mod record;
mod shared;

use std::collections::{BTreeMap, BTreeSet};
use std::fmt;
use std::io;
use std::path::{Component, Path, PathBuf};

use acif::{Diagnostic, Rendering, Roots, Value};
use adapters::Placement;
use unicode_normalization::UnicodeNormalization;

pub use fs::{Fs, Node, RealFs};
pub use record::{Entry, Prior, Record};
use record::{Snapshot, sha256};

const RECORD_FILE: &str = "ownership.json";
const SNAPSHOT_FILE: &str = "snapshot.json";

/// One item's placement, as [`adapters::place`] returned it.
#[derive(Debug, Clone)]
pub struct Placed {
    /// The item that owns what the placement writes, by its ACIF identity.
    pub owner: String,
    /// The item's content name, which keys its contribution to a shared
    /// file.
    pub name: String,
    pub placement: Placement,
}

/// How an apply or remove treats a file that differs from the record.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Options {
    /// Replace a hand-edited file or contribution, or a file syllago did not
    /// place, instead of stopping. A replaced file that syllago did not
    /// place is kept in the record and restored by remove.
    pub overwrite: bool,
}

/// What a run does to one file or one contribution.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Change {
    pub path: PathBuf,
    /// The contribution's key, for a contribution to a shared file.
    pub key: Option<String>,
    pub owner: String,
    pub action: Action,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Action {
    Create,
    Update,
    Unchanged,
    Delete,
    /// Put back the file that was there before the item was applied.
    Restore,
}

impl Action {
    pub fn as_str(self) -> &'static str {
        match self {
            Action::Create => "create",
            Action::Update => "update",
            Action::Unchanged => "unchanged",
            Action::Delete => "delete",
            Action::Restore => "restore",
        }
    }
}

/// A file or contribution a run would not change without
/// [`Options::overwrite`], or at all.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Conflict {
    pub path: PathBuf,
    pub key: Option<String>,
    pub reason: Reason,
    /// What is there now: the file's bytes, or a contribution's bytes
    /// between its markers. `None` when there is no file, or no single
    /// block, to read.
    pub current: Option<Vec<u8>>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Reason {
    /// An owned file or contribution was changed or removed by hand.
    Drifted { owner: String },
    /// A file syllago did not place is in the way, or a block carries the
    /// key with no record behind it.
    Unowned,
    /// Another item owns the file or key.
    OwnedBy { owner: String },
    /// A directory, symlink or other non-file is where a file goes.
    NotAFile,
    /// A shared file is a symlink that resolves to nothing, or to
    /// something other than a regular file (`target`).
    Symlink { target: Option<PathBuf> },
}

impl Conflict {
    pub fn id(&self) -> &'static str {
        match self.reason {
            Reason::Drifted { .. } => "syllago.apply.drifted",
            Reason::Unowned => "syllago.apply.unowned",
            Reason::OwnedBy { .. } => "syllago.apply.owned_by_other",
            Reason::NotAFile => "syllago.apply.not_a_file",
            Reason::Symlink { .. } => "syllago.apply.symlink_unusable",
        }
    }

    /// Whether [`Options::overwrite`] lifts it.
    pub fn overridable(&self) -> bool {
        matches!(self.reason, Reason::Drifted { .. } | Reason::Unowned)
    }
}

impl fmt::Display for Conflict {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let path = self.path.display();
        let at = match &self.key {
            Some(key) => format!("the {key} block in {path}"),
            None => path.to_string(),
        };
        match &self.reason {
            Reason::Drifted { owner } => write!(
                f,
                "{}: {at}, placed for {owner}, was changed by hand; capture the edit into the \
                 Library to keep it, or pass --overwrite to discard it",
                self.id()
            ),
            Reason::Unowned => write!(
                f,
                "{}: {at} is in the way and syllago did not place it; move it aside, or pass \
                 --overwrite to replace it (remove puts a replaced file back)",
                self.id()
            ),
            Reason::OwnedBy { owner } => write!(
                f,
                "{}: {at} is placed for {owner}; remove that item first",
                self.id()
            ),
            Reason::NotAFile => write!(
                f,
                "{}: {path} is a directory or symlink where a file goes; move it aside",
                self.id()
            ),
            Reason::Symlink { target: None } => write!(
                f,
                "{}: {path} is a symlink that points at nothing, so there is no file to add \
                 to; point it at a file or remove it",
                self.id()
            ),
            Reason::Symlink {
                target: Some(target),
            } => write!(
                f,
                "{}: {path} is a symlink to {}, which is not a regular file; syllago writes \
                 through a link only to a file",
                self.id(),
                target.display()
            ),
        }
    }
}

/// What a run would do, computed without changing anything.
#[derive(Debug, Clone)]
pub struct Plan {
    /// Every file and contribution the run writes, deletes or leaves, by
    /// path and key.
    pub changes: Vec<Change>,
    /// Why the run would stop. A run with conflicts changes nothing.
    pub conflicts: Vec<Conflict>,
    /// `acif.install.contribution_not_found` for an owned contribution
    /// whose block is gone; nothing is removed for it.
    pub diagnostics: Vec<Diagnostic>,
    mkdirs: Vec<PathBuf>,
    ops: Vec<Op>,
    record: Record,
}

#[derive(Debug, Clone)]
enum Op {
    Write {
        path: PathBuf,
        bytes: Vec<u8>,
        executable: bool,
    },
    Delete(PathBuf),
}

impl Op {
    fn path(&self) -> &Path {
        match self {
            Op::Write { path, .. } | Op::Delete(path) => path,
        }
    }
}

/// What a finished run did.
#[derive(Debug, Clone)]
pub struct Outcome {
    pub changes: Vec<Change>,
    pub diagnostics: Vec<Diagnostic>,
    /// The snapshot could not be deleted after the run finished. Nothing
    /// needs restoring; [`Engine::recover`] deletes it.
    pub leftover_snapshot: Option<PathBuf>,
}

/// One step of a rollback that failed, to be done by hand, in order.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ManualStep {
    /// Write `bytes` to `path`, executable when `executable` is set.
    Restore {
        path: PathBuf,
        bytes: Vec<u8>,
        executable: bool,
    },
    /// Delete the file at `path`, which the run created.
    Delete { path: PathBuf },
    /// Remove the directory at `path`, which the run created.
    RemoveDir { path: PathBuf },
    /// Create the directory at `path`, which the run removed.
    CreateDir { path: PathBuf },
}

impl fmt::Display for ManualStep {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ManualStep::Restore { path, .. } => {
                write!(f, "write the saved bytes back to {}", path.display())
            }
            ManualStep::Delete { path } => write!(f, "delete {}", path.display()),
            ManualStep::RemoveDir { path } => write!(f, "remove the directory {}", path.display()),
            ManualStep::CreateDir { path } => write!(f, "create the directory {}", path.display()),
        }
    }
}

/// How a failed run was undone.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Rollback {
    /// Every change was undone and the snapshot deleted.
    Complete,
    /// Some changes could not be undone. The snapshot is kept, so
    /// [`Engine::recover`] can retry; `steps` say what is still to do,
    /// in order, for doing it by hand.
    Incomplete {
        snapshot: PathBuf,
        steps: Vec<ManualStep>,
    },
}

/// Why a run did not happen, or failed and was rolled back.
#[derive(Debug)]
pub enum Error {
    /// Nothing was changed: the plan has conflicts.
    Blocked(Vec<Conflict>),
    /// Two placements write the same file or the same contribution.
    Collision { path: PathBuf, key: Option<String> },
    /// Two placements write paths that differ only by case, which name one
    /// file on default macOS and Windows filesystems.
    CaseCollision { paths: (PathBuf, PathBuf) },
    /// A placement names a path or key that cannot be written.
    InvalidTarget(String),
    /// The ownership record cannot be read.
    RecordMalformed { path: PathBuf, message: String },
    /// A snapshot from an earlier run is still there.
    SnapshotPending(PathBuf),
    /// Reading `path` failed before anything was changed.
    Io { path: PathBuf, error: io::Error },
    /// A change to `path` failed, and the run was rolled back.
    Failed {
        path: PathBuf,
        error: io::Error,
        rollback: Rollback,
    },
}

impl Error {
    pub fn id(&self) -> &'static str {
        match self {
            Error::Blocked(_) => "syllago.apply.blocked",
            Error::Collision { .. } => "syllago.apply.collision",
            Error::CaseCollision { .. } => "syllago.apply.case_collision",
            Error::InvalidTarget(_) => "syllago.apply.invalid_target",
            Error::RecordMalformed { .. } => "syllago.apply.record_malformed",
            Error::SnapshotPending(_) => "syllago.apply.snapshot_pending",
            Error::Io { .. } => "syllago.apply.read_failed",
            Error::Failed {
                rollback: Rollback::Complete,
                ..
            } => "syllago.apply.failed",
            Error::Failed { .. } => "syllago.apply.rollback_incomplete",
        }
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            Error::Blocked(conflicts) => {
                write!(f, "{id}: nothing was changed")?;
                for conflict in conflicts {
                    write!(f, "\n  {conflict}")?;
                }
                Ok(())
            }
            Error::Collision { path, key } => write!(
                f,
                "{id}: two items write {}{}; target them in separate loadouts",
                path.display(),
                key.as_ref()
                    .map_or(String::new(), |key| format!(" under the key {key}"))
            ),
            Error::CaseCollision { paths: (a, b) } => write!(
                f,
                "{id}: {} and {} differ only by case, so on macOS and Windows they are one file \
                 and one item would replace the other; rename one of the items",
                a.display(),
                b.display()
            ),
            Error::InvalidTarget(message) => write!(f, "{id}: {message}"),
            Error::RecordMalformed { path, message } => write!(
                f,
                "{id}: the ownership record {} cannot be read ({message}); restore it from a \
                 backup, since remove and drift detection rely on it",
                path.display()
            ),
            Error::SnapshotPending(path) => write!(
                f,
                "{id}: an earlier run stopped before it finished and left {}; run recover, \
                 which puts back what that run changed, before anything else",
                path.display()
            ),
            Error::Io { path, error } => write!(
                f,
                "{id}: reading {}: {error}; nothing was changed",
                path.display()
            ),
            Error::Failed {
                path,
                error,
                rollback,
            } => {
                write!(f, "{id}: changing {}: {error}; ", path.display())?;
                match rollback {
                    Rollback::Complete => write!(f, "every change was rolled back"),
                    Rollback::Incomplete { snapshot, steps } => {
                        write!(
                            f,
                            "rollback could not finish, so the snapshot {} is kept; run recover \
                             once the cause is fixed, which retries what is left:",
                            snapshot.display()
                        )?;
                        for step in steps {
                            write!(f, "\n  {step}")?;
                        }
                        Ok(())
                    }
                }
            }
        }
    }
}

impl std::error::Error for Error {}

fn read_failed(path: &Path) -> impl FnOnce(io::Error) -> Error + '_ {
    move |error| Error::Io {
        path: path.to_path_buf(),
        error,
    }
}

/// The apply engine over a filesystem, the roots placements resolve
/// against, and the state directory holding the record and snapshot.
pub struct Engine<'a, F: Fs> {
    fs: &'a F,
    roots: Roots<'a>,
    state: &'a Path,
}

impl<'a, F: Fs> Engine<'a, F> {
    pub fn new(fs: &'a F, roots: Roots<'a>, state: &'a Path) -> Engine<'a, F> {
        Engine { fs, roots, state }
    }

    pub fn record_path(&self) -> PathBuf {
        self.state.join(RECORD_FILE)
    }

    pub fn snapshot_path(&self) -> PathBuf {
        self.state.join(SNAPSHOT_FILE)
    }

    /// The ownership record; empty before the first apply.
    pub fn record(&self) -> Result<Record, Error> {
        let path = self.record_path();
        let malformed = |message: String| Error::RecordMalformed {
            path: path.clone(),
            message,
        };
        match self.fs.read(&path).map_err(read_failed(&path))? {
            Node::Missing => Ok(Record::default()),
            Node::File { bytes, .. } => {
                let record: Record =
                    serde_json::from_slice(&bytes).map_err(|e| malformed(e.to_string()))?;
                if record.version != record::VERSION {
                    return Err(malformed(format!("unknown version {}", record.version)));
                }
                Ok(record)
            }
            _ => Err(malformed("not a file".into())),
        }
    }

    /// What applying `placed` would do. Each item's placements must all be
    /// here: whatever the record says an item placed that these do not
    /// place again is removed.
    pub fn preview_apply(&self, placed: &[Placed], options: Options) -> Result<Plan, Error> {
        let owners = placed.iter().map(|p| p.owner.as_str()).collect();
        self.plan(placed, &owners, options)
    }

    /// Applies `placed` across every provider it targets, all or nothing.
    pub fn apply(&self, placed: &[Placed], options: Options) -> Result<Outcome, Error> {
        let plan = self.preview_apply(placed, options)?;
        self.commit(plan)
    }

    /// What removing `owners` would do.
    pub fn preview_remove(&self, owners: &[&str], options: Options) -> Result<Plan, Error> {
        self.plan(&[], &owners.iter().copied().collect(), options)
    }

    /// Removes what the record says `owners` placed, all or nothing: a file
    /// an item placed is deleted, or the file it replaced put back, and an
    /// item's contribution is taken out of its shared file.
    pub fn remove(&self, owners: &[&str], options: Options) -> Result<Outcome, Error> {
        let plan = self.preview_remove(owners, options)?;
        self.commit(plan)
    }

    /// Every owned file and contribution that differs from the record,
    /// with its owner and what is there now, so it can be captured.
    pub fn drift(&self) -> Result<Vec<Conflict>, Error> {
        let mut drifted = Vec::new();
        for entry in self.record()?.entries {
            let current = match &entry {
                Entry::File { path, .. } => match self.read(path)? {
                    node if !edited(&entry, &node) => continue,
                    Node::File { bytes, .. } => Some(bytes),
                    _ => None,
                },
                Entry::Contribution {
                    path, key, sha256, ..
                } => {
                    let text = match self.resolve(path)? {
                        Ok((_, Node::File { bytes, .. })) => bytes,
                        _ => Vec::new(),
                    };
                    match shared::find(&text, key).as_slice() {
                        [block] if record::sha256(&text[block.inner.clone()]) == *sha256 => {
                            continue;
                        }
                        [block] => Some(text[block.inner.clone()].to_vec()),
                        _ => None,
                    }
                }
                Entry::CreatedFile { .. } | Entry::Dir { .. } => continue,
            };
            let key = match &entry {
                Entry::Contribution { key, .. } => Some(key.clone()),
                _ => None,
            };
            drifted.push(Conflict {
                path: entry.path().to_path_buf(),
                key,
                reason: Reason::Drifted {
                    owner: entry.owner().unwrap_or_default().to_string(),
                },
                current,
            });
        }
        Ok(drifted)
    }

    fn read(&self, path: &Path) -> Result<Node, Error> {
        self.fs.read(path).map_err(read_failed(path))
    }

    /// The path a shared file is written at and what is there: the file
    /// itself, or a symlink's target when that is a regular file. A link
    /// to nothing, or to anything else, is refused.
    fn resolve(&self, path: &Path) -> Result<Result<(PathBuf, Node), Reason>, Error> {
        Ok(match self.read(path)? {
            Node::Link {
                target: Some(target),
            } => match self.read(&target)? {
                node @ Node::File { .. } => Ok((target, node)),
                _ => Err(Reason::Symlink {
                    target: Some(target),
                }),
            },
            Node::Link { target: None } => Err(Reason::Symlink { target: None }),
            node => Ok((path.to_path_buf(), node)),
        })
    }

    fn plan(
        &self,
        placed: &[Placed],
        owners: &BTreeSet<&str>,
        options: Options,
    ) -> Result<Plan, Error> {
        // What an unfinished run left would read as conflicts, or be
        // planned over, so it is reported first.
        let snapshot_path = self.snapshot_path();
        if self.read(&snapshot_path)? != Node::Missing {
            return Err(Error::SnapshotPending(snapshot_path));
        }
        let old = self.record()?;
        let mut files: BTreeMap<PathBuf, (&str, &[u8], bool)> = BTreeMap::new();
        let mut contributions: BTreeMap<PathBuf, Vec<Wanted<'_>>> = BTreeMap::new();
        for p in placed {
            let target = p
                .placement
                .target
                .native(&self.roots)
                .map_err(|e| Error::InvalidTarget(e.to_string()))?;
            match &p.placement.rendering {
                Rendering::Files(rendered) => {
                    for file in rendered {
                        let path = join(&target, &file.path)?;
                        let value = (p.owner.as_str(), file.bytes.as_slice(), file.executable);
                        if files.insert(path.clone(), value).is_some() {
                            return Err(Error::Collision { path, key: None });
                        }
                    }
                }
                Rendering::Contribution(bytes) => {
                    if !shared::valid_key(&p.name) {
                        return Err(Error::InvalidTarget(format!(
                            "{:?} cannot key a contribution to {}",
                            p.name,
                            target.display()
                        )));
                    }
                    let list = contributions.entry(target.clone()).or_default();
                    if list.iter().any(|(_, key, _)| *key == p.name) {
                        return Err(Error::Collision {
                            path: target,
                            key: Some(p.name.clone()),
                        });
                    }
                    list.push((p.owner.as_str(), p.name.as_str(), bytes.as_slice()));
                }
            }
        }

        // Paths that differ only by case name one file on default macOS and
        // Windows filesystems, so one write would replace the other.
        let mut seen: BTreeMap<String, &PathBuf> = BTreeMap::new();
        for path in files.keys().chain(contributions.keys()) {
            if let Some(other) = seen.insert(folded(path), path)
                && other != path
            {
                return Err(Error::CaseCollision {
                    paths: (other.clone(), path.clone()),
                });
            }
        }

        let mine = |entry: &Entry| entry.owner().is_some_and(|o| owners.contains(o));
        let mut planner = Planner {
            engine: self,
            options,
            next: old.entries.iter().filter(|e| !mine(e)).cloned().collect(),
            changes: Vec::new(),
            conflicts: Vec::new(),
            diagnostics: Vec::new(),
            mkdirs: BTreeSet::new(),
            ops: Vec::new(),
        };

        // Shared files: those placed into now, and those an owner here
        // contributed to before.
        let mut shared_paths: BTreeSet<PathBuf> = contributions.keys().cloned().collect();
        for entry in &old.entries {
            if let Entry::Contribution { path, .. } = entry
                && mine(entry)
            {
                shared_paths.insert(path.clone());
            }
        }
        for (path, file) in &files {
            if shared_paths.contains(path) {
                return Err(Error::Collision {
                    path: path.clone(),
                    key: None,
                });
            }
            planner.place_file(&old, path, *file)?;
        }
        for entry in &old.entries {
            if let Entry::File { path, .. } = entry
                && mine(entry)
                && !files.contains_key(path)
            {
                if shared_paths.contains(path) {
                    return Err(Error::Collision {
                        path: path.clone(),
                        key: None,
                    });
                }
                planner.unplace_file(entry)?;
            }
        }
        for path in &shared_paths {
            let wanted = contributions.get(path).map_or(&[][..], Vec::as_slice);
            let stale: Vec<&Entry> = old
                .entries
                .iter()
                .filter(|e| match e {
                    Entry::Contribution { path: p, key, .. } => {
                        p == path && mine(e) && !wanted.iter().any(|(_, k, _)| k == key)
                    }
                    _ => false,
                })
                .collect();
            planner.merge(&old, path, wanted, &stale)?;
        }

        let Planner {
            changes,
            conflicts,
            diagnostics,
            mkdirs,
            ops,
            mut next,
            ..
        } = planner;
        let mut mkdirs: Vec<PathBuf> = mkdirs.into_iter().collect();
        mkdirs.sort_by_key(|dir| dir.components().count());
        next.extend(mkdirs.iter().map(|path| Entry::Dir { path: path.clone() }));
        Ok(Plan {
            changes,
            conflicts,
            diagnostics,
            mkdirs,
            ops,
            record: Record {
                version: record::VERSION,
                entries: next,
            },
        })
    }

    /// Snapshots, makes the plan's changes, prunes empty directories the
    /// record owns and writes the record; on any failure, undoes the run
    /// from its snapshot.
    fn commit(&self, plan: Plan) -> Result<Outcome, Error> {
        if !plan.conflicts.is_empty() {
            return Err(Error::Blocked(plan.conflicts));
        }
        let snapshot_path = self.snapshot_path();
        if self.read(&snapshot_path)? != Node::Missing {
            return Err(Error::SnapshotPending(snapshot_path));
        }
        let record_path = self.record_path();
        let mut snapshot = Snapshot {
            committed: false,
            record: match self.read(&record_path)? {
                Node::File { bytes, .. } => Some(Prior {
                    bytes,
                    executable: false,
                }),
                _ => None,
            },
            files: Vec::new(),
            created_dirs: plan.mkdirs.clone(),
            prunable_dirs: Vec::new(),
        };
        for op in &plan.ops {
            let prior = match self.read(op.path())? {
                Node::Missing => None,
                Node::File { bytes, executable } => Some(Prior { bytes, executable }),
                _ => {
                    return Err(Error::Io {
                        path: op.path().to_path_buf(),
                        error: io::Error::other("no longer a file"),
                    });
                }
            };
            snapshot.files.push((op.path().to_path_buf(), prior));
        }
        let mut record = plan.record;
        // Shallowest first; pruning walks them in reverse, so a directory
        // empties before its parent is tried. Only those there now can be
        // removed, so only they are recreated.
        let mut prunable = Vec::new();
        for entry in &record.entries {
            if let Entry::Dir { path } = entry
                && self.read(path)? == Node::Dir
            {
                prunable.push(path.clone());
            }
        }
        prunable.sort_by_key(|dir| dir.components().count());
        snapshot.prunable_dirs = prunable.clone();

        let failed = |path: &Path, error| Error::Failed {
            path: path.to_path_buf(),
            error,
            rollback: Rollback::Complete,
        };
        if self.read(self.state)? == Node::Missing {
            self.fs
                .create_dir(self.state)
                .map_err(|e| failed(self.state, e))?;
        }
        self.write_snapshot(&snapshot)
            .map_err(|e| failed(&snapshot_path, e))?;

        let result = self
            .execute(&plan.mkdirs, &plan.ops, &prunable, &mut record)
            .and_then(|()| {
                snapshot.committed = true;
                self.write_snapshot(&snapshot)
                    .map_err(|e| (snapshot_path.clone(), e))
            });
        if let Err((path, error)) = result {
            snapshot.committed = false;
            let rollback = self.replay(&snapshot);
            return Err(Error::Failed {
                path,
                error,
                rollback,
            });
        }
        let leftover_snapshot = self
            .fs
            .remove_file(&snapshot_path)
            .err()
            .map(|_| snapshot_path);
        Ok(Outcome {
            changes: plan.changes,
            diagnostics: plan.diagnostics,
            leftover_snapshot,
        })
    }

    fn write_snapshot(&self, snapshot: &Snapshot) -> io::Result<()> {
        let json = serde_json::to_vec_pretty(snapshot).expect("a snapshot serializes");
        self.fs.write(&self.snapshot_path(), &json, false)
    }

    fn execute(
        &self,
        mkdirs: &[PathBuf],
        ops: &[Op],
        prunable: &[PathBuf],
        record: &mut Record,
    ) -> Result<(), (PathBuf, io::Error)> {
        let at = |path: &Path| {
            let path = path.to_path_buf();
            move |error| (path, error)
        };
        for dir in mkdirs {
            self.fs.create_dir(dir).map_err(at(dir))?;
        }
        for op in ops {
            match op {
                Op::Write {
                    path,
                    bytes,
                    executable,
                } => self.fs.write(path, bytes, *executable).map_err(at(path))?,
                Op::Delete(path) => match self.fs.remove_file(path) {
                    Err(e) if e.kind() != io::ErrorKind::NotFound => return Err((path.clone(), e)),
                    _ => {}
                },
            }
        }
        for dir in prunable.iter().rev() {
            let gone = match self.fs.remove_dir(dir) {
                Ok(()) => true,
                Err(e) if e.kind() == io::ErrorKind::NotFound => true,
                Err(e) if e.kind() == io::ErrorKind::DirectoryNotEmpty => false,
                Err(e) => return Err((dir.clone(), e)),
            };
            if gone {
                record
                    .entries
                    .retain(|e| !matches!(e, Entry::Dir { path } if path == dir));
            }
        }
        let path = self.record_path();
        let mut json = serde_json::to_vec_pretty(record).expect("a record serializes");
        json.push(b'\n');
        self.fs.write(&path, &json, false).map_err(at(&path))
    }

    /// Undoes a run that stopped part way, from the snapshot it left: a
    /// failure whose rollback could not finish, or a crash. A run that had
    /// written its record is kept, and only its snapshot deleted. Recovery
    /// can be run again until it is [`Rollback::Complete`]; with no
    /// snapshot there is nothing to do.
    pub fn recover(&self) -> Result<Rollback, Error> {
        let path = self.snapshot_path();
        let bytes = match self.read(&path)? {
            Node::Missing => return Ok(Rollback::Complete),
            Node::File { bytes, .. } => bytes,
            _ => {
                return Err(Error::RecordMalformed {
                    path,
                    message: "the snapshot is not a file".into(),
                });
            }
        };
        let snapshot: Snapshot =
            serde_json::from_slice(&bytes).map_err(|e| Error::RecordMalformed {
                path: path.clone(),
                message: e.to_string(),
            })?;
        Ok(self.replay(&snapshot))
    }

    /// Puts back everything `snapshot` saved, unless the run committed:
    /// pruned directories, then each file, then the created directories,
    /// then the record. Every step is safe to repeat. A step that fails,
    /// and whose path is not already as it should be, becomes a manual step
    /// and the rest still run, so what is left to do by hand is as short as
    /// it can be; the snapshot is kept until none is left.
    fn replay(&self, snapshot: &Snapshot) -> Rollback {
        let mut steps = Vec::new();
        if !snapshot.committed {
            for path in &snapshot.prunable_dirs {
                if self.fs.create_dir(path).is_err() && self.fs.read(path).ok() != Some(Node::Dir) {
                    steps.push(ManualStep::CreateDir { path: path.clone() });
                }
            }
            for (path, prior) in &snapshot.files {
                self.put_back(path, prior, &mut steps);
            }
            for path in snapshot.created_dirs.iter().rev() {
                if self.fs.remove_dir(path).is_err()
                    && self.fs.read(path).ok() != Some(Node::Missing)
                {
                    steps.push(ManualStep::RemoveDir { path: path.clone() });
                }
            }
            self.put_back(&self.record_path(), &snapshot.record, &mut steps);
        }
        let snapshot_path = self.snapshot_path();
        if steps.is_empty() && self.fs.remove_file(&snapshot_path).is_ok() {
            return Rollback::Complete;
        }
        Rollback::Incomplete {
            snapshot: snapshot_path,
            steps,
        }
    }

    /// Writes `prior` back to `path`, or deletes `path` when there was
    /// nothing.
    fn put_back(&self, path: &Path, prior: &Option<Prior>, steps: &mut Vec<ManualStep>) {
        match prior {
            Some(prior) => {
                let done = |node: Node| {
                    node == Node::File {
                        bytes: prior.bytes.clone(),
                        executable: prior.executable,
                    }
                };
                if self.fs.write(path, &prior.bytes, prior.executable).is_err()
                    && !self.fs.read(path).is_ok_and(done)
                {
                    steps.push(ManualStep::Restore {
                        path: path.to_path_buf(),
                        bytes: prior.bytes.clone(),
                        executable: prior.executable,
                    });
                }
            }
            None => {
                if self.fs.remove_file(path).is_err()
                    && self.fs.read(path).ok() != Some(Node::Missing)
                {
                    steps.push(ManualStep::Delete {
                        path: path.to_path_buf(),
                    });
                }
            }
        }
    }
}

/// Whether what is at an owned file's path differs from what the record
/// says was placed: other bytes, no file, or on Linux and macOS another
/// executable bit.
fn edited(entry: &Entry, node: &Node) -> bool {
    let Entry::File {
        sha256, executable, ..
    } = entry
    else {
        return false;
    };
    match node {
        Node::File {
            bytes,
            executable: x,
        } => record::sha256(bytes) != *sha256 || (cfg!(unix) && x != executable),
        _ => true,
    }
}

/// A path folded the way default macOS and Windows filesystems compare
/// names: NFC, then lowercase.
fn folded(path: &Path) -> String {
    path.to_string_lossy()
        .nfc()
        .collect::<String>()
        .to_lowercase()
}

/// A contribution placed now: its owner, key and bytes.
type Wanted<'p> = (&'p str, &'p str, &'p [u8]);

/// `target` joined with a rendered file's `/`-separated path, each segment
/// one plain name.
fn join(target: &Path, relative: &str) -> Result<PathBuf, Error> {
    let mut path = target.to_path_buf();
    for segment in relative.split('/').filter(|s| !s.is_empty()) {
        let mut components = Path::new(segment).components();
        match (components.next(), components.next()) {
            (Some(Component::Normal(name)), None) if name == segment => path.push(segment),
            _ => {
                return Err(Error::InvalidTarget(format!(
                    "{relative:?} under {} is not a plain relative path",
                    target.display()
                )));
            }
        }
    }
    Ok(path)
}

/// Builds a plan up path by path.
struct Planner<'e, 'a, F: Fs> {
    engine: &'e Engine<'a, F>,
    options: Options,
    /// The record after the run: every entry no owner here holds, and what
    /// the run places.
    next: Vec<Entry>,
    changes: Vec<Change>,
    conflicts: Vec<Conflict>,
    diagnostics: Vec<Diagnostic>,
    mkdirs: BTreeSet<PathBuf>,
    ops: Vec<Op>,
}

impl<F: Fs> Planner<'_, '_, F> {
    fn conflict(&mut self, path: &Path, key: Option<&str>, reason: Reason, current: Option<&[u8]>) {
        self.conflicts.push(Conflict {
            path: path.to_path_buf(),
            key: key.map(str::to_string),
            reason,
            current: current.map(<[u8]>::to_vec),
        });
    }

    fn change(&mut self, path: &Path, key: Option<&str>, owner: &str, action: Action) {
        self.changes.push(Change {
            path: path.to_path_buf(),
            key: key.map(str::to_string),
            owner: owner.to_string(),
            action,
        });
    }

    /// Plans the directories missing above `path`. `false`, with a
    /// conflict, when a file sits where a directory goes.
    fn ensure_parent(&mut self, path: &Path) -> Result<bool, Error> {
        let mut missing = Vec::new();
        let mut dir = path.parent();
        while let Some(current) = dir {
            if current.as_os_str().is_empty() || self.mkdirs.contains(current) {
                break;
            }
            match self.engine.read(current)? {
                Node::Dir => break,
                Node::Missing => missing.push(current.to_path_buf()),
                Node::File { .. } | Node::Link { .. } | Node::Other => {
                    self.conflict(current, None, Reason::NotAFile, None);
                    return Ok(false);
                }
            }
            dir = current.parent();
        }
        self.mkdirs.extend(missing);
        Ok(true)
    }

    /// Plans one whole file.
    fn place_file(
        &mut self,
        old: &Record,
        path: &Path,
        (owner, bytes, executable): (&str, &[u8], bool),
    ) -> Result<(), Error> {
        let recorded = old.entries.iter().find(|e| e.path() == path);
        let mut prior = None;
        match recorded {
            Some(
                recorded @ Entry::File {
                    owner: o, prior: p, ..
                },
            ) if o == owner => {
                prior = p.clone();
                let node = self.engine.read(path)?;
                if let Node::File { bytes: on_disk, .. } = &node
                    && edited(recorded, &node)
                    && !self.options.overwrite
                {
                    let reason = Reason::Drifted {
                        owner: owner.into(),
                    };
                    self.conflict(path, None, reason, Some(on_disk));
                    return Ok(());
                }
                return self.write_file(path, owner, bytes, executable, node, prior);
            }
            Some(entry) => {
                let owner = entry.owner().unwrap_or("another apply").to_string();
                self.conflict(path, None, Reason::OwnedBy { owner }, None);
                return Ok(());
            }
            None => {}
        }
        let node = self.engine.read(path)?;
        if let Node::File {
            bytes: on_disk,
            executable: x,
        } = &node
        {
            if !self.options.overwrite {
                self.conflict(path, None, Reason::Unowned, Some(on_disk));
                return Ok(());
            }
            prior = Some(Prior {
                bytes: on_disk.clone(),
                executable: *x,
            });
        }
        self.write_file(path, owner, bytes, executable, node, prior)
    }

    fn write_file(
        &mut self,
        path: &Path,
        owner: &str,
        bytes: &[u8],
        executable: bool,
        node: Node,
        prior: Option<Prior>,
    ) -> Result<(), Error> {
        let action = match node {
            Node::Missing => Action::Create,
            Node::File {
                bytes: on_disk,
                executable: x,
            } if on_disk == bytes && x == executable => Action::Unchanged,
            Node::File { .. } => Action::Update,
            Node::Dir | Node::Link { .. } | Node::Other => {
                self.conflict(path, None, Reason::NotAFile, None);
                return Ok(());
            }
        };
        if action != Action::Unchanged {
            if !self.ensure_parent(path)? {
                return Ok(());
            }
            self.ops.push(Op::Write {
                path: path.to_path_buf(),
                bytes: bytes.to_vec(),
                executable,
            });
        }
        self.change(path, None, owner, action);
        self.next.push(Entry::File {
            path: path.to_path_buf(),
            owner: owner.to_string(),
            sha256: sha256(bytes),
            executable,
            prior,
        });
        Ok(())
    }

    /// Plans taking away a file an owner here placed and no longer places.
    /// A file the item replaced is put back whatever is at the path now;
    /// when it cannot be, the entry is kept and the run stops, so the only
    /// copy of the original is never dropped.
    fn unplace_file(&mut self, entry: &Entry) -> Result<(), Error> {
        let Entry::File {
            path, owner, prior, ..
        } = entry
        else {
            return Ok(());
        };
        let node = self.engine.read(path)?;
        let not_a_file = matches!(node, Node::Dir | Node::Link { .. } | Node::Other);
        if prior.is_some() && not_a_file {
            self.conflict(path, None, Reason::NotAFile, None);
            self.next.push(entry.clone());
            return Ok(());
        }
        if node != Node::Missing && edited(entry, &node) && !self.options.overwrite {
            let current = match &node {
                Node::File { bytes, .. } => Some(bytes.as_slice()),
                _ => None,
            };
            let reason = Reason::Drifted {
                owner: owner.clone(),
            };
            self.conflict(path, None, reason, current);
            self.next.push(entry.clone());
            return Ok(());
        }
        match (prior, node) {
            (Some(prior), _) => {
                if !self.ensure_parent(path)? {
                    self.next.push(entry.clone());
                    return Ok(());
                }
                self.ops.push(Op::Write {
                    path: path.clone(),
                    bytes: prior.bytes.clone(),
                    executable: prior.executable,
                });
                self.change(path, None, owner, Action::Restore);
            }
            (None, Node::File { .. }) => {
                self.ops.push(Op::Delete(path.clone()));
                self.change(path, None, owner, Action::Delete);
            }
            // Already gone, or replaced by something that is not
            // syllago's and that --overwrite leaves alone.
            (None, _) => {}
        }
        Ok(())
    }

    /// Plans one shared file: the contributions `wanted` there now, and the
    /// `stale` ones owners here made before and no longer make.
    fn merge(
        &mut self,
        old: &Record,
        path: &Path,
        wanted: &[Wanted<'_>],
        stale: &[&Entry],
    ) -> Result<(), Error> {
        // A shared file kept as a symlink, as a dotfiles checkout keeps
        // one, is written through: its target is replaced and the link
        // stays.
        let (target, node) = match self.engine.resolve(path)? {
            Ok(resolved) => resolved,
            Err(reason) => {
                self.conflict(path, None, reason, None);
                return Ok(());
            }
        };
        let (before, existed, executable) = match node {
            Node::Missing => (Vec::new(), false, false),
            Node::File { bytes, executable } => (bytes, true, executable),
            _ => {
                self.conflict(path, None, Reason::NotAFile, None);
                return Ok(());
            }
        };
        if let Some(Entry::File { owner, .. }) = old
            .entries
            .iter()
            .find(|e| matches!(e, Entry::File { path: p, .. } if p == path))
        {
            let reason = Reason::OwnedBy {
                owner: owner.clone(),
            };
            self.conflict(path, None, reason, None);
            return Ok(());
        }
        let recorded = |key: &str| {
            old.entries.iter().find_map(|e| match e {
                Entry::Contribution {
                    path: p,
                    owner,
                    key: k,
                    sha256,
                    separator,
                } if p == path && k == key => Some((owner, sha256, separator)),
                _ => None,
            })
        };
        let overwrite = self.options.overwrite;
        let mut text = before.clone();

        for &(owner, key, bytes) in wanted {
            let inner = shared::inner_bytes(bytes);
            let blocks = shared::find(&text, key);
            let record = recorded(key);
            if let Some((o, _, _)) = record
                && o != owner
            {
                let reason = Reason::OwnedBy { owner: o.clone() };
                self.conflict(path, Some(key), reason, None);
                continue;
            }
            let block_text = |b: &shared::Block| text[b.inner.clone()].to_vec();
            let (action, separator) = match (record, blocks.as_slice()) {
                (Some((_, sha, separator)), [block]) => {
                    let on_disk = block_text(block);
                    if record::sha256(&on_disk) != *sha && !overwrite {
                        let reason = Reason::Drifted {
                            owner: owner.into(),
                        };
                        self.conflict(path, Some(key), reason, Some(&on_disk));
                        continue;
                    }
                    let action = if on_disk == inner {
                        Action::Unchanged
                    } else {
                        Action::Update
                    };
                    text = shared::replace(&text, block, &inner);
                    (action, separator.clone())
                }
                // The markers are gone or damaged: a hand edit.
                (Some(_), []) if !overwrite => {
                    let reason = Reason::Drifted {
                        owner: owner.into(),
                    };
                    self.conflict(path, Some(key), reason, None);
                    continue;
                }
                (Some(_), []) => {
                    self.not_found(path, key, owner);
                    let (appended, separator) = shared::append(&text, key, &inner);
                    text = appended;
                    (Action::Create, separator)
                }
                (None, []) => {
                    let (appended, separator) = shared::append(&text, key, &inner);
                    text = appended;
                    (Action::Create, separator)
                }
                (record, blocks) => {
                    if !overwrite {
                        let reason = match record {
                            Some(_) => Reason::Drifted {
                                owner: owner.into(),
                            },
                            None => Reason::Unowned,
                        };
                        let current = (blocks.len() == 1).then(|| block_text(&blocks[0]));
                        self.conflict(path, Some(key), reason, current.as_deref());
                        continue;
                    }
                    // Every block carrying the key goes, and one is written.
                    for block in blocks.iter().rev() {
                        text = shared::remove(&text, block, "");
                    }
                    let (appended, separator) = shared::append(&text, key, &inner);
                    text = appended;
                    (Action::Update, separator)
                }
            };
            self.change(path, Some(key), owner, action);
            self.next.push(Entry::Contribution {
                path: path.to_path_buf(),
                owner: owner.to_string(),
                key: key.to_string(),
                sha256: sha256(&inner),
                separator,
            });
        }

        for entry in stale {
            let Entry::Contribution {
                owner,
                key,
                sha256,
                separator,
                ..
            } = entry
            else {
                continue;
            };
            let blocks = shared::find(&text, key);
            match blocks.as_slice() {
                [] if overwrite => self.not_found(path, key, owner),
                [] => {
                    let reason = Reason::Drifted {
                        owner: owner.clone(),
                    };
                    self.conflict(path, Some(key), reason, None);
                }
                [block] if record::sha256(&text[block.inner.clone()]) == *sha256 || overwrite => {
                    text = shared::remove(&text, block, separator);
                    self.change(path, Some(key), owner, Action::Delete);
                }
                _ if overwrite => {
                    for block in blocks.iter().rev() {
                        text = shared::remove(&text, block, "");
                    }
                    self.change(path, Some(key), owner, Action::Delete);
                }
                [block] => {
                    let on_disk = text[block.inner.clone()].to_vec();
                    let reason = Reason::Drifted {
                        owner: owner.clone(),
                    };
                    self.conflict(path, Some(key), reason, Some(&on_disk));
                }
                _ => {
                    let reason = Reason::Drifted {
                        owner: owner.clone(),
                    };
                    self.conflict(path, Some(key), reason, None);
                }
            }
        }

        let created = !existed
            || old
                .entries
                .iter()
                .any(|e| matches!(e, Entry::CreatedFile { path: p } if p == path));
        self.next
            .retain(|e| !matches!(e, Entry::CreatedFile { path: p } if p == path));
        let remaining = self
            .next
            .iter()
            .any(|e| matches!(e, Entry::Contribution { path: p, .. } if p == path));
        if remaining && created {
            self.next.push(Entry::CreatedFile {
                path: path.to_path_buf(),
            });
        }
        if !remaining && created && text.is_empty() {
            if existed {
                self.ops.push(Op::Delete(target));
            }
        } else if (text != before || !existed) && self.ensure_parent(&target)? {
            self.ops.push(Op::Write {
                path: target,
                bytes: text,
                executable,
            });
        }
        Ok(())
    }

    fn not_found(&mut self, path: &Path, key: &str, owner: &str) {
        let params = [
            ("file", Value::from(path.display().to_string())),
            ("item", Value::from(owner)),
            ("key", Value::from(key)),
        ];
        self.diagnostics.push(Diagnostic {
            id: "acif.install.contribution_not_found",
            params: params
                .into_iter()
                .map(|(name, value)| (name.to_string(), value))
                .collect(),
        });
    }
}
