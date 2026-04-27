# Zed: Hooks / Lifecycle Events

## Status: Not Supported

Zed does not have a user-facing hook/lifecycle system comparable to Claude Code's hooks
(pre/post command triggers) or VS Code's task events. There is no configuration surface
for running scripts on save, on file open, on agent action, or similar lifecycle events.

### What Zed Does Have (Internal Only)

Zed's codebase contains internal editor lifecycle events, but these are not exposed to
users or extensions as configurable hooks:

- **EditorEvent::Edited** -- emitted when a specific editor performs an edit [Inferred from DeepWiki]
- **EditorEvent::BufferEdited** -- emitted when the underlying buffer changes from any source [Inferred from DeepWiki]
- **InlayHintCache events** -- NewLinesShown, BufferEdited, SettingsChange trigger hint refreshes [Inferred from DeepWiki]
- **DismissEvent** -- emitted when a managed view (modal, popover) should close [Inferred from DeepWiki]

These are Rust-level GPUI framework events used internally by the editor. They cannot be
subscribed to from settings, rules files, or extensions.

### External Agent Hooks

For external agents running via the Agent Client Protocol (ACP), hooks are explicitly
listed as **not supported**:

> "Hooks are currently not supported."
> -- [Official] https://zed.dev/docs/ai/external-agents

This means neither Zed's built-in agent panel nor external agents (Claude Agent, Codex,
Gemini CLI) can register lifecycle hooks.

### Extension Tasks (Partial Alternative)

Zed extensions can define **tasks** (build/run configurations), but these are manually
triggered -- not event-driven hooks. They do not fire automatically on lifecycle events.

### Implications for Syllago

- Zed has no hook content type to import or export
- No equivalent format exists to map Claude Code hooks to
- If Zed adds hooks in the future, they would likely surface through the extension API
  or settings.json, but nothing is announced as of March 2026

## Sources

- [Official] Zed External Agents docs: https://zed.dev/docs/ai/external-agents
- [Official] Zed All Settings: https://zed.dev/docs/reference/all-settings
- [Community] DeepWiki Editor Core: https://deepwiki.com/zed-industries/zed/4.1-editor-core
