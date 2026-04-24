<!-- modeled after: (synthesized for D12) -->
## Notes

This fixture intentionally contains a line with two trailing spaces.  
The previous line ends with two spaces and a newline, which is a
common markdown idiom for a forced line break. The splitter and
normalization pipeline must preserve this byte-for-byte.

Standard prose follows to keep the file above the skip-split
thresholds while exercising the trailing-whitespace preservation.

## Expectations

Normalization rules preserve trailing whitespace on lines. Only the
final trailing newline count is normalized. Indentation is preserved
as authored.

The canonical helper at cli/internal/converter/canonical touches only
CRLF-to-LF, UTF-8 BOM stripping, and trailing newline count. Nothing
else changes.

## Coverage

The end-to-end roundtrip test in cli/internal/installer/roundtrip_test.go
uses this fixture as one of its ten matrix cells. Byte equality after
uninstall is the acceptance criterion.

If the whitespace survives the full chain, the normalization layers
agree on preservation semantics. If it does not survive, one of the
five normalization call sites is applying extra transforms.

## Debugging

When the roundtrip fails on this fixture, compare the rule.md bytes
against .history/sha256-<hex>.md bytes first. Any difference there
points at the write path. If those match, the mismatch is downstream.
