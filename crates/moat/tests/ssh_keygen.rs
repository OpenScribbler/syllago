//! Interoperability with OpenSSH's ssh-keygen. Every test passes without
//! checking anything when ssh-keygen is not on PATH, except under CI (`CI`
//! set), where a missing ssh-keygen fails the test.

mod common;

use std::io::Write;
use std::path::Path;
use std::process::{Command, Output, Stdio};

use common::TempDir;
use moat::Signer as _;
use serde_json::Value;

/// Runs ssh-keygen, or returns `None` when it is not installed. Under CI it
/// panics instead, so a runner without ssh-keygen cannot pass these tests
/// unchecked.
fn ssh_keygen(args: &[&str], stdin: &[u8]) -> Option<Output> {
    let child = Command::new("ssh-keygen")
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn();
    let mut child = match child {
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
            if std::env::var_os("CI").is_some() {
                panic!("ssh-keygen not found; CI must run the interop tests");
            }
            eprintln!("ssh-keygen not found; skipping");
            return None;
        }
        result => result.unwrap(),
    };
    child.stdin.take().unwrap().write_all(stdin).unwrap();
    Some(child.wait_with_output().unwrap())
}

fn path(p: &Path) -> &str {
    p.to_str().unwrap()
}

/// `ssh-keygen -l -E sha256` on a public key file: the fingerprint field.
fn keygen_fingerprint(pub_file: &Path) -> Option<String> {
    let out = ssh_keygen(&["-l", "-E", "sha256", "-f", path(pub_file)], b"")?;
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8(out.stdout).unwrap();
    Some(stdout.split_whitespace().nth(1).unwrap().to_string())
}

/// Signs `message` with the private key at `key` via `ssh-keygen -Y sign`.
fn keygen_sign(dir: &Path, key: &str, message: &[u8], extra: &[&str]) -> String {
    let msg = dir.join("msg");
    let sig = dir.join("msg.sig");
    std::fs::write(&msg, message).unwrap();
    let _ = std::fs::remove_file(&sig);
    let key = dir.join(key);
    let mut args = vec!["-Y", "sign", "-q", "-f", path(&key)];
    args.extend_from_slice(extra);
    args.push(path(&msg));
    let out = ssh_keygen(&args, b"").unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    std::fs::read_to_string(sig).unwrap()
}

/// Whether `ssh-keygen -Y verify -n file` accepts `signature` over `message`
/// for `principal` under `allowed_signers`, or `None` when ssh-keygen is not
/// installed.
fn keygen_verify(
    dir: &Path,
    allowed_signers: &[u8],
    principal: &str,
    signature: &str,
    message: &[u8],
) -> Option<bool> {
    let signers = dir.join("allowed_signers");
    let sig = dir.join("verify.sig");
    std::fs::write(&signers, allowed_signers).unwrap();
    std::fs::write(&sig, signature).unwrap();
    let args = [
        "-Y",
        "verify",
        "-f",
        path(&signers),
        "-I",
        principal,
        "-n",
        "file",
        "-s",
        path(&sig),
    ];
    Some(ssh_keygen(&args, message)?.status.success())
}

fn vector<'a>(doc: &'a Value, id: &str) -> &'a Value {
    doc["author_key"]["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap()
}

#[test]
fn vector_key_fingerprints_match_ssh_keygen() {
    let doc = common::vectors();
    let dir = TempDir::new("fingerprints");
    for (label, key) in doc["author_key"]["keys"].as_object().unwrap() {
        let line = key["line"].as_str().unwrap();
        let (_, public) = line.split_once(' ').unwrap();
        let pub_file = dir.path().join(format!("{label}.pub"));
        std::fs::write(&pub_file, format!("{public}\n")).unwrap();
        let Some(fingerprint) = keygen_fingerprint(&pub_file) else {
            return;
        };
        assert_eq!(fingerprint, key["fingerprint"], "{label}");
    }
}

#[test]
fn ssh_keygen_signature_verifies() {
    let dir = TempDir::new("sign");
    let key = dir.path().join("real");
    let Some(out) = ssh_keygen(
        &["-q", "-t", "ed25519", "-N", "", "-C", "", "-f", path(&key)],
        b"",
    ) else {
        return;
    };
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let pub_file = dir.path().join("real.pub");
    let fingerprint = keygen_fingerprint(&pub_file).unwrap();
    let public = std::fs::read_to_string(&pub_file).unwrap();
    let mut fields = public.split_whitespace();
    let line = format!(
        "{fingerprint} {} {}",
        fields.next().unwrap(),
        fields.next().unwrap()
    );
    let list = format!("# moat-sequence: 0\n{line}\n");

    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    let message = moat::item_message(&hash);
    let signature = keygen_sign(dir.path(), "real", &message, &["-n", "file"]);
    let signer = moat::verify_item(&hash, &signature, list.as_bytes()).unwrap();
    assert_eq!(signer.fingerprint.as_str(), fingerprint);
    assert_eq!(signer.key_type, moat::KeyType::Ed25519);

    // ssh-keygen reads a list that carries the sequence line.
    let accepted = keygen_verify(
        dir.path(),
        list.as_bytes(),
        &fingerprint,
        &signature,
        &message,
    );
    assert_eq!(accepted, Some(true));

    for extra in [&["-n", "git"][..], &["-n", "file", "-O", "hashalg=sha256"]] {
        let other = keygen_sign(dir.path(), "real", &message, extra);
        assert_eq!(
            moat::verify_item(&hash, &other, list.as_bytes()).unwrap_err(),
            moat::Refusal::SignatureInvalid,
            "{extra:?}"
        );
    }
}

/// Divergence: `ssh-keygen -Y verify` does not require the user-presence
/// flag, so it accepts TV-AK-08, which the profile refuses. A client cannot
/// delegate the flag check to ssh-keygen. TV-AK-06, the same key with the
/// flag set, is the control.
#[test]
fn ssh_keygen_accepts_tv_ak_08_without_user_presence() {
    let doc = common::vectors();
    let dir = TempDir::new("tv-ak-08");
    let key = &doc["author_key"]["keys"]["D"];
    let line = format!("{}\n", key["line"].as_str().unwrap());
    let principal = key["fingerprint"].as_str().unwrap();
    let hash = common::content_hash(&doc["acif"][3]);
    for (id, ours) in [("TV-AK-06", "ok"), ("TV-AK-08", "signature_invalid")] {
        let v = vector(&doc, id);
        let signature = v["signature"].as_str().unwrap();
        let message = v["message"].as_str().unwrap().as_bytes();
        let Some(accepted) =
            keygen_verify(dir.path(), line.as_bytes(), principal, signature, message)
        else {
            return;
        };
        assert!(accepted, "{id}: ssh-keygen -Y verify refused it");
        let got = moat::verify_item(&hash, signature, common::list(&doc, "LD"));
        assert_eq!(got.map(|_| "ok").unwrap_or_else(|r| r.id()), ours, "{id}");
    }
}

/// Generates an ssh-ed25519 key named `name` in `dir` with `comment`, or
/// returns `None` when ssh-keygen is not installed.
fn generate_key(dir: &Path, name: &str, comment: &str) -> Option<moat::SshPublicKey> {
    let key = dir.join(name);
    let args = [
        "-q",
        "-t",
        "ed25519",
        "-N",
        "",
        "-C",
        comment,
        "-f",
        path(&key),
    ];
    let out = ssh_keygen(&args, b"")?;
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let public = std::fs::read_to_string(dir.join(format!("{name}.pub"))).unwrap();
    Some(moat::SshPublicKey::parse(&public).unwrap())
}

/// The ssh-keygen signer with a fresh plain key, which only the explicit
/// allow option admits. Its signature verifies, and its Rekor entry's
/// public key drops the `.pub` file's comment.
#[test]
fn ssh_keygen_signer_signs_an_item() {
    let dir = TempDir::new("signer");
    let Some(public_key) = generate_key(dir.path(), "author", "alice@example.com") else {
        return;
    };
    let pub_text = std::fs::read_to_string(dir.path().join("author.pub")).unwrap();
    assert!(pub_text.contains("alice@example.com"));
    let fingerprint = public_key.fingerprint();
    let signer = moat::SshKeygenSigner::new(
        dir.path().join("author"),
        public_key,
        moat::NonSkKeys::Allow,
    )
    .unwrap();

    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    let signed = moat::sign_item(&signer, &hash).unwrap();
    let mut fields = pub_text.split_whitespace();
    let (key_type, blob) = (fields.next().unwrap(), fields.next().unwrap());
    let list = format!("# moat-sequence: 0\n{fingerprint} {key_type} {blob}\n");
    let signer_verified = moat::verify_item(&hash, &signed.signature, list.as_bytes()).unwrap();
    assert_eq!(signer_verified.fingerprint, fingerprint);

    let entry: Value = serde_json::from_slice(&signed.rekord_entry).unwrap();
    assert_eq!(
        entry["spec"]["signature"]["publicKey"]["content"],
        common::base64(format!("{key_type} {blob}\n").as_bytes())
    );
    assert_eq!(
        entry["spec"]["signature"]["content"],
        common::base64(signed.signature.as_bytes())
    );
}

/// A key file whose key is not the configured public key: ssh-keygen
/// signs, and the signing check refuses the result.
#[test]
fn ssh_keygen_signer_refuses_a_signature_by_another_key() {
    let dir = TempDir::new("signer-mismatch");
    let (Some(configured), Some(_)) = (
        generate_key(dir.path(), "configured", ""),
        generate_key(dir.path(), "other", ""),
    ) else {
        return;
    };
    let signer =
        moat::SshKeygenSigner::new(dir.path().join("other"), configured, moat::NonSkKeys::Allow)
            .unwrap();
    assert_eq!(
        moat::sign_message(&signer, b"message").unwrap_err(),
        moat::SignError::SignatureUnverified
    );

    let missing = moat::SshKeygenSigner::new(
        dir.path().join("missing"),
        signer.public_key().clone(),
        moat::NonSkKeys::Allow,
    )
    .unwrap();
    let err = moat::sign_message(&missing, b"message").unwrap_err();
    assert_eq!(err.id(), "syllago.moat.ssh_keygen_failed");
}

const CHILD_DIR: &str = "MOAT_TEST_SIGN_CHILD_DIR";
const PASSPHRASE: &str = "moat-test-passphrase";

/// With no terminal, ssh-keygen reads a key's passphrase from stdin, so a
/// signer that sent the message through stdin would lose its first line to
/// the passphrase prompt. This runs [`sign_with_passphrase_on_stdin`] in a
/// child of this test binary, under `setsid` so it has no controlling
/// terminal, with only the passphrase on its stdin. Linux only, where
/// util-linux provides `setsid`.
#[cfg(target_os = "linux")]
#[test]
fn ssh_keygen_signer_leaves_stdin_to_prompts() {
    let dir = TempDir::new("signer-stdin");
    let key = dir.path().join("author");
    let args = [
        "-q",
        "-t",
        "ed25519",
        "-N",
        PASSPHRASE,
        "-C",
        "",
        "-f",
        path(&key),
    ];
    let Some(out) = ssh_keygen(&args, b"") else {
        return;
    };
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let child = Command::new("setsid")
        .arg("-w")
        .arg(std::env::current_exe().unwrap())
        .args(["--exact", "sign_with_passphrase_on_stdin", "--nocapture"])
        .env(CHILD_DIR, dir.path())
        .env_remove("DISPLAY")
        .env_remove("SSH_ASKPASS")
        .env_remove("SSH_ASKPASS_REQUIRE")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn();
    let mut child = match child {
        Err(e) if e.kind() == std::io::ErrorKind::NotFound && std::env::var_os("CI").is_none() => {
            eprintln!("setsid not found; skipping");
            return;
        }
        result => result.unwrap(),
    };
    let mut stdin = child.stdin.take().unwrap();
    stdin
        .write_all(format!("{PASSPHRASE}\n").as_bytes())
        .unwrap();
    drop(stdin);
    let out = child.wait_with_output().unwrap();
    let stdout = String::from_utf8_lossy(&out.stdout);
    assert!(
        out.status.success() && stdout.contains("1 passed"),
        "{stdout}\n{}",
        String::from_utf8_lossy(&out.stderr)
    );
}

/// The child half of [`ssh_keygen_signer_leaves_stdin_to_prompts`]; passes
/// without checking anything when run on its own.
#[test]
fn sign_with_passphrase_on_stdin() {
    let Some(dir) = std::env::var_os(CHILD_DIR) else {
        return;
    };
    let dir = Path::new(&dir);
    let public = std::fs::read_to_string(dir.join("author.pub")).unwrap();
    let public_key = moat::SshPublicKey::parse(&public).unwrap();
    let fingerprint = public_key.fingerprint();
    let signer =
        moat::SshKeygenSigner::new(dir.join("author"), public_key, moat::NonSkKeys::Allow).unwrap();
    let doc = common::vectors();
    let hash = common::content_hash(&doc["acif"][3]);
    let signed = moat::sign_item(&signer, &hash).unwrap();
    let mut fields = public.split_whitespace();
    let (key_type, blob) = (fields.next().unwrap(), fields.next().unwrap());
    let list = format!("# moat-sequence: 0\n{fingerprint} {key_type} {blob}\n");
    moat::verify_item(&hash, &signed.signature, list.as_bytes()).unwrap();
}
