# Skill Runtime Loading Behavior: GitHub Copilot, Cline, Roo Code

**Research date:** 2026-03-30  
**Scope:** Runtime skill loading behavior for three agents: GitHub Copilot (VS Code + CLI), Cline, and Roo Code.  
**Method:** Official docs, source code inspection via GitHub API, and release notes.

---

## Quick Reference

| Question | GitHub Copilot | Cline | Roo Code |
|---|---|---|---|
| Discovery reading depth | Frontmatter only (name, description) | Frontmatter only | Frontmatter only |
| Inactive skills loaded? | No | No | No |
| Activation loading scope | SKILL.md body only | SKILL.md body only | SKILL.md body only |
| Eager link resolution | No | No | No (explicit instruction against it) |
| Recognized directories | `.github/skills/`, `.claude/skills/`, `.agents/skills/` (project); `~/.copilot/skills/`, `~/.claude/skills/`, `~/.agents/skills/` (personal) | `.cline/skills/`, `.clinerules/skills/`, `.claude/skills/`, `.agents/skills/` (project); `~/.cline/skills/` (global) | `.roo/skills/`, `.roo/skills-{mode}/`, `.agents/skills/`, `.agents/skills-{mode}/` (project + global mirrors) |
| Resource enumeration | Not enumerated at discovery; accessed by model with read_file when referenced | Not enumerated at discovery; skill dir path exposed to model | Not enumerated at discovery; skill dir path exposed to model |
| Path resolution base | Skill directory (relative paths from SKILL.md) | Skill directory (absolute path exposed to model) | Skill directory (absolute path exposed to model) |
| Frontmatter handling | Stripped by platform before injection | Stripped; only body injected | Stripped; only body injected |
| Content wrapping | `<additional_skills_reminder>` XML tag for reminder only; skill content loaded raw via read_file | Raw text: `# Skill "{name}" is now active\n\n{instructions}\n---` | Raw text: `Skill: {name}\nDescription: ...\n\n--- Skill Instructions ---\n\n{body}` |
| Reactivation deduplication | URI-level dedup via ResourceSet (`hasSeen`); content-level dedup via Set (`hasSeenContent`) | Prompt instruction only: "do not call use_skill again" | Prompt instruction only: "Do NOT reload a skill whose instructions already appear in this conversation" |
| Context compaction protection | Not documented | Not documented | Not documented |
| Trust gating | No documented approval step for project skills; terminal tool execution has optional auto-approve allow-lists | No approval step for skill loading; tool call is shown to user as `useSkill` event | Yes — `askApproval("tool", ...)` called before loading skill body; user must approve |
| Nested skill discovery | Not documented | Not supported (flat directory scan only) | Not supported (flat directory scan only) |
| Cross-skill invocation | Not documented | Not documented; model-driven only | Not documented; model-driven only |
| Invocation depth limit | Not documented | Not documented | Not documented |

---

## GitHub Copilot (VS Code + CLI)

### Overview

Copilot implements the agentskills.io open standard. Skills are folders containing a `SKILL.md` file. Copilot's implementation diverges from Cline and Roo Code in one critical architectural difference: **Copilot does not use a skill tool call to load skill content**. Instead, the model is prompted to use the standard `read_file` tool to read SKILL.md when it determines a skill applies.

**Key source files:**
- `src/platform/customInstructions/common/promptTypes.ts` — `WORKSPACE_SKILL_FOLDERS`, `PERSONAL_SKILL_FOLDERS`, `SKILL_FILENAME` constants
- `src/platform/customInstructions/common/customInstructionsService.ts` — skill discovery, detection, dedup logic
- `src/extension/prompts/node/agent/agentPrompt.tsx` — `SkillAdherenceReminder` component
- `src/extension/prompts/node/panel/customInstructions.tsx` — deduplication via `hasSeen` and `hasSeenContent`

### Q1. Discovery reading depth

At discovery, Copilot reads only the **YAML frontmatter** (`name` and `description`) from `SKILL.md`. The full file body is not loaded. Inactive skills (those not matching the current request) are not loaded into context.

The instruction index file (a workspace-level index of skill paths and skill folders) is built at startup and maintained via file watchers. The index contains skill file URIs and skill folder URIs — not the skill content itself.

**Source:** `customInstructionsService.ts` — `parseInstructionIndexFile()`, `IInstructionIndexFile` interface; `SkillAdherenceReminder` checks `indexFile.skills.size`, not file content  
**Source:** `promptTypes.ts` — `WORKSPACE_SKILL_FOLDERS = ['.github/skills', '.claude/skills']`, `PERSONAL_SKILL_FOLDERS = ['.copilot/skills', '.claude/skills']`

### Q2. Activation loading scope

When a skill is relevant, the model is instructed to use `read_file` to load `SKILL.md`. This loads the full file content including body but **not** referenced subdirectory files. Those are accessed only if the model explicitly reads them.

The `SkillAdherenceReminder` prompt states: "Always check if any skills apply to the user's request. If so, use the `read_file` tool to read the corresponding SKILL.md files."

**Source:** `agentPrompt.tsx` lines ~512–533 — `SkillAdherenceReminder` class and its render output

### Q3. Eager link resolution

No. The VS Code docs state: "additional files in the skill directory...are accessed only when it references them." The model must explicitly issue `read_file` calls for any linked files.

**Source:** VS Code docs — `https://code.visualstudio.com/docs/copilot/customization/agent-skills`

### Q4. Recognized directory set

Project-level skills:
- `.github/skills/`
- `.claude/skills/`

Personal/global skills:
- `~/.copilot/skills/`
- `~/.claude/skills/`

Additional locations configurable via `chat.agentSkillsLocations` setting. `.agents/skills/` is also recognized per the `.agents/skills/` directory present in the vscode-copilot-chat repo and the agentskills.io spec. No `scripts/`, `docs/`, or `references/` subdirectories are explicitly enumerated; they are accessible by model request.

**Source:** `promptTypes.ts` — `WORKSPACE_SKILL_FOLDERS`, `PERSONAL_SKILL_FOLDERS`  
**Source:** `copilotCLISkills.ts` — `SKILLS_LOCATION_KEY` config expansion  
**Source:** GitHub Copilot docs — `https://docs.github.com/en/copilot/concepts/agents/about-agent-skills`

### Q5. Resource enumeration

Not enumerated at discovery. The skill folder URI is registered in the instruction index, meaning any file under the skill directory can be read by the model using `read_file`. The model decides when to read additional files based on the instructions in `SKILL.md`.

**Source:** `customInstructionsService.ts` — `isExtensionPromptFile()`: "For skills, the URI points to SKILL.md — allow everything under the parent folder"

### Q6. Path resolution base

Paths in SKILL.md are resolved relative to the **skill directory**. The agentskills.io spec states: "When referencing other files in your skill, use relative paths from the skill root." The `customInstructionsService` uses `skillFolderUri` as the base for resolving file access permissions.

**Source:** agentskills.io spec — `https://agentskills.io/specification`  
**Source:** `customInstructionsService.ts` — `isExtensionPromptFile()` checks `extUriBiasedIgnorePathCase.isEqualOrParent(uri, skillFolderUri)`

### Q7. Frontmatter handling

The `SKILL.md` frontmatter is available to the model when it reads the file (Copilot uses `read_file`, so the model receives the raw file including frontmatter). However, the frontmatter is not separately injected as structured data; the skill metadata is tracked internally via `skillName` and `skillFolderUri`. The model reads the file contents as-is.

**Source:** `customInstructionsService.ts` — `isSkillMdFile()` identifies the file; no stripping logic found in source  
**Note:** Behavior differs from Cline/Roo Code which explicitly strip frontmatter before injecting body text

### Q8. Content wrapping

Skill content is **not wrapped in XML tags** when loaded. The model reads the raw `SKILL.md` file content via `read_file`. The only XML-wrapped element is the `SkillAdherenceReminder` reminder, which is wrapped in `<additional_skills_reminder>` tags and is placed in the prompt's `<reminderInstructions>` section — this is the cue, not the content.

**Source:** `agentPrompt.tsx` — `<Tag name='additional_skills_reminder'>` for the reminder only

### Q9. Reactivation deduplication

Yes, deduplication operates at two levels for instruction files generally:
1. **URI-level:** `hasSeen` (a `ResourceSet`) prevents loading the same file URI twice
2. **Content-level:** `hasSeenContent` (a `Set<string>`) prevents injecting duplicate content even from different URIs

This mechanism applies to instruction files and custom agents. For skills specifically, the model reads SKILL.md via `read_file` in the conversation context, so duplicate reads would just repeat content in the conversation history.

**Source:** `customInstructions.tsx` — `hasSeen` and `hasSeenContent` dedup logic, lines ~70–110

### Q10. Context compaction

Not documented. No evidence found of special skill protection from context window pruning or summarization.

### Q11. Trust gating

No approval step documented for loading project-level skills themselves. Terminal tool execution (running scripts from skill directories) has optional auto-approve allow-lists configurable via VS Code settings. The `allowed-tools` frontmatter field (experimental) can pre-approve specific tool calls.

**Source:** VS Code docs — `https://code.visualstudio.com/docs/copilot/customization/agent-skills`

### Q12. Nested skill discovery

Not documented. The discovery mechanism uses `WORKSPACE_SKILL_FOLDERS` as fixed top-level directories and then expects `{skill-name}/SKILL.md` directly inside. There is no recursive scan for nested SKILL.md files.

**Source:** `customInstructionsService.ts` — `relativePath.split('/')[0]` extracts skill name as first path segment, implying single-level depth

### Q13. Cross-skill invocation

Not documented as a feature. Multiple skills can be loaded in a single request (the reminder says "Multiple skill files may be needed"), but this is parallel loading by the model, not one skill invoking another.

**Source:** `agentPrompt.tsx` — "Multiple skill files may be needed for a single request"

### Q14. Invocation depth limit

Not documented.

---

## Cline

### Overview

Cline implements a skill loading system based on the agentskills.io spec. At discovery, only frontmatter metadata is parsed. When a request matches a skill, the model calls the `use_skill` tool which loads the SKILL.md body and returns it as a tool result. No approval step is required for the load itself; the model acts on the returned instructions.

**Key source files:**
- `src/core/context/instructions/user-instructions/skills.ts` — discovery, metadata loading, content loading
- `src/core/prompts/system-prompt/components/skills.ts` — system prompt injection (skill list)
- `src/core/prompts/system-prompt/tools/use_skill.ts` — tool definition
- `src/core/task/tools/handlers/UseSkillToolHandler.ts` — tool execution and result format
- `src/shared/skills.ts` — `SkillMetadata` and `SkillContent` type definitions

### Q1. Discovery reading depth

At discovery, `loadSkillMetadata()` reads the full `SKILL.md` file from disk but **parses only the frontmatter** (`name` and `description`). The markdown body is discarded at this stage. Inactive skills are **not** loaded into context; only their name and description appear in the system prompt.

The `SkillMetadata` type confirms: `name`, `description`, `path`, `source` — no `instructions` field.

**Source:** `skills.ts` — `loadSkillMetadata()`: "const { data: frontmatter } = parseFrontmatter(fileContent)" — body is not stored  
**Source:** `shared/skills.ts` — `SkillMetadata` interface (no `instructions` field); `SkillContent extends SkillMetadata` adds `instructions: string`

### Q2. Activation loading scope

When the `use_skill` tool is called, `getSkillContent()` reads the full `SKILL.md` file, strips frontmatter, and returns the markdown body as `instructions`. Only the SKILL.md body is returned — no other files in the skill directory are automatically loaded.

The tool result explicitly tells the model: "You may access other files in the skill directory at: `{path}`" — the model must explicitly request them.

**Source:** `UseSkillToolHandler.ts` — final return value includes `skillContent.instructions` and the directory path hint  
**Source:** `skills.ts` — `getSkillContent()`: "const { content: body } = parseFrontmatter(fileContent)"

### Q3. Eager link resolution

No. The tool result does not pre-fetch linked files. The instruction in the tool result tells the model where the skill directory is, but files are only loaded if the model explicitly requests them.

**Source:** `UseSkillToolHandler.ts` — tool result format; no pre-fetch logic

### Q4. Recognized directory set

Project-level scan directories (from `getSkillsDirectoriesForScan` test stubs):
- `.clinerules/skills/`
- `.cline/skills/`
- `.claude/skills/`
- `.agents/skills/`

Global/personal:
- `~/.cline/skills/`
- `~/.agents/skills/`

**Source:** `skills.test.ts` — `getSkillsDirectoriesForScan` stub returns these 6 paths  
**Source:** Cline docs — `https://github.com/cline/cline/blob/main/docs/customization/skills.mdx`

### Q5. Resource enumeration

Not enumerated at discovery. The tool result provides the skill directory path to the model. The Cline docs describe `docs/`, `templates/`, and `scripts/` as valid subdirectories but the loader does not enumerate them. The model accesses them via `read_file` or shell execution when referenced in the SKILL.md body.

**Source:** Cline skills docs — "Place skill directories in `.cline/skills/`... Cline reads documentation files using `read_file` when the instructions reference them. Scripts can be executed directly"

### Q6. Path resolution base

The skill directory's absolute path is exposed to the model in the tool result: `skillContent.path.replace(/SKILL\.md$/, "")`. Relative paths in SKILL.md body resolve from the skill directory root.

**Source:** `UseSkillToolHandler.ts` — "You may access other files in the skill directory at: `${skillContent.path.replace(/SKILL\.md$/, "")}`"

### Q7. Frontmatter handling

Frontmatter is stripped before the body is returned to the model. `parseFrontmatter()` splits at `--- frontmatter ---` and returns only `body`. The model never sees the raw frontmatter YAML.

**Source:** `skills.ts` — `getSkillContent()`: "const { content: body } = parseFrontmatter(fileContent)" then "instructions: body.trim()"  
**Source:** `frontmatter.ts` — `parseYamlFrontmatter()` extracts `body` as the content after the frontmatter block

### Q8. Content wrapping

No XML wrapping. The tool result is plain text:
```
# Skill "{name}" is now active

{instructions}

---
IMPORTANT: The skill is now loaded. Do NOT call use_skill again for this task. ...
```

The system prompt lists available skills as plain text bullet points:
```
Available skills:
  - "skill-name": description text
```

**Source:** `UseSkillToolHandler.ts` — return string template  
**Source:** `system-prompt/components/skills.ts` — `getSkillsSection()` returns plain markdown text

### Q9. Reactivation deduplication

Instruction-level only. The tool description says: "Use this tool ONCE when a user's request matches one of the available skill descriptions... After activation, follow the skill's instructions directly - do not call use_skill again." No runtime dedup mechanism prevents a second call.

**Source:** `use_skill.ts` — tool description text  
**Note:** There is no code-level guard; the instruction relies on the model's compliance

### Q10. Context compaction

Not documented. No evidence of special skill instruction protection from context pruning.

### Q11. Trust gating

No approval step for skill loading. The `UseSkillToolHandler.execute()` method calls `config.callbacks.say("tool", message, ...)` to display the skill use event in the UI, but there is no `askApproval` call. The model loads the skill content without user confirmation.

**Source:** `UseSkillToolHandler.ts` — no `askApproval` call; only `config.callbacks.say()` for display

### Q12. Nested skill discovery

Not supported. The `scanSkillsDirectory()` function iterates the top-level entries of the skills directory and looks for `{entry}/SKILL.md`. It does not recurse into subdirectories.

**Source:** `skills.ts` — `scanSkillsDirectory()`: "for (const entryName of entries)" — single level only

### Q13. Cross-skill invocation

Not a built-in feature. One skill's instructions could tell the model to call `use_skill` for another skill, but this is model-driven and not architecturally supported or prohibited. No evidence of chaining support.

### Q14. Invocation depth limit

Not documented.

---

## Roo Code

### Overview

Roo Code has the most mature and complete skills implementation of the three agents. It was an early adopter of the agentskills.io spec and implements it with full mode-scoping, symlink support, project-level override priority, and file watchers. The trust model is the most explicit: skill loading requires user approval via `askApproval` before the skill body is returned to the model.

**Key source files:**
- `src/services/skills/SkillsManager.ts` — full discovery, scanning, content loading
- `src/services/skills/skillInvocation.ts` — tool execution helpers: `buildSkillApprovalMessage`, `buildSkillResult`
- `src/core/tools/SkillTool.ts` — tool handler with `askApproval` gate
- `src/core/prompts/sections/skills.ts` — system prompt `<available_skills>` XML injection
- `src/core/prompts/tools/native-tools/skill.ts` — tool definition
- `src/shared/skills.ts` — `SkillMetadata` and `SkillContent` types
- `.roo/skills/` — example skills in Roo Code's own repo

### Q1. Discovery reading depth

At discovery, `loadSkillMetadata()` reads the full `SKILL.md` file but parses **only the frontmatter** (name, description, modeSlugs). The body is discarded. Only `SkillMetadata` is stored — no `instructions` field.

**Source:** `SkillsManager.ts` — `loadSkillMetadata()`: "const { data: frontmatter, content: body } = matter(fileContent)" — `body` not stored in metadata  
**Source:** `shared/skills.ts` — `SkillMetadata` has no `instructions` field

### Q2. Activation loading scope

When the `skill` tool is called, `getSkillContent()` reads `SKILL.md` again from disk, strips frontmatter using `gray-matter`, and returns only the markdown body as `instructions`. No other files in the skill directory are loaded automatically.

The `buildSkillResult()` helper formats the body as plain text. The skill directory path is NOT included in the tool result (contrast with Cline which explicitly tells the model the directory path).

**Source:** `SkillsManager.ts` — `getSkillContent()`: "const { content: body } = matter(fileContent); return { ...skill, instructions: body.trim() }"  
**Source:** `skillInvocation.ts` — `buildSkillResult()`: returns `Skill: {name}\nDescription: ...\n\n--- Skill Instructions ---\n\n{body}`

### Q3. Eager link resolution

No. The system prompt section explicitly instructs against it: "Files linked from the skill are NOT loaded automatically" and "The model MUST explicitly decide to read a linked file based on task relevance."

**Source:** `src/core/prompts/sections/skills.ts` — `<linked_file_handling>` block in prompt output

### Q4. Recognized directory set

Roo Code scans the most directories of any of the three agents, with an 8-tier priority system:

Project-level (highest priority at top):
1. `.roo/skills/`
2. `.roo/skills-{mode}/` (for each active mode)
3. `.agents/skills/`
4. `.agents/skills-{mode}/`

Global (mirrors of above):
5. `~/.roo/skills/`
6. `~/.roo/skills-{mode}/`
7. `~/.agents/skills/`
8. `~/.agents/skills-{mode}/`

Mode slugs include both built-in modes (`code`, `architect`, `ask`, etc.) and custom modes.

**Source:** `SkillsManager.ts` — `getSkillsDirectories()` method; full directory list construction  
**Source:** Roo Code docs — `https://docs.roocode.com/features/skills`

### Q5. Resource enumeration

Not enumerated at discovery. The `buildSkillResult()` tool response does not include a directory listing. The agentskills.io spec's recognized subdirectories (`scripts/`, `references/`, `assets/`) are not enumerated in any Roo Code source file. The `<linked_file_handling>` prompt instructs the model to read linked files "on demand, not mandatory context."

**Source:** `skillInvocation.ts` — `buildSkillResult()` output format  
**Source:** `src/core/prompts/sections/skills.ts` — `<linked_file_handling>` block

### Q6. Path resolution base

Files in the skill body are resolved relative to the **skill directory**. The agentskills.io spec (which Roo Code references) states paths resolve from the skill root. The `SkillMetadata.path` field is an absolute path to `SKILL.md`, so the skill directory is `path.dirname(skill.path)`.

**Source:** agentskills.io spec  
**Source:** `SkillsManager.ts` — `skill.path` is absolute path to SKILL.md; `skillDir = path.dirname(skill.path)` pattern in delete/move operations

### Q7. Frontmatter handling

Frontmatter is stripped using the `gray-matter` library before the body is returned. The model only receives the markdown body. `gray-matter` is an explicit dependency (`import matter from "gray-matter"`) in `SkillsManager.ts`.

**Source:** `SkillsManager.ts` — "const { data: frontmatter, content: body } = matter(fileContent)" — only `body` used in `SkillContent.instructions`

### Q8. Content wrapping

At the system prompt level, available skills are listed in **XML**: `<available_skills><skill><name>...</name><description>...</description><location>...</location></skill></available_skills>`. The `<location>` field is the absolute path to SKILL.md.

When a skill activates (tool result), the content is **plain text**, not XML:
```
Skill: {name}
Description: {description}
Provided arguments: {args}
Source: {global|project}

--- Skill Instructions ---

{body}
```

**Source:** `src/core/prompts/sections/skills.ts` — XML generation for `<available_skills>` block  
**Source:** `skillInvocation.ts` — `buildSkillResult()` — plain text format

### Q9. Reactivation deduplication

Instruction-level only. The system prompt contains: "Do NOT reload a skill whose instructions already appear in this conversation." No code-level guard prevents a second `skill` tool call.

**Source:** `src/core/prompts/sections/skills.ts` — `<mandatory_skill_check>` block: "Do NOT reload a skill whose instructions already appear in this conversation."

### Q10. Context compaction

Not documented. No evidence found of skill instruction protection from context window pruning.

### Q11. Trust gating

**Yes — Roo Code requires explicit user approval before loading skill content.**

The `SkillTool.execute()` method calls `askApproval("tool", toolMessage)` with a JSON payload containing skill name, args, source, and description. If the user denies, the tool returns without loading the skill body. This is the only one of the three agents with a hard approval gate.

**Source:** `SkillTool.ts` — "const didApprove = await askApproval('tool', toolMessage); if (!didApprove) { return; }"  
**Source:** `src/services/skills/__tests__/skillTool.spec.ts` — test cases verify `askApproval` is called and skill is not loaded on denial

### Q12. Nested skill discovery

Not supported. `scanSkillsDirectory()` iterates the immediate entries of the skills directory and looks for `{entry}/SKILL.md`. It does not recurse into subdirectories. Symlinks to directories are followed (supporting symlinked skill directories), but nested SKILL.md files inside a skill's `docs/` or `references/` subdirectory would not be discovered.

**Source:** `SkillsManager.ts` — `scanSkillsDirectory()`: iterates one level; no recursion

### Q13. Cross-skill invocation

Not a built-in feature. The `<mandatory_skill_check>` prompt instructs: "Select EXACTLY ONE skill" and "Load skills ONLY after a skill is selected." This actively discourages loading multiple skills. No mechanism exists for one skill to trigger another.

**Source:** `src/core/prompts/sections/skills.ts` — `<mandatory_skill_check>`: "Select EXACTLY ONE skill. Prefer the most specific skill when multiple skills match."

### Q14. Invocation depth limit

Not documented as a formal limit. The `<mandatory_skill_check>` prompt's single-skill constraint ("Select EXACTLY ONE skill") effectively creates a depth limit of 1 for the automated skill selection path.

**Source:** `src/core/prompts/sections/skills.ts` — `<if_skill_applies>` block

---

## Comparative Analysis

### Architectural difference: tool call vs read_file

The most significant architectural difference between the three agents:

- **Copilot:** Skills are loaded by the model using the standard `read_file` tool. There is no dedicated skill tool. The model receives a reminder and decides whether to read SKILL.md.
- **Cline and Roo Code:** Skills are loaded via a dedicated `use_skill` / `skill` tool call. The tool performs discovery, content loading, and result formatting server-side.

### Discovery format: XML vs plain text

- **Roo Code** injects available skills as XML (`<available_skills>`, `<skill>`, `<name>`, `<description>`, `<location>`) in the system prompt.
- **Cline** injects skills as plain text bullets in the system prompt.
- **Copilot** uses an instruction index file and a `<additional_skills_reminder>` XML element; skill metadata is not directly listed in the system prompt.

### Trust model

- **Roo Code:** Hard approval gate — `askApproval` blocks loading until user confirms.
- **Cline:** Soft notification — shows tool use event but no blocking approval.
- **Copilot:** No approval step for loading; script execution has optional allow-lists.

### Mode scoping

- **Roo Code:** Native mode scoping via `skills-{mode}/` directories and `modeSlugs` frontmatter field. Skills can be restricted to specific modes.
- **Cline:** No mode scoping mechanism.
- **Copilot:** No mode scoping mechanism.

### File watching

- **Roo Code:** Active file watchers on all skills directories rediscover skills when SKILL.md files change.
- **Cline:** Toggling and discovery are on-demand; no documented file watchers.
- **Copilot:** Uses VS Code's `onDidChangeSkills` event (platform-provided).

---

## Sources

| Source | URL |
|---|---|
| GitHub Copilot agent skills docs | `https://docs.github.com/en/copilot/concepts/agents/about-agent-skills` |
| VS Code agent skills docs | `https://code.visualstudio.com/docs/copilot/customization/agent-skills` |
| VS Code copilot customization docs | `https://code.visualstudio.com/docs/copilot/copilot-customization` |
| agentskills.io specification | `https://agentskills.io/specification` |
| vscode-copilot-chat: promptTypes.ts | `github.com/microsoft/vscode-copilot-chat/src/platform/customInstructions/common/promptTypes.ts` |
| vscode-copilot-chat: customInstructionsService.ts | `github.com/microsoft/vscode-copilot-chat/src/platform/customInstructions/common/customInstructionsService.ts` |
| vscode-copilot-chat: agentPrompt.tsx | `github.com/microsoft/vscode-copilot-chat/src/extension/prompts/node/agent/agentPrompt.tsx` |
| vscode-copilot-chat: customInstructions.tsx | `github.com/microsoft/vscode-copilot-chat/src/extension/prompts/node/panel/customInstructions.tsx` |
| vscode-copilot-chat: copilotCLISkills.ts | `github.com/microsoft/vscode-copilot-chat/src/extension/chatSessions/copilotcli/node/copilotCLISkills.ts` |
| Cline skills docs | `github.com/cline/cline/blob/main/docs/customization/skills.mdx` |
| Cline: skills.ts (user-instructions) | `github.com/cline/cline/src/core/context/instructions/user-instructions/skills.ts` |
| Cline: system prompt skills component | `github.com/cline/cline/src/core/prompts/system-prompt/components/skills.ts` |
| Cline: use_skill tool | `github.com/cline/cline/src/core/prompts/system-prompt/tools/use_skill.ts` |
| Cline: UseSkillToolHandler.ts | `github.com/cline/cline/src/core/task/tools/handlers/UseSkillToolHandler.ts` |
| Cline: shared/skills.ts | `github.com/cline/cline/src/shared/skills.ts` |
| Cline: skills.test.ts | `github.com/cline/cline/src/core/context/instructions/user-instructions/__tests__/skills.test.ts` |
| Roo Code skills docs | `https://docs.roocode.com/features/skills` |
| Roo Code: SkillsManager.ts | `github.com/RooVetGit/Roo-Code/src/services/skills/SkillsManager.ts` |
| Roo Code: skillInvocation.ts | `github.com/RooVetGit/Roo-Code/src/services/skills/skillInvocation.ts` |
| Roo Code: SkillTool.ts | `github.com/RooVetGit/Roo-Code/src/core/tools/SkillTool.ts` |
| Roo Code: prompts/sections/skills.ts | `github.com/RooVetGit/Roo-Code/src/core/prompts/sections/skills.ts` |
| Roo Code: shared/skills.ts | `github.com/RooVetGit/Roo-Code/src/shared/skills.ts` |
| Roo Code: skill native tool | `github.com/RooVetGit/Roo-Code/src/core/prompts/tools/native-tools/skill.ts` |
| Roo Code: SkillsManager.spec.ts | `github.com/RooVetGit/Roo-Code/src/services/skills/__tests__/SkillsManager.spec.ts` |
| Roo Code: skillInvocation.spec.ts | `github.com/RooVetGit/Roo-Code/src/services/skills/__tests__/skillInvocation.spec.ts` |
