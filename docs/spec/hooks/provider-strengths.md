# Provider Strengths

> **Non-normative companion** to the [Hook Interchange Format Specification](hooks.md).
> This document highlights what each provider does best, independent of how many
> canonical features it supports.

**Last Modified:** 2026-04-08
**Status:** Initial Development

---

Each provider brings unique capabilities to the hook ecosystem. This document highlights what each provider does best, independent of how many canonical features it supports.

## Claude Code

The richest structured output contract. Hooks can return nuanced decisions (`allow`/`deny`/`ask`), rewrite tool inputs, inject system messages, suppress output, and add conversation context. Four handler types (command, HTTP, prompt, agent) cover deterministic, network, and LLM-evaluated hooks. The deepest event set (25+ events) provides fine-grained lifecycle control.

## Gemini CLI

Unique LLM-interaction hooks. `before_model` can intercept or mock LLM API calls before they reach the model. `after_model` enables real-time response redaction and PII filtering. `before_tool_selection` dynamically filters which tools the model can choose. No other provider offers this level of control over the model interaction layer.

## Kiro

File system lifecycle hooks. `file_created`, `file_saved`, and `file_deleted` events with glob pattern matching enable hooks that respond to file changes, not just tool invocations. Spec task hooks (`before_task`, `after_task`) integrate with Kiro's specification-driven development workflow.

## Cursor

Pre-read file access control. The `beforeReadFile` event lets hooks gate file reads, enabling data classification and access control patterns. The split-event model (`beforeShellExecution`, `beforeMCPExecution`, `beforeReadFile`) provides category-specific context without matcher overhead.

## Windsurf

Enterprise deployment infrastructure. Cloud dashboard hook management, MDM deployment (Jamf, Intune, Ansible), immutable system-level hooks, and a three-tier priority system (system > user > workspace) support organizational policy enforcement at scale. Transcript access hooks enable compliance auditing.

## OpenCode

Programmatic extensibility. TypeScript/JavaScript plugins with in-process execution, mutable argument objects, custom tool definitions, and npm package distribution. The highest event granularity (~30+ events) including LSP integration and TUI interaction hooks. Best suited for deep tool customization beyond what declarative hook configs can express.

## VS Code Copilot

Convergent implementation. Intentionally aligned with Claude Code's hook contract (same event names, same output schema, same `hookSpecificOutput` pattern). Adds OS-specific command overrides and custom environment variables. The `/create-hook` AI-assisted generation feature lowers the authoring barrier.

## Copilot CLI

Conservative safety design. Hook failures are explicitly non-blocking ("a broken hook script shouldn't take down your workflow"). Repository-bound hooks (`.github/hooks/`) ensure hooks travel with code. The `bash`/`powershell` split provides explicit cross-platform support without runtime detection.
