//! Trust decisions: MOAT Trust States (moat-spec.md §Trust State Error
//! Vocabulary) and the causes behind a refusal.

use std::fmt;
use std::time::Duration;

use crate::Refusal;

/// A MOAT Trust State. The crate never produces `MOAT_REVOKED`, which the
/// spec reserves.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TrustState {
    /// `MOAT_SIGNED`: the signature verified and the signer matched the pin.
    Signed,
    /// `MOAT_UNSIGNED`: no bundle and no pinned signing identity.
    Unsigned,
    /// `MOAT_INVALID`: cryptographic verification failed.
    Invalid,
    /// `MOAT_IDENTITY_MISMATCH`: the signature verified, but the wrong
    /// party signed it, or the declared identity differs from the pin.
    IdentityMismatch,
    /// `MOAT_IDENTITY_UNPINNED`: the signature verified against the declared
    /// identity, but the client has no pin for it yet (first fetch).
    IdentityUnpinned,
    /// `MOAT_TRUSTED_ROOT_STALE`: the bundled trusted root is past its
    /// freshness cliff, so verification did not run.
    TrustedRootStale,
}

impl TrustState {
    /// The state's MOAT identifier.
    pub fn id(self) -> &'static str {
        match self {
            TrustState::Signed => "MOAT_SIGNED",
            TrustState::Unsigned => "MOAT_UNSIGNED",
            TrustState::Invalid => "MOAT_INVALID",
            TrustState::IdentityMismatch => "MOAT_IDENTITY_MISMATCH",
            TrustState::IdentityUnpinned => "MOAT_IDENTITY_UNPINNED",
            TrustState::TrustedRootStale => "MOAT_TRUSTED_ROOT_STALE",
        }
    }
}

impl fmt::Display for TrustState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.id())
    }
}

/// A Sigstore identity: a Fulcio certificate's SAN and OIDC issuer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Identity {
    /// The SAN, which MOAT calls the subject.
    pub subject: String,
    /// The OIDC issuer.
    pub issuer: String,
}

impl fmt::Display for Identity {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} ({})", self.subject, self.issuer)
    }
}

/// Why a verification did not succeed. Each cause belongs to one
/// [`TrustState`], and its [`Cause::id`] is stable.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Cause {
    /// A trusted-root file does not parse as a valid Sigstore trusted root.
    TrustedRootMalformed(String),
    /// The bundled trusted root is older than the cliff.
    TrustedRootStale { age: Duration, max_age: Duration },
    /// The Sigstore bundle is not JSON or not a Sigstore bundle.
    BundleMalformed(String),
    /// `sigstore-verify` refused the bundle: chain, SCT, signature, digest,
    /// transparency log or timing.
    SigstoreVerification(String),
    /// The manifest is not JSON, or its `registry_signing_profile` is
    /// missing or has an empty or missing `issuer` or `subject`.
    SigningProfileMalformed(String),
    /// The certificate's identity is not the expected one.
    IdentityMismatch { expected: Identity, found: Identity },
    /// The manifest declares a signing profile that differs from the pin.
    ProfileChanged {
        pinned: Identity,
        declared: Identity,
    },
    /// The signature verified against the declared profile, and there is no
    /// pin to compare it with.
    ProfileUnpinned { declared: Identity },
    /// The Rekor log could not be reached or answered with an error.
    RekorUnavailable(String),
    /// No Rekor entry exists for the lookup, or none matches the signing
    /// profile's key.
    RekorEntryMissing(String),
    /// A Rekor entry is not the kind expected or does not parse.
    RekorEntryMalformed(String),
    /// A Rekor entry's data hash is not the hash of the expected message.
    RekorPayloadMismatch,
    /// A `rekord` entry's public key carries a comment.
    RekorPubkeyComment,
    /// A `rekord` entry's signed entry timestamp is missing or does not
    /// verify.
    SetInvalid(String),
    /// A `rekord` entry's inclusion proof is missing or does not verify.
    InclusionProofInvalid(String),
    /// A `rekord` entry's checkpoint does not parse, is not signed by the
    /// log, or does not match the inclusion proof.
    CheckpointInvalid(String),
    /// The author-key signature was refused.
    AuthorKey(Refusal),
    /// An attestation's DSSE payload is not an in-toto v1 statement.
    StatementMalformed(String),
    /// An attestation's statement names a predicate type other than the
    /// expected one.
    PredicateTypeMismatch { expected: String, found: String },
    /// No subject of an attestation's statement carries the SHA-256 of the
    /// artifact.
    SubjectDigestMismatch { digest: String },
}

impl Cause {
    /// The stable identifier for this cause. Author-key refusals keep their
    /// MOAT identifiers; every other cause is syllago's own.
    pub fn id(&self) -> &'static str {
        match self {
            Cause::TrustedRootMalformed(_) => "syllago.moat.trusted_root_malformed",
            Cause::TrustedRootStale { .. } => "syllago.moat.trusted_root_stale",
            Cause::BundleMalformed(_) => "syllago.moat.bundle_malformed",
            Cause::SigstoreVerification(_) => "syllago.moat.sigstore_verification_failed",
            Cause::SigningProfileMalformed(_) => "syllago.moat.signing_profile_malformed",
            Cause::IdentityMismatch { .. } => "syllago.moat.identity_mismatch",
            Cause::ProfileChanged { .. } => "syllago.moat.signing_profile_changed",
            Cause::ProfileUnpinned { .. } => "syllago.moat.signing_profile_unpinned",
            Cause::RekorUnavailable(_) => "syllago.moat.rekor_unavailable",
            Cause::RekorEntryMissing(_) => "syllago.moat.rekor_entry_missing",
            Cause::RekorEntryMalformed(_) => "syllago.moat.rekor_entry_malformed",
            Cause::RekorPayloadMismatch => "syllago.moat.rekor_payload_mismatch",
            Cause::RekorPubkeyComment => "syllago.moat.rekor_pubkey_comment",
            Cause::SetInvalid(_) => "syllago.moat.rekor_set_invalid",
            Cause::InclusionProofInvalid(_) => "syllago.moat.rekor_inclusion_proof_invalid",
            Cause::CheckpointInvalid(_) => "syllago.moat.rekor_checkpoint_invalid",
            Cause::AuthorKey(refusal) => refusal.id(),
            Cause::StatementMalformed(_) => "syllago.moat.statement_malformed",
            Cause::PredicateTypeMismatch { .. } => "syllago.moat.predicate_type_mismatch",
            Cause::SubjectDigestMismatch { .. } => "syllago.moat.subject_digest_mismatch",
        }
    }

    /// The Trust State this cause resolves to.
    pub fn state(&self) -> TrustState {
        match self {
            Cause::TrustedRootStale { .. } => TrustState::TrustedRootStale,
            Cause::IdentityMismatch { .. } | Cause::ProfileChanged { .. } => {
                TrustState::IdentityMismatch
            }
            Cause::ProfileUnpinned { .. } => TrustState::IdentityUnpinned,
            _ => TrustState::Invalid,
        }
    }
}

impl fmt::Display for Cause {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            Cause::TrustedRootMalformed(detail)
            | Cause::BundleMalformed(detail)
            | Cause::SigstoreVerification(detail)
            | Cause::SigningProfileMalformed(detail)
            | Cause::RekorUnavailable(detail)
            | Cause::RekorEntryMissing(detail)
            | Cause::RekorEntryMalformed(detail)
            | Cause::SetInvalid(detail)
            | Cause::InclusionProofInvalid(detail)
            | Cause::CheckpointInvalid(detail)
            | Cause::StatementMalformed(detail) => write!(f, "{id}: {detail}"),
            Cause::TrustedRootStale { age, max_age } => write!(
                f,
                "{id}: the bundled trusted root is {} days old, past the {}-day limit; \
                 update syllago or supply a current trusted root",
                age.as_secs() / 86_400,
                max_age.as_secs() / 86_400
            ),
            Cause::IdentityMismatch { expected, found } => {
                write!(f, "{id}: expected signer {expected}, found {found}")
            }
            Cause::ProfileChanged { pinned, declared } => write!(
                f,
                "{id}: the registry signing profile changed from {pinned} to {declared}; \
                 re-approval is required"
            ),
            Cause::ProfileUnpinned { declared } => write!(
                f,
                "{id}: no pinned signing profile; the manifest declares {declared}"
            ),
            Cause::RekorPayloadMismatch => write!(
                f,
                "{id}: the Rekor entry's data hash is not the hash of the expected message"
            ),
            Cause::RekorPubkeyComment => {
                write!(f, "{id}: the rekord entry's public key carries a comment")
            }
            Cause::AuthorKey(refusal) => refusal.fmt(f),
            Cause::PredicateTypeMismatch { expected, found } => write!(
                f,
                "{id}: the attestation's predicate type is {found:?}, not {expected:?}"
            ),
            Cause::SubjectDigestMismatch { digest } => write!(
                f,
                "{id}: no subject of the attestation has the fetched bytes' sha256 {digest}"
            ),
        }
    }
}

/// A verification that did not reach `MOAT_SIGNED`: the Trust State to
/// report and the cause behind it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TrustFailure {
    pub state: TrustState,
    pub cause: Cause,
}

impl TrustFailure {
    /// The Trust State's MOAT identifier.
    pub fn id(&self) -> &'static str {
        self.state.id()
    }
}

impl From<Cause> for TrustFailure {
    fn from(cause: Cause) -> TrustFailure {
        TrustFailure {
            state: cause.state(),
            cause,
        }
    }
}

impl fmt::Display for TrustFailure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.state, self.cause)
    }
}

impl std::error::Error for TrustFailure {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn states_map_to_moat_identifiers_and_never_revoked() {
        let all = [
            TrustState::Signed,
            TrustState::Unsigned,
            TrustState::Invalid,
            TrustState::IdentityMismatch,
            TrustState::IdentityUnpinned,
            TrustState::TrustedRootStale,
        ];
        let ids: Vec<_> = all.iter().map(|s| s.id()).collect();
        assert_eq!(
            ids,
            [
                "MOAT_SIGNED",
                "MOAT_UNSIGNED",
                "MOAT_INVALID",
                "MOAT_IDENTITY_MISMATCH",
                "MOAT_IDENTITY_UNPINNED",
                "MOAT_TRUSTED_ROOT_STALE"
            ]
        );
        assert!(!ids.contains(&"MOAT_REVOKED"));
    }

    #[test]
    fn causes_resolve_to_their_state() {
        let id = Identity {
            subject: "s".into(),
            issuer: "i".into(),
        };
        let failure = TrustFailure::from(Cause::ProfileChanged {
            pinned: id.clone(),
            declared: id.clone(),
        });
        assert_eq!(failure.id(), "MOAT_IDENTITY_MISMATCH");
        assert_eq!(
            TrustFailure::from(Cause::ProfileUnpinned { declared: id }).id(),
            "MOAT_IDENTITY_UNPINNED"
        );
        let author = Cause::AuthorKey(Refusal::SignerNotAllowed);
        assert_eq!(author.id(), "signer_not_allowed");
        assert_eq!(TrustFailure::from(author).id(), "MOAT_INVALID");
    }
}
