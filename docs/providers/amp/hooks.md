<!-- provider-audit-meta
provider: amp
provider_version: "unknown (auth-gated)"
report_format: 1
researched: 2026-03-23
researcher: claude-opus-4.6
changelog_checked: https://ampcode.com/blog
-->

# Amp — Hooks

## Status: Not Supported

Amp does not have an event-based hooks system comparable to Claude Code, Gemini CLI, or Copilot CLI. [Inferred]

## Automation Model

Instead of hooks, Amp uses an agent-based automation approach:

- **Course Correct agent** — a parallel agent that runs alongside the main thread, monitors inference completion, and can inject corrections [Community]
- **Code Review agent** — automated code review that runs in parallel, customizable via `.agents/checks/` directory [Community]
- **Subagents** — independent agents spawned for parallel work on multi-step tasks [Community]

These are structurally different from hooks:
- Hooks are event-triggered scripts that run at specific lifecycle points (pre/post tool use, pre/post message)
- Amp's agents are autonomous processes that run continuously or on-demand

## Implications for Syllago

- No hook conversion path to/from Amp
- Amp is excluded from the hook interchange format (`docs/spec/hooks-v1.md`)
- The provider definition correctly returns `""` for `InstallDir(catalog.Hooks)` and `false` for `SupportsType(catalog.Hooks)`

## Documentation Gaps

- Whether Amp has any event-triggered automation beyond agent-based workflows
- Whether `.agents/checks/` could serve as a hook-like mechanism
- Full specification of the Course Correct agent's capabilities
