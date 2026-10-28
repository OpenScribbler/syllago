//! The capmon capability feed: fetch it, verify its build provenance fail
//! closed, and expose each provider's install-target rows.
//!
//! The trust chain is capmon's (its ADR 0004). capmon's publish workflow
//! attests `v2/index.json` with GitHub's build provenance, and the index names
//! the SHA-256 of every provider document. So the index is trusted only once
//! an attestation by that workflow names its digest, and a provider document
//! only once its bytes match the index. Nothing is parsed before the check
//! that covers it, and any failure refuses the whole feed. The attestation is
//! verified by the `moat` crate's Sigstore core, the one verifier syllago
//! has.
//!
//! The crate holds no path table: every row comes from the verified feed.
//! It reads only through a [`FeedSource`], so tests and the end-to-end test
//! run on a pinned snapshot ([`DirSource`]).

use std::fmt;
use std::path::PathBuf;
use std::time::SystemTime;

use acif::{EntryPoint, Kind};
use moat::{Identity, TrustFailure, TrustRoot, verify_attestation};
use serde_json::Value;
use sha2::{Digest, Sha256};

/// The feed major syllago reads. The URL major is capmon's compatibility
/// contract, so a new major is a code change here.
pub const MAJOR: &str = "v2";

/// The predicate type of GitHub's build-provenance attestation.
pub const PROVENANCE: &str = "https://slsa.dev/provenance/v1";

/// The only identity whose attestation the feed is trusted under: capmon's
/// publish workflow on `main`, signing through GitHub Actions OIDC.
pub fn publisher() -> Identity {
    Identity {
        subject:
            "https://github.com/OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main"
                .into(),
        issuer: "https://token.actions.githubusercontent.com".into(),
    }
}

/// The repository the publishing build must run in, as Fulcio's Source
/// Repository URI extension records it. The SAN names the workflow file;
/// this pins the repository whose run used it, so a reusable-workflow call
/// from another repository's build is never capmon's feed.
pub const SOURCE_REPOSITORY: &str = "https://github.com/OpenScribbler/capmon";

/// The content types the feed exposes rows for, with capmon's name for each.
const CONTENT_TYPES: [(Kind, &str); 2] = [(Kind::Skill, "skills"), (Kind::Rule, "rules")];

/// Where feed bytes come from.
pub trait FeedSource {
    /// The bytes published at `path`, `/`-separated and relative to the
    /// feed root, such as `v2/index.json`.
    fn fetch(&self, path: &str) -> Result<Vec<u8>, String>;

    /// The Sigstore bundles that attest an artifact with this SHA-256
    /// (lowercase hex), as GitHub's attestations API lists them for capmon.
    /// Empty when there are none.
    fn attestations(&self, sha256: &str) -> Result<Vec<Vec<u8>>, String>;
}

/// A feed snapshot on disk: the published files under their feed paths, and
/// each GitHub attestations API response at
/// `attestations/sha256-<hex>.json`.
#[derive(Debug, Clone)]
pub struct DirSource {
    root: PathBuf,
}

impl DirSource {
    pub fn new(root: impl Into<PathBuf>) -> DirSource {
        DirSource { root: root.into() }
    }
}

impl FeedSource for DirSource {
    fn fetch(&self, path: &str) -> Result<Vec<u8>, String> {
        let file = path.split('/').fold(self.root.clone(), |p, s| p.join(s));
        std::fs::read(&file).map_err(|e| e.to_string())
    }

    fn attestations(&self, sha256: &str) -> Result<Vec<Vec<u8>>, String> {
        let file = self
            .root
            .join("attestations")
            .join(format!("sha256-{sha256}.json"));
        match std::fs::read(&file) {
            Ok(bytes) => parse_attestations_response(&bytes),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Vec::new()),
            Err(e) => Err(e.to_string()),
        }
    }
}

/// The bundles in a GitHub attestations API response,
/// `{"attestations": [{"bundle": {...}}, ...]}`, each re-encoded as JSON.
pub fn parse_attestations_response(bytes: &[u8]) -> Result<Vec<Vec<u8>>, String> {
    let response: Value = serde_json::from_slice(bytes)
        .map_err(|_| "the attestations response is not JSON".to_string())?;
    response["attestations"]
        .as_array()
        .ok_or_else(|| "the attestations response has no attestations list".to_string())?
        .iter()
        .map(|attestation| match &attestation["bundle"] {
            Value::Object(_) => Ok(attestation["bundle"].to_string().into_bytes()),
            _ => Err("an attestation has no bundle".to_string()),
        })
        .collect()
}

/// A verified feed: the rows of each provider it was loaded for.
#[derive(Debug, Clone)]
pub struct Feed {
    index_sha256: String,
    generated_at: String,
    source_commit: Option<String>,
    providers: Vec<Provider>,
}

#[derive(Debug, Clone)]
struct Provider {
    slug: String,
    rows: Vec<(Kind, Vec<EntryPoint>)>,
}

impl Feed {
    /// The SHA-256 of the attested index, which identifies this feed.
    pub fn index_sha256(&self) -> &str {
        &self.index_sha256
    }

    /// The index's `generated_at`.
    pub fn generated_at(&self) -> &str {
        &self.generated_at
    }

    /// The capmon commit the index was exported from, when it names one.
    pub fn source_commit(&self) -> Option<&str> {
        self.source_commit.as_deref()
    }

    /// The ordered rows for `provider` and `kind`. `None` when the feed was
    /// not loaded for `provider` or does not expose `kind`; an empty list
    /// when the provider publishes no rows for it.
    pub fn entry_points(&self, provider: &str, kind: Kind) -> Option<&[EntryPoint]> {
        self.providers
            .iter()
            .find(|p| p.slug == provider)?
            .rows
            .iter()
            .find(|(k, _)| *k == kind)
            .map(|(_, rows)| rows.as_slice())
    }
}

/// Fetches and verifies the feed, then reads the documents of `providers`.
///
/// The index is refused unless one of its attestations verifies under
/// `root` at `now` as [`PROVENANCE`] by [`publisher`] over the index's
/// bytes; each provider document is refused unless its bytes have the
/// SHA-256 the index names for it.
pub fn load(
    source: &dyn FeedSource,
    root: &TrustRoot,
    now: SystemTime,
    providers: &[&str],
) -> Result<Feed, FeedError> {
    let index_path = format!("{MAJOR}/index.json");
    let index_bytes = fetch(source, &index_path)?;
    let index_sha256 = hex(&Sha256::digest(&index_bytes));
    let bundles = source
        .attestations(&index_sha256)
        .map_err(|detail| FeedError::Fetch {
            path: format!("attestations for sha256:{index_sha256}"),
            detail,
        })?;
    if bundles.is_empty() {
        return Err(FeedError::AttestationMissing {
            digest: index_sha256,
        });
    }
    let mut failures = Vec::new();
    for bundle in &bundles {
        match verify_attestation(&index_bytes, bundle, PROVENANCE, &publisher(), root, now) {
            Ok(attested)
                if attested.source_repository_uri.as_deref() == Some(SOURCE_REPOSITORY) =>
            {
                failures.clear();
                break;
            }
            Ok(attested) => failures.push(BundleFailure::SourceRepositoryMismatch {
                found: attested.source_repository_uri,
            }),
            Err(failure) => failures.push(BundleFailure::Trust(failure)),
        }
    }
    if !failures.is_empty() {
        return Err(FeedError::AttestationRefused {
            digest: index_sha256,
            failures,
        });
    }

    let index: Value = serde_json::from_slice(&index_bytes)
        .map_err(|_| FeedError::IndexMalformed("the index is not JSON".into()))?;
    let listed = index["providers"]
        .as_array()
        .ok_or_else(|| FeedError::IndexMalformed("the index has no providers list".into()))?;
    let text = |name: &str| index[name].as_str().map(str::to_string);
    let mut feed = Feed {
        index_sha256,
        generated_at: text("generated_at").unwrap_or_default(),
        source_commit: text("source_commit"),
        providers: Vec::new(),
    };
    for &slug in providers {
        let mut entries = listed.iter().filter(|entry| entry["slug"] == slug);
        let entry = entries.next().ok_or_else(|| FeedError::ProviderMissing {
            slug: slug.to_string(),
        })?;
        if entries.next().is_some() {
            return Err(FeedError::IndexMalformed(format!(
                "the index lists {slug} more than once"
            )));
        }
        let (Some(relative), Some(expected)) = (entry["path"].as_str(), entry["sha256"].as_str())
        else {
            return Err(FeedError::IndexMalformed(format!(
                "the index entry for {slug} has no path or sha256"
            )));
        };
        if !is_plain_relative(relative) {
            return Err(FeedError::IndexMalformed(format!(
                "the index entry for {slug} has the path {relative:?}"
            )));
        }
        let path = format!("{MAJOR}/{relative}");
        let bytes = fetch(source, &path)?;
        let found = hex(&Sha256::digest(&bytes));
        if found != expected {
            return Err(FeedError::DocumentDigestMismatch {
                path,
                expected: expected.to_string(),
                found,
            });
        }
        feed.providers.push(read_provider(slug, &path, &bytes)?);
    }
    Ok(feed)
}

fn fetch(source: &dyn FeedSource, path: &str) -> Result<Vec<u8>, FeedError> {
    source.fetch(path).map_err(|detail| FeedError::Fetch {
        path: path.to_string(),
        detail,
    })
}

/// Reads the skill and rule rows from a verified provider document.
fn read_provider(slug: &str, path: &str, bytes: &[u8]) -> Result<Provider, FeedError> {
    let malformed = |detail: String| FeedError::DocumentMalformed {
        path: path.to_string(),
        detail,
    };
    let document: Value =
        serde_json::from_slice(bytes).map_err(|_| malformed("the document is not JSON".into()))?;
    if document["slug"] != slug {
        return Err(malformed(format!("the document is not {slug}'s")));
    }
    let mut rows = Vec::new();
    for (kind, name) in CONTENT_TYPES {
        let list = match &document["content_types"][name]["install_entry_points"] {
            Value::Null => Vec::new(),
            Value::Array(list) => list
                .iter()
                .map(EntryPoint::from_value)
                .collect::<Result<_, _>>()
                .map_err(|e| malformed(format!("{name}: {e}")))?,
            _ => {
                return Err(malformed(format!(
                    "{name}.install_entry_points is not a list"
                )));
            }
        };
        rows.push((kind, list));
    }
    Ok(Provider {
        slug: slug.to_string(),
        rows,
    })
}

/// A path of plain segments: no root, no empty, `.` or `..` segment, and no
/// `\` or `:`, so it stays under the feed root on every OS.
fn is_plain_relative(path: &str) -> bool {
    path.split('/')
        .all(|s| !s.is_empty() && s != "." && s != ".." && !s.contains(['\\', ':']))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// Why one attestation of the index was not accepted.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BundleFailure {
    /// The attestation did not verify as [`publisher`]'s provenance over the
    /// index.
    Trust(TrustFailure),
    /// It verified, but the certificate does not name [`SOURCE_REPOSITORY`]
    /// as the repository the build ran in.
    SourceRepositoryMismatch { found: Option<String> },
}

impl BundleFailure {
    /// The trust cause's identifier, or syllago's own for a repository
    /// mismatch.
    pub fn id(&self) -> &'static str {
        match self {
            BundleFailure::Trust(failure) => failure.cause.id(),
            BundleFailure::SourceRepositoryMismatch { .. } => {
                "syllago.feed.source_repository_mismatch"
            }
        }
    }
}

impl fmt::Display for BundleFailure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            BundleFailure::Trust(failure) => failure.fmt(f),
            BundleFailure::SourceRepositoryMismatch { found } => write!(
                f,
                "{}: the build ran in {}, not {SOURCE_REPOSITORY}",
                self.id(),
                found.as_deref().unwrap_or("an unnamed repository")
            ),
        }
    }
}

/// Why the feed was refused. Each has a stable [`FeedError::id`].
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FeedError {
    /// A feed file or the attestation list could not be fetched.
    Fetch { path: String, detail: String },
    /// No attestation names the index's digest.
    AttestationMissing { digest: String },
    /// Attestations exist, and none verifies; one failure per bundle.
    AttestationRefused {
        digest: String,
        failures: Vec<BundleFailure>,
    },
    /// The attested index is not the shape syllago reads.
    IndexMalformed(String),
    /// The index does not list a requested provider.
    ProviderMissing { slug: String },
    /// A provider document's bytes do not have the digest the index names.
    DocumentDigestMismatch {
        path: String,
        expected: String,
        found: String,
    },
    /// A verified provider document is not the shape syllago reads.
    DocumentMalformed { path: String, detail: String },
}

impl FeedError {
    pub fn id(&self) -> &'static str {
        match self {
            FeedError::Fetch { .. } => "syllago.feed.fetch_failed",
            FeedError::AttestationMissing { .. } => "syllago.feed.attestation_missing",
            FeedError::AttestationRefused { .. } => "syllago.feed.attestation_refused",
            FeedError::IndexMalformed(_) => "syllago.feed.index_malformed",
            FeedError::ProviderMissing { .. } => "syllago.feed.provider_missing",
            FeedError::DocumentDigestMismatch { .. } => "syllago.feed.document_digest_mismatch",
            FeedError::DocumentMalformed { .. } => "syllago.feed.document_malformed",
        }
    }
}

impl fmt::Display for FeedError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            FeedError::Fetch { path, detail } => {
                write!(f, "{id}: could not fetch {path}: {detail}; try again later")
            }
            FeedError::AttestationMissing { digest } => write!(
                f,
                "{id}: no build-provenance attestation names the feed index sha256:{digest}; \
                 the feed may be mid-deploy, so try again in a few minutes"
            ),
            FeedError::AttestationRefused { digest, failures } => {
                write!(
                    f,
                    "{id}: no attestation of the feed index sha256:{digest} verifies as capmon's \
                     publish workflow; the feed is not trusted"
                )?;
                for failure in failures {
                    write!(f, "; {failure}")?;
                }
                Ok(())
            }
            FeedError::IndexMalformed(detail) => write!(f, "{id}: {detail}"),
            FeedError::ProviderMissing { slug } => {
                write!(f, "{id}: the feed lists no provider {slug}")
            }
            FeedError::DocumentDigestMismatch {
                path,
                expected,
                found,
            } => write!(
                f,
                "{id}: {path} has sha256 {found}, but the attested index names {expected}; \
                 the feed may be mid-deploy, so try again in a few minutes"
            ),
            FeedError::DocumentMalformed { path, detail } => write!(f, "{id}: {path}: {detail}"),
        }
    }
}

impl std::error::Error for FeedError {}
