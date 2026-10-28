//! A Sigstore instance generated at test time: a Fulcio-style CA, a CT log, a
//! Rekor v1 log and a trusted root naming all three, with P-256 keys derived
//! from fixed seeds. Nothing here touches the network or the filesystem.
//!
//! The certificates are hand-encoded DER so the tests need no X.509 builder.
//! They carry what `sigstore-verify` checks: a CODE_SIGNING EKU, a critical
//! SAN, the Fulcio issuer extension and an embedded SCT from the CT log.

use p256::ecdsa::signature::Signer;
use p256::ecdsa::{Signature, SigningKey};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};

use crate::wire;
use crate::{TrustRoot, TrustRootOrigin};

/// When every test entry was logged: 2026-01-01T00:00:00Z.
pub const LOGGED_AT: i64 = 1_767_225_600;

/// The registry's signing identity in the tests.
pub const SUBJECT: &str =
    "https://github.com/owner/registry/.github/workflows/moat-registry.yml@refs/heads/main";
pub const ISSUER: &str = "https://token.actions.githubusercontent.com";

const OID_EC_PUBLIC_KEY: &[u8] = &[0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01];
const OID_P256: &[u8] = &[0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07];
const OID_ECDSA_SHA256: &[u8] = &[0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02];
const OID_COMMON_NAME: &[u8] = &[0x55, 0x04, 0x03];
const OID_BASIC_CONSTRAINTS: &[u8] = &[0x55, 0x1d, 0x13];
const OID_KEY_USAGE: &[u8] = &[0x55, 0x1d, 0x0f];
const OID_EXT_KEY_USAGE: &[u8] = &[0x55, 0x1d, 0x25];
const OID_SUBJECT_ALT_NAME: &[u8] = &[0x55, 0x1d, 0x11];
const OID_CODE_SIGNING: &[u8] = &[0x2b, 0x06, 0x01, 0x05, 0x05, 0x07, 0x03, 0x03];
/// 1.3.6.1.4.1.57264.1.8, Fulcio's OIDC issuer as a DER UTF8String.
const OID_FULCIO_ISSUER: &[u8] = &[0x2b, 0x06, 0x01, 0x04, 0x01, 0x83, 0xbf, 0x30, 0x01, 0x08];
/// 1.3.6.1.4.1.57264.1.12, Fulcio's Source Repository URI as a DER
/// UTF8String.
const OID_FULCIO_SOURCE_REPOSITORY: &[u8] =
    &[0x2b, 0x06, 0x01, 0x04, 0x01, 0x83, 0xbf, 0x30, 0x01, 0x0c];
/// 1.3.6.1.4.1.11129.2.4.2, the embedded SCT list.
const OID_SCT_LIST: &[u8] = &[0x2b, 0x06, 0x01, 0x04, 0x01, 0xd6, 0x79, 0x02, 0x04, 0x02];

fn tlv(tag: u8, content: &[u8]) -> Vec<u8> {
    let mut out = vec![tag];
    let len = content.len();
    if len < 0x80 {
        out.push(len as u8);
    } else {
        let bytes = len.to_be_bytes();
        let skip = bytes.iter().take_while(|b| **b == 0).count();
        out.push(0x80 | (bytes.len() - skip) as u8);
        out.extend_from_slice(&bytes[skip..]);
    }
    out.extend_from_slice(content);
    out
}

fn seq(parts: &[Vec<u8>]) -> Vec<u8> {
    tlv(0x30, &parts.concat())
}

fn oid(encoded: &[u8]) -> Vec<u8> {
    tlv(0x06, encoded)
}

fn integer(value: u64) -> Vec<u8> {
    let bytes = value.to_be_bytes();
    let skip = bytes.iter().take_while(|b| **b == 0).count().min(7);
    let mut content = bytes[skip..].to_vec();
    if content[0] & 0x80 != 0 {
        content.insert(0, 0);
    }
    tlv(0x02, &content)
}

fn bit_string(bytes: &[u8]) -> Vec<u8> {
    tlv(0x03, &[&[0u8][..], bytes].concat())
}

fn octet_string(bytes: &[u8]) -> Vec<u8> {
    tlv(0x04, bytes)
}

fn utc_time(unix: i64) -> Vec<u8> {
    let text = jiff::Timestamp::from_second(unix)
        .unwrap()
        .strftime("%y%m%d%H%M%SZ")
        .to_string();
    tlv(0x17, text.as_bytes())
}

fn extension(id: &[u8], critical: bool, value: Vec<u8>) -> Vec<u8> {
    let mut parts = vec![oid(id)];
    if critical {
        parts.push(vec![0x01, 0x01, 0xff]);
    }
    parts.push(octet_string(&value));
    seq(&parts)
}

fn common_name(cn: &str) -> Vec<u8> {
    seq(&[tlv(
        0x31,
        &seq(&[oid(OID_COMMON_NAME), tlv(0x0c, cn.as_bytes())]),
    )])
}

fn ecdsa_sha256() -> Vec<u8> {
    seq(&[oid(OID_ECDSA_SHA256)])
}

/// A P-256 key derived from `seed`.
pub fn key(seed: u8) -> SigningKey {
    SigningKey::from_slice(&Sha256::digest([seed])).unwrap()
}

/// A key's SubjectPublicKeyInfo DER.
pub fn spki(key: &SigningKey) -> Vec<u8> {
    let point = key.verifying_key().to_sec1_point(false);
    seq(&[
        seq(&[oid(OID_EC_PUBLIC_KEY), oid(OID_P256)]),
        bit_string(point.as_bytes()),
    ])
}

/// A DER ECDSA-SHA256 signature over `data`.
pub fn sign(key: &SigningKey, data: &[u8]) -> Vec<u8> {
    let signature: Signature = key.sign(data);
    signature.to_der().as_bytes().to_vec()
}

/// Standard base64 with padding.
pub fn base64(bytes: &[u8]) -> String {
    wire::base64(bytes)
}

pub fn sha256(data: &[u8]) -> [u8; 32] {
    Sha256::digest(data).into()
}

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

fn pem_certificate(der: &[u8]) -> String {
    let encoded = wire::base64(der);
    let lines: Vec<&str> = encoded
        .as_bytes()
        .chunks(64)
        .map(|c| std::str::from_utf8(c).unwrap())
        .collect();
    format!(
        "-----BEGIN CERTIFICATE-----\n{}\n-----END CERTIFICATE-----\n",
        lines.join("\n")
    )
}

/// A leaf certificate's identity: the SAN and the OIDC issuer.
pub enum San<'a> {
    Uri(&'a str),
    Email(&'a str),
}

/// The CA, CT log and Rekor log keys and the CA certificate.
pub struct Instance {
    ca: SigningKey,
    ca_cert: Vec<u8>,
    ct: SigningKey,
    pub rekor: SigningKey,
}

impl Default for Instance {
    fn default() -> Instance {
        Instance::new()
    }
}

impl Instance {
    pub fn new() -> Instance {
        let ca = key(1);
        let name = common_name("moat test ca");
        let tbs = seq(&[
            tlv(0xa0, &integer(2)),
            integer(1),
            ecdsa_sha256(),
            name.clone(),
            seq(&[utc_time(1_577_836_800), utc_time(2_524_607_999)]), // 2020..2049
            name,
            spki(&ca),
            tlv(
                0xa3,
                &seq(&[
                    extension(OID_BASIC_CONSTRAINTS, true, seq(&[vec![0x01, 0x01, 0xff]])),
                    // keyCertSign and cRLSign.
                    extension(OID_KEY_USAGE, true, tlv(0x03, &[0x01, 0x06])),
                ]),
            ),
        ]);
        let ca_cert = seq(&[tbs.clone(), ecdsa_sha256(), bit_string(&sign(&ca, &tbs))]);
        Instance {
            ca,
            ca_cert,
            ct: key(2),
            rekor: key(3),
        }
    }

    /// The trusted root, in the shape of the public-good one.
    pub fn trusted_root_json(&self) -> String {
        let log = |key: &SigningKey, url: &str| {
            json!({
                "baseUrl": url,
                "hashAlgorithm": "SHA2_256",
                "publicKey": {
                    "rawBytes": wire::base64(&spki(key)),
                    "keyDetails": "PKIX_ECDSA_P256_SHA_256",
                    "validFor": {"start": "2025-01-01T00:00:00Z"}
                },
                "logId": {"keyId": wire::base64(&sha256(&spki(key)))}
            })
        };
        json!({
            "mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
            "tlogs": [log(&self.rekor, "https://rekor.test")],
            "certificateAuthorities": [{
                "subject": {"organization": "moat test", "commonName": "moat test ca"},
                "uri": "https://fulcio.test",
                "certChain": {"certificates": [{"rawBytes": wire::base64(&self.ca_cert)}]},
                "validFor": {"start": "2025-01-01T00:00:00Z"}
            }],
            "ctlogs": [log(&self.ct, "https://ct.test")],
            "timestampAuthorities": []
        })
        .to_string()
    }

    /// The trusted root as an invocation override.
    pub fn trust_root(&self) -> TrustRoot {
        TrustRoot::from_override(
            self.trusted_root_json().as_bytes(),
            TrustRootOrigin::Invocation {
                path: "/test/trusted_root.json".into(),
                registry: "test".into(),
            },
        )
        .unwrap()
    }

    /// A Fulcio-style leaf for `signer`, valid for ten minutes from a minute
    /// before `at`, with an SCT from the CT log.
    pub fn leaf(&self, signer: &SigningKey, san: San<'_>, issuer: &str, at: i64) -> Vec<u8> {
        self.ci_leaf(signer, san, issuer, at, None)
    }

    /// [`Instance::leaf`], also carrying Fulcio's Source Repository URI
    /// extension when `source_repository` is given, as a certificate issued
    /// to a CI workload does.
    pub fn ci_leaf(
        &self,
        signer: &SigningKey,
        san: San<'_>,
        issuer: &str,
        at: i64,
        source_repository: Option<&str>,
    ) -> Vec<u8> {
        let san = match san {
            San::Uri(uri) => tlv(0x86, uri.as_bytes()),
            San::Email(email) => tlv(0x81, email.as_bytes()),
        };
        let mut extensions = vec![
            // digitalSignature.
            extension(OID_KEY_USAGE, true, tlv(0x03, &[0x07, 0x80])),
            extension(OID_EXT_KEY_USAGE, false, seq(&[oid(OID_CODE_SIGNING)])),
            extension(OID_SUBJECT_ALT_NAME, true, seq(&[san])),
            extension(OID_FULCIO_ISSUER, false, tlv(0x0c, issuer.as_bytes())),
        ];
        if let Some(repository) = source_repository {
            extensions.push(extension(
                OID_FULCIO_SOURCE_REPOSITORY,
                false,
                tlv(0x0c, repository.as_bytes()),
            ));
        }
        let tbs = |extensions: &[Vec<u8>]| {
            seq(&[
                tlv(0xa0, &integer(2)),
                integer(0x1000 + u64::try_from(at).unwrap() % 0x1000),
                ecdsa_sha256(),
                common_name("moat test ca"),
                seq(&[utc_time(at - 60), utc_time(at + 540)]),
                seq(&[]),
                spki(signer),
                tlv(0xa3, &seq(extensions)),
            ])
        };
        let precert = tbs(&extensions);
        let timestamp = u64::try_from(at).unwrap() * 1000;

        let mut signed = vec![0, 0];
        signed.extend_from_slice(&timestamp.to_be_bytes());
        signed.extend_from_slice(&[0, 1]);
        signed.extend_from_slice(&sha256(&spki(&self.ca)));
        signed.extend_from_slice(&u32::try_from(precert.len()).unwrap().to_be_bytes()[1..]);
        signed.extend_from_slice(&precert);
        signed.extend_from_slice(&[0, 0]);
        let signature = sign(&self.ct, &signed);

        let mut sct = vec![0];
        sct.extend_from_slice(&sha256(&spki(&self.ct)));
        sct.extend_from_slice(&timestamp.to_be_bytes());
        sct.extend_from_slice(&[0, 0, 4, 3]);
        sct.extend_from_slice(&u16::try_from(signature.len()).unwrap().to_be_bytes());
        sct.extend_from_slice(&signature);
        let mut list = u16::try_from(sct.len() + 2).unwrap().to_be_bytes().to_vec();
        list.extend_from_slice(&u16::try_from(sct.len()).unwrap().to_be_bytes());
        list.extend_from_slice(&sct);
        extensions.push(extension(OID_SCT_LIST, false, octet_string(&list)));

        let tbs = tbs(&extensions);
        seq(&[
            tbs.clone(),
            ecdsa_sha256(),
            bit_string(&sign(&self.ca, &tbs)),
        ])
    }
}

/// The canonicalized body of a `hashedrekord` 0.0.1 entry: `signer`'s
/// signature over `artifact`, with `certificate` as its public key.
pub fn hashedrekord_body(artifact: &[u8], signature: &[u8], certificate: &[u8]) -> Vec<u8> {
    format!(
        r#"{{"apiVersion":"0.0.1","kind":"hashedrekord","spec":{{"data":{{"hash":{{"algorithm":"sha256","value":"{}"}}}},"signature":{{"content":"{}","publicKey":{{"content":"{}"}}}}}}}}"#,
        hex(&sha256(artifact)),
        wire::base64(signature),
        wire::base64(pem_certificate(certificate).as_bytes()),
    )
    .into_bytes()
}

fn leaf_hash(body: &[u8]) -> [u8; 32] {
    sha256(&[&[0u8][..], body].concat())
}

fn node_hash(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    sha256(&[&[1u8][..], left, right].concat())
}

fn split(n: usize) -> usize {
    let mut k = 1;
    while k * 2 < n {
        k *= 2;
    }
    k
}

/// RFC 6962 Merkle tree hash.
fn tree_hash(leaves: &[[u8; 32]]) -> [u8; 32] {
    if leaves.len() == 1 {
        return leaves[0];
    }
    let k = split(leaves.len());
    node_hash(&tree_hash(&leaves[..k]), &tree_hash(&leaves[k..]))
}

/// RFC 6962 audit path for leaf `m`.
fn audit_path(m: usize, leaves: &[[u8; 32]]) -> Vec<[u8; 32]> {
    if leaves.len() == 1 {
        return Vec::new();
    }
    let k = split(leaves.len());
    if m < k {
        let mut path = audit_path(m, &leaves[..k]);
        path.push(tree_hash(&leaves[k..]));
        path
    } else {
        let mut path = audit_path(m - k, &leaves[k..]);
        path.push(tree_hash(&leaves[..k]));
        path
    }
}

/// A Rekor v1 log held in memory. Entries are appended, then served with
/// evidence against the tree as it stands when asked.
pub struct Log {
    key: SigningKey,
    bodies: Vec<Vec<u8>>,
    /// Added to an entry's shard-local index to give its global
    /// `logIndex`, as on a sharded public-good log. The SET covers the
    /// global index; the inclusion proof uses the local one.
    pub shard_offset: u64,
}

/// One entry's log evidence.
pub struct Evidence {
    pub set: Vec<u8>,
    pub hashes: Vec<[u8; 32]>,
    pub root: [u8; 32],
    pub tree_size: u64,
    pub checkpoint: String,
}

impl Log {
    pub fn new(key: SigningKey) -> Log {
        // Filler leaves so every proof has a few hashes.
        let bodies = (0..3).map(|i| format!("filler {i}").into_bytes()).collect();
        Log {
            key,
            bodies,
            shard_offset: 0,
        }
    }

    pub fn log_id(&self) -> [u8; 32] {
        sha256(&spki(&self.key))
    }

    pub fn append(&mut self, body: Vec<u8>) -> u64 {
        self.bodies.push(body);
        self.bodies.len() as u64 - 1
    }

    /// The global `logIndex` of the entry at shard-local `index`.
    pub fn global_index(&self, index: u64) -> u64 {
        index + self.shard_offset
    }

    pub fn body(&self, index: u64) -> &[u8] {
        &self.bodies[index as usize]
    }

    /// A signed checkpoint over the tree, by `key`.
    pub fn checkpoint(key: &SigningKey, tree_size: u64, root: &[u8; 32]) -> String {
        let note = format!("rekor.test - 1\n{tree_size}\n{}\n", wire::base64(root));
        let mut signature = sha256(&spki(key))[..4].to_vec();
        signature.extend_from_slice(&sign(key, note.as_bytes()));
        format!("{note}\n\u{2014} rekor.test {}\n", wire::base64(&signature))
    }

    pub fn evidence(&self, index: u64, integrated_time: i64) -> Evidence {
        let leaves: Vec<[u8; 32]> = self.bodies.iter().map(|b| leaf_hash(b)).collect();
        let root = tree_hash(&leaves);
        let tree_size = leaves.len() as u64;
        let set_payload = format!(
            r#"{{"body":"{}","integratedTime":{integrated_time},"logID":"{}","logIndex":{}}}"#,
            wire::base64(self.body(index)),
            hex(&self.log_id()),
            self.global_index(index),
        );
        Evidence {
            set: sign(&self.key, set_payload.as_bytes()),
            hashes: audit_path(index as usize, &leaves),
            root,
            tree_size,
            checkpoint: Log::checkpoint(&self.key, tree_size, &root),
        }
    }

    /// Entry `index` as the Rekor v1 API returns it: `{uuid: entry}`.
    pub fn api_entry(&self, index: u64) -> Value {
        let evidence = self.evidence(index, LOGGED_AT);
        let uuid = hex(&leaf_hash(self.body(index)));
        json!({ uuid: {
            "body": wire::base64(self.body(index)),
            "integratedTime": LOGGED_AT,
            "logID": hex(&self.log_id()),
            "logIndex": self.global_index(index),
            "verification": {
                "inclusionProof": {
                    "checkpoint": evidence.checkpoint,
                    "hashes": evidence.hashes.iter().map(|h| hex(h)).collect::<Vec<_>>(),
                    "logIndex": index,
                    "rootHash": hex(&evidence.root),
                    "treeSize": evidence.tree_size,
                },
                "signedEntryTimestamp": wire::base64(&evidence.set),
            }
        }})
    }

    /// Entry `index` as a v0.3 bundle's `tlogEntries` item.
    pub fn bundle_entry(&self, index: u64) -> Value {
        let evidence = self.evidence(index, LOGGED_AT);
        json!({
            "logIndex": self.global_index(index).to_string(),
            "logId": {"keyId": wire::base64(&self.log_id())},
            "kindVersion": {"kind": "hashedrekord", "version": "0.0.1"},
            "integratedTime": LOGGED_AT.to_string(),
            "inclusionPromise": {"signedEntryTimestamp": wire::base64(&evidence.set)},
            "inclusionProof": {
                "logIndex": index.to_string(),
                "rootHash": wire::base64(&evidence.root),
                "treeSize": evidence.tree_size.to_string(),
                "hashes": evidence.hashes.iter().map(|h| wire::base64(h)).collect::<Vec<_>>(),
                "checkpoint": {"envelope": evidence.checkpoint},
            },
            "canonicalizedBody": wire::base64(self.body(index)),
        })
    }
}

/// A v0.3 bundle for `artifact`, signed by `signer` with `certificate` and
/// logged in `log`.
pub fn bundle(log: &mut Log, artifact: &[u8], signer: &SigningKey, certificate: &[u8]) -> Value {
    let signature = sign(signer, artifact);
    let index = log.append(hashedrekord_body(artifact, &signature, certificate));
    json!({
        "mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
        "verificationMaterial": {
            "certificate": {"rawBytes": wire::base64(certificate)},
            "tlogEntries": [log.bundle_entry(index)],
        },
        "messageSignature": {
            "messageDigest": {"algorithm": "SHA2_256", "digest": wire::base64(&sha256(artifact))},
            "signature": wire::base64(&signature),
        }
    })
}

/// A v0.3 bundle whose content is a DSSE envelope instead of a message
/// signature: `signer`'s signature over an in-toto statement naming
/// `artifact`'s SHA-256 as its subject, logged in `log` as a `dsse` entry.
pub fn dsse_bundle(
    log: &mut Log,
    artifact: &[u8],
    signer: &SigningKey,
    certificate: &[u8],
) -> Value {
    let statement = json!({
        "_type": "https://in-toto.io/Statement/v1",
        "subject": [{"name": "manifest", "digest": {"sha256": hex(&sha256(artifact))}}],
        "predicateType": "https://example.test/approval",
        "predicate": {"approved": false},
    })
    .to_string();
    dsse_statement_bundle(
        log,
        "application/vnd.in-toto+json",
        statement.as_bytes(),
        signer,
        certificate,
    )
}

/// A v0.3 bundle whose content is a DSSE envelope over `payload` of
/// `payload_type`, signed by `signer` with `certificate` and logged in `log`
/// as a `dsse` 0.0.1 entry.
pub fn dsse_statement_bundle(
    log: &mut Log,
    payload_type: &str,
    payload: &[u8],
    signer: &SigningKey,
    certificate: &[u8],
) -> Value {
    let mut pae = format!(
        "DSSEv1 {} {payload_type} {} ",
        payload_type.len(),
        payload.len()
    )
    .into_bytes();
    pae.extend_from_slice(payload);
    let signature = sign(signer, &pae);
    let envelope = json!({
        "payload": wire::base64(payload),
        "payloadType": payload_type,
        "signatures": [{"sig": wire::base64(&signature)}],
    });
    let body = json!({
        "apiVersion": "0.0.1",
        "kind": "dsse",
        "spec": {
            "envelopeHash": {"algorithm": "sha256", "value": hex(&sha256(envelope.to_string().as_bytes()))},
            "payloadHash": {"algorithm": "sha256", "value": hex(&sha256(payload))},
            "signatures": [{
                "signature": wire::base64(&signature),
                "verifier": wire::base64(pem_certificate(certificate).as_bytes()),
            }],
        }
    });
    let index = log.append(body.to_string().into_bytes());
    let mut entry = log.bundle_entry(index);
    entry["kindVersion"] = json!({"kind": "dsse", "version": "0.0.1"});
    json!({
        "mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
        "verificationMaterial": {
            "certificate": {"rawBytes": wire::base64(certificate)},
            "tlogEntries": [entry],
        },
        "dsseEnvelope": envelope,
    })
}
