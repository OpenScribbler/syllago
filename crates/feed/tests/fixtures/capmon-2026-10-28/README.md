# capmon feed snapshot, 2026-10-28

The live capmon feed as published on 2026-10-28, with GitHub's
build-provenance attestation for its index. `tests/feed.rs` verifies it under
the bundled public-good Sigstore root, and the adapters crate's golden
fixtures are rendered from it. Every file is byte-exact (`-text` in
`.gitattributes`), because the digests below are checked.

| File | Source | SHA-256 |
|---|---|---|
| `v2/index.json` | https://openscribbler.github.io/capmon/v2/index.json | `7d3d2ad15d229365c6e769fd89c613d8e4327131d0b0fd161075ed8f84c8c9a5` |
| `v2/capabilities/claude-code.json` | https://openscribbler.github.io/capmon/v2/capabilities/claude-code.json | `1ea7f75c97587077aca69ddd9a21d1e96b45ffc6a2ae1778689439f0a987389d` |
| `v2/capabilities/codex.json` | https://openscribbler.github.io/capmon/v2/capabilities/codex.json | `a36ab0cc368598d0847986a95315a5203444eb0b3309f62acdd59384d9dbf702` |
| `attestations/sha256-<index digest>.json` | https://api.github.com/repos/OpenScribbler/capmon/attestations/sha256:<index digest> | the API response, verbatim |

The index names `source_commit` `eabd8de632962982e825324d37a9448222106bbd`
and `generated_at` `2026-10-28T09:11:22Z`. The attestation's certificate is
for `https://github.com/OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main`,
logged in Rekor at integrated time 1793178684.

## Refreshing

Take all files in one sitting: capmon redeploys daily, and each deploy
changes the index and its digest.

```sh
dir=crates/feed/tests/fixtures/capmon-<date>
mkdir -p "$dir/v2/capabilities" "$dir/attestations"
base=https://openscribbler.github.io/capmon
curl -fsS -o "$dir/v2/index.json" "$base/v2/index.json"
for p in claude-code codex; do
  curl -fsS -o "$dir/v2/capabilities/$p.json" "$base/v2/capabilities/$p.json"
done
digest=$(sha256sum "$dir/v2/index.json" | cut -d' ' -f1)
curl -fsS -H 'Accept: application/vnd.github+json' \
  -o "$dir/attestations/sha256-$digest.json" \
  "https://api.github.com/repos/OpenScribbler/capmon/attestations/sha256:$digest"
```

Check that each provider document's digest matches the index entry for it.
Then point `snapshot()` in `tests/feed.rs` and `snapshot_feed()` in
`crates/adapters/tests/common/mod.rs` at the new directory, update the
expected index digest and the verification time (a day after the
attestation), and regenerate the golden fixtures with
`SYLLAGO_UPDATE_GOLDEN=1 sandbox-run --in-place cargo test -p adapters --test golden`.
The golden diff shows what syllago would write differently. Update this
README's table and remove the old directory.
