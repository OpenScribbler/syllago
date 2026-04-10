# Investigation: Skills `is-mode` Field

> Research compiled 2026-03-22. Investigating whether `is-mode` is a real SKILL.md frontmatter field.

## Summary

`is-mode` is **not a real field**. It does not exist in the Agent Skills specification, Claude Code's official documentation, any other provider's skill spec, or any real-world SKILL.md files on GitHub. The only related claim comes from a single third-party blog post ("Claude Agent Skills: A First Principles Deep Dive" by Lee Han Chung) which describes a `mode` boolean field -- but this field is absent from every authoritative source and cannot be found in the Claude Code repository. The blog post likely contains hallucinated or speculative content dressed up as source code analysis. Syllago should not support this field.

## Evidence Found

### Official Claude Code Documentation (code.claude.com)
- URL: https://code.claude.com/docs/en/skills
- Finding: **No `is-mode` or `mode` field.** The official frontmatter reference table lists exactly these fields: `name`, `description`, `argument-hint`, `disable-model-invocation`, `user-invocable`, `allowed-tools`, `model`, `effort`, `context`, `agent`, `hooks`. No mode-related boolean field exists.

### Agent Skills Specification (agentskills.io)
- URL: https://agentskills.io/specification
- Finding: **No `is-mode` or `mode` field.** The open standard defines only: `name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools` (experimental). The spec is minimal by design.

### Claude Platform API Docs (platform.claude.com)
- URL: https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview
- Finding: **No `is-mode` or `mode` field** in best practices or overview documentation.

### Repovive Skill Frontmatter Reference
- URL: https://repovive.com/roadmaps/claude-code/claude-md-skills-hooks-mcp/skill-frontmatter-fields
- Finding: **No `is-mode` or `mode` field.** Lists the same 8 fields from official docs.

### Third-Party Blog: "Claude Agent Skills: A First Principles Deep Dive"
- URL: https://leehanchung.github.io/blogs/2025/10/26/claude-skills-deep-dive/
- Finding: **This is the likely origin of the claim.** The blog describes a `mode` (not `is-mode`) boolean field that supposedly "categorizes a skill as a 'mode command'" and makes it "appear in a special 'Mode Commands' section at the top of the skills list." The author claims this is based on source code analysis of Claude Code internals (referencing functions like `fN2()`, `formatSkill()`, `getAllCommands()`). However, searching the `anthropics/claude-code` GitHub repository for `mode`, `is-mode`, `isMode`, and `Mode Commands` returns zero results. The obfuscated function names (`fN2()`) suggest the author was reading minified/bundled code, which is highly prone to misinterpretation. This is almost certainly a misread of minified source or outright fabrication.

### Mikhail Shilkov's Claude Code Skills Article
- URL: https://mikhail.io/2025/10/claude-code-skills/
- Finding: **No mention of `is-mode` or `mode` field.** Only documents `name` and `description`.

### GitHub Code Search: `is-mode` in SKILL.md files
- Method: `gh search code "is-mode" --filename SKILL.md` and `gh search code "is-mode:" --filename SKILL.md`
- Finding: **Zero results.** No SKILL.md file on all of public GitHub contains `is-mode` as a frontmatter key. The word "mode" appears only in skill names (e.g., `deep-analysis-mode`) and content body text, never as a standalone frontmatter field.

### GitHub Code Search: Claude Code repository
- Method: `gh search code` for `mode`, `is-mode`, `isMode`, `Mode Commands` in `anthropics/claude-code`
- Finding: **Zero results** for any mode-related skill frontmatter field in the Claude Code source.

### Web Search: Exact-match queries (8 queries)
- Queries: `"is-mode" SKILL.md`, `"is-mode" claude`, `"is-mode: true" skill`, `"is-mode" agent skills frontmatter`, `"is-mode" claude code skill`, `agentskills.io "is-mode"`, `site:docs.anthropic.com "is-mode"`, `"is-mode" yaml frontmatter coding agent`
- Finding: **All returned zero results.** The exact string `is-mode` does not appear in any indexed web content related to skills or AI coding tools.

### Claude Code GitHub Issues: Frontmatter Validator Bugs
- URLs: github.com/anthropics/claude-code/issues/25380, /30611, /24826
- Finding: These issues document validator bugs where the validator rejected valid Claude Code extended fields. The fields complained about are `allowed-tools`, `argument-hint`, `context`, `agent`, `model`, `hooks`, `disable-model-invocation`, `user-invocable`. No `is-mode` or `mode` field appears in any issue or complaint, which it would if it were a real field being incorrectly rejected.

## Conclusion

**`is-mode` is not real.** It is not documented, not implemented, and not used anywhere in practice. The closest thing that exists is a claim in one blog post about a `mode` boolean, which itself appears to be a misinterpretation of minified Claude Code source code.

**Recommended action:** Do not add `is-mode` support to syllago. If a future Claude Code release adds a mode-related frontmatter field, it will appear in the official docs at code.claude.com/docs/en/skills and can be added at that time. No preemptive work is warranted.
