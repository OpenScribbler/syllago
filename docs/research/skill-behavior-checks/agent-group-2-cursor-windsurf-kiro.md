# Runtime Skill Loading Behavior: Cursor, Windsurf, Kiro

**Research date:** 2026-03-30
**Researcher:** Maive (Claude Sonnet 4.6)

## Methodology

Primary sources fetched for each agent:
- **Cursor:** https://cursor.com/docs/rules, https://cursor.com/docs/skills
- **Windsurf:** https://docs.windsurf.com/windsurf/cascade/skills, https://docs.windsurf.com/windsurf/cascade/memories, https://docs.windsurf.com/windsurf/cascade/agents-md
- **Kiro:** https://kiro.dev/docs/steering/

Findings are scoped to what these official sources explicitly state. Where a question is not addressed by any source, the answer is "Not documented."

---

## Cursor

Cursor's equivalent of "skills" is split across two systems: **Rules** (`.cursor/rules/*.md` or `.mdc` files) and **Skills** (`.cursor/skills/<skill-name>/SKILL.md`). Both are documented. Questions are answered for whichever system is relevant.

### 1. Discovery Reading Depth

**Rules:** At discovery, Cursor reads the frontmatter (`description`, `globs`, `alwaysApply`) to determine rule type. The documentation does not explicitly state whether the full file body is read at discovery or deferred.

**Skills:** "When Cursor starts, it automatically discovers skills from skill directories and makes them available to Agent." The docs state "Skills load resources on demand, keeping context usage efficient" and "agents load resources progressively—only when needed." This strongly implies only `name` and `description` from frontmatter are read at discovery, not the full body.

Inactive skills (those not yet invoked) do not load into context based on the progressive loading description.

**Source:** https://cursor.com/docs/skills

### 2. Activation Loading Scope

**Rules:** When a rule activates, "rule contents are included at the start of the model context." No mention of loading sibling files.

**Skills:** When a skill activates, the full `SKILL.md` body loads. The spec lists optional subdirectories (`scripts/`, `references/`, `assets/`). "Reference scripts in your `SKILL.md` using relative paths from the skill root." This implies scripts are referenced by path and executed when invoked, not bulk-loaded into context. `references/` files are described as "loaded on demand."

The documentation does not confirm that all files in subdirectories are injected as context — only that scripts can be executed and references loaded on demand.

**Source:** https://cursor.com/docs/skills

### 3. Eager Link Resolution

**Rules:** Rules can reference files using `@filename.ts` syntax. The documentation does not describe whether linked files are pre-fetched at rule discovery or lazily resolved at invocation time.

**Skills:** Not documented.

**Source:** https://cursor.com/docs/rules (FAQ section: "Can rules reference other rules or files? Yes. Use `@filename.ts` syntax.")

### 4. Recognized Directory Set

**Skills:** Three optional directories are explicitly listed:
- `scripts/` — "Executable code that agents can run"
- `references/` — Additional documentation loaded on demand
- `assets/` — Templates, images, or data files

**Source:** https://cursor.com/docs/skills

### 5. Resource Enumeration

For `references/`: described as "loaded on demand." For `scripts/`: agents execute referenced scripts when invoked (via explicit path references in `SKILL.md`). For `assets/`: not specified.

The documentation does not describe bulk enumeration of subdirectory contents. Resources appear to be referenced explicitly in `SKILL.md` rather than auto-enumerated.

**Source:** https://cursor.com/docs/skills

### 6. Path Resolution Base

**Skills:** "Reference scripts in your `SKILL.md` using relative paths from the skill root." Path resolution base is the skill directory (skill root), not project root or CWD.

**Source:** https://cursor.com/docs/skills

### 7. Frontmatter Handling

Not documented. The documentation shows frontmatter structure for both rules and skills but does not specify whether frontmatter is stripped before injection or passed intact to the model.

**Source:** https://cursor.com/docs/rules, https://cursor.com/docs/skills

### 8. Content Wrapping

Not documented. No mention of XML tags or special wrapping. Rules are described as injected "at the start of the model context" as plain markdown.

**Source:** https://cursor.com/docs/rules

### 9. Reactivation Deduplication

Not documented.

### 10. Context Compaction

Not documented. No mention of rules or skills being protected from context window pruning or summarization.

### 11. Trust Gating

Not documented for project-level skills or rules. Team Rules support enforcement (admin can prevent users from disabling them), but no approval-before-load mechanism is described for project-level content.

**Source:** https://cursor.com/docs/rules (Team Rules section)

### 12. Nested Skill Discovery

**Rules / AGENTS.md:** "Cursor supports AGENTS.md in the project root and subdirectories." Nested `AGENTS.md` files are "automatically applied when working with files in that directory or its children" with more specific instructions taking precedence.

**Skills:** No documentation on nested skill discovery (i.e., a `SKILL.md` inside another skill's subdirectory). The spec places skills flat in `.cursor/skills/<skill-name>/`.

**Source:** https://cursor.com/docs/rules (Nested AGENTS.md support section)

### 13. Cross-Skill Invocation

Not documented for skills. For rules, the FAQ states rules can reference other files via `@filename.ts` but does not confirm rule-to-rule invocation. No skill-triggers-skill mechanism is described.

**Source:** https://cursor.com/docs/rules

### 14. Invocation Depth Limit

Not documented.

---

## Windsurf

Windsurf calls its system **Skills** and **Rules** (plus **Memories**). Skills live in `.windsurf/skills/<skill-name>/SKILL.md`. Rules live in `.windsurf/rules/`. Both are managed by the Cascade engine.

### 1. Discovery Reading Depth

The documentation explicitly states the progressive disclosure model: "only the skill's `name` and `description` are shown to the model by default. The full `SKILL.md` content and supporting files are loaded **only when Cascade decides to invoke the skill**."

Inactive skills are not loaded into context — only their `name` and `description` are surfaced.

**Source:** https://docs.windsurf.com/windsurf/cascade/skills

### 2. Activation Loading Scope

When a skill activates, "the skill's complete `SKILL.md` file and all supporting resources in the skill folder become available." Supporting files are described as "placed in the skill folder alongside `SKILL.md`."

No specific subdirectory structure (`scripts/`, `references/`, `assets/`) is mandated or mentioned. All files in the skill folder load when the skill is invoked.

**Source:** https://docs.windsurf.com/windsurf/cascade/skills

### 3. Eager Link Resolution

Not documented.

### 4. Recognized Directory Set

Not documented. The Windsurf skills spec does not enumerate specific subdirectory names like `scripts/` or `references/`. It only says "supporting files" are placed "in the skill folder alongside `SKILL.md`."

**Source:** https://docs.windsurf.com/windsurf/cascade/skills

### 5. Resource Enumeration

The documentation states all supporting resources in the skill folder "become available" when a skill is invoked, implying all files in the skill directory are loaded (not selectively enumerated). No selective loading or on-demand mechanism is described.

**Source:** https://docs.windsurf.com/windsurf/cascade/skills

### 6. Path Resolution Base

Not documented.

### 7. Frontmatter Handling

Not documented. Required frontmatter fields (`name`, `description`) are described but whether they are stripped before the model sees the content is not specified.

**Source:** https://docs.windsurf.com/windsurf/cascade/skills

### 8. Content Wrapping

Not documented.

### 9. Reactivation Deduplication

Not documented.

### 10. Context Compaction

Not documented.

### 11. Trust Gating

Not documented. No approval-before-load mechanism is described for project-level skills or rules.

### 12. Nested Skill Discovery

Not documented for skills. For AGENTS.md: "All `AGENTS.md` files within your workspace and its subdirectories are discovered." The system also "searches parent directories up to the git root" in git repositories. Subdirectory `AGENTS.md` files are scoped using glob patterns (e.g., `/frontend/AGENTS.md` applies to `/frontend/**`).

**Source:** https://docs.windsurf.com/windsurf/cascade/agents-md

### 13. Cross-Skill Invocation

Not documented.

### 14. Invocation Depth Limit

Not documented.

---

## Kiro

**CORRECTION (validated 2026-03-30):** Kiro has TWO separate systems: **Steering** (`.kiro/steering/`, single files) AND **Skills** (`.kiro/skills/`, SKILL.md directories following the agentskills.io spec). The original research only checked `kiro.dev/docs/steering/` and missed `kiro.dev/docs/skills/`. The answers below cover Kiro's Steering system only. See the behavior matrix (`00-behavior-matrix.md`) for corrected Kiro skills data sourced from `kiro.dev/docs/skills/` and the agentskills.io specification/implementation guide.

Kiro's Steering system uses markdown files in `.kiro/steering/` with YAML frontmatter controlling inclusion mode. Each steering file is a single `.md` file, not a directory with supporting files. Kiro's Skills system uses the standard SKILL.md directory model with `scripts/`, `references/`, `assets/` conventions.

### 1. Discovery Reading Depth

Kiro discovers steering files by scanning `.kiro/steering/` (workspace) and `~/.kiro/steering/` (global). Inclusion mode frontmatter determines when files load:

- `inclusion: always` — loaded in every interaction automatically
- `inclusion: fileMatch` — loaded when working with files matching glob patterns
- `inclusion: auto` — "Kiro uses the description to decide when the steering file is relevant"
- `inclusion: manual` — loaded only when explicitly referenced via `#filename` in chat

For `auto` mode, only the `description` field is used to decide relevance, implying the full file is not read until the steering file is selected. For `always` and `fileMatch`, the full content loads when the condition is met.

The documentation does not state whether the full file body is read at discovery time or only at activation.

**Source:** https://kiro.dev/docs/steering/

### 2. Activation Loading Scope

Kiro steering files are single markdown files, not directories. There is no concept of a skill directory with supporting files like `scripts/` or `references/`. Steering files can reference workspace files using `#[[file:<relative_file_name>]]` syntax, but this is an inline reference to workspace files, not a bundled directory.

**Source:** https://kiro.dev/docs/steering/

### 3. Eager Link Resolution

The `#[[file:path/to/file]]` syntax links to workspace files. The documentation does not specify whether referenced files are pre-fetched eagerly at steering file load time or resolved lazily when the model encounters the reference. This is explicitly not documented.

**Source:** https://kiro.dev/docs/steering/ (File references section)

### 4. Recognized Directory Set

Not applicable. Kiro steering uses single files, not skill directories. There is no bundled directory structure.

**Source:** https://kiro.dev/docs/steering/

### 5. Resource Enumeration

Not applicable. See Q4 — Kiro steering does not have a directory-based model.

### 6. Path Resolution Base

For `#[[file:]]` references: the documentation shows examples like `#[[file:api/openapi.yaml]]` and `#[[file:components/ui/button.tsx]]`, which appear to be relative to the workspace root. Not explicitly stated.

**Source:** https://kiro.dev/docs/steering/ (File references section)

### 7. Frontmatter Handling

Not documented. Frontmatter fields (`inclusion`, `fileMatchPattern`, `name`, `description`) are defined but whether they are stripped before the model sees the steering file content is not specified.

**Source:** https://kiro.dev/docs/steering/

### 8. Content Wrapping

Not documented.

### 9. Reactivation Deduplication

When conflicts arise between global and workspace steering, "Kiro will prioritize the workspace steering instructions." This addresses priority, not deduplication of identical content. Deduplication of repeated injection within a conversation is not documented.

**Source:** https://kiro.dev/docs/steering/

### 10. Context Compaction

Not documented.

### 11. Trust Gating

Not documented for steering file loading. The privacy and security docs describe an approval workflow for terminal command execution ("By default, Kiro requires approval before running any command"), but no equivalent approval mechanism is described for loading steering files.

**Source:** https://kiro.dev/docs/privacy-and-security/

### 12. Nested Skill Discovery

Kiro discovers steering files only in the flat `.kiro/steering/` directory (workspace) and `~/.kiro/steering/` (global). No subdirectory scanning within `.kiro/steering/` is mentioned.

For `AGENTS.md`: Kiro automatically picks up `AGENTS.md` files placed in `~/.kiro/steering/` or workspace root. AGENTS.md files do not support inclusion modes and are always included.

**Source:** https://kiro.dev/docs/steering/

### 13. Cross-Skill Invocation

Not applicable in the same way. Steering files can use `#[[file:]]` references to pull in other workspace files, but there is no described mechanism for one steering file to trigger another steering file's inclusion.

**Source:** https://kiro.dev/docs/steering/

### 14. Invocation Depth Limit

Not documented.

---

## Summary Table

| Question | Cursor | Windsurf | Kiro |
|----------|--------|----------|------|
| Discovery reading depth | Frontmatter only for skills (progressive); rules unclear | Frontmatter only (name + description) | Frontmatter; full body deferred for `auto` mode |
| Inactive skills load into context | No (progressive loading) | No (progressive disclosure) | No (deferred) |
| Activation loading scope | SKILL.md body + on-demand references; scripts executed via path | Full SKILL.md + all files in skill folder | Single steering file only; no bundled directory |
| Eager link resolution | Not documented | Not documented | Not documented |
| Recognized directory set | `scripts/`, `references/`, `assets/` | Not enumerated — all files in skill folder | Not applicable (single-file model) |
| Resource enumeration | References: on demand; scripts: explicit paths | All files in skill folder loaded on activation | Not applicable |
| Path resolution base | Relative to skill root | Not documented | Workspace root (implied by examples) |
| Frontmatter handling | Not documented | Not documented | Not documented |
| Content wrapping | Not documented | Not documented | Not documented |
| Reactivation deduplication | Not documented | Not documented | Not documented |
| Context compaction | Not documented | Not documented | Not documented |
| Trust gating | Not documented | Not documented | Not documented |
| Nested skill discovery | AGENTS.md: yes, subdirectory scoped. Skills: not documented | AGENTS.md: yes, full subdirectory scan. Skills: not documented | Not documented for steering; AGENTS.md at root only |
| Cross-skill invocation | Not documented | Not documented | Not applicable (no skill-triggers-skill) |
| Invocation depth limit | Not documented | Not documented | Not documented |

---

## Key Findings

**Structural divergence:** Kiro uses a fundamentally different model — single markdown files in a flat directory, not a SKILL.md + bundled directory pattern. This means questions about subdirectory handling, recognized directory sets, and resource enumeration simply don't apply to Kiro.

**Progressive loading:** Both Cursor and Windsurf explicitly document progressive/lazy loading — only `name` and `description` are surfaced at discovery, full content loads on invocation. Kiro implies something similar for `auto` mode but is less explicit.

**Windsurf loads all files on activation:** Windsurf is the only agent that explicitly states all files in the skill folder load when a skill is invoked. Cursor is more selective — references are "on demand" and scripts are executed via explicit paths.

**Large documentation gap:** Questions 7–10 (frontmatter stripping, content wrapping, deduplication, context compaction), question 11 (trust gating for content loading), and questions 13–14 (cross-skill invocation, chaining limits) are uniformly undocumented across all three agents. These are implementation details not exposed in user-facing documentation.

**Cursor's recognized directories:** Cursor is the only agent to explicitly name and describe subdirectory conventions (`scripts/`, `references/`, `assets/`). This reflects its role as the originator of the Agent Skills standard that syllago's format is modeled on.
