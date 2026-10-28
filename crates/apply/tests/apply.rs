//! The apply engine end to end on a real filesystem in a temporary home:
//! the test skill and rule placed for Claude Code and Codex from the pinned
//! capmon feed snapshot, applied, checked on disk, and removed. Failures are
//! injected through a filesystem stub that fails one chosen change, and
//! every path any test touches lies under its own temporary directory.

use std::cell::Cell;
use std::collections::BTreeMap;
use std::io;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, SystemTime};

use acif::{Body, Kind, Os, Roots, Scope, ingest_body};
use adapters::{ADAPTERS, Adapter, CLAUDE_CODE, CODEX, Source, feed_providers, place};
use apply::{
    Action, Engine, Entry, Error, Fs, ManualStep, Node, Options, Placed, RealFs, Reason, Rollback,
};

/// The test rule as both providers render it.
const RULE: &[u8] =
    b"Write in plain sentences. Prefer the active voice, and keep each paragraph\nto one idea.\n";

struct TempDir(PathBuf);

impl TempDir {
    fn new(label: &str) -> TempDir {
        static COUNT: AtomicUsize = AtomicUsize::new(0);
        let n = COUNT.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("apply-test-{}-{n}-{label}", std::process::id()));
        std::fs::create_dir_all(path.join("home")).unwrap();
        std::fs::create_dir_all(path.join("state")).unwrap();
        TempDir(path)
    }

    fn home(&self) -> PathBuf {
        self.0.join("home")
    }

    fn state(&self) -> PathBuf {
        self.0.join("state")
    }

    fn write(&self, relative: &str, bytes: &[u8]) {
        let path = self.home().join(relative);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, bytes).unwrap();
    }

    fn read(&self, relative: &str) -> Vec<u8> {
        std::fs::read(self.home().join(relative)).unwrap()
    }
}

impl Drop for TempDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn snapshot_feed() -> feed::Feed {
    let dir =
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../feed/tests/fixtures/capmon-2026-10-28");
    let now = SystemTime::UNIX_EPOCH + Duration::from_secs(1_793_232_000);
    feed::load(
        &feed::DirSource::new(dir),
        &moat::TrustRoot::bundled(),
        now,
        &feed_providers(&ADAPTERS),
    )
    .unwrap()
}

fn golden_item(name: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../adapters/tests/golden/items")
        .join(name)
}

/// The test skill, copied under `dir` with its script made executable where
/// the filesystem records the bit.
fn skill_body(dir: &TempDir) -> Body {
    let root = dir.0.join("items/hello");
    std::fs::create_dir_all(root.join("scripts")).unwrap();
    for file in ["SKILL.md", "scripts/greet.sh"] {
        std::fs::copy(golden_item("hello").join(file), root.join(file)).unwrap();
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let script = root.join("scripts/greet.sh");
        std::fs::set_permissions(script, std::fs::Permissions::from_mode(0o755)).unwrap();
    }
    Body::read(&root).unwrap()
}

/// Places each `(owner, name, kind, body, entry file)` for `adapters`.
fn placements(adapters: &[&Adapter], items: &[(&str, &str, Kind, &Body, &str)]) -> Vec<Placed> {
    let feed = snapshot_feed();
    let mut placed = Vec::new();
    for &(owner, name, kind, body, entry_file) in items {
        let item = ingest_body(kind, body, entry_file).unwrap();
        let source = Source {
            item: &item,
            body,
            entry_file,
        };
        for placement in place(adapters, &feed, &source, name, Scope::User, Os::Linux).unwrap() {
            placed.push(Placed {
                owner: owner.to_string(),
                name: name.to_string(),
                placement,
            });
        }
    }
    placed
}

/// The skill and the rule, for both providers.
fn both(dir: &TempDir) -> Vec<Placed> {
    let skill = skill_body(dir);
    let rule = Body::read(&golden_item("style")).unwrap();
    placements(
        &[&CLAUDE_CODE, &CODEX],
        &[
            ("skill-hello", "hello", Kind::Skill, &skill, "SKILL.md"),
            ("rule-style", "style", Kind::Rule, &rule, "style.md"),
        ],
    )
}

/// Every directory and file under the home directory, with each file's
/// bytes and executable bit.
type Tree = BTreeMap<String, Option<(Vec<u8>, bool)>>;

fn tree(dir: &TempDir) -> Tree {
    fn walk(path: &Path, prefix: &str, tree: &mut Tree) {
        for entry in std::fs::read_dir(path).unwrap() {
            let entry = entry.unwrap();
            let name = format!("{prefix}{}", entry.file_name().to_str().unwrap());
            match RealFs.read(&entry.path()).unwrap() {
                Node::Dir => {
                    tree.insert(format!("{name}/"), None);
                    walk(&entry.path(), &format!("{name}/"), tree);
                }
                Node::File { bytes, executable } => {
                    tree.insert(name, Some((bytes, executable)));
                }
                Node::Link { .. } => {
                    let target = std::fs::read_link(entry.path()).unwrap();
                    let target = format!("-> {}", target.display());
                    tree.insert(name, Some((target.into_bytes(), false)));
                }
                other => panic!("{name}: {other:?}"),
            }
        }
    }
    let mut tree = Tree::new();
    walk(&dir.home(), "", &mut tree);
    tree
}

fn engine<'a, F: Fs>(fs: &'a F, dir: &'a TempDir, home: &'a Path) -> Engine<'a, F> {
    let roots = Roots {
        home,
        appdata: None,
        project: None,
    };
    Engine::new(fs, roots, Box::leak(dir.state().into_boxed_path()))
}

/// User files already in the home directory before any apply: one with a
/// shared file to contribute to, one where the apply creates it.
fn user_files(dir: &TempDir, with_agents_md: bool) {
    dir.write(".claude/settings.json", b"{}\n");
    dir.write(
        ".claude/skills/mine/SKILL.md",
        b"---\nname: mine\n---\nMine.\n",
    );
    if with_agents_md {
        // No final newline: the separator must come back out exactly.
        dir.write(".codex/AGENTS.md", b"# My rules\n\nBe brief.");
    }
}

/// Fails the `fail_at`th change (counting from 0) and, when `sticky`, every
/// change after it, so a rollback fails too.
struct FailingFs {
    fail_at: usize,
    sticky: bool,
    count: Cell<usize>,
}

impl FailingFs {
    fn new(fail_at: usize, sticky: bool) -> FailingFs {
        FailingFs {
            fail_at,
            sticky,
            count: Cell::new(0),
        }
    }

    fn tick(&self) -> io::Result<()> {
        let n = self.count.get();
        self.count.set(n + 1);
        if n == self.fail_at || (self.sticky && n > self.fail_at) {
            return Err(io::Error::other("injected failure"));
        }
        Ok(())
    }
}

impl Fs for FailingFs {
    fn read(&self, path: &Path) -> io::Result<Node> {
        RealFs.read(path)
    }
    fn write(&self, path: &Path, bytes: &[u8], executable: bool) -> io::Result<()> {
        self.tick()?;
        RealFs.write(path, bytes, executable)
    }
    fn remove_file(&self, path: &Path) -> io::Result<()> {
        self.tick()?;
        RealFs.remove_file(path)
    }
    fn create_dir(&self, path: &Path) -> io::Result<()> {
        self.tick()?;
        RealFs.create_dir(path)
    }
    fn remove_dir(&self, path: &Path) -> io::Result<()> {
        self.tick()?;
        RealFs.remove_dir(path)
    }
}

#[test]
fn apply_to_both_providers_then_remove_restores_the_tree() {
    for with_agents_md in [true, false] {
        let dir = TempDir::new("roundtrip");
        user_files(&dir, with_agents_md);
        let before = tree(&dir);
        let home = dir.home();
        let engine = engine(&RealFs, &dir, &home);
        let placed = both(&dir);

        // The preview lists every file and contribution and writes nothing.
        let plan = engine.preview_apply(&placed, Options::default()).unwrap();
        assert!(plan.conflicts.is_empty());
        let mut previewed: Vec<_> = plan
            .changes
            .iter()
            .map(|c| {
                let path = c.path.strip_prefix(&home).unwrap();
                let path = path.to_str().unwrap().replace('\\', "/"); // path-literal-ok: compared as text only
                (path, c.key.clone(), c.action)
            })
            .collect();
        previewed.sort();
        let create = |path: &str, key: Option<&str>| {
            (path.to_string(), key.map(str::to_string), Action::Create)
        };
        assert_eq!(
            previewed,
            [
                create(".agents/skills/hello/SKILL.md", None),
                create(".agents/skills/hello/scripts/greet.sh", None),
                create(".claude/rules/style.md", None),
                create(".claude/skills/hello/SKILL.md", None),
                create(".claude/skills/hello/scripts/greet.sh", None),
                create(".codex/AGENTS.md", Some("style")),
            ]
        );
        assert_eq!(tree(&dir), before);

        engine.apply(&placed, Options::default()).unwrap();
        let skill = std::fs::read(golden_item("hello").join("SKILL.md")).unwrap();
        assert_eq!(dir.read(".claude/skills/hello/SKILL.md"), skill);
        assert_eq!(dir.read(".agents/skills/hello/SKILL.md"), skill);
        assert_eq!(dir.read(".claude/rules/style.md"), RULE);
        let agents = dir.read(".codex/AGENTS.md");
        let block = [
            &b"<!-- syllago:begin style -->\n"[..],
            RULE,
            b"<!-- syllago:end style -->\n",
        ]
        .concat();
        let expected = if with_agents_md {
            [&b"# My rules\n\nBe brief.\n\n"[..], &block].concat()
        } else {
            block
        };
        assert_eq!(agents, expected);
        assert!(!engine.snapshot_path().exists());
        let record = engine.record().unwrap();
        let owned = |kind: fn(&Entry) -> bool| record.entries.iter().filter(|e| kind(e)).count();
        assert_eq!(owned(|e| matches!(e, Entry::File { .. })), 5);
        assert_eq!(owned(|e| matches!(e, Entry::Contribution { .. })), 1);
        assert!(engine.drift().unwrap().is_empty());

        // Applying again changes nothing.
        let again = engine.apply(&placed, Options::default()).unwrap();
        assert!(again.changes.iter().all(|c| c.action == Action::Unchanged));

        engine
            .remove(&["skill-hello", "rule-style"], Options::default())
            .unwrap();
        assert_eq!(tree(&dir), before, "with AGENTS.md: {with_agents_md}");
        assert!(engine.record().unwrap().entries.is_empty());
    }
}

/// Fails each change of an apply, then each change of a remove, in turn:
/// every failure leaves the home directory and the record as they were.
#[test]
fn failure_at_each_step_rolls_back_byte_for_byte() {
    let mut steps = 0;
    for fail_at in 0.. {
        let dir = TempDir::new("fail-apply");
        user_files(&dir, true);
        let before = tree(&dir);
        let home = dir.home();
        let fs = FailingFs::new(fail_at, false);
        let engine = engine(&fs, &dir, &home);
        match engine.apply(&both(&dir), Options::default()) {
            Ok(_) => break,
            Err(Error::Failed { rollback, .. }) => {
                assert_eq!(rollback, Rollback::Complete, "step {fail_at}");
                // Files the apply created are gone again, the shared file
                // has its bytes back, and no record was written.
                assert_eq!(tree(&dir), before, "step {fail_at}");
                assert!(!engine.record_path().exists(), "step {fail_at}");
                assert!(!engine.snapshot_path().exists(), "step {fail_at}");
            }
            Err(e) => panic!("step {fail_at}: {e}"),
        }
        steps += 1;
    }
    // Snapshot, directories, files, the shared file and the record.
    assert!(steps > 10, "{steps}");

    let mut steps = 0;
    for fail_at in 0.. {
        let dir = TempDir::new("fail-remove");
        user_files(&dir, true);
        let home = dir.home();
        engine(&RealFs, &dir, &home)
            .apply(&both(&dir), Options::default())
            .unwrap();
        let applied = tree(&dir);
        let record = std::fs::read(engine(&RealFs, &dir, &home).record_path()).unwrap();
        let fs = FailingFs::new(fail_at, false);
        let engine = engine(&fs, &dir, &home);
        match engine.remove(&["skill-hello", "rule-style"], Options::default()) {
            Ok(_) => break,
            Err(Error::Failed { rollback, .. }) => {
                assert_eq!(rollback, Rollback::Complete, "step {fail_at}");
                assert_eq!(tree(&dir), applied, "step {fail_at}");
                assert_eq!(std::fs::read(engine.record_path()).unwrap(), record);
            }
            Err(e) => panic!("step {fail_at}: {e}"),
        }
        steps += 1;
    }
    assert!(steps > 10, "{steps}");
}

/// A rollback whose own changes fail keeps the snapshot and names each step
/// left; doing those by hand restores the home directory byte for byte.
#[test]
fn rollback_that_cannot_finish_keeps_the_snapshot_and_says_what_to_restore() {
    for fail_at in 1.. {
        let dir = TempDir::new("stuck");
        user_files(&dir, true);
        let before = tree(&dir);
        let home = dir.home();
        let fs = FailingFs::new(fail_at, true);
        let engine = engine(&fs, &dir, &home);
        let (snapshot, steps) = match engine.apply(&both(&dir), Options::default()) {
            Ok(_) => break,
            Err(Error::Failed {
                rollback: Rollback::Incomplete { snapshot, steps },
                ..
            }) => (snapshot, steps),
            Err(e) => panic!("step {fail_at}: {e}"),
        };
        assert!(snapshot.exists(), "step {fail_at}");
        // A later run refuses until the snapshot is dealt with.
        let next = Engine::new(
            &RealFs,
            Roots {
                home: &home,
                appdata: None,
                project: None,
            },
            &dir.state(),
        )
        .apply(&both(&dir), Options::default());
        assert!(
            matches!(next, Err(Error::SnapshotPending(_))),
            "step {fail_at}"
        );

        for step in steps {
            match step {
                ManualStep::Restore {
                    path,
                    bytes,
                    executable,
                } => RealFs.write(&path, &bytes, executable).unwrap(),
                ManualStep::Delete { path } => std::fs::remove_file(path).unwrap(),
                ManualStep::RemoveDir { path } => std::fs::remove_dir(path).unwrap(),
                ManualStep::CreateDir { path } => std::fs::create_dir(path).unwrap(),
            }
        }
        assert_eq!(tree(&dir), before, "step {fail_at}");
    }
}

/// Two rules whose rendered bytes are identical both go into Codex's
/// `AGENTS.md`; removing one leaves exactly the other's entry.
#[test]
fn removing_one_of_two_identical_contributions_leaves_the_other() {
    let rule = Body::read(&golden_item("style")).unwrap();
    let style = ("rule-style", "style", Kind::Rule, &rule, "style.md");
    let tone = ("rule-tone", "tone", Kind::Rule, &rule, "style.md");

    let alone = TempDir::new("tone-alone");
    user_files(&alone, true);
    let home = alone.home();
    engine(&RealFs, &alone, &home)
        .apply(&placements(&[&CODEX], &[tone]), Options::default())
        .unwrap();

    let dir = TempDir::new("two");
    user_files(&dir, true);
    let before = tree(&dir);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    engine
        .apply(&placements(&[&CODEX], &[style, tone]), Options::default())
        .unwrap();
    engine.remove(&["rule-style"], Options::default()).unwrap();
    // Exactly the other item's entry is left, with the user's text before
    // it; only the blank line between them may differ from a file the
    // other item went into alone.
    let left = dir.read(".codex/AGENTS.md");
    let normalize = |bytes: Vec<u8>| {
        String::from_utf8(bytes)
            .unwrap()
            .replace("\n\n<!--", "\n<!--")
    };
    assert_eq!(normalize(left), normalize(alone.read(".codex/AGENTS.md")));
    // Re-applying the remaining item touches nothing.
    let again = engine
        .apply(&placements(&[&CODEX], &[tone]), Options::default())
        .unwrap();
    assert_eq!(again.changes[0].action, Action::Unchanged);
    engine.remove(&["rule-tone"], Options::default()).unwrap();
    assert_eq!(tree(&dir), before);
}

#[test]
fn hand_edited_owned_file_blocks_apply_until_overwrite() {
    let dir = TempDir::new("drift");
    user_files(&dir, true);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let placed = both(&dir);
    engine.apply(&placed, Options::default()).unwrap();
    let rendered = dir.read(".claude/skills/hello/SKILL.md");

    dir.write(".claude/skills/hello/SKILL.md", b"my fix\n");
    let edited = tree(&dir);
    let drift = engine.drift().unwrap();
    assert_eq!(drift.len(), 1);
    assert_eq!(drift[0].path, home.join(".claude/skills/hello/SKILL.md"));
    assert_eq!(
        drift[0].reason,
        Reason::Drifted {
            owner: "skill-hello".into()
        }
    );
    assert_eq!(drift[0].current.as_deref(), Some(&b"my fix\n"[..]));

    let Err(error @ Error::Blocked(_)) = engine.apply(&placed, Options::default()) else {
        panic!("applied over a hand edit");
    };
    assert!(error.to_string().contains("SKILL.md"));
    let Error::Blocked(conflicts) = error else {
        unreachable!()
    };
    assert_eq!(conflicts, drift);
    assert_eq!(tree(&dir), edited);
    // Remove stops on it too.
    assert!(matches!(
        engine.remove(&["skill-hello"], Options::default()),
        Err(Error::Blocked(_))
    ));

    let overwrite = Options { overwrite: true };
    engine.apply(&placed, overwrite).unwrap();
    assert_eq!(dir.read(".claude/skills/hello/SKILL.md"), rendered);
    assert!(engine.drift().unwrap().is_empty());

    // An edit inside an owned contribution names the file and its key.
    let agents = dir.read(".codex/AGENTS.md");
    let edited = String::from_utf8(agents)
        .unwrap()
        .replace("plain sentences", "long sentences");
    dir.write(".codex/AGENTS.md", edited.as_bytes());
    let Err(Error::Blocked(conflicts)) = engine.apply(&placed, Options::default()) else {
        panic!("applied over a hand edit");
    };
    assert_eq!(conflicts[0].path, home.join(".codex/AGENTS.md"));
    assert_eq!(conflicts[0].key.as_deref(), Some("style"));
    assert_eq!(
        conflicts[0].current.as_deref(),
        Some(
            &String::from_utf8(RULE.to_vec())
                .unwrap()
                .replace("plain", "long")
                .into_bytes()[..]
        )
    );
    engine.apply(&placed, overwrite).unwrap();
    assert!(engine.drift().unwrap().is_empty());
}

#[test]
fn remove_never_deletes_a_file_the_user_created() {
    let dir = TempDir::new("user-files");
    user_files(&dir, true);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    engine.apply(&both(&dir), Options::default()).unwrap();
    dir.write(".claude/skills/hello/notes.md", b"mine\n");
    engine
        .remove(&["skill-hello", "rule-style"], Options::default())
        .unwrap();
    let left = tree(&dir);
    assert_eq!(
        left.get(".claude/skills/hello/notes.md"),
        Some(&Some((b"mine\n".to_vec(), false)))
    );
    assert!(left.contains_key(".claude/skills/hello/"));
    assert!(!left.contains_key(".claude/skills/hello/SKILL.md"));
    assert!(!left.contains_key(".claude/skills/hello/scripts/"));
    assert!(!left.contains_key(".agents/"));
}

/// A file the user put where a placement goes stops the apply; with
/// `--overwrite` it is replaced, and remove puts it back.
#[test]
fn a_users_file_in_the_way_blocks_apply_and_is_restored_after_overwrite() {
    let dir = TempDir::new("unowned");
    user_files(&dir, true);
    dir.write(".claude/rules/style.md", b"my own style\n");
    let before = tree(&dir);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let placed = both(&dir);
    let Err(Error::Blocked(conflicts)) = engine.apply(&placed, Options::default()) else {
        panic!("replaced a user's file");
    };
    assert_eq!(conflicts.len(), 1);
    assert_eq!(conflicts[0].reason, Reason::Unowned);
    assert_eq!(conflicts[0].path, home.join(".claude/rules/style.md"));
    assert_eq!(tree(&dir), before);

    let outcome = engine.apply(&placed, Options { overwrite: true }).unwrap();
    assert!(outcome.changes.iter().any(|c| c.action == Action::Update));
    assert_eq!(dir.read(".claude/rules/style.md"), RULE);
    let plan = engine
        .preview_remove(&["skill-hello", "rule-style"], Options::default())
        .unwrap();
    assert!(
        plan.changes
            .iter()
            .any(|c| c.path.ends_with("style.md") && c.action == Action::Restore)
    );
    engine
        .remove(&["skill-hello", "rule-style"], Options::default())
        .unwrap();
    assert_eq!(tree(&dir), before);
}

/// Re-applying an item that no longer places a file removes that file.
#[test]
fn reapply_removes_what_the_item_no_longer_places() {
    let dir = TempDir::new("stale");
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let skill = skill_body(&dir);
    let item = ("skill-hello", "hello", Kind::Skill, &skill, "SKILL.md");
    engine
        .apply(&placements(&[&CODEX], &[item]), Options::default())
        .unwrap();
    std::fs::remove_file(dir.0.join("items/hello/scripts/greet.sh")).unwrap();
    std::fs::remove_dir(dir.0.join("items/hello/scripts")).unwrap();
    let smaller = Body::read(&dir.0.join("items/hello")).unwrap();
    let item = ("skill-hello", "hello", Kind::Skill, &smaller, "SKILL.md");
    let outcome = engine
        .apply(&placements(&[&CODEX], &[item]), Options::default())
        .unwrap();
    assert!(
        outcome
            .changes
            .iter()
            .any(|c| c.path.ends_with("greet.sh") && c.action == Action::Delete)
    );
    let left = tree(&dir);
    assert!(left.contains_key(".agents/skills/hello/SKILL.md"));
    assert!(!left.contains_key(".agents/skills/hello/scripts/"));
}

#[cfg(unix)]
#[test]
fn executable_bit_is_carried_to_every_provider() {
    let dir = TempDir::new("exec");
    let home = dir.home();
    engine(&RealFs, &dir, &home)
        .apply(&both(&dir), Options::default())
        .unwrap();
    let tree = tree(&dir);
    for root in [".claude/skills/hello/", ".agents/skills/hello/"] {
        let executable = |file: &str| tree[&format!("{root}{file}")].as_ref().unwrap().1;
        assert!(executable("scripts/greet.sh"), "{root}");
        assert!(!executable("SKILL.md"), "{root}");
    }
}

#[test]
fn two_items_writing_one_file_collide() {
    let dir = TempDir::new("collide");
    let home = dir.home();
    let skill = skill_body(&dir);
    let placed = placements(
        &[&CODEX],
        &[
            ("one", "hello", Kind::Skill, &skill, "SKILL.md"),
            ("two", "hello", Kind::Skill, &skill, "SKILL.md"),
        ],
    );
    let result = engine(&RealFs, &dir, &home).apply(&placed, Options::default());
    assert!(matches!(result, Err(Error::Collision { .. })));
}

/// A contribution the user deleted by hand stops remove as drift; with
/// `--overwrite` it is already removed: remove discloses it, touches
/// nothing else in the file, and forgets it.
#[test]
fn remove_discloses_a_contribution_that_is_gone() {
    let dir = TempDir::new("gone");
    user_files(&dir, true);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let rule = Body::read(&golden_item("style")).unwrap();
    let style = ("rule-style", "style", Kind::Rule, &rule, "style.md");
    engine
        .apply(&placements(&[&CODEX], &[style]), Options::default())
        .unwrap();
    dir.write(".codex/AGENTS.md", b"rewritten by hand\n");
    assert!(matches!(
        engine.remove(&["rule-style"], Options::default()),
        Err(Error::Blocked(_))
    ));
    let outcome = engine
        .remove(&["rule-style"], Options { overwrite: true })
        .unwrap();
    assert_eq!(
        outcome.diagnostics[0].id,
        "acif.install.contribution_not_found"
    );
    assert_eq!(outcome.diagnostics[0].params["item"], "rule-style");
    assert_eq!(dir.read(".codex/AGENTS.md"), b"rewritten by hand\n");
    assert!(engine.record().unwrap().entries.is_empty());
}

#[cfg(unix)]
#[test]
fn real_write_keeps_permissions_and_leaves_no_temporary_file() {
    use std::os::unix::fs::PermissionsExt;
    let dir = TempDir::new("realfs");
    let path = dir.home().join("private.md");
    std::fs::write(&path, b"old\n").unwrap();
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600)).unwrap();
    RealFs.write(&path, b"new\r\n", false).unwrap();
    assert_eq!(std::fs::read(&path).unwrap(), b"new\r\n");
    let mode = |p: &Path| std::fs::metadata(p).unwrap().permissions().mode() & 0o777;
    assert_eq!(mode(&path), 0o600);
    RealFs.write(&path, b"new\n", true).unwrap();
    assert_eq!(mode(&path), 0o700);
    let names: Vec<_> = std::fs::read_dir(dir.home())
        .unwrap()
        .map(|e| e.unwrap().file_name())
        .collect();
    assert_eq!(names, ["private.md"]);
}

/// A file the user keeps where an older build put its temporary file is
/// neither truncated nor renamed away.
#[test]
fn a_users_file_at_a_temporary_name_survives_a_write() {
    let dir = TempDir::new("temp-name");
    user_files(&dir, true);
    dir.write(".codex/.AGENTS.md.syllago-tmp", b"my backup\n");
    dir.write(".claude/rules/.style.md.syllago-tmp", b"my other backup\n");
    let home = dir.home();
    engine(&RealFs, &dir, &home)
        .apply(&both(&dir), Options::default())
        .unwrap();
    assert_eq!(dir.read(".codex/.AGENTS.md.syllago-tmp"), b"my backup\n");
    assert_eq!(
        dir.read(".claude/rules/.style.md.syllago-tmp"),
        b"my other backup\n"
    );
}

/// The file `--overwrite` replaced comes back on remove even when the
/// placed file, or its whole directory, was deleted by hand.
#[test]
fn remove_restores_a_replaced_file_whatever_is_there_now() {
    for delete_dir in [false, true] {
        let dir = TempDir::new("prior");
        user_files(&dir, true);
        dir.write(".claude/rules/style.md", b"my original\n");
        let before = tree(&dir);
        let home = dir.home();
        let engine = engine(&RealFs, &dir, &home);
        engine
            .apply(&both(&dir), Options { overwrite: true })
            .unwrap();
        if delete_dir {
            std::fs::remove_dir_all(home.join(".claude/rules")).unwrap();
        } else {
            std::fs::remove_file(home.join(".claude/rules/style.md")).unwrap();
        }
        engine
            .remove(&["skill-hello", "rule-style"], Options::default())
            .unwrap();
        assert_eq!(tree(&dir), before, "directory deleted: {delete_dir}");
    }
}

/// When the replaced file cannot be put back, the run stops and the record
/// keeps it, even with `--overwrite`.
#[test]
fn a_replaced_file_that_cannot_be_restored_is_kept() {
    let dir = TempDir::new("prior-kept");
    user_files(&dir, true);
    dir.write(".claude/rules/style.md", b"my original\n");
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    engine
        .apply(&both(&dir), Options { overwrite: true })
        .unwrap();
    std::fs::remove_file(home.join(".claude/rules/style.md")).unwrap();
    std::fs::create_dir(home.join(".claude/rules/style.md")).unwrap();
    for overwrite in [false, true] {
        let result = engine.remove(&["skill-hello", "rule-style"], Options { overwrite });
        let Err(Error::Blocked(conflicts)) = result else {
            panic!("removed with the original unrestorable");
        };
        assert_eq!(conflicts[0].path, home.join(".claude/rules/style.md"));
        assert_eq!(conflicts[0].reason, Reason::NotAFile);
    }
    let kept =
        engine.record().unwrap().entries.into_iter().any(
            |e| matches!(e, Entry::File { prior: Some(p), .. } if p.bytes == b"my original\n"),
        );
    assert!(kept);
}

/// A damaged marker around an owned contribution is a hand edit: apply and
/// remove stop on it, and `--overwrite` leaves one block.
#[test]
fn a_damaged_marker_is_drift() {
    let dir = TempDir::new("marker");
    user_files(&dir, true);
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let placed = both(&dir);
    engine.apply(&placed, Options::default()).unwrap();
    let agents = String::from_utf8(dir.read(".codex/AGENTS.md")).unwrap();
    let damaged = agents.replace("<!-- syllago:end style -->", "<!-- end of style -->");
    dir.write(".codex/AGENTS.md", damaged.as_bytes());

    assert_eq!(engine.drift().unwrap().len(), 1);
    let Err(Error::Blocked(conflicts)) = engine.apply(&placed, Options::default()) else {
        panic!("applied over a damaged marker");
    };
    assert_eq!(conflicts[0].path, home.join(".codex/AGENTS.md"));
    assert_eq!(conflicts[0].key.as_deref(), Some("style"));
    assert!(matches!(
        engine.remove(&["rule-style"], Options::default()),
        Err(Error::Blocked(_))
    ));
    // With --overwrite a fresh block goes in. Where the damaged one ends
    // cannot be told, so its text is left for the user rather than guessed
    // at, and it no longer reads as a block.
    let outcome = engine.apply(&placed, Options { overwrite: true }).unwrap();
    assert_eq!(
        outcome.diagnostics[0].id,
        "acif.install.contribution_not_found"
    );
    let agents = String::from_utf8(dir.read(".codex/AGENTS.md")).unwrap();
    assert!(agents.starts_with(&damaged));
    assert!(engine.drift().unwrap().is_empty());
    engine.remove(&["rule-style"], Options::default()).unwrap();
    assert_eq!(dir.read(".codex/AGENTS.md"), damaged.as_bytes());
}

#[cfg(unix)]
#[test]
fn a_changed_executable_bit_is_drift() {
    use std::os::unix::fs::PermissionsExt;
    let dir = TempDir::new("chmod");
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let placed = both(&dir);
    engine.apply(&placed, Options::default()).unwrap();
    let script = home.join(".claude/skills/hello/scripts/greet.sh");
    std::fs::set_permissions(&script, std::fs::Permissions::from_mode(0o644)).unwrap();

    let drift = engine.drift().unwrap();
    assert_eq!(drift.len(), 1);
    assert_eq!(drift[0].path, script);
    assert!(matches!(
        engine.apply(&placed, Options::default()),
        Err(Error::Blocked(_))
    ));
    assert!(matches!(
        engine.remove(&["skill-hello"], Options::default()),
        Err(Error::Blocked(_))
    ));
    engine.apply(&placed, Options { overwrite: true }).unwrap();
    assert!(engine.drift().unwrap().is_empty());
    assert_ne!(
        std::fs::metadata(&script).unwrap().permissions().mode() & 0o111,
        0
    );
}

/// A deleted owned file loses no edit, so apply recreates it without
/// `--overwrite`; drift still reports it until then.
#[test]
fn a_deleted_owned_file_is_reported_and_recreated() {
    let dir = TempDir::new("deleted");
    let home = dir.home();
    let engine = engine(&RealFs, &dir, &home);
    let placed = both(&dir);
    engine.apply(&placed, Options::default()).unwrap();
    let applied = tree(&dir);
    std::fs::remove_file(home.join(".agents/skills/hello/SKILL.md")).unwrap();
    let drift = engine.drift().unwrap();
    assert_eq!(drift.len(), 1);
    assert_eq!(drift[0].current, None);
    engine.apply(&placed, Options::default()).unwrap();
    assert_eq!(tree(&dir), applied);
}

/// A run that stops at any step, as a crash would, with nothing after it
/// done: recover returns the home directory and the record to exactly
/// before the run, or, when the run had written its record, to after it.
/// Covers an apply that replaces a user's file and a remove that restores
/// it, where the saved original lives only in the snapshot mid-run.
#[test]
fn recover_after_a_crash_at_any_step() {
    let overwrite = Options { overwrite: true };
    let setup = |dir: &TempDir| {
        user_files(dir, true);
        dir.write(".claude/rules/style.md", b"my original\n");
    };
    let state = |dir: &TempDir| {
        let record = std::fs::read(dir.state().join("ownership.json")).ok();
        (tree(dir), record)
    };
    for removing in [false, true] {
        let mut crashes = 0;
        for fail_at in 0.. {
            let dir = TempDir::new("crash");
            setup(&dir);
            let home = dir.home();
            if removing {
                engine(&RealFs, &dir, &home)
                    .apply(&both(&dir), overwrite)
                    .unwrap();
            }
            let before = state(&dir);
            // The same run with no failure, for its end state.
            let clean = TempDir::new("crash-clean");
            setup(&clean);
            let clean_home = clean.home();
            let clean_engine = engine(&RealFs, &clean, &clean_home);
            clean_engine.apply(&both(&clean), overwrite).unwrap();
            if removing {
                clean_engine
                    .remove(&["skill-hello", "rule-style"], overwrite)
                    .unwrap();
            }

            let fs = FailingFs::new(fail_at, true);
            let crashed = engine(&fs, &dir, &home);
            let result = if removing {
                crashed.remove(&["skill-hello", "rule-style"], overwrite)
            } else {
                crashed.apply(&both(&dir), overwrite)
            };
            let engine = engine(&RealFs, &dir, &home);
            if dir.state().join("snapshot.json").exists() {
                let next = engine.apply(&both(&dir), overwrite);
                let Err(pending @ Error::SnapshotPending(_)) = next else {
                    panic!("step {fail_at}: ran over a pending snapshot");
                };
                assert!(pending.to_string().contains("recover"));
            }
            assert_eq!(
                engine.recover().unwrap(),
                Rollback::Complete,
                "step {fail_at}"
            );
            assert!(!dir.state().join("snapshot.json").exists());
            let after = state(&dir);
            match result {
                Err(_) => assert_eq!(after, before, "step {fail_at}"),
                Ok(_) => {
                    // The last step, deleting the snapshot, is all that
                    // failed: the run stands.
                    let (tree_after, record_after) = after;
                    let rel = |t: Tree| t;
                    assert_eq!(rel(tree_after), tree(&clean), "step {fail_at}");
                    assert!(record_after.is_some());
                    break;
                }
            }
            crashes += 1;
        }
        assert!(crashes > 10, "{crashes}");
    }
}

#[test]
fn rules_whose_names_differ_only_by_case_collide() {
    let dir = TempDir::new("case");
    let home = dir.home();
    let rule = Body::read(&golden_item("style")).unwrap();
    let placed = placements(
        &[&CLAUDE_CODE],
        &[
            ("upper", "Style", Kind::Rule, &rule, "style.md"),
            ("lower", "style", Kind::Rule, &rule, "style.md"),
        ],
    );
    let result = engine(&RealFs, &dir, &home).apply(&placed, Options::default());
    let Err(error @ Error::CaseCollision { .. }) = result else {
        panic!("applied two paths that are one file on macOS and Windows");
    };
    assert!(error.to_string().contains("differ only by case"));
    assert_eq!(tree(&dir), Tree::new());
}

/// A shared file kept as a symlink into a dotfiles checkout is written
/// through: the link stays, its target takes the contribution, and remove
/// gives the target its bytes back.
#[cfg(unix)]
#[test]
fn a_symlinked_shared_file_is_written_through() {
    let dir = TempDir::new("link");
    let home = dir.home();
    dir.write("dotfiles/AGENTS.md", b"# Mine\n");
    std::fs::create_dir_all(home.join(".codex")).unwrap();
    std::os::unix::fs::symlink(
        home.join("dotfiles/AGENTS.md"),
        home.join(".codex/AGENTS.md"),
    )
    .unwrap();
    let before = tree(&dir);
    let engine = engine(&RealFs, &dir, &home);
    let rule = Body::read(&golden_item("style")).unwrap();
    let placed = placements(
        &[&CODEX],
        &[("rule-style", "style", Kind::Rule, &rule, "style.md")],
    );
    engine.apply(&placed, Options::default()).unwrap();
    assert!(
        std::fs::symlink_metadata(home.join(".codex/AGENTS.md"))
            .unwrap()
            .file_type()
            .is_symlink()
    );
    let target = String::from_utf8(dir.read("dotfiles/AGENTS.md")).unwrap();
    assert!(target.starts_with("# Mine\n\n<!-- syllago:begin style -->\n"));
    assert!(engine.drift().unwrap().is_empty());
    engine.remove(&["rule-style"], Options::default()).unwrap();
    assert_eq!(tree(&dir), before);
}

#[cfg(unix)]
#[test]
fn a_dangling_or_directory_symlink_for_a_shared_file_is_refused() {
    let dir = TempDir::new("bad-link");
    let home = dir.home();
    std::fs::create_dir_all(home.join(".codex")).unwrap();
    std::os::unix::fs::symlink(home.join("nowhere"), home.join(".codex/AGENTS.md")).unwrap();
    let rule = Body::read(&golden_item("style")).unwrap();
    let placed = placements(
        &[&CODEX],
        &[("rule-style", "style", Kind::Rule, &rule, "style.md")],
    );
    let engine = engine(&RealFs, &dir, &home);
    let Err(Error::Blocked(conflicts)) = engine.apply(&placed, Options { overwrite: true }) else {
        panic!("wrote through a dangling link");
    };
    assert_eq!(conflicts[0].reason, Reason::Symlink { target: None });
    assert!(conflicts[0].to_string().contains("points at nothing"));

    // A link to a FIFO resolves, but not to a regular file.
    let fifo = home.join("pipe");
    assert!(
        std::process::Command::new("mkfifo")
            .arg(&fifo)
            .status()
            .unwrap()
            .success()
    );
    std::fs::remove_file(home.join(".codex/AGENTS.md")).unwrap();
    std::os::unix::fs::symlink(&fifo, home.join(".codex/AGENTS.md")).unwrap();
    let Err(Error::Blocked(conflicts)) = engine.apply(&placed, Options::default()) else {
        panic!("wrote through a link to a pipe");
    };
    assert!(matches!(
        conflicts[0].reason,
        Reason::Symlink { target: Some(_) }
    ));
    assert!(conflicts[0].to_string().contains("not a regular file"));
}
