#!/usr/bin/env bash
# moat-trust-surfacing.sh — Manual smoke fixture for MOAT TUI trust surfacing
# (ADR 0007 Phase 2c: syllago-hf5am + syllago-0kbbx).
#
# Human-driven. This script sets up a sandboxed HOME with synthetic MOAT
# registry content and launches the TUI so you can visually verify:
#   - verified-skill   → ✓ glyph + "Trust: DUAL-ATTESTED" metapanel line
#   - recalled-skill   → R glyph + red revocation banner + publisher-warn
#                        modal on install attempt
#   - private-skill    → P glyph + "Visibility: Private" metapanel chip
#
# The fixture is fully sandboxed: HOME is redirected to a fresh temp dir and
# nothing in your real ~/.syllago is touched.
#
# Usage:
#   ./tests/smoke/moat-trust-surfacing.sh
#
# Teardown is manual — the script prints the temp root so you can `rm -rf` it
# when done (or just let /tmp garbage-collect on reboot).

set -euo pipefail

# ── Resolve syllago binary ────────────────────────────────────────────────
# Prefer the freshly-built dev binary under cli/, falling back to $PATH. We
# explicitly do NOT install or build — the user is expected to `make build`
# before running this.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SYLLAGO_BIN="${REPO_ROOT}/cli/syllago"
if [[ ! -x "${SYLLAGO_BIN}" ]]; then
  SYLLAGO_BIN="$(command -v syllago || true)"
fi
if [[ -z "${SYLLAGO_BIN}" || ! -x "${SYLLAGO_BIN}" ]]; then
  echo "error: syllago binary not found. Run 'make build' first." >&2
  exit 1
fi

# ── Sandboxed HOME ────────────────────────────────────────────────────────

SMOKE_HOME="$(mktemp -d -t syllago-smoke-moat-XXXXXX)"
SYLLAGO_DIR="${SMOKE_HOME}/.syllago"
REG_NAME="smoke-moat"
REG_CLONE="${SYLLAGO_DIR}/registries/${REG_NAME}"
MOAT_CACHE="${SYLLAGO_DIR}/moat/registries/${REG_NAME}"
MANIFEST_URI="https://example.invalid/smoke-moat/manifest.json"

mkdir -p "${SYLLAGO_DIR}"
mkdir -p "${REG_CLONE}/skills/verified-skill"
mkdir -p "${REG_CLONE}/skills/recalled-skill"
mkdir -p "${REG_CLONE}/skills/private-skill"
mkdir -p "${MOAT_CACHE}"

# Provider stub: an empty ~/.claude satisfies claude-code provider detection
# so the install wizard has at least one target to offer. Without it, the
# wizard would block at provider-select with no options.
mkdir -p "${SMOKE_HOME}/.claude"

# ── Library content (scanned as registry items) ───────────────────────────
# Each skill gets a minimal SKILL.md with frontmatter so the catalog scanner
# picks it up as a skill-type content item.

write_skill() {
  local name="$1"; local desc="$2"
  cat >"${REG_CLONE}/skills/${name}/SKILL.md" <<EOF
---
name: ${name}
description: ${desc}
---

# ${name}

Smoke-test fixture — not real content.
EOF
}

write_skill "verified-skill" "Happy-path dual-attested fixture item"
write_skill "recalled-skill" "Publisher-revoked fixture item (triggers warn modal)"
write_skill "private-skill"  "Private-repo fixture item (lock chip)"

# ── MOAT manifest ─────────────────────────────────────────────────────────
# Three content entries + one revocation referencing recalled-skill's hash.
# Hashes are synthetic (64 hex chars) — enrichment does not compare them to
# real file bytes. If you later extend this to exercise syllago-u0jna's
# PreInstallCheck path, you'll need real content_hash values.

HASH_VERIFIED="sha256:1111111111111111111111111111111111111111111111111111111111111111"
HASH_RECALLED="sha256:2222222222222222222222222222222222222222222222222222222222222222"
HASH_PRIVATE="sha256:3333333333333333333333333333333333333333333333333333333333333333"

cat >"${MOAT_CACHE}/manifest.json" <<EOF
{
  "schema_version": 1,
  "manifest_uri": "${MANIFEST_URI}",
  "name": "Smoke MOAT Registry",
  "operator": "Smoke Test Operator",
  "updated_at": "2026-04-20T00:00:00Z",
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:example/smoke-moat:ref:refs/heads/main"
  },
  "content": [
    {
      "name": "verified-skill",
      "display_name": "Verified Skill",
      "type": "skill",
      "content_hash": "${HASH_VERIFIED}",
      "source_uri": "https://example.invalid/smoke/verified-skill",
      "attested_at": "2026-04-20T00:00:00Z",
      "private_repo": false,
      "rekor_log_index": 1000000001,
      "signing_profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject": "repo:example/verified-publisher:ref:refs/heads/main"
      }
    },
    {
      "name": "recalled-skill",
      "display_name": "Recalled Skill",
      "type": "skill",
      "content_hash": "${HASH_RECALLED}",
      "source_uri": "https://example.invalid/smoke/recalled-skill",
      "attested_at": "2026-04-20T00:00:00Z",
      "private_repo": false,
      "rekor_log_index": 1000000002
    },
    {
      "name": "private-skill",
      "display_name": "Private Skill",
      "type": "skill",
      "content_hash": "${HASH_PRIVATE}",
      "source_uri": "https://example.invalid/smoke/private-skill",
      "attested_at": "2026-04-20T00:00:00Z",
      "private_repo": true,
      "rekor_log_index": 1000000003
    }
  ],
  "revocations": [
    {
      "content_hash": "${HASH_RECALLED}",
      "reason": "compromised",
      "details_url": "https://example.invalid/smoke/recall/recalled-skill",
      "source": "publisher"
    }
  ]
}
EOF

# Signature bundle: content is not verified at enrich time (per
# producer.go's trust-boundary comment — sync is the only component
# that cryptographically verifies). A stub is sufficient; its presence is
# the gating check.
printf 'stub-bundle-bytes' >"${MOAT_CACHE}/signature.bundle"

# ── Global syllago config ─────────────────────────────────────────────────
# Declares the MOAT registry so EnrichFromMOATManifests picks it up.

cat >"${SYLLAGO_DIR}/config.json" <<EOF
{
  "providers": ["claude-code"],
  "registries": [
    {
      "name": "${REG_NAME}",
      "url": "",
      "type": "moat",
      "manifest_uri": "${MANIFEST_URI}",
      "signing_profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject": "repo:example/smoke-moat:ref:refs/heads/main"
      }
    }
  ]
}
EOF

# ── MOAT lockfile (project-scoped) ────────────────────────────────────────
# The lockfile lives at <projectRoot>/.syllago/moat-lockfile.json. We make
# SMOKE_HOME itself the project root so `cd $SMOKE_HOME && syllago` finds
# the lockfile at a stable path and CheckRegistry returns Fresh.

NOW="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
mkdir -p "${SMOKE_HOME}/.syllago"
cat >"${SMOKE_HOME}/.syllago/moat-lockfile.json" <<EOF
{
  "moat_lockfile_version": 1,
  "registries": {
    "${MANIFEST_URI}": {
      "fetched_at": "${NOW}"
    }
  },
  "entries": [],
  "revoked_hashes": []
}
EOF

# ── Print checklist and launch ────────────────────────────────────────────

cat <<EOF

╭─ MOAT smoke fixture ready ─────────────────────────────────────────────╮
│ Sandboxed HOME: ${SMOKE_HOME}
│ Binary:         ${SYLLAGO_BIN}
│ Registry:       ${REG_NAME} (MOAT-type, synthetic manifest)
╰────────────────────────────────────────────────────────────────────────╯

Manual verification checklist — run through these in the TUI:

 Library row glyphs (Collections > Library tab, default landing page):
   [ ] verified-skill row shows a ✓ glyph
   [ ] recalled-skill row shows an R glyph
   [ ] private-skill  row shows a P glyph

 Metapanel drill-down (cursor onto each row with ↑/↓):
   [ ] verified-skill metapanel: "Trust: Verified (registry-attested)"
       (or DUAL-ATTESTED tier text)
   [ ] recalled-skill metapanel: red "RECALLED (publisher) — compromised"
       banner with issuer + details URL
   [ ] private-skill  metapanel: "Visibility: Private" chip with lock glyph

 Publisher-warn install modal (cursor on recalled-skill, press [i]):
   [ ] Install wizard opens normally (scope pick, etc.)
   [ ] At confirm, a red-bordered modal appears with:
       - Title referencing "recalled-skill"
       - "The publisher has revoked this item."
       - "Reason: compromised"
       - "Issued by:" with the signing-profile subject
       - "Details:" URL
   [ ] Cancel button works (modal closes, no install)
   [ ] Install anyway button works (modal closes, install proceeds)
   [ ] Click the Cancel button with the mouse — works
   [ ] Click the Install anyway button with the mouse — works
   [ ] Esc dismisses the modal

 Responsive layout (resize the terminal while the TUI is open):
   [ ] At 60x20 or smaller: "Terminal too small — minimum 80x20"
   [ ] At 80x30: all glyphs + banners render cleanly
   [ ] At 120x40: no truncation artifacts on any row or banner

Launching TUI now.  Press q from the Library tab to quit when done.
Cleanup when finished:  rm -rf ${SMOKE_HOME}

EOF

# The sandboxed HOME override is the only state change needed. syllago's
# os.UserHomeDir() reads $HOME first. We also chdir so projectRoot resolves
# to SMOKE_HOME — the moat-lockfile.json lives there.
cd "${SMOKE_HOME}"
exec env HOME="${SMOKE_HOME}" "${SYLLAGO_BIN}"
