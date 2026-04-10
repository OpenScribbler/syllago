# Skill Optimization Template

Reusable template for condensing skills (language-patterns, review-standards, IaC-patterns, etc.) to minimize token consumption while preserving usefulness.

---

## Rule-Statement Format

Replace verbose Bad/Good/Why code blocks with condensed rule statements:

```markdown
### Pattern Name
**Severity**: high | **Category**: concurrency

- Rule: [1-2 sentence description of what to do/avoid]
- Gotcha: [subtle detail Claude might miss, if any]
- Fix: [concrete action to take, if not obvious from rule]
```

### When to Include a Code Snippet (5-10 lines max, "Good" only)

- Pattern requires non-obvious syntax (e.g., iterator protocol, functional options)
- Correct approach has subtle ordering/structure requirements
- Project-specific convention not in Claude's training data
- **IaC/HCL note**: Terraform and similar IaC skills need more code snippets than language skills because HCL block syntax is less intuitive, features ship rapidly (training data lags), and provider-specific syntax varies significantly
- **Multi-provider/multi-cloud content**: Use one canonical YAML/HCL example + a provider summary table instead of repeating near-identical blocks per provider. Only the config keys differ — a table captures the differences more efficiently than 4 YAML blocks

### When to Omit Code (Rule Statement Only)

- Standard idiom (table-driven tests, strings.Builder, interface composition)
- Pattern is a "don't do X" rule where the alternative is obvious
- Claude reliably produces the correct pattern without a reminder
- **Test code examples**: Describe WHAT to test (cases, payloads, assertions, gotchas), not HOW to write the test function. Claude knows test structure — it needs the domain-specific test cases and expected behaviors

### Review Skills: Additional Content Types

Review-focused skills (code-review-standards, security-audit) have content types that need different treatment than language patterns:

- **Decision trees** (flag vs don't-flag): Condense to 2-line rule + "Flag only if:" condition. Preserve the real-issue vs non-issue distinction — this is the core value of review skills.
- **Verification checklists**: Behavioral instructions ("check X before claiming Y"). Condense but keep the verification structure intact.
- **Auth/session/crypto checklists**: High value for reviews. Keep intact even when they could theoretically be condensed further.
- **Cross-skill redundancy**: Before condensing a review reference for language X, check if `<language>-patterns` already covers the same topics. If yes, delete the review file and route to the language skill instead.

## Content Trimming Checklist

Apply to every reference file:

- [ ] Remove "When to Load" sections (SKILL.md handles routing)
- [ ] Remove "Sources" sections (not actionable for agents)
- [ ] Remove Quick Reference tables that duplicate body content
- [ ] Remove "Complete Example" sections when individual pattern sections already cover every piece
- [ ] Remove wrapper command sections duplicated from SKILL.md
- [ ] Replace full Bad+Good code blocks with rule statements
- [ ] Keep code snippets only for non-obvious patterns (5-10 lines max)
- [ ] Audit for cross-file duplication (same pattern in 2+ files → keep in one, cross-ref from others)
- [ ] If file has both rule statements AND a summary checklist, verify the checklist is not restating rules — checklists should be action items, not rule restatements
- [ ] Merge platform-specific command blocks (Mac/Linux + Windows): show one platform, add a one-line note for the other (e.g., "Windows: use `.ps1` extension and `$env:VAR` syntax")
- [ ] Exclude copy-paste template files (report templates, scaffolds) from condensation — their value IS their verbatim content
- [ ] Target: 50-200 lines per reference file

## File Size Targets

| File Type | Line Target | When Exceeded |
|-----------|-------------|---------------|
| Focused reference (single topic) | 50-100 lines | Split into subtopics |
| Broad reference (3+ related categories) | 100-200 lines | Condense further or split |
| Wrapper/command reference | 50-100 lines | Already lean, minor trim |
| SKILL.md entry point | 50-80 lines | Move content to references |
| Workflow file | 150-250 lines | Cross-reference instead of duplicating |
| Copy-paste template (report, scaffold) | No target | Keep as-is; value is verbatim content |

**Classifying files**: A file covering 3+ distinct categories (e.g., anti-patterns for config, modules, variables, operations, performance) is a "broad reference." A file covering one domain (e.g., wrapper commands) is "focused."

### Workflow File Optimization

Workflow files (interactive multi-phase processes) follow different rules than reference files:
- **Cross-reference, don't duplicate**: If SKILL.md or a reference file has the canonical content (command lists, checklists, templates), the workflow should say "Load `references/X.md`" instead of repeating it
- **Keep interactive structure**: Phase boundaries, approval gates, and AskUserQuestion prompts are the workflow's value — don't condense these
- **Condense presentation blocks**: Draft report previews, completion summaries, and example outputs can be shortened — they're illustrative, not canonical

## SKILL.md Routing Table Design

Use specific trigger keywords so agents load fewer, more targeted files:

```markdown
| When to Use | Reference |
|-------------|-----------|
| [specific keywords/scenarios] | [filename](references/filename.md) |
```

**Good triggers**: "Language footguns: nil, slices, maps, defer, mutex"
**Bad triggers**: "General code review" (too broad, loads everything)

### Hub-and-Spoke Skills

When a universal skill (e.g., testing-patterns) covers the same domain as language-specific skills (e.g., go-patterns/testing.md, python-patterns/testing.md), the universal skill is the "hub" and should include an explicit cross-reference table routing to each language "spoke." Keep universal principles in the hub; language-specific patterns, code examples, and wrapper commands belong only in the spokes.

## Splitting Criteria

Split a file when:
- It covers 3+ unrelated categories (e.g., language gotchas + design smells + testing)
- Different agents need different parts at different times
- File exceeds 200 lines after condensing

**Common hotspot**: `anti-patterns.md` files tend to become catch-all buckets that bundle security, testing, modern-feature pitfalls, and design smells. After condensing, audit categories and move domain-specific anti-patterns to their natural home (e.g., testing anti-patterns → testing.md, security anti-patterns → security.md, feature pitfalls → the feature's reference file).

Split by **use case**, not by **topic hierarchy**:
- Writing code vs reviewing code
- Language-level vs design-level
- Implementation vs debugging
- Pitfalls belong with the feature they relate to, not in a generic anti-patterns file

**Anti-patterns + dedicated references**: When a skill has both an anti-patterns file and dedicated reference files (e.g., `networking.md`, `rbac.md`), keep anti-patterns as rule-statement-only and put canonical YAML/code in the dedicated files. Cross-reference from anti-patterns: "See networking.md for network policy YAML."

### Language Skill Standard Files

Language-pattern skills should always include these dedicated files (in addition to topic-specific references):

- **`gotchas.md`** — Language-specific footguns and subtle bugs (type coercion, scoping, async pitfalls, etc.). High-value, low-token content that agents need as reminders even when the gotchas are "well known" — agents still make these mistakes in generated code.
- **`anti-patterns.md`** — Standalone file with SKILL.md routing entry. Do NOT inline anti-pattern tables at the bottom of other reference files (e.g., bundling.md, express-patterns.md) — they become undiscoverable. Consolidate all anti-patterns into one file, organized by category (General, Framework, Testing, etc.).

## Applying to Other Skills (Two-Pass Process)

### Pass 1: Condense format
1. Run `wc -l` on all reference files to identify bloated ones
2. Read each file, categorize content as: rule-only vs rule+snippet
3. Apply content trimming checklist (including cross-file deduplication audit)
4. Update SKILL.md routing with specific trigger keywords
5. Verify no topics were silently dropped

### Pass 2: Audit structure
6. Check files >200 lines — split by use case if needed
7. Audit anti-patterns files for bundled categories → move to natural homes
8. Grep for common cross-file duplicates: `sensitive`, version pinning, variable/parameter design rules, naming conventions — these are the most frequent offenders
9. Verify no cross-file duplication was introduced during condensation
10. Check for **whole-skill redundancy**: Two skills covering the same domain (e.g., documentation-patterns + technical-writing-frameworks) should be merged into one. Merge unique content from the smaller skill into the larger, delete the smaller, and update all agent references
11. Run final `wc -l` to confirm all files are within targets
