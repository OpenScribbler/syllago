<!-- modeled after: uhop/stream-json .cursorrules -->
This file is a handoff. Cursor-specific behaviors are small; the core
rules live alongside the agent instructions.

## See AGENTS.md

The canonical rules are in AGENTS.md at the repository root. Read
that file before writing any code. This .cursorrules file captures
only the subset that is Cursor-specific.

## Cursor Tips

When a multi-file refactor is needed, use Cursor's composer mode so
the tool can reason about the whole change set at once.

Prefer Cursor's built-in search over a manual grep when looking for
symbols across the codebase. The search is symbol-aware.

Chat sessions do not persist across editor restarts; save important
conclusions to a file before closing the window.

## Formatting

Cursor respects the editor config in the repo. Do not override the
indentation or line-ending settings; the editor config is the source
of truth.

If a format-on-save misbehaves, re-open the file. Cursor caches the
formatter at file-open time; a stale cache can ignore recent changes.

## Review

Before submitting a PR, ask Cursor to review the diff. It catches many
obvious issues. Treat its suggestions as one reviewer's opinion, not
as the final word.
