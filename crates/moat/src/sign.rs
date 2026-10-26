//! Author-Key signing (moat-spec.md §Author-Key Profile): sign an item
//! message with `ssh-keygen -Y sign`, check the signature with this crate's
//! own verifier, and build the `rekord` entry the Publisher proposes to
//! Rekor.
//!
//! Signing shells out to OpenSSH rather than signing in process, because a
//! security-key (`sk-`) key signs through libfido2 or an `SSH_SK_PROVIDER`
//! and a key file may name a public key whose private half is in ssh-agent.

use std::ffi::OsString;
use std::fmt;
use std::fs::DirBuilder;
use std::io;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use crate::wire::{Reader, base64, base64_decode};
use crate::{ContentHash, Fingerprint, KeyType, item_message, sshsig};

/// An SSH public key of a type the profile accepts, read from its OpenSSH
/// public-key form. Only the key blob is kept, so a comment on the line it
/// was read from never reaches a signature check or a Rekor entry.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SshPublicKey {
    key_type: KeyType,
    blob: Vec<u8>,
}

impl SshPublicKey {
    /// Reads one OpenSSH public-key line, `<type> <base64 blob> [comment]`,
    /// as a `.pub` file holds it. The type field must equal the blob's own
    /// type, and both must be a type the profile accepts.
    pub fn parse(line: &str) -> Result<SshPublicKey, SignError> {
        let invalid = |detail: &str| SignError::PublicKeyInvalid(detail.to_string());
        let mut fields = line.split_whitespace();
        let (Some(type_field), Some(blob_field)) = (fields.next(), fields.next()) else {
            return Err(invalid("expected `<type> <base64 key>`"));
        };
        let blob = base64_decode(blob_field).ok_or_else(|| invalid("the key is not base64"))?;
        let key = SshPublicKey::from_blob(blob)?;
        if key.key_type.as_str() != type_field {
            return Err(invalid("the type field does not match the key's own type"));
        }
        Ok(key)
    }

    /// A key from its SSH public-key blob.
    pub(crate) fn from_blob(blob: Vec<u8>) -> Result<SshPublicKey, SignError> {
        let name = Reader::new(&blob)
            .string()
            .ok_or_else(|| SignError::PublicKeyInvalid("the key blob is truncated".into()))?;
        let key_type = KeyType::from_wire(name).ok_or_else(|| {
            SignError::PublicKeyInvalid(format!(
                "key type {:?} is not one the profile accepts",
                String::from_utf8_lossy(name)
            ))
        })?;
        Ok(SshPublicKey { key_type, blob })
    }

    #[cfg(feature = "testing")]
    pub(crate) fn blob(&self) -> &[u8] {
        &self.blob
    }

    pub fn key_type(&self) -> KeyType {
        self.key_type
    }

    pub fn fingerprint(&self) -> Fingerprint {
        Fingerprint::of_key_blob(&self.blob)
    }

    /// The key in OpenSSH public-key form with no comment field and a
    /// trailing LF: what OpenSSH's `MarshalAuthorizedKey` writes, and the
    /// form Rekor stores an SSH public key in.
    fn authorized_key(&self) -> String {
        format!("{} {}\n", self.key_type.as_str(), base64(&self.blob))
    }
}

/// Produces armored SSHSIG signatures for one public key.
pub trait Signer {
    /// The key this signer signs with.
    fn public_key(&self) -> &SshPublicKey;

    /// Signs `message` under namespace `file` and returns the armored
    /// SSHSIG as `ssh-keygen -Y sign` writes it. The result is not checked;
    /// [`sign_message`] and [`sign_item`] check it.
    fn sign(&self, message: &[u8]) -> Result<String, SignError>;
}

/// Whether [`SshKeygenSigner`] signs with an `ssh-ed25519` key. The profile
/// accepts one, but Publishers should use a security-key (`sk-`) key, so a
/// plain key needs the caller's explicit choice.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NonSkKeys {
    Refuse,
    Allow,
}

/// Signs by writing the message to a file in a fresh private temporary
/// directory and running `ssh-keygen -Y sign -n file -f <key file>
/// <message file>`, which writes the signature beside it. The process gets
/// no shell and inherits this process's environment untouched, so
/// `SSH_SK_PROVIDER` and `SSH_AUTH_SOCK` reach it. It inherits stdin,
/// stdout and stderr too, so its prompts for a passphrase, a PIN or a touch
/// of the security key reach the user. The message never goes through
/// stdin: with no terminal, ssh-keygen reads a passphrase or PIN from stdin
/// and would consume the message's first line as one.
#[derive(Debug, Clone)]
pub struct SshKeygenSigner {
    key_file: PathBuf,
    public_key: SshPublicKey,
}

impl SshKeygenSigner {
    /// A signer for `key_file`, the argument to `ssh-keygen -f`: a private
    /// key file, or a public key file whose private half is in ssh-agent.
    /// `public_key` is the key it must sign with; every signature is checked
    /// against it. Any key but a security-key (`sk-`) key is refused unless
    /// `non_sk` is [`NonSkKeys::Allow`].
    pub fn new(
        key_file: impl Into<PathBuf>,
        public_key: SshPublicKey,
        non_sk: NonSkKeys,
    ) -> Result<SshKeygenSigner, SignError> {
        if !public_key.key_type.is_security_key() && non_sk == NonSkKeys::Refuse {
            return Err(SignError::KeyNotSecurityKey(public_key.key_type));
        }
        Ok(SshKeygenSigner {
            key_file: key_file.into(),
            public_key,
        })
    }
}

impl Signer for SshKeygenSigner {
    fn public_key(&self) -> &SshPublicKey {
        &self.public_key
    }

    fn sign(&self, message: &[u8]) -> Result<String, SignError> {
        let failed = |detail: String| SignError::SshKeygenFailed(detail);
        let dir = PrivateDir::create()
            .map_err(|e| failed(format!("could not create a temporary directory: {e}")))?;
        let message_file = dir.0.join("message");
        std::fs::write(&message_file, message)
            .map_err(|e| failed(format!("could not write the message file: {e}")))?;
        let status = Command::new("ssh-keygen")
            .args(sign_args(&self.key_file, &message_file))
            .status()
            .map_err(|e| failed(format!("could not run ssh-keygen: {e}")))?;
        if !status.success() {
            return Err(failed(format!("ssh-keygen exited with {status}")));
        }
        let mut signature_file = message_file.into_os_string();
        signature_file.push(".sig");
        std::fs::read_to_string(signature_file)
            .map_err(|e| failed(format!("could not read ssh-keygen's signature: {e}")))
    }
}

/// ssh-keygen's arguments for signing `message_file` with `key_file`.
fn sign_args(key_file: &Path, message_file: &Path) -> Vec<OsString> {
    let mut args: Vec<OsString> = ["-Y", "sign", "-q", "-n", "file", "-f"]
        .into_iter()
        .map(OsString::from)
        .collect();
    args.push(key_file.into());
    args.push(message_file.into());
    args
}

/// A new directory under the system temporary directory, readable only by
/// this user, removed with everything in it on drop. On Unix it is created
/// with mode 0700; on Windows the temporary directory is already the
/// user's own. Creation fails rather than reuse a directory that exists, so
/// a directory another user planted is never adopted.
struct PrivateDir(PathBuf);

impl PrivateDir {
    fn create() -> io::Result<PrivateDir> {
        static NEXT: AtomicU32 = AtomicU32::new(0);
        let mut builder = DirBuilder::new();
        #[cfg(unix)]
        std::os::unix::fs::DirBuilderExt::mode(&mut builder, 0o700);
        let mut attempts = 0;
        loop {
            let nanos = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_or(0, |d| d.subsec_nanos());
            let path = std::env::temp_dir().join(format!(
                "moat-sign-{}-{}-{nanos}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
            match builder.create(&path) {
                Ok(()) => return Ok(PrivateDir(path)),
                Err(e) if e.kind() == io::ErrorKind::AlreadyExists && attempts < 16 => {
                    attempts += 1;
                }
                Err(e) => return Err(e),
            }
        }
    }
}

impl Drop for PrivateDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// Signs `message` and checks the signature with this crate's verifier:
/// namespace `file`, hash `sha512`, the user-presence flag on a
/// security-key signature, and a signing key equal to the signer's
/// configured key.
pub fn sign_message(signer: &dyn Signer, message: &[u8]) -> Result<String, SignError> {
    let signature = signer.sign(message)?;
    let key = signer.public_key();
    // Fingerprints are SHA-256 of the key blobs, so equal fingerprints mean
    // equal blobs.
    match sshsig::verify(&signature, message) {
        Some(verified)
            if verified.fingerprint == key.fingerprint() && verified.key_type == key.key_type =>
        {
            Ok(signature)
        }
        _ => Err(SignError::SignatureUnverified),
    }
}

/// An item signature and the Rekor entry that records it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SignedItem {
    /// The armored SSHSIG over the item message.
    pub signature: String,
    /// The `rekord` entry to propose to Rekor, as [`rekord_entry`] builds
    /// it.
    pub rekord_entry: Vec<u8>,
}

/// Signs the item message for `content_hash` with [`sign_message`]'s checks
/// and builds its `rekord` entry.
pub fn sign_item(signer: &dyn Signer, content_hash: &ContentHash) -> Result<SignedItem, SignError> {
    let message = item_message(content_hash);
    let signature = sign_message(signer, &message)?;
    let rekord_entry = rekord_entry(&message, &signature, signer.public_key());
    Ok(SignedItem {
        signature,
        rekord_entry,
    })
}

/// The Rekor `rekord` (v0.0.1) proposed entry that records `signature` over
/// `message` (moat-spec.md §Author-Key Profile, Transparency log entry):
/// public-key format `ssh`, the message as the data, the armored SSHSIG as
/// the signature, and the key in comment-free OpenSSH form, built from the
/// key blob. These are the fields `rekor-cli upload --type rekord
/// --pki-format ssh` sends, with the keys in JCS order. Every value is
/// base64 or a fixed ASCII string, so formatting needs no JSON escaping.
pub fn rekord_entry(message: &[u8], signature: &str, public_key: &SshPublicKey) -> Vec<u8> {
    format!(
        r#"{{"apiVersion":"0.0.1","kind":"rekord","spec":{{"data":{{"content":"{}"}},"signature":{{"content":"{}","format":"ssh","publicKey":{{"content":"{}"}}}}}}}}"#,
        base64(message),
        base64(signature.as_bytes()),
        base64(public_key.authorized_key().as_bytes()),
    )
    .into_bytes()
}

/// A security-key (`sk-`) type `ssh-keygen -t` can create.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SkKeyType {
    /// `ed25519-sk`, which creates `sk-ssh-ed25519@openssh.com` keys.
    Ed25519Sk,
    /// `ecdsa-sk`, which creates `sk-ecdsa-sha2-nistp256@openssh.com` keys.
    EcdsaSk,
}

impl SkKeyType {
    /// The name `ssh-keygen -t` takes.
    pub fn as_str(self) -> &'static str {
        match self {
            SkKeyType::Ed25519Sk => "ed25519-sk",
            SkKeyType::EcdsaSk => "ecdsa-sk",
        }
    }

    /// The key type of the keys it creates.
    pub fn key_type(self) -> KeyType {
        match self {
            SkKeyType::Ed25519Sk => KeyType::SkEd25519,
            SkKeyType::EcdsaSk => KeyType::SkEcdsaP256,
        }
    }
}

/// An operating system with a known default security-key type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Os {
    Linux,
    MacOs,
    Windows,
}

/// The security-key type to create by default on `os`. Linux distributions
/// build OpenSSH with libfido2, which creates `ed25519-sk` keys. macOS's
/// bundled OpenSSH has no libfido2, and the `SSH_SK_PROVIDER` macOS is reported to ship handles
/// only ECDSA. Windows OpenSSH signs through WebAuthn, where an open
/// Win32-OpenSSH issue reports `ed25519-sk` failing with Windows Hello.
pub fn default_sk_key_type(os: Os) -> SkKeyType {
    match os {
        Os::Linux => SkKeyType::Ed25519Sk,
        Os::MacOs | Os::Windows => SkKeyType::EcdsaSk,
    }
}

/// `ssh-keygen`'s arguments for creating a `key_type` key at `key_file`.
/// The comment is empty, so the public key file carries no personal
/// identifier. ssh-keygen asks for a passphrase itself.
pub fn keygen_args(key_type: SkKeyType, key_file: &Path) -> Vec<OsString> {
    let mut args: Vec<OsString> = ["-t", key_type.as_str(), "-C", "", "-f"]
        .into_iter()
        .map(OsString::from)
        .collect();
    args.push(key_file.into());
    args
}

/// Why signing failed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SignError {
    /// The public key is malformed or of a type the profile does not
    /// accept.
    PublicKeyInvalid(String),
    /// The key is not a security-key (`sk-`) key and plain keys were not
    /// allowed.
    KeyNotSecurityKey(KeyType),
    /// ssh-keygen could not be run, failed, or wrote no signature file.
    SshKeygenFailed(String),
    /// The signature does not verify under the profile with the signer's
    /// configured key.
    SignatureUnverified,
}

impl SignError {
    /// The stable identifier for this error. A detail never changes it.
    pub fn id(&self) -> &'static str {
        match self {
            SignError::PublicKeyInvalid(_) => "syllago.moat.signing_key_invalid",
            SignError::KeyNotSecurityKey(_) => "syllago.moat.signing_key_not_sk",
            SignError::SshKeygenFailed(_) => "syllago.moat.ssh_keygen_failed",
            SignError::SignatureUnverified => "syllago.moat.signature_unverified",
        }
    }
}

impl fmt::Display for SignError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            SignError::PublicKeyInvalid(detail) | SignError::SshKeygenFailed(detail) => {
                write!(f, "{id}: {detail}")
            }
            SignError::KeyNotSecurityKey(key_type) => write!(
                f,
                "{id}: {} is not a security-key (sk-) key; create one with \
                 `ssh-keygen -t ed25519-sk` or `ssh-keygen -t ecdsa-sk`, or allow a plain key \
                 explicitly",
                key_type.as_str()
            ),
            SignError::SignatureUnverified => write!(
                f,
                "{id}: the signature does not verify under the profile with the configured key"
            ),
        }
    }
}

impl std::error::Error for SignError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sign_args_are_fixed() {
        assert_eq!(
            sign_args(Path::new("dir/id_ed25519_sk.pub"), Path::new("tmp/message")),
            [
                "-Y",
                "sign",
                "-q",
                "-n",
                "file",
                "-f",
                "dir/id_ed25519_sk.pub",
                "tmp/message"
            ]
        );
    }

    #[test]
    fn authorized_key_drops_the_comment() {
        let line =
            "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAII5C8Sf8CM8ca7csOPFWeq4DJeLtXJEUFJ1C6C7CZ4Ig";
        let key = SshPublicKey::parse(&format!("{line} alice@example.com\n")).unwrap();
        assert_eq!(key.authorized_key(), format!("{line}\n"));
    }
}
