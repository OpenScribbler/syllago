# Hook Canonical Specification: Design Document

**Date:** 2026-03-22
**Status:** Draft
**Feature:** hook-canonical-spec
**Research:** docs/research/hook-validation/ (5 reports)

---

## Problem Statement

Every AI coding tool has hooks, but every tool's hook system is different: different event names, different JSON schemas, different response contracts, different script handling. Developers using multiple tools duplicate hook logic. Enterprise teams maintaining policy enforcement across tools do it manually.

Syllago already converts hooks between providers. This design upgrades that conversion system from an internal IR into a **standalone canonical hook specification** that:

1. Defines a neutral, provider-independent hook format
2. Models capabilities and portability explicitly
3. Enables write-once, deploy-anywhere hook authoring
4. Could be adopted by the broader ecosystem as an interchange standard

---

## Design Decisions (from brainstorm)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Naming convention | **Provider-neutral snake_case** | 4 naming conventions exist across providers. Neutral names avoid Claude Code bias. Breaking change is free (pre-release, one user). |
| Spec location | **Standalone spec in docs/spec/, syllago as reference implementation** | Signals independence. Start embedded, extractable later if adoption warrants. |
| Extension model | **Capabilities + opaque provider data** | Capabilities describe semantic intents (any provider could implement). Opaque provider data is passthrough for provider-internal details. No gray zone: "can you describe the intent without naming a provider?" |
| Portability | **Capability support matrix, not binary portable/non-portable** | Portability is relative to target. Each capability lists which providers support it. |
| Versioning | **Three artifacts versioned independently** | Spec (semver), registries (date-stamped), support matrix (living document). Different change rates need different version cadences. |
| File format | **JSON canonical, YAML accepted** | JSON is the canonical interchange format. Tooling also accepts YAML for authoring (Kubernetes pattern). Spec defines JSON; one sentence notes YAML is a valid authoring representation. |
| Script handling | **Directory is the distribution unit** | Everything in `hooks/my-hook/` ships together. Scripts travel with the manifest. |
| MCP tool names | **Structured object, not combined string** | MCP protocol doesn't define a combined format — each provider invented their own separator. Structured `{"mcp": {"server": "...", "tool": "..."}}` avoids parsing ambiguity entirely. Adapters format into provider-specific strings. |
| Timeout unit | **Seconds** | Humans think in seconds. Hook timeouts are 5-60s range where `10` is clearer than `10000`. Adapters convert to ms for providers that need it. |
| Enterprise flags | **Separate config spec** | Hook manifests define what a hook does. Whether hooks are allowed to run (DisableAllHooks, AllowManagedHooksOnly) is organizational policy — a separate concern for a future hook policy spec (OCI "split by concern" pattern). |
| Governance | **Open from day one** | Permissive license (CC-BY-4.0 for spec), CONTRIBUTING.md, GitHub-based evolution. Signals openness immediately. Formal governance structures deferred until community exists. |
| OpenCode strategy | **Export: lossless bridge plugin. Import: hook-like patterns only.** | Export generates ~30-line TypeScript plugin wrapping shell commands — maps every canonical field (events, matchers, blocking, input rewrite, structured output, timeout). Drops into `.opencode/plugin/`. Import ONLY covers hook-like behavior: tool guards (match+throw), arg rewrites, shell-out wrappers (~15-20% of real-world plugins). The remaining ~80% are application extensions (auth, custom tools, chat transforms, SDK integrations) — these are NOT hooks and are explicitly out of scope. This boundary must be clearly documented in CLI output, TUI, docs, and README. See `docs/research/hook-validation/06-opencode-deep-dive.md`. |

### Review-Driven Decisions

Decisions from the five-persona spec design audit (`docs/reviews/spec-design-audit-*.md`).

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Exit code contract | **0=success, 1=error, 2=block, other=error** | Most critical runtime behavior was only in research notes, never formally specified. 6 of 7 shell providers use exit 2 for blocking. Spec must formally define this. |
| Canonical output schema | **Core fields only: `decision`, `reason`, `continue`, `context`** | Advanced fields (`updated_input`, `suppress_output`, `system_message`) stay in capabilities. Core fields have 4-5 provider support. Clear signal: if it's in canonical, it works broadly. |
| Matcher disambiguation | **Typed matchers with string shorthand** | Bare string = tool name lookup (90% case). `{"pattern": "..."}` = regex. `{"mcp": {...}}` = MCP tool. Array = OR. Follows LSP precedent. Zero ambiguity. |
| Blocking semantics | **Per-provider behavior matrix (prevent/retry/observe)** | Replaces "Varies" which was not actionable. Extensible vocabulary — new behaviors added without structural change. |
| Capabilities | **Inferred by tooling, not declared by authors** | Every capability is detectable from manifest fields or output patterns. Tooling computes capabilities; authors never write them manually. Optional override for edge cases. |
| Degradation strategy | **Author-specified with safe defaults** | Hook manifest can specify fallback per capability. Defaults: `block` for safety capabilities (input_rewrite), `warn` for convenience capabilities (custom_env). Prevents false sense of security. |
| Script scanning | **Spec defines security metadata; doesn't mandate scanning** | Follows OCI/npm/OpenAPI pattern. Security considerations section + per-file content hashes + pluggable scanner recommendation. Scanning is ecosystem concern, not format concern. |
| Signing/provenance | **Spec defines metadata fields; syllago implements features** | Spec adds `signatures` field, per-file hashes, author metadata. Syllago implements Sigstore/cosign, trust tiers, audit logging, revocation. |
| Hook policy timing | **Interface contract ships with v1** | Enterprise can't adopt manifest spec without knowing the policy interface. Publish field definitions and control semantics alongside hooks v1. Full policy spec comes later. |
| Authoring format | **JSON canonical, YAML accepted (Kubernetes pattern)** | Spec defines JSON as canonical interchange format. Tooling also accepts YAML for authoring. Follows the most successful config spec pattern (Kubernetes). |
| CC bias mitigation | **Reframe narrative + provider-centric strengths view** | Keep unified events (better for interchange). Add provider-centric capability view highlighting each provider's unique strengths. Language audit for neutrality. Diversify examples. |
| Terminology | **Glossary section with precise definitions** | Define every key term once. Enumerate canonical provider slugs. Standardize: `provider_data` (not "opaque data"), "capability" (not "feature"), "adapter" (not "converter"). |
| Test vectors | **Written during spec drafting** | Canonical JSON with expected encode/decode per provider. Forces design validation against real formats. Conformance levels: core/extended/full. |
| Spec vs design doc | **Spec is purely normative** | Design doc stays as companion "why" document. Spec strips brainstorm notes, internal migration plans, resolved questions. |
| Distribution concerns | **Tool concerns, not spec concerns** | Hybrid distribution, update mechanism, registry publishing are syllago features. Beads created for implementation. |

---

## Research Validation

Five research tracks confirmed or refined the design. Key findings:

### Confirmed by ecosystem

- **Hub-and-spoke canonical format works** (assistantkit, casr, sondera all prove it)
- **Capability matrix is essential** (assistantkit implements it; LSP and MCP demonstrate the pattern)
- **Opaque provider data is necessary** (casr's `extra`/`metadata` fields across 14+ providers)
- **Ignore unknown fields is the #1 forward-compat rule** (OpenAPI, LSP, MCP, Standard Webhooks all require it)
- **Exit code 2 = block is de facto standard** (6 of 7 shell-based providers)
- **Pre/post tool use is universal** (8/8 providers)

### New findings incorporated

- **Two output contract families**: Rich JSON (Claude Code, VS Code Copilot, Gemini CLI) vs simple exit codes (Windsurf, Cursor, Copilot CLI). Spec must support both.
- **VS Code Copilot converges with Claude Code** — same event names, same `hookSpecificOutput` pattern. One adapter may serve both.
- **Schema versioning is a gap** — no existing tool does it. Our three-artifact approach is novel.
- **`{vendor}/{name}` extension identifiers** (MCP pattern) are cleaner than `x-` prefix.
- **Formalize existing practice** (OCI lesson) — don't invent an ideal, codify what providers already do.
- **Small core surface** (Standard Webhooks lesson) — required fields should fit on one page.
- **Read-back verification** (casr pattern) — after conversion, re-parse with target decoder to verify fidelity.
- **Trust model fragmentation** — where hooks install differs by provider (user-local, repo-bound, project-scoped). Must be transparent.
- **CVE history** — hooks were an RCE vector (Feb 2026). Registry-distributed hooks need provenance.

### Platform-specific script handling

- **CWD divergence**: 3 providers have configurable `cwd` (Windsurf, Copilot CLI, VS Code Copilot), 5 don't
- **Platform commands**: VS Code Copilot's `command` + `windows/linux/osx` override is the most expressive model
- **Environment variables are provider-branded**: each has `$PROVIDER_PROJECT_DIR`
- **OpenCode has two layers**: Hook-like behavior (~15-20% of plugins: tool guards, arg rewrites, shell-outs) maps cleanly to canonical. Application extensions (~80%: auth, custom tools, chat transforms, SDK integrations) are NOT hooks — out of scope. Export is lossless via bridge plugin. Import covers hook-like patterns only, with clear messaging that OpenCode plugins ≠ hooks.

---

## Specification Architecture

### Three Artifacts

```
docs/spec/
  hooks-v1.md              # The Specification (semver: 1.0.0)
  CONTRIBUTING.md          # Contribution process
  LICENSE                  # CC-BY-4.0
  glossary.md              # Precise term definitions
  security-considerations.md  # Threat model, attack surface, mitigations
  policy-interface.md      # Hook policy interface contract (ships with v1)
  registries/
    events.yaml            # Event Registry (date-stamped: 2026.03)
    capabilities.yaml      # Capability Registry
    tools.yaml             # Tool Vocabulary
  support-matrix.yaml      # Provider Support Matrix (living document)
  schema/
    hook.schema.json       # JSON Schema for validation
  test-vectors/
    canonical/             # Canonical hook JSON files
    claude-code/           # Expected encode output per provider
    gemini-cli/
    cursor/
    windsurf/
    copilot-cli/
    vs-code-copilot/
    kiro/
    opencode/
```

**Spec** changes rarely (structural evolution). **Registries** grow as providers add features. **Support matrix** updates with every provider release. **Test vectors** written during spec drafting — force design validation against real provider formats.

### Spec Document Structure

The spec (`hooks-v1.md`) is purely normative. It contains:
1. **Glossary** — precise definitions for: hook, handler, adapter, capability, `provider_data`, matcher, blocking, event, canonical format, provider slug (with enumerated list of canonical slugs)
2. **Format specification** — field reference, exit code contract, canonical output schema, matcher types
3. **Event registry** — canonical events with blocking behavior matrix
4. **Capability registry** — capabilities with support matrix and degradation strategies
5. **Tool vocabulary** — canonical tool names with provider mappings
6. **Security considerations** — threat model, attack surface, scanner recommendations
7. **Policy interface contract** — field definitions for DisableAllHooks, AllowManagedHooksOnly, per-capability restrictions
8. **Conformance levels** — core (all core events + exit codes), extended (capabilities + structured output), full (provider_data round-trip + verification)
9. **Test vectors** — canonical JSON with expected encode/decode per provider

The design doc (`2026-03-22-hook-canonical-spec-design.md`) stays as the companion "why" document — brainstorm decisions, research validation, internal migration plans. Not part of the spec.

### Three-Layer Hook Model

**Layer 1: Core** (universal, every provider must support)
- Event binding (which lifecycle event triggers this hook)
- Handler definition (command to execute)
- Blocking semantics (can this hook prevent the action?)

**Layer 2: Capabilities** (optional features with a provider support matrix)
- Each capability describes a semantic intent any provider could implement
- Capabilities are **inferred by tooling** from manifest fields and hook output patterns — authors do not declare them manually
- Conversion checks target provider support and generates specific warnings
- Each capability has a **degradation strategy**: author-specified fallback with safe defaults (block for safety capabilities, warn for convenience)

**Layer 3: Provider Data** (opaque passthrough for provider-internal details)
- Preserved on import, rendered for matching provider on export, silently dropped otherwise
- No schema defined by the spec — it's a JSON bag
- When patterns emerge across providers, they get promoted to capabilities

---

## Canonical Hook Format

### Minimal Example (core only)

```json
{
  "spec": "hooks/1.0",
  "hooks": [
    {
      "event": "before_tool_execute",
      "handler": {
        "type": "command",
        "command": "./check.sh"
      },
      "blocking": true
    }
  ]
}
```

### Full Example (all layers)

```json
{
  "spec": "hooks/1.0",
  "hooks": [
    {
      "event": "before_tool_execute",
      "matcher": "shell",
      "handler": {
        "type": "command",
        "command": "./safety-check.sh",
        "platform": {
          "windows": "./safety-check.ps1"
        },
        "cwd": ".",
        "env": {
          "AUDIT_LOG": "./audit.log"
        },
        "timeout": 10
      },
      "blocking": true,
      "degradation": {
        "input_rewrite": "block"
      },
      "provider_data": {
        "windsurf": {
          "show_output": true,
          "working_directory": "/opt/hooks"
        }
      }
    },
    {
      "event": "before_tool_execute",
      "matcher": {"mcp": {"server": "github", "tool": "create_issue"}},
      "handler": {
        "type": "command",
        "command": "./validate-issue.sh"
      },
      "blocking": true
    },
    {
      "event": "before_tool_execute",
      "matcher": ["file_write", "file_edit"],
      "handler": {
        "type": "command",
        "command": "./lint-on-write.sh"
      },
      "blocking": false
    },
    {
      "event": "session_start",
      "handler": {
        "type": "command",
        "command": "./init.sh"
      },
      "blocking": false
    }
  ]
}
```

Note: `capabilities` field is absent — tooling infers capabilities from the manifest (e.g., presence of `platform` block infers `platform_commands`; hook script output patterns infer `structured_output` and `input_rewrite`). The `degradation` field is optional — only needed when the author wants to override safe defaults.
```

### Field Reference

#### Top-level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec` | string | Yes | Spec version declaration (e.g., `"hooks/1.0"`) |
| `hooks` | array | Yes | List of hook definitions |

#### Hook definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event` | string | Yes | Canonical event name from the event registry |
| `matcher` | string, object, or array | No | Typed matcher (see below). Bare string = tool vocabulary lookup. `{"pattern": "..."}` = regex. `{"mcp": {...}}` = MCP tool/server. Array = OR of multiple matchers. Omitted = match all tools for this event. |
| `handler` | object | Yes | How the hook executes (see handler fields) |
| `blocking` | boolean | No | Whether this hook can prevent the triggering action (default: `false`) |
| `capabilities` | array | No | List of capability identifiers. **Inferred by tooling** — authors do not write this manually. Tooling detects capabilities from manifest fields and output patterns. Optional override for edge cases. |
| `degradation` | object | No | Per-capability fallback strategy when target provider lacks support. Keys are capability names, values are `"block"`, `"warn"`, or `"exclude"`. Overrides safe defaults. |
| `provider_data` | object | No | Keyed by provider slug, opaque passthrough data |

#### Handler fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Handler type: `"command"` (universal), or provider-specific via capabilities |
| `command` | string | Yes* | Shell command or script path (relative to hook directory) |
| `platform` | object | No | Per-OS command overrides: `windows`, `linux`, `osx`. Falls back to `command`. |
| `cwd` | string | No | Working directory, relative to project root |
| `env` | object | No | Custom environment variables passed to the hook process |
| `timeout` | number | No | Timeout in seconds (canonical unit) |
| `async` | boolean | No | Fire-and-forget execution (default: `false`) |

*Required when `type` is `"command"`. Other handler types (defined as capabilities) have their own required fields.

#### Exit Code Contract

| Exit Code | Meaning | Behavior |
|-----------|---------|----------|
| 0 | Success | Parse stdout as JSON if present. Action proceeds. |
| 1 | Hook error | Non-blocking warning. Action proceeds. Stderr logged. |
| 2 | Block | Action prevented. Stderr fed to agent/user as reason. |
| Other | Hook error | Same as exit 1 (non-blocking warning). |

#### Canonical Output Schema (stdout JSON)

All fields are optional. Adapters translate to provider-specific shapes.

| Field | Type | Description |
|-------|------|-------------|
| `decision` | string | `"allow"`, `"deny"`, or `"ask"`. Controls whether the action proceeds. |
| `reason` | string | Human-readable explanation of the decision. |
| `continue` | boolean | Whether the agent should continue after this hook. Default: `true`. |
| `context` | string | Additional context injected into the agent's conversation. |

Advanced output fields (`updated_input`, `suppress_output`, `system_message`) are capability-gated and defined in the capability registry, not here. Adapters for providers that don't support JSON output (e.g., Windsurf) map `decision: "deny"` to exit code 2 automatically.

#### Matcher Types

| Syntax | Meaning | Example |
|--------|---------|---------|
| Bare string | Tool vocabulary lookup (exact match) | `"shell"` |
| `{"pattern": "..."}` | Regex match against tool name | `{"pattern": "file_.*"}` |
| `{"mcp": {"server": "...", "tool": "..."}}` | MCP tool match | `{"mcp": {"server": "github", "tool": "create_issue"}}` |
| `{"mcp": {"server": "..."}}` | MCP server match (all tools) | `{"mcp": {"server": "github"}}` |
| Array | OR of multiple matchers | `["shell", "file_write", {"mcp": {"server": "github"}}]` |
| Omitted | Match all tools for this event | *(field absent)* |

---

## Event Registry

Canonical event names use `snake_case`, are provider-neutral, and describe the lifecycle moment.

### Core Events (universal support)

| Event | Description | Providers |
|-------|-------------|-----------|
| `before_tool_execute` | Before any tool runs | All 8 |
| `after_tool_execute` | After a tool completes | All 8 |
| `session_start` | Session begins | 7/8 (not Cursor) |
| `session_end` | Session ends | 6/8 |
| `before_prompt` | Before user input reaches the agent | 7/8 |
| `agent_stop` | Agent loop ends | 7/8 |

### Blocking Behavior Matrix

Each cell defines what happens when a hook returns exit code 2 for that event on that provider.

| Behavior | Meaning |
|----------|---------|
| **prevent** | The triggering action is stopped |
| **retry** | The agent is re-engaged to try again |
| **observe** | Hook runs but cannot affect the outcome |

| Event | Claude Code | Gemini CLI | Cursor | Windsurf | VS Code Copilot | Copilot CLI | Kiro | OpenCode |
|-------|-------------|------------|--------|----------|-----------------|-------------|------|----------|
| `before_tool_execute` | prevent | prevent | prevent | prevent | prevent | prevent (deny only) | prevent | prevent |
| `after_tool_execute` | observe | observe | observe | observe | observe | observe | observe | observe |
| `session_start` | observe | observe | - | - | observe | observe | observe | observe |
| `session_end` | observe | observe | - | - | observe | observe | observe | - |
| `before_prompt` | prevent | prevent | observe | observe | prevent | observe | observe | - |
| `agent_stop` | retry | retry | observe | observe | retry | observe | observe | observe |

### Extended Events (partial support)

| Event | Description | Providers |
|-------|-------------|-----------|
| `before_compact` | Before context compression | Claude Code, Gemini CLI, VS Code Copilot |
| `notification` | Non-blocking system notification | Claude Code, Gemini CLI |
| `error_occurred` | Agent encountered an error | Claude Code (StopFailure), Copilot CLI, OpenCode |
| `subagent_start` | Nested agent spawned | Claude Code, VS Code Copilot |
| `subagent_stop` | Nested agent finished | Claude Code, VS Code Copilot, Copilot CLI |
| `permission_request` | Permission check for sensitive action | Claude Code, OpenCode |

### Provider-Exclusive Events

| Event | Description | Provider |
|-------|-------------|----------|
| `before_model` | Before LLM API call | Gemini CLI |
| `after_model` | After LLM response received | Gemini CLI |
| `before_tool_selection` | Before LLM chooses which tool to use | Gemini CLI |
| `config_change` | Configuration was modified | Claude Code |
| `file_created` / `file_saved` / `file_deleted` | File system events | Kiro |
| `before_task` / `after_task` | Spec task execution | Kiro |
| `worktree_create` / `worktree_remove` | Git worktree lifecycle | Claude Code, Windsurf |

---

## Tool Vocabulary

Canonical tool names for use in `matcher` fields. Provider adapters translate to/from native names.

| Canonical | Description | Claude Code | Gemini CLI | Cursor | Copilot CLI | Kiro |
|-----------|-------------|-------------|------------|--------|-------------|------|
| `shell` | Shell command execution | Bash | run_shell_command | run_terminal_cmd | bash | execute_bash |
| `file_read` | Read file contents | Read | read_file | read_file | view | fs_read |
| `file_write` | Create/overwrite file | Write | write_file | edit_file | create | fs_write |
| `file_edit` | Modify existing file | Edit | replace | edit_file | edit | fs_write |
| `search` | Search file contents | Grep | grep_search | grep_search | grep | grep |
| `find` | Find files by pattern | Glob | glob | file_search | glob | glob |
| `web_search` | Search the web | WebSearch | google_web_search | web_search | - | web_search |
| `web_fetch` | Fetch URL content | WebFetch | web_fetch | - | web_fetch | web_fetch |
| `agent` | Spawn sub-agent | Agent | - | - | task | use_subagent |

MCP tool names use a structured object in the `matcher` field rather than a combined string:

```json
{"mcp": {"server": "github", "tool": "create_issue"}}
```

This avoids the parsing ambiguity of combined formats (each provider uses a different separator). Adapters format the structured object into provider-specific strings:

| Provider | Output format | Example |
|----------|--------------|---------|
| Claude Code, Kiro | `mcp__server__tool` | `mcp__github__create_issue` |
| Gemini CLI | `mcp_server_tool` | `mcp_github_create_issue` |
| Copilot CLI, Codex | `server/tool` | `github/create_issue` |
| Cursor, Windsurf, Cline | `server__tool` | `github__create_issue` |
| Zed | `mcp:server:tool` | `mcp:github:create_issue` |

---

## Capability Registry

Capabilities describe semantic intents that any provider could implement. Each has a support matrix.

### `structured_output`

Hook returns JSON with decision fields beyond simple exit codes.

| Provider | Mechanism |
|----------|-----------|
| Claude Code | `hookSpecificOutput` with `permissionDecision`, `updatedInput`, `additionalContext`, `systemMessage` |
| VS Code Copilot | Same as Claude Code |
| Gemini CLI | `decision`, `systemMessage`, `hookSpecificOutput.toolConfig` |
| Copilot CLI | `permissionDecision` only |
| Cursor | `permission`, `userMessage`, `agentMessage` |
| Windsurf | Not supported (exit codes only) |

### `input_rewrite`

Modify tool arguments before execution. **This is a safety-critical capability** — a hook that sanitizes inputs provides no protection if the rewrite is silently dropped.

| Provider | Mechanism |
|----------|-----------|
| Claude Code | `hookSpecificOutput.updatedInput` |
| VS Code Copilot | `hookSpecificOutput.updatedInput` |
| OpenCode | Mutable `output.args` in plugin |
| Others | Not supported |

**Default degradation: `block`.** If the target provider doesn't support input rewriting, the hook blocks the action entirely (exit code 2) rather than allowing unmodified input through. This prevents false security. Hook authors can override to `warn` or `exclude` via the `degradation` field if the rewrite is cosmetic rather than safety-critical.

### `llm_evaluated`

Hook logic evaluated by an LLM rather than a deterministic script.

| Provider | Mechanism |
|----------|-----------|
| Claude Code | `type: "prompt"` (single-turn) and `type: "agent"` (multi-turn with tools) |
| Kiro | "Ask Kiro" agent prompt actions (IDE only, consumes credits) |
| Others | Not supported |

**Default degradation: `exclude`.** LLM evaluation IS the hook — wrapping it in a CLI shim is fragile and requires the source provider's CLI to be installed. Hook authors can override to `warn` if partial functionality is acceptable.

### `http_handler`

Hook executes via HTTP POST to an endpoint.

| Provider | Mechanism |
|----------|-----------|
| Claude Code | `type: "http"` with `url`, `headers`, `allowedEnvVars` |
| Others | Not supported |

**Default degradation: `warn`.** Can be approximated with a `curl` command in a generated shell script. Hook author can override to `block` or `exclude`.

### `async_execution`

Hook runs without blocking the agent.

| Provider | Mechanism |
|----------|-----------|
| Claude Code | `async: true` |
| Others | Not supported (hooks are synchronous) |

**Default degradation: `warn`.** Hook executes synchronously. Warning that it may block the agent.

### `platform_commands`

Per-OS command overrides.

| Provider | Mechanism |
|----------|-----------|
| VS Code Copilot | `windows`, `linux`, `osx` fields with `command` fallback |
| Copilot CLI | `bash`, `powershell` fields |
| Others | Single `command` field only |

**Default degradation: `warn`.** Default `command` used. Platform-specific overrides dropped with warning.

### `custom_env`

Custom environment variables passed to hook process.

| Provider | Mechanism |
|----------|-----------|
| VS Code Copilot | `env` field (key-value object) |
| Others | Not supported |

**Default degradation: `warn`.** Env vars dropped.

### `configurable_cwd`

Explicit working directory for hook execution.

| Provider | Mechanism |
|----------|-----------|
| Windsurf | `working_directory` field |
| Copilot CLI | `cwd` field |
| VS Code Copilot | `cwd` field |
| Others | Not configurable (uses current directory) |

**Default degradation: `warn`.** CWD ignored. Warning that hook may execute from unexpected directory.

---

## Conversion Pipeline

### Current

```
source JSON -> Canonicalize() -> canonical JSON -> Render() -> target JSON
```

### New

```
source format
  -> adapter.Decode(source, provider)
  -> canonical hook (core + capabilities + provider_data)
  -> validate(canonical, target_provider)     <- capability checking, warnings
  -> adapter.Encode(canonical, target_provider)
  -> target format
  -> verify(re-decode target)                 <- read-back verification
```

### Pipeline stages

**1. Decode** — Provider adapter reads native format, emits canonical.
- Translates event names to canonical (`PreToolUse` -> `before_tool_execute`)
- Translates tool names in matchers (`Bash` -> `shell`)
- Converts timeout units (provider ms -> canonical seconds)
- Preserves provider-specific fields in `provider_data`
- For split-event providers (Cursor, Windsurf): maps `beforeShellExecution` -> `before_tool_execute` + `matcher: "shell"`

**2. Validate** — Check canonical against target provider capabilities.
- Infer capabilities from manifest fields and handler output patterns
- For each inferred capability: is it supported by the target?
- Apply degradation strategy per capability (author-specified or safe defaults)
- For each event: is it supported by the target? Check blocking behavior matrix.
- Generate specific warnings: "This hook uses `input_rewrite` which Cursor does not support. Degradation: blocking action instead of sanitizing."
- Plausibility check: at least one event with one handler

**3. Encode** — Provider adapter writes target format.
- Translates event names from canonical to target
- Translates tool names in matchers
- Converts timeout units (canonical seconds -> provider ms)
- Renders `provider_data` for matching target provider
- For split-event providers: maps `before_tool_execute` + `matcher: "shell"` -> `beforeShellExecution`
- Maps `platform` overrides to provider-specific fields (e.g., `platform.windows` -> Copilot CLI's `powershell` field)

**4. Verify** — Read-back the encoded output.
- Re-parse the output with the target adapter's decoder
- Compare structural fidelity (event count, handler types, command strings)
- On failure: report encoding error (don't silently produce corrupt output)

### Adapter Interface (Go)

```go
type HookAdapter interface {
    // ProviderSlug returns the provider identifier (e.g., "claude-code")
    ProviderSlug() string

    // Decode reads a provider-native hook config and returns canonical hooks
    Decode(content []byte) (*CanonicalHooks, error)

    // Encode writes canonical hooks to provider-native format
    Encode(hooks *CanonicalHooks) (*EncodedResult, error)

    // Capabilities returns what this provider supports
    Capabilities() ProviderCapabilities
}

type EncodedResult struct {
    Content    []byte            // the encoded hook config
    Filename   string            // target filename (e.g., "settings.json", "hooks.json")
    Scripts    map[string][]byte // extra files (generated wrapper scripts, etc.)
    Warnings   []ConversionWarning
}

type ConversionWarning struct {
    Severity    string // "info", "warning", "error"
    Capability  string // which capability was affected
    Description string // human-readable explanation
    Suggestion  string // what the user can do about it
}
```

---

## Hook Distribution

### Directory Structure

```
hooks/my-hook/
  hook.json           # canonical manifest (spec-compliant)
  check.sh            # referenced script (Unix)
  check.ps1           # platform alternative (Windows)
  lib/
    helpers.sh        # helper scripts travel with the hook
  .syllago.yaml       # syllago metadata
```

The entire directory is the distribution unit. On install:

1. Copy directory to `~/.syllago/hooks/<name>/`
2. Set executable permissions on script files
3. Rewrite `command` paths to absolute (already implemented in `resolveHookScripts()`)
4. Map `platform` overrides to provider-specific fields
5. Merge into provider's settings file (JSON merge for CC/Gemini, separate file for Cursor/Windsurf)

### Portability Metadata (.syllago.yaml)

```yaml
name: safety-check
description: Block dangerous shell commands across all AI coding tools
version: 1.0.0
tags: [security, shell, cross-platform]
capabilities:
  - structured_output
tested_on:
  - claude-code
  - gemini-cli
  - cursor
  - copilot-cli
```

The TUI surfaces this during browse/install:
- "Works on: Claude Code, Gemini CLI, VS Code Copilot"
- "Limited on: Cursor (no structured output)"
- "Not supported: OpenCode (TypeScript plugins only)"

### Trust and Provenance

Given CVE history (hooks as RCE vectors, Feb 2026), the spec defines metadata fields that enable security tooling. Actual security features (signing, scanning, revocation) are implementation concerns for syllago and other tools.

**Spec-defined metadata fields** (in `.syllago.yaml` or hook manifest):
- `signatures`: Optional field for cryptographic signatures (Sigstore/cosign or GPG). Enables trust policies.
- `content_hashes`: Per-file SHA256 hashes for every file in the hook directory. Enables integrity verification.
- `author`: Author identity metadata (name, email, verified identity URL). Enables provenance tracking.

**Spec-defined security considerations section** (in the spec document):
- Threat model: scripts are the attack surface, manifests are metadata
- `provider_data` is opaque — adapters MUST validate their own section during encode
- `input_rewrite` is a privileged/safety-critical capability
- Generated bridge plugins (OpenCode) inherit the trust level of the source hook
- Pluggable scanner recommendation: implementations SHOULD provide a mechanism for integrating external security scanning tools

**Syllago implementation** (tool features, not spec):
- Sigstore/cosign integration for signing
- Registry trust tiers (internal vs community)
- Execution audit logging
- Revocation mechanisms
- Script content scanning (beyond manifest-level regex)
- Install prompts with security scan results

---

## Migration from Current Format

### Breaking Changes

The current canonical format uses Claude Code event names (`PreToolUse`, `SessionStart`) and tool names (`Bash`, `Read`). The new spec uses neutral names (`before_tool_execute`, `shell`).

**Migration path:**
1. Update `toolmap.go` — add neutral canonical names alongside existing CC names
2. Update `hooks.go` — `Canonicalize()` emits new names, `Render()` accepts both old and new
3. Update `HookEvents` and `ToolNames` maps — keys become neutral names, CC entries move into the provider map
4. Existing hook.json files in registries get a one-time migration (automated, since we're pre-release)

### Scope of Neutral Naming

This change also affects other canonicalized formats in syllago (skills, agents, commands, MCP). Those should get the same treatment in future work (beads for later — the pattern is the same but each content type has its own mapping).

---

## Implementation Plan (High-Level)

### Phase 1: Spec Document

Write `docs/spec/hooks-v1.md` — the human-readable specification covering everything in this design doc. Publish alongside JSON Schema for validation.

### Phase 2: Neutral Naming Migration

Update `toolmap.go` and `hooks.go` to use neutral canonical names. Update all tests. This is the foundational change everything else builds on.

### Phase 3: Adapter Refactor

Refactor `HooksConverter` into per-provider adapters implementing the `HookAdapter` interface. Add capability declarations per provider. Add the validation step.

### Phase 4: Enhanced Format

Add `spec` version field, `capabilities` array, `provider_data` block, `platform` overrides, `cwd`, and `env` to the canonical format. Update all parsers and renderers.

### Phase 5: Read-Back Verification

Add the verify step to the conversion pipeline. Re-decode encoded output to catch encoding bugs.

### Phase 6: Portability Scoring

Surface portability information in the TUI and CLI output. Show which providers a hook supports, what's limited, what's lost.

---

## Open Questions

All design questions have been resolved. See the brainstorm decisions table and review-driven decisions table above for the complete decision log.

## Future Work (Beads)

### Spec-adjacent
- **Neutral naming for other content types**: Skills, agents, commands, and MCP canonical formats also use Claude Code-biased names. Apply the same neutral naming pattern. (Same approach, separate implementation per content type.)
- **Hook policy spec (full)**: Enterprise execution controls (DisableAllHooks, AllowManagedHooksOnly, per-capability restrictions, per-event policy, enforcement mechanism). Interface contract ships with v1; full spec is follow-on work.
- **Spec extraction**: If adoption happens, extract `docs/spec/` into a standalone repository with its own governance.

### Syllago security features
- **Cryptographic signing**: Sigstore/cosign integration for hook manifests and scripts.
- **Registry trust tiers**: Internal vs community registries with per-registry policy defaults.
- **Execution audit logging**: Timestamp, event, hook name, exit code per hook invocation.
- **Revocation mechanism**: Pull malicious hooks from fleet; forced re-verification on next execution.
- **Script content scanning**: Scan actual script files in hook directories, not just manifest command strings.
- **Pluggable scanner interface**: `HookScanner` interface for enterprise SAST/DAST integration.

### Syllago distribution features
- **Hybrid distribution**: Distribute hooks as both canonical (for syllago) and native provider configs (for direct users).
- **Hook update mechanism**: When a hook author ships a new version, how does it reach installed users?
- **Registry publishing flow**: How does a hook author get their hooks into a syllago registry?

### Syllago tooling
- **`syllago convert --batch`**: Automated batch migration of existing provider-native hooks to canonical format.
- **Capability inference engine**: Detect capabilities from manifest fields and script output patterns.

---

## Governance

The spec is published under **CC-BY-4.0** (Creative Commons Attribution). The reference implementation (syllago) uses its existing license.

### Contribution Process

1. **Propose**: Open a GitHub issue or Discussion describing the change
2. **Discuss**: Community feedback, especially from provider implementors
3. **Implement**: PR against the spec document + reference implementation
4. **Validate**: At least one adapter must implement the change before merge

### Principles

- **Additive by default**: New events, capabilities, and provider mappings don't require spec version bumps
- **Breaking changes are rare**: Removing or renaming fields requires a major spec version
- **Reference implementation required**: No paper specs — every spec feature must work in syllago
- **Provider-neutral governance**: The spec is not owned by any AI coding tool vendor

---

## References

### Research
- `docs/research/hook-research.md` — Original landscape survey (12+ tools, hook ecosystem)
- `docs/research/hook-validation/01-implementations.md` — Existing implementations (sondera, assistantkit, 1Password, casr)
- `docs/research/hook-validation/02-provider-current-state.md` — Current hook APIs for all 8 providers
- `docs/research/hook-validation/03-discourse.md` — Community demand signals, CVE history, cross-platform discourse
- `docs/research/hook-validation/04-standards-precedents.md` — Spec precedents (Git hooks, OpenAPI, LSP, MCP, OCI, Cedar, Standard Webhooks)
- `docs/research/hook-validation/05-script-references.md` — Script path resolution and platform handling per provider
- `docs/research/hook-validation/06-opencode-deep-dive.md` — OpenCode plugin model analysis (export/import feasibility)

### Spec Design Reviews
- `docs/reviews/spec-design-audit-enterprise-security.md` — Jordan (Senior Security Engineer, Fortune 500)
- `docs/reviews/spec-design-audit-hook-author.md` — Sam (Hook author, 30+ hooks, 1,200+ GitHub stars)
- `docs/reviews/spec-design-audit-tool-provider.md` — Alex (Staff Engineer, AI coding tool provider)
- `docs/reviews/spec-design-audit-tech-writer.md` — Riley (Staff Technical Writer, spec specialist)
- `docs/reviews/spec-design-audit-solo-developer.md` — Casey (Freelance dev, 3-tool user)
