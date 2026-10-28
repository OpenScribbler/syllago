#!/usr/bin/env bash
# Runs the ACIF conformance suite against the acif crate's adapter.
#
# Usage: ACIF_DIR=<agent-content-interchange-format checkout> scripts/acif-conformance.sh
#
# The first run claims the core, skill and rule scopes; the next two run
# TV-L2-g (frontmatter to publisher_section) and TV-L3-e (the attestation
# input digest) on their own. The script exits non-zero unless the runner
# terminated normally each time, the core, skill and rule scopes each pass,
# and TV-L2-g and TV-L3-e pass. An env-blocked vector (the runner could
# not build its fixture on this filesystem) fails the run unless the
# allowlist below names it for this OS.
set -euo pipefail

: "${ACIF_DIR:?set ACIF_DIR to an ACIF repository checkout}"
PYTHON="${PYTHON:-python3}"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

adapter="$(cd "$repo" && cargo build --locked -p acif-conformance --message-format=json |
  "$PYTHON" -c '
import json, sys
paths = [m["executable"] for m in map(json.loads, sys.stdin)
         if m.get("reason") == "compiler-artifact" and m.get("executable")]
print(paths[-1].replace("\\", "/"))
')"

reports="$(mktemp -d)"
trap 'rm -rf "$reports"' EXIT

runner() {
  (cd "$ACIF_DIR" && PYTHONDONTWRITEBYTECODE=1 "$PYTHON" -m conformance.runner \
    --adapter "$adapter" "$@")
}

scopes_exit=0
runner --report "$reports/scopes.json" --scope core --scope skill --scope rule || scopes_exit=$?
l2g_exit=0
runner --report "$reports/l2g.json" --only TV-L2-g || l2g_exit=$?
l3e_exit=0
runner --report "$reports/l3e.json" --only TV-L3-e || l3e_exit=$?

"$PYTHON" - "$reports/scopes.json" "$scopes_exit" "$reports/l2g.json" "$l2g_exit" \
  "$reports/l3e.json" "$l3e_exit" <<'EOF'
import json, sys

# Vector IDs allowed to be env-blocked, keyed by sys.platform (linux, darwin,
# win32). Name a vector here only with the reason its fixture cannot be built
# on that OS.
ENV_BLOCKED_ALLOWED = {
    "linux": [],
    "darwin": [
        # APFS cannot hold two names that differ only by Unicode
        # normalization, so the runner cannot build the fixture.
        "TV-13",
        # APFS refuses a file name that is not UTF-8, so the runner cannot
        # build TV-14's not_utf8 fixtures.
        "TV-14",
    ],
    "win32": [
        # NTFS names are UTF-16, so the runner cannot create a name that is
        # not UTF-8, and NTFS refuses control characters in a name, so the
        # runner cannot build TV-14's fixtures.
        "TV-14",
    ],
}
allowed = set(ENV_BLOCKED_ALLOWED.get(sys.platform, []))
failed = False


def fail(message):
    global failed
    print(f"FAIL: {message}")
    failed = True


def load(path, exit_code):
    """The runner's report, after checking it terminated normally: it exits
    0 on a clean run and 1 when a vector failed or harness-errored or the
    handshake failed. Any other exit, a missing report, or an exit its own
    report does not account for means it crashed."""
    try:
        with open(path, encoding="utf-8") as file:
            report = json.load(file)
    except (OSError, ValueError) as error:
        print(f"FAIL: runner exited {exit_code} without a readable report: {error}")
        sys.exit(1)
    if report["adapter"].get("hello_error"):
        fail(f"adapter hello: {report['adapter']['hello_error']}")
    reported = any(row["status"] in {"fail", "harness-error"} for row in report["vectors"])
    expected = 1 if report["adapter"].get("hello_error") or reported else 0
    if exit_code != expected:
        fail(f"runner exited {exit_code}, but its report accounts for exit {expected}")
    return report


scopes = load(sys.argv[1], int(sys.argv[2]))
singles = {"TV-L2-g": load(sys.argv[3], int(sys.argv[4])), "TV-L3-e": load(sys.argv[5], int(sys.argv[6]))}
for row in scopes["vectors"]:
    if row["status"] in {"fail", "harness-error", "unsupported"}:
        fail(f"{row['id']} [{row['status']}] {row.get('message', '')}")
    elif row["status"] == "env-blocked":
        if row["id"] in allowed:
            print(f"WARN: {row['id']} [env-blocked, allowed on {sys.platform}] {row.get('message', '')}")
        else:
            fail(f"{row['id']} [env-blocked] {row.get('message', '')}")
for scope in ("core", "skill", "rule"):
    info = scopes["scopes"]["summary"].get(scope, {"status": "missing", "blocking": []})
    print(f"scope {scope}: {info['status']}")
    tolerated = info["status"] == "not-passed" and info["blocking"] and all(
        blocker["status"] == "env-blocked" and blocker["id"] in allowed
        for blocker in info["blocking"]
    )
    if info["status"] != "pass" and not tolerated:
        blocking = ", ".join(f"{b['id']} [{b['status']}]" for b in info["blocking"])
        fail(f"scope {scope} is {info['status']}" + (f": {blocking}" if blocking else ""))
for vector_id, report in singles.items():
    row = next((row for row in report["vectors"] if row["id"] == vector_id), None)
    status = row["status"] if row else "missing"
    print(f"{vector_id}: {status}")
    if status != "pass":
        fail(f"{vector_id} is {status}" + (f": {row.get('message', '')}" if row else ""))
sys.exit(1 if failed else 0)
EOF
