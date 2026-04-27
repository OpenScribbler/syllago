# Roo Code: Hooks

## Status: Not Supported

Roo Code does not have a hooks system as of March 2026. There is no equivalent to Claude Code's lifecycle hooks (PreToolUse, PostToolUse, SessionStart, etc.). The official documentation at docs.roocode.com lists no hooks feature, and searching the GitHub repository confirms this. [Official](https://docs.roocode.com/features/)

### Community Feature Request

A GitHub discussion (#6147) proposes adding event-driven automation with hooks stored as individual JSON files under `.roo/hooks/` (e.g., `test-automation.json`, `documentation-sync.json`). This remains a feature request, not an implemented feature. [Community](https://github.com/RooCodeInc/Roo-Code/discussions/6147)

### Closest Alternatives

Roo Code offers several features that partially overlap with what hooks provide in other tools:

| Feature | What It Does | Hooks Overlap |
|---------|-------------|---------------|
| **Custom Tools** | TypeScript/JS files in `.roo/tools/` that Roo can invoke like built-in tools | Can encode project-specific actions, but are agent-invoked, not event-driven |
| **Custom Instructions** | Always-active behavioral guidelines | Can enforce standards, but apply universally rather than on specific events |
| **Skills** | On-demand instruction packages with bundled files | Activate when requests match, but are prompt-driven, not lifecycle-driven |
| **Mode tool restrictions** | `fileRegex` patterns on edit groups | Prevent edits to certain files, similar to PreToolUse blocking, but static |

[Official — Custom Tools](https://docs.roocode.com/features/experimental/custom-tools)
[Official — Custom Instructions](https://docs.roocode.com/features/custom-instructions)
[Official — Skills](https://docs.roocode.com/features/skills)

### Implications for Syllago

Since Roo Code has no hooks system, syllago's hook content type has no export target for Roo Code. If hooks are imported from another provider (e.g., Claude Code), they cannot be converted to a Roo Code equivalent — the conversion should either skip hooks with a warning or document the gap.
