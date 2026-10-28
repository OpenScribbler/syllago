//! Placement across several providers: a location every targeted provider
//! reads takes the item once, a declared cross-read counts as a read, and a
//! provider that would load the item twice is refused. The providers here
//! are made up, with rows built in the test, because no two of the shipped
//! adapters share a user-scope location.

mod common;

use std::collections::BTreeMap;

use acif::{EntryPoint, Kind, Os, Rendering, Scope};
use adapters::{Adapter, CrossRead, InstallRecords, PlaceError, Tier, feed_providers, place};
use common::Items;
use serde_json::json;

struct Rows(BTreeMap<(&'static str, &'static str), Vec<EntryPoint>>);

impl InstallRecords for Rows {
    fn entry_points(&self, provider: &str, kind: Kind) -> Option<&[EntryPoint]> {
        self.0
            .iter()
            .find(|((p, k), _)| *p == provider && *k == kind.as_str())
            .map(|(_, rows)| rows.as_slice())
    }
}

fn row(template: &str, layout: &str) -> EntryPoint {
    EntryPoint::from_value(&json!({
        "scope": "user", "path_template": template, "layout": layout, "status": "current"
    }))
    .unwrap()
}

fn adapter(slug: &'static str, cross_reads: &'static [CrossRead]) -> Adapter {
    Adapter {
        slug,
        tier: Tier::Experimental,
        tool_version: "test",
        cross_reads,
    }
}

fn placed_at(placements: &[adapters::Placement]) -> Vec<(&str, Vec<&str>)> {
    placements
        .iter()
        .map(|p| (p.target.path.as_str(), p.providers.clone()))
        .collect()
}

/// A rule goes into the shared `AGENTS.md` once when every targeted
/// provider reads it, even where it is not a provider's first row.
#[test]
fn shared_file_read_by_every_provider_is_written_once() {
    let rows = Rows(BTreeMap::from([
        (
            ("alpha", "rule"),
            vec![row("~/AGENTS.md", "merged_into_shared_file")],
        ),
        (
            ("beta", "rule"),
            vec![
                row("~/.beta/RULES.md", "merged_into_shared_file"),
                row("~/AGENTS.md", "merged_into_shared_file"),
            ],
        ),
    ]));
    let (alpha, beta) = (adapter("alpha", &[]), adapter("beta", &[]));
    let items = Items::load();
    let (_, name, source) = items.sources()[1];
    let placements = place(
        &[&beta, &alpha],
        &rows,
        &source,
        name,
        Scope::User,
        Os::Linux,
    )
    .unwrap();
    assert_eq!(
        placed_at(&placements),
        [("~/AGENTS.md", vec!["beta", "alpha"])]
    );
    assert!(matches!(
        placements[0].rendering,
        Rendering::Contribution(_)
    ));
}

/// A skill is written once to the shared skills directory when the other
/// provider declares that it also reads there.
#[test]
fn cross_read_counts_as_a_read() {
    const READS_SHARED: &[CrossRead] = &[CrossRead {
        provider: "shared",
        kind: Kind::Skill,
        scope: Scope::User,
    }];
    let rows = Rows(BTreeMap::from([
        (
            ("shared", "skill"),
            vec![row(
                "~/.agents/skills/<content-name>/",
                "directory_of_files",
            )],
        ),
        (
            ("own", "skill"),
            vec![row("~/.own/skills/<content-name>/", "directory_of_files")],
        ),
    ]));
    let (shared, own) = (adapter("shared", &[]), adapter("own", READS_SHARED));
    assert_eq!(feed_providers(&[&own]), ["own", "shared"]);
    let items = Items::load();
    let (_, name, source) = items.sources()[0];
    let together = place(
        &[&own, &shared],
        &rows,
        &source,
        name,
        Scope::User,
        Os::Linux,
    )
    .unwrap();
    assert_eq!(
        placed_at(&together),
        [("~/.agents/skills/hello/", vec!["own", "shared"])]
    );
    // Without the cross-read, each provider gets its own copy.
    let plain = adapter("own", &[]);
    let apart = place(
        &[&plain, &shared],
        &rows,
        &source,
        name,
        Scope::User,
        Os::Linux,
    )
    .unwrap();
    assert_eq!(
        placed_at(&apart),
        [
            ("~/.own/skills/hello/", vec!["own"]),
            ("~/.agents/skills/hello/", vec!["shared"])
        ]
    );
    // A cross-read of a provider the records do not hold cannot be checked.
    let rows = Rows(BTreeMap::from([(
        ("own", "skill"),
        vec![row("~/.own/skills/<content-name>/", "directory_of_files")],
    )]));
    let err = place(&[&own], &rows, &source, name, Scope::User, Os::Linux).unwrap_err();
    assert_eq!(err.id(), "syllago.adapter.not_in_feed");
}

/// alpha reads both alpha's and beta's files, while gamma reads only
/// alpha's: alpha's file must be written for gamma, beta's for beta, and
/// alpha would then load the rule twice.
#[test]
fn provider_loading_the_item_twice_is_refused() {
    let rows = Rows(BTreeMap::from([
        (
            ("alpha", "rule"),
            vec![
                row("~/.alpha/RULES.md", "merged_into_shared_file"),
                row("~/.beta/RULES.md", "merged_into_shared_file"),
            ],
        ),
        (
            ("beta", "rule"),
            vec![row("~/.beta/RULES.md", "merged_into_shared_file")],
        ),
        (
            ("gamma", "rule"),
            vec![row("~/.alpha/RULES.md", "merged_into_shared_file")],
        ),
    ]));
    let (a, b, g) = (
        adapter("alpha", &[]),
        adapter("beta", &[]),
        adapter("gamma", &[]),
    );
    let items = Items::load();
    let (_, name, source) = items.sources()[1];
    let err = place(&[&a, &b, &g], &rows, &source, name, Scope::User, Os::Linux).unwrap_err();
    let PlaceError::LoadedTwice {
        provider,
        locations,
    } = &err
    else {
        panic!("{err}");
    };
    assert_eq!(*provider, "alpha");
    assert_eq!(locations, &["~/.alpha/RULES.md", "~/.beta/RULES.md"]);
    assert_eq!(err.id(), "syllago.adapter.loaded_twice");
    // alpha with beta alone shares beta's file.
    let placements = place(&[&a, &b], &rows, &source, name, Scope::User, Os::Linux).unwrap();
    assert_eq!(
        placed_at(&placements),
        [("~/.beta/RULES.md", vec!["alpha", "beta"])]
    );
}

#[test]
fn missing_and_refused_rows_are_named() {
    let rows = Rows(BTreeMap::from([(
        ("alpha", "skill"),
        vec![row("~/.alpha/skills/<content-name>/", "directory_of_files")],
    )]));
    let alpha = adapter("alpha", &[]);
    let items = Items::load();
    let (_, name, rule) = items.sources()[1];
    let err = place(&[&alpha], &rows, &rule, name, Scope::User, Os::Linux).unwrap_err();
    assert_eq!(err.id(), "syllago.adapter.not_in_feed");
    let (_, name, skill) = items.sources()[0];
    let err = place(&[&alpha], &rows, &skill, name, Scope::Project, Os::Linux).unwrap_err();
    assert_eq!(err.id(), "acif.install.scope_unavailable");
    let err = place(&[&alpha], &rows, &skill, "../x", Scope::User, Os::Linux).unwrap_err();
    assert_eq!(err.id(), "acif.install.content_name_invalid");
    // A skill into a shared file has no rendering.
    let rows = Rows(BTreeMap::from([(
        ("alpha", "skill"),
        vec![row("~/SKILLS.md", "merged_into_shared_file")],
    )]));
    let err = place(&[&alpha], &rows, &skill, name, Scope::User, Os::Linux).unwrap_err();
    assert_eq!(err.id(), "syllago.adapter.unsupported", "{err}");
}
