<!-- modeled after: (synthesized for D12) -->
## Preface

This fixture intentionally lacks a final newline. The last byte of the
file is not LF. Normalization must add one so that every downstream
byte path has a single trailing newline.

## Body

Missing trailing newlines are common in hand-edited files, especially
when the editor is configured not to add one automatically. Syllago
normalizes at write time so that the canonical form always ends with
exactly one LF.

## Notes

The roundtrip test uses this fixture as one of its ten matrix cells.
After install and uninstall, the target file must be byte-identical to
its pre-install state.

## Expectations

This file's final byte is a period, not a newline.