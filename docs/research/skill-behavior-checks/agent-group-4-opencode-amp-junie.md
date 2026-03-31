# Runtime Skill Loading Behavior: OpenCode, Amp, JetBrains Junie CLI

Research date: 2026-03-30  
Researcher: Maive (Claude Sonnet 4.6)

## Sources

| Agent | Primary Sources |
|-------|----------------|
| OpenCode | https://opencode.ai/docs/skills (official docs), `packages/opencode/src/skill/index.ts`, `packages/opencode/src/skill/discovery.ts`, `packages/opencode/src/tool/skill.ts`, `packages/opencode/src/config/markdown.ts` (github.com/sst/opencode) |
| Amp | https://ampcode.com/manual (auth-gated, could not fetch), `amp-extractions/agents/agent-tools.md`, `amp-extractions/agents/smart-system-prompt.md`, `amp-extractions/config/settings.md` (github.com/ben-vargas/ai-amp-cli — extracted from Amp CLI v0.0.1770366910), `github.com/vercel-labs/agent-skills` (official Vercel skills repo), `github.com/JokerRun/amp-skills` (community skills repo) |
| Junie CLI | https://junie.jetbrains.com/docs/agent-skills.html, https://junie.jetbrains.com/docs/guidelines-and-memory.html, https://junie.jetbrains.com/docs/junie-cli-subagents.html (JetBrains official docs) |

**Important caveat on Amp:** The ampcode.com/manual requires authenticated login and could not be fetched directly. Amp behavior is documented from: (a) source code extraction from `node_modules/@sourcegraph/amp/dist/main.js` (published in ben-vargas/ai-amp-cli), (b) official Vercel skill repo (vercel-labs/agent-skills) best-practices docs, and (c) the public `/manual` page overview retrieved before the auth redirect.

---

## Q1: Discovery Reading Depth

**Does the agent read only SKILL.md frontmatter at discovery, or the entire file? Does it load inactive skills into context?**

### OpenCode

At discovery time, OpenCode reads the **entire SKILL.md file** using `ConfigMarkdown.parse()`, which calls `gray-matter` on the full file content. Both the frontmatter (`name`, `description`) and the body (`md.content`) are stored in the `Skill.Info` object:

```typescript
state.skills[parsed.data.name] = {
  name: parsed.data.name,
  description: parsed.data.description,
  location: match,
  content: md.content,  // full body, stored at discovery time
}
```

However, only the **name and description** are surfaced to the model for inactive skills. The `Skill.fmt()` function produces a listing like `- **name**: description` that appears in the skill tool's description. The full body (`content`) is stored in memory but not injected into the conversation until the skill tool is explicitly called.

SOURCE: `packages/opencode/src/skill/index.ts` (github.com/sst/opencode, lines ~add() function and fmt() function)

### Amp

At discovery, Amp loads only the **name and description** (frontmatter fields) of each skill. The full `SKILL.md` content is not read until activation. The Vercel agent-skills best-practices document explicitly states:

> "Skills are loaded on-demand — only the skill name and description are loaded at startup. The full `SKILL.md` loads into context only when the agent decides the skill is relevant."

Inactive skills therefore only occupy context via their name+description summary in the skill tool's parameter description block.

SOURCE: `github.com/vercel-labs/agent-skills` — `AGENTS.md` ("Best Practices for Context Efficiency" section)

### Junie CLI

Junie uses **progressive disclosure**: only name and description are read at discovery; the full body is not loaded until the skill is determined relevant. The official documentation states:

> "Each skill's name and description are available to Junie, so it knows what skills exist, but doesn't read the full content of a skill until it determines its relevance to the task."

Inactive skills do not enter context.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html

---

## Q2: Activation Loading Scope

**When a skill activates, does it load only SKILL.md body or also files from the skill directory (references/, scripts/, etc.)?**

### OpenCode

On activation, OpenCode loads:
1. The SKILL.md body (the `content` field stored at discovery, which is the post-frontmatter markdown)
2. A **file listing** of up to 10 non-SKILL.md files in the skill directory, enumerated via `ripgrep --files`. These are listed inside `<skill_files>` tags as absolute paths.

The file listing tells the model what resources exist, but the actual file contents are **not automatically read** — the model must use the Read/file tools to access them. The full output block is:

```
<skill_content name="skill-name">
# Skill: skill-name

[SKILL.md body]

Base directory for this skill: file:///path/to/skill
Relative paths in this skill (e.g., scripts/, reference/) are relative to this base directory.
Note: file list is sampled.

<skill_files>
<file>/absolute/path/to/file1</file>
...
</skill_files>
</skill_content>
```

SOURCE: `packages/opencode/src/tool/skill.ts` (github.com/sst/opencode, execute() function)

### Amp

On activation, the skill tool injects "detailed instructions, workflows, and access to bundled resources into the conversation context." The tool description says:

> "The skill will inject detailed instructions, workflows, and access to bundled resources (scripts, templates, etc.) into the conversation context."

Based on the Vercel best-practices guidance, the body is loaded on activation. The best-practices advice to "use progressive disclosure — reference supporting files that get read only when needed" and "file references work one level deep" implies that sub-directory files are **not automatically loaded** — they are referenced by path in SKILL.md and the model reads them on demand.

The `/mnt/skills/user/{skill-name}/scripts/{script}.sh` path pattern in official Vercel skill examples indicates that scripts are referenced by absolute path and executed when needed, not pre-loaded.

SOURCE: `github.com/ben-vargas/ai-amp-cli` — `amp-extractions/agents/agent-tools.md` (tool #46 description); `github.com/vercel-labs/agent-skills` — `AGENTS.md`

### Junie CLI

On activation, Junie loads the SKILL.md body and "follows the skill instructions, loading referenced materials or executing bundled scripts as needed." The documentation states:

> "The body (everything after the closing `---`) is the main skill documentation, which should contain actionable instructions that Junie CLI should follow along with the paths to relevant project files, templates, or additional materials within the skill folder."

This indicates that sub-directory files (`scripts/`, `templates/`, `checklists/`) are not automatically loaded — they are referenced within SKILL.md and Junie accesses them as the instructions direct.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html

---

## Q3: Eager Link Resolution

**Does it follow markdown links in SKILL.md and pre-fetch linked files?**

### OpenCode

No evidence of eager link resolution. OpenCode enumerates up to 10 files in the skill directory and provides their absolute paths in `<skill_files>` tags, but does not follow markdown links or pre-fetch them. The model must actively use the Read tool.

SOURCE: `packages/opencode/src/tool/skill.ts` (no link-following logic present)

### Amp

No. The Vercel best-practices guidance explicitly says "file references work one level deep — link directly from SKILL.md to supporting files." This describes a manual reference pattern (model reads files on demand), not pre-fetching. No eager link resolution is documented.

SOURCE: `github.com/vercel-labs/agent-skills` — `AGENTS.md`

### Junie CLI

Not documented. No mention of eager link resolution in official documentation.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of documentation)

---

## Q4: Recognized Directory Set

**What directories inside a skill folder does the agent recognize?**

### OpenCode

OpenCode does not recognize any named subdirectories specially. The skill tool enumerates **all non-SKILL.md files** in the skill directory (up to 10, sampled) using `ripgrep --files` with `hidden: true`. Any files present, in any subdirectory, appear in the `<skill_files>` listing. No directory name has special semantic meaning.

SOURCE: `packages/opencode/src/tool/skill.ts` (Ripgrep.files call with no directory filtering)

### Amp

No named subdirectories are given special semantic treatment by the runtime. The Vercel-maintained skills use `scripts/` (for executable scripts referenced as `/mnt/skills/user/{skill}/scripts/{script}.sh`) and some use `resources/` (e.g., `deploy-to-vercel/resources/`). The community amp-skills AGENTS.md recommends `scripts/` as the canonical name. These are conventions, not runtime constraints — the agent reads them only when instructed to by SKILL.md content.

SOURCE: `github.com/vercel-labs/agent-skills` — directory listings and `AGENTS.md`

### Junie CLI

The official documentation lists the standard layout as:
- `scripts/` — optional executable scripts
- `templates/` — optional code or documentation templates
- `checklists/` — optional verification lists

These are documented as conventions, not enforced by the runtime. No special automatic loading behavior is specified for any of them.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html ("File Structure Recognition" section)

---

## Q5: Resource Enumeration

**How does it handle multiple files in a skill's subdirectories? Enumerate? Load all? Ignore until requested?**

### OpenCode

Enumerates up to 10 files (sampled, not exhaustive). The code explicitly caps at `limit = 10` and notes "file list is sampled." Files are listed as absolute paths in `<skill_files>` tags. Content is not loaded — the model must read them explicitly.

SOURCE: `packages/opencode/src/tool/skill.ts` (limit = 10, "Note: file list is sampled." string)

### Amp

Not enumerated automatically. The model accesses files referenced in SKILL.md content by reading them on demand. Best-practices guidance explicitly recommends referencing files rather than auto-loading them: "Use progressive disclosure — reference supporting files that get read only when needed."

SOURCE: `github.com/vercel-labs/agent-skills` — `AGENTS.md`

### Junie CLI

Ignored until requested via SKILL.md instructions. Files are only loaded or executed when Junie follows explicit instructions in the SKILL.md body pointing to them.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html

---

## Q6: Path Resolution Base

**Do file paths in skills resolve relative to the skill directory, project root, or CWD?**

### OpenCode

Explicitly relative to the **skill directory**. The tool output includes:

```
Base directory for this skill: file:///path/to/skill/dir
Relative paths in this skill (e.g., scripts/, reference/) are relative to this base directory.
```

The base URL is computed as `pathToFileURL(path.dirname(skill.location)).href`.

SOURCE: `packages/opencode/src/tool/skill.ts` (base variable and output string)

### Amp

Relative to the **skill directory**, mounted at `/mnt/skills/user/{skill-name}/`. Official Vercel skill scripts use the pattern `bash /mnt/skills/user/{skill-name}/scripts/{script}.sh`, confirming that the skill directory is the resolution root. The `amp.skills.path` config setting accepts absolute paths or `~`-prefixed paths to custom skill directories.

SOURCE: `github.com/vercel-labs/agent-skills` — `AGENTS.md` (script path pattern); `github.com/ben-vargas/ai-amp-cli` — `amp-extractions/config/settings.md`

### Junie CLI

Not explicitly documented. The documentation describes paths as relative (e.g., `checklists/review.md`, `scripts/setup.sh`) but does not specify whether they resolve relative to the skill directory or project root. Based on structural conventions with other agents in this research, skill directory is the likely base, but this is not confirmed in the docs.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of explicit documentation)

---

## Q7: Frontmatter Handling

**Does the agent pass YAML frontmatter to the model or strip it?**

### OpenCode

**Stripped.** `ConfigMarkdown.parse()` calls `gray-matter`, which separates frontmatter into `.data` and body into `.content`. Only `md.content` (the post-frontmatter body) is stored in `Skill.Info.content` and injected into the conversation. Frontmatter fields are stored separately as structured data (only `name` and `description` are used; all other fields are ignored).

SOURCE: `packages/opencode/src/skill/index.ts` (the `add()` function stores `content: md.content`) and `packages/opencode/src/config/markdown.ts` (`gray-matter` usage)

### Amp

Not explicitly documented. Based on the Vercel best-practices AGENTS.md, the description says "The full `SKILL.md` loads into context" on activation, which could imply the complete file including frontmatter. However, standard practice for skill runners is to strip frontmatter before injection. No definitive source available due to auth-gating on official docs.

SOURCE: Not documented (ampcode.com/manual auth-gated)

### Junie CLI

Not explicitly documented. The documentation only describes the frontmatter fields (`name`, `description`) as metadata used for matching, and the body as "the main skill documentation." This implies stripping, but the implementation detail is not confirmed in official docs.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of explicit documentation)

---

## Q8: Content Wrapping

**Does the agent wrap skill content in XML tags or inject as raw markdown?**

### OpenCode

**XML-wrapped.** The skill content is wrapped in `<skill_content name="...">` tags. The full output structure is:

```xml
<skill_content name="skill-name">
# Skill: skill-name

[SKILL.md body]

Base directory for this skill: file:///...
Relative paths in this skill (e.g., scripts/, reference/) are relative to this base directory.
Note: file list is sampled.

<skill_files>
<file>/absolute/path/to/file</file>
</skill_files>
</skill_content>
```

The skill tool description also explicitly advertises this: "Tool output includes a `<skill_content name="...">` block with the loaded content."

The skill listing for inactive skills also uses XML in verbose mode:
```xml
<available_skills>
  <skill>
    <name>...</name>
    <description>...</description>
    <location>file:///.../SKILL.md</location>
  </skill>
</available_skills>
```

SOURCE: `packages/opencode/src/tool/skill.ts` and `packages/opencode/src/skill/index.ts` (fmt() function)

### Amp

Not explicitly documented due to auth-gating. The tool description says the skill "will inject detailed instructions, workflows, and access to bundled resources into the conversation context" but does not specify the wrapping format.

SOURCE: `github.com/ben-vargas/ai-amp-cli` — `amp-extractions/agents/agent-tools.md` (tool #46 description); official format undocumented

### Junie CLI

Not documented. No mention of XML tags or content wrapping format in official documentation.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of documentation)

---

## Q9: Reactivation Deduplication

**If a skill activates twice in a conversation, is the content deduplicated?**

### OpenCode

No deduplication mechanism is present in the source code. The skill tool can be called multiple times with the same name — each call returns the full `<skill_content>` block again. A warning is logged during discovery if two different SKILL.md files declare the same `name` (`log.warn("duplicate skill name", ...)`), but there is no runtime deduplication for repeated activations.

SOURCE: `packages/opencode/src/skill/index.ts` (duplicate name warning only; no activation tracking); `packages/opencode/src/tool/skill.ts` (no deduplication logic)

### Amp

Not documented.

SOURCE: Not documented (ampcode.com/manual auth-gated)

### Junie CLI

Not documented.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of documentation)

---

## Q10: Context Compaction

**Are skill instructions protected from context window pruning/summarization?**

### OpenCode

No skill-specific protection from compaction. The compaction prompt (`packages/opencode/src/agent/prompt/compaction.txt`) is a generic summarization instruction with no skill-awareness — it does not pin or protect skill content. The `<skill_content>` blocks in the conversation would be subject to summarization like any other tool output.

SOURCE: `packages/opencode/src/agent/prompt/compaction.txt`

### Amp

Not documented. Amp has `amp.experimental.compaction` as a boolean/number setting (default: false), suggesting compaction is not yet standard behavior. No skill-protection mechanism is documented.

SOURCE: `github.com/ben-vargas/ai-amp-cli` — `amp-extractions/config/settings.md` (compaction setting)

### Junie CLI

Not documented.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html (absence of documentation)

---

## Q11: Trust Gating

**Does the agent require approval before loading project-level skills?**

### OpenCode

**Yes, configurable.** OpenCode has a permission system that gates skill loading via `ctx.ask({ permission: "skill", patterns: [skillName] })` in the tool's `execute()` method. The default permission config (`opencode.json`) supports pattern-based rules:

```json
{
  "permission": {
    "skill": {
      "*": "allow",
      "internal-*": "deny",
      "experimental-*": "ask"
    }
  }
}
```

Patterns can be `"allow"`, `"deny"`, or `"ask"` (prompts the user). The `skill` tool itself can also be disabled entirely. The default behavior depends on configuration — if no skill permission rules are set, the default from the agent configuration applies (the `build` agent uses `"*": "allow"` for most permissions but skill-specific defaults are configurable).

SOURCE: `packages/opencode/src/tool/skill.ts` (ctx.ask call); https://opencode.ai/docs/skills (permission section)

### Amp

No approval step for skill loading is documented. Skills are invoked by the agent automatically based on task matching. The `amp.dangerouslyAllowAll` flag controls command execution approval broadly, but no skill-specific gating is mentioned.

SOURCE: `github.com/ben-vargas/ai-amp-cli` — `amp-extractions/agents/agent-tools.md`; absence of documentation

### Junie CLI

No automatic approval step is documented, but the documentation warns users to exercise caution:

> "Junie modifies your code and executes scripts, so it's important to make sure that the skills it uses are safe. Treat third-party skills with the same caution as you would with any third-party code you add to your project."

The trust model is advisory (user vets skills before installing), not runtime-enforced (no approval dialog when a skill loads).

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html

---

## Q12: Nested Skill Discovery

**Does it discover SKILL.md files nested inside another skill's directory?**

### OpenCode

**Yes, by pattern.** The scan pattern `skills/**/SKILL.md` (for `.claude` and `.agents` directories) uses `**` glob which would match nested SKILL.md files. However, the pattern for OpenCode's own config dirs is `{skill,skills}/**/SKILL.md`. The discovered SKILL.md files are stored by `name` key, so a nested skill with a unique name would be registered. There is no explicit exclusion of nested skills.

SOURCE: `packages/opencode/src/skill/index.ts` (EXTERNAL_SKILL_PATTERN = "skills/**/SKILL.md")

### Amp

Not documented. No mention of nested skill discovery in available sources.

SOURCE: Not documented

### Junie CLI

Not documented. The documentation describes discovery as scanning `<projectRoot>/.junie/skills/<skill-name>/` directories (one level deep), which would not pick up nested SKILL.md files inside a skill's subdirectories.

SOURCE: https://junie.jetbrains.com/docs/agent-skills.html

---

## Q13: Cross-Skill Invocation

**Can one skill's instructions trigger activation of another skill?**

### OpenCode

**Technically yes.** Skills are loaded as tool calls. Since the model has access to the `skill` tool during a session, a skill's body instructions could include text like "If you need X, use the `y-skill` skill." The model could then call `skill({ name: "y-skill" })`. There is no runtime restriction preventing this. No formal "cross-skill invocation" feature is documented — it would be emergent behavior from the model following instructions.

SOURCE: `packages/opencode/src/tool/skill.ts` (no restriction on skill-calling-skill); reasoning from runtime architecture

### Amp

Not documented. Same emergent-behavior reasoning applies — if the model has the `skill` tool available and SKILL.md text instructs it to call another skill, it may do so.

SOURCE: Not documented

### Junie CLI

Not documented. Subagent frontmatter supports a `skills` field that lists skill names to load, which is a form of cross-entity skill invocation, but this applies to subagent definitions, not skill-to-skill invocation.

SOURCE: https://junie.jetbrains.com/docs/junie-cli-subagents.html

---

## Q14: Invocation Depth Limit

**Is there a limit on skill chaining depth?**

### OpenCode

Not documented. No depth limit on skill invocation is present in the source code. The `skill.ts` tool has no recursion tracking or depth counter.

SOURCE: `packages/opencode/src/tool/skill.ts` (absence of depth limiting)

### Amp

Not documented.

SOURCE: Not documented

### Junie CLI

Not documented.

SOURCE: Not documented

---

## Summary Matrix

| Question | OpenCode | Amp | Junie CLI |
|----------|----------|-----|-----------|
| Q1: Discovery reads full file? | Yes (full file read, name+desc surfaced to model) | No (name+desc only) | No (name+desc only) |
| Q1: Inactive skills in context? | Name+desc only, not full body | Name+desc only | Name+desc only |
| Q2: Activation loads sub-dirs? | File listing (up to 10, paths only) | Not auto-loaded; referenced by path | Not auto-loaded; referenced by SKILL.md instructions |
| Q3: Eager link resolution? | No | No | Not documented |
| Q4: Recognized directories? | Any file, no named dirs special | `scripts/` (convention) | `scripts/`, `templates/`, `checklists/` (conventions) |
| Q5: Resource enumeration? | Enumerate up to 10 (sampled, paths only) | Not enumerated | Not enumerated |
| Q6: Path resolution base? | Skill directory (explicit) | Skill directory (`/mnt/skills/user/<name>/`) | Not documented |
| Q7: Frontmatter stripped? | Yes (gray-matter strips) | Not documented | Not documented |
| Q8: Content wrapping? | `<skill_content name="...">` XML | Not documented | Not documented |
| Q9: Reactivation dedup? | No | Not documented | Not documented |
| Q10: Context compaction protection? | No | Not documented | Not documented |
| Q11: Trust gating? | Yes (configurable permission system) | No | Advisory only (user vets before install) |
| Q12: Nested skill discovery? | Yes (`**` glob pattern) | Not documented | No (one level deep implied) |
| Q13: Cross-skill invocation? | Technically yes (emergent) | Not documented | Via subagent `skills` field only |
| Q14: Invocation depth limit? | Not documented | Not documented | Not documented |
