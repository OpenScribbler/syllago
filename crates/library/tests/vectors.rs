//! The published syllago-loadout-v1 vectors, from
//! crates/library/tests/vectors/loadout-vectors.json.

use std::collections::BTreeSet;

use library::loadout::{self, Manifest, Record};
use serde_json::Value;

fn vectors() -> Value {
    serde_json::from_str(include_str!("vectors/loadout-vectors.json")).unwrap()
}

fn bytes(v: &Value) -> &[u8] {
    v["bytes"].as_str().unwrap().as_bytes()
}

fn outcome<T>(result: &Result<T, loadout::Refusal>) -> &'static str {
    match result {
        Ok(_) => "ok",
        Err(refusal) => refusal.id(),
    }
}

fn ids(vectors: &[Value]) -> Vec<&str> {
    vectors.iter().map(|v| v["id"].as_str().unwrap()).collect()
}

#[test]
fn digests() {
    let doc = vectors();
    let vectors = doc["digests"].as_array().unwrap();
    assert_eq!(ids(vectors), ["LD-01", "LD-02", "LD-03", "LD-04"]);
    for v in vectors {
        let id = v["id"].as_str().unwrap();
        assert_eq!(
            moat::canonical_text(bytes(v)),
            v["canonical"].as_str().unwrap().as_bytes(),
            "{id}"
        );
        let parsed = Manifest::parse(v["file_name"].as_str().unwrap(), bytes(v));
        match v["expected"].as_str() {
            None => assert_eq!(parsed.unwrap().digest(), v["digest"], "{id}"),
            Some(expected) => assert_eq!(outcome(&parsed), expected, "{id}"),
        }
    }
    assert_eq!(vectors[0]["digest"], vectors[1]["digest"]);
    assert_eq!(vectors[0]["digest"], vectors[2]["digest"]);
}

#[test]
fn manifests() {
    let doc = vectors();
    let vectors = doc["manifests"].as_array().unwrap();
    let want: Vec<String> = (1..=30).map(|n| format!("LM-{n:02}")).collect();
    assert_eq!(ids(vectors), want);
    for v in vectors {
        let id = v["id"].as_str().unwrap();
        let parsed = Manifest::parse(v["file_name"].as_str().unwrap(), bytes(v));
        assert_eq!(outcome(&parsed), v["expected"], "{id}: {parsed:?}");
        if let Ok(manifest) = parsed {
            assert_eq!(manifest.name(), v["name"], "{id}");
        }
    }
}

#[test]
fn name_sets() {
    let doc = vectors();
    let vectors = doc["name_sets"].as_array().unwrap();
    assert_eq!(ids(vectors), ["LN-01", "LN-02"]);
    for v in vectors {
        let files: Vec<(&str, &[u8])> = v["files"]
            .as_array()
            .unwrap()
            .iter()
            .map(|f| (f["file_name"].as_str().unwrap(), bytes(f)))
            .collect();
        let result = loadout::parse_all(&files);
        assert_eq!(outcome(&result), v["expected"], "{}", v["id"]);
    }
}

fn record(v: &Value) -> Option<Record> {
    let r = v.as_object()?;
    Some(Record {
        sequence: r["sequence"].as_u64().unwrap(),
        digest: r["digest"].as_str().unwrap().into(),
        active: r["active"].as_bool().unwrap(),
    })
}

#[test]
fn signatures() {
    let doc = vectors();
    let vectors = doc["signatures"].as_array().unwrap();
    let got: BTreeSet<&str> = ids(vectors).into_iter().collect();
    let want: Vec<String> = (1..=17).map(|n| format!("LV-{n:02}")).collect();
    assert_eq!(got, want.iter().map(String::as_str).collect());

    for v in vectors {
        let id = v["id"].as_str().unwrap();
        let source_uri = v["source_uri"].as_str().unwrap();
        let manifest = Manifest::parse(v["file_name"].as_str().unwrap(), bytes(v)).unwrap();
        assert_eq!(
            loadout::loadout_message(source_uri, &manifest).unwrap(),
            v["message"].as_str().unwrap().as_bytes(),
            "{id}"
        );
        let list = doc["lists"][v["list"].as_str().unwrap()].as_str().unwrap();
        let result = loadout::decide(
            source_uri,
            &manifest,
            v["signature"].as_str(),
            list.as_bytes(),
            record(&v["record"]).as_ref(),
        );
        let got = match &result {
            Ok(decision) => decision.id(),
            Err(refusal) => {
                assert_eq!(refusal.loadout(), manifest.name(), "{id}");
                refusal.id()
            }
        };
        assert_eq!(got, v["expected"], "{id}");
    }
}

/// The valid vector's signer is key A, and the source_uri the vectors sign
/// for is the header's.
#[test]
fn valid_vector_signer_is_key_a() {
    let doc = vectors();
    let v = &doc["signatures"][0];
    assert_eq!(v["source_uri"], doc["source_uri"]);
    let manifest = Manifest::parse("work.json", bytes(v)).unwrap();
    let signer = loadout::verify_loadout(
        v["source_uri"].as_str().unwrap(),
        &manifest,
        v["signature"].as_str(),
        doc["lists"]["LA"].as_str().unwrap().as_bytes(),
    )
    .unwrap();
    assert_eq!(signer.fingerprint.as_str(), doc["keys"]["A"]["fingerprint"]);
    assert_eq!(signer.key_type.as_str(), doc["keys"]["A"]["type"]);
}
