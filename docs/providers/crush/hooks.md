# Crush Hooks System

Crush does **not** have a user-facing hook system. There is no way for users to
run custom scripts at lifecycle points (before/after tool use, session
start/end, etc.). This is a notable gap compared to Claude Code, Cursor, and
Gemini CLI, all of which support deterministic lifecycle hooks.

**Status:** Not supported as of 2026-03-30. Plugin proposal open but not
implemented.

## What internal/event/ Actually Is

Crush's `internal/event/` package is **PostHog telemetry**, not lifecycle hooks.
It collects pseudonymous usage metrics (OS, architecture, shell, command usage,
errors) and sends them to PostHog for product analytics. [Official:
https://github.com/charmbracelet/crush]

Users can opt out via environment variables:

- `CRUSH_DISABLE_METRICS=true`
- `DO_NOT_TRACK=true`

This package has no user-extensible hook points. It is a one-way telemetry
emitter, not a lifecycle event system that users can subscribe to.

## Internal Event Bus

Crush does have an internal event-driven architecture: services publish
`tea.Msg` events to `app.events` (a buffered channel), and the Bubble Tea TUI
subscribes via `app.Subscribe()`. This is an internal implementation detail for
UI updates, not an extensibility point for users. [Community:
https://deepwiki.com/charmbracelet/crush]

## Plugin Proposal (Issue #2038)

A Caddy-style plugin system was proposed in Issue #2038 (opened January 28,
2026). The proposal describes build-time Go modules included via an `xcrush`
tool. Key points from the issue:

- Plugins would be Go modules compiled into custom Crush builds
- A minimal proof-of-concept exists (PR #2037) but has not been merged
- The author acknowledges the interface is "very new and very unstable"
- Long-term, gRPC or WASM adapters could be built on top of the plugin
  interface, enabling non-Go plugins
- The proposal explicitly notes: "adding tools should not be the primary purpose
  of plugins -- MCP is good for that"

**Status:** Open proposal, not implemented. No timeline for inclusion.

[Official: https://github.com/charmbracelet/crush/issues/2038]

## Git Hook Integration (Not Lifecycle Hooks)

Crush can be invoked from Git hooks (e.g., `pre-commit`) as a CLI tool:

```bash
# .git/hooks/pre-commit
crush review --staged-files
crush test --quick
crush format --check
```

This is standard CLI invocation from Git hooks, not a Crush-native hook system.
Any CLI tool can be called this way. [Community:
https://deepwiki.com/charmbracelet/crush]

## Comparison with Other Providers

| Feature | Crush | Claude Code | Cursor | Gemini CLI |
|---|---|---|---|---|
| Lifecycle hooks | No | Yes (4 events) | Yes (18+ events) | Yes (7 events) |
| Hook config format | N/A | JSONC | JSON | JSON Schema |
| Blocking hooks | N/A | Yes | Yes | Yes |
| Plugin system | Proposed (#2038) | N/A | N/A | N/A |
| Extensibility model | MCP only | Hooks + MCP | Hooks + MCP | Hooks + MCP |

## Implications for Syllago

Because Crush has no hook system, syllago cannot import or export hooks for
Crush. Hook content from other providers has no target in Crush. If the plugin
system (Issue #2038) ships and includes lifecycle hook support, this document
should be updated.

## Sources

- [Crush GitHub Repository](https://github.com/charmbracelet/crush) [Official]
- [Plugin Support Issue #2038](https://github.com/charmbracelet/crush/issues/2038) [Official]
- [Crush Architecture -- DeepWiki](https://deepwiki.com/charmbracelet/crush) [Community]
