use std::fmt;

use sha2::{Digest, Sha256};

/// A digest as ACIF records write it: `{algorithm, value}`, with `value` in
/// lowercase hex. It displays as `algorithm:value`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Hash {
    pub algorithm: String,
    pub value: String,
}

impl Hash {
    /// The SHA-256 digest of `bytes`.
    pub fn sha256(bytes: &[u8]) -> Hash {
        let digest = Sha256::digest(bytes);
        let value = digest.iter().map(|byte| format!("{byte:02x}")).collect();
        Hash {
            algorithm: "sha256".to_string(),
            value,
        }
    }
}

impl fmt::Display for Hash {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}:{}", self.algorithm, self.value)
    }
}

/// The attestation input digest (registry §6): SHA-256 over
/// `"acif-attestation-input-v1\n" BH "\n" MH "\n"`, where BH and MH are
/// written `algorithm:value` and MH is `null` when the item has no
/// `metadata_hash`.
pub fn attestation_input_digest(body_hash: &Hash, metadata_hash: Option<&Hash>) -> Hash {
    let metadata = metadata_hash.map_or_else(|| "null".to_string(), Hash::to_string);
    Hash::sha256(format!("acif-attestation-input-v1\n{body_hash}\n{metadata}\n").as_bytes())
}
