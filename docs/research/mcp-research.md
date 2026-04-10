# MCP: Cross-Provider Deep Dive

> Research compiled 2026-03-22. Covers MCP configuration across 13 AI coding tools.
> Output feeds into: converter/compat.go unified compat scorer (syllago-ohdb)

---

## Table of Contents

1. [Overview](#overview)
2. [Per-Provider Deep Dive](#per-provider-deep-dive)
   - [Claude Code](#claude-code)
   - [Cursor](#cursor)
   - [Gemini CLI](#gemini-cli)
   - [Copilot CLI (GitHub Copilot)](#copilot-cli-github-copilot)
   - [VS Code Copilot](#vs-code-copilot)
   - [Windsurf](#windsurf)
   - [Kiro](#kiro)
   - [Codex CLI](#codex-cli)
   - [Cline](#cline)
   - [OpenCode](#opencode)
   - [Roo Code](#roo-code)
   - [Zed](#zed)
   - [Amp](#amp)
3. [Cross-Platform Normalization Problem](#cross-platform-normalization-problem)
4. [Canonical Mapping](#canonical-mapping)
5. [Feature/Capability Matrix](#featurecapability-matrix)
6. [Compat Scoring Implications](#compat-scoring-implications)
7. [Converter Coverage Audit](#converter-coverage-audit)

---

## Overview

MCP (Model Context Protocol) allows AI coding tools to connect to external tool servers via a standardized protocol. Despite the protocol itself being standard, each provider has adopted MCP with varying config formats, field names, transport support, and scoping rules. This document catalogs the differences to inform syllago's cross-provider MCP conversion layer.

### Summary Table

| Provider | MCP Support | Top-Level Key | Config Format | Transports | Scoping |
|----------|------------|---------------|---------------|------------|---------|
| Claude Code | Yes | `mcpServers` | JSON | stdio, sse, http | Project + Local + User |
| Cursor | Yes | `mcpServers` | JSON | stdio, sse, streamable-http | Project + Global |
| Gemini CLI | Yes | `mcpServers` | JSON | stdio, sse/http (via httpUrl) | Project + User |
| Copilot CLI | Yes | `mcpServers` | JSON | stdio, sse, http | Global + Workspace |
| VS Code Copilot | Yes | `servers` | JSON | stdio, http, sse | Workspace + User |
| Windsurf | Yes | `mcpServers` | JSON | stdio, sse, streamable-http | Global only |
| Kiro | Yes | `mcpServers` | JSON | stdio, http | Project + Global |
| Codex CLI | Yes | `[mcp_servers.*]` | TOML | stdio, streamable-http | Project + Global |
| Cline | Yes | `mcpServers` | JSON | stdio, sse | Global only |
| OpenCode | Yes | `mcp` | JSONC | local (stdio), remote (sse) | Project + Global |
| Roo Code | Yes | `mcpServers` | JSON | stdio, sse, streamable-http | Project + Global |
| Zed | Yes | `context_servers` | JSON | stdio, http (via mcp-remote) | Project + Global |
| Amp | Yes | `amp.mcpServers` | JSON | stdio, http | Project + Global |

---

## Per-Provider Deep Dive

### Claude Code

> Sources: [Claude Code MCP Docs](https://code.claude.com/docs/en/mcp)

**MCP Support:** Yes (full)

**Config Locations:**
- Project (shared): `.mcp.json` in project root (version-controlled)
- Local (private): `~/.claude.json` under project path
- User (cross-project): `~/.claude.json` global section
- Managed (enterprise): `managed-mcp.json` in system directories

**Precedence:** Local > Project > User > Managed

**Top-Level Key:** `mcpServers`

**Format:** JSON (with `${VAR}` and `${VAR:-default}` env var expansion)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `cwd` | string | Working directory |
| `type` | string | `"stdio"` \| `"sse"` \| `"http"` |
| `url` | string | HTTP/SSE transport URL |
| `headers` | object | HTTP headers |
| `autoApprove` | string[] | Tool names to auto-approve |
| `oauth` | object | OAuth config (includes `authServerMetadataUrl`) |

**Transport Types:** stdio, sse (deprecated), http (recommended for remote)

**Key Details:**
- Claude Code uses `"http"` as the type value for streamable HTTP, not `"streamable-http"`. This is a notable divergence from Cursor and Roo Code.
- SSE is explicitly deprecated in favor of HTTP.
- Environment variable expansion supports `${VAR:-default}` fallback syntax.
- Plugin MCP servers can be bundled via `.mcp.json` at plugin root or inline in `plugin.json`.
- `MCP_TIMEOUT` env var controls server startup timeout.
- `MAX_MCP_OUTPUT_TOKENS` controls output size limit (default 10,000).

**Converter audit note:** The existing converter uses `"streamable-http"` as a type value. Claude Code actually uses `"http"` for this transport. Need to verify whether Claude Code accepts `"streamable-http"` as an alias or if this is a bug.

---

### Cursor

> Sources: [Cursor MCP Docs](https://cursor.com/docs/context/mcp), [Cursor MCP CLI](https://cursor.com/docs/cli/mcp)

**MCP Support:** Yes (full)

**Config Locations:**
- Project: `.cursor/mcp.json`
- Global: `~/.cursor/mcp.json`
- Project wins when same server defined in both.

**Top-Level Key:** `mcpServers`

**Format:** JSON (supports `${env:VAR_NAME}` env var syntax)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `url` | string | HTTP transport URL |
| `headers` | object | HTTP headers |

**Transport Types:** stdio, sse, streamable-http

**Key Details:**
- Env var syntax is `${env:VAR_NAME}` (different from Claude Code's `${VAR}`).
- 40-tool limit across all MCP servers combined. Exceeding this silently drops tools.
- Cursor does NOT document `type`, `cwd`, `autoApprove`, or `disabled` fields in its official docs.
- Deep-link install buttons supported for one-click server installation.

**Converter audit note:** The existing converter emits `autoApprove` for Cursor, but Cursor's docs don't mention this field. It may be silently ignored. The converter also emits `cwd` and `type`, which are undocumented. Need to verify actual support.

---

### Gemini CLI

> Sources: [Gemini CLI MCP Docs](https://geminicli.com/docs/tools/mcp-server/), [GitHub source](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md)

**MCP Support:** Yes (full)

**Config Locations:**
- Project: `.gemini/settings.json`
- User: `~/.gemini/settings.json`

**Top-Level Key:** `mcpServers` (per-server configs) + `mcp` (global MCP settings)

**Format:** JSON

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `cwd` | string | Working directory |
| `httpUrl` | string | HTTP/SSE transport URL (NOT `url`) |
| `headers` | object | HTTP headers |
| `trust` | boolean | Bypass all tool confirmations when true |
| `includeTools` | string[] | Allowlist of tools (intersection with extension overrides) |
| `excludeTools` | string[] | Denylist of tools (union with extension overrides; takes precedence over includeTools) |
| `timeout` | number | Request timeout in ms (default: 600,000 = 10 min) |

**Global MCP Settings** (under `mcp` key, not `mcpServers`):
- `mcp.serverCommand` — global command to start an MCP server
- `mcp.allowed` — allowlist of server names
- `mcp.excluded` — denylist of server names

**Transport Types:** stdio, sse/http (via `httpUrl`)

**Key Details:**
- `trust` is a boolean, not a string. The existing canonical format stores it as a string -- this is a type mismatch.
- `httpUrl` is used instead of `url` for ALL HTTP transports. Gemini does not use `url`.
- `excludeTools` always takes precedence over `includeTools` when both are set.
- Extension overrides: inclusions intersected, exclusions unioned.
- OAuth 2.0 supported for remote servers.
- Sensitive env vars automatically redacted from base environment (tokens, secrets, keys, etc.).
- `timeout` field exists on Gemini -- the existing canonical format has this but only maps it for OpenCode.

**Converter audit note:** The `trust` type mismatch (bool in Gemini, string in canonical) should be fixed. Gemini's `timeout` field is not being canonicalized from Gemini source.

---

### Copilot CLI (GitHub Copilot)

> Sources: [GitHub Docs - Adding MCP servers](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers)

**MCP Support:** Yes (full)

**Config Locations:**
- Global: `~/.copilot/mcp-config.json`
- Workspace: `.vscode/mcp.json` or `.copilot/mcp-config.json` in project root
- Built-in GitHub MCP server pre-installed (read-only by default)

**Top-Level Key:** `mcpServers`

**Format:** JSON

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `type` | string | `"http"` \| `"sse"` (for remote) |
| `url` | string | HTTP/SSE transport URL |
| `headers` | object | HTTP headers |

**Transport Types:** stdio, sse (deprecated), http (streamable HTTP)

**Key Details:**
- 128-tool limit per request across all servers.
- Built-in GitHub MCP server; use `--enable-all-github-mcp-tools` for write access.
- OAuth NOT currently supported for remote servers (manual tokens only).
- Uses `"http"` type value (same as Claude Code, not `"streamable-http"`).
- Servers can be disabled/enabled via `/mcp disable` and `/mcp enable` commands.

**Converter audit note:** The existing converter emits `type` for Copilot CLI which is correct. However, if the type value is `"streamable-http"` from canonical, Copilot CLI expects `"http"`. The converter should map this.

---

### VS Code Copilot

> Sources: [VS Code MCP Docs](https://code.visualstudio.com/docs/copilot/customization/mcp-servers), [MCP Config Reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration)

**MCP Support:** Yes (full, distinct from Copilot CLI)

**Config Locations:**
- Workspace: `.vscode/mcp.json`
- User: `mcp.json` in user profile folder (via `MCP: Open User Configuration` command)

**Top-Level Key:** `servers` (NOT `mcpServers`!) + optional `inputs`

**Format:** JSON

**Per-Server Fields (stdio):**
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"stdio"` |
| `command` | string | Executable command |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `envFile` | string | Path to env file |
| `sandboxEnabled` | boolean | Enable sandboxing (macOS/Linux) |
| `sandbox` | object | Filesystem and network access rules |
| `dev` | object | Development mode (watch, debug) |

**Per-Server Fields (http/sse):**
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"http"` \| `"sse"` |
| `url` | string | Server endpoint URL |
| `headers` | object | HTTP headers |

**Inputs Format:**
```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "unique-id",
      "description": "Prompt text",
      "password": true
    }
  ],
  "servers": {
    "my-server": {
      "type": "stdio",
      "command": "npx",
      "args": ["server"],
      "env": { "KEY": "${input:unique-id}" }
    }
  }
}
```

**Transport Types:** stdio, http, sse

**Key Details:**
- Uses `servers` NOT `mcpServers` as the top-level key -- completely different from all other providers.
- Has an `inputs` system for securely prompting users for sensitive values.
- `envFile` field for loading env vars from a file -- unique to VS Code.
- Sandboxing support with granular filesystem and network access rules.
- Development mode with file watching and debugger integration.
- Settings like `chat.mcp.access`, `chat.mcp.discovery.enabled`, `chat.mcp.autostart`.
- Supports Unix socket/named pipe connections.
- Uses `${workspaceFolder}` variable expansion.

**Converter audit note:** VS Code Copilot is NOT covered by the existing converter. The `servers` key (not `mcpServers`), `inputs` system, `envFile`, `sandboxEnabled`, and `sandbox` fields are all unique. This is the most structurally different format in the ecosystem. Adding converter support would require a new canonicalize/render pair.

---

### Windsurf

> Sources: [Windsurf MCP Docs](https://docs.windsurf.com/windsurf/cascade/mcp)

**MCP Support:** Yes (full)

**Config Locations:**
- Global only: `~/.codeium/windsurf/mcp_config.json`
  - macOS: `~/.codeium/windsurf/mcp_config.json`
  - Windows: `%USERPROFILE%\.codeium\windsurf\mcp_config.json`
- No documented project-level config support.

**Top-Level Key:** `mcpServers`

**Format:** JSON (supports `${env:VARIABLE_NAME}` env var syntax)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `serverUrl` | string | Streamable HTTP URL (NOT `url`) |
| `url` | string | SSE URL |
| `headers` | object | HTTP headers |
| `disabledTools` | string[] | Tools to disable per server |

**Transport Types:** stdio, sse (via `url`), streamable-http (via `serverUrl`)

**Key Details:**
- 100-tool limit across all servers.
- Uses `serverUrl` for streamable-http and `url` for SSE -- a unique split not seen in other providers.
- Env var interpolation uses `${env:VARIABLE_NAME}` syntax (same as Cursor, different from Claude Code).
- `disabledTools` field for per-tool disabling (similar to Kiro).
- OAuth supported for all transport types.
- Enterprise whitelisting: once any server is whitelisted, all non-whitelisted are blocked.

**Converter audit note:** The existing converter handles `serverUrl` mapping correctly. However, `disabledTools` is NOT in the Windsurf canonicalize path or the canonical format -- this field would be silently lost on import from Windsurf. Windsurf also lacks `cwd` and `autoApprove` support.

---

### Kiro

> Sources: [Kiro MCP Configuration](https://kiro.dev/docs/mcp/configuration/), [Kiro CLI MCP](https://kiro.dev/docs/cli/mcp/configuration/)

**MCP Support:** Yes (full)

**Config Locations:**
- Workspace: `.kiro/settings/mcp.json`
- User: `~/.kiro/settings/mcp.json`
- Agent: Agent configuration files
- Also: `.kiro/mcp.json` (workspace-level)
- Merged with workspace taking precedence.

**Top-Level Key:** `mcpServers`

**Format:** JSON (supports `${VARIABLE}` env var expansion)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (local) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `url` | string | Remote server URL |
| `headers` | object | HTTP headers |
| `disabled` | boolean | Disable without removal |
| `autoApprove` | string[] | Tool auto-approval (supports `"*"` for all) |
| `disabledTools` | string[] | Tools to permanently disable |

**Transport Types:** stdio (local), http (remote via `url`)

**Key Details:**
- `autoApprove` supports wildcard `"*"` for approving all tools -- unique feature.
- `disabledTools` blocks specific dangerous operations (e.g., `delete_repository`, `force_push`).
- UI-based tool enable/disable in Kiro panel.
- Enterprise governance: admins can disable MCP entirely or restrict to vetted server list.
- Changes apply automatically on save (no restart needed).

**Converter audit note:** The existing renderer emits `disabledTools` for Kiro, which is correct. However, the `excludeTools` from Gemini canonical is mapped separately. Should `disabledTools` be in the canonical format? Currently it's only in the Kiro-specific render struct, but Windsurf also has it. Consider adding to canonical.

---

### Codex CLI

> Sources: [Codex MCP Docs](https://developers.openai.com/codex/mcp), [Config Reference](https://developers.openai.com/codex/config-reference), [Sample Config](https://developers.openai.com/codex/config-sample)

**MCP Support:** Yes (full)

**Config Locations:**
- Global: `~/.codex/config.toml`
- Project: `.codex/config.toml` (trusted projects only)
- Shared between CLI and IDE extension.

**Top-Level Key:** `[mcp_servers.<name>]` (TOML table syntax)

**Format:** TOML (NOT JSON -- unique in the ecosystem)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Server executable (stdio) |
| `args` | string[] | Command arguments |
| `env` | table | Environment variables |
| `env_vars` | string[] | Env vars to forward from host |
| `cwd` | string | Working directory |
| `url` | string | Streamable HTTP URL |
| `bearer_token_env_var` | string | Env var name for Bearer token |
| `http_headers` | table | Static HTTP headers |
| `env_http_headers` | table | Headers sourced from env vars |
| `enabled` | boolean | Enable/disable toggle (default true) |
| `required` | boolean | Fail if server doesn't start |
| `enabled_tools` | string[] | Allowlist of tool names |
| `disabled_tools` | string[] | Denylist (applied after allowlist) |
| `startup_timeout_sec` | float | Startup timeout (default 10s) |
| `tool_timeout_sec` | float | Tool execution timeout (default 60s) |
| `scopes` | string[] | OAuth scopes |

**OAuth (top-level):**
- `mcp_oauth_callback_port` — fixed OAuth callback port
- `mcp_oauth_callback_url` — custom callback URL

**Transport Types:** stdio, streamable-http

**Example:**
```toml
[mcp_servers.docs]
enabled = true
required = true
command = "docs-server"
args = ["--port", "4000"]
env = { "API_KEY" = "value" }
env_vars = ["ANOTHER_SECRET"]
cwd = "/path/to/server"
startup_timeout_sec = 10.0
tool_timeout_sec = 60.0
enabled_tools = ["search", "summarize"]
disabled_tools = ["slow-tool"]
scopes = ["read:docs"]

[mcp_servers.remote]
url = "https://mcp.example.com/mcp"
bearer_token_env_var = "REMOTE_TOKEN"
```

**Key Details:**
- Only provider using TOML format (all others use JSON/JSONC).
- `env_vars` is unique -- forwards named env vars from host without exposing values in config.
- `bearer_token_env_var` is unique -- references an env var name for the Bearer token rather than embedding it.
- `env_http_headers` is unique -- HTTP headers whose values come from env vars.
- `required` field is unique -- makes server startup mandatory.
- `scopes` for OAuth scope specification is unique.
- `enabled_tools` + `disabled_tools` dual filtering (allowlist then denylist).
- Server allowlisting with identity verification.
- Has SSE mentioned in docs as deprecated but may still work.

**Converter audit note:** Codex CLI is NOT covered by the existing converter. It requires TOML format (not JSON), has unique fields (`env_vars`, `bearer_token_env_var`, `env_http_headers`, `required`, `scopes`, `startup_timeout_sec`, `tool_timeout_sec`), and uses `enabled` polarity (like OpenCode). Adding support would require a TOML serializer and a new canonicalize/render pair. Many fields have no canonical equivalent.

---

### Cline

> Sources: [Cline MCP Docs](https://docs.cline.bot/mcp/configuring-mcp-servers)

**MCP Support:** Yes

**Config Locations:**
- Global only: `cline_mcp_settings.json` in VS Code extension storage
  - macOS: `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
  - Linux: `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
  - Windows: `%APPDATA%/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
- No project-level support (community-requested feature).

**Top-Level Key:** `mcpServers`

**Format:** JSON

**Per-Server Fields (stdio):**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `alwaysAllow` | string[] | Tool auto-approval (NOT `autoApprove`) |
| `disabled` | boolean | Disable toggle |

**Per-Server Fields (SSE):**
| Field | Type | Description |
|-------|------|-------------|
| `url` | string | SSE server URL |
| `headers` | object | HTTP headers |
| `alwaysAllow` | string[] | Tool auto-approval |
| `disabled` | boolean | Disable toggle |

**Transport Types:** stdio, sse

**Key Details:**
- Uses `alwaysAllow` instead of `autoApprove`.
- SSE transport IS supported (contradicting the existing converter which says "Cline only supports stdio"). The existing converter skips HTTP servers for Cline -- this is incorrect for SSE servers.
- No `cwd` support.
- Network timeout adjustable (30s to 1h, default 1 min).

**Converter audit note:** The existing converter incorrectly states "Cline only supports stdio, not HTTP" and skips servers with URLs. Cline actually supports SSE transport. The renderer should emit SSE servers with `url` and `headers` fields. The canonicalizer also doesn't handle Cline's SSE fields (url, headers) -- these would be lost on import.

---

### OpenCode

> Sources: [OpenCode MCP Docs](https://opencode.ai/docs/mcp-servers/), [OpenCode Config](https://opencode.ai/docs/config/)

**MCP Support:** Yes (full)

**Config Locations:**
- Project: `opencode.json` or `opencode.jsonc` in project root
- Global: `~/.config/opencode/config.json`

**Top-Level Key:** `mcp` (NOT `mcpServers`)

**Format:** JSONC (JSON with comments; has `$schema` support)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"local"` (stdio) \| `"remote"` (HTTP) |
| `command` | string[] | Command as array (NOT string + args) |
| `environment` | object | Env vars (NOT `env`) |
| `enabled` | boolean | Enable toggle (default true, NOT `disabled`) |
| `url` | string | Remote server URL |
| `headers` | object | HTTP headers |
| `timeout` | number | Timeout in ms |
| `oauth` | object | OAuth configuration |

**Transport Types:** local (stdio), remote (HTTP/SSE)

**Key Details:**
- Three naming divergences: `mcp` not `mcpServers`, `environment` not `env`, `command` as array not string+args.
- `enabled` with true default (opposite polarity of `disabled`).
- `type` uses `local`/`remote` instead of `stdio`/`sse`.
- Auto-loads config from project dir on `cd` -- security concern with untrusted repos.
- OAuth handled automatically for remote servers.
- `$schema` field support for IDE validation.

**Converter audit note:** The existing converter handles OpenCode correctly: command array split, environment mapping, enabled/disabled flip, local/remote type mapping. No gaps found.

---

### Roo Code

> Sources: [Roo Code MCP Docs](https://docs.roocode.com/features/mcp/using-mcp-in-roo), [Transport Docs](https://docs.roocode.com/features/mcp/server-transports)

**MCP Support:** Yes (full)

**Config Locations:**
- Global: `mcp_settings.json` in VS Code extension storage
- Project: `.roo/mcp.json` in project root (version-controllable)
- Project takes precedence when same server name exists in both.

**Top-Level Key:** `mcpServers`

**Format:** JSON (supports `${env:VARIABLE_NAME}` env var syntax)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `cwd` | string | Working directory |
| `type` | string | `"streamable-http"` \| `"sse"` (omit for stdio) |
| `url` | string | HTTP/SSE transport URL |
| `headers` | object | HTTP headers |
| `alwaysAllow` | string[] | Tool auto-approval (NOT `autoApprove`) |
| `disabled` | boolean | Disable toggle |
| `timeout` | number | Per-server timeout in seconds (1-3600, default 60) |
| `disabledTools` | string[] | Tools to hide |
| `watchPaths` | string[] | File paths to monitor for auto-restart |

**Transport Types:** stdio, sse (legacy), streamable-http (recommended)

**Key Details:**
- Uses `alwaysAllow` (like Cline).
- Supports `cwd` -- the existing converter drops this but Roo Code actually supports it.
- `disabledTools` for per-tool filtering.
- `watchPaths` for auto-restarting on file changes (unique feature).
- `timeout` in seconds (not ms like OpenCode).
- Project-level config via `.roo/mcp.json` -- enables team sharing.
- Community request for `alwaysAllowResources` to extend auto-approval to MCP resources.
- Type inference enhancement requested: auto-detect `streamable-http` when `url` present.

**Converter audit note:** The existing converter drops `cwd` for Roo Code with a warning, but Roo Code actually supports `cwd`. This should be preserved. Also, `disabledTools`, `timeout`, and `watchPaths` are not captured in the canonical format. The `headers` field is dropped with a warning but Roo Code supports it for HTTP servers.

---

### Zed

> Sources: [Zed MCP Docs](https://zed.dev/docs/ai/mcp), [Zed MCP Extensions](https://zed.dev/docs/extensions/mcp-extensions)

**MCP Support:** Yes

**Config Locations:**
- Project: `.zed/settings.json`
- Global: `~/.config/zed/settings.json`

**Top-Level Key:** `context_servers` (NOT `mcpServers`)

**Format:** JSON (embedded in Zed settings file)

**Per-Server Fields (command-based):**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |

**Per-Server Fields (URL-based):**
| Field | Type | Description |
|-------|------|-------------|
| `url` | string | Remote server URL (set to `"custom"` for custom servers) |
| `headers` | object | HTTP headers |

**Transport Types:** stdio natively; streaming HTTP via `mcp-remote` bridge

**Key Details:**
- `context_servers` is a completely different top-level key from all other providers.
- `source` field used in extension-installed servers (e.g., `"custom"` for manually added).
- URL-based servers use `"url": "custom"` with `headers` -- the URL value `"custom"` is a sentinel, not an actual URL. The actual URL may be handled differently.
- Native HTTP/SSE support is NOT yet shipped. The `mcp-remote` npm package is used as a local stdio bridge to streaming HTTP servers.
- Supports MCP Tools and Prompts specifications, plus `notifications/tools/list_changed`.
- Agent profiles can control which MCP tools are enabled per profile.

**Converter audit note:** The existing converter only handles stdio for Zed, which is correct for native support. However, Zed now has URL-based server config in settings.json. The canonicalizer should be updated to handle `url`/`headers` from Zed. The renderer should be updated to emit URL-based configs when the server has a URL. The `"url": "custom"` sentinel value needs investigation.

---

### Amp

> Sources: [Amp Manual](https://ampcode.com/manual), [Amp MCP Setup Guide](https://github.com/sourcegraph/amp-examples-and-guides/blob/main/guides/mcp/amp-mcp-setup-guide.md)

**MCP Support:** Yes (full)

**Config Locations:**
- Workspace: `.amp/settings.json` in project root (requires approval)
- Global: `~/.config/amp/settings.json` (macOS)
- Managed: `/Library/Application Support/ampcode/managed-settings.json` (enterprise)
- Via CLI: `--mcp-config` flag

**Top-Level Key:** `amp.mcpServers`

**Format:** JSON (supports `${VAR_NAME}` env var expansion)

**Per-Server Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Executable path (stdio) |
| `args` | string[] | Command arguments |
| `env` | object | Environment variables |
| `url` | string | Remote server URL |
| `headers` | object | HTTP headers |
| `includeTools` | string[] | Tool name glob patterns to filter |

**Transport Types:** stdio, http (remote)

**Key Details:**
- Uses `amp.mcpServers` as the top-level key (namespaced, unique across all providers).
- `includeTools` supports glob patterns (e.g., `"navigate_*"`, `"fill*"`) -- more powerful than Gemini's plain string lists.
- Skills can bundle MCP servers via `mcp.json` in skill directories for lazy-loading.
- Workspace servers require explicit approval before running.
- Global/CLI-configured servers do NOT require approval.
- Toolbox system as alternative to MCP for simple scripts.
- Permission system: blanket allow/reject or per-tool approval.
- Context window management: recommends selective tool inclusion to avoid bloat.
- Precedence hierarchy when same server name appears in multiple sources.

**Converter audit note:** Amp is NOT covered by the existing converter. Requires new canonicalize/render pair. The `amp.mcpServers` key is namespaced. `includeTools` with glob patterns overlaps with Gemini's `includeTools` (plain strings) -- canonical should support both. The core fields (command, args, env, url, headers) are standard.

---

## Cross-Platform Normalization Problem

The MCP ecosystem has significant fragmentation despite a common underlying protocol:

### Key Divergences

| Aspect | Variants | Providers |
|--------|----------|-----------|
| **Top-level key** | `mcpServers` | Claude, Cursor, Gemini, Copilot CLI, Windsurf, Kiro, Cline, Roo Code |
| | `servers` | VS Code Copilot |
| | `mcp` | OpenCode |
| | `context_servers` | Zed |
| | `amp.mcpServers` | Amp |
| | `[mcp_servers.*]` | Codex CLI (TOML) |
| **URL field** | `url` | Most providers |
| | `httpUrl` | Gemini CLI |
| | `serverUrl` | Windsurf (for streamable-http) |
| **Env vars key** | `env` | Most providers |
| | `environment` | OpenCode |
| **Command format** | string + args | Most providers |
| | string[] array | OpenCode |
| **Disable polarity** | `disabled: true` | Cline, Roo Code, Kiro |
| | `enabled: false` | OpenCode, Codex CLI |
| **Auto-approval** | `autoApprove` | Claude Code, Cursor, Kiro |
| | `alwaysAllow` | Cline, Roo Code |
| | Permissions system | Amp |
| **Transport naming** | `stdio`/`sse`/`http` | Claude Code, Copilot CLI, VS Code |
| | `stdio`/`sse`/`streamable-http` | Cursor, Roo Code, Windsurf |
| | `local`/`remote` | OpenCode |
| **Tool filtering** | `includeTools`/`excludeTools` | Gemini CLI |
| | `includeTools` (with globs) | Amp |
| | `disabledTools` | Kiro, Windsurf, Roo Code |
| | `enabled_tools`/`disabled_tools` | Codex CLI |
| **Config format** | JSON | Most providers |
| | JSONC | OpenCode |
| | TOML | Codex CLI |
| **Env var syntax** | `${VAR}` / `${VAR:-default}` | Claude Code |
| | `${env:VAR_NAME}` | Cursor, Windsurf, Roo Code |
| | `"$VAR"` (shell-style) | Gemini CLI |
| | `${VAR_NAME}` | Amp |

### Transport Type Value Mapping

The `type` field value for "streamable HTTP" transport is inconsistent:

| Provider | Type Value |
|----------|-----------|
| Claude Code | `"http"` |
| Copilot CLI | `"http"` |
| VS Code Copilot | `"http"` |
| Cursor | `"streamable-http"` |
| Roo Code | `"streamable-http"` |
| Windsurf | (implicit via `serverUrl`) |
| OpenCode | `"remote"` |

This means the canonical format needs to normalize `"http"`, `"streamable-http"`, and `"remote"` to a single canonical value, and the renderers need to emit the correct provider-specific value.

---

## Canonical Mapping

Syllago's canonical format (`mcpServerConfig`) maps provider-specific fields to a unified superset:

| Canonical Field | Claude | Cursor | Gemini | Copilot CLI | VS Code | OpenCode | Zed | Cline | Roo Code | Kiro | Windsurf | Codex | Amp |
|----------------|--------|--------|--------|-------------|---------|----------|-----|-------|----------|------|----------|-------|-----|
| `command` | command | command | command | command | command | cmd[0] | command | command | command | command | command | command | command |
| `args` | args | args | args | args | args | cmd[1:] | args | args | args | args | args | args | args |
| `env` | env | env | env | env | env | environ. | env | env | env | env | env | env | env |
| `cwd` | cwd | cwd | cwd | cwd | -- | -- | -- | -- | cwd | -- | -- | cwd | -- |
| `url` | url | url | httpUrl | url | url | url | url | url | url | url | svrUrl/url | url | url |
| `headers` | headers | headers | headers | headers | headers | headers | headers | headers | headers | headers | headers | http_hdrs | headers |
| `type` | type | type | -- | type | type | loc/rem | -- | -- | type | -- | -- | -- | -- |
| `disabled` | -- | -- | -- | -- | -- | !enabled | -- | disabled | disabled | disabled | -- | !enabled | -- |
| `autoApprove` | autoAppr | autoAppr | -- | -- | -- | -- | -- | alwAllow | alwAllow | autoAppr | -- | -- | -- |
| `trust` | -- | -- | trust | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |
| `includeTools` | -- | -- | inclTools | -- | -- | -- | -- | -- | -- | -- | -- | en_tools | inclTools |
| `excludeTools` | -- | -- | exclTools | -- | -- | -- | -- | -- | -- | -- | -- | dis_tools | -- |
| `disabledTools` | -- | -- | -- | -- | -- | -- | -- | -- | disTools | disTools | disTools | -- | -- |
| `timeout` | -- | -- | timeout | -- | -- | timeout | -- | -- | timeout | -- | -- | timeouts | -- |
| `oauth` | oauth | -- | -- | -- | -- | oauth | -- | -- | -- | -- | -- | scopes | -- |

**Legend:** `--` = not supported, abbreviated names used for space

---

## Feature/Capability Matrix

| Capability | Claude | Cursor | Gemini | Cop CLI | VS Code | OpenCode | Zed | Cline | Roo | Kiro | Windsurf | Codex | Amp |
|-----------|--------|--------|--------|---------|---------|----------|-----|-------|-----|------|----------|-------|-----|
| stdio | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| SSE | Y* | Y | Y | Y* | Y | Y | -- | Y | Y* | -- | Y | -- | -- |
| streamable-http | Y | Y | -- | Y | Y | -- | --+ | -- | Y | Y | Y | Y | Y |
| Auto-approve | Y | ? | -- | -- | -- | -- | -- | Y | Y | Y | -- | -- | -- |
| Tool allowlist | -- | -- | Y | -- | -- | -- | -- | -- | -- | -- | -- | Y | Y |
| Tool denylist | -- | -- | Y | -- | -- | -- | -- | -- | Y | Y | Y | Y | -- |
| OAuth | Y | -- | Y | -- | -- | Y | -- | -- | -- | -- | Y | Y | -- |
| Project scope | Y | Y | Y | Y | Y | Y | Y | -- | Y | Y | -- | Y | Y |
| Global scope | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| cwd | Y | Y | Y | Y | -- | -- | -- | -- | Y | -- | -- | Y | -- |
| Headers | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| Env var expansion | Y | Y | Y | -- | Y | -- | -- | -- | Y | Y | Y | -- | Y |
| Sandboxing | -- | -- | -- | -- | Y | -- | -- | -- | -- | -- | -- | -- | -- |

`Y*` = SSE deprecated in this provider; `--+` = via mcp-remote bridge only; `?` = undocumented

---

## Compat Scoring Implications

For the compat scorer, key dimensions to evaluate:

### Lossless Conversions (score 1.0)

These field mappings are semantically equivalent with no information loss:

- `autoApprove` <-> `alwaysAllow` (rename only)
- `disabled` <-> `enabled` (polarity flip)
- `command` string + `args` <-> `command` array (split/join)
- `env` <-> `environment` (rename only)
- `url` <-> `httpUrl` <-> `serverUrl` (rename + type inference)
- `stdio` <-> `local`, `sse` <-> `remote` (rename)

### Lossy Conversions (score penalty)

Fields that exist in source but are dropped on target:

| Field | Providers That Have It | Providers That Drop It |
|-------|----------------------|----------------------|
| `trust` | Gemini | All others |
| `includeTools` | Gemini, Amp, Codex | Most others |
| `excludeTools` | Gemini | Most others |
| `disabledTools` | Kiro, Windsurf, Roo Code | Claude, Cursor, Cline, OpenCode |
| `cwd` | Claude, Cursor, Gemini, Copilot, Roo, Codex | OpenCode, Zed, Cline, Kiro, Windsurf, Amp |
| `headers` | Most | Zed (stdio), Cline (stdio) |
| `oauth` | Claude, OpenCode, Codex | Most others |
| `autoApprove` | Claude, Cursor, Kiro, Cline, Roo | Gemini, OpenCode, Zed, Windsurf, Codex, Amp |
| `timeout` | OpenCode, Gemini, Roo, Codex | Claude, Cursor, Cline, Zed, Amp |

### Transport Incompatibility (score 0.0 for affected servers)

- Servers using HTTP transport cannot be converted to Zed (stdio only natively).
- Servers using streamable-http cannot be converted to Cline (sse or stdio only), OpenCode (no streamable-http), or Gemini (no streamable-http).

### Structural Incompatibility

These require non-trivial conversion beyond field mapping:

- **VS Code Copilot:** `servers` key, `inputs` system, `envFile`, `sandbox` -- fundamentally different structure.
- **Codex CLI:** TOML format, `env_vars` (forward by name), `bearer_token_env_var`, `env_http_headers` -- unique concepts with no JSON equivalent.
- **Zed:** `context_servers` key, `source` metadata field.

### Proposed Scoring Model

| Score | Meaning | Criteria |
|-------|---------|----------|
| 1.0 | Full compat | All fields have semantic equivalents |
| 0.9 | Near-full | Core fields + most extras preserved; 1-2 minor fields dropped |
| 0.7-0.8 | High | Core fields preserved; provider-specific extras lost |
| 0.5-0.6 | Medium | Some transport types skipped or significant fields lost |
| 0.3-0.4 | Low | Major structural differences; many fields lost |
| 0.0 | None | Transport incompatible or provider doesn't support MCP |

---

## Converter Coverage Audit

### Currently Covered Providers (10)

| Provider | Canonicalize | Render | Status |
|----------|-------------|--------|--------|
| Claude Code | Yes (default) | Yes | Functional but see issues below |
| Cursor | Yes (default) | Yes | Functional but emits undocumented fields |
| Gemini CLI | Yes (default, httpUrl) | Yes | Trust type mismatch |
| Copilot CLI | Yes (default) | Yes | Type value mapping issue |
| OpenCode | Yes (custom) | Yes | Correct |
| Zed | Yes (custom) | Yes | Missing URL-based server support |
| Cline | Yes (custom) | Yes | Incorrectly blocks SSE servers |
| Roo Code | Yes (custom) | Yes | Missing cwd, headers, timeout support |
| Kiro | No custom canonicalize | Yes | Correct render |
| Windsurf | Yes (custom) | Yes | Missing disabledTools on import |

### Providers NOT Covered (3)

| Provider | MCP Support | Effort | Priority |
|----------|------------|--------|----------|
| VS Code Copilot | Yes (full) | High -- structural differences (servers key, inputs, sandbox) | Medium -- large user base but complex format |
| Codex CLI | Yes (full) | High -- TOML format, unique fields | Medium -- growing user base |
| Amp | Yes (full) | Low -- standard fields plus includeTools | High -- simple to add |

### Issues Found

#### Critical

1. **Cline SSE support missing.** The renderer says "Cline only supports stdio, not HTTP" and skips URL-based servers. Research shows Cline supports SSE transport with `url` and `headers` fields. Affected code: `renderClineMCP()` in `mcp.go:564-568`. Fix: allow SSE servers through, emit `url` and `headers`.

2. **Cline canonicalize missing SSE fields.** The `clineMCPServerConfig` struct only has `command`, `args`, `env`, `alwaysAllow`, `disabled`. It's missing `url` and `headers` for SSE servers. Importing a Cline config with SSE servers would silently lose those servers.

#### High

3. **Transport type value mismatch.** Claude Code and Copilot CLI use `"http"` for streamable HTTP. The canonical format and most renderers use `"streamable-http"`. This means:
   - Importing from Claude Code: `"http"` type is preserved as-is (no mapping).
   - Rendering to Claude Code: `"streamable-http"` is emitted, which may not be recognized.
   - Fix: canonicalize should map `"http"` -> `"streamable-http"` on import; renderers for Claude Code and Copilot CLI should map `"streamable-http"` -> `"http"` on export.

4. **Gemini `trust` type mismatch.** Canonical stores `trust` as `string`, but Gemini uses `boolean`. The JSON marshal/unmarshal may handle this implicitly, but it's semantically incorrect. Fix: change canonical `Trust` to `bool` or handle both types.

5. **Roo Code field support gaps.** The renderer drops `cwd` and `headers` with warnings, but Roo Code actually supports both fields. Fix: emit `cwd` and `headers` in `renderRooCodeMCP()`.

#### Medium

6. **Missing `disabledTools` in canonical format.** Three providers support `disabledTools` (Kiro, Windsurf, Roo Code), but it's not in the canonical `mcpServerConfig` struct. The Kiro renderer has it in a separate struct, but Windsurf and Roo Code canonicalizers don't capture it. Fix: add `disabledTools` to canonical and handle in all relevant canonicalize/render paths.

7. **Missing `timeout` mapping for Gemini.** Gemini supports a `timeout` field but the default canonicalizer doesn't capture it. Only OpenCode's custom canonicalizer maps `timeout`. Fix: handle `timeout` in the default canonicalize path.

8. **Cursor `autoApprove` may not be supported.** Cursor's official docs don't document `autoApprove`. The renderer emits it, which may be silently ignored or cause errors. Needs verification.

9. **Windsurf `disabledTools` lost on import.** Windsurf's canonicalizer doesn't capture `disabledTools`. Fix: add to `windsurfServerConfig` and map in `canonicalizeWindsurfMCP()`.

10. **Zed URL-based servers not handled.** Zed now supports URL-based server configs (`url` + `headers`), but the canonicalizer and renderer only handle command-based (stdio) servers. Fix: update both paths.

#### Low

11. **Gemini `timeout` not rendered.** When rendering to Gemini, the timeout from canonical is not emitted. Add `timeout` to the Gemini renderer.

12. **Roo Code `timeout` not mapped.** Roo Code supports per-server `timeout` (in seconds), but neither canonicalize nor render handles it. Note the unit difference: OpenCode uses ms, Roo Code uses seconds.

13. **VS Code Copilot, Codex CLI, and Amp converters missing.** All three providers have MCP support but no converter coverage.

### Recommended Priority

1. Fix Cline SSE support (Critical -- data loss on import/export)
2. Fix transport type value mapping (High -- Claude Code/Copilot CLI interop broken)
3. Add `disabledTools` to canonical (Medium -- 3 providers affected)
4. Fix Roo Code cwd/headers rendering (High -- unnecessary field loss)
5. Fix Gemini trust type (High -- type safety)
6. Add Amp converter (Low effort, standard fields)
7. Add Zed URL support (Medium effort)
8. Add Codex CLI converter (High effort, TOML)
9. Add VS Code Copilot converter (High effort, structural differences)
