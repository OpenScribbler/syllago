# Agent Skill Loading Behavior Matrix

**Date:** 2026-03-30
**Scope:** 12 agents, 14 behavioral checks from [agentskillimplementation.com/checks](https://agentskillimplementation.com/checks/)
**Method:** Official docs + source code inspection. "Not documented" means no evidence was found — not "no."

---

## How to Read This Document

Each cell contains a finding and its evidence level:
- **SRC** = verified in source code (highest confidence)
- **DOC** = stated in official documentation
- **INF** = inferred from architecture or related docs (lower confidence)
- **N/D** = not documented in any available source

---

## Matrix: Discovery and Loading (Checks 1-6)

### Q1: Discovery Reading Depth

*Does the agent read the entire SKILL.md at discovery? Are inactive skills loaded into context?*

| Agent              | Reads Full File at Discovery?                  | Inactive Skills in Context?                              | Evidence                                                                     |
|--------------------|------------------------------------------------|----------------------------------------------------------|------------------------------------------------------------------------------|
| **Claude Code**    | Yes                                            | Description only (1% of context, 250 char cap per skill) | DOC: code.claude.com/docs/en/skills                                          |
| **Codex CLI**      | Yes                                            | Name + description + path only                           | SRC: `loader.rs` (`parse_skill_file`), `render.rs` (`render_skills_section`) |
| **Gemini CLI**     | Yes                                            | Name + description + location only                       | SRC: `skillLoader.ts`, `snippets.ts` (`renderAgentSkills`)                   |
| **Cursor**         | Not documented                                 | Name + description only (progressive loading)            | DOC: cursor.com/docs/skills                                                  |
| **Windsurf**       | Not documented                                 | Name + description only                                  | DOC: docs.windsurf.com/windsurf/cascade/skills                               |
| **Kiro**           | Not documented                                 | Name + description only (progressive disclosure)         | DOC: kiro.dev/docs/skills/, agentskills.io spec                              |
| **GitHub Copilot** | Frontmatter only                               | Name + description only                                  | SRC: `customInstructionsService.ts` (`parseInstructionIndexFile`)            |
| **Cline**          | Yes (full read, body discarded)                | Name + description only                                  | SRC: `skills.ts` (`loadSkillMetadata`)                                       |
| **Roo Code**       | Yes (full read, body discarded)                | Name + description only                                  | SRC: `SkillsManager.ts` (`loadSkillMetadata`)                                |
| **OpenCode**       | Yes (full file, body stored in memory)         | Name + description only                                  | SRC: `skill/index.ts` (`add()`, `fmt()`)                                     |
| **Amp**            | Name + description only                        | Name + description only                                  | DOC: vercel-labs/agent-skills AGENTS.md                                      |
| **Junie CLI**      | Not documented (progressive disclosure stated) | Name + description only                                  | DOC: junie.jetbrains.com/docs/agent-skills.html                              |

**Finding:** Universal consensus: inactive skills surface only name + description. 6 agents read the full file at discovery but discard the body. Claude Code uniquely caps description budget at 1% of context window / 250 chars per skill.

---

### Q2: Activation Loading Scope

*When a skill activates, does it load only SKILL.md body or also supporting files?*

| Agent              | What Loads on Activation                                  | Supporting Files                                                                          | Evidence                                              |
|--------------------|-----------------------------------------------------------|-------------------------------------------------------------------------------------------|-------------------------------------------------------|
| **Claude Code**    | SKILL.md body only                                        | On-demand via Read tool                                                                   | DOC: code.claude.com/docs/en/skills                   |
| **Codex CLI**      | Full SKILL.md (including frontmatter)                     | On-demand; model told to load selectively                                                 | SRC: `injection.rs` (`build_skill_injections`)        |
| **Gemini CLI**     | SKILL.md body + directory tree listing (up to 200 items)  | File-system access granted to skill dir; content on-demand                                | SRC: `activate-skill.ts` (`execute()`)                |
| **Cursor**         | SKILL.md body; references on-demand, scripts via path     | On-demand                                                                                 | DOC: cursor.com/docs/skills                           |
| **Windsurf**       | SKILL.md + all files in skill folder "become available"   | Docs say files "become available" — may mean context-loaded or tool-accessible; ambiguous | DOC: docs.windsurf.com/windsurf/cascade/skills        |
| **Kiro**           | SKILL.md body only; supporting files on demand            | On-demand (spec recommends enumerate but not load)                                        | DOC: kiro.dev/docs/skills/, agentskills.io impl guide |
| **GitHub Copilot** | SKILL.md body via `read_file` tool                        | On-demand via `read_file`                                                                 | SRC: `agentPrompt.tsx` (`SkillAdherenceReminder`)     |
| **Cline**          | SKILL.md body only; skill dir path exposed                | On-demand                                                                                 | SRC: `UseSkillToolHandler.ts`                         |
| **Roo Code**       | SKILL.md body only; dir path NOT exposed                  | On-demand (model uses linked paths from body)                                             | SRC: `skillInvocation.ts` (`buildSkillResult`)        |
| **OpenCode**       | SKILL.md body + file listing (up to 10 files, paths only) | On-demand                                                                                 | SRC: `tool/skill.ts` (`execute()`)                    |
| **Amp**            | SKILL.md body; scripts referenced by absolute path        | On-demand                                                                                 | DOC: vercel-labs/agent-skills AGENTS.md               |
| **Junie CLI**      | SKILL.md body; referenced materials loaded as needed      | On-demand                                                                                 | DOC: junie.jetbrains.com/docs/agent-skills.html       |

**Finding:** Three distinct loading strategies:
1. **Body only** (Claude Code, Codex, Cursor, Kiro, Copilot, Cline, Roo Code, Amp, Junie) — model reads files on demand
2. **Body + directory enumeration** (Gemini CLI, OpenCode) — model sees what files exist, reads on demand
3. **Everything "available"** (Windsurf) — docs say all files "become available" on activation; unclear whether this means loaded into context or made accessible via tools

---

### Q3: Eager Link Resolution

*Does the agent pre-fetch files linked from SKILL.md?*

| Agent         | Eager Resolution? | Evidence                                                                                                                                                                                   |
|---------------|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| All 12 agents | **No**            | DOC/SRC: Every agent with documentation on this explicitly states lazy/on-demand loading. Roo Code source explicitly instructs "Files linked from the skill are NOT loaded automatically." |

**Finding:** Universal: no agent pre-fetches linked files. This is a safe authoring assumption.

---

### Q4: Recognized Directory Set

*What subdirectories inside a skill folder are specially recognized?*

| Agent              | Named Directories                                          | Treatment                                                                        | Evidence |
|--------------------|------------------------------------------------------------|----------------------------------------------------------------------------------|----------|
| **Claude Code**    | `scripts/`, `examples/` (recommended, not enforced)        | Any file/dir works                                                               | DOC      |
| **Codex CLI**      | `scripts/`, `references/`, `assets/`, `agents/`            | `agents/openai.yaml` special-cased; `scripts/` for implicit invocation detection | SRC      |
| **Gemini CLI**     | `scripts/`, `references/`, `assets/`                       | No special runtime treatment; all enumerated equally                             | SRC      |
| **Cursor**         | `scripts/`, `references/`, `assets/`                       | Documented as optional conventions                                               | DOC      |
| **Windsurf**       | None named                                                 | "Supporting files" — all files in skill folder                                   | DOC      |
| **Kiro**           | `scripts/`, `references/`, `assets/` + arbitrary           | Follows agentskills.io spec                                                      | DOC      |
| **GitHub Copilot** | None named                                                 | Any file under skill dir accessible via `read_file`                              | SRC      |
| **Cline**          | `docs/`, `templates/`, `scripts/` (documented conventions) | No special runtime treatment                                                     | DOC      |
| **Roo Code**       | None named specially                                       | `skills-{mode}/` directories for mode scoping                                    | SRC      |
| **OpenCode**       | None named                                                 | Any file enumerated (up to 10)                                                   | SRC      |
| **Amp**            | `scripts/` (convention)                                    | Referenced by absolute path                                                      | DOC      |
| **Junie CLI**      | `scripts/`, `templates/`, `checklists/`                    | Documented conventions, not enforced                                             | DOC      |

**Finding:** `scripts/` is the most common convention (8 agents mention it). `references/` and `assets/` appear in 3-4 agents. No agent enforces directory names at runtime — they are authoring conventions only. Windsurf names no specific directories and treats all files in the skill folder equally (whether "available" means loaded into context or tool-accessible is ambiguous — see Q5).

---

### Q5: Resource Enumeration on Activation

| Agent          | Behavior                                                        | Evidence |
|----------------|-----------------------------------------------------------------|----------|
| **Gemini CLI** | Enumerates full directory tree (up to 200 items), paths only    | SRC      |
| **OpenCode**   | Enumerates up to 10 files (sampled), paths only                 | SRC      |
| **Windsurf**   | All files "become available" (ambiguous: loaded or accessible?) | DOC      |
| **All others** | Not enumerated; accessed on-demand when referenced in SKILL.md  | DOC/SRC  |

---

### Q6: Path Resolution Base

| Agent              | Resolution Base            | Mechanism                                                         | Evidence |
|--------------------|----------------------------|-------------------------------------------------------------------|----------|
| **Claude Code**    | Skill directory            | `${CLAUDE_SKILL_DIR}` substitution variable                       | DOC      |
| **Codex CLI**      | Skill directory            | Model instructed; absolute path in discovery listing              | SRC      |
| **Gemini CLI**     | Skill directory            | `path.dirname(skill.location)`                                    | SRC      |
| **Cursor**         | Skill root                 | "Relative paths from the skill root"                              | DOC      |
| **Windsurf**       | N/D                        | —                                                                 | —        |
| **Kiro**           | Skill directory            | Follows agentskills.io spec: "relative paths from the skill root" | DOC      |
| **GitHub Copilot** | Skill directory            | `skillFolderUri` used as permission base                          | SRC      |
| **Cline**          | Skill directory            | Absolute path in tool result                                      | SRC      |
| **Roo Code**       | Skill directory            | `path.dirname(skill.path)`                                        | SRC      |
| **OpenCode**       | Skill directory (explicit) | `pathToFileURL(path.dirname(skill.location))` in tool output      | SRC      |
| **Amp**            | Skill directory            | `/mnt/skills/user/{skill-name}/` mount path                       | DOC      |
| **Junie CLI**      | N/D                        | Paths appear skill-relative but not confirmed                     | INF      |

**Finding:** 10 of 12 agents resolve relative to the skill directory. This is a safe authoring assumption.

---

## Matrix: Content Presentation (Checks 7-8)

### Q7: Frontmatter Handling

*Is YAML frontmatter stripped before the model sees skill content?*

| Agent              | Frontmatter                                                   | Evidence                                                              |
|--------------------|---------------------------------------------------------------|-----------------------------------------------------------------------|
| **Claude Code**    | **Stripped**                                                  | DOC: "YAML frontmatter... and markdown content" described as separate |
| **Codex CLI**      | **Included** (raw file injected)                              | SRC: `injection.rs` — `fs::read_to_string` with no stripping          |
| **Gemini CLI**     | **Stripped**                                                  | SRC: `skillLoader.ts` — `FRONTMATTER_REGEX`, `body = match[2]`        |
| **Cursor**         | N/D                                                           | —                                                                     |
| **Windsurf**       | N/D                                                           | —                                                                     |
| **Kiro**           | Both valid per spec; stripped more common for dedicated tools | DOC: agentskills.io impl guide                                        |
| **GitHub Copilot** | **Included** (model reads raw file via `read_file`)           | SRC: no stripping logic in `customInstructionsService.ts`             |
| **Cline**          | **Stripped**                                                  | SRC: `skills.ts` — `parseFrontmatter()` returns only body             |
| **Roo Code**       | **Stripped**                                                  | SRC: `SkillsManager.ts` — `gray-matter` strips frontmatter            |
| **OpenCode**       | **Stripped**                                                  | SRC: `skill/index.ts` — `gray-matter`, stores `md.content` only       |
| **Amp**            | N/D                                                           | Auth-gated docs                                                       |
| **Junie CLI**      | N/D                                                           | Implied stripped but not confirmed                                    |

**Finding:** Split behavior. 5 agents confirmed strip, 2 confirmed include (Codex, Copilot), 5 undocumented. Skills that include sensitive metadata in frontmatter (API keys in `metadata:` fields) could be exposed on Codex and Copilot.

---

### Q8: Content Wrapping Format

*How is skill content presented to the model?*

| Agent              | Discovery Format                                       | Activation Format                                                          | Evidence                       |
|--------------------|--------------------------------------------------------|----------------------------------------------------------------------------|--------------------------------|
| **Claude Code**    | N/D                                                    | N/D                                                                        | —                              |
| **Codex CLI**      | `<skills_instructions>` XML, user-role msg             | `<skill><name>...<path>...{body}</skill>` XML, user-role msg               | SRC                            |
| **Gemini CLI**     | `<available_skills><skill>` XML, system prompt         | `<activated_skill><instructions>...<available_resources>` XML, tool result | SRC                            |
| **Cursor**         | N/D                                                    | N/D (rules injected at "start of model context")                           | DOC                            |
| **Windsurf**       | N/D                                                    | N/D                                                                        | —                              |
| **Kiro**           | N/D                                                    | XML recommended per spec impl guide; Kiro's actual format N/D              | DOC: agentskills.io impl guide |
| **GitHub Copilot** | Instruction index + `<additional_skills_reminder>` XML | Raw file via `read_file` (no wrapping)                                     | SRC                            |
| **Cline**          | Plain text bullets                                     | Plain text: `# Skill "{name}" is now active\n\n{body}`                     | SRC                            |
| **Roo Code**       | `<available_skills><skill>` XML                        | Plain text: `Skill: {name}\n--- Skill Instructions ---\n{body}`            | SRC                            |
| **OpenCode**       | `<available_skills><skill>` XML (verbose)              | `<skill_content name="...">` XML with `<skill_files>`                      | SRC                            |
| **Amp**            | N/D                                                    | N/D                                                                        | Auth-gated                     |
| **Junie CLI**      | N/D                                                    | N/D                                                                        | —                              |

**Finding:** Three approaches: XML wrapping (Codex, Gemini, OpenCode, Roo Code discovery), plain text (Cline, Roo Code activation), and raw file read (Copilot). XML wrapping helps the model distinguish skill instructions from conversation history.

---

## Matrix: Lifecycle and Security (Checks 9-14)

### Q9: Reactivation Deduplication

| Agent              | Mechanism                                                  | Level                    | Evidence |
|--------------------|------------------------------------------------------------|--------------------------|----------|
| **Claude Code**    | N/D                                                        | —                        | —        |
| **Codex CLI**      | `seen_paths: HashSet` — per-turn only                      | Code-level (single turn) | SRC      |
| **Gemini CLI**     | `activeSkillNames: Set` tracks state, but no guard in tool | None (model-level)       | SRC      |
| **GitHub Copilot** | `hasSeen` (URI) + `hasSeenContent` (content hash)          | Code-level (session)     | SRC      |
| **Cline**          | Prompt instruction: "do not call use_skill again"          | Prompt-level             | SRC      |
| **Roo Code**       | Prompt instruction: "Do NOT reload a skill"                | Prompt-level             | SRC      |
| **OpenCode**       | None                                                       | None                     | SRC      |
| **Others**         | N/D                                                        | —                        | —        |

**Finding:** Only Copilot has robust session-level deduplication (URI + content hash). Codex deduplicates per-turn but not cross-turn. Cline and Roo Code rely on prompt instructions (model compliance). Most agents have no deduplication.

---

### Q10: Context Compaction Protection

| Agent           | Protected?                                                         | Evidence |
|-----------------|--------------------------------------------------------------------|----------|
| **Claude Code** | CLAUDE.md-embedded skills survive `/compact`; on-demand skills N/D | DOC      |
| **OpenCode**    | **No** — compaction prompt has no skill awareness                  | SRC      |
| **All others**  | N/D                                                                | —        |

**Finding:** Massively undocumented across the ecosystem. Only OpenCode definitively does NOT protect skills. This is a significant portability concern — skills that work in short conversations may silently lose instructions in long ones.

---

### Q11: Trust Gating

| Agent              | Approval Required?                                                         | Mechanism                                            | Evidence                       |
|--------------------|----------------------------------------------------------------------------|------------------------------------------------------|--------------------------------|
| **Claude Code**    | No (permission system controls Skill tool access)                          | Runtime permission model                             | DOC                            |
| **Codex CLI**      | No                                                                         | —                                                    | SRC                            |
| **Gemini CLI**     | **Yes** — two layers: folder trust + per-activation consent                | `isTrustedFolder()` + `getConfirmationDetails()`     | SRC                            |
| **Cursor**         | No (Team Rules enforcement only)                                           | —                                                    | DOC                            |
| **Windsurf**       | N/D                                                                        | —                                                    | —                              |
| **Kiro**           | Recommended per spec; `allowed-tools` experimental; Kiro-specific impl N/D | Spec recommends trust check for project-level skills | DOC: agentskills.io impl guide |
| **GitHub Copilot** | No (auto-approve allow-lists for script execution)                         | —                                                    | SRC                            |
| **Cline**          | No (shows UI event, no blocking)                                           | `callbacks.say()` only                               | SRC                            |
| **Roo Code**       | **Yes** — `askApproval("tool", ...)` blocks until user confirms            | Hard gate                                            | SRC                            |
| **OpenCode**       | **Yes** — configurable: allow/deny/ask per skill name pattern              | `ctx.ask({ permission: "skill" })`                   | SRC                            |
| **Amp**            | No                                                                         | —                                                    | DOC                            |
| **Junie CLI**      | No (advisory: "vet skills before installing")                              | —                                                    | DOC                            |

**Finding:** Only 3 of 12 agents definitively gate skill loading: Gemini CLI (strongest — folder trust + consent), Roo Code (hard approval), OpenCode (configurable patterns). Kiro follows the agentskills.io spec which recommends gating, but its specific implementation is not documented. Of the remaining 8, Claude Code controls access via its permission system (no per-skill approval), and the other 7 auto-load project-level skills with no approval — a security concern for repository cloning scenarios.

---

### Q12: Nested Skill Discovery

| Agent              | Discovers Nested SKILL.md?                   | Depth             | Evidence                                          |
|--------------------|----------------------------------------------|-------------------|---------------------------------------------------|
| **Claude Code**    | Yes (nested `.claude/skills/` dirs)          | Arbitrary         | DOC                                               |
| **Codex CLI**      | Yes (BFS)                                    | Depth 6 max       | SRC: `MAX_SCAN_DEPTH = 6`                         |
| **Gemini CLI**     | Partial (`*/SKILL.md` glob)                  | 1 level deep only | SRC                                               |
| **Cursor**         | AGENTS.md: yes. Skills: N/D                  | —                 | DOC                                               |
| **Windsurf**       | AGENTS.md: yes. Skills: N/D                  | —                 | DOC                                               |
| **Kiro**           | N/D for skills system; steering is flat only | N/D               | DOC: agentskills.io spec recommends max depth 4-6 |
| **GitHub Copilot** | No (single-level path segment extraction)    | 1                 | SRC                                               |
| **Cline**          | No (flat scan)                               | 1                 | SRC                                               |
| **Roo Code**       | No (flat scan)                               | 1                 | SRC                                               |
| **OpenCode**       | Yes (`**/SKILL.md` glob)                     | Arbitrary         | SRC                                               |
| **Amp**            | N/D                                          | —                 | —                                                 |
| **Junie CLI**      | No (one level deep)                          | 1                 | DOC                                               |

**Finding:** Split: 4 agents support nested discovery (Claude Code, Codex, OpenCode at arbitrary depth; Gemini at 1 level). 6 agents do flat single-level scan only. This creates a portability hazard for monorepo-style skill organizations.

---

### Q13: Cross-Skill Invocation

| Agent                           | Supported?                       | Mechanism                                                             | Evidence |
|---------------------------------|----------------------------------|-----------------------------------------------------------------------|----------|
| **All agents with skill tools** | Technically yes                  | Model-driven: skill body can instruct the model to call another skill | INF      |
| **Roo Code**                    | Actively discouraged             | Prompt: "Select EXACTLY ONE skill"                                    | SRC      |
| **Codex CLI**                   | Yes, via `$skill-name` sigil     | `extract_tool_mentions_with_sigil` parses skill refs from any content | SRC      |
| **Junie CLI**                   | Via subagent `skills` field only | Subagent definitions can list skills to preload                       | DOC      |

**Finding:** Cross-skill invocation is model-emergent, not architecturally supported. Roo Code actively discourages it. Codex has the strongest support via its `$skill-name` sigil parsing. No agent has a formal skill dependency/chaining system.

---

### Q14: Invocation Depth Limit

| Agent          | Limit?                                            | Evidence |
|----------------|---------------------------------------------------|----------|
| **Roo Code**   | Effective limit of 1 ("Select EXACTLY ONE skill") | SRC      |
| **All others** | N/D                                               | —        |

**Finding:** No agent documents an explicit depth limit. Roo Code's single-skill constraint is the only de facto limit. Circular invocation protection is absent across the ecosystem.

---

## Cross-Cutting Analysis

### The Three Loading Architectures

| Architecture             | Agents                                           | How Skills Load                                                                                              |
|--------------------------|--------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| **Dedicated skill tool** | Codex CLI, Gemini CLI, Cline, Roo Code, OpenCode | Runtime provides a `skill`/`activate_skill`/`use_skill` tool that handles discovery, loading, and formatting |
| **Standard read_file**   | GitHub Copilot                                   | No dedicated tool; model prompted to use `read_file` on SKILL.md when relevant                               |
| **Implicit injection**   | Claude Code, Cursor, Windsurf, Kiro, Amp, Junie  | Skill content injected into context by the runtime when activation conditions are met                        |

### Security Posture Tiers

| Tier                                      | Agents                                                  | Behavior                                                              |
|-------------------------------------------|---------------------------------------------------------|-----------------------------------------------------------------------|
| **Gated** (approval required)             | Gemini CLI, Roo Code, OpenCode                          | User must approve before skill body enters context                    |
| **Permissioned** (tool access controlled) | Claude Code                                             | Skill tool can be disabled; no per-skill approval                     |
| **Open** (auto-load, no approval)         | Codex CLI, Cursor, Windsurf, Copilot, Cline, Amp, Junie | Project-level skills load without user confirmation                   |
| **Unknown** (impl N/D)                    | Kiro                                                    | Spec recommends gating; Kiro's specific implementation not documented |

### Portability Risk Summary

| Check                     | Risk Level | Issue                                                                                                                                            |
|---------------------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| Activation scope (Q2)     | **High**   | Windsurf may load all files; others load body only. Skills depending on auto-loaded resources may break on 11 of 12 agents.                      |
| Frontmatter handling (Q7) | **Medium** | Codex and Copilot include frontmatter; others strip. Skills with sensitive metadata in frontmatter are exposed on 2 agents.                      |
| Nested discovery (Q12)    | **Medium** | 4 agents discover nested skills; 6 don't. Monorepo skill layouts break on most agents.                                                           |
| Trust gating (Q11)        | **High**   | Only 3 agents gate loading (Gemini, Roo, OpenCode). 7 auto-load with no approval; 1 (Claude Code) has permission-level control; 1 (Kiro) is N/D. |
| Context compaction (Q10)  | **High**   | Almost entirely undocumented. Skills may silently lose instructions in long conversations.                                                       |
| Deduplication (Q9)        | **Medium** | Only Copilot has robust dedup. Duplicate skill injection wastes tokens on most agents.                                                           |
| Content wrapping (Q8)     | **Low**    | Varies but skills don't need to know their wrapping format — this is transparent to authors.                                                     |

---

## Detailed Research Files

| Group | Agents                             | File                                                                             |
|-------|------------------------------------|----------------------------------------------------------------------------------|
| 1     | Claude Code, Codex CLI, Gemini CLI | [agent-group-1-cc-codex-gemini.md](./agent-group-1-cc-codex-gemini.md)           |
| 2     | Cursor, Windsurf, Kiro             | [agent-group-2-cursor-windsurf-kiro.md](./agent-group-2-cursor-windsurf-kiro.md) |
| 3     | GitHub Copilot, Cline, Roo Code    | [agent-group-3-copilot-cline-roo.md](./agent-group-3-copilot-cline-roo.md)       |
| 4     | OpenCode, Amp, Junie CLI           | [agent-group-4-opencode-amp-junie.md](./agent-group-4-opencode-amp-junie.md)     |

---

*Research conducted 2026-03-30. Checks framework from [agentskillimplementation.com/checks](https://agentskillimplementation.com/checks/).*
