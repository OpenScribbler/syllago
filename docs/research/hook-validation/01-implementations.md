# Cross-Platform Hook Normalization: Implementation Survey

**Date:** 2026-03-21
**Purpose:** Survey existing implementations of cross-platform hook normalization for AI coding tools, to inform syllago's hook validation and conversion design.

**Our proposed model reference points:**
- Capabilities + opaque provider data
- Neutral naming (not tied to any provider's terminology)
- Three-artifact versioning (schema, canonical, provider-specific)

---

## 1. sondera-ai/sondera-coding-agent-hooks

**URL:** https://github.com/sondera-ai/sondera-coding-agent-hooks
**Language:** Rust (85%), YARA (8%), Shell (7%)
**License:** MIT
**Focus:** Security enforcement (reference monitor), not content portability

### Canonical Event Model

Events normalize into four categories across all supported agents (Claude Code, Cursor, Copilot, Gemini CLI):

| Category | Examples | Purpose |
|----------|----------|---------|
| **Actions** | ShellCommand, FileRead, FileWrite, WebFetch | Pre-execution operations (blockable) |
| **Observations** | ShellCommandOutput, FileOperationResult | Post-execution responses |
| **Control** | Started, Completed, Failed, Adjudicated | Lifecycle management |
| **State** | Working dir, open files, git branch | Environment snapshots |

This is a different taxonomy than what we'd use for hooks (they're modeling security events, not automation triggers), but the normalization pattern is instructive.

### Adapter Architecture

Each agent gets a dedicated hook binary that translates native event formats:
- Claude Code's "Bash" tool -> ShellCommand
- Cursor's shell execution hook -> ShellCommand
- All adapters normalize to the same canonical types

Adapters forward events over tarpc RPC to a central harness service via Unix socket. The process isolation (separate binary per provider -> central daemon) is a security design, not a portability design, but it cleanly separates provider-specific parsing from policy evaluation.

### Key Design Decisions

- **Cedar policy engine** for evaluation -- policies are provider-agnostic once events are normalized
- **Hybrid enforcement:** deterministic YARA signatures + probabilistic LLM classifiers
- **Entity state** stored in Fjall KV store, so policies can reference accumulated context (not just single events)

### Relevance to Syllago

**Validates our model:**
- Canonical event normalization across providers works -- they prove it at the security layer
- Provider-specific adapters are the right boundary for format translation
- Once events are canonical, the same logic (their Cedar policies, our validation) applies uniformly

**Differences from our problem:**
- They normalize for security enforcement, we normalize for content portability
- Their canonical model is richer (4 categories) because they need to track state and control flow
- We don't need the Observations/Control/State categories -- just the trigger events (Before/After patterns)

**Gaps/problems they've hit:**
- Unix socket path is security-critical -- production deployment requires pre-created `/var/run/sondera/` with restricted ownership to prevent pre-binding attacks
- LLM classifier requires 12 GB model -- heavy dependency for optional features
- No mention of handling provider-specific hook capabilities that don't exist on other providers (they just normalize what overlaps)

---

## 2. plexusone/assistantkit (hooks package)

**URL:** https://pkg.go.dev/github.com/plexusone/assistantkit/hooks
**Language:** Go
**License:** MIT
**Focus:** Unified configuration management across AI coding assistants -- the closest analog to syllago's hook conversion problem

### Canonical Event Taxonomy

This is the most complete canonical event taxonomy I found. 20 events organized by domain:

**Universal events (supported by 2+ providers):**
- `BeforeFileRead`, `AfterFileRead`, `BeforeFileWrite`, `AfterFileWrite`
- `BeforeCommand`, `AfterCommand`
- `BeforeMCP`, `AfterMCP`
- `BeforePrompt`, `OnStop`

**Claude-only events:**
- `OnSessionStart`, `OnSessionEnd`, `OnPermission`, `OnNotification`, `BeforeCompact`, `OnSubagentStop`

**Cursor-only events:**
- `AfterResponse`, `AfterThought`, `BeforeTabRead`, `AfterTabEdit`

### Adapter Interface

```go
type Adapter interface {
    Name() string
    Encode(cfg *Config) ([]byte, error)
    Decode(data []byte) (*Config, error)
}
```

Three adapters: `claude`, `cursor`, `windsurf`. Registry pattern with `GetAdapter(name)`.

Direct conversion function: `Convert(data []byte, from, to string) ([]byte, error)` -- translates between any two provider formats through the canonical intermediate.

### Hook Types

Two hook types: `HookTypeCommand` (shell command) and `HookTypePrompt` (AI prompt, Claude-only).

Builder pattern for hooks: `NewCommandHook("cmd").WithTimeout(30).WithWorkingDir("/tmp").WithShowOutput(true)`

### Capability Matrix (critical finding)

They explicitly track which events each provider supports:

| Event | Claude | Cursor | Windsurf |
|-------|--------|--------|----------|
| before_file_read | Yes | Yes | Yes |
| after_file_read | Yes | No | Yes |
| before_file_write | Yes | No | Yes |
| after_file_write | Yes | Yes | Yes |
| before_command | Yes | Yes | Yes |
| after_command | Yes | Yes | Yes |
| on_session_start | Yes | No | No |
| on_session_end | Yes | No | No |
| after_response | No | Yes | No |
| after_thought | No | Yes | No |
| on_permission | Yes | No | No |
| before_compact | Yes | No | No |
| on_subagent_stop | Yes | No | No |

This is exactly the "capabilities" concept we proposed -- not all events are portable, and the system needs to know what's available where.

### Config Model

```go
type Config struct {
    DisableAllHooks      bool
    AllowManagedHooksOnly bool
    // ... hooks organized by event, with optional tool matchers
}
```

Methods: `AddHook(event, hook)`, `AddHookWithMatcher(event, matcher, hook)`, `FilterByTool(tool)`, `HookCount()`, `Events()`.

The `matcher` concept is interesting -- hooks can target specific tools within a provider (e.g., only fire BeforeCommand for "Bash" tool, not all command tools).

### Test Coverage

143 tests, 92.2% coverage across core + all three provider adapters. This is a mature, well-tested library.

### Relevance to Syllago

**Strongly validates our model:**
- Hub-and-spoke through canonical format is exactly what they do
- Capability matrix per provider is essential (not all events exist everywhere)
- Adapter pattern with Encode/Decode is the right interface boundary
- They prove the Go implementation is clean and testable

**Key design decisions we should note:**
- They use Claude's format as the "reference implementation" -- canonical format is Claude-flavored rather than truly neutral. This is pragmatic (Claude has the richest hook support) but means their canonical is slightly biased.
- Provider-specific events (Claude's `OnPermission`, Cursor's `AfterThought`) exist in the canonical taxonomy but are marked as unsupported on other providers. This is the "include everything, gate on capabilities" approach rather than "only canonicalize the intersection."
- `HookTypePrompt` is Claude-only but exists in canonical -- opaque provider data that doesn't translate.

**Differences from our approach:**
- They define the canonical as a Go library, not a file format. No schema versioning visible.
- No mention of validation beyond type safety -- if you create a config with Cursor-only events and encode for Claude, unclear what happens.
- No "opaque provider data" concept -- everything is typed in the canonical model.

**What we should learn:**
- The capability matrix is essential and they've done the research. We can reference their findings.
- Tool matchers (hook targets specific sub-tools within a provider) are a feature we haven't considered.
- `DisableAllHooks` and `AllowManagedHooksOnly` are config-level flags that matter for enterprise use.

---

## 3. 1Password/agent-hooks

**URL:** https://github.com/1Password/agent-hooks
**License:** MIT
**Focus:** Single-purpose hooks (1Password secret validation) distributed across providers

### Architecture

Rather than normalizing hooks, 1Password takes a simpler approach: one hook implementation with provider-specific installation wrappers.

**Key components:**
- `bin/run-hook.sh` -- single entry point for all hooks, all providers
- `adapters/` -- provider-specific configuration adapters
- `hooks/[hook-name]/` -- hook implementations (currently just `1password-validate-mounted-env-files`)
- `schemas/` -- JSON schema definitions
- `install.sh` -- orchestrates provider-specific installation

### Normalization Approach

Minimal normalization. The hook logic itself is provider-agnostic (a shell script), but configuration is provider-specific:

**Cursor:** `.cursor/hooks.json`
```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [{
      "command": "cursor-1password-hooks-bundle/bin/run-hook.sh 1password-validate-mounted-env-files"
    }]
  }
}
```

**GitHub Copilot:** `.github/hooks/hooks.json` (different path, potentially different event names)

The "normalization" is that the hook script is identical -- only the config wrapper that triggers it differs per provider.

### Installation Design

Two modes:
- **Bundle mode:** Creates portable directory, user configures manually
- **Bundle and move mode:** Creates + configures in one step

Key rule: **Never overwrites existing configuration.** Additive only. This matters for enterprise deployment where multiple tools may manage hooks.

### Relevance to Syllago

**Validates our model (partially):**
- Even a simple single-hook system needs provider-specific adapters
- The "same script, different config wrapper" pattern is the simplest form of what we're doing
- JSON schemas for validation are present

**Interesting design choices:**
- Project-level scoping (hooks in `.cursor/`, `.github/hooks/`) rather than global -- per-repo customization
- Relative path references for portability
- No-overwrite installation policy

**Limitations relevant to us:**
- Only handles one hook type (beforeShellExecution) -- no attempt at a general taxonomy
- No canonical intermediate -- just shell scripts that happen to be the same
- No conversion between providers -- you install separately for each
- This is "same hook on multiple providers" not "convert hooks between providers"

---

## 4. fcakyon/claude-codex-settings

**URL:** https://github.com/fcakyon/claude-codex-settings
**Stars:** 521
**License:** Apache-2.0
**Focus:** Plugin ecosystem for Claude Code + Codex with hooks as one content type

### Plugin Manifest Format

Plugins follow a directory convention:
```
plugins/[plugin-name]/
  agents/[agent-name].md
  commands/[command-name].md
  skills/[skill-name]/SKILL.md
  hooks/scripts/[hook-name].py
  .mcp.json
```

Distribution via marketplace:
```
/plugin marketplace add fcakyon/claude-codex-settings
/plugin install [plugin-name]@claude-settings
```

### Hook Implementation

- Hooks are Python scripts in `hooks/scripts/`
- Confirmation hooks require user input before destructive operations (git)
- Platform detection for tool selection (Ruff, Prettier, ShellCheck)
- No formal hook manifest -- scripts follow naming conventions

### Cross-Platform Patterns

- Symlink strategy: `ln -sfn CLAUDE.md AGENTS.md` for tool interoperability
- Environment abstraction for bash working dirs, MCP output limits
- Stateless MCP configs externalized to `.mcp.json` for portability

### Relevance to Syllago

**Validates our model (weakly):**
- Hooks are packaged alongside other content types (skills, agents, commands, MCP) -- consistent with syllago's multi-content-type model
- Distribution through a registry/marketplace pattern

**Differences:**
- No cross-provider conversion -- this is Claude Code + Codex specific
- No canonical format -- hooks are just scripts with conventions
- No formal manifest for hooks (no event binding, no capability declaration)
- The plugin system is Claude Code's native marketplace, not a portable format

**What we should learn:**
- Progressive skill disclosure is a UX pattern worth noting for our TUI
- The "plugins bundle multiple content types" pattern validates loadouts
- Python hooks with platform detection show that hook scripts themselves often need to be cross-platform even within a single provider

---

## 5. Dicklesworthstone/cross_agent_session_resumer (casr)

**URL:** https://github.com/Dicklesworthstone/cross_agent_session_resumer
**Language:** Rust
**Focus:** Session portability (not hooks), but canonical IR design is directly relevant

### Canonical IR Design

```
CanonicalSession:
  - session_id, provider_slug
  - workspace_path, title
  - timestamps (started/ended)
  - messages: Vec<CanonicalMessage>
  - metadata: serde_json::Value  // <-- opaque provider data!

CanonicalMessage:
  - role: enum { User, Assistant, Tool, System }
  - content: String
  - timestamp, author
  - tool_calls, tool_results
  - extra: serde_json::Value     // <-- opaque provider data!
```

**Critical design choice:** Both session and message levels have an opaque JSON `extra`/`metadata` field for provider-specific data that doesn't map to canonical fields. This is exactly our "opaque provider data" concept.

### Pipeline Architecture

Eight-stage deterministic pipeline:
1. Resolve target provider
2. Discover source session
3. Read source into canonical IR
4. Validate canonical session
5. Optionally enrich with synthetic context
6. Skip if no format change needed
7. Write target-native session
8. **Re-read written output to verify structural fidelity**

### Lossy Conversion Handling

Philosophy: **"Permissive conversion over brittle strictness."**

- Warnings for imperfect input when conversion remains useful
- Provider-specific extras preserved in metadata when possible
- Graceful fallback heuristics for malformed content
- Hard errors only for sessions without plausible user-assistant exchanges (minimum: one user + one assistant message)

Specific normalizations:
- Mixed string/block content -> `flatten_content`
- Multiple timestamp formats (ISO, epoch seconds, epoch ms) -> `parse_timestamp`
- Provider-specific role names -> canonical roles
- Message reindexing after filtering

### Read-Back Verification Pattern

After writing target-native format:
1. Re-parse the written file using the target provider's reader
2. Compare structural fidelity (message count, role sequences, content preservation)
3. On failure: rollback and restore `.bak` backup

This catches writer bugs before users attempt to use the converted output. The cost is one extra parse, but it prevents silent corruption.

### Provider Support

14+ providers: Claude Code, Codex, Gemini CLI, Cursor, Cline, Aider, Amp, OpenCode, ChatGPT, and more. Each implements a common `Provider` trait (read + write).

### Relevance to Syllago

**Strongly validates our model:**
- **Opaque provider data**: Their `extra`/`metadata` fields prove this pattern works at scale (14+ providers). Provider-specific data that doesn't canonicalize goes in a JSON bag, preserved through round-trips when possible.
- **Hub-and-spoke**: Read into canonical, write out to target. Same architecture we use.
- **Permissive over brittle**: Accept imperfect input, warn rather than fail. This is the right default for content portability.
- **Validation after conversion**: Their read-back verification is a pattern we should consider.

**Key design decisions to note:**
- They chose Rust for safety guarantees (atomic writes, rollback on failure)
- `serde_json::Value` for opaque data is the Rust equivalent of our `map[string]any`
- Plausibility thresholds (minimum viable session) prevent garbage-in-garbage-out
- Deterministic pipeline with numbered stages makes debugging straightforward

**Patterns we should adopt:**
- **Read-back verification**: After converting a hook config, re-parse it with the target provider's decoder to verify structural fidelity
- **Permissive conversion with warnings**: Don't fail on provider-specific fields that can't map -- preserve them if possible, warn if not
- **Plausibility validation**: Minimum viable hook config (at least one event with one hook) before accepting input

---

## 6. Bonus: zircote/ccpkg

**URL:** https://github.com/zircote/ccpkg
**Focus:** Packaging format for AI coding assistant extensions

### Archive Format

ZIP-based `.ccpkg` archives:
```
example-plugin-1.0.0.ccpkg
  manifest.json
  skills/
  agents/
  commands/
  hooks/          # hooks.json + scripts
  mcp/
  lsp/
  instructions/   # tool-specific instruction files
  LICENSE
```

### Cross-Provider Approach

- `manifest.json` has a `targets` field for per-platform instruction files
- Standards-based: MCP, LSP, Agent Skills format
- Decentralized registries (JSON files on GitHub Pages, S3, etc.)
- JSON Schema validation, semver for versioning

### Relevance

Validates that hooks-as-part-of-bundles is the direction the ecosystem is moving. Their `targets` field for per-platform variations is analogous to our provider-specific artifacts within a loadout.

---

## Summary: Cross-Cutting Findings

### What the ecosystem validates about our model

1. **Hub-and-spoke through canonical format works.** assistantkit (hooks) and casr both prove this at scale with multiple providers.

2. **Capability matrices are essential.** assistantkit's explicit tracking of which events each provider supports is the right approach -- not all hooks are portable, and the system must know what's available where.

3. **Opaque provider data is necessary.** casr's `extra`/`metadata` fields and assistantkit's `HookTypePrompt` (Claude-only) both show that some data doesn't canonicalize and must be carried along as-is.

4. **Neutral naming matters but is hard.** assistantkit uses neutral names (`BeforeCommand` not `PreToolUse`) but still has provider-specific events in the taxonomy (`OnSubagentStop`, `AfterThought`). True neutrality requires a universal subset plus provider extensions.

5. **Adapter pattern is the consensus architecture.** Every project uses some form of provider-specific adapter for encoding/decoding.

### Patterns we should adopt

| Pattern | Source | Application to Syllago |
|---------|--------|----------------------|
| Capability matrix | assistantkit | Track which events/features each provider supports; gate conversion on capabilities |
| Read-back verification | casr | After converting hook config, re-parse with target decoder to verify fidelity |
| Permissive conversion | casr | Warn on lossy conversion rather than failing; preserve opaque data when possible |
| Tool matchers | assistantkit | Hooks can target sub-tools within a provider (e.g., only "Bash" commands) |
| No-overwrite installation | 1Password | Never clobber existing hook configs during install |
| Plausibility validation | casr | Minimum viable config check before accepting input |

### Gaps in the ecosystem (opportunities for syllago)

1. **No schema versioning.** None of these projects version their canonical format. assistantkit is a Go library (types are the schema), casr uses Rust structs. If the canonical changes, there's no migration path. Our three-artifact versioning would be novel.

2. **No validation beyond type safety.** assistantkit doesn't validate that a config with Cursor-only events will work on Claude. casr validates structural fidelity but not semantic correctness. Provider-aware validation is a gap.

3. **No conflict resolution.** When converting hooks that use unsupported events on the target provider, none of these tools offer alternatives or suggestions. They either silently drop or fail.

4. **No bidirectional round-trip testing.** casr verifies writes, but nobody tests encode -> decode -> encode stability (round-trip fidelity through canonical).

### Surprises / things we hadn't considered

1. **Tool matchers** (assistantkit): Hooks can target specific sub-tools, not just event types. A `BeforeCommand` hook might only fire for "Bash" but not "Python". This is a sub-event granularity we should support.

2. **Config-level flags** (assistantkit): `DisableAllHooks` and `AllowManagedHooksOnly` are enterprise controls that exist at the config level, not individual hook level. These need to survive conversion.

3. **Prompt hooks** (assistantkit): Claude supports hooks that are AI prompts rather than shell commands. This is a fundamentally different hook type that other providers can't execute. It's the strongest case for opaque provider data.

4. **Security implications of hook normalization** (sondera): Their entire project exists because normalized hooks create a unified attack surface. If syllago converts hooks between providers, we should consider whether the converted hooks could introduce security risks the user didn't intend.
