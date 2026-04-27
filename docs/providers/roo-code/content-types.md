# Roo Code Content Types

Roo Code (fork of Cline) supports several content types that can be configured
globally or per-project. This document covers all known content types, their
formats, and storage locations.

---

## 1. Rules (Custom Instructions)

Rules provide persistent instructions appended to the system prompt. They control
coding style, conventions, and behavioral guidelines.

### File Locations

**Project-level (workspace root):**

| Path | Scope |
|------|-------|
| `.roo/rules/` | All modes (directory, preferred) |
| `.roo/rules-{mode-slug}/` | Specific mode only (e.g., `.roo/rules-code/`) |
| `.roorules` | All modes (single file, fallback) |
| `.roorules-{mode-slug}` | Specific mode only (single file, fallback) |
| `.clinerules` | Legacy Cline compatibility (lowest priority) |
| `.clinerules-{mode-slug}` | Legacy mode-specific compatibility |

**Global (user home):**

| Path | Scope |
|------|-------|
| `~/.roo/rules/` | All modes, all projects |
| `~/.roo/rules-{mode-slug}/` | Specific mode, all projects |

### Format

- Plain text or Markdown files (`.md`, `.txt`, or any text file)
- No required frontmatter or structure
- Files in directories are read **recursively** and concatenated in
  **alphabetical order** (case-insensitive by basename)
- Symbolic links supported (5-level resolution depth max, cycle detection)

### Exclusions

System/cache files are automatically excluded: `.DS_Store`, `*.bak`, `*.cache`,
`*.log`, `*.tmp`, `Thumbs.db`.

### Loading Precedence

1. Language preference (if set in VS Code)
2. Global custom instructions (from Prompts Tab UI)
3. Mode-specific rules (`.roo/rules-{slug}/` then `.roorules-{slug}`)
4. Workspace-wide rules (`.roo/rules/` then `.roorules`)
5. Legacy files (`.clinerules*`) only if no directory-based content exists

Directory-based rules override single-file fallbacks within each level. Project
rules override global rules. Mode-specific rules appear before general rules in
the system prompt.

### Example Structure

```
project/
├── .roo/
│   ├── rules/
│   │   ├── 01-general.md
│   │   └── 02-coding-style.md
│   └── rules-code/
│       ├── 01-typescript.md
│       └── 02-testing.md
└── .roorules           # fallback if .roo/rules/ is empty/missing
```

[Official] Source: [Custom Instructions docs](https://docs.roocode.com/features/custom-instructions)

---

## 2. AGENTS.md

Cross-agent instruction file following an emerging standard. Loaded automatically
alongside Roo Code's own rules.

### File Locations

- `{workspace}/AGENTS.md` (primary)
- `{workspace}/AGENT.md` (fallback)
- Subdirectory `AGENTS.md` files loaded when Context setting enabled

### Format

Plain Markdown. No required frontmatter.

### Loading Behavior

- Loaded after mode-specific rules, before generic workspace rules
- Controlled by VS Code setting: `"roo-cline.useAgentRules": true` (default)
- Empty/whitespace-only files are ignored

[Official] Source: [Custom Instructions docs](https://docs.roocode.com/features/custom-instructions)

---

## 3. Custom Modes

Modes define operational personalities with specific tool access, instructions,
and model preferences. Roo Code ships five built-in modes; users can create
custom modes or override built-in ones.

### Built-in Modes

| Mode | Slug | Tool Access |
|------|------|-------------|
| Code | `code` | read, edit, command, mcp, browser |
| Debug | `debug` | read, edit, command, mcp, browser |
| Ask | `ask` | read only |
| Architect | `architect` | read, command (limited) |
| Orchestrator | `orchestrator` | read, edit, command, mcp |

### Configuration Files

**Global:**
- `{vscode-global-storage}/rooveterinaryinc.roo-cline/settings/custom_modes.yaml` (preferred)
- `{vscode-global-storage}/rooveterinaryinc.roo-cline/settings/custom_modes.json` (fallback)

**Project-level:**
- `{workspace}/.roomodes` (YAML or JSON)

### Format (YAML preferred)

```yaml
customModes:
  - slug: reviewer
    name: "Code Reviewer"
    description: "Reviews code for quality and standards"
    roleDefinition: |
      You are a senior code reviewer focused on maintainability,
      correctness, and adherence to team standards.
    whenToUse: "When reviewing pull requests or code changes"
    customInstructions: "Always check for error handling and test coverage"
    groups:
      - read
      - - edit
        - fileRegex: "\\.(md|txt)$"
          description: "Documentation files only"
```

### Mode Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `slug` | string | yes | Unique ID (letters, numbers, hyphens) |
| `name` | string | yes | Display name in UI |
| `description` | string | no | Brief summary in mode selector |
| `roleDefinition` | string | yes | Detailed role/expertise/personality |
| `groups` | array | yes | Tool access permissions |
| `whenToUse` | string | no | Guidance for automated mode selection |
| `customInstructions` | string | no | Additional behavioral guidelines |

### Tool Groups Format

- **Unrestricted**: `"read"`, `"edit"`, `"command"`, `"mcp"`, `"browser"`
- **Restricted** (edit group only): two-element array with file regex filter

### Precedence

Project `.roomodes` completely overrides global modes with the same slug (no
property merging). Built-in modes can be overridden by creating a custom mode
with the same slug.

### Import/Export

Modes can be exported to portable YAML files including associated rules, then
imported into other projects.

[Official] Source: [Custom Modes docs](https://docs.roocode.com/features/custom-modes)

---

## 4. MCP Server Configuration

Configures Model Context Protocol servers that extend Roo Code with external tools
and resources.

### File Locations

**Global:**
- `{vscode-global-storage}/rooveterinaryinc.roo-cline/settings/cline_mcp_settings.json`
- Accessed via "Edit Global MCP" button in Roo Code UI

**Project-level:**
- `{workspace}/.roo/mcp.json`
- Can be committed to version control for team sharing

### Format

```json
{
  "mcpServers": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "@some/mcp-package@latest"],
      "env": {
        "API_KEY": "your-key"
      },
      "disabled": false,
      "alwaysAllow": ["tool-name-1", "tool-name-2"]
    }
  }
}
```

### Server Types

**Stdio (command-based):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | yes | Executable to run |
| `args` | array | no | Command-line arguments |
| `env` | object | no | Environment variables |
| `disabled` | boolean | no | Enable/disable server |
| `alwaysAllow` | array | no | Tools to auto-approve |

**SSE (network-based):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | Must be `"sse"` |
| `url` | string | yes | SSE endpoint URL |
| `disabled` | boolean | no | Enable/disable server |

**Streamable HTTP:** Also supported via `StreamableHTTPClientTransport`.

### Precedence

Project-level `.roo/mcp.json` overrides global settings for servers with the same
name.

[Official] Source: [Using MCP in Roo Code](https://docs.roocode.com/features/mcp/using-mcp-in-roo)

---

## 5. Skills

Instruction packages that activate on-demand when a request matches the skill's
purpose. Unlike rules (always loaded), skills remain dormant until needed.

### File Locations

**Global:**
- `~/.roo/skills/{skill-name}/SKILL.md` (Roo-specific, priority)
- `~/.agents/skills/{skill-name}/SKILL.md` (cross-agent)

**Project-level:**
- `{workspace}/.roo/skills/{skill-name}/SKILL.md` (Roo-specific)
- `{workspace}/.agents/skills/{skill-name}/SKILL.md` (cross-agent)

**Mode-specific:**
- `.roo/skills-{mode-slug}/{skill-name}/SKILL.md`

### Format (SKILL.md)

```markdown
---
name: my-skill
description: What this skill does and when to use it
---

# Skill Instructions

Detailed instructions that Roo follows when this skill is activated.
Reference bundled files with relative paths.
```

### Frontmatter Properties

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | 1-64 chars, lowercase letters/numbers/hyphens |
| `description` | string | yes | 1-1024 chars, specific enough for matching |

### Loading Levels (Progressive Disclosure)

1. **Discovery**: Frontmatter metadata read at startup
2. **Instructions**: Full SKILL.md loaded via `read_file` when request matches
3. **Resources**: Bundled scripts/templates loaded on-demand as referenced

### Override Priority

Project skills override global skills. `.roo/` paths take precedence over
`.agents/`. Mode-specific skills override generic ones.

### Built-in Skills

- `create-mcp-server` — Guide for creating MCP servers
- `create-mode` — Guide for creating custom modes
- `find-skills` — Discovers and installs agent skills

[Official] Source: [Skills docs](https://docs.roocode.com/features/skills)

---

## 6. Custom Tools (Experimental)

User-defined tools as TypeScript/JavaScript files that Roo calls like built-in
tools.

### File Locations

- `{workspace}/.roo/tools/` — project-specific (shared via version control)
- `~/.roo/tools/` — personal/global tools

Tools from later directories override earlier ones with identical names.

### Format

TypeScript or JavaScript files exporting a `defineCustomTool()` object:

```typescript
import { parametersSchema as z, defineCustomTool } from "@roo-code/types"

export default defineCustomTool({
  name: "tool_name",
  description: "What the tool does",
  parameters: z.object({
    param1: z.string().describe("Parameter description"),
  }),
  async execute(args, context) {
    // Tool logic
    return "Result string shown to AI"
  }
})
```

### Properties

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tool identifier |
| `description` | string | yes | Explains functionality to AI |
| `parameters` | Zod schema | yes | Input validation (converts to JSON Schema) |
| `execute` | async function | yes | Implementation, must return string |

### Key Notes

- npm dependencies supported (install in tool directory)
- `.env` files alongside tools are copied to cache directories
- Auto-approval when enabled (no permission prompts)
- Results must be strings; cannot prompt users interactively

[Official] Source: [Custom Tools docs](https://docs.roocode.com/features/experimental/custom-tools)

---

## 7. .rooignore

Controls which files Roo Code can see and access, similar to `.gitignore`.

### File Location

- `{workspace}/.rooignore`

### Format

Gitignore-style patterns. Respected during file listing, search, and indexing.
Works alongside `.gitignore` patterns.

[Official] Source: [Roo Code GitHub](https://github.com/RooCodeInc/Roo-Code)

---

## Summary Table

| Content Type | Location | Format | Scope |
|-------------|----------|--------|-------|
| Rules | `.roo/rules/`, `.roorules` | Markdown/text | System prompt instructions |
| AGENTS.md | Workspace root | Markdown | Cross-agent instructions |
| Custom Modes | `.roomodes`, global settings | YAML/JSON | Mode definitions |
| MCP Config | `.roo/mcp.json`, global settings | JSON | Server connections |
| Skills | `.roo/skills/` | SKILL.md (frontmatter + markdown) | On-demand instructions |
| Custom Tools | `.roo/tools/` | TypeScript/JavaScript | User-defined tools |
| .rooignore | Workspace root | Gitignore patterns | File access control |

---

## Sources

- [Custom Instructions](https://docs.roocode.com/features/custom-instructions) [Official]
- [Custom Modes](https://docs.roocode.com/features/custom-modes) [Official]
- [Using MCP in Roo Code](https://docs.roocode.com/features/mcp/using-mcp-in-roo) [Official]
- [Skills](https://docs.roocode.com/features/skills) [Official]
- [Custom Tools](https://docs.roocode.com/features/experimental/custom-tools) [Official]
- [RooCodeInc/Roo-Code GitHub](https://github.com/RooCodeInc/Roo-Code) [Official]
- [Custom Instructions and Rules (DeepWiki)](https://deepwiki.com/RooCodeInc/Roo-Code/9.4-custom-instructions-and-rules) [Community]
