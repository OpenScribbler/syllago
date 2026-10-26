//! MOAT's published vectors: TV-ACIF-01..04 and TV-AK-01..12 with TV-AK-05b,
//! from crates/moat/tests/vectors/moat-vectors.json.

mod common;

use std::collections::BTreeSet;

use moat::{AllowedSigners, Refusal, VerifiedSigner};
use serde_json::Value;

fn outcome(result: &Result<VerifiedSigner, Refusal>) -> &'static str {
    match result {
        Ok(_) => "ok",
        Err(refusal) => refusal.id(),
    }
}

#[test]
fn header_names_the_moat_commit() {
    let doc = common::vectors();
    assert_eq!(
        doc["moat_commit"],
        "8330bf227f9894f8a88b48f9cad5e1af04044835"
    );
}

#[test]
fn acif_vectors() {
    let doc = common::vectors();
    let vectors = doc["acif"].as_array().unwrap();
    let ids: Vec<&str> = vectors.iter().map(|v| v["id"].as_str().unwrap()).collect();
    assert_eq!(
        ids,
        ["TV-ACIF-01", "TV-ACIF-02", "TV-ACIF-03", "TV-ACIF-04"]
    );
    for v in vectors {
        let id = v["id"].as_str().unwrap();
        let item = common::acif_item(v["files"].as_object().unwrap());
        assert_eq!(
            item.body_hash.as_ref().unwrap().to_string(),
            v["body_hash"],
            "{id}"
        );
        assert_eq!(
            item.metadata_hash().map(|h| h.to_string()),
            v["metadata_hash"].as_str().map(str::to_string),
            "{id}"
        );
        let hash = moat::ContentHash::of_acif(&item).unwrap();
        assert_eq!(hash.to_string(), v["content_hash"], "{id}");
        let payload = moat::attestation_payload(&hash);
        assert_eq!(payload, v["payload"].as_str().unwrap().as_bytes(), "{id}");
    }
}

#[test]
fn list_digests_and_malformed_lists() {
    let doc = common::vectors();
    for (name, list) in doc["author_key"]["lists"].as_object().unwrap() {
        let parsed = AllowedSigners::parse(list["text"].as_str().unwrap().as_bytes());
        match name.as_str() {
            "LBAD" | "LNOSEQ" => {
                assert_eq!(
                    parsed.unwrap_err().id(),
                    "allowed_signers_malformed",
                    "{name}"
                );
            }
            _ => assert_eq!(parsed.unwrap().digest().as_str(), list["digest"], "{name}"),
        }
    }
}

/// A list holding only a key's line parses only when the line's
/// fingerprint is the one computed from its blob.
#[test]
fn key_fingerprints() {
    let doc = common::vectors();
    for (label, key) in doc["author_key"]["keys"].as_object().unwrap() {
        let line = key["line"].as_str().unwrap();
        assert!(
            line.starts_with(key["fingerprint"].as_str().unwrap()),
            "{label}"
        );
        let list = format!("{line}\n# moat-sequence: 1\n");
        assert!(AllowedSigners::parse(list.as_bytes()).is_ok(), "{label}");
    }
}

fn run(doc: &Value, v: &Value) -> Result<VerifiedSigner, Refusal> {
    let signature = v["signature"].as_str().unwrap();
    match v["kind"].as_str().unwrap() {
        "item" => {
            let hash = item_hash(doc, v);
            moat::verify_item(
                &hash,
                signature,
                common::list(doc, v["list"].as_str().unwrap()),
            )
        }
        _ => moat::verify_list_change(
            v["source_uri"].as_str().unwrap(),
            common::list(doc, v["previous_list"].as_str().unwrap()),
            common::list(doc, v["new_list"].as_str().unwrap()),
            signature,
        ),
    }
}

/// The Content Hash a verifier computes for an item vector: TV-ACIF-04's
/// files, with scripts/run.sh tampered for TV-AK-03.
fn item_hash(doc: &Value, v: &Value) -> moat::ContentHash {
    let mut files = doc["acif"][3]["files"].as_object().unwrap().clone();
    assert_eq!(doc["acif"][3]["id"], "TV-ACIF-04");
    if v["id"] == "TV-AK-03" {
        files.insert("scripts/run.sh".into(), "#!/bin/sh\necho bye\n".into());
    }
    let hash = common::content_hash(&files);
    assert_eq!(hash.to_string(), v["content_hash"], "{}", v["id"]);
    hash
}

#[test]
fn author_key_vectors() {
    let doc = common::vectors();
    let vectors = doc["author_key"]["vectors"].as_array().unwrap();
    let ids: BTreeSet<String> = vectors
        .iter()
        .map(|v| v["id"].as_str().unwrap().into())
        .collect();
    let want: BTreeSet<String> = (1..=12)
        .map(|n| format!("TV-AK-{n:02}"))
        .chain(["TV-AK-05b".to_string()])
        .collect();
    assert_eq!(ids, want);

    for v in vectors {
        let id = v["id"].as_str().unwrap();
        let message = match v["kind"].as_str().unwrap() {
            "item" => moat::item_message(&item_hash(&doc, v)),
            _ => moat::list_change_message(
                v["source_uri"].as_str().unwrap(),
                &AllowedSigners::parse(common::list(&doc, v["previous_list"].as_str().unwrap()))
                    .unwrap(),
                &AllowedSigners::parse(common::list(&doc, v["new_list"].as_str().unwrap()))
                    .unwrap(),
            )
            .unwrap(),
        };
        assert_eq!(message, v["message"].as_str().unwrap().as_bytes(), "{id}");

        let result = run(&doc, v);
        assert_eq!(outcome(&result), v["expected"], "{id}");
        if let Ok(signer) = result {
            let key = &doc["author_key"]["keys"][v["signer"].as_str().unwrap()];
            assert_eq!(signer.fingerprint.as_str(), key["fingerprint"], "{id}");
            assert_eq!(signer.key_type.as_str(), key["type"], "{id}");
        }
    }
}

#[test]
fn rotated_key_is_not_allowed_by_the_previous_list() {
    let doc = common::vectors();
    let v = doc["author_key"]["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == "TV-AK-05b")
        .unwrap();
    let result = moat::verify_item(
        &item_hash(&doc, v),
        v["signature"].as_str().unwrap(),
        common::list(&doc, "L1"),
    );
    assert_eq!(result.unwrap_err(), Refusal::SignerNotAllowed);
}
