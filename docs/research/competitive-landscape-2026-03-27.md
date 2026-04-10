# Competitive Landscape Research — 2026-03-27

Research session analyzing AI coding tool content managers and provider hook/MCP ecosystems.

---

## Competitors

### rulesync (dyoshikawa/rulesync)

- **Stars:** 936 | **npm/mo:** ~608K (CI-inflated) | **Language:** TypeScript | **License:** MIT
- **Architecture:** Hub-and-spoke with canonical format (same as syllago)
- **Providers:** 27 targets
- **Content types:** Rules, commands, subagents, skills, MCP, hooks, ignore, permissions
- **Hook support:** 27 canonical events, 7 provider adapters, camelCase canonical names
- **MCP support:** 16 providers, field stripping per capability
- **Enterprise readiness:** None (solo maintainer, Node 22 dependency, no audits/SBOM/private registry)
- **Maintenance model:** Reactive (no provider monitoring), AI-assisted (3 maintainer accounts + Claude bot), automated releases via OpenCode agent
- **Key issues:** #1340 (MCP field loss), #1317 (hook scripts not copied), #900 (path refs not transformed), #329 (wants registry/marketplace)
- **Bus factor:** 1 (95%+ commits from maintainer's accounts)

### ai-rulez (Goldziher/ai-rulez)

- **Stars:** 95 | **Language:** Go | **License:** MIT
- **Architecture:** Fan-out generator (NOT hub-and-spoke)
- **Providers:** 13 dedicated generators
- **Content types:** Rules, context, skills, agents, commands, MCP (partial)
- **Hook support:** None
- **Unique features:** Built-in content library (23 domains), 4-level token compression, MCP server mode, profiles, remote git includes
- **Enterprise readiness:** None (solo maintainer)

### block/ai-rules (Block/Square)

- **Stars:** 83 | **Language:** Rust | **License:** Apache-2.0
- **Architecture:** Symlink farm (simplest)
- **Providers:** 11
- **Content types:** Rules, commands, skills, MCP (copy only)
- **Hook support:** None
- **Unique features:** Monorepo nested traversal, status/sync checking
- **Enterprise readiness:** Corporate backing (Block) but no enterprise features

### syllago (us)

- **Architecture:** Hub-and-spoke with canonical format + formal hook spec
- **Providers:** 12
- **Content types:** Rules, skills, agents, commands, hooks, MCP, prompts, loadouts
- **Unique advantages:** Loadouts (curated bundles), registry ecosystem, TUI, Go binary (zero deps), formal hook spec
- **Gaps vs rulesync:** Provider count (12 vs 27), hook event coverage (~10 vs 27 canonical), no permissions type, no simulated features

---

## Provider Documentation Sources

### Changelog/Release Monitoring (Early Signal)

| Provider | Source | Type |
|---|---|---|
| Claude Code | code.claude.com/docs/en/changelog | Hosted |
| Cursor | cursor.com/changelog | Hosted |
| Gemini CLI | github.com/google-gemini/gemini-cli/releases | GitHub API |
| Copilot CLI | github.com/github/docs (PRs to copilot/) | GitHub API |
| Windsurf | docs.windsurf.com (llms.txt index) | Raw .md |
| Cline | github.com/cline/cline/releases | GitHub API |
| Roo Code | github.com/RooVetGit/Roo-Code/releases | GitHub API |
| OpenCode | github.com/opencode-ai/opencode/releases | GitHub API |
| Codex | developers.openai.com/codex | Hosted |
| Kiro | kiro.dev/docs/ | Unstable |
| Amp | ampcode.com/news | Hosted |
| Zed | zed.dev/releases | Hosted |

### Format Reference Pages

| Provider | Rules | Hooks | MCP | Skills/Agents |
|---|---|---|---|---|
| Claude Code | code.claude.com/docs/en/memory | /hooks | /mcp | /skills, /sub-agents |
| Cursor | cursor.com/docs/rules | /hooks | /mcp | — |
| Gemini CLI | GitHub docs/reference/configuration.md | docs/hooks/index.md | docs/tools/mcp-server.md | — |
| Copilot CLI | docs.github.com/.../add-custom-instructions | .../use-hooks | .../add-mcp-servers | — |
| Windsurf | docs.windsurf.com/.../memories | .../hooks | .../mcp | .../skills |
| Cline | docs.cline.bot/customization/cline-rules | — | /mcp/configuring-mcp-servers | — |
| Roo Code | docs.roocode.com/features/custom-instructions | — | /features/mcp/using-mcp-in-roo | /features/custom-modes |
| OpenCode | opencode.ai/docs/config/ | /docs/plugins/ | /docs/mcp-servers/ | /docs/agents/ |
| Codex | developers.openai.com/codex/guides/agents-md | /codex/hooks | /codex/mcp | /codex/skills |
| Kiro | kiro.dev/docs/steering/ | /docs/hooks/ | /docs/mcp/configuration/ | /docs/specs/ |
| Amp | ampcode.com/manual | — (has "checks") | /manual (MCP in skills) | /manual (skills) |
| Zed | zed.dev/docs/ai/rules | — | /docs/ai/mcp | — |

### Best Raw/Diffable Sources

| Provider | URL | Format |
|---|---|---|
| Gemini CLI | raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/*.md | Raw markdown |
| Windsurf | docs.windsurf.com/llms.txt → *.md URLs | Raw markdown |
| Copilot CLI | github.com/github/docs/tree/main/content/copilot/ | Raw markdown |
| OpenCode | opencode.ai/config.json | JSON Schema |
| Codex | developers.openai.com/codex/* | Versioned HTML |

---

## Hook Event Inventory (Official Docs, March 2026)

### Event Classification Criteria

**Spec-canonical (2+ providers):** Universal lifecycle concepts independently implemented by multiple providers. These belong in the hook spec's canonical event list.

**Provider-specific (1 provider):** Unique to one provider's architecture. These are handled in syllago's adapter layer via provider_data, NOT in the spec.

### Full Event Matrix

#### Clearly Canonical (broad consensus)

| Canonical Concept | CC | Cursor | Gemini | Copilot | Kiro | Codex | Windsurf | OpenCode | Count |
|---|---|---|---|---|---|---|---|---|---|
| before_tool_execute | PreToolUse | preToolUse | BeforeTool | preToolUse | Pre Tool Use | PreToolUse | pre_* (split) | tool.execute.before | 8 |
| after_tool_execute | PostToolUse | postToolUse | AfterTool | postToolUse | Post Tool Use | PostToolUse | post_* (split) | tool.execute.after | 8 |
| session_start | SessionStart | sessionStart | SessionStart | sessionStart | — | SessionStart | session_start | session.created | 7 |
| session_end | SessionEnd | sessionEnd | SessionEnd | sessionEnd | — | — | session_end | — | 5 |
| before_prompt | UserPromptSubmit | beforeSubmitPrompt | BeforeAgent | userPromptSubmitted | Prompt Submit | UserPromptSubmit | — | — | 6 |
| agent_stop | Stop | — | AfterAgent | — | Agent Stop | Stop | — | session.idle | 5 |
| tool_use_failure | PostToolUseFailure | postToolUseFailure | — | errorOccurred | — | — | — | — | 3 |

#### Likely Canonical (2-3 providers)

| Canonical Concept | Providers | Count | Notes |
|---|---|---|---|
| before_compact | CC (PreCompact), Cursor (preCompact), Gemini (PreCompress) | 3 | Context window management |
| after_compact | CC (PostCompact) | 1* | CC only but concept pairs with before_compact |
| notification | CC (Notification), Gemini (Notification) | 2 | System notification events |
| subagent_start | CC (SubagentStart), Cursor (subagentStart) | 2 | Multi-agent orchestration |
| subagent_stop | CC (SubagentStop), Cursor (subagentStop) | 2 | Multi-agent orchestration |
| permission_request | CC (PermissionRequest), Cursor (?), OpenCode (permission.asked) | 2-3 | Permission system hooks |
| worktree_create | CC (WorktreeCreate), Cursor (worktreeCreate) | 2 | Git worktree lifecycle |
| worktree_remove | CC (WorktreeRemove), Cursor (worktreeRemove) | 2 | Git worktree lifecycle |
| file_changed | CC (FileChanged), Kiro (File Save), OpenCode (file.edited), Cursor (afterFileEdit) | 4 | Filesystem events |
| file_created | Kiro (File Create) | 1* | Filesystem events (may grow) |
| file_deleted | Kiro (File Delete) | 1* | Filesystem events (may grow) |

#### Borderline (granular variants of canonical events)

| Concept | Providers | Notes |
|---|---|---|
| before_shell | Cursor (beforeShellExecution), Windsurf (pre_run_command) | Granular pre_tool_use for shell specifically |
| after_shell | Cursor (afterShellExecution), Windsurf (post_run_command) | Granular post_tool_use for shell |
| before_mcp | Cursor (beforeMCPExecution), Windsurf (pre_mcp_tool_use) | Granular pre_tool_use for MCP |
| after_mcp | Cursor (afterMCPExecution), Windsurf (post_mcp_tool_use) | Granular post_tool_use for MCP |
| before_file_read | Cursor (beforeReadFile), Windsurf (pre_read_code) | Granular pre_tool_use for file read |
| before_model | Gemini (BeforeModel), Cursor (beforeAgentResponse) | Pre-LLM-call hook |
| after_model | Gemini (AfterModel), Cursor (afterAgentResponse) | Post-LLM-call hook |
| after_thought | Cursor (afterAgentThought) | Post-thinking hook |
| before_tool_selection | Gemini (BeforeToolSelection), Cursor (beforeToolSelection) | Pre-tool-selection hook |

**Design question for these:** Are these separate canonical events or should the spec handle them via matchers on before_tool_execute? Windsurf and Cursor split pre_tool_use into per-tool-category events. The spec could either:
- (A) Define granular events (before_shell, before_mcp, etc.) — more events, cleaner mapping
- (B) Keep one before_tool_execute with matcher patterns — fewer events, adapter translates

#### Provider-Specific (1 provider only, NOT for spec)

| Event | Provider | Why It's Provider-Specific |
|---|---|---|
| TeammateIdle | Claude Code | CC multi-agent ("teammates") feature |
| TaskCreated / TaskCompleted | Claude Code | CC task management system |
| ConfigChange | Claude Code | CC settings change detection |
| InstructionsLoaded | Claude Code | CC rule loading lifecycle |
| CwdChanged | Claude Code | CC working directory tracking |
| Elicitation / ElicitationResult | Claude Code | CC UI interaction model |
| StopFailure | Claude Code | CC error variant of Stop |
| Manual Trigger | Kiro | Kiro UI button |
| Pre/Post Task Execution | Kiro | Kiro specs/task system |
| beforeTabFileRead / afterTabFileEdit | Cursor | Cursor tab-specific file events |

---

## Learnings for syllago Implementation

### From rulesync

1. **Shared converter pattern** — Use a config struct for most providers, custom adapters only for fundamentally different formats (OpenCode JS plugins, Copilot platform-aware output)
2. **Defensive schema design** — Use loose parsing (like z.looseObject) to pass through unknown fields without breaking
3. **Don't silently drop content** — rulesync issue #1340 shows field loss is a real user problem. Preserve via provider_data.
4. **Hook script copying** — rulesync issue #1317. syllago already handles this via resolveHookScripts().
5. **Path reference rewriting** — rulesync issue #900. Transform internal file references during conversion.

### From rulesync issues (community pain points)

1. Cross-platform path bugs (Windows vs Linux) — test on both
2. Provider-specific frontmatter silently ignored — validate and warn
3. Generated files create PR noise — .gitattributes linguist-generated
4. Global mode has subtle bugs — test global paths thoroughly
5. Users want a registry/marketplace (#329) — syllago already has this

### Architecture Patterns Worth Adopting

1. **Centralized MCP field capability metadata** — strip unsupported fields before converter, not inside it
2. **Factory meta pattern** — provider behavior as metadata structs, not conditional branches
3. **Round-trip tests per provider** — canonical → provider → canonical must produce same content
4. **JSONC + $schema for config** — free IDE validation
5. **Lockfile with integrity hashes** — content verification for CI

### Things NOT to Copy

1. rulesync's camelCase canonical names — syllago uses snake_case, which is more spec-like
2. rulesync's per-provider override sections in hooks.json — complex, buggy (issue #1259)
3. rulesync's simulated features as default — make it opt-in with clear warnings
4. ai-rulez's fan-out architecture — syllago's hub-and-spoke is architecturally superior
5. block/ai-rules' symlink approach — no real conversion happening
