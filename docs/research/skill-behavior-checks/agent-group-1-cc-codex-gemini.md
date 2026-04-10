# Runtime Skill Loading Behavior: Claude Code, Codex CLI, Gemini CLI

Research date: 2026-03-30  
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| Claude Code | https://code.claude.com/docs/en/skills (official docs), https://code.claude.com/docs/en/memory, https://code.claude.com/docs/en/sub-agents |
| Codex CLI | https://developers.openai.com/codex/skills (official docs), `codex-rs/core-skills/src/loader.rs`, `codex-rs/core-skills/src/injection.rs`, `codex-rs/core-skills/src/render.rs`, `codex-rs/instructions/src/` (github.com/openai/codex) |
| Gemini CLI | `packages/core/src/skills/skillLoader.ts`, `packages/core/src/skills/skillManager.ts`, `packages/core/src/tools/activate-skill.ts`, `packages/core/src/prompts/snippets.ts`, `docs/cli/skills.md`, `docs/cli/creating-skills.md` (github.com/google-gemini/gemini-cli) |

---

## Q1: Discovery Reading Depth

**Does the agent read only SKILL.md frontmatter at discovery, or the entire file? Does it load inactive skills into context?**

### Claude Code
The entire SKILL.md is read at discovery but only the `description` field (from frontmatter) enters context for inactive skills. The docs state:

> "In a regular session, skill descriptions are loaded into context so Claude knows what's available, but full skill content only loads when invoked."

Skills with `disable-model-invocation: true` have their description excluded from context entirely (not even the name/description is surfaced). Skills with `user-invocable: false` still have their description in context (for Claude to use) but are hidden from the `/` menu.

The description budget is dynamic: 1% of the context window, with a fallback of 8,000 characters. Each individual description is capped at 250 characters.

SOURCE: https://code.claude.com/docs/en/skills (section "Control who invokes a skill" table; section "Skill descriptions are cut short")

### Codex CLI
The entire SKILL.md is read at discovery. `loader.rs` (`parse_skill_file`) reads the full file contents, then extracts only name and description from frontmatter for the discovery index. The body is not stored in `SkillMetadata` — only `path_to_skills_md`. The render layer (`render.rs`) puts only `name`, `description`, and `path_to_skills_md` into the system prompt for all discovered skills.

From `render.rs` (verbatim):
```
"- Discovery: The list above is the skills available in this session (name + description + file path). Skill bodies live on disk at the listed paths."
```

Inactive skill bodies are not loaded into context. The model is told where the file lives and instructed to open it using `Read` when needed.

SOURCE: `codex-rs/core-skills/src/loader.rs` (function `parse_skill_file`), `codex-rs/core-skills/src/render.rs` (function `render_skills_section`)

### Gemini CLI
The entire SKILL.md is read at discovery. `skillLoader.ts` (`loadSkillFromFile`) reads the full file content and parses both frontmatter (name, description) and the body. However, discovery only injects name, description, and location into the system prompt via `renderAgentSkills()` in `snippets.ts`:

```typescript
// snippets.ts - renderAgentSkills()
const skillsXml = skills.map((skill) => `  <skill>
    <name>${skill.name}</name>
    <description>${skill.description}</description>
    <location>${skill.location}</location>
  </skill>`).join('\n');
```

The body is stored in `SkillDefinition.body` at load time but not injected until `activate_skill` is called. Inactive skill bodies do not enter context.

SOURCE: `packages/core/src/skills/skillLoader.ts` (function `loadSkillFromFile`), `packages/core/src/prompts/snippets.ts` (function `renderAgentSkills`)

---

## Q2: Activation Loading Scope

**When a skill activates, does it load only SKILL.md body or also files from the skill directory (references/, scripts/, etc.)?**

### Claude Code
Activation loads only the SKILL.md body. Supporting files (references/, scripts/, etc.) are **not** automatically loaded. The docs are explicit:

> "Large reference docs, API specifications, or example collections don't need to load into context every time the skill runs."

The recommended pattern is to reference supporting files from SKILL.md so Claude knows what they contain and when to load them. Files are loaded lazily via Claude's own Read tool when the skill instructions direct it to do so.

The exception is subagents with `skills:` field: those inject the full SKILL.md body at startup, but still do not auto-load supporting files.

SOURCE: https://code.claude.com/docs/en/skills (section "Add supporting files")

### Codex CLI
Activation loads the full SKILL.md contents (including frontmatter) from disk. Supporting files are not automatically loaded. The model is explicitly instructed:

> "If SKILL.md points to extra folders such as references/, load only the specific files needed for the request; don't bulk-load everything."
> "If scripts/ exist, prefer running or patching them instead of retyping large code blocks."
> "If assets/ or templates exist, reuse them instead of recreating from scratch."

The full SKILL.md contents (including frontmatter) are wrapped in `<skill>...</skill>` tags and injected as a user-role message. See Q8 for wrapping details.

SOURCE: `codex-rs/core-skills/src/render.rs`, `codex-rs/core-skills/src/injection.rs`

### Gemini CLI
Activation via `activate_skill` tool loads the SKILL.md body and also enumerates the skill's directory structure. The `execute()` method in `activate-skill.ts`:

1. Calls `skillManager.activateSkill(name)` to mark it active
2. Calls `config.getWorkspaceContext().addDirectory(path.dirname(skill.location))` — grants the agent read permission to the entire skill directory
3. Calls `getFolderStructure(path.dirname(skill.location))` — generates a tree listing of all files/folders in the skill directory (up to 200 items)
4. Returns all of this wrapped in `<activated_skill>` XML

The model receives the skill body AND a directory listing of available resources but does NOT receive the file contents of those resources — it must read them explicitly.

SOURCE: `packages/core/src/tools/activate-skill.ts` (function `ActivateSkillToolInvocation.execute`)

---

## Q3: Eager Link Resolution

**Does it follow markdown links in SKILL.md and pre-fetch linked files?**

### Claude Code
No. The docs explicitly describe a lazy-load pattern where Claude reads linked files on demand using its Read tool, guided by references in SKILL.md. There is no pre-fetching of linked files at activation time.

SOURCE: https://code.claude.com/docs/en/skills (section "Add supporting files")

### Codex CLI
No. The model receives the SKILL.md content verbatim (including any markdown links) but no link resolution occurs at injection time. The model is instructed to open files when needed:

> "After deciding to use a skill, open its SKILL.md. Read only enough to follow the workflow."
> "Avoid deep reference-chasing: prefer opening only files directly linked from SKILL.md unless you're blocked."

SOURCE: `codex-rs/core-skills/src/render.rs` (instructions in `render_skills_section`)

### Gemini CLI
No. The `activate_skill` tool injects the body text and directory structure, but does not follow or pre-fetch markdown links. The agent must use file-reading tools to access referenced content. The directory grant (Q2) means those files are accessible without additional permission prompts.

SOURCE: `packages/core/src/tools/activate-skill.ts`

---

## Q4: Recognized Directory Set

**What directories inside a skill folder does the agent recognize?**

### Claude Code
The docs describe and recommend: `scripts/`, `examples/`, and unnamed reference files (e.g., `reference.md`, `examples.md`). No fixed set of directories is enforced or recognized specially — any file or directory can be included and referenced from SKILL.md.

```
my-skill/
├── SKILL.md           # required
├── template.md
├── examples/
│   └── sample.md
└── scripts/
    └── validate.sh
```

SOURCE: https://code.claude.com/docs/en/skills (section "Add supporting files")

### Codex CLI
The Codex skill standard explicitly recognizes four directories: `scripts/`, `references/`, `assets/`, and `agents/` (for `openai.yaml` metadata). From `loader.rs`, path constants:
- `SKILLS_METADATA_DIR = "agents"` — for `openai.yaml`
- `scripts/` — executable scripts (referenced in `invocation_utils.rs` for implicit invocation detection)

The `skill-creator` built-in skill creates all four dirs. Only `agents/openai.yaml` has special loader treatment. `scripts/` has special treatment for implicit invocation detection (see Q6/Q5).

SOURCE: `codex-rs/core-skills/src/loader.rs` (constants), `codex-rs/core-skills/src/invocation_utils.rs`, https://developers.openai.com/codex/skills

### Gemini CLI
The docs and `creating-skills.md` define three recognized directories: `scripts/` (executable scripts), `references/` (static documentation), and `assets/` (templates and resources). The `getFolderStructure` utility recursively enumerates all directories on activation (no special treatment by directory name, beyond the default ignore list: `node_modules`, `.git`, `dist`, `__pycache__`).

SOURCE: `docs/cli/creating-skills.md`, `packages/core/src/utils/getFolderStructure.ts`

---

## Q5: Resource Enumeration

**How does it handle multiple files in a skill's subdirectories? Enumerate? Load all? Ignore until requested?**

### Claude Code
Files in subdirectories are entirely ignored until Claude's instructions (in SKILL.md) direct it to read them. No enumeration occurs at activation. Claude uses its standard Read/Glob tools to access files when needed.

SOURCE: https://code.claude.com/docs/en/skills (section "Add supporting files")

### Codex CLI
Files in subdirectories are not auto-loaded or enumerated at activation. The model receives only the SKILL.md content. The model is explicitly instructed to load only specific files referenced from SKILL.md and to avoid bulk-loading. `scripts/` files get one special behavior: implicit invocation detection (see Q11) watches for shell commands that invoke scripts inside a skill's `scripts/` directory.

SOURCE: `codex-rs/core-skills/src/render.rs`, `codex-rs/core-skills/src/invocation_utils.rs`

### Gemini CLI
On activation, `getFolderStructure` enumerates (up to 200 items) the entire skill directory tree and injects this listing into context alongside the skill body. The listing shows filenames and the folder structure but not file contents. Individual files must be read explicitly by the model. The workspace context grant (Q2) ensures all listed files are accessible.

SOURCE: `packages/core/src/tools/activate-skill.ts` (calls `getFolderStructure`), `packages/core/src/utils/getFolderStructure.ts`

---

## Q6: Path Resolution Base

**Do file paths in skills resolve relative to the skill directory, project root, or CWD?**

### Claude Code
Relative paths in SKILL.md are not resolved by the runtime — they are passed verbatim as instructions to the model. The `${CLAUDE_SKILL_DIR}` variable is available as a substitution in SKILL.md content and resolves to the skill's containing directory. This is the recommended way to reference bundled scripts or files:

> "Use this in bash injection commands to reference scripts or files bundled with the skill, regardless of the current working directory."

For `!`command`` shell injection blocks, the command runs in the CWD of the session. Scripts should be referenced using `${CLAUDE_SKILL_DIR}` to form absolute paths.

SOURCE: https://code.claude.com/docs/en/skills (section "Available string substitutions")

### Codex CLI
Relative paths in SKILL.md body are resolved relative to the skill directory by the model's own reasoning, guided by explicit instructions in the system prompt:

> "When SKILL.md references relative paths (e.g., scripts/foo.py), resolve them relative to the skill directory listed above first, and only consider other paths if needed."

The skill directory path is provided in the discovery listing (`file: <absolute_path>`). No runtime path rewriting occurs — the model is responsible for constructing absolute paths.

SOURCE: `codex-rs/core-skills/src/render.rs` (instructions block)

### Gemini CLI
The `${CLAUDE_SKILL_DIR}` equivalent in Gemini CLI is `skill.location` (the absolute path to SKILL.md). On activation, the skill directory is added to `workspaceContext` and the model receives the absolute path via the `location` field in the XML discovery listing. When `activate_skill` returns, `path.dirname(skill.location)` is used as the base. No runtime path rewriting in SKILL.md content occurs.

SOURCE: `packages/core/src/tools/activate-skill.ts` (uses `path.dirname(skill.location)`)

---

## Q7: Frontmatter Handling

**Does the agent pass YAML frontmatter to the model or strip it?**

### Claude Code
Frontmatter is **stripped** before the skill body is injected. The frontmatter fields are used by the runtime (name, description, allowed-tools, etc.) and the body (markdown content after the closing `---`) is what gets loaded into context. The docs describe them as separate: "YAML frontmatter (between `---` markers)... and markdown content with instructions."

SOURCE: https://code.claude.com/docs/en/skills (section "Write SKILL.md")

### Codex CLI
Frontmatter is **included** in what gets injected. `injection.rs` reads the full file contents with `fs::read_to_string(&skill.path_to_skills_md)` and passes the raw `contents` (including frontmatter) to `SkillInstructions { contents }`. The frontmatter is not stripped before injection.

SOURCE: `codex-rs/core-skills/src/injection.rs` (function `build_skill_injections`)

### Gemini CLI
Frontmatter is **stripped**. `skillLoader.ts` uses `FRONTMATTER_REGEX = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n([\s\S]*))?/` to parse the file. The `body` field is set to `match[2]?.trim() ?? ''` — only the content after the closing `---`. The `activate_skill` tool injects `skill.body` (the stripped body) into the `<instructions>` block.

SOURCE: `packages/core/src/skills/skillLoader.ts` (constant `FRONTMATTER_REGEX`, function `loadSkillFromFile`), `packages/core/src/tools/activate-skill.ts`

---

## Q8: Content Wrapping

**Does the agent wrap skill content in XML tags or inject as raw markdown?**

### Claude Code
Not documented. The docs do not describe an XML wrapping scheme for skill injection in the main conversation context. Content is treated as context/instructions. For subagents with preloaded skills, content is injected at startup — exact format not publicly documented.

SOURCE: Not documented in https://code.claude.com/docs/en/skills or https://code.claude.com/docs/en/sub-agents

### Codex CLI
Skill content is wrapped in `<skill>...</skill>` tags and injected as a **user-role message** (not part of the system prompt). From `fragment.rs`:
```rust
pub const SKILL_OPEN_TAG: &str = "<skill>";
pub const SKILL_CLOSE_TAG: &str = "</skill>";
pub const SKILL_FRAGMENT: ContextualUserFragmentDefinition =
    ContextualUserFragmentDefinition::new(SKILL_OPEN_TAG, SKILL_CLOSE_TAG);
```

From `user_instructions.rs`, the injected message format is:
```
<skill>
<name>{skill.name}</name>
<path>{skill.path}</path>
{skill.contents}
</skill>
```

The `into_message` call creates a `ResponseItem::Message { role: "user" }`.

The discovery listing (at session start) is wrapped in `<skills_instructions>...</skills_instructions>` tags and also placed as a user-role message. From `protocol.rs`:
```rust
pub const SKILLS_INSTRUCTIONS_OPEN_TAG: &str = "<skills_instructions>";
pub const SKILLS_INSTRUCTIONS_CLOSE_TAG: &str = "</skills_instructions>";
```

SOURCE: `codex-rs/instructions/src/fragment.rs`, `codex-rs/instructions/src/user_instructions.rs`, `codex-rs/protocol/src/protocol.rs`

### Gemini CLI
Skill content is wrapped in `<activated_skill>` XML with nested `<instructions>` and `<available_resources>` blocks, and returned as the tool result of the `activate_skill` tool call. The discovery listing is wrapped in `<available_skills>` XML injected into the system prompt. From `snippets.ts` and `activate-skill.ts`:

Discovery (system prompt):
```xml
<available_skills>
  <skill>
    <name>...</name>
    <description>...</description>
    <location>...</location>
  </skill>
</available_skills>
```

Activation (tool result):
```xml
<activated_skill name="{skillName}">
  <instructions>
    {skill.body}
  </instructions>
  <available_resources>
    {folderStructure}
  </available_resources>
</activated_skill>
```

SOURCE: `packages/core/src/prompts/snippets.ts`, `packages/core/src/tools/activate-skill.ts`

---

## Q9: Reactivation Deduplication

**If a skill activates twice in a conversation, is the content deduplicated?**

### Claude Code
Not documented. No deduplication mechanism is described in the public docs.

SOURCE: Not documented in https://code.claude.com/docs/en/skills

### Codex CLI
No deduplication. `injection.rs` (`collect_explicit_skill_mentions`) uses a `seen_paths: HashSet<PathBuf>` to track which skills have been collected in a single turn, preventing the same skill from being injected twice **within one message**. However, there is no mechanism preventing the same skill from being injected in separate turns of the same conversation — each turn processes mentions fresh.

SOURCE: `codex-rs/core-skills/src/injection.rs` (function `collect_explicit_skill_mentions`, variable `seen_paths`)

### Gemini CLI
Skills are tracked by `activeSkillNames: Set<string>` in `SkillManager`. Once a skill is activated (`activateSkill(name)` is called), `isSkillActive(name)` returns true. However, there is no guard in `activate-skill.ts` that checks `isSkillActive` before executing — the tool unconditionally re-injects on each call. Whether the model chooses to call `activate_skill` twice on the same skill is a model-level behavior, not enforced by the runtime.

SOURCE: `packages/core/src/skills/skillManager.ts` (methods `activateSkill`, `isSkillActive`), `packages/core/src/tools/activate-skill.ts`

---

## Q10: Context Compaction

**Are skill instructions protected from context window pruning/summarization?**

### Claude Code
SKILL.md content **survives compaction** when it is held in CLAUDE.md (since CLAUDE.md fully survives `/compact`). For skills loaded on-demand, the situation is less clear — skill descriptions in the session context are re-loaded from disk, but whether an activated skill's body is re-injected post-compaction is not documented.

From the memory docs: "After `/compact`, Claude re-reads your CLAUDE.md from disk and re-injects it fresh into the session."

For subagents, compaction events are separate from the parent conversation transcript and do not affect preloaded skill content.

SOURCE: https://code.claude.com/docs/en/memory (section "Instructions seem lost after /compact"), https://code.claude.com/docs/en/sub-agents (section "Subagent transcripts persist independently")

### Codex CLI
Not documented. No compaction mechanism is described for skill instructions in the public docs or source code reviewed.

SOURCE: Not documented

### Gemini CLI
Not documented. No compaction mechanism is described for skill instructions in the public docs or source code reviewed.

SOURCE: Not documented

---

## Q11: Trust Gating

**Does the agent require approval before loading project-level skills?**

### Claude Code
No approval prompt for loading. Skills are loaded automatically based on their location and frontmatter. However, skill invocation respects the standard permission system — Claude can invoke skills unless `disable-model-invocation: true` is set or the `Skill` tool is denied via permissions. No distinct "trust" check for project-level `.claude/skills/`.

SOURCE: https://code.claude.com/docs/en/skills (section "Restrict Claude's skill access")

### Codex CLI
No explicit approval for skill loading documented. The config rules system (`config_rules.rs`) allows disabling specific skills by name or path. No trust-folder mechanism exists for skills in the Rust source reviewed.

SOURCE: `codex-rs/core-skills/src/config_rules.rs`

### Gemini CLI
**Yes** — workspace skills require a trusted folder. `skillManager.ts` (`discoverSkills`) only loads workspace skills (`.gemini/skills/` and `.agents/skills/`) when `isTrusted: boolean` parameter is true:

```typescript
// skillManager.ts
if (!isTrusted) {
  debugLogger.debug('Workspace skills disabled because folder is not trusted.');
  return;
}
```

`isTrustedFolder()` in `config.ts` checks the IDE workspace trust state or falls back to the `trustedFolder` config parameter. User skills and extension skills are always loaded regardless of folder trust.

Additionally, the `activate_skill` tool shows a **consent confirmation prompt** before activating any non-builtin skill, showing the skill name, description, and resource directory. Built-in skills skip this confirmation.

SOURCE: `packages/core/src/skills/skillManager.ts` (function `discoverSkills`), `packages/core/src/config/config.ts` (function `isTrustedFolder`), `packages/core/src/tools/activate-skill.ts` (function `getConfirmationDetails`)

---

## Q12: Nested Skill Discovery

**Does it discover SKILL.md files nested inside another skill's directory?**

### Claude Code
**Yes** — explicitly documented:

> "When you work with files in subdirectories, Claude Code automatically discovers skills from nested `.claude/skills/` directories. For example, if you're editing a file in `packages/frontend/`, Claude Code also looks for skills in `packages/frontend/.claude/skills/`."

This supports monorepo setups. Discovery walks the subdirectory hierarchy and finds `.claude/skills/` directories at any depth.

SOURCE: https://code.claude.com/docs/en/skills (section "Automatic discovery from nested directories")

### Codex CLI
**Yes** — the loader uses BFS from each skill root with `MAX_SCAN_DEPTH = 6` and `MAX_SKILLS_DIRS_PER_ROOT = 2000`. `discover_skills_under_root` recursively traverses directories looking for `SKILL.md` files at any depth within the root, up to depth 6. This means a SKILL.md inside another skill's directory would be discovered as a separate skill.

SOURCE: `codex-rs/core-skills/src/loader.rs` (constants `MAX_SCAN_DEPTH`, `MAX_SKILLS_DIRS_PER_ROOT`, function `discover_skills_under_root`)

### Gemini CLI
**Yes** — `skillLoader.ts` (`loadSkillsFromDir`) uses glob patterns `['SKILL.md', '*/SKILL.md']` which only finds skills one directory deep. However, `discoverSkills` is called for multiple roots (workspace, user, `.agents/skills/`), and symlinks are followed. A SKILL.md nested directly inside another skill's directory (i.e., `skill-a/nested-skill/SKILL.md`) would **not** be found by the `*/SKILL.md` glob pattern (only one level deep). Deeper nesting is not discovered by the current glob pattern.

SOURCE: `packages/core/src/skills/skillLoader.ts` (function `loadSkillsFromDir`, pattern `['SKILL.md', '*/SKILL.md']`)

---

## Q13: Cross-Skill Invocation

**Can one skill's instructions trigger activation of another skill?**

### Claude Code
**Yes** — skill instructions can include text that triggers another skill. Since Claude reads skill content and then acts, if a skill's instructions say "use the /other-skill pattern" or if Claude judges another skill is relevant mid-execution, it can invoke additional skills. This is model-level behavior; there is no explicit cross-skill invocation API.

SOURCE: Not explicitly documented, but implied by the model-driven invocation system. https://code.claude.com/docs/en/skills

### Codex CLI
**Yes** — skills are invoked by text mention (`$skill-name`) or model judgment. If a SKILL.md body contains `$other-skill-name` references or instructions that match another skill's description, the model may activate multiple skills. The `collect_explicit_skill_mentions` function parses `$skill-name` tokens from any user input, including injected skill content. No explicit chaining depth limit for cross-skill invocation is documented.

SOURCE: `codex-rs/core-skills/src/injection.rs` (function `extract_tool_mentions_with_sigil`, sigil `$`)

### Gemini CLI
**Yes** — once activated, a skill's instructions become part of the conversation context. If those instructions reference another skill name, the model can call `activate_skill` again with the other skill's name. No guard prevents this. The consent prompt (Q11) would fire for each non-builtin activation.

SOURCE: `packages/core/src/tools/activate-skill.ts`, `packages/core/src/skills/skillManager.ts`

---

## Q14: Invocation Depth Limit

**Is there a limit on skill chaining depth?**

### Claude Code
Not documented. No explicit depth limit for skill chaining is described in the public docs.

SOURCE: Not documented in https://code.claude.com/docs/en/skills

### Codex CLI
Not documented. No explicit depth limit for skill chaining (as distinct from discovery depth) is in the source. The loader has `MAX_SCAN_DEPTH = 6` for directory traversal during discovery, but this is unrelated to invocation chaining depth.

SOURCE: Not documented. `MAX_SCAN_DEPTH` in `codex-rs/core-skills/src/loader.rs` is a discovery-time constant, not an invocation limit.

### Gemini CLI
Not documented. No explicit depth limit for skill activation chaining is in the source. The `activate_skill` tool can be called repeatedly within a single session without restriction (other than the consent prompt per activation).

SOURCE: Not documented in `packages/core/src/tools/activate-skill.ts` or `docs/cli/skills.md`

---

## Summary Table

| # | Question | Claude Code | Codex CLI | Gemini CLI |
|---|----------|-------------|-----------|------------|
| 1 | Discovery reading depth | Full file read; only description in context for inactive skills | Full file read at load; only name+description+path in system prompt | Full file read at load; only name+description+location in system prompt |
| 2 | Activation loading scope | SKILL.md body only; supporting files loaded on-demand | Full SKILL.md contents (incl. frontmatter); supporting files loaded on-demand | SKILL.md body + skill directory structure listing; supporting files accessed on-demand |
| 3 | Eager link resolution | No | No | No |
| 4 | Recognized directory set | No fixed set; any files/dirs; scripts/ and examples/ mentioned in docs | scripts/, references/, assets/, agents/ (agents/openai.yaml special-cased) | scripts/, references/, assets/ |
| 5 | Resource enumeration | Not enumerated; ignored until referenced | Not enumerated; loaded on model request | Enumerated (tree listing) on activation; contents loaded on request |
| 6 | Path resolution base | `${CLAUDE_SKILL_DIR}` variable resolves to skill dir; no runtime rewriting | Model resolves relative to skill dir path provided in discovery listing | Skill dir absolute path provided via location field; no runtime rewriting |
| 7 | Frontmatter handling | Stripped | Included (raw file contents injected) | Stripped |
| 8 | Content wrapping | Not documented | `<skill>...</skill>` XML, user-role message | `<activated_skill>` XML, as tool result |
| 9 | Reactivation deduplication | Not documented | Per-turn dedup via `seen_paths` set; cross-turn not deduped | No runtime dedup; model decides whether to re-call `activate_skill` |
| 10 | Context compaction | CLAUDE.md-based instructions survive; on-demand skill content behavior not documented | Not documented | Not documented |
| 11 | Trust gating | No approval prompt; permission system controls Skill tool access | No trust check for loading | Workspace skills require trusted folder; non-builtin activation requires consent prompt |
| 12 | Nested skill discovery | Yes, nested `.claude/skills/` dirs in subdirectories | Yes, BFS to depth 6 within each skill root | Partial — only `SKILL.md` and `*/SKILL.md` patterns (one level deep in each root) |
| 13 | Cross-skill invocation | Yes, model-driven | Yes, via `$skill-name` mentions in injected content | Yes, model can call `activate_skill` again |
| 14 | Invocation depth limit | Not documented | Not documented | Not documented |

---

## Key Behavioral Differences

**Frontmatter injection (Q7):** Codex is the outlier — it injects the full file including frontmatter. Claude Code and Gemini CLI both strip frontmatter before sending the body to the model.

**Activation scope (Q2):** Gemini CLI is the only agent that enumerates the skill's directory structure on activation and grants file-system access to the entire skill directory. Claude Code and Codex rely on the model to request files explicitly.

**Trust gating (Q11):** Gemini CLI is the only agent with a two-layer trust mechanism: folder-level trust for workspace skill discovery, plus a per-activation consent prompt for non-builtin skills. Neither Claude Code nor Codex require an approval prompt for skill loading.

**Discovery listing format (Q8):** All three agents inject a discovery listing at session start. Claude Code and Gemini CLI use XML (`<available_skills>`). Codex uses XML (`<skills_instructions>`) with a plain-text skill list inside. Activation wrapping also varies: Codex uses `<skill>` tags in user-role messages; Gemini CLI returns `<activated_skill>` as a tool result; Claude Code's exact format is not publicly documented.

**Nested discovery depth (Q12):** Codex supports BFS to depth 6. Claude Code supports arbitrary depth via nested `.claude/skills/` directories as you descend the file tree. Gemini CLI only finds skills one level deep (the `*/SKILL.md` glob pattern) within each configured root.
