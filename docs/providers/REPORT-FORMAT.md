# Provider Research Report Format

Each provider gets four research files in `docs/providers/<provider-slug>/`.

## Report Metadata

**Every report file MUST start with a metadata block.** This is machine-parseable and used by the provider-audit skill to track staleness.

```markdown
<!-- provider-audit-meta
provider: zed
provider_version: "0.178.x"
report_format: 1
researched: 2026-03-21
researcher: claude-opus-4.6
changelog_checked: https://zed.dev/releases
-->
```

| Field | Required | Description |
|-------|----------|-------------|
| `provider` | Yes | Provider slug |
| `provider_version` | Yes | Version researched against (release tag, "latest" if rolling, or "N/A" if no versioning) |
| `report_format` | Yes | Report format version (currently `1`) |
| `researched` | Yes | Date research was conducted (YYYY-MM-DD) |
| `researcher` | Yes | Who/what did the research (model name, human name) |
| `changelog_checked` | Yes | URL of the changelog or release page that was verified |

**Why this matters:** Without metadata, there's no way to know if a report is current. A report from March 2026 researched against Cursor 2.3 is stale when Cursor 2.5 ships new tools — but nothing tells you that unless the provider version is tracked.

## Changelog Sources

Every provider has an official changelog or release page. The audit skill MUST check these during both audit and diff workflows.

| Provider | Changelog URL |
|----------|--------------|
| Claude Code | https://github.com/anthropics/claude-code/releases |
| Gemini CLI | https://github.com/google-gemini/gemini-cli/releases |
| Cursor | https://cursor.com/changelog |
| Windsurf | https://docs.windsurf.com/changelog |
| Codex | https://github.com/openai/codex/releases |
| Copilot CLI | https://github.blog/changelog/ (filter: Copilot) |
| Cline | https://github.com/cline/cline/releases |
| Roo Code | https://github.com/RooVetGit/Roo-Code/releases |
| OpenCode | https://github.com/opencode-ai/opencode/releases |
| Kiro | https://kiro.dev/changelog |
| Zed | https://zed.dev/releases |

## The Four Report Files

### 1. `tools.md` — Built-in Tools
For each tool the provider exposes:
- **Name**: Exact tool name as used in config/allowed-tools
- **Purpose**: What it does (file read, file write, shell, search, etc.)
- **Parameters**: Key parameters/options if documented
- **Notes**: Any quirks, aliases, or cross-provider equivalents
- **Provider-unique tools**: Tools with no equivalent in other providers

### 2. `content-types.md` — Content Type Configurations
For each content type the provider supports (rules, skills, agents, hooks, MCP, commands, prompts):
- **File format**: Extension, encoding (md, mdc, json, yaml, toml, jsonc)
- **Directory structure**: Where files live on disk, naming conventions
- **Config schema**: All frontmatter fields / JSON keys with types and descriptions
- **Required vs optional fields**: Which fields are mandatory
- **Examples**: Representative config snippets

### 3. `hooks.md` — Hook Events & Lifecycle
- **Supported events**: All lifecycle event names
- **Hook configuration**: How hooks are defined (JSON, YAML, inline)
- **Matcher syntax**: How to filter which hooks fire
- **Hook types**: Shell, command, webhook, etc.
- **Execution model**: Sync/async, timeout, error handling
- **Config file location**: Where hook config lives

### 4. `skills-agents.md` — Skill & Agent Metadata
- **Skill definition format**: All frontmatter/config fields
- **Agent definition format**: All config fields
- **Model selection**: How to specify models
- **Tool permissions**: allowed-tools, disallowed-tools syntax
- **Invocation patterns**: How skills/agents are triggered
- **Provider-specific fields**: Fields unique to this provider

## Source Attribution

Every claim must include one of:
- `[Official]` — from provider's official docs or repo
- `[Community]` — from blog posts, tutorials, community forums
- `[Inferred]` — derived from source code or config examples
- `[Unverified]` — stated but not confirmed from primary source

Include URLs for all sources. Prefer linking to specific pages, not just the docs root.

## Staleness Detection

The provider-audit skill uses the metadata block to detect stale reports:
- **Date check**: Reports older than 30 days should be re-audited
- **Version check**: If the provider has released a new version since `provider_version`, the report is stale
- **Changelog check**: The `changelog_checked` URL is fetched to look for releases newer than the `researched` date
