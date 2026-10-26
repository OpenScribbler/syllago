//! A software security-key signer for tests, ported from the MOAT
//! reference's `sign_sk_ed25519_emulated` and `sign_sk_ecdsa_emulated`. It
//! signs as a FIDO authenticator would, with the reference's defaults:
//! application `ssh:` and counter 1. It is not constant-time and its ECDSA
//! nonce is derived from the key and digest, so it must never sign real
//! content. It exists only under the `testing` feature, which only this
//! crate's dev-dependency enables; CI checks that within the workspace.

use ed25519_dalek::Signer as _;
use p256::elliptic_curve::Field as _;
use p256::elliptic_curve::ops::Reduce;
use p256::elliptic_curve::point::AffineCoordinates;
use p256::{FieldBytes, ProjectivePoint, Scalar};
use sha2::{Digest, Sha256};

use crate::KeyType;
use crate::sign::{SignError, Signer, SshPublicKey};
use crate::sshsig::{BEGIN, END, HASH_ALGORITHM, MAGIC, NAMESPACE, SK_USER_PRESENT, signed_data};
use crate::wire::{base64, put_string};

const APPLICATION: &[u8] = b"ssh:";
const COUNTER: u32 = 1;

enum Key {
    Ed25519(ed25519_dalek::SigningKey),
    EcdsaP256(Scalar),
}

/// An emulated `sk-ssh-ed25519@openssh.com` or
/// `sk-ecdsa-sha2-nistp256@openssh.com` signer.
pub struct EmulatedSkSigner {
    key: Key,
    public_key: SshPublicKey,
    flags: u8,
}

impl EmulatedSkSigner {
    /// An `sk-ssh-ed25519@openssh.com` signer for an Ed25519 seed.
    pub fn ed25519(seed: [u8; 32]) -> EmulatedSkSigner {
        let key = ed25519_dalek::SigningKey::from_bytes(&seed);
        let mut blob = Vec::new();
        put_string(&mut blob, KeyType::SkEd25519.as_str().as_bytes());
        put_string(&mut blob, key.verifying_key().as_bytes());
        put_string(&mut blob, APPLICATION);
        EmulatedSkSigner::new(Key::Ed25519(key), blob)
    }

    /// An `sk-ecdsa-sha2-nistp256@openssh.com` signer for a P-256 scalar,
    /// big-endian and reduced mod n, as the reference's fixtures reduce it.
    /// Panics when the scalar reduces to zero.
    pub fn ecdsa_p256(scalar: [u8; 32]) -> EmulatedSkSigner {
        let d = <Scalar as Reduce<FieldBytes>>::reduce(&scalar.into());
        assert!(!bool::from(d.is_zero()), "the P-256 scalar is zero mod n");
        let point = (ProjectivePoint::GENERATOR * d).to_affine();
        let mut q = vec![0x04];
        q.extend_from_slice(&point.x());
        q.extend_from_slice(&point.y());
        let mut blob = Vec::new();
        put_string(&mut blob, KeyType::SkEcdsaP256.as_str().as_bytes());
        put_string(&mut blob, b"nistp256");
        put_string(&mut blob, &q);
        put_string(&mut blob, APPLICATION);
        EmulatedSkSigner::new(Key::EcdsaP256(d), blob)
    }

    fn new(key: Key, blob: Vec<u8>) -> EmulatedSkSigner {
        EmulatedSkSigner {
            key,
            public_key: SshPublicKey::from_blob(blob).expect("an accepted key type"),
            flags: SK_USER_PRESENT,
        }
    }

    /// Signs with `flags` as the authenticator's flags byte instead of the
    /// user-presence flag alone.
    pub fn with_flags(mut self, flags: u8) -> EmulatedSkSigner {
        self.flags = flags;
        self
    }
}

impl Signer for EmulatedSkSigner {
    fn public_key(&self) -> &SshPublicKey {
        &self.public_key
    }

    fn sign(&self, message: &[u8]) -> Result<String, SignError> {
        // The bytes a FIDO authenticator signs (PROTOCOL.u2f).
        let mut signed = Sha256::digest(APPLICATION).to_vec();
        signed.push(self.flags);
        signed.extend_from_slice(&COUNTER.to_be_bytes());
        signed.extend_from_slice(&Sha256::digest(signed_data(message)));

        let key_type = self.public_key.key_type().as_str().as_bytes();
        let mut field = Vec::new();
        put_string(&mut field, key_type);
        match &self.key {
            Key::Ed25519(key) => put_string(&mut field, &key.sign(&signed).to_bytes()),
            Key::EcdsaP256(d) => {
                let (r, s) = ecdsa_sign(d, &Sha256::digest(&signed).into());
                let mut rs = Vec::new();
                put_string(&mut rs, &mpint(&r.to_bytes().into()));
                put_string(&mut rs, &mpint(&s.to_bytes().into()));
                put_string(&mut field, &rs);
            }
        }
        field.push(self.flags);
        field.extend_from_slice(&COUNTER.to_be_bytes());

        let mut blob = MAGIC.to_vec();
        blob.extend_from_slice(&1u32.to_be_bytes());
        put_string(&mut blob, self.public_key.blob());
        put_string(&mut blob, NAMESPACE);
        put_string(&mut blob, b"");
        put_string(&mut blob, HASH_ALGORITHM);
        put_string(&mut blob, &field);
        Ok(armor(&blob))
    }
}

/// The reference's `p256_sign_test_only`: ECDSA with the nonce
/// SHA-256(`moat-test-nonce` || d || digest) mod n and no low-S
/// normalization.
fn ecdsa_sign(d: &Scalar, digest: &[u8; 32]) -> (Scalar, Scalar) {
    let e = <Scalar as Reduce<FieldBytes>>::reduce(&(*digest).into());
    let mut nonce = b"moat-test-nonce".to_vec();
    nonce.extend_from_slice(&d.to_bytes());
    nonce.extend_from_slice(digest);
    let nonce: [u8; 32] = Sha256::digest(&nonce).into();
    let k = <Scalar as Reduce<FieldBytes>>::reduce(&nonce.into());
    let x = (ProjectivePoint::GENERATOR * k).to_affine().x();
    let r = <Scalar as Reduce<FieldBytes>>::reduce(&x);
    let s = k.invert().expect("a nonzero nonce") * (e + r * d);
    (r, s)
}

/// An unsigned big-endian integer as an SSH mpint's content: no leading
/// zero bytes, and one zero byte prepended when the high bit is set.
fn mpint(bytes: &[u8; 32]) -> Vec<u8> {
    let start = bytes.iter().position(|&b| b != 0).unwrap_or(bytes.len());
    let mut out = Vec::new();
    if bytes.get(start).is_some_and(|&b| b & 0x80 != 0) {
        out.push(0);
    }
    out.extend_from_slice(&bytes[start..]);
    out
}

/// The reference's `_armor`: base64 in lines of 70 characters between the
/// BEGIN and END lines, ending with LF.
fn armor(blob: &[u8]) -> String {
    let text = base64(blob);
    let mut out = format!("{BEGIN}\n");
    for line in text.as_bytes().chunks(70) {
        out.push_str(std::str::from_utf8(line).expect("base64 is ASCII"));
        out.push('\n');
    }
    out.push_str(END);
    out.push('\n');
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn mpint_matches_the_reference() {
        let mut n = [0u8; 32];
        assert_eq!(mpint(&n), b"");
        n[31] = 0x7f;
        assert_eq!(mpint(&n), [0x7f]);
        n[30] = 0x80;
        assert_eq!(mpint(&n), [0x00, 0x80, 0x7f]);
    }
}
