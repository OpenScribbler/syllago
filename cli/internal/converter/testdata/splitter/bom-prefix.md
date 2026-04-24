<!-- modeled after: (synthesized for D12) -->
## Preface

This fixture begins with a UTF-8 BOM. The canonical normalization
helper strips the BOM before any further processing so that
downstream byte equality holds.

## Body

BOM-prefixed files appear in the wild when a user saves with an editor
that emits BOMs by default. Syllago normalizes at every boundary to
make the presence or absence of a BOM invisible to the splitter and
install paths.

## Notes

The roundtrip test uses this fixture as one of its ten matrix cells.
After install and uninstall, the target file must be byte-identical to
its pre-install state, regardless of whether the fixture had a BOM.

## Expectations

Normalization strips the BOM exactly once on read. Write paths do not
re-introduce a BOM. The canonical form has no BOM.
