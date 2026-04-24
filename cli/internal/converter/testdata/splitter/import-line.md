<!-- modeled after: (synthesized for D4) -->
## Introduction

This fixture contains an @import directive. The splitter must preserve
the line byte-for-byte so that whichever atomic rule contains the
directive can reproduce it on install.

## Import

Shared rules live elsewhere. Include them via an import directive:

@import shared/rules.md

The directive above must pass through the splitter unchanged. No
rewriting, no slug substitution, no resolution at split time.

## Conventions

Imports are resolved at install time by the target provider, not by
syllago. Syllago treats the directive as opaque text. The user is
responsible for ensuring the imported file exists at the expected path.

If the target provider does not support imports, the directive is
ignored by the provider (but still preserved on disk by syllago).

## Coverage

Splitter tests assert byte preservation of the @import line. The
roundtrip test covers install and uninstall of a rule that contains an
import directive.
