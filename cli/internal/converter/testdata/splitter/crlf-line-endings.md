<!-- modeled after: (synthesized for D12) — CRLF throughout -->
## Preface

This fixture exercises the CRLF to LF normalization rule. Every line
below this preface ends in CRLF. The canonical helper must convert
each CRLF to a single LF before downstream processing.

## Body

The splitter and the install path should see identical byte sequences
for this fixture and for its LF-only sibling once normalization runs.
If they disagree, the normalization layers do not agree.

Run the roundtrip test to confirm: install followed by uninstall must
leave the target file byte-identical to its pre-install state.

## Notes

CRLF files are common on Windows-authored rules files. Users do not
always normalize line endings before committing. Syllago normalizes
at every byte boundary so that downstream logic is line-ending
agnostic.

## Expectations

When a CRLF fixture is read, split, written to the library, and
installed, the target file should contain the canonical (LF-only)
content. Uninstall searches for the canonical bytes and removes them.
