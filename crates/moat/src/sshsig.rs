//! SSHSIG verification ([PROTOCOL.sshsig]) narrowed to the MOAT Author-Key
//! Profile: namespace `file`, hash `sha512`, and the three accepted key
//! types, with the user-presence flag required on security-key signatures.
//!
//! [PROTOCOL.sshsig]: https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig

use p256::ecdsa::signature::hazmat::PrehashVerifier;
use sha2::{Digest, Sha256, Sha512};

use crate::wire::{Reader, base64_decode, put_string, py_splitlines, py_strip};
use crate::{Fingerprint, KeyType, VerifiedSigner};

pub(crate) const MAGIC: &[u8] = b"SSHSIG";
pub(crate) const NAMESPACE: &[u8] = b"file";
pub(crate) const HASH_ALGORITHM: &[u8] = b"sha512";
pub(crate) const BEGIN: &str = "-----BEGIN SSH SIGNATURE-----";
pub(crate) const END: &str = "-----END SSH SIGNATURE-----";
pub(crate) const SK_USER_PRESENT: u8 = 0x01;

/// The longest armored signature read, in UTF-8 bytes, as in the reference.
/// Real armor for the profile's key types is under 1 KiB. Without the cap, a
/// signature of millions of blank lines allocates a slice per line before
/// anything is checked.
const MAX_ARMOR_BYTES: usize = 16 * 1024;

/// The prime 2^255 - 19, little-endian.
const ED25519_P: [u8; 32] = {
    let mut p = [0xff; 32];
    p[0] = 0xed;
    p[31] = 0x7f;
    p
};

/// Verifies an armored SSHSIG over `message`, returning the signer, or
/// `None` when the signature is not valid under the profile.
pub(crate) fn verify(armored: &str, message: &[u8]) -> Option<VerifiedSigner> {
    let blob = unarmor(armored)?;
    let mut sig = Reader::new(&blob);
    if sig.take(MAGIC.len())? != MAGIC || sig.u32()? != 1 {
        return None;
    }
    let key_blob = sig.string()?;
    let namespace = sig.string()?;
    let _reserved = sig.string()?;
    let hash_algorithm = sig.string()?;
    let sig_field = sig.string()?;
    if !sig.done() || namespace != NAMESPACE || hash_algorithm != HASH_ALGORITHM {
        return None;
    }
    let data = signed_data(message);

    let mut key = Reader::new(key_blob);
    let mut sig = Reader::new(sig_field);
    let key_type_name = key.string()?;
    if sig.string()? != key_type_name {
        return None;
    }
    let key_type = KeyType::from_wire(key_type_name)?;
    let ok = match key_type {
        KeyType::Ed25519 => {
            let public = key.string()?;
            let signature = sig.string()?;
            key.done() && sig.done() && ed25519_verify(public, &data, signature)
        }
        KeyType::SkEd25519 => {
            let public = key.string()?;
            let signature = sig.string()?;
            let signed = sk_signed(&mut key, &mut sig, &data)?;
            ed25519_verify(public, &signed, signature)
        }
        KeyType::SkEcdsaP256 => {
            if key.string()? != b"nistp256" {
                return None;
            }
            let public = key.string()?;
            let mut inner = Reader::new(sig.string()?);
            let r = inner.string()?;
            let s = inner.string()?;
            if !inner.done() {
                return None;
            }
            let signed = sk_signed(&mut key, &mut sig, &data)?;
            p256_verify(public, &Sha256::digest(&signed), r, s)
        }
    };
    ok.then(|| VerifiedSigner {
        fingerprint: Fingerprint::of_key_blob(key_blob),
        key_type,
    })
}

/// Python's `_unarmor`: strip, split into lines, strip each line, require
/// the BEGIN and END lines, and decode what lies between. Text longer than
/// [`MAX_ARMOR_BYTES`] is refused first.
pub(crate) fn unarmor(text: &str) -> Option<Vec<u8>> {
    if text.len() > MAX_ARMOR_BYTES {
        return None;
    }
    let lines: Vec<&str> = py_splitlines(py_strip(text))
        .into_iter()
        .map(py_strip)
        .collect();
    if lines.first() != Some(&BEGIN) || lines.last() != Some(&END) || lines.len() < 2 {
        return None;
    }
    base64_decode(&lines[1..lines.len() - 1].concat())
}

/// The bytes an SSHSIG signer signs for `message` (PROTOCOL.sshsig §3).
pub(crate) fn signed_data(message: &[u8]) -> Vec<u8> {
    let mut out = MAGIC.to_vec();
    put_string(&mut out, NAMESPACE);
    put_string(&mut out, b"");
    put_string(&mut out, HASH_ALGORITHM);
    put_string(&mut out, &Sha512::digest(message));
    out
}

/// Reads a security key's application from the key and its flags and
/// counter from the signature, requires both readers to be exhausted and
/// the user-presence flag set, and returns the bytes the authenticator
/// signed (PROTOCOL.u2f).
fn sk_signed(key: &mut Reader, sig: &mut Reader, data: &[u8]) -> Option<Vec<u8>> {
    let application = key.string()?;
    std::str::from_utf8(application).ok()?;
    let flags = sig.take(1)?[0];
    let counter = sig.take(4)?;
    if !key.done() || !sig.done() || flags & SK_USER_PRESENT == 0 {
        return None;
    }
    let mut out = Sha256::digest(application).to_vec();
    out.push(flags);
    out.extend_from_slice(counter);
    out.extend_from_slice(&Sha256::digest(data));
    Some(out)
}

/// Strict Ed25519 verification, with the reference's checks. Encodings whose
/// y coordinate is not reduced mod p are refused first; `verify_strict` then
/// refuses small-order keys and R values and an S not below L.
fn ed25519_verify(public: &[u8], message: &[u8], signature: &[u8]) -> bool {
    let (Ok(public), Ok(signature)) = (
        <[u8; 32]>::try_from(public),
        <[u8; 64]>::try_from(signature),
    ) else {
        return false;
    };
    let r: [u8; 32] = signature[..32].try_into().expect("64-byte signature");
    if !y_is_reduced(&public) || !y_is_reduced(&r) {
        return false;
    }
    let Ok(key) = ed25519_dalek::VerifyingKey::from_bytes(&public) else {
        return false;
    };
    key.verify_strict(message, &ed25519_dalek::Signature::from_bytes(&signature))
        .is_ok()
}

/// Whether a compressed Edwards point's y coordinate (the low 255 bits,
/// little-endian) is less than p.
fn y_is_reduced(point: &[u8; 32]) -> bool {
    let mut y = *point;
    y[31] &= 0x7f;
    y.iter().rev().lt(ED25519_P.iter().rev())
}

/// ECDSA P-256 over a SHA-256 digest. The key must be an uncompressed SEC1
/// point; `r` and `s` are SSH mpints read as unsigned, as the reference
/// reads them, and must lie in [1, n). High-S signatures are accepted.
fn p256_verify(public: &[u8], digest: &[u8], r: &[u8], s: &[u8]) -> bool {
    if public.len() != 65 || public[0] != 0x04 {
        return false;
    }
    let (Some(r), Some(s)) = (scalar_bytes(r), scalar_bytes(s)) else {
        return false;
    };
    let Ok(key) = p256::ecdsa::VerifyingKey::from_sec1_bytes(public) else {
        return false;
    };
    let Ok(signature) = p256::ecdsa::Signature::from_scalars(r, s) else {
        return false;
    };
    key.verify_prehash(digest, &signature).is_ok()
}

/// An unsigned big-endian integer as 32 bytes, or `None` when it needs more.
fn scalar_bytes(mpint: &[u8]) -> Option<[u8; 32]> {
    let start = mpint.iter().position(|&b| b != 0).unwrap_or(mpint.len());
    let digits = &mpint[start..];
    let mut out = [0u8; 32];
    out.get_mut(32usize.checked_sub(digits.len())?..)?
        .copy_from_slice(digits);
    Some(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn y_reduction_boundary() {
        let mut below = ED25519_P;
        below[0] -= 1;
        assert!(y_is_reduced(&below));
        assert!(!y_is_reduced(&ED25519_P));
        let mut sign_bit = below;
        sign_bit[31] |= 0x80;
        assert!(y_is_reduced(&sign_bit));
        assert!(!y_is_reduced(&[0xff; 32]));
    }

    #[test]
    fn scalar_bytes_reads_unsigned_mpints() {
        assert_eq!(scalar_bytes(&[]), Some([0; 32]));
        let mut one = [0u8; 32];
        one[31] = 1;
        assert_eq!(scalar_bytes(&[0, 0, 1]), Some(one));
        assert_eq!(scalar_bytes(&[0x80]).unwrap()[31], 0x80);
        assert_eq!(scalar_bytes(&[1; 33]), None);
        let mut leading_zero = vec![0u8];
        leading_zero.extend_from_slice(&[0xff; 32]);
        assert_eq!(scalar_bytes(&leading_zero), Some([0xff; 32]));
    }
}
