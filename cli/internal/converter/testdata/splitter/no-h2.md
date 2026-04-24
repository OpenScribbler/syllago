<!-- modeled after: (synthesized) -->
# Project Conventions

This document captures the informal conventions the team has settled
into over time. It intentionally avoids section structure; the content
is a single flowing piece of prose rather than a menu.

Keep commits small. Large commits are hard to review and hard to
revert. If a change grows while you are writing it, split it into
multiple commits with clear logical boundaries.

Lint and format on every save. The editor config enforces whitespace
rules automatically, but the linter catches things the formatter will
not, like unused imports and dead code paths.

Prefer explicit names over clever ones. Code is read far more often
than it is written. Spell out what a variable holds rather than coining
a short name that requires a context switch to decode.

Tests live alongside the code they cover. This is a small thing but it
adds up: discovering where a test for foo.go lives should be a muscle
reflex, not a directory walk.

When a function grows past thirty lines, look for a factoring. A long
function is usually three short functions in disguise. Extract them and
the call site reads as pseudocode.

Never commit secrets. Use the vault integration for production
credentials and .env.example for local defaults. Rotate any secret
that has been exposed, even briefly, and note the rotation in the
incident log.

Write for the next reader. Comments answer why, not what. When the code
does something that looks odd, leave a one-line note explaining the
reason. Future you is the most grateful reader.
