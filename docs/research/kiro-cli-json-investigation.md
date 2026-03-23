# Investigation: Kiro CLI JSON Agent Format

> Research compiled 2026-03-22. Investigating Kiro CLI's JSON agent format vs IDE markdown format.

## Summary

Kiro uses **two distinct agent formats** for two distinct products: the **IDE** (VS Code-based) uses markdown files with YAML frontmatter, while the **CLI** (terminal tool, formerly Amazon Q Developer CLI) uses JSON configuration files. Both store agents in `.kiro/agents/` but the file extensions differ (`.md` vs `.json`). The CLI JSON format is significantly richer than the IDE markdown format, supporting fields like `allowedTools`, `toolAliases`, `toolsSettings`, `resources`, `hooks`, `keyboardShortcut`, and `welcomeMessage` that have no IDE markdown equivalent. Syllago currently handles the IDE markdown format only. Adding CLI JSON support is recommended -- the CLI is gaining traction (especially in AWS/enterprise contexts) and the JSON format is well-documented with a stable schema.

## IDE Format (Currently Supported)

Syllago's `kiroAgentMeta` struct in `cli/internal/converter/agents.go` handles Kiro IDE agents: markdown files with YAML frontmatter in `.kiro/agents/*.md`.

**Supported IDE frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Agent identifier |
| `description` | string | No | Purpose (used for auto-selection of subagents) |
| `tools` | string[] | No | Tool access list (`read`, `write`, `shell`, `@mcp-server`, `*`) |
| `model` | string | No | LLM model ID (e.g., `claude-sonnet-4`) |
| `includeMcpJson` | bool | No | Load MCP servers from settings |
| `includePowers` | bool | No | Include Powers MCP tools |

The markdown body contains the agent's system prompt.

**Example IDE agent** (`code-reviewer.md`):
```markdown
---
name: code-reviewer
description: Expert code review assistant.
tools: ["read", "@context7"]
model: claude-sonnet-4
---
You are a senior code reviewer.
```

## CLI Format (JSON)

Kiro CLI agents are JSON files in `.kiro/agents/*.json` (project-local) or `~/.kiro/agents/*.json` (global). The filename (minus `.json`) becomes the agent name. The CLI format is substantially richer than the IDE format.

**CLI JSON fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Agent name (derived from filename if omitted) |
| `description` | string | No | Human-readable purpose |
| `prompt` | string | No | System prompt text or `file://` URI to external file |
| `model` | string | No | LLM model ID |
| `tools` | string[] | No | Available tools (`read`, `write`, `shell`, `aws`, `knowledge`, `@server`, `@server/tool`, `*`, `@builtin`) |
| `allowedTools` | string[] | No | Tools pre-approved (no permission prompt); supports glob patterns |
| `toolAliases` | map[string]string | No | Remap tool names (e.g., `"@git/git_status": "status"`) |
| `toolsSettings` | map[string]object | No | Per-tool config (e.g., `write.allowedPaths`, `aws.allowedServices`, `shell.allowedCommands`) |
| `mcpServers` | map[string]object | No | Embedded MCP server definitions with `command`, `args`, `env`, `timeout`, `oauth` |
| `resources` | string[] or object[] | No | Context files/skills/knowledge bases (`file://`, `skill://`, or knowledge base objects) |
| `hooks` | map[string]object[] | No | Lifecycle hooks: `agentSpawn`, `userPromptSubmit`, `preToolUse`, `postToolUse`, `stop` |
| `keyboardShortcut` | string | No | Shortcut to switch to agent (e.g., `ctrl+r`) |
| `welcomeMessage` | string | No | Message shown when switching to agent |
| `includeMcpJson` | bool | No | Load MCP servers from `~/.kiro/settings/mcp.json` |
| `useLegacyMcpJson` | bool | No | Backward compat for legacy MCP config |

**Example CLI agent** (`aws-specialist-agent.json`):
```json
{
  "name": "aws-specialist-agent",
  "description": "Specialized custom agent for AWS infrastructure and development tasks",
  "prompt": "You are an expert AWS infrastructure specialist",
  "tools": ["read", "write", "shell", "aws"],
  "allowedTools": ["read", "aws"],
  "toolsSettings": {
    "aws": {
      "allowedServices": ["s3", "lambda", "cloudformation"]
    },
    "write": {
      "allowedPaths": ["infrastructure/**", "scripts/**"]
    }
  },
  "resources": [
    "file://README.md",
    "file://infrastructure/**/*.yaml"
  ],
  "hooks": {
    "agentSpawn": [
      {
        "command": "aws sts get-caller-identity",
        "timeout_ms": 10000,
        "cache_ttl_seconds": 300
      }
    ]
  },
  "model": "claude-sonnet-4"
}
```

### Hook objects

Each hook entry supports:
- `command` (required): shell command
- `timeout_ms` (optional): execution timeout
- `cache_ttl_seconds` (optional): cache duration for hook output
- `max_output_size` (optional): truncate output
- `matcher` (optional): tool name pattern for `preToolUse`/`postToolUse` hooks

### Resource objects (knowledge bases)

```json
{
  "type": "knowledgeBase",
  "source": "file://./docs",
  "name": "ProjectDocs",
  "description": "Project documentation",
  "indexType": "best",
  "autoUpdate": true
}
```

## Format Comparison

| Field | IDE (.md frontmatter) | CLI (.json) | Notes |
|-------|----------------------|-------------|-------|
| `name` | Yes | Yes | |
| `description` | Yes | Yes | |
| `prompt` (body/field) | Markdown body | `prompt` field (or `file://`) | Different mechanism, same purpose |
| `model` | Yes | Yes | |
| `tools` | Yes | Yes | Same syntax |
| `allowedTools` | No | Yes | CLI-only: pre-approved tools |
| `toolAliases` | No | Yes | CLI-only: tool name remapping |
| `toolsSettings` | No | Yes | CLI-only: per-tool config (paths, services, commands) |
| `mcpServers` | No | Yes (embedded) | CLI embeds full server definitions |
| `resources` | No | Yes | CLI-only: file/skill/knowledge base context |
| `hooks` | No | Yes | CLI-only: lifecycle hooks |
| `keyboardShortcut` | No | Yes | CLI-only |
| `welcomeMessage` | No | Yes | CLI-only |
| `includeMcpJson` | Yes | Yes | Both |
| `includePowers` | Yes | No | IDE-only |
| `useLegacyMcpJson` | No | Yes | CLI-only |

## Prevalence

**Kiro CLI is gaining significant traction.** Key indicators:

- **Launched late 2025** as a rebrand of Amazon Q Developer CLI; reached general availability December 2025.
- **Enterprise adoption:** AWS-centric organizations (BT Group, National Australia Bank) are deploying it. The AWS Public Sector Blog actively promotes it for DevOps automation.
- **Active ecosystem:** Multiple community agent repositories on GitHub (aws-samples/sample-kiro-assistant, Theadd/kiro-agents, iamaanahmad/everything-kiro-ide). Third-party tools (Budgie MCP server, AssistantKit) already convert between agent formats.
- **CLI agents are the power-user format.** The richer JSON schema supports enterprise concerns (tool restrictions, path allowlists, AWS service scoping) that the simpler IDE markdown format does not.
- **ACP protocol:** Kiro CLI implements the Agent Client Protocol, enabling use in JetBrains, Zed, and other editors -- expanding reach beyond the IDE.

The IDE markdown format remains the entry point for casual users, but the CLI JSON format is where the configuration depth lives and where enterprise/team sharing happens. Both formats coexist in `.kiro/agents/` -- a project can have both `.md` and `.json` agent files.

## Recommendation

**Yes, syllago should add CLI JSON agent support. Priority: Medium-High.**

### Rationale

1. **It's a distinct format.** The CLI JSON is not a variant of the IDE markdown -- it's a separate schema with a superset of fields. Treating `.json` files in `.kiro/agents/` requires a dedicated code path.

2. **Field mapping is straightforward.** Most CLI JSON fields have clear canonical equivalents or can be handled the same way as existing Kiro-specific fields (dropped with warnings). The `prompt` field maps to the markdown body. `tools` uses the same vocabulary. `name`, `description`, `model` are direct mappings.

3. **Fields needing special handling:**
   - `allowedTools` -- no canonical equivalent; drop with warning (same as current `toolsSettings` handling)
   - `toolAliases` -- no canonical equivalent; drop with warning (already handled for IDE format)
   - `toolsSettings` -- no canonical equivalent; drop with warning (already handled)
   - `mcpServers` (embedded objects) -- could map server names to canonical `mcpServers` string list; the full server config has no canonical equivalent
   - `resources` -- no canonical equivalent; drop with warning
   - `hooks` (CLI format is richer than IDE format) -- the hook objects with `command`/`timeout_ms`/`cache_ttl_seconds` are CLI-specific; could map to canonical hooks or drop with warning
   - `keyboardShortcut`, `welcomeMessage` -- drop with warning (already handled)
   - `prompt` as `file://` URI -- would need to resolve or note that the prompt is external

4. **Detection is simple.** Check file extension: `.json` = CLI format, `.md` = IDE format. Both live in `.kiro/agents/`.

5. **Rendering support.** When rendering to Kiro, syllago could offer a choice of format, or default to the richer CLI JSON format for maximum fidelity. The CLI JSON can express everything the IDE markdown can, plus more.

### Implementation sketch

- Add `canonicalizeKiroAgentJSON(content []byte)` for JSON-to-canonical conversion
- Add `renderKiroAgentJSON(meta AgentMeta, body string)` for canonical-to-JSON rendering
- Update `kiro.go` provider to detect `.json` files in agents directory
- Consider a provider variant or format flag if both formats need to coexist

### Sources

- [Agent Configuration Reference (CLI)](https://kiro.dev/docs/cli/custom-agents/configuration-reference/)
- [Creating Custom Agents (CLI)](https://kiro.dev/docs/cli/custom-agents/creating/)
- [Agent Examples (CLI)](https://kiro.dev/docs/cli/custom-agents/examples/)
- [Custom Agents Overview (CLI)](https://kiro.dev/docs/cli/custom-agents/)
- [Subagents (IDE)](https://kiro.dev/docs/chat/subagents/)
- [Introducing Kiro CLI (Blog)](https://kiro.dev/blog/introducing-kiro-cli/)
- [IDE Changelog](https://kiro.dev/changelog/ide/)
