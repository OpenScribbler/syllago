# Community Repository Content Organization Patterns

**Date:** 2026-03-31
**Purpose:** Empirical research into how ~50 real-world GitHub repositories organize AI coding tool content (agents, skills, hooks, rules, commands, MCP configs). Used to inform syllago's content analyzer/detector design.

---

## Methodology

Searched GitHub for popular repositories sharing AI coding tool content across Claude Code, Cursor, Windsurf, Copilot, Cline, Roo Code, Codex, Gemini, and multi-provider tools. Examined directory structures, file formats, naming conventions, metadata approaches, and content organization patterns.

---

## Claude Code Content Repos

### 1. anthropics/claude-code (Official)
- **Stars:** ~90,700
- **URL:** https://github.com/anthropics/claude-code
- **Structure:** Plugin system with `.claude-plugin/plugin.json` manifests
- **Agents:** `plugins/<name>/agents/<agent>.md` (flat .md files)
- **Skills:** `plugins/<name>/skills/<skill-name>/SKILL.md` (directory with SKILL.md)
- **Hooks:** `plugins/<name>/hooks/hooks.json` + handler scripts (.py, .sh)
- **Commands:** `plugins/<name>/commands/<cmd>.md` (flat .md files)
- **Other:** `examples/hooks/`, `examples/settings/` with sample settings.json files
- **13 official plugins:** agent-sdk-dev, claude-opus-4-5-migration, code-review, commit-commands, explanatory-output-style, feature-dev, frontend-design, hookify, learning-output-style, plugin-dev, pr-review-toolkit, ralph-wiggum, security-guidance
- **No .syllago.yaml or registry.yaml**

### 2. affaan-m/everything-claude-code
- **Stars:** ~126,500
- **URL:** https://github.com/affaan-m/everything-claude-code
- **Structure:** Multi-provider with `.claude/`, `.cursor/`, `.codex/`, `.gemini/`, `.opencode/`, `.agents/`, `.codebuddy/`
- **Agents:** `.codex/agents/*.toml`; `.agents/skills/*/agents/openai.yaml`
- **Skills:** `.claude/skills/<name>/SKILL.md`, `.cursor/skills/<name>/SKILL.md`, `.agents/skills/<name>/SKILL.md`
- **Hooks:** `.cursor/hooks/*.js` (JS handlers), `.cursor/hooks.json` (config)
- **Commands:** `.claude/commands/<cmd>.md`, `.opencode/commands/<cmd>.md`
- **Rules:** `.claude/rules/<name>.md` (flat .md), `.cursor/rules/<lang>-<topic>.md`
- **Other:** `.claude/homunculus/instincts/`, `.claude/identity.json`, `.claude-plugin/`, `.codex-plugin/`

### 3. rohitg00/awesome-claude-code-toolkit
- **Stars:** ~978
- **URL:** https://github.com/rohitg00/awesome-claude-code-toolkit
- **Structure:** Top-level directories (NOT inside `.claude/`)
- **Agents:** `agents/<category>/<agent>.md` (flat .md, categorized subdirs: business-product, core-development, data-ai, developer-experience, infrastructure, language-experts, quality-security, specialized-domains)
- **Skills:** `skills/<skill-name>/SKILL.md` (directory with SKILL.md, 35 skills)
- **Commands:** `commands/<cmd>.md` (flat .md, 42 commands)
- **Rules:** `rules/<topic>.md` (flat .md, 15 rules)
- **Other:** `templates/claude-md/*.md`, `setup/install.sh`, `.claude-plugin/` with marketplace.json

### 4. alirezarezvani/claude-skills
- **Stars:** ~8,389
- **URL:** https://github.com/alirezarezvani/claude-skills
- **Structure:** Multi-provider with `.claude/`, `.codex/`, `.gemini/`, plus domain-organized top-level dirs
- **Skills:** Organized by business domain as top-level dirs: `engineering-team/<skill>/SKILL.md`, `business-growth/<skill>/SKILL.md`, etc. Mirrored into `.codex/skills/` and `.gemini/skills/`
- **Commands:** `.claude/commands/<cmd>.md`, nested `commands/git/<cmd>.md`
- **Other:** `.claude-plugin/marketplace.json`, `.codex-plugin/plugin.json`, `CLAUDE.md`, `GEMINI.md`

### 5. ChrisWiles/claude-code-showcase
- **Stars:** ~5,651
- **URL:** https://github.com/ChrisWiles/claude-code-showcase
- **Structure:** Everything inside `.claude/` directory
- **Agents:** `.claude/agents/<agent>.md` (flat .md: code-reviewer.md, github-workflow.md)
- **Skills:** `.claude/skills/<name>/SKILL.md` (directory-based)
- **Hooks:** `.claude/hooks/skill-eval.js`, `.claude/hooks/skill-eval.sh`, `.claude/hooks/skill-rules.json`
- **Commands:** `.claude/commands/<cmd>.md` (flat .md)
- **MCP:** `.mcp.json` at root
- **Other:** `.claude/settings.json`, `CLAUDE.md`

### 6. disler/claude-code-hooks-mastery
- **Stars:** ~3,447
- **URL:** https://github.com/disler/claude-code-hooks-mastery
- **Structure:** Everything inside `.claude/`
- **Agents:** `.claude/agents/<agent>.md` and `.claude/agents/<category>/<agent>.md` (mixed flat + categorized)
- **Hooks:** `.claude/hooks/*.py` (Python with uv): notification.py, permission_request.py, post_tool_use.py, pre_compact.py, pre_tool_use.py, session_end.py, session_start.py, stop.py, subagent_start.py, subagent_stop.py, user_prompt_submit.py; plus `hooks/utils/` and `hooks/validators/`
- **Commands:** `.claude/commands/<cmd>.md`
- **Other:** `.claude/settings.json`, `.claude/output-styles/<style>.md`

### 7. feiskyer/claude-code-settings
- **Stars:** ~1,392
- **URL:** https://github.com/feiskyer/claude-code-settings
- **Structure:** Mix of top-level + plugin system
- **Agents:** `agents/<agent>.md` (flat .md at root)
- **Skills:** `skills/<name>/SKILL.md` (directory-based with helpers/, scripts/, references/); also in `plugins/<plugin>/skills/<name>/SKILL.md`
- **Hooks:** `hooks/hooks.json` (config file at root)
- **Commands:** Inside plugins: `plugins/<name>/commands/<cmd>.md`
- **MCP:** `.mcp.json` at root, plus `config.json`
- **Other:** `plugins/` with 6 plugins, `guidances/` for non-Claude providers

### 8. carlrannaberg/claudekit
- **Stars:** ~643
- **URL:** https://github.com/carlrannaberg/claudekit
- **Structure:** Dual: `.claude/` for installed config + `src/` for source content
- **Agents:** `.claude/agents/<agent>.md` and `.claude/agents/<category>/` (flat + categorized)
- **Commands:** `.claude/commands/<cmd>.md` and `.claude/commands/<category>/`
- **Cross-provider:** `.cursorrules`, `.clinerules`, `.windsurfrules`, `.replit.md`, `.idx/airules.md`, `.github/copilot-instructions.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `AGENT.md`
- **Published as npm package with CLI**

### 9. FlorianBruniaux/claude-code-ultimate-guide
- **Stars:** ~2,641
- **URL:** https://github.com/FlorianBruniaux/claude-code-ultimate-guide
- **Structure:** Examples directory with full content collection
- **Agents:** `examples/agents/<agent>.md` (flat .md) + `examples/agents/<multi-agent-team>/` (directories with multiple agent .md files)
- **Skills:** `examples/skills/<name>/SKILL.md` (directory-based with references/, checklists/, scoring/)
- **Hooks:** `examples/hooks/bash/*.sh` (30+ bash scripts), `examples/hooks/powershell/*.ps1`
- **Commands:** `examples/commands/<cmd>.md` + nested `examples/commands/learn/*.md`
- **Rules:** `examples/rules/<topic>.md` (flat .md)
- **MCP:** `examples/mcp-configs/figma.json`

### 10. shanraisshan/claude-code-best-practice
- **Stars:** ~28,539
- **URL:** https://github.com/shanraisshan/claude-code-best-practice
- **Structure:** Everything inside `.claude/`, plus documentation
- **Agents:** `.claude/agents/<agent>.md` (flat) + `.claude/agents/workflows/<workflow>/<agent>.md` (nested)
- **Skills:** `.claude/skills/<name>/SKILL.md` and `.claude/skills/<category>/<name>/SKILL.md`
- **Hooks:** `.claude/hooks/scripts/hooks.py`, `.claude/hooks/config/hooks-config.json`, `.claude/hooks/sounds/`
- **Commands:** `.claude/commands/<cmd>.md` + `.claude/commands/workflows/<category>/<cmd>.md`
- **Rules:** `.claude/rules/<topic>.md`
- **MCP:** `.mcp.json` at root

### 11. karanb192/claude-code-hooks
- **Stars:** ~309
- **URL:** https://github.com/karanb192/claude-code-hooks
- **Structure:** Top-level `hook-scripts/` directory
- **Hooks:** `hook-scripts/<event-type>/<hook>.js` organized by event: `pre-tool-use/`, `post-tool-use/`, `notification/`; plus `utils/event-logger.py`
- **Tests:** `hook-scripts/tests/<event>/<hook>.test.js`

### 12. aemccormick/ai-tools (the repo that prompted this investigation)
- **Stars:** small/private
- **URL:** https://github.com/aemccormick/ai-tools
- **Agents:** `agents/*.md` (flat .md files with YAML frontmatter: name, description, model, color, tools)
- **Skills:** `skills/<name>/SKILL.md` with `references/` and optional `Workflows/` subdirs (~25 skills)
- **Hooks:** `hooks/*.ts` (TypeScript, Bun runtime) — wired via `.claude/settings.json`
- **MCP:** `.mcp.json` at root
- **Other:** `bin/` with helper shell scripts, `CLAUDE.md`, `SETUP.md`

### 13. disler/claude-code-hooks-multi-agent-observability
- **Stars:** ~1,300
- **URL:** https://github.com/disler/claude-code-hooks-multi-agent-observability
- **Hooks:** `.claude/hooks/*.py` (12 Python scripts, one per event)
- **Agents:** `.claude/agents/team/builder.md`, `.claude/agents/team/validator.md`
- **Commands:** `.claude/commands/`
- **Other:** Universal `send_event.py` dispatcher, Bun observability server

### 14. zircote/.claude
- **Stars:** ~20
- **URL:** https://github.com/zircote/.claude
- **Structure:** Top-level directories (meant to be cloned as ~/.claude/)
- **Agents:** `agents/<##-category>/<agent>.md` (numbered category dirs: 01-core-development, 02-language-specialists, etc.)
- **Skills:** `skills/<name>/SKILL.md` with references/ and assets/
- **Hooks:** `hooks/prompt_capture_hook.py`
- **Commands:** `commands/<cmd>.md` + `commands/git/<cmd>.md`

### 15. shakacode/claude-code-commands-skills-agents
- **Stars:** ~21
- **URL:** https://github.com/shakacode/claude-code-commands-skills-agents
- **Agents:** `agents/<agent>.md` (flat .md)
- **Skills:** `skills/<name>/SKILL.md` with `references/`
- **Commands:** `commands/<cmd>.md` (flat .md)
- **Other:** `prompts/`, `bin/sync-commands`, `bin/sync-skills`, `AGENTS.md` at root

---

## Cursor Content Repos

### 16. PatrickJS/awesome-cursorrules
- **Stars:** ~38,808 (largest Cursor rules collection)
- **URL:** https://github.com/PatrickJS/awesome-cursorrules
- **Rules (legacy):** `rules/<descriptive-name>-cursorrules-prompt-file/.cursorrules` (directory per rule set, 170+)
- **Rules (new):** `rules-new/<technology>.mdc` (flat file per technology)
- **MDC format:** YAML frontmatter with `description` and `globs` fields, then markdown content
- **Categorization:** 12 categories in README

### 17. sanjeed5/awesome-cursor-rules-mdc
- **Stars:** ~3,424
- **URL:** https://github.com/sanjeed5/awesome-cursor-rules-mdc
- **Rules:** `rules-mdc/<technology>.mdc` (243 flat .mdc files, auto-generated from Exa + Gemini)
- **MDC format:** YAML frontmatter with `description` and `globs`
- **Naming:** Lowercase kebab-case matching library name

### 18. ivangrynenko/cursorrules
- **Stars:** ~79
- **URL:** https://github.com/ivangrynenko/cursorrules
- **Rules:** `.cursor/rules/<descriptive-name>.mdc` (60+ .mdc files)
- **Commands:** `.cursor/commands/<command-name>.md` (19 command files)
- **Cross-provider:** `AGENTS.md`, `CLAUDE.md` at root
- **Naming:** Kebab-case with language/concern prefix

### 19. sparesparrow/cursor-rules
- **Stars:** ~65
- **URL:** https://github.com/sparesparrow/cursor-rules
- **Rules:** `.cursor/rules/` with nested subdirectories: `domain/` (20 numbered rules), `mcp/client/`, `mcp/server/`
- **Naming:** Numbered prefix (`01-base-agentic.rules.mdc`)

### 20. JuroOravec/agents
- **Stars:** ~0 (architecturally interesting)
- **URL:** https://github.com/JuroOravec/agents
- **Full Cursor structure:**
  - `.cursor/agents/<role>.md` (4 agent files)
  - `.cursor/hooks.json` + `.cursor/hooks/`
  - `.cursor/rules/<name>.md` (3 rule files)
  - `.cursor/skills/<category>/<skill>/SKILL.md` (act/, meta/, role/, root/)
- **Uses git submodules for multi-project sharing**

### 21. blefnk/awesome-cursor-rules
- **Stars:** ~79
- **URL:** https://github.com/blefnk/awesome-cursor-rules
- **Rules:** Root-level numbered `.md` files: `000-cursor-rules.md` through `2008-polar-payments.md`
- **Numeric ordering:** 000-004 = core, 100 = tools, 300 = testing, 1000 = languages, 2000 = frameworks
- **Multi-provider:** Cursor, Windsurf, VS Code Copilot

### 22. digitalchild/cursor-best-practices
- **Stars:** ~125
- **URL:** https://github.com/digitalchild/cursor-best-practices
- **Rules:** Technology subdirectories at root: `Claude/`, `Playwright/`, `Python/`, `WordPress/` — each containing `.mdc` or `.cursorrules.*` files
- **Mixed formats:** `.mdc` and legacy `.cursorrules.*.md`

---

## Windsurf / Copilot / Multi-Provider Repos

### 23. github/awesome-copilot (Official)
- **Stars:** ~27,900
- **URL:** https://github.com/github/awesome-copilot
- **Structure:** Most sophisticated content taxonomy
  - `agents/*.md` (flat .md agent definitions)
  - `instructions/*.instructions.md` (rule/instruction files)
  - `skills/*/SKILL.md` (directory-based skills)
  - `plugins/` (bundled collections)
  - `hooks/` (lifecycle hooks)
  - `workflows/` (GitHub Actions workflows)
  - `.schemas/` (JSON validation schemas)
- **Plugin install system:** `copilot plugin install <name>@awesome-copilot`

### 24. intellectronica/ruler
- **Stars:** ~2,600
- **URL:** https://github.com/intellectronica/ruler
- **Target:** 30+ tools (broadest provider support)
- **Structure:** `.ruler/` directory with markdown rules + `ruler.toml` config
- **Mechanism:** Reads `.ruler/*.md`, concatenates, writes to each tool's native config file
- **Outputs:** AGENTS.md, CLAUDE.md, .cursorrules, .aider.conf.yml, etc.

### 25. Bhartendu-Kumar/rules_template
- **Stars:** ~1,100
- **URL:** https://github.com/Bhartendu-Kumar/rules_template
- **Target:** Cline, RooCode, Cursor, Windsurf
- **Structure:** `.cursor/rules/` as single source of truth
- **Cross-provider:** Symlinks from `.clinerules/` and `.roo/` to `.cursor/rules/`
- **Rules:** `.cursor/rules/*.mdc` (plan.mdc, implement.mdc, debug.mdc, memory.mdc)

### 26. botingw/rulebook-ai
- **Stars:** ~585
- **URL:** https://github.com/botingw/rulebook-ai
- **Target:** 10 tools (Cursor, Windsurf, Cline, RooCode, Copilot, Claude Code, Codex, Gemini, Warp, Kilo Code)
- **Structure:** Universal "packs" with `rules/`, `memory/`, `tools/` directories
- **Naming:** Zero-padded numeric prefixes for ordering (`10-`, `20-`)
- **Generates:** `.cursor/rules/`, `GEMINI.md`, `CLAUDE.md`, etc.

### 27. block/ai-rules
- **Stars:** ~82
- **URL:** https://github.com/block/ai-rules
- **Target:** 11 agents (AMP, Claude Code, Cline, Codex, Copilot, Cursor, etc.)
- **Rust CLI from Block (Square's parent)**
- **Structure:** `ai-rules/` directory with markdown files + `ai-rules-config.yaml`
- **CI integration:** `ai-rules status` checks sync, can block merges

### 28. lbb00/ai-rules-sync
- **Stars:** ~22
- **URL:** https://github.com/lbb00/ai-rules-sync
- **Target:** 10+ tools
- **Manifest:** `ai-rules-sync.json` per project, `ai-rules-sync.local.json` for local-only
- **File formats by tool:** `.mdc` for Cursor, `.instructions.md` for Copilot, `.md` for Claude, `.toml` for Gemini, `.rules` for Codex

### 29. instructa/ai-prompts
- **Stars:** ~1,025
- **URL:** https://github.com/instructa/ai-prompts
- **Structure:** `prompts/<name>/` (97 prompt directories)
- **Metadata:** Per-directory `aiprompt.json` with rich fields: name, description, type (rule/feature/starter), slug, tags, tech_stack, ai_editor, author, version, files, published
- **Master index:** `data/index.json` cataloging all prompts
- **Cross-provider via `ai_editor` field in metadata**

### 30. balqaasem/awesome-windsurfrules
- **Stars:** ~47
- **URL:** https://github.com/balqaasem/awesome-windsurfrules
- **Structure:** Fork of awesome-cursorrules adapted for Windsurf
- **Rules:** `rules/global_rules/` and `rules/.windsurfrules/`
- **Naming:** `[technology]-[focus]-windsurfrules-prompt-file/`

### 31. nedcodes-ok/cursorrules-collection
- **Stars:** ~23
- **URL:** https://github.com/nedcodes-ok/cursorrules-collection
- **Dual format:** `rules-mdc/` (modern .mdc) and `rules/` (legacy .cursorrules)
- **Categorization:** `languages/`, `frameworks/`, `practices/`, `tools/` (110+ rules)
- **MDC frontmatter:** `alwaysApply`, `globs`, `description`

### 32. grapeot/devin.cursorrules
- **Stars:** ~5,965
- **URL:** https://github.com/grapeot/devin.cursorrules
- **Single opinionated ruleset targeting Cursor + Copilot**
- **Cross-provider:** `.cursorrules` + `.github/copilot-instructions.md`

---

## Hook-Specific Collections

### 33. johnlindquist/claude-hooks
- **Stars:** ~339
- **URL:** https://github.com/johnlindquist/claude-hooks
- **TypeScript-based** with full type safety
- **Structure:** `.claude/hooks/` with `index.ts`, `lib.ts`, `session.ts`
- **Settings:** `settings.json` co-located in `.claude/`

### 34. decider/claude-hooks
- **Stars:** ~68
- **URL:** https://github.com/decider/claude-hooks
- **Dual structure:** `.claude/hooks/` for active, `hooks/` for implementations, `portable-hooks/` for portable versions
- **Dispatcher pattern:** `universal-*.py` files route to specific implementations
- **Hierarchical config:** `.claude/hooks.json` (root), `.claude-hooks.json` (overrides), `.claude/settings.local.json`

---

## MCP Config Repos

### 35. modelcontextprotocol/registry (Official)
- **Stars:** ~6,600
- **URL:** https://github.com/modelcontextprotocol/registry
- **Go service backed by PostgreSQL** — not a flat-file collection
- **Namespaced identifiers:** `io.github.username/server-name`

### 36. modelcontextprotocol/servers (Official)
- **Stars:** ~82,600
- **URL:** https://github.com/modelcontextprotocol/servers
- **Reference implementations** (not configs)
- **Has `.mcp.json` at root** (dogfooding)

### 37. abcdan/mcp.json
- **Stars:** ~6
- **URL:** https://github.com/abcdan/mcp.json
- **Single flat `.mcp.json`** with all MCP server configs in one `mcpServers` object
- **Represents the most common MCP sharing pattern**

---

## Multi-Content-Type Collections (Hooks + Skills + Commands + MCP)

### 38. davepoon/buildwithclaude
- **Stars:** ~2,700
- **URL:** https://github.com/davepoon/buildwithclaude
- **Largest collection:** 117 agents, 175 commands, 28 hooks, 26 skills, 50 plugin packages
- **Structure:** `plugins/` with prefix-based naming: `agents-*/`, `commands-*/`, `hooks-*/`
- **YAML frontmatter + markdown body** for all content
- **MCP:** `mcp-servers.json` indexes 4,500+ servers
- **Web UI:** `web-ui/` for browsing

### 39. serpro69/claude-toolbox
- **Stars:** ~78
- **URL:** https://github.com/serpro69/claude-toolbox
- **Plugin distribution** via Claude Code marketplace system
- **Structure:** `klaude-plugin/` with `manifest.json`, `skills/`, `commands/`, `hooks/`
- **Template sync** via GitHub Actions

### 40. Dev-GOM/claude-code-marketplace
- **Stars:** ~79
- **URL:** https://github.com/Dev-GOM/claude-code-marketplace
- **Per-plugin directories:** `plugins/hook-git-auto-backup/`, `plugins/hook-todo-collector/`, etc.
- **Each plugin:** `plugin.json` (hook definitions + metadata) + script files + `README.md`
- **Hook types in plugin.json:** SessionStart, PreToolUse, PostToolUse, Stop, etc.

---

## Curated Lists / Meta-Collections

### 41. hesreallyhim/awesome-claude-code
- **Stars:** ~35,100
- **URL:** https://github.com/hesreallyhim/awesome-claude-code
- **Meta-list** aggregating hooks, skills, commands, plugins, agents across 8+ categories
- **Has `THE_RESOURCES_TABLE.csv`** as structured data export

### 42. VoltAgent/awesome-agent-skills
- **Stars:** ~13,567
- **URL:** https://github.com/VoltAgent/awesome-agent-skills
- **Curated list** of skills repos

### 43. BehiSecc/awesome-claude-skills
- **Stars:** ~8,054
- **URL:** https://github.com/BehiSecc/awesome-claude-skills
- **Curated list** of skills repos

### 44. anthropics/skills (Official)
- **Stars:** ~108,000
- **URL:** https://github.com/anthropics/skills
- **Official skill repository.** `skills/` with self-contained directories, each with `SKILL.md` using YAML frontmatter
- **Has `spec/` (Agent Skills specification) and `template/`**

---

## Pattern Analysis

### Content Type Format Summary

| Content Type | Dominant Pattern | Variations |
|---|---|---|
| **Agents** | Flat `.md` files in `agents/` or `.claude/agents/` | Categorized subdirs, `.toml` for Codex, `.yaml` for OpenAI |
| **Skills** | `skills/<name>/SKILL.md` with optional `references/` | Nested by category, mirrored across provider dirs |
| **Hooks** | Script files (.py, .js, .ts, .sh) + settings/config JSON | Organized by event type; co-located or separate from wiring |
| **Commands** | Flat `.md` files in `commands/` | Nested subdirs for categories |
| **Rules** | Provider-specific file/path conventions | `.cursorrules`, `.mdc`, `.windsurfrules`, `.md` |
| **MCP** | `.mcp.json` at repo root | Rarely per-server directories |
| **Plugins** | `.claude-plugin/plugin.json` + content dirs | Growing adoption |

### Provider-Specific Content Locations

| Provider | Rules | Agents | Skills | Commands | Hooks | MCP |
|---|---|---|---|---|---|---|
| **Claude Code** | `CLAUDE.md`, `.claude/rules/*.md` | `.claude/agents/*.md`, `agents/*.md` | `.claude/skills/*/SKILL.md`, `skills/*/SKILL.md` | `.claude/commands/*.md`, `commands/*.md` | `.claude/hooks/*` + `settings.json` | `.mcp.json` |
| **Cursor** | `.cursorrules`, `.cursor/rules/*.mdc` | `.cursor/agents/*.md` | `.cursor/skills/*/SKILL.md` | `.cursor/commands/*.md` | `.cursor/hooks.json` + `.cursor/hooks/` | `.mcp.json` |
| **Copilot** | `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md` | `.github/agents/*.md` | `skills/*/SKILL.md` | via plugins | `hooks/` | `.mcp.json` |
| **Windsurf** | `.windsurfrules` | — | — | — | — | — |
| **Cline** | `.clinerules`, `.clinerules/` | — | — | — | — | `.vscode/mcp.json` |
| **Roo Code** | `.roo/rules/*.md` | — | — | — | — | `.roo/mcp.json` |
| **Codex** | `AGENTS.md`, `.codex/` | `.codex/agents/*.toml` | `.codex/skills/` | — | — | — |
| **Gemini** | `GEMINI.md` | — | `.gemini/skills/*/SKILL.md` | — | — | — |
| **Aider** | `CONVENTIONS.md` | — | — | — | — | — |

### Top-Level (Provider-Agnostic) Content Locations

Many repos put content at the root level without any provider prefix:

| Content Type | Path Pattern |
|---|---|
| Agents | `agents/*.md`, `agents/<category>/*.md` |
| Skills | `skills/<name>/SKILL.md` |
| Hooks | `hooks/`, `hook-scripts/` |
| Commands | `commands/*.md` |
| Rules | `rules/*.md`, `rules/*.mdc` |
| MCP | `.mcp.json` at root |
| Prompts | `prompts/*.md` |

### Key Observations

1. **No repo uses .syllago.yaml or registry.yaml** — syllago's metadata format is novel
2. **Flat agent .md files are universal** — directory-per-agent with AGENT.md is syllago's convention only
3. **Skills are the most standardized** — `SKILL.md` in a named directory is near-universal
4. **Hooks are the most fragmented** — Python, JS, TS, Bash, PowerShell; many wiring approaches
5. **Two placement strategies:** content inside `.claude/` (for direct use) vs top-level directories (for sharing)
6. **Cross-provider repos duplicate content** across provider trees (`.claude/skills/`, `.cursor/skills/`, `.codex/skills/`)
7. **`.mcp.json` at root is nearly universal** for MCP configs
8. **The plugin system** (`.claude-plugin/plugin.json`) is emerging as a package format
9. **Most repos have no manifest** — README is the index
10. **instructa/ai-prompts is the closest to a proper registry** with per-item JSON metadata
