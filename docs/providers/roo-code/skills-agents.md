# Roo Code: Skills, Agents & Custom Modes

## Custom Modes (Roo Code's Agent/Persona System)

Custom modes are Roo Code's primary mechanism for creating specialized AI agents. Each mode defines a persona with specific expertise, tool permissions, and behavioral instructions. Modes are Roo Code's key differentiator from Cline (its upstream fork). [Official](https://docs.roocode.com/features/custom-modes)

### Built-in Modes

| Mode | Slug | Tool Groups | Purpose |
|------|------|------------|---------|
| Code | `code` | read, edit, command, mcp | Default. Writing code, implementing features, debugging |
| Ask | `ask` | read, mcp | Code explanation, concept exploration, technical learning |
| Architect | `architect` | read, mcp, edit (markdown only) | System design, high-level planning, architecture |
| Debug | `debug` | read, edit, command, mcp | Bug tracking, error diagnosis, issue resolution |
| Orchestrator | `orchestrator` | new_task only | Multi-step workflow coordination, delegates to other modes |

[Official](https://docs.roocode.com/basic-usage/using-modes)

### Mode Definition Format

Modes are defined in YAML (preferred) or JSON. Both `.roomodes` (project) and `custom_modes.yaml` (global) use the same schema.

#### All Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `slug` | string | Yes | Unique identifier. Pattern: `/^[a-zA-Z0-9-]+$/`. Used in file paths like `.roo/rules-{slug}/` |
| `name` | string | Yes | Display name shown in UI. Supports spaces and emojis |
| `roleDefinition` | string | Yes | Core identity and expertise. Placed at beginning of system prompt |
| `groups` | GroupEntry[] | Yes | Tool groups and file restrictions (see below) |
| `description` | string | No | Short summary shown below mode name in the mode selector UI |
| `whenToUse` | string | No | Guidance for automated mode selection by the Orchestrator. Not displayed in UI |
| `customInstructions` | string | No | Additional behavioral guidelines. Appended near end of system prompt |

[Official](https://docs.roocode.com/features/custom-modes)

#### YAML Example

```yaml
customModes:
  - slug: security-review
    name: "Security Reviewer"
    description: Reviews code for security vulnerabilities
    roleDefinition: >
      You are a security specialist who reviews code for vulnerabilities,
      injection risks, authentication flaws, and data exposure issues.
    whenToUse: Use when reviewing code for security concerns or conducting security audits.
    customInstructions: >
      Always check for OWASP Top 10 vulnerabilities. Flag any hardcoded
      secrets or credentials. Suggest fixes for every issue found.
    groups:
      - read
      - - edit
        - fileRegex: SECURITY\.md$
          description: Security documentation only
      - command
```

#### JSON Example

```json
{
  "customModes": [
    {
      "slug": "security-review",
      "name": "Security Reviewer",
      "description": "Reviews code for security vulnerabilities",
      "roleDefinition": "You are a security specialist...",
      "whenToUse": "Use when reviewing code for security concerns.",
      "customInstructions": "Always check for OWASP Top 10...",
      "groups": [
        "read",
        ["edit", { "fileRegex": "SECURITY\\.md$", "description": "Security docs only" }],
        "command"
      ]
    }
  ]
}
```

### Tool Groups & Permissions

Each mode specifies which tool groups are available. Groups can be granted unrestricted or with file restrictions.

#### Available Tool Groups

| Group | Tools Included |
|-------|---------------|
| `read` | `read_file`, `list_files`, `read_command_output`, `search_files`, `codebase_search` |
| `edit` | `apply_diff`, `apply_patch`, `edit`, `edit_file`, `search_replace`, `write_to_file` |
| `browser` | `browser_action` (requires vision-capable model) |
| `command` | `execute_command`, `run_slash_command` (experimental) |
| `mcp` | `use_mcp_tool`, `access_mcp_resource` |

[Official](https://docs.roocode.com/advanced-usage/available-tools/tool-use-overview)

Additional workflow tools are always available regardless of mode: `ask_followup_question`, `attempt_completion`, `switch_mode`, `new_task`, `update_todo_list`, `skill`.

#### File Restrictions

The `edit` group supports regex-based file restrictions. When restricted, the mode can read any file but only edit files matching the pattern.

```yaml
# Unrestricted edit access
groups:
  - edit

# Restricted to TypeScript files only
groups:
  - - edit
    - fileRegex: \.(ts|tsx)$
      description: TypeScript files only
```

- YAML uses single backslashes in regex: `\.(ts|tsx)$`
- JSON requires double-escaped backslashes: `\\.(ts|tsx)$`
- Patterns match the full relative path from workspace root
- If a tool attempts to write outside allowed patterns, a `FileRestrictionError` is thrown

[Official](https://docs.roocode.com/features/custom-modes)

### Model Selection Per Mode (Sticky Models)

Roo Code automatically remembers the last model used with each mode. When you switch modes, the model switches too. This allows assigning different models to different tasks without manual reconfiguration:

- Architect mode can use a high-reasoning model (e.g., Claude Opus)
- Code mode can use a faster model (e.g., Claude Sonnet)
- Model preferences persist between sessions

This is automatic behavior, not a configuration field — there is no `model` field in the mode definition. [Official](https://docs.roocode.com/features/custom-modes)

### Storage Locations

| Scope | Mode Config | Mode-Specific Rules |
|-------|------------|-------------------|
| Global | `{vscode-global-storage}/settings/custom_modes.yaml` | `~/.roo/rules-{slug}/` |
| Project | `.roomodes` (workspace root) | `.roo/rules-{slug}/` |

**Override behavior:** Project modes completely override global modes with the same slug — all fields, not a merge. [Official](https://docs.roocode.com/features/custom-modes)

**Platform paths for global storage:**
- Linux: `~/.config/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/`
- macOS: `~/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/`
- Windows: `%APPDATA%\Code\User\globalStorage\rooveterinaryinc.roo-cline\settings\`

[Community](https://github.com/Michaelzag/RooCode-Tips-Tricks/blob/main/personal_roo_docs/technical/custom-modes.md)

### Mode Switching

Four methods to switch modes:
1. **Dropdown menu** in the Roo Code panel
2. **Slash commands**: `/code`, `/architect`, `/ask`, `/debug`
3. **Keyboard shortcut**: Ctrl/Cmd + .
4. **Agent-initiated**: Via `switch_mode` tool or Orchestrator's `new_task`

[Official](https://docs.roocode.com/basic-usage/using-modes)

---

## Skills

Skills are on-demand instruction packages that activate when a user's request matches the skill's purpose. Unlike custom instructions (always active) or modes (user-selected), skills use progressive disclosure to stay dormant until needed. [Official](https://docs.roocode.com/features/skills)

### How Skills Work (Three-Level Progressive Disclosure)

1. **Discovery**: System reads only SKILL.md frontmatter (name + description) — no full content loaded
2. **Instructions**: When a request matches, the full SKILL.md is loaded into context
3. **Resources**: Instructions reference bundled files (scripts, templates) accessed on-demand

This keeps the base prompt focused — large skill libraries don't cause prompt bloat.

### Skill Format

Each skill is a directory containing a `SKILL.md` file with mandatory frontmatter:

```markdown
---
name: my-skill-name
description: What this skill does and when to use it
---

## Instructions

Detailed instructions for Roo when this skill is activated.

## Resources

Reference bundled files like `./templates/component.tsx` that Roo
can read on-demand.
```

**Naming constraints:** 1-64 characters, lowercase alphanumeric and hyphens only, no leading/trailing/consecutive hyphens.

### Storage Locations (Priority Order)

| Priority | Path | Scope |
|----------|------|-------|
| 1 (highest) | `.roo/skills-{mode}/{skill-name}/SKILL.md` | Project, mode-specific |
| 2 | `.roo/skills/{skill-name}/SKILL.md` | Project, all modes |
| 3 | `.agents/skills-{mode}/{skill-name}/SKILL.md` | Project, cross-agent, mode-specific |
| 4 | `.agents/skills/{skill-name}/SKILL.md` | Project, cross-agent, all modes |
| 5 | `~/.roo/skills-{mode}/{skill-name}/SKILL.md` | Global, mode-specific |
| 6 | `~/.roo/skills/{skill-name}/SKILL.md` | Global, all modes |
| 7 | `~/.agents/skills-{mode}/{skill-name}/SKILL.md` | Global, cross-agent, mode-specific |
| 8 (lowest) | `~/.agents/skills/{skill-name}/SKILL.md` | Global, cross-agent, all modes |

Project skills override global; `.roo/` overrides `.agents/`; mode-specific overrides generic.

### Skills vs Other Content Types

| Aspect | Skills | Custom Instructions | Custom Modes | Custom Tools |
|--------|--------|-------------------|-------------|-------------|
| Activation | On-demand (request matching) | Always active | User-selected or orchestrator-selected | Agent-invoked |
| Format | SKILL.md with frontmatter | Text rules | YAML/JSON config | TypeScript/JavaScript |
| Bundled files | Yes | No | No | N/A (they ARE executable) |
| Mode targeting | Yes (skills-{mode}/) | Yes | N/A | No |
| Cross-agent | Yes (.agents/) | No | No | No |

---

## Differences from Cline

Roo Code forked from Cline and diverged significantly. The mode and skill systems are the primary differentiators. [Inferred from multiple sources]

| Feature | Roo Code | Cline |
|---------|----------|-------|
| **Agent model** | Multi-mode (specialized personas) | Single-agent (Plan/Act) |
| **Custom modes** | Yes — full mode definition system with tool groups, file restrictions, role definitions | No — single agent with consistent behavior |
| **Orchestrator** | Yes — delegates subtasks to specialized modes | No |
| **Skills** | Yes — on-demand instruction packages | No |
| **Tool restrictions** | Per-mode with regex file patterns | Global |
| **Model per task** | Sticky models per mode | Single model selection |
| **MCP support** | Yes | Yes (with marketplace ecosystem) |
| **Custom tools** | `.roo/tools/` TypeScript/JS files | No |
| **Cross-agent dirs** | `.agents/` convention | No |

[Community](https://www.qodo.ai/blog/roo-code-vs-cline/)
[Community](https://betterstack.com/community/comparisons/cline-vs-roo-code-vs-cursor/)
[Community](https://serenitiesai.com/articles/roo-code-vs-cline-ai-coding-2026)

### Key Architectural Difference

Cline uses a linear Plan/Act workflow where one agent handles everything. Roo Code structures work through specialized modes — each with its own system prompt, tool access, and model preference. The Orchestrator mode can coordinate multi-step workflows by spawning subtasks in specific modes, effectively creating a multi-agent system within a single extension.
