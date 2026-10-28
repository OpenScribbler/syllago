#!/usr/bin/env python3
"""Writes the syllago-loadout-v1 test vectors as JSON on stdout.

Usage: MOAT_DIR=<OpenScribbler/moat checkout> python3 scripts/loadout-vectors.py

Every signature comes from MOAT's reference implementation,
reference/moat_author_key.py, and every digest and message is built here
from the format in crates/library/tests/vectors/README.md, so the vectors
never come from the Rust code they check. The expected outcomes are written
by hand. The keys are ssh-ed25519 keys from fixed seeds, so the output is
byte-identical on every run and OS.

CI regenerates the file and fails when it differs from the committed
crates/library/tests/vectors/loadout-vectors.json.
"""

import hashlib
import json
import os
import pathlib
import sys

moat_dir = os.environ.get("MOAT_DIR")
if not moat_dir:
    sys.exit("set MOAT_DIR to an OpenScribbler/moat checkout")
moat = pathlib.Path(moat_dir)
sys.path.insert(0, str(moat / "reference"))

import moat_author_key as ak  # noqa: E402

SOURCE_URI = "https://git.example.com/alice/library.git"
OTHER_SOURCE_URI = "https://git.example.com/mallory/library.git"
MALFORMED = "syllago.loadout.malformed"


def seed(label):
    return hashlib.sha256(f"syllago-loadout-test-key-{label}".encode()).digest()


def key_blob(label):
    return ak.key_blob_ed25519(ak.ed25519_public(seed(label)))


KEYS = {
    label: {
        "type": "ssh-ed25519",
        "seed": f"sha256(\"syllago-loadout-test-key-{label}\")",
        "fingerprint": ak.fingerprint(key_blob(label)),
        "line": ak.allowed_signers_line(key_blob(label)),
    }
    for label in ("A", "B")
}
LISTS = {
    "LA": f"# moat-sequence: 0\n{KEYS['A']['line']}\n",
    "LB": f"# moat-sequence: 1\n{KEYS['B']['line']}\n",
}


def canonical(data):
    if data.startswith(b"\xef\xbb\xbf"):
        data = data[3:]
    return data.replace(b"\r\n", b"\n").replace(b"\r", b"\n")


def digest(data):
    return "sha256:" + hashlib.sha256(canonical(data)).hexdigest()


def message(source_uri, name, sequence, manifest_bytes):
    return (f"syllago-loadout-v1\n{source_uri}\n{name}\n{sequence}\n"
            f"{digest(manifest_bytes)}\n").encode()


def sign(label, data):
    return ak.sign_ed25519(seed(label), data)


def pin(label):
    return "sha256:" + hashlib.sha256(f"item-{label}".encode()).hexdigest()


ITEM_1 = "0f8fad5b-d9cb-469f-a165-70867728950e"
ITEM_2 = "7c9e6679-7425-40de-944b-e07fc1f90ae7"


def manifest(sequence='"3"', providers='["claude-code", "codex"]', items=None, extra=""):
    if items is None:
        items = (f'    {{"id": "{ITEM_1}", "attestation_hash": "{pin(1)}"}},\n'
                 f'    {{"id": "{ITEM_2}", "attestation_hash": "{pin(2)}"}}\n')
    return ("{\n" + extra + f'  "sequence": {sequence},\n  "providers": {providers},\n'
            f'  "items": [\n{items}  ]\n}}\n')


WORK_3 = manifest()
WORK_2 = manifest(sequence='"2"', providers='["claude-code"]')
WORK_4 = manifest(sequence='"4"')
WORK_3_EDITED = manifest(providers='["claude-code"]')


def digest_vectors():
    lf = WORK_3.encode()
    crlf = b"\xef\xbb\xbf" + lf.replace(b"\n", b"\r\n")
    cr = lf.replace(b"\n", b"\r")
    return [
        {"id": "LD-01", "title": "LF manifest", "file_name": "work.json", "bytes": lf},
        {"id": "LD-02", "title": "Byte-order mark and CRLF give the LF digest",
         "file_name": "work.json", "bytes": crlf},
        {"id": "LD-03", "title": "Lone CR gives the LF digest", "file_name": "work.json",
         "bytes": cr},
        {"id": "LD-04", "title": "A byte-order mark after the start is kept",
         "file_name": "work.json", "bytes": b" \xef\xbb\xbf" + lf, "expected": MALFORMED},
    ]


def manifest_vectors():
    item = f'    {{"id": "{ITEM_1}", "attestation_hash": "{pin(1)}"}}\n'
    cases = [
        ("LM-01", "Valid, no name field", "work.json", WORK_3, "ok"),
        ("LM-02", "Valid, name field equal to the file name", "work.json",
         manifest(extra='  "name": "work",\n'), "ok"),
        ("LM-03", "Valid, smallest sequence and no items", "a-1.json",
         manifest(sequence='"0"', items=""), "ok"),
        ("LM-04", "Valid, largest sequence", "work.json",
         manifest(sequence='"9007199254740991"'), "ok"),
        ("LM-05", "Name with an uppercase letter", "Work.json", WORK_3, MALFORMED),
        ("LM-06", "Name with an underscore", "my_work.json", WORK_3, MALFORMED),
        ("LM-07", "Name with a dot", "work.v2.json", WORK_3, MALFORMED),
        ("LM-08", "Empty name", ".json", WORK_3, MALFORMED),
        ("LM-09", "File name without the extension", "work", WORK_3, MALFORMED),
        ("LM-10", "Name field differs from the file name", "work.json",
         manifest(extra='  "name": "home",\n'), MALFORMED),
        ("LM-11", "Name field is null", "work.json",
         manifest(extra='  "name": null,\n'), MALFORMED),
        ("LM-12", "Sequence with a leading zero", "work.json", manifest(sequence='"03"'),
         MALFORMED),
        ("LM-13", "Sequence with a plus sign", "work.json", manifest(sequence='"+3"'),
         MALFORMED),
        ("LM-14", "Negative sequence", "work.json", manifest(sequence='"-3"'), MALFORMED),
        ("LM-15", "Sequence above 2^53 - 1", "work.json",
         manifest(sequence='"9007199254740992"'), MALFORMED),
        ("LM-16", "Sequence as a JSON number", "work.json", manifest(sequence="3"),
         MALFORMED),
        ("LM-17", "Empty sequence", "work.json", manifest(sequence='""'), MALFORMED),
        ("LM-18", "No providers", "work.json", manifest(providers="[]"), MALFORMED),
        ("LM-19", "Repeated provider", "work.json",
         manifest(providers='["codex", "codex"]'), MALFORMED),
        ("LM-20", "Uppercase item id", "work.json",
         manifest(items=item.replace(ITEM_1, ITEM_1.upper())), MALFORMED),
        ("LM-21", "Repeated item id", "work.json",
         manifest(items=item.replace("\n", ",\n") + item), MALFORMED),
        ("LM-22", "attestation_hash that is not sha256", "work.json",
         manifest(items=item.replace("sha256:", "sha512:")), MALFORMED),
        ("LM-23", "Unknown field", "work.json", manifest(extra='  "version": 2,\n'),
         MALFORMED),
        ("LM-24", "Repeated key", "work.json", manifest(extra='  "sequence": "4",\n'),
         MALFORMED),
        ("LM-25", "Not JSON", "work.json", "sequence: 3\n", MALFORMED),
        ("LM-26", "Manifest written as a JSON array", "work.json",
         f'["work", "3", ["codex"], [{{"id": "{ITEM_1}", "attestation_hash": "{pin(1)}"}}]]\n',
         MALFORMED),
        ("LM-27", "Item written as a JSON array", "work.json",
         manifest(items=f'    ["{ITEM_1}", "{pin(1)}"]\n'), MALFORMED),
        ("LM-28", "Missing required field", "work.json",
         '{\n  "sequence": "3",\n  "providers": ["codex"]\n}\n', MALFORMED),
        ("LM-29", "Unknown field inside an item", "work.json",
         manifest(items=item.replace("}\n", ', "version": 2}\n')), MALFORMED),
        ("LM-30", "Provider outside the name grammar", "work.json",
         manifest(providers='["Claude Code"]'), MALFORMED),
    ]
    out = []
    for vid, title, file_name, text, expected in cases:
        v = {"id": vid, "title": title, "file_name": file_name, "bytes": text.encode(),
             "expected": expected}
        if expected == "ok":
            v["name"] = file_name.removesuffix(".json")
        out.append(v)
    return out


def name_set_vectors():
    return [
        {"id": "LN-01", "title": "Two manifests with different names",
         "files": [{"file_name": "work.json", "bytes": WORK_3.encode()},
                   {"file_name": "home.json", "bytes": WORK_3.encode()}],
         "expected": "ok"},
        {"id": "LN-02", "title": "Two manifests with the same name",
         "files": [{"file_name": "work.json", "bytes": WORK_3.encode()},
                   {"file_name": "work.json", "bytes": WORK_2.encode()}],
         "expected": MALFORMED},
    ]


def record(text, sequence, active):
    return {"sequence": sequence, "digest": digest(text.encode()), "active": active}


def signature_vectors():
    w3 = WORK_3.encode()
    w3_crlf = b"\xef\xbb\xbf" + w3.replace(b"\n", b"\r\n")
    sig_w3 = sign("A", message(SOURCE_URI, "work", 3, w3))
    item_hash = pin(1)
    la = LISTS["LA"].encode()
    lb = LISTS["LB"].encode()
    cases = [
        # (id, title, source_uri, file_name, bytes, signature, list, record, expected)
        ("LV-01", "Valid, no record: reported, not applied", SOURCE_URI, "work.json", w3,
         sig_w3, "LA", None, "report_no_record"),
        ("LV-02", "Valid, recorded inactive with the same digest", SOURCE_URI, "work.json",
         w3, sig_w3, "LA", record(WORK_3, 3, False), "report_inactive"),
        ("LV-03", "Valid, recorded active with the same digest: unchanged", SOURCE_URI,
         "work.json", w3, sig_w3, "LA", record(WORK_3, 3, True), "apply"),
        ("LV-04", "Valid, a higher sequence than the active record", SOURCE_URI,
         "work.json", w3, sig_w3, "LA", record(WORK_2, 2, True), "apply"),
        ("LV-05", "Byte-order mark and CRLF: signed over the canonical digest", SOURCE_URI,
         "work.json", w3_crlf, sig_w3, "LA", record(WORK_2, 2, True), "apply"),
        ("LV-06", "Missing .sig", SOURCE_URI, "work.json", w3, None, "LA",
         record(WORK_2, 2, True), "signature_invalid"),
        ("LV-07", "Manifest edited after signing", SOURCE_URI, "work.json",
         WORK_3_EDITED.encode(), sig_w3, "LA", record(WORK_2, 2, True),
         "signature_invalid"),
        ("LV-08", "Signed by a key the trusted list does not allow", SOURCE_URI,
         "work.json", w3, sign("B", message(SOURCE_URI, "work", 3, w3)), "LA",
         record(WORK_2, 2, True), "signer_not_allowed"),
        ("LV-09", "Signer removed from the current trusted list", SOURCE_URI, "work.json",
         w3, sig_w3, "LB", record(WORK_3, 3, True), "signer_not_allowed"),
        ("LV-10", "Changed manifest at the recorded sequence", SOURCE_URI, "work.json",
         WORK_3_EDITED.encode(),
         sign("A", message(SOURCE_URI, "work", 3, WORK_3_EDITED.encode())), "LA",
         record(WORK_3, 3, True), "sequence_not_greater"),
        ("LV-11", "Rollback to an older signed version", SOURCE_URI, "work.json",
         WORK_2.encode(), sign("A", message(SOURCE_URI, "work", 2, WORK_2.encode())), "LA",
         record(WORK_4, 4, True), "sequence_not_greater"),
        ("LV-12", "Rollback while recorded inactive", SOURCE_URI, "work.json",
         WORK_2.encode(), sign("A", message(SOURCE_URI, "work", 2, WORK_2.encode())), "LA",
         record(WORK_4, 4, False), "sequence_not_greater"),
        ("LV-17", "Changed manifest at a higher sequence while recorded inactive",
         SOURCE_URI, "work.json", w3, sig_w3, "LA", record(WORK_2, 2, False),
         "report_inactive"),
        ("LV-13", "Loadout copied to another name", SOURCE_URI, "home.json", w3, sig_w3,
         "LA", None, "signature_invalid"),
        ("LV-14", "Loadout replayed under another source_uri", OTHER_SOURCE_URI,
         "work.json", w3, sig_w3, "LA", None, "signature_invalid"),
        ("LV-15", "MOAT item signature presented as a loadout signature", SOURCE_URI,
         "work.json", w3, sign("A", ak.item_message(item_hash)), "LA", None,
         "signature_invalid"),
        ("LV-16", "MOAT list-change signature presented as a loadout signature",
         SOURCE_URI, "work.json", w3,
         sign("A", ak.list_change_message(SOURCE_URI, la, lb)), "LA", None,
         "signature_invalid"),
    ]
    out = []
    for vid, title, uri, file_name, data, signature, lst, rec, expected in cases:
        name = file_name.removesuffix(".json")
        sequence = json.loads(canonical(data))["sequence"]
        out.append({
            "id": vid, "title": title, "source_uri": uri, "file_name": file_name,
            "bytes": data, "signature": signature, "list": lst, "record": rec,
            "message": message(uri, name, sequence, data), "expected": expected,
        })
    return out


def text(value):
    """Bytes as JSON text: every vector's bytes are UTF-8."""
    if isinstance(value, bytes):
        return value.decode("utf-8")
    if isinstance(value, list):
        return [text(v) for v in value]
    if isinstance(value, dict):
        return {k: text(v) for k, v in value.items()}
    return value


def main():
    doc = {
        "about": "syllago-loadout-v1 test vectors. Generated by scripts/loadout-vectors.py "
                 "with MOAT's reference signer; do not edit. "
                 "crates/library/tests/vectors/README.md states the format.",
        "source_uri": SOURCE_URI,
        "keys": KEYS,
        "lists": LISTS,
        "digests": [
            {**v, "canonical": canonical(v["bytes"])}
            | ({} if "expected" in v else {"digest": digest(v["bytes"])})
            for v in digest_vectors()
        ],
        "manifests": manifest_vectors(),
        "name_sets": name_set_vectors(),
        "signatures": signature_vectors(),
    }
    # Binary stdout, so Windows never rewrites LF as CRLF.
    sys.stdout.buffer.write((json.dumps(text(doc), indent=2) + "\n").encode("ascii"))


main()
