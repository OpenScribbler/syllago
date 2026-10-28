//! The feed is trusted only through its attestation: a pinned snapshot of
//! the live feed verifies under the public-good root, and every break in the
//! chain refuses the feed with a named reason.

use std::collections::BTreeMap;
use std::path::PathBuf;
use std::time::{Duration, SystemTime};

use acif::{EntryPoint, Kind};
use feed::{DirSource, FeedError, FeedSource, PROVENANCE, SOURCE_REPOSITORY, load, publisher};
use moat::TrustRoot;
use moat::testing::pki::{self, ISSUER, Instance, LOGGED_AT, Log, San};
use serde_json::{Value, json};

/// The live feed as published on 2026-10-28, with GitHub's attestation for
/// its index.
fn snapshot() -> DirSource {
    DirSource::new(
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/capmon-2026-10-28"),
    )
}

/// The day after the snapshot was attested.
fn snapshot_now() -> SystemTime {
    SystemTime::UNIX_EPOCH + Duration::from_secs(1_793_232_000) // 2026-10-29T00:00:00Z
}

#[test]
fn pinned_snapshot_verifies_under_the_public_good_root() {
    let feed = load(
        &snapshot(),
        &TrustRoot::bundled(),
        snapshot_now(),
        &["claude-code", "codex"],
    )
    .unwrap();
    assert_eq!(
        feed.index_sha256(),
        "7d3d2ad15d229365c6e769fd89c613d8e4327131d0b0fd161075ed8f84c8c9a5"
    );
    let skills = feed.entry_points("claude-code", Kind::Skill).unwrap();
    assert_eq!(skills[0].path_template, "~/.claude/skills/<content-name>/");
    let rules = feed.entry_points("codex", Kind::Rule).unwrap();
    assert_eq!(rules[0].path_template, "~/.codex/AGENTS.md");
    assert_eq!(feed.entry_points("cursor", Kind::Skill), None);
    assert_eq!(feed.entry_points("codex", Kind::Hook), None);
}

#[test]
fn pinned_snapshot_under_another_root_is_refused() {
    let other = Instance::new().trust_root();
    let err = load(&snapshot(), &other, snapshot_now(), &["codex"]).unwrap_err();
    assert_refused(&err, "syllago.moat.sigstore_verification_failed");
}

/// Feed bytes held in memory; every digest gets the same bundles, so a
/// tampered index still meets its old attestation.
struct MemSource {
    files: BTreeMap<String, Vec<u8>>,
    bundles: Vec<Vec<u8>>,
}

impl FeedSource for MemSource {
    fn fetch(&self, path: &str) -> Result<Vec<u8>, String> {
        self.files
            .get(path)
            .cloned()
            .ok_or_else(|| format!("{path}: not found"))
    }

    fn attestations(&self, _sha256: &str) -> Result<Vec<Vec<u8>>, String> {
        Ok(self.bundles.clone())
    }
}

fn now() -> SystemTime {
    SystemTime::UNIX_EPOCH + Duration::from_secs(u64::try_from(LOGGED_AT).unwrap() + 86_400)
}

fn sha256(bytes: &[u8]) -> String {
    pki::hex(&pki::sha256(bytes))
}

/// A test Sigstore instance signing as `san`.
struct Publisher {
    pki: Instance,
    log: Log,
    san: String,
    /// The Source Repository URI its certificates carry.
    repository: Option<String>,
}

impl Publisher {
    fn new(san: &str) -> Publisher {
        let pki = Instance::new();
        let log = Log::new(pki.rekor.clone());
        Publisher {
            pki,
            log,
            san: san.into(),
            repository: Some(SOURCE_REPOSITORY.into()),
        }
    }

    fn capmon() -> Publisher {
        Publisher::new(&publisher().subject)
    }

    /// A provenance attestation naming each of `artifacts`.
    fn attest(&mut self, artifacts: &[&[u8]]) -> Vec<u8> {
        let subject: Vec<Value> = artifacts
            .iter()
            .map(|a| json!({"name": "index.json", "digest": {"sha256": sha256(a)}}))
            .collect();
        let statement = json!({
            "_type": "https://in-toto.io/Statement/v1",
            "subject": subject,
            "predicateType": PROVENANCE,
            "predicate": {},
        })
        .to_string();
        let key = pki::key(30);
        let cert = self.pki.ci_leaf(
            &key,
            San::Uri(&self.san),
            ISSUER,
            LOGGED_AT,
            self.repository.as_deref(),
        );
        pki::dsse_statement_bundle(
            &mut self.log,
            "application/vnd.in-toto+json",
            statement.as_bytes(),
            &key,
            &cert,
        )
        .to_string()
        .into_bytes()
    }
}

fn provider_document(slug: &str, skill_template: &str) -> Vec<u8> {
    json!({
        "schema_version": "3",
        "slug": slug,
        "content_types": {
            "skills": {"supported": true, "install_entry_points": [
                {"scope": "user", "path_template": skill_template, "layout": "directory_of_files", "status": "current", "as_of": "test"}
            ]},
            "rules": {"supported": true},
        }
    })
    .to_string()
    .into_bytes()
}

/// An index entry: slug, path and the document it names.
type Listed<'a> = (&'a str, &'a str, &'a [u8]);

fn index(providers: &[Listed<'_>]) -> Vec<u8> {
    let providers: Vec<Value> = providers
        .iter()
        .map(|(slug, path, doc)| json!({"slug": slug, "path": path, "status": "tracked", "sha256": sha256(doc)}))
        .collect();
    json!({
        "schema_version": "1",
        "status": "live",
        "generated_at": "2026-01-01T00:00:00Z",
        "source_commit": "abc",
        "providers": providers,
        "files": {},
    })
    .to_string()
    .into_bytes()
}

/// A feed with one provider, `tool`, attested by capmon's identity.
fn signed_feed() -> (MemSource, TrustRoot) {
    let doc = provider_document("tool", "~/.tool/skills/<content-name>/");
    let index = index(&[("tool", "capabilities/tool.json", &doc)]);
    let mut publisher = Publisher::capmon();
    let bundle = publisher.attest(&[b"another index", &index]);
    let source = MemSource {
        files: BTreeMap::from([
            ("v2/index.json".into(), index),
            ("v2/capabilities/tool.json".into(), doc),
        ]),
        bundles: vec![bundle],
    };
    (source, publisher.pki.trust_root())
}

fn assert_refused(err: &FeedError, cause: &str) {
    let FeedError::AttestationRefused { failures, .. } = err else {
        panic!("expected an attestation refusal, got {err}");
    };
    assert_eq!(err.id(), "syllago.feed.attestation_refused");
    assert_eq!(failures[0].id(), cause, "{err}");
}

#[test]
fn attested_feed_exposes_its_rows() {
    let (source, root) = signed_feed();
    let feed = load(&source, &root, now(), &["tool"]).unwrap();
    assert_eq!(feed.source_commit(), Some("abc"));
    assert_eq!(feed.generated_at(), "2026-01-01T00:00:00Z");
    let skills = feed.entry_points("tool", Kind::Skill).unwrap();
    assert_eq!(
        skills,
        [EntryPoint {
            scope: "user".into(),
            os: None,
            path_template: "~/.tool/skills/<content-name>/".into(),
            layout: "directory_of_files".into(),
            dialect: None,
            status: "current".into(),
            as_of: Some("test".into()),
        }]
    );
    // A content type with no rows is an empty list, not an absence.
    assert_eq!(feed.entry_points("tool", Kind::Rule), Some(&[][..]));
}

#[test]
fn index_with_another_digest_is_refused() {
    let (mut source, root) = signed_feed();
    let index = source.files.get_mut("v2/index.json").unwrap();
    let redirected = String::from_utf8(index.clone())
        .unwrap()
        .replace("abc", "abd");
    *index = redirected.into_bytes();
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    assert_refused(&err, "syllago.moat.subject_digest_mismatch");
    assert!(err.to_string().contains("not trusted"), "{err}");
}

#[test]
fn attestation_by_another_signer_is_refused() {
    let (mut source, _) = signed_feed();
    let index = source.files["v2/index.json"].clone();
    for san in [
        "https://github.com/OpenScribbler/capmon/.github/workflows/ci.yml@refs/heads/main",
        "https://github.com/someone/capmon/.github/workflows/publish.yml@refs/heads/main",
        "https://github.com/OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/topic",
    ] {
        let mut other = Publisher::new(san);
        source.bundles = vec![other.attest(&[&index])];
        let err = load(&source, &other.pki.trust_root(), now(), &["tool"]).unwrap_err();
        assert_refused(&err, "syllago.moat.identity_mismatch");
    }
}

/// capmon's own workflow file run from another repository's build, as a
/// reusable-workflow call would be, names capmon's workflow in the SAN but
/// another repository in the certificate's Source Repository URI.
#[test]
fn attestation_from_another_repository_is_refused() {
    let (mut source, _) = signed_feed();
    let index = source.files["v2/index.json"].clone();
    for repository in [Some("https://github.com/someone/capmon"), None] {
        let mut other = Publisher::capmon();
        other.repository = repository.map(str::to_string);
        source.bundles = vec![other.attest(&[&index])];
        let err = load(&source, &other.pki.trust_root(), now(), &["tool"]).unwrap_err();
        assert_refused(&err, "syllago.feed.source_repository_mismatch");
    }
}

#[test]
fn missing_attestation_is_refused() {
    let (mut source, root) = signed_feed();
    source.bundles.clear();
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    assert_eq!(err.id(), "syllago.feed.attestation_missing");
    assert!(err.to_string().contains("tampered"), "{err}");
    // The snapshot's directory has no attestation for another digest.
    let dir = snapshot();
    assert_eq!(
        dir.attestations(&"0".repeat(64)).unwrap(),
        Vec::<Vec<u8>>::new()
    );
}

#[test]
fn one_verifying_attestation_among_several_is_enough() {
    let (mut source, root) = signed_feed();
    let good = source.bundles[0].clone();
    source.bundles = vec![b"not a bundle".to_vec(), good];
    load(&source, &root, now(), &["tool"]).unwrap();
    // With none verifying, every failure is reported.
    source.bundles = vec![b"not a bundle".to_vec(), b"{}".to_vec()];
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    let FeedError::AttestationRefused { failures, .. } = &err else {
        panic!("{err}");
    };
    assert_eq!(failures.len(), 2);
}

/// A list longer than the cap is refused before any bundle is verified,
/// even when a good one is in it.
#[test]
fn too_many_attestations_are_refused() {
    let (mut source, root) = signed_feed();
    let good = source.bundles[0].clone();
    source.bundles = vec![good; feed::MAX_ATTESTATIONS];
    load(&source, &root, now(), &["tool"]).unwrap();
    source.bundles.push(b"not a bundle".to_vec());
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    assert_eq!(err.id(), "syllago.feed.too_many_attestations", "{err}");
}

#[test]
fn provider_document_must_match_the_index() {
    let (mut source, root) = signed_feed();
    source.files.insert(
        "v2/capabilities/tool.json".into(),
        provider_document("tool", "~/.elsewhere/<content-name>/"),
    );
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    assert_eq!(err.id(), "syllago.feed.document_digest_mismatch", "{err}");
}

#[test]
fn index_must_list_providers_safely() {
    let doc = provider_document("tool", "~/.tool/skills/<content-name>/");
    let cases: [(&[Listed<'_>], &str, &str); 5] = [
        (
            &[("tool", "capabilities/tool.json", &doc)],
            "other",
            "syllago.feed.provider_missing",
        ),
        (
            &[("tool", "../tool.json", &doc)],
            "tool",
            "syllago.feed.index_malformed",
        ),
        (
            &[("tool", "/tool.json", &doc)],
            "tool",
            "syllago.feed.index_malformed",
        ),
        (
            &[("tool", "c:\\tool.json", &doc)],
            "tool",
            "syllago.feed.index_malformed",
        ),
        (
            &[
                ("tool", "capabilities/tool.json", &doc),
                ("tool", "capabilities/tool.json", &doc),
            ],
            "tool",
            "syllago.feed.index_malformed",
        ),
    ];
    for (providers, wanted, id) in cases {
        let index = index(providers);
        let mut publisher = Publisher::capmon();
        let source = MemSource {
            bundles: vec![publisher.attest(&[&index])],
            files: BTreeMap::from([
                ("v2/index.json".into(), index),
                ("v2/capabilities/tool.json".into(), doc.clone()),
            ]),
        };
        let err = load(&source, &publisher.pki.trust_root(), now(), &[wanted]).unwrap_err();
        assert_eq!(err.id(), id, "{err}");
    }
}

#[test]
fn verified_documents_of_the_wrong_shape_are_refused() {
    for doc in [
        provider_document("other", "~/.tool/skills/<content-name>/"),
        json!({"slug": "tool", "content_types": {"skills": {"install_entry_points": {}}}})
            .to_string()
            .into_bytes(),
        json!({"slug": "tool", "content_types": {"rules": {"install_entry_points": [{"scope": "user"}]}}})
            .to_string()
            .into_bytes(),
        b"not json".to_vec(),
    ] {
        let index = index(&[("tool", "capabilities/tool.json", &doc)]);
        let mut publisher = Publisher::capmon();
        let source = MemSource {
            bundles: vec![publisher.attest(&[&index])],
            files: BTreeMap::from([
                ("v2/index.json".into(), index),
                ("v2/capabilities/tool.json".into(), doc),
            ]),
        };
        let err = load(&source, &publisher.pki.trust_root(), now(), &["tool"]).unwrap_err();
        assert_eq!(err.id(), "syllago.feed.document_malformed", "{err}");
    }
}

#[test]
fn unfetchable_files_are_named() {
    let (mut source, root) = signed_feed();
    source.files.remove("v2/capabilities/tool.json");
    let err = load(&source, &root, now(), &["tool"]).unwrap_err();
    assert_eq!(
        err,
        FeedError::Fetch {
            path: "v2/capabilities/tool.json".into(),
            detail: "v2/capabilities/tool.json: not found".into()
        }
    );
}

#[test]
fn attestations_response_parsing() {
    let ok = br#"{"attestations": [{"bundle": {"a": 1}, "bundle_url": "x"}]}"#;
    assert_eq!(
        feed::parse_attestations_response(ok).unwrap(),
        [br#"{"a":1}"#.to_vec()]
    );
    for bad in [&b"nope"[..], br#"{}"#, br#"{"attestations": [{}]}"#] {
        assert!(feed::parse_attestations_response(bad).is_err());
    }
}
