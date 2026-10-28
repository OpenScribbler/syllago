//! Golden fixtures pinned to a tool version. Each adapter places the test
//! skill and rule from the pinned capmon feed snapshot; the result must
//! equal `tests/golden/<provider>/`: `golden.json` records the tool version,
//! the feed it was rendered from and every placement, and `home/` holds each
//! placed file at its resolved path. A feed change that moves a path the
//! skeleton uses fails here until the fixtures are regenerated with
//! `SYLLAGO_UPDATE_GOLDEN=1` (run through `sandbox-run --in-place`), so the
//! regeneration diff shows what syllago would write differently.

mod common;

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use acif::{Anchor, EntryPoint, Kind, Os, Rendering, Scope};
use adapters::{ADAPTERS, Adapter, CLAUDE_CODE, CODEX, InstallRecords, Placement, place};
use common::{Items, golden_dir, snapshot_feed};
use serde_json::{Value, json};

/// Every placed file, keyed by its anchored path, with a contribution keyed
/// by its shared file's path.
type Tree = BTreeMap<String, Vec<u8>>;

fn tree(placements: &[Placement]) -> Tree {
    let mut tree = Tree::new();
    for placement in placements {
        assert_eq!(placement.target.anchor, Anchor::Home);
        let path = &placement.target.path;
        let files = match &placement.rendering {
            Rendering::Files(files) => files
                .iter()
                .map(|f| {
                    let file = if f.path.is_empty() {
                        path.clone()
                    } else {
                        format!("{}/{}", path.trim_end_matches('/'), f.path)
                    };
                    (file, f.bytes.clone())
                })
                .collect(),
            Rendering::Contribution(bytes) => vec![(path.clone(), bytes.clone())],
        };
        for (path, bytes) in files {
            assert!(tree.insert(path.clone(), bytes).is_none(), "{path} twice");
        }
    }
    tree
}

fn manifest(adapter: &Adapter, feed: &feed::Feed, placed: &[(Kind, &str, Placement)]) -> String {
    let placements: Vec<Value> = placed
        .iter()
        .map(|(kind, name, p)| {
            json!({
                "kind": kind.as_str(),
                "content_name": name,
                "path": p.target.path,
                "layout": p.target.layout.as_str(),
                "providers": p.providers,
                "diagnostics": p.diagnostics.iter().map(|d| d.id).collect::<Vec<_>>(),
            })
        })
        .collect();
    let manifest = json!({
        "provider": adapter.slug,
        "tool_version": adapter.tool_version,
        "tier": format!("{:?}", adapter.tier).to_lowercase(),
        "feed": {
            "index_sha256": feed.index_sha256(),
            "source_commit": feed.source_commit(),
        },
        "scope": "user",
        "placements": placements,
    });
    serde_json::to_string_pretty(&manifest).unwrap() + "\n"
}

/// Places the test skill and rule for `adapter` alone on `os`.
fn render(
    adapter: &Adapter,
    records: &impl InstallRecords,
    items: &Items,
    os: Os,
) -> Vec<(Kind, &'static str, Placement)> {
    let mut placed = Vec::new();
    for (kind, name, source) in items.sources() {
        for placement in place(&[adapter], records, &source, name, Scope::User, os).unwrap() {
            placed.push((kind, name, placement));
        }
    }
    placed
}

fn read_tree(root: &Path) -> Tree {
    fn walk(dir: &Path, prefix: &str, tree: &mut Tree) {
        let mut entries: Vec<_> = std::fs::read_dir(dir)
            .unwrap()
            .map(|e| e.unwrap())
            .collect();
        entries.sort_by_key(|e| e.file_name());
        for entry in entries {
            let name = entry.file_name().into_string().unwrap();
            let path = format!("{prefix}{name}");
            if entry.file_type().unwrap().is_dir() {
                walk(&entry.path(), &format!("{path}/"), tree);
            } else {
                tree.insert(path, std::fs::read(entry.path()).unwrap());
            }
        }
    }
    let mut tree = Tree::new();
    if root.exists() {
        walk(root, "~/", &mut tree);
    }
    tree
}

fn write_golden(dir: &Path, manifest: &str, tree: &Tree) {
    // Nothing is deleted: a file a placement no longer writes stays and
    // fails the path comparison until it is removed by hand.
    std::fs::create_dir_all(dir).unwrap();
    std::fs::write(dir.join("golden.json"), manifest).unwrap();
    for (path, bytes) in tree {
        let file = path
            .strip_prefix("~/")
            .unwrap()
            .split('/')
            .fold(dir.join("home"), |p, s| p.join(s));
        std::fs::create_dir_all(file.parent().unwrap()).unwrap();
        std::fs::write(file, bytes).unwrap();
    }
}

/// Places for `adapter` from `records` and compares the result with its
/// committed fixture, writing the fixture first when `update` is set.
fn compare(
    adapter: &Adapter,
    feed: &feed::Feed,
    records: &impl InstallRecords,
    update: bool,
) -> Result<(), String> {
    let items = Items::load();
    let placed = render(adapter, records, &items, Os::Linux);
    // The user-scope rows the skeleton uses apply on every OS, so every
    // target OS places the same bytes at the same anchored paths.
    for os in Os::ALL {
        assert_eq!(render(adapter, records, &items, os), placed, "{os:?}");
    }
    let placements: Vec<Placement> = placed.iter().map(|(_, _, p)| p.clone()).collect();
    let tree = tree(&placements);
    let manifest = manifest(adapter, feed, &placed);
    let dir = golden_dir().join(adapter.slug);
    if update {
        write_golden(&dir, &manifest, &tree);
    }
    let slug = adapter.slug;
    let recorded = std::fs::read_to_string(dir.join("golden.json")).unwrap_or_default();
    if recorded != manifest {
        return Err(format!("{slug}: golden.json differs:\n{manifest}"));
    }
    let on_disk = read_tree(&dir.join("home"));
    if on_disk.keys().ne(tree.keys()) {
        return Err(format!(
            "{slug}: placed paths differ: {:?} on disk, {:?} placed",
            on_disk.keys().collect::<Vec<_>>(),
            tree.keys().collect::<Vec<_>>()
        ));
    }
    match tree.iter().find(|(path, bytes)| on_disk[*path] != **bytes) {
        Some((path, _)) => Err(format!("{slug}: {path} differs")),
        None => Ok(()),
    }
}

fn check(adapter: &Adapter) {
    let feed = snapshot_feed();
    let update = std::env::var_os("SYLLAGO_UPDATE_GOLDEN").is_some();
    if let Err(diff) = compare(adapter, &feed, &feed, update) {
        panic!("{diff}\nregenerate with SYLLAGO_UPDATE_GOLDEN=1 through sandbox-run --in-place");
    }
}

#[test]
fn claude_code_golden() {
    check(&CLAUDE_CODE);
}

#[test]
fn codex_golden() {
    check(&CODEX);
}

/// Claude Code and Codex share no user-scope location for skills or rules,
/// so targeting both writes each its own copy, the same bytes the
/// per-provider fixtures hold.
#[test]
fn both_providers_together_match_their_fixtures() {
    let feed = snapshot_feed();
    let items = Items::load();
    let mut together = Tree::new();
    let mut apart = Tree::new();
    for (_, name, source) in items.sources() {
        let placements = place(&ADAPTERS, &feed, &source, name, Scope::User, Os::Windows).unwrap();
        for p in &placements {
            assert_eq!(p.providers.len(), 1, "{}", p.target.path);
        }
        together.extend(tree(&placements));
    }
    for adapter in ADAPTERS {
        apart.extend(read_tree(&golden_dir().join(adapter.slug).join("home")));
    }
    assert_eq!(together, apart);
}

/// The snapshot's rows, with one template moved.
struct Moved<'a> {
    feed: &'a feed::Feed,
    rows: Vec<EntryPoint>,
}

impl InstallRecords for Moved<'_> {
    fn entry_points(&self, provider: &str, kind: Kind) -> Option<&[EntryPoint]> {
        if provider == CODEX.slug && kind == Kind::Skill {
            return Some(&self.rows);
        }
        self.feed.entry_points(provider, kind)
    }
}

/// A feed change that moves a path the skeleton uses fails the fixture
/// comparison until the fixture is regenerated.
#[test]
fn a_moved_feed_path_breaks_the_fixture() {
    let feed = snapshot_feed();
    let mut rows = feed.entry_points(CODEX.slug, Kind::Skill).unwrap().to_vec();
    rows[0].path_template = "~/.codex/skills/<content-name>/".into();
    let moved = Moved { feed: &feed, rows };
    let err = compare(&CODEX, &feed, &moved, false).unwrap_err();
    assert!(err.contains("~/.codex/skills/hello/"), "{err}");
    compare(&CODEX, &feed, &feed, false).unwrap();
}

/// Resolved paths map to the native filesystem under the user's home.
#[test]
fn placements_map_to_native_paths() {
    let feed = snapshot_feed();
    let items = Items::load();
    let home = PathBuf::from("home").join("user");
    let roots = acif::Roots {
        home: &home,
        appdata: None,
        project: None,
    };
    let (_, name, source) = items.sources().into_iter().next().unwrap();
    let placement = &place(
        &[&CLAUDE_CODE],
        &feed,
        &source,
        name,
        Scope::User,
        Os::Windows,
    )
    .unwrap()[0];
    assert_eq!(
        placement.target.native(&roots).unwrap(),
        home.join(".claude").join("skills").join("hello")
    );
}
