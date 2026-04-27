# Codex CLI Content Types

Research date: 2026-03-21

Codex CLI is OpenAI's open-source coding agent (built in Rust). Configuration uses
TOML (`config.toml`) with layered scoping: global (`~/.codex/`), project
(`.codex/`), and per-directory overrides.

---

## 1. Instructions (AGENTS.md)

`[Official]` https://developers.openai.com/codex/guides/agents-md

### Format

Plain Markdown. No front matter required. Free-form prose instructions that Codex
reads before starting work.

### Discovery (root-to-leaf walk)

Codex walks from the project root down to the current working directory, checking
each level in order:

1. `AGENTS.override.md` (if present, used instead of `AGENTS.md` at that level)
2. `AGENTS.md`
3. Fallback filenames from `project_doc_fallback_filenames`

Files concatenate root-to-leaf; closer files take higher priority.

Global scope is also checked: `~/.codex/AGENTS.override.md` then `~/.codex/AGENTS.md`.

### File Locations

| Scope | Path | Purpose |
|-------|------|---------|
| Global | `~/.codex/AGENTS.md` | Personal defaults across all repos |
| Global override | `~/.codex/AGENTS.override.md` | Temporary global override |
| Project root | `AGENTS.md` | Repo-wide instructions |
| Project override | `AGENTS.override.md` | Repo-wide temporary override |
| Subdirectory | `subdir/AGENTS.md` | Team/service-specific guidance |
| Subdirectory override | `subdir/AGENTS.override.md` | Subdirectory temporary override |

### Config Knobs (in `config.toml`)

```toml
# Alternative filenames to try when AGENTS.md is missing at a directory level
project_doc_fallback_filenames = ["TEAM_GUIDE.md", ".agents.md", "CONTRIBUTING.md"]

# Maximum combined instruction size (default: 32 KiB)
project_doc_max_bytes = 32768
```

### Example

**`~/.codex/AGENTS.md`** (global):
```markdown
- Always run tests after modifications
- Prefer pnpm for dependencies
- Request confirmation before adding production dependencies
```

**`AGENTS.md`** (repo root):
```markdown
- Run npm run lint before pull requests
- Document public utilities in docs/
```

**`services/payments/AGENTS.override.md`** (nested override):
```markdown
- Use make test-payments instead of npm test
- Never rotate API keys without notifying security
```

---

## 2. MCP Servers

`[Official]` https://developers.openai.com/codex/mcp

MCP server configuration lives in `config.toml` under `[mcp_servers.<id>]` tables.
Supports both STDIO and Streamable HTTP transports.

### Config Location

| Scope | Path |
|-------|------|
| Global | `~/.codex/config.toml` |
| Project | `.codex/config.toml` (trusted projects only) |

### STDIO Transport Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | Yes | - | Launcher command |
| `args` | string[] | No | `[]` | Command arguments |
| `cwd` | string | No | - | Working directory |
| `env` | map | No | - | Environment key/value pairs copied as-is |
| `env_vars` | string[] | No | - | Forward named vars from parent environment |
| `enabled` | bool | No | `true` | Disable without removing config |
| `required` | bool | No | `false` | Fail startup if server can't initialize |
| `enabled_tools` | string[] | No | - | Allowlist of tool names |
| `disabled_tools` | string[] | No | - | Denylist (applied after allowlist) |
| `startup_timeout_sec` | number | No | `10.0` | Startup timeout in seconds |
| `tool_timeout_sec` | number | No | `60.0` | Per-tool execution timeout |
| `scopes` | string[] | No | - | OAuth scopes |
| `oauth_resource` | string | No | - | RFC 8707 resource parameter |

### HTTP Transport Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | Yes | - | Streamable HTTP endpoint |
| `bearer_token_env_var` | string | No | - | Env var sourcing bearer token |
| `http_headers` | map | No | - | Static request headers |
| `env_http_headers` | map | No | - | Headers sourced from env vars |
| `enabled` | bool | No | `true` | Disable without removing config |
| `required` | bool | No | `false` | Fail startup if unavailable |
| `enabled_tools` | string[] | No | - | Allowlist |
| `disabled_tools` | string[] | No | - | Denylist |
| `startup_timeout_sec` | number | No | `10.0` | Startup timeout |
| `tool_timeout_sec` | number | No | `60.0` | Tool execution timeout |

### Global OAuth Settings

```toml
# Fixed OAuth callback port (optional)
mcp_oauth_callback_port = 8080

# Custom callback URL for remote environments (optional)
mcp_oauth_callback_url = "http://localhost:8080/callback"

# Credential storage: auto | file | keyring
mcp_oauth_credentials_store = "auto"
```

### TOML Example

```toml
[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]

[mcp_servers.context7.env]
MY_ENV_VAR = "MY_ENV_VALUE"

[mcp_servers.figma]
url = "https://mcp.figma.com/mcp"
bearer_token_env_var = "FIGMA_OAUTH_TOKEN"
http_headers = { "X-Figma-Region" = "us-east-1" }

[mcp_servers.chrome_devtools]
url = "http://localhost:3000/mcp"
enabled_tools = ["open", "screenshot"]
startup_timeout_sec = 20
tool_timeout_sec = 45
```

### CLI Management

```bash
codex mcp add <server-name> --env VAR1=VALUE1 -- <stdio-command>
codex mcp list [--json]
codex mcp remove <server-name>
```

---

## 3. Custom Agents

`[Official]` https://developers.openai.com/codex/subagents

Custom agents are standalone TOML files, one per agent. They define specialized
roles that can be spawned as subagents.

### File Locations

| Scope | Path |
|-------|------|
| Personal | `~/.codex/agents/<name>.toml` |
| Project | `.codex/agents/<name>.toml` |

### Agent TOML Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Identifier for spawning (source of truth, not filename) |
| `description` | string | Yes | Guidance on when to use this agent |
| `developer_instructions` | string | Yes | Core behavioral directives |
| `nickname_candidates` | string[] | No | Display names for spawned instances |
| `model` | string | No | LLM selection override |
| `model_reasoning_effort` | enum | No | `minimal\|low\|medium\|high\|xhigh` |
| `sandbox_mode` | enum | No | `read-only\|workspace-write\|danger-full-access` |

Agents can also include `[mcp_servers]` and `[skills.config]` sub-tables to scope
tools and skills to that agent.

### Agent Example

**`~/.codex/agents/pr_explorer.toml`**:
```toml
name = "pr_explorer"
description = "Read-only codebase explorer for gathering evidence before changes are proposed."
developer_instructions = """
You explore codebases to gather evidence. Do not make changes.
Focus on understanding architecture, dependencies, and patterns.
"""

model = "gpt-5.3-codex-spark"
model_reasoning_effort = "medium"
sandbox_mode = "read-only"
```

### Built-in Agents

These can be overridden by placing a file with the matching name:

| Name | Purpose |
|------|---------|
| `default` | General-purpose fallback |
| `worker` | Execution and implementation |
| `explorer` | Read-heavy codebase analysis |

### Global Agent Settings (in `config.toml`)

```toml
[agents]
max_threads = 6                  # Concurrent agent threads (default: 6)
max_depth = 1                    # Nesting depth (default: 1)
job_max_runtime_seconds = 1800   # Per-worker timeout for CSV jobs
```

### Agent Roles (in `config.toml`)

Named roles can also be defined inline in `config.toml`:

```toml
[agents.reviewer]
description = "Looks for correctness, security, and test risks"
config_file = "agents/reviewer.toml"  # Relative to this config file
nickname_candidates = ["Rev-A", "Rev-B", "Rev-C"]
```

---

## 4. Skills

`[Official]` https://developers.openai.com/codex/skills

Skills are the primary extensibility mechanism. A skill packages instructions,
references, scripts, and optional metadata into a directory.

### Directory Structure

```
my-skill/
  SKILL.md              # Required: front matter + instructions
  scripts/              # Optional: executable scripts
  references/           # Optional: reference documents
  assets/               # Optional: icons, images
  agents/
    openai.yaml         # Optional: UI metadata and policy
```

### SKILL.md Format

YAML front matter followed by Markdown instructions:

```yaml
---
name: draft-pr
description: >
  Drafts a pull request description from staged changes.
  Use when the user asks to create or draft a PR.
---

## Instructions

1. Run `git diff --cached` to see staged changes
2. Summarize the changes into a PR title and description
3. Use conventional commit style for the title
```

**Required front matter fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill identifier |
| `description` | string | When this skill should/should not trigger |

### agents/openai.yaml Schema

Optional metadata for UI and behavior:

```yaml
interface:
  display_name: "Draft PR"
  short_description: "Create PR descriptions from staged changes"
  icon_small: "./assets/small-logo.svg"
  icon_large: "./assets/large-logo.png"
  brand_color: "#3B82F6"
  default_prompt: "Draft a PR for the current changes"

policy:
  allow_implicit_invocation: true   # default: true

dependencies:
  tools:
    - type: "mcp"
      value: "github-mcp"
      description: "GitHub API access"
      transport: "streamable_http"
      url: "https://mcp.github.com"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interface.display_name` | string | - | User-facing name |
| `interface.short_description` | string | - | Brief description |
| `interface.icon_small` | path | - | Small icon (SVG/PNG) |
| `interface.icon_large` | path | - | Large icon |
| `interface.brand_color` | hex | - | Brand color |
| `interface.default_prompt` | string | - | Surrounding prompt text |
| `policy.allow_implicit_invocation` | bool | `true` | Auto-trigger on matching prompts |
| `dependencies.tools[]` | object | - | Required tool declarations |

### Skill Locations (scan order)

| Scope | Path | Use Case |
|-------|------|----------|
| Repo (CWD) | `.agents/skills/` | Folder-specific workflows |
| Repo (parent dirs) | `.agents/skills/` | Inherited repo skills |
| Repo (root) | `.agents/skills/` | Organization-wide repo skills |
| User | `~/.agents/skills/` | Personal cross-repo skills |
| Admin | `/etc/codex/skills/` | System-wide defaults |
| System | (bundled) | Built-in skills (e.g., skill-creator) |

Symlinked folders are followed.

### Activation

- **Explicit**: `/skills` slash command or `$skill-name` mention syntax
- **Implicit**: Codex auto-matches task prompts to skill descriptions
  (disable with `allow_implicit_invocation: false` in openai.yaml)

### Config in `config.toml`

```toml
# Disable a specific skill
[[skills.config]]
path = "/path/to/skill/SKILL.md"
enabled = false
```

### Built-in Skills

| Name | Purpose |
|------|---------|
| `$skill-creator` | Create new skills interactively |
| `$skill-installer` | Install community skills |

---

## 5. Custom Prompts (Deprecated)

`[Official]` https://developers.openai.com/codex/custom-prompts

**Deprecated in favor of Skills.** Documented here for completeness.

Custom prompts were Markdown files that became slash commands.

### File Location

`~/.codex/prompts/<name>.md` -- top-level files only, subdirectories ignored.

### Format

```yaml
---
description: Draft a pull request from staged changes
argument-hint: FILES=<paths> PR_TITLE="<title>"
---

Review the following files: $FILES

Create a PR with title: $PR_TITLE and a description summarizing the changes.
```

**Front matter fields:**

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Shown in slash command menu |
| `argument-hint` | string | Documents expected parameters |

**Placeholder system:**

| Placeholder | Description |
|-------------|-------------|
| `$1` - `$9` | Positional arguments (space-separated) |
| `$ARGUMENTS` | All arguments combined |
| `$NAME` | Named placeholder, supplied as `NAME=value` |
| `$$` | Literal dollar sign |

### Invocation

```
/prompts:draftpr FILES="src/ lib/" PR_TITLE="Add caching layer"
```

---

## 6. Profiles

`[Official]` https://developers.openai.com/codex/config-advanced

Profiles are named configuration sets defined in `config.toml`. Not a separate
content type per se, but a way to bundle model, sandbox, and behavior settings.

### Format

```toml
# Set default profile
profile = "deep-review"

[profiles.deep-review]
model = "gpt-5-pro"
model_reasoning_effort = "high"
approval_policy = "never"

[profiles.fast]
model = "gpt-5.3-codex-spark"
model_reasoning_effort = "low"
service_tier = "fast"
```

### Available Profile Fields

Profiles support a subset of top-level config keys:

| Field | Description |
|-------|-------------|
| `model` | Model override |
| `model_reasoning_effort` | Reasoning level |
| `model_instructions_file` | Custom instructions path |
| `model_catalog_json` | Custom model catalog path |
| `personality` | `none\|friendly\|pragmatic` |
| `plan_mode_reasoning_effort` | Plan-mode reasoning override |
| `service_tier` | `flex\|fast` |
| `web_search` | `disabled\|cached\|live` |
| `oss_provider` | `lmstudio\|ollama` |
| `tools_view_image` | Enable image viewing |
| `analytics.enabled` | Toggle analytics |
| `windows.sandbox` | `unelevated\|elevated` |

---

## 7. Custom Themes

`[Official]` https://developers.openai.com/codex/cli/features

Syntax highlighting themes for the TUI.

### Format & Location

- **Location**: `$CODEX_HOME/themes/` (default: `~/.codex/themes/`)
- **Format**: `.tmTheme` (TextMate theme format)
- **Selection**: Via `/theme` slash command in the TUI

---

## 8. Model Providers

`[Official]` https://developers.openai.com/codex/config-reference

Custom LLM provider definitions in `config.toml`.

### Format

```toml
[model_providers.azure]
name = "Azure OpenAI"
base_url = "https://my-resource.openai.azure.com/openai"
env_key = "AZURE_OPENAI_API_KEY"
wire_api = "responses"
query_params = { "api-version" = "2025-04-01-preview" }

[model_providers.ollama]
name = "Ollama"
base_url = "http://localhost:11434/v1"
env_key = "OLLAMA_API_KEY"
```

### Provider Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name |
| `base_url` | string | API endpoint |
| `env_key` | string | Env var for API key |
| `env_key_instructions` | string | Setup guidance for the user |
| `wire_api` | enum | `responses` |
| `http_headers` | map | Static request headers |
| `env_http_headers` | map | Headers from env vars |
| `query_params` | map | Extra URL query parameters |
| `request_max_retries` | number | Default: 4 |
| `stream_idle_timeout_ms` | number | Default: 300000 |
| `stream_max_retries` | number | Default: 5 |
| `supports_websockets` | bool | WebSocket support |
| `requires_openai_auth` | bool | Require OpenAI auth flow |
| `experimental_bearer_token` | string | Direct bearer token |

---

## 9. Permissions Profiles

`[Official]` https://developers.openai.com/codex/config-reference

Named filesystem and network permission sets.

### Format

```toml
default_permissions = "restricted"

[permissions.restricted.filesystem]
"/tmp" = "write"
":project_roots" = { "src" = "write", "docs" = "read" }

[permissions.restricted.network]
enabled = true
mode = "limited"
allowed_domains = ["api.github.com", "registry.npmjs.org"]
```

### Network Permission Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | - | Enable network access |
| `mode` | enum | - | `limited\|full` |
| `allowed_domains` | string[] | - | Domain allowlist |
| `denied_domains` | string[] | - | Domain denylist |
| `allow_local_binding` | bool | - | Allow localhost binding |
| `proxy_url` | string | - | HTTP proxy endpoint |
| `socks_url` | string | - | SOCKS5 endpoint |

---

## Summary: Content Types for Syllago

| Content Type | Format | Shareable/Portable | Primary Location |
|-------------|--------|-------------------|-----------------|
| **Instructions** | Markdown (AGENTS.md) | Yes (repo) | Project root + subdirs |
| **MCP Servers** | TOML (config.toml) | Partially (project .codex/) | `~/.codex/config.toml` |
| **Custom Agents** | TOML (standalone files) | Yes (project .codex/agents/) | `~/.codex/agents/` |
| **Skills** | SKILL.md + directory | Yes (repo .agents/skills/) | `.agents/skills/` |
| **Custom Prompts** | Markdown (deprecated) | No (local only) | `~/.codex/prompts/` |
| **Profiles** | TOML (config.toml) | No (embedded in config) | `~/.codex/config.toml` |
| **Themes** | .tmTheme | Yes (file copy) | `~/.codex/themes/` |
| **Model Providers** | TOML (config.toml) | No (embedded in config) | `~/.codex/config.toml` |
| **Permissions** | TOML (config.toml) | No (embedded in config) | `~/.codex/config.toml` |

---

## Sources

- [Custom instructions with AGENTS.md](https://developers.openai.com/codex/guides/agents-md) `[Official]`
- [Model Context Protocol](https://developers.openai.com/codex/mcp) `[Official]`
- [Subagents](https://developers.openai.com/codex/subagents) `[Official]`
- [Agent Skills](https://developers.openai.com/codex/skills) `[Official]`
- [Custom Prompts (Deprecated)](https://developers.openai.com/codex/custom-prompts) `[Official]`
- [Configuration Reference](https://developers.openai.com/codex/config-reference) `[Official]`
- [Advanced Configuration](https://developers.openai.com/codex/config-advanced) `[Official]`
- [Sample Configuration](https://developers.openai.com/codex/config-sample) `[Official]`
- [CLI Features](https://developers.openai.com/codex/cli/features) `[Official]`
- [Config Basics](https://developers.openai.com/codex/config-basic) `[Official]`
