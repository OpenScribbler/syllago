---
name: provider-audit
description: Use when auditing AI coding tool providers OR researching provider capabilities OR updating provider docs OR checking what changed in a provider. Runs structured research against official docs and produces standardized reports for syllago's provider accuracy.
---

# Provider Audit

Structured research workflow for auditing AI coding tool providers supported by syllago. Takes a provider slug, researches official docs, and produces/updates standardized 4-file reports that drive syllago's converter accuracy.

## Usage

```
/provider-audit <provider-slug>          # Full audit of one provider
/provider-audit diff <provider-slug>     # Diff against existing reports
/provider-audit --all                    # Audit all 11 providers
```

## Supported Providers

`claude-code`, `gemini-cli`, `cursor`, `windsurf`, `codex`, `copilot-cli`, `cline`, `roo-code`, `opencode`, `kiro`, `zed`

## Report Format

Each provider gets 4 research files in the syllago repo at `docs/providers/<slug>/`:

| File | Covers |
|------|--------|
| `tools.md` | Built-in tools: exact names, purposes, parameters, cross-provider equivalents |
| `content-types.md` | Rules, skills, agents, hooks, MCP, commands: file formats, directory structure, config schema, required fields |
| `hooks.md` | Hook events, config format, matcher syntax, execution model, structured output support |
| `skills-agents.md` | Skill/agent definition format, model selection, tool permissions, invocation patterns |

## Source Attribution

Every finding must be tagged:
- `[Official]` — provider's official docs or repo
- `[Community]` — blog posts, tutorials, forums
- `[Inferred]` — derived from source code or config examples
- `[Unverified]` — stated but not confirmed

Include URLs for all sources.

## Official Documentation Sources

### Claude Code
- Docs: https://docs.anthropic.com/en/docs/claude-code
- GitHub: https://github.com/anthropics/claude-code
- Hooks: https://docs.anthropic.com/en/docs/claude-code/hooks
- Skills: https://docs.anthropic.com/en/docs/claude-code/skills

### Gemini CLI
- GitHub: https://github.com/google-gemini/gemini-cli
- Docs: https://googlegemini.github.io/gemini-cli/
- Settings: https://github.com/google-gemini/gemini-cli/blob/main/docs/settings.md

### Cursor
- Docs: https://docs.cursor.com
- Rules: https://docs.cursor.com/context/rules
- MCP: https://docs.cursor.com/context/model-context-protocol

### Windsurf
- Docs: https://docs.windsurf.com
- Rules: https://docs.windsurf.com/windsurf/memories#rules

### Codex
- GitHub: https://github.com/openai/codex
- Config: https://github.com/openai/codex/blob/main/codex-cli/docs/config.md

### Copilot CLI
- Docs: https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-coding-agent
- MCP: https://docs.github.com/en/copilot/customizing-copilot/using-model-context-protocol

### Cline
- GitHub: https://github.com/cline/cline
- Docs: https://docs.cline.bot

### Roo Code
- GitHub: https://github.com/RooVetGit/Roo-Code
- Docs: https://docs.roocode.com

### OpenCode
- GitHub: https://github.com/opencode-ai/opencode
- Docs: https://opencode.ai/docs

### Kiro
- Docs: https://kiro.dev/docs
- Steering: https://kiro.dev/docs/steering

### Zed
- Docs: https://zed.dev/docs
- Assistant: https://zed.dev/docs/assistant

## Audit Workflow

### Step 1: Validate the provider slug

### Step 2: Load existing reports
Check `docs/providers/<slug>/`. If reports exist, read them first — focus on verifying and updating, not starting from scratch.

### Step 3: Research each area
For each of the 4 report files, fetch official docs and extract findings. Use parallel subagents (one per report area) for speed.

**Research strategy:**
1. Official docs first — fetch key pages from the URLs above
2. GitHub source code — for open-source providers, check for undocumented features
3. Changelog/releases — search for recent changes
4. Community sources — only to fill gaps in official docs
5. Prefer Readability MCP for fetching; fall back to WebFetch for JS-heavy sites (Cursor, Windsurf, Cline)

### Step 4: Write reports
Write all 4 files to `docs/providers/<slug>/`. Follow the format above exactly.

### Step 5: Summarize
Report: tools found, content types supported, hook events, key differences from previous reports, recommended syllago changes (toolmap updates, provider definition fixes, new beads needed).

## Diff Workflow

### Step 1: Load existing reports from `docs/providers/<slug>/`
### Step 2: Research current state (same strategy as audit)
### Step 3: Produce diff report to stdout:

```
## New
- [tools.md] New tool: `codebase_search` [Official](url)

## Changed
- [content-types.md] MCP path changed to mcp-config.json [Official](url)

## Removed
- [skills-agents.md] Legacy JSON agent format deprecated [Official](url)

## Recommended Actions
- Update toolmap.go: add codebase_search
- Create bead for MCP path fix
```

### Step 4: Ask whether to update reports, create beads, or both

## Gotchas

- **JS-heavy doc sites**: Cursor, Windsurf, and Cline docs are React SPAs — use WebFetch not Readability MCP or you get empty content
- **DuckDuckGo rate limiting**: After ~20 searches, fall back to WebSearch
- **GitHub raw content**: For open-source providers, fetch from `raw.githubusercontent.com` — much cleaner than rendered pages
- **Kiro dual format**: Agents support both JSON and markdown. Markdown is primary as of 2026-03.
- **Source attribution is critical**: Without it, future audits can't distinguish verified facts from assumptions
- **Check git dates**: Before diffing, check `git log -1 --format=%ai -- docs/providers/<slug>/` to know how stale the reports are
