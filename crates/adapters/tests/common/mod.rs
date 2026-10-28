#![allow(dead_code)]

use std::path::PathBuf;
use std::time::{Duration, SystemTime};

use acif::{Body, Item, Kind, ingest_body};
use adapters::{ADAPTERS, Source, feed_providers};

pub fn golden_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/golden")
}

/// The capmon feed snapshot the fixtures are pinned to, verified as of the
/// day after it was attested.
pub fn snapshot_feed() -> feed::Feed {
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

/// The test skill and rule, ingested from `tests/golden/items`.
pub struct Items {
    skill: (Item, Body),
    rule: (Item, Body),
}

impl Items {
    pub fn load() -> Items {
        let read = |kind, dir: &str, entry: &str| {
            let body = Body::read(&golden_dir().join("items").join(dir)).unwrap();
            (ingest_body(kind, &body, entry).unwrap(), body)
        };
        Items {
            skill: read(Kind::Skill, "hello", "SKILL.md"),
            rule: read(Kind::Rule, "style", "style.md"),
        }
    }

    /// Each item with its kind and content name.
    pub fn sources(&self) -> Vec<(Kind, &'static str, Source<'_>)> {
        vec![
            (
                Kind::Skill,
                "hello",
                Source {
                    item: &self.skill.0,
                    body: &self.skill.1,
                    entry_file: "SKILL.md",
                },
            ),
            (
                Kind::Rule,
                "style",
                Source {
                    item: &self.rule.0,
                    body: &self.rule.1,
                    entry_file: "style.md",
                },
            ),
        ]
    }
}
