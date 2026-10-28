# syllago-loadout-v1 test vectors

`loadout-vectors.json` holds the test vectors for syllago's signed loadout
manifests. `scripts/loadout-vectors.py` generates it with the signer from
MOAT's reference implementation, `reference/moat_author_key.py`; do not edit
it by hand. This file states the format a client must implement to pass
them.

## Files

A loadout is a manifest file `<name>.json` in the sync repository. Its
armored signature sits beside it at `<name>.json.sig`.

## Name

The loadout's name is the manifest's file name without `.json`. A name is
one or more of lowercase `a`-`z`, `0`-`9` and `-`. A client refuses as
malformed a manifest whose file name does not end in `.json`, whose name
breaks the grammar, whose `name` field, when present, differs from the file
name, or whose name another manifest in the repository also has.

## Canonical text and digest

The manifest's canonical text is its bytes with a leading UTF-8 byte-order
mark removed and each CRLF and lone CR replaced by LF, as MOAT's list digest
defines. The manifest digest is `sha256:` and the lowercase hex SHA-256 of
the canonical text. The client parses the canonical text, never the raw
bytes.

## Manifest

The manifest file is at most 65536 bytes; a larger one is malformed before
it is read further. Its canonical text is one JSON object with these fields
and no others, and each item is a JSON object; an array in place of either
is malformed. A repeated key is malformed.

| Field       | Required | Value                                                                |
| ----------- | -------- | -------------------------------------------------------------------- |
| `name`      | no       | A string equal to the file name's name; `null` is malformed.         |
| `sequence`  | yes      | A string: canonical decimal from 0 to 2^53 - 1.                      |
| `providers` | yes      | One or more distinct, non-empty provider strings.                    |
| `items`     | yes      | An array of `{"id", "attestation_hash"}` objects, `id`s distinct.     |

An item's `id` is its ACIF `id`: a UUID version 4, hyphenated, with
lowercase hex digits. Its `attestation_hash` is the ACIF `attestation_hash`
it is pinned to: `sha256:` and 64 lowercase hex digits.

Canonical decimal has no sign, no leading zero (except `0` itself), no
fraction and no exponent. The sequence is a JSON string rather than a JSON
number so that the field holds exactly the form the signed message holds; a
JSON number is malformed.

## Message

The Author Key signs the loadout message, each line ending in LF:

```text
syllago-loadout-v1
<source_uri>
<name>
<sequence>
<manifest digest>
```

`source_uri` is the sync repository's, and is never empty and holds no CR or
LF. `sequence` is written in canonical decimal. The `source_uri` stops a
loadout signed for one repository verifying in another, and the name stops a
signed loadout being copied over another loadout.

## Signature

The signature is MOAT's author-key scheme: an armored SSHSIG over the
message, namespace `file`, hash `sha512`, by an `ssh-ed25519`,
`sk-ssh-ed25519@openssh.com` or `sk-ecdsa-sha2-nistp256@openssh.com` key,
with MOAT's checks. The `syllago-loadout-v1` line keeps a loadout signature
from ever verifying as a MOAT item or list change, and theirs from verifying
as a loadout.

## Decision

The client keeps a record per `source_uri` and loadout name: the accepted
sequence, the manifest digest, and whether the loadout is active on this
machine. For each well-formed manifest it decides, in this order:

1. `signature_invalid` when the `.sig` file is missing or the signature does
   not verify over the message; `signer_not_allowed` when its key is not in
   the repository's current trusted allowed-signers list.
2. `sequence_not_greater` when a record exists, the manifest digest differs
   from the recorded digest, and the sequence is not greater than the
   recorded sequence, whether the record is active or not. A digest equal to
   the record is unchanged.
3. `report_no_record` when no record exists, `report_inactive` when the
   record is inactive, and `apply` when it is active. Only `apply` applies.

## Vectors

- `keys`: two `ssh-ed25519` keys from fixed seeds. `lists` holds `LA`, which
  allows key A, and `LB`, a later list that allows only key B.
- `digests`: manifest bytes, their canonical text and their digest. A
  vector with `expected` is malformed rather than digested.
- `manifests`: a file name and bytes, and `ok` or
  `syllago.loadout.malformed`. An `ok` vector gives the loadout's `name`.
- `name_sets`: several manifests from one repository, and whether the set is
  well formed.
- `signatures`: a `source_uri`, file name, bytes, signature (`null` when the
  `.sig` is missing), trusted list, record (`null` when none), the message
  the verifier builds, and the decision or refusal. Each vector's manifest is
  well formed.
