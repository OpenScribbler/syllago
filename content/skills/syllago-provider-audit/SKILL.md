---
name: syllago-provider-audit
description: Use when auditing AI coding tool providers OR researching provider capabilities OR updating provider docs OR checking what changed in a provider. Runs structured research against official docs and produces standardized reports for syllago's provider accuracy.
---

# Provider Audit

Structured research workflow for auditing AI coding tool providers supported by syllago. Takes a provider slug, researches official docs, and produces/updates standardized 4-file reports that drive syllago's converter accuracy.

## Accuracy Standard

**Every discrepancy matters. No exceptions.**

Syllago's converter output is only as good as its provider knowledge. A wrong tool name, a stale directory path, a renamed config field — these produce silent failures for users. Content converts "successfully" but doesn't work in the target provider. The user blames their setup, not our data.

This means:
- **Fix every finding**, no matter how small. A renamed URL path is as important as a missing tool.
- **Never dismiss a discrepancy as "minor"**. If the docs say one thing and our reports say another, our reports are wrong.
- **Always update reports AND create beads** for anything that affects the converter, toolmap, or provider definition. Don't just note it — fix it or track it.
- **After a diff, the default action is "both"** (update reports + create beads). Only skip if the user explicitly says otherwise.

## Usage

```
/syllago-provider-audit <provider-slug>          # Full audit of one provider
/syllago-provider-audit diff <provider-slug>     # Diff against existing reports
/syllago-provider-audit --all                    # Audit all 12 providers
```

## Supported Providers

`amp`, `claude-code`, `gemini-cli`, `cursor`, `windsurf`, `codex`, `copilot-cli`, `cline`, `roo-code`, `opencode`, `kiro`, `zed`

## Report Format

Each provider gets 4 research files in the syllago repo at `docs/providers/<slug>/`:

| File | Covers |
|------|--------|
| `tools.md` | Built-in tools: exact names, purposes, parameters, cross-provider equivalents |
| `content-types.md` | Rules, skills, agents, hooks, MCP, commands: file formats, directory structure, config schema, required fields |
| `hooks.md` | Hook events, config format, matcher syntax, execution model, structured output support |
| `skills-agents.md` | Skill/agent definition format, model selection, tool permissions, invocation patterns |

## Report Metadata

**Every report file MUST start with a metadata block:**

```
<!-- provider-audit-meta
provider: zed
provider_version: "0.178.x"
report_format: 1
researched: 2026-03-21
researcher: claude-opus-4.6
changelog_checked: https://zed.dev/releases
-->
```

All fields are required. `provider_version` is the version you researched against. `changelog_checked` is the URL you verified for recent releases. See `docs/providers/REPORT-FORMAT.md` for full spec.

## Changelog Sources (MUST CHECK)

Every audit and diff MUST check the provider's changelog for releases newer than the last research date. This is not optional.

| Provider | Changelog |
|----------|-----------|
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
| Amp | https://ampcode.com/manual (check for version/feature updates) |

## Source Attribution

Every finding must be tagged:
- `[Official]` — from provider's official docs or repo
- `[Community]` — blog posts, tutorials, forums
- `[Inferred]` — derived from source code or config examples
- `[Unverified]` — stated but not confirmed

Include URLs for all sources. Prefer specific pages, not docs root.

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
- AI Overview: https://zed.dev/docs/ai/overview
- Rules: https://zed.dev/docs/ai/rules
- MCP: https://zed.dev/docs/ai/mcp
- Tools: https://zed.dev/docs/ai/tools

### Amp
- Manual: https://ampcode.com/manual (single-page reference — sub-pages are auth-gated)
- Developer: Sourcegraph (uses Claude models but is NOT an Anthropic product)
- Note: Sub-pages under `/manual/` require authentication, but the main `/manual` page contains the full reference

## Audit Workflow

### Step 1: Validate the provider slug

### Step 2: Check changelog FIRST
Fetch the provider's changelog URL (from the table above). Identify:
- The latest release version/date
- What changed since our last research (check metadata block in existing reports for `researched` date)
- Any new features, renamed tools, moved paths, deprecated features

This gives you a targeted list of what to verify, instead of re-researching everything blind.

### Step 3: Load existing reports
Check `docs/providers/<slug>/`. If reports exist, read them and their metadata blocks. Note the `provider_version` and `researched` date.

### Step 4: Research each area
For each of the 4 report files, fetch official docs and extract findings. Use parallel subagents (one per report area) for speed. **Prioritize areas flagged by changelog changes.**

**Research strategy:**
1. Official docs first — fetch key pages from the URLs above
2. GitHub source code — for open-source providers, check for undocumented features
3. Changelog/releases — search for recent changes
4. Community sources — only to fill gaps in official docs
5. Prefer Readability MCP for fetching; fall back to WebFetch for JS-heavy sites (Cursor, Windsurf, Cline)

### Step 5: Write reports
Write all 4 files to `docs/providers/<slug>/`. Each file MUST start with the metadata block (see Report Metadata section). Set `provider_version` to the latest release found in Step 2, `researched` to today, and `changelog_checked` to the changelog URL.

### Step 6: Summarize
Report: provider version audited, tools found, content types supported, hook events, key differences from previous reports, recommended syllago changes (toolmap updates, provider definition fixes, new beads needed).

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

### Step 4: Act on every finding

**Default: update reports AND create beads.** Don't ask — just do both unless the user says otherwise.

For each finding in the diff:

| Finding type | Action |
|-------------|--------|
| New tool | Update tools.md + check if toolmap.go needs a mapping |
| Changed path/filename | Update content-types.md + check provider definition (InstallDir, DiscoveryPaths) |
| Changed config field | Update content-types.md + check converter (canonicalize/render) |
| Changed URL | Update all reports referencing old URL |
| New content type | Update content-types.md + create bead for provider expansion |
| Removed/deprecated feature | Update relevant report + check if converter handles gracefully |
| Tool name discrepancy | Update tools.md + fix toolmap.go entry |

After applying all changes:
1. Update the report files in `docs/providers/<slug>/`
2. Create beads for any converter/toolmap/provider definition changes
3. Run `make build && make test` if code was changed
4. Summarize what was updated and what beads were created

## Gotchas

- **JS-heavy doc sites**: Cursor, Windsurf, and Cline docs are React SPAs — use WebFetch not Readability MCP or you get empty content
- **DuckDuckGo rate limiting**: After ~20 searches, fall back to WebSearch
- **GitHub raw content**: For open-source providers, fetch from `raw.githubusercontent.com` — much cleaner than rendered pages
- **Kiro dual format**: Agents support both JSON and markdown. Markdown is primary as of 2026-03.
- **Source attribution is critical**: Without it, future audits can't distinguish verified facts from assumptions
- **Check git dates**: Before diffing, check `git log -1 --format=%ai -- docs/providers/<slug>/` to know how stale the reports are
- **Never dismiss "minor" findings**: A renamed URL, a slightly different tool name, a moved config path — these all cause silent converter failures. Every discrepancy gets fixed or tracked. The diff workflow defaults to "update + create beads" for this reason.
