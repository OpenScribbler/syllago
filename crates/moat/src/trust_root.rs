//! Sigstore trusted roots (moat-spec.md §Trusted-Root Acquisition): the
//! bundled root and its staleness cliff, the two override modes and their
//! precedence, and the auditor-visible notice an override requires.
//!
//! The crate reads no files. The caller picks the source with
//! [`TrustRootOrigin::select`], reads the override file if one applies, and
//! passes its bytes to [`TrustRoot::from_override`].

use std::fmt;
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime};

use sigstore_verify::Verifier;
use sigstore_verify::crypto::Keyring;
use sigstore_verify::trust_root::{SIGSTORE_PRODUCTION_TRUSTED_ROOT, TrustedRoot};

use crate::crypto_provider;
use crate::trust::{Cause, TrustFailure};

/// When the bundled root was taken: the release date of `sigstore-verify`
/// 0.15.1, whose embedded public-good root syllago ships. Update it with
/// every `sigstore-verify` bump.
const BUNDLED_ROOT_DATE_UNIX: u64 = 1_792_713_600; // 2026-10-23T00:00:00Z

/// The bundled root's freshness cliff. The spec ties it to the public-good
/// instance's longest key-rotation interval, 12 months.
pub const BUNDLED_ROOT_MAX_AGE: Duration = Duration::from_secs(365 * 86_400);

/// Where a trusted root came from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TrustRootOrigin {
    /// The root embedded in the binary.
    Bundled,
    /// A file named on the registry's configuration entry.
    Registry { path: PathBuf, registry: String },
    /// A file passed at invocation.
    Invocation { path: PathBuf, registry: String },
}

impl TrustRootOrigin {
    /// Picks the source by the spec's precedence: an invocation override,
    /// then the registry's override, then the bundled root.
    pub fn select(
        invocation: Option<&Path>,
        registry_override: Option<&Path>,
        registry: &str,
    ) -> TrustRootOrigin {
        match (invocation, registry_override) {
            (Some(path), _) => TrustRootOrigin::Invocation {
                path: path.to_path_buf(),
                registry: registry.to_string(),
            },
            (None, Some(path)) => TrustRootOrigin::Registry {
                path: path.to_path_buf(),
                registry: registry.to_string(),
            },
            (None, None) => TrustRootOrigin::Bundled,
        }
    }
}

/// A parsed, validated Sigstore trusted root and where it came from.
pub struct TrustRoot {
    pub(crate) verifier: Verifier,
    /// The Rekor log keys, for the `rekord` checks `sigstore-verify` does
    /// not expose.
    pub(crate) rekor_keys: Keyring,
    origin: TrustRootOrigin,
}

impl fmt::Debug for TrustRoot {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("TrustRoot")
            .field("origin", &self.origin)
            .finish_non_exhaustive()
    }
}

impl TrustRoot {
    /// The bundled public-good root.
    pub fn bundled() -> TrustRoot {
        Self::parse(
            SIGSTORE_PRODUCTION_TRUSTED_ROOT.as_bytes(),
            TrustRootOrigin::Bundled,
        )
        .expect("the bundled trusted root is valid")
    }

    /// An override root from a file the caller read. The file must parse as
    /// a valid Sigstore trusted root. No staleness cliff applies to it.
    /// `origin` should name the override; with
    /// [`TrustRootOrigin::Bundled`] the cliff applies as for the bundled root.
    pub fn from_override(json: &[u8], origin: TrustRootOrigin) -> Result<TrustRoot, TrustFailure> {
        Self::parse(json, origin)
    }

    fn parse(json: &[u8], origin: TrustRootOrigin) -> Result<TrustRoot, TrustFailure> {
        crypto_provider::install();
        let malformed = |detail: String| TrustFailure::from(Cause::TrustedRootMalformed(detail));
        let text = std::str::from_utf8(json).map_err(|e| malformed(e.to_string()))?;
        let root = TrustedRoot::from_json(text).map_err(|e| malformed(e.to_string()))?;
        let verifier = Verifier::new(&root).map_err(|e| malformed(e.to_string()))?;
        let rekor_keys = root.rekor_keys().map_err(|e| malformed(e.to_string()))?;
        Ok(TrustRoot {
            verifier,
            rekor_keys,
            origin,
        })
    }

    /// Where this root came from.
    pub fn origin(&self) -> &TrustRootOrigin {
        &self.origin
    }

    /// The auditor-visible line the spec requires whenever an override is in
    /// effect, naming the path and the registry. `None` for the bundled root.
    pub fn audit_notice(&self) -> Option<String> {
        match &self.origin {
            TrustRootOrigin::Bundled => None,
            TrustRootOrigin::Registry { path, registry } => Some(format!(
                "using the trusted root from {} for registry {registry} (registry override)",
                path.display()
            )),
            TrustRootOrigin::Invocation { path, registry } => Some(format!(
                "using the trusted root from {} for registry {registry} (invocation override)",
                path.display()
            )),
        }
    }

    /// The bundled root's age at `now`, so a caller can warn before the
    /// cliff. `None` for an override root, whose freshness is the
    /// operator's.
    pub fn age(&self, now: SystemTime) -> Option<Duration> {
        if self.origin != TrustRootOrigin::Bundled {
            return None;
        }
        let taken = SystemTime::UNIX_EPOCH + Duration::from_secs(BUNDLED_ROOT_DATE_UNIX);
        Some(now.duration_since(taken).unwrap_or(Duration::ZERO))
    }

    /// Refuses a bundled root older than [`BUNDLED_ROOT_MAX_AGE`] with
    /// `MOAT_TRUSTED_ROOT_STALE`. Every verification calls this before it
    /// starts, so a stale root refuses even a bundle that would have
    /// verified.
    pub fn check_fresh(&self, now: SystemTime) -> Result<(), TrustFailure> {
        match self.age(now) {
            Some(age) if age > BUNDLED_ROOT_MAX_AGE => Err(Cause::TrustedRootStale {
                age,
                max_age: BUNDLED_ROOT_MAX_AGE,
            }
            .into()),
            _ => Ok(()),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use sha2::{Digest, Sha256};

    #[test]
    fn bundled_root_is_the_dated_snapshot() {
        // BUNDLED_ROOT_DATE_UNIX dates this exact file. A sigstore-verify
        // bump that changes it must update the date too.
        assert_eq!(
            crate::allowed_signers::hex(&Sha256::digest(
                SIGSTORE_PRODUCTION_TRUSTED_ROOT.as_bytes()
            )),
            "6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66"
        );
        let root = TrustRoot::bundled();
        assert_eq!(root.origin(), &TrustRootOrigin::Bundled);
        assert_eq!(root.audit_notice(), None);
    }

    #[test]
    fn bundled_root_goes_stale_after_the_cliff() {
        let root = TrustRoot::bundled();
        let taken = SystemTime::UNIX_EPOCH + Duration::from_secs(BUNDLED_ROOT_DATE_UNIX);
        assert!(root.check_fresh(taken + BUNDLED_ROOT_MAX_AGE).is_ok());
        assert_eq!(
            root.age(taken - Duration::from_secs(5)),
            Some(Duration::ZERO)
        );
        let err = root
            .check_fresh(taken + BUNDLED_ROOT_MAX_AGE + Duration::from_secs(1))
            .unwrap_err();
        assert_eq!(err.id(), "MOAT_TRUSTED_ROOT_STALE");
        assert_eq!(err.cause.id(), "syllago.moat.trusted_root_stale");
    }

    #[test]
    fn override_root_has_no_cliff_and_a_notice() {
        let origin = TrustRootOrigin::select(None, Some(Path::new("/etc/root.json")), "reg");
        let root =
            TrustRoot::from_override(SIGSTORE_PRODUCTION_TRUSTED_ROOT.as_bytes(), origin).unwrap();
        let far = SystemTime::UNIX_EPOCH + Duration::from_secs(BUNDLED_ROOT_DATE_UNIX * 2);
        assert_eq!(root.age(far), None);
        assert!(root.check_fresh(far).is_ok());
        let notice = root.audit_notice().unwrap();
        assert!(notice.contains("/etc/root.json") && notice.contains("reg"));
    }

    #[test]
    fn precedence_is_invocation_then_registry_then_bundled() {
        let inv = Path::new("/inv.json");
        let reg = Path::new("/reg.json");
        assert_eq!(
            TrustRootOrigin::select(Some(inv), Some(reg), "r"),
            TrustRootOrigin::Invocation {
                path: inv.into(),
                registry: "r".into()
            }
        );
        assert_eq!(
            TrustRootOrigin::select(None, Some(reg), "r"),
            TrustRootOrigin::Registry {
                path: reg.into(),
                registry: "r".into()
            }
        );
        assert_eq!(
            TrustRootOrigin::select(None, None, "r"),
            TrustRootOrigin::Bundled
        );
    }

    #[test]
    fn malformed_override_is_refused() {
        let origin = TrustRootOrigin::Invocation {
            path: "/x".into(),
            registry: "r".into(),
        };
        for bad in [&b"not json"[..], b"{\"tlogs\": 5}", b"\xff"] {
            let err = TrustRoot::from_override(bad, origin.clone()).unwrap_err();
            assert_eq!(err.id(), "MOAT_INVALID");
            assert_eq!(err.cause.id(), "syllago.moat.trusted_root_malformed");
        }
    }
}
