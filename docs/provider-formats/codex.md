# Codex (OpenAI) — Format Reference

Provider slug: `codex`

Supports: Rules, Commands

Sources: [OpenAI Codex docs](https://developers.openai.com/codex), [GitHub](https://github.com/openai/codex)

---

## Rules

**Location:** `AGENTS.md` (project root)

**Format:** Plain markdown (no frontmatter)

**Schema:** Single-file, no structured metadata. All content is always-active. No support for conditional activation, globs, or per-rule descriptions.

**Example:**

```markdown
# Project Standards

## Code Style
- Use TypeScript strict mode
- Run `npm run lint` before committing
- Maximum line length: 100

## Testing
- Write tests for all new features
- Maintain >80% code coverage
- Use Jest for unit tests
```

**Converter notes:** Only `alwaysApply: true` rules survive export to Codex. Non-always rules are excluded with a warning.

---

## Commands

**Location:** `.codex/commands/` directory

**Format:** Markdown (YAML frontmatter + body) — follows the emerging Agent Skills standard

**Status:** Limited public documentation. Format inferred from repository examples and Open Agent Skills standard.

**Frontmatter fields (estimated):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Command identifier |
| `description` | string | Yes | What the command does |

**Body:** Markdown instructions for the command.

**Note:** Codex also supports a `agents/openai.yaml` metadata file alongside skills/commands for UI integration:

```yaml
interface:
  display_name: "Human-Readable Name"
  short_description: "Brief description for UI"
  default_prompt: |
    Default instructions
```

---

## Agents

**Location:** `.codex/agents/<name>.toml` (per-agent) or `AGENTS.toml` (multi-agent)

**Format:** TOML

**Per-agent fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Agent display name |
| `description` | string | Agent purpose |
| `model` | string | LLM model to use |
| `model_reasoning_effort` | string | Reasoning effort level |
| `tools` | string[] | Available tools |
| `developer_instructions` | string | System prompt |
| `sandbox_mode` | string | Sandbox environment mode (Codex-specific, no canonical equivalent) |
| `nickname_candidates` | string[] | UI nickname options (Codex-specific, no canonical equivalent) |
| `mcp_servers` | table | MCP server configuration |
| `skills.config` | table | Skill configuration |

**Converter notes:**
- `sandbox_mode` and `nickname_candidates` are parsed during canonicalization but have no canonical equivalent — they are silently dropped. These are Codex UI features with no cross-provider analog.
- `model_reasoning_effort` maps to canonical `effort` field.
- Multi-agent format uses `[agents.<name>]` sections; single-agent uses `[agent]`.

---

## Unsupported Content Types

Codex does not support Skills (as a separate concept from commands), Hooks, or MCP configuration through file-based formats (MCP uses TOML, tracked in syllago-6mrx).
