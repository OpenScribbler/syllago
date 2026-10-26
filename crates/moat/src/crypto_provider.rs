//! A `sigstore-crypto` provider built on RustCrypto, so the crate links no
//! C crypto library.
//!
//! `sigstore-verify` with its default features off has no crypto backend and
//! panics on first use unless one is installed. This provider covers what
//! the Sigstore public-good instance signs with: ECDSA P-256 and P-384 with
//! SHA-256 or SHA-384 (Fulcio's CA chain, CT logs, Rekor v1) and Ed25519
//! (Rekor v2). RSA and ML-DSA are refused as unsupported; a trusted root that
//! relies on them fails to verify rather than verifying with a weaker check.

use std::sync::Once;

use ecdsa_core::signature::hazmat::PrehashVerifier;
use p256::ecdsa as ecdsa_core;
use sha2::Digest;
use sigstore_verify::crypto::{
    CryptoProvider, Error, HashContext, Result, SigningScheme, install_default_provider,
};
use sigstore_verify::types::HashAlgorithm;

#[derive(Debug)]
struct RustCrypto;

static PROVIDER: RustCrypto = RustCrypto;
static INSTALL: Once = Once::new();

/// Installs this provider as the process-wide default before any Sigstore
/// verification runs. If another provider is already installed, that one
/// stays: `sigstore-crypto` allows only one, and any provider it accepts
/// implements the same checks.
pub(crate) fn install() {
    INSTALL.call_once(|| {
        let _ = install_default_provider(&PROVIDER);
    });
}

struct Hasher<D>(D);

impl<D: Digest + Send> HashContext for Hasher<D> {
    fn update(&mut self, data: &[u8]) {
        Digest::update(&mut self.0, data);
    }

    fn finalize(self: Box<Self>) -> Vec<u8> {
        self.0.finalize().to_vec()
    }
}

fn digest(algorithm: HashAlgorithm, data: &[u8]) -> Result<Vec<u8>> {
    match algorithm {
        HashAlgorithm::Sha2256 => Ok(sha2::Sha256::digest(data).to_vec()),
        HashAlgorithm::Sha2384 => Ok(sha2::Sha384::digest(data).to_vec()),
        HashAlgorithm::Sha2512 => Ok(sha2::Sha512::digest(data).to_vec()),
        other => Err(Error::UnsupportedAlgorithm(format!(
            "cannot compute {other} digests"
        ))),
    }
}

fn unsupported(scheme: SigningScheme) -> Error {
    Error::UnsupportedAlgorithm(format!(
        "{} is not supported by syllago's RustCrypto provider",
        scheme.name()
    ))
}

fn invalid(scheme: SigningScheme) -> Error {
    Error::Verification(format!("{} signature invalid", scheme.name()))
}

/// Verifies an ASN.1 DER ECDSA signature over `prehash`. `key` is an
/// uncompressed SEC1 point.
fn verify_ecdsa(scheme: SigningScheme, key: &[u8], prehash: &[u8], signature: &[u8]) -> Result<()> {
    let ok = match scheme {
        SigningScheme::EcdsaP256Sha256 | SigningScheme::EcdsaP256Sha384 => {
            let key = p256::ecdsa::VerifyingKey::from_sec1_bytes(key)
                .map_err(|_| Error::InvalidKey("bad P-256 public key".into()))?;
            let sig = p256::ecdsa::Signature::from_der(signature).map_err(|_| invalid(scheme))?;
            key.verify_prehash(prehash, &sig).is_ok()
        }
        SigningScheme::EcdsaP384Sha256 | SigningScheme::EcdsaP384Sha384 => {
            let key = p384::ecdsa::VerifyingKey::from_sec1_bytes(key)
                .map_err(|_| Error::InvalidKey("bad P-384 public key".into()))?;
            let sig = p384::ecdsa::Signature::from_der(signature).map_err(|_| invalid(scheme))?;
            key.verify_prehash(prehash, &sig).is_ok()
        }
        _ => return Err(unsupported(scheme)),
    };
    if ok { Ok(()) } else { Err(invalid(scheme)) }
}

impl CryptoProvider for RustCrypto {
    fn hash(&self, algorithm: HashAlgorithm) -> Result<Box<dyn HashContext>> {
        match algorithm {
            HashAlgorithm::Sha2256 => Ok(Box::new(Hasher(sha2::Sha256::new()))),
            HashAlgorithm::Sha2384 => Ok(Box::new(Hasher(sha2::Sha384::new()))),
            HashAlgorithm::Sha2512 => Ok(Box::new(Hasher(sha2::Sha512::new()))),
            other => Err(Error::UnsupportedAlgorithm(format!(
                "cannot compute {other} digests"
            ))),
        }
    }

    fn verify_raw(
        &self,
        scheme: SigningScheme,
        spki_pubkey_bytes: &[u8],
        data: &[u8],
        signature: &[u8],
    ) -> Result<()> {
        match scheme {
            SigningScheme::Ed25519 => {
                let key: &[u8; 32] = spki_pubkey_bytes
                    .try_into()
                    .map_err(|_| Error::InvalidKey("bad Ed25519 public key".into()))?;
                let key = ed25519_dalek::VerifyingKey::from_bytes(key)
                    .map_err(|_| Error::InvalidKey("bad Ed25519 public key".into()))?;
                let sig =
                    ed25519_dalek::Signature::from_slice(signature).map_err(|_| invalid(scheme))?;
                key.verify_strict(data, &sig).map_err(|_| invalid(scheme))
            }
            _ => {
                let algorithm = scheme.hash_algorithm().ok_or_else(|| unsupported(scheme))?;
                let prehash = digest(algorithm, data)?;
                verify_ecdsa(scheme, spki_pubkey_bytes, &prehash, signature)
            }
        }
    }

    fn verify_prehashed(
        &self,
        scheme: SigningScheme,
        spki_pubkey_bytes: &[u8],
        digest: &[u8],
        signature: &[u8],
    ) -> Result<()> {
        // RustCrypto truncates a long digest and accepts a short one; the
        // digest must be exactly the scheme's hash output.
        let algorithm = scheme.hash_algorithm().ok_or_else(|| unsupported(scheme))?;
        if digest.len() != algorithm.digest_size() {
            return Err(invalid(scheme));
        }
        verify_ecdsa(scheme, spki_pubkey_bytes, digest, signature)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use p256::ecdsa::signature::Signer;

    fn p256_key() -> p256::ecdsa::SigningKey {
        p256::ecdsa::SigningKey::from_slice(&[7u8; 32]).unwrap()
    }

    #[test]
    fn verifies_p256_raw_and_prehashed_and_refuses_tamper() {
        let key = p256_key();
        let point = key.verifying_key().to_sec1_point(false);
        let sig: p256::ecdsa::Signature = key.sign(b"message");
        let der = sig.to_der();
        let scheme = SigningScheme::EcdsaP256Sha256;
        assert!(
            PROVIDER
                .verify_raw(scheme, point.as_bytes(), b"message", der.as_bytes())
                .is_ok()
        );
        assert!(
            PROVIDER
                .verify_raw(scheme, point.as_bytes(), b"messagf", der.as_bytes())
                .is_err()
        );
        let prehash = sha2::Sha256::digest(b"message");
        assert!(
            PROVIDER
                .verify_prehashed(scheme, point.as_bytes(), &prehash, der.as_bytes())
                .is_ok()
        );
    }

    #[test]
    fn prehashed_digest_must_be_the_schemes_length() {
        use p256::ecdsa::signature::hazmat::PrehashSigner;
        let key = p256_key();
        let point = key.verifying_key().to_sec1_point(false);
        let scheme = SigningScheme::EcdsaP256Sha256;
        // RustCrypto truncates a long digest to the field size, so a SHA-256
        // signature also verifies over its digest with bytes appended.
        let sig: p256::ecdsa::Signature = key.sign(b"message");
        let mut long = sha2::Sha256::digest(b"message").to_vec();
        long.extend_from_slice(&[0; 16]);
        // And it accepts a digest shorter than the field.
        let short = [1u8; 24];
        let short_sig: p256::ecdsa::Signature = key.sign_prehash(&short).unwrap();
        for (digest, sig) in [(&long[..], sig), (&short[..], short_sig)] {
            let err = PROVIDER
                .verify_prehashed(scheme, point.as_bytes(), digest, sig.to_der().as_bytes())
                .unwrap_err();
            assert!(matches!(err, Error::Verification(_)), "{err:?}");
        }
    }

    #[test]
    fn verifies_p384() {
        let key = p384::ecdsa::SigningKey::from_slice(&[9u8; 48]).unwrap();
        let point = key.verifying_key().to_sec1_point(false);
        let sig: p384::ecdsa::Signature = key.sign(b"message");
        let der = sig.to_der();
        let scheme = SigningScheme::EcdsaP384Sha384;
        assert!(
            PROVIDER
                .verify_raw(scheme, point.as_bytes(), b"message", der.as_bytes())
                .is_ok()
        );
        assert!(
            PROVIDER
                .verify_raw(scheme, point.as_bytes(), b"other", der.as_bytes())
                .is_err()
        );
    }

    #[test]
    fn refuses_rsa_as_unsupported() {
        let err = PROVIDER
            .verify_raw(SigningScheme::RsaPkcs1Sha256, &[0; 10], b"m", &[0; 10])
            .unwrap_err();
        assert!(matches!(err, Error::UnsupportedAlgorithm(_)));
    }

    #[test]
    fn hashes_streaming() {
        let mut ctx = PROVIDER.hash(HashAlgorithm::Sha2256).unwrap();
        ctx.update(b"ab");
        ctx.update(b"c");
        assert_eq!(ctx.finalize(), sha2::Sha256::digest(b"abc").to_vec());
    }
}
