# Kitchen Sink E2E Test Repo - Implementation Plan

**Goal:** Create a standalone E2E test repo that validates syllago's full content lifecycle across all 11 providers using golden file comparison against provider-sourced format specs.

**Architecture:** Standalone GitHub repo (OpenScribbler/syllago-kitchen-sink) with provider fixture files, shell-based test harness, and golden files sourced from official provider documentation.

**Tech Stack:** Bash, shell test harness, syllago binary

**Design Doc:** docs/plans/2026-03-06-kitchen-sink-design.md

---

## Phase 0: Provider Spec Research

Research each provider's official documentation to establish ground truth for fixture files and golden files. Each task produces a spec doc at `docs/provider-specs/<provider>.md`.

All 11 research tasks are independent and can run in parallel.

### Task 1: Research Claude Code format specs

**Files:**
- Create: `docs/provider-specs/claude-code.md`

**Depends on:** None

**Success Criteria:**
- [ ] Spec doc covers: rules, skills, agents, commands, hooks, MCP config
- [ ] File paths documented (CLAUDE.md, .claude/rules/, .claude/skills/, .claude/agents/, .claude/commands/, .claude/settings.json)
- [ ] Frontmatter format for rules documented (globs field, alwaysApply)
- [ ] Skill directory structure documented (SKILL.md inside subdirs)
- [ ] Agent markdown format documented
- [ ] Command format documented
- [ ] settings.json schema for hooks and MCP documented
- [ ] All source URLs cited

---

**Research targets:**
- Claude Code official docs for rules, skills, agents, commands
- settings.json schema (hooks key, mcpServers key)
- YAML frontmatter fields for rules (globs, description, alwaysApply)
- Skill SKILL.md frontmatter (name, description)
- Agent frontmatter fields
- Command format and frontmatter

### Task 2: Research Gemini CLI format specs

**Files:**
- Create: `docs/provider-specs/gemini-cli.md`

**Depends on:** None

**Success Criteria:**
- [ ] Spec doc covers: rules, skills, agents, commands, MCP config
- [ ] File paths documented (GEMINI.md, .gemini/ structure)
- [ ] settings.json schema documented
- [ ] Skill/agent/command formats documented
- [ ] All source URLs cited

---

**Research targets:**
- Gemini CLI docs for GEMINI.md and .gemini/ directory structure
- settings.json format (MCP server config)
- How Gemini handles skills, agents, commands (similar to Claude Code?)
- Any Gemini-specific fields or conventions

### Task 3: Research Cursor format specs

**Files:**
- Create: `docs/provider-specs/cursor.md`

**Depends on:** None

**Success Criteria:**
- [ ] .mdc file format fully documented
- [ ] Frontmatter fields documented (description, globs, alwaysApply)
- [ ] .cursor/rules/ directory structure documented
- [ ] All source URLs cited

---

**Research targets:**
- Cursor docs for .cursor/rules/ and .mdc format
- MDC frontmatter fields: description, globs (array), alwaysApply (boolean)
- How globs scoping works in Cursor
- Any other content types Cursor supports

### Task 4: Research Windsurf format specs

**Files:**
- Create: `docs/provider-specs/windsurf.md`

**Depends on:** None

**Success Criteria:**
- [ ] .windsurfrules file format documented
- [ ] trigger: field values documented (always_on, glob, model_decision)
- [ ] Rule section format documented
- [ ] All source URLs cited

---

**Research targets:**
- Windsurf docs for .windsurfrules format
- trigger: field and its valid values
- How multiple rules are concatenated in the file
- description: field format

### Task 5: Research Codex format specs

**Files:**
- Create: `docs/provider-specs/codex.md`

**Depends on:** None

**Success Criteria:**
- [ ] AGENTS.md format documented
- [ ] .codex/agents/ TOML format documented with all fields
- [ ] All source URLs cited

---

**Research targets:**
- OpenAI Codex docs for AGENTS.md conventions
- TOML agent format (.codex/agents/*.toml) - fields, structure
- How AGENTS.md is shared with other providers

### Task 6: Research Copilot CLI format specs

**Files:**
- Create: `docs/provider-specs/copilot-cli.md`

**Depends on:** None

**Success Criteria:**
- [ ] .github/copilot-instructions.md format documented
- [ ] .copilot/ directory structure documented (agents, commands, mcp.json, hooks.json)
- [ ] hooks.json schema documented
- [ ] mcp.json schema documented
- [ ] All source URLs cited

---

**Research targets:**
- GitHub Copilot CLI docs for instructions and .copilot/ structure
- hooks.json format and fields
- mcp.json format
- Agent and command markdown formats

### Task 7: Research Zed format specs

**Files:**
- Create: `docs/provider-specs/zed.md`

**Depends on:** None

**Success Criteria:**
- [ ] .rules file format documented
- [ ] How rules are structured in the flat file
- [ ] All source URLs cited

---

**Research targets:**
- Zed editor docs for .rules file
- Whether Zed supports any other AI content types
- Format conventions (headers, separators)

### Task 8: Research Cline format specs

**Files:**
- Create: `docs/provider-specs/cline.md`

**Depends on:** None

**Success Criteria:**
- [ ] .clinerules/ directory structure documented
- [ ] Rule markdown format documented (paths: field)
- [ ] All source URLs cited

---

**Research targets:**
- Cline docs for .clinerules/ directory
- paths: field format and how it maps to file globs
- Rule file naming conventions

### Task 9: Research Roo Code format specs

**Files:**
- Create: `docs/provider-specs/roo-code.md`

**Depends on:** None

**Success Criteria:**
- [ ] .roorules file format documented
- [ ] .roo/ directory structure documented (rules/, rules-code/, rules-architect/, etc.)
- [ ] Mode-specific directories documented
- [ ] mcp.json schema documented
- [ ] All source URLs cited

---

**Research targets:**
- Roo Code docs for .roorules and .roo/ structure
- Mode-specific rule directories (rules-code, rules-architect, etc.)
- mcp.json format
- How Roo Code handles agents vs mode-specific rules

### Task 10: Research OpenCode format specs

**Files:**
- Create: `docs/provider-specs/opencode.md`

**Depends on:** None

**Success Criteria:**
- [ ] opencode.json format documented (JSONC, mcp key)
- [ ] .opencode/ directory structure documented (skill/, agents/, commands/)
- [ ] Note: skill/ (singular) not skills/
- [ ] AGENTS.md shared discovery path documented
- [ ] All source URLs cited

---

**Research targets:**
- OpenCode docs for opencode.json and .opencode/ structure
- JSONC format specifics
- MCP config under the `mcp` key
- How OpenCode shares AGENTS.md with Codex

### Task 11: Research Kiro format specs

**Files:**
- Create: `docs/provider-specs/kiro.md`

**Depends on:** None

**Success Criteria:**
- [ ] .kiro/ directory structure documented (steering/, agents/, settings/)
- [ ] Steering file format documented (used for both rules and skills)
- [ ] Agent JSON format documented (including hooks embedding)
- [ ] settings/mcp.json schema documented
- [ ] All source URLs cited

---

**Research targets:**
- Kiro docs for .kiro/ directory structure
- Steering format (how it handles both rules and skills)
- Agent JSON schema (fields, hooks embedding)
- settings/mcp.json format

## Phase 1: Repo Scaffolding

Initialize the GitHub repo with basic structure and documentation.

### Task 12: Create GitHub repo and initialize

**Files:**
- Create: `README.md`
- Create: `.gitignore`
- Create: `docs/provider-specs/.gitkeep`
- Create: `tests/golden/.gitkeep`

**Depends on:** None (can start before Phase 0 completes)

**Success Criteria:**
- [ ] `OpenScribbler/syllago-kitchen-sink` repo exists on GitHub
- [ ] README.md explains what the repo is, how to run tests, and how to contribute
- [ ] .gitignore excludes temp dirs, OS files

---

### Step 1: Create the GitHub repo

```bash
gh repo create OpenScribbler/syllago-kitchen-sink --public --clone \
  --description "E2E test fixtures and golden files for syllago cross-provider content lifecycle"
cd syllago-kitchen-sink
```

### Step 2: Create .gitignore

```
# tests/run.sh creates temp dirs here
/tmp-test-*/

# OS
.DS_Store
Thumbs.db

# Editor
*.swp
*~
.idea/
.vscode/
```

### Step 3: Create README.md

```markdown
# syllago-kitchen-sink

E2E test repo for [syllago](https://github.com/OpenScribbler/syllago). A "polyglot AI project" with all 11 providers configured simultaneously, used to validate the full content lifecycle: discovery, add, install, export, and convert.

## Quick Start

```bash
# Requires: syllago binary in PATH
./tests/run.sh
```

## What This Tests

This repo contains fixture files for every AI coding tool provider that syllago supports, each in their native format:

| Provider | Content Types |
|----------|--------------|
| Claude Code | Rules, skills, agents, commands, hooks, MCP |
| Gemini CLI | Rules, skills, agents, commands, MCP |
| Cursor | Rules (.mdc) |
| Windsurf | Rules (.windsurfrules) |
| Codex | Agents (TOML), shared AGENTS.md |
| Copilot CLI | Rules, agents, commands, hooks, MCP |
| Zed | Rules (.rules) |
| Cline | Rules (.clinerules) |
| Roo Code | Rules (mode-specific), MCP |
| OpenCode | Skills, agents, commands, MCP (JSONC) |
| Kiro | Steering (rules+skills), agents (JSON), MCP |

## Test Suites

| Suite | What it tests |
|-------|--------------|
| `test_discovery.sh` | Discovery mode finds expected items per provider |
| `test_add.sh` | Adding content from each provider populates the library |
| `test_install.sh` | Installing to each provider produces correct native format |
| `test_convert.sh` | Converting between formats produces correct output |

## Running Tests

```bash
./tests/run.sh                        # Full lifecycle, all suites
./tests/run.sh --seed                  # Pre-seed library for faster install/convert tests
./tests/run.sh --suite discovery       # Run a single suite
./tests/run.sh --seed --suite install  # Combine flags
```

## Golden Files

The `tests/golden/` directory contains expected output for each provider, sourced from official provider documentation (not from syllago output). Provider format specs are documented in `docs/provider-specs/`.

When a test fails, the diff shows exactly what syllago produced vs. what the provider spec says it should produce.

## Provider Specs

Each provider's format specification is documented in `docs/provider-specs/<provider>.md` with cited source URLs. These serve as the ground truth for both fixture files and golden files.
```

### Step 4: Create directory placeholders

```bash
mkdir -p docs/provider-specs tests/golden
touch docs/provider-specs/.gitkeep tests/golden/.gitkeep
```

### Step 5: Initial commit

```bash
git add -A
git commit -m "feat: initialize kitchen-sink E2E test repo"
git push -u origin main
```

## Phase 2: Provider Fixtures

Create native-format fixture files for each provider. These are the files that syllago discovers and reads from. Content is written from the Phase 0 provider spec research — exact field names, frontmatter syntax, and structure must match what each provider actually expects.

All 11 fixture tasks are independent of each other but depend on their corresponding Phase 0 research task.

**Canonical content used across all providers (adapted to each format):**
- **Security rule body:** "All code must validate user input at system boundaries. Never trust external data. Use parameterized queries for database access. Sanitize HTML output to prevent XSS."
- **Code-reviewer agent body:** "Review code changes for security vulnerabilities, performance issues, and adherence to project conventions. Flag any use of deprecated APIs. Suggest improvements for readability and maintainability."
- **Greeting skill body:** "When the user greets you, respond with a friendly greeting that includes the current time of day (morning/afternoon/evening). Keep it brief and warm."
- **Summarize command body:** "Summarize the provided content in 3-5 bullet points, focusing on key decisions, action items, and open questions."
- **MCP config:** A single example server entry (e.g., a filesystem MCP server) adapted to each provider's JSON schema.
- **Hooks config:** A simple pre-commit hook that runs a linter, adapted to each provider's hooks format.

### Task 13: Create Claude Code fixtures

**Files:**
- Create: `CLAUDE.md`
- Create: `.claude/settings.json`
- Create: `.claude/rules/security.md`
- Create: `.claude/skills/greeting/SKILL.md`
- Create: `.claude/agents/code-reviewer.md`
- Create: `.claude/commands/summarize.md`

**Depends on:** Task 1 (Claude Code spec research)

**Success Criteria:**
- [ ] CLAUDE.md exists with a project-level rule
- [ ] settings.json has valid hooks and mcpServers keys
- [ ] Rule has correct YAML frontmatter (globs field)
- [ ] Skill has SKILL.md with correct frontmatter (name, description)
- [ ] Agent has correct frontmatter format
- [ ] Command has correct format
- [ ] All formats match the provider spec from Task 1

---

Write each file using the exact format documented in `docs/provider-specs/claude-code.md`. The canonical content bodies listed above should be adapted into each file's format.

### Task 14: Create Gemini CLI fixtures

**Files:**
- Create: `GEMINI.md`
- Create: `.gemini/settings.json`
- Create: `.gemini/skills/greeting/SKILL.md`
- Create: `.gemini/agents/code-reviewer.md`
- Create: `.gemini/commands/summarize.md`

**Depends on:** Task 2 (Gemini CLI spec research)

**Success Criteria:**
- [ ] GEMINI.md exists with a project-level rule
- [ ] settings.json has valid MCP config
- [ ] Skill, agent, command formats match Gemini spec from Task 2
- [ ] All formats verified against provider spec

---

Write each file using the exact format documented in `docs/provider-specs/gemini-cli.md`.

### Task 15: Create Cursor fixtures

**Files:**
- Create: `.cursor/rules/security.mdc`
- Create: `.cursor/rules/code-review.mdc`

**Depends on:** Task 3 (Cursor spec research)

**Success Criteria:**
- [ ] .mdc files have correct frontmatter (description, globs, alwaysApply)
- [ ] security.mdc uses glob scoping (alwaysApply: false with globs)
- [ ] code-review.mdc uses alwaysApply: true or model_decision
- [ ] Formats match Cursor spec from Task 3

---

Write each .mdc file using the exact frontmatter format documented in `docs/provider-specs/cursor.md`.

### Task 16: Create Windsurf fixtures

**Files:**
- Create: `.windsurfrules`

**Depends on:** Task 4 (Windsurf spec research)

**Success Criteria:**
- [ ] .windsurfrules uses correct trigger: field syntax
- [ ] Contains security rule section
- [ ] Format matches Windsurf spec from Task 4

---

Write the .windsurfrules file using the exact format documented in `docs/provider-specs/windsurf.md`.

### Task 17: Create Codex fixtures

**Files:**
- Create: `AGENTS.md`
- Create: `.codex/agents/code-reviewer.toml`

**Depends on:** Task 5 (Codex spec research)

**Success Criteria:**
- [ ] AGENTS.md follows Codex conventions (shared with OpenCode)
- [ ] TOML agent has correct structure and fields
- [ ] Formats match Codex spec from Task 5

---

Write AGENTS.md and the TOML agent using the exact format documented in `docs/provider-specs/codex.md`. Note: AGENTS.md is shared with OpenCode (Task 22).

### Task 18: Create Copilot CLI fixtures

**Files:**
- Create: `.github/copilot-instructions.md`
- Create: `.copilot/agents/code-reviewer.md`
- Create: `.copilot/commands/summarize.md`
- Create: `.copilot/mcp.json`
- Create: `.copilot/hooks.json`

**Depends on:** Task 6 (Copilot CLI spec research)

**Success Criteria:**
- [ ] copilot-instructions.md has correct format
- [ ] Agent and command markdown formats match spec
- [ ] mcp.json has correct schema
- [ ] hooks.json has correct flat hooks format
- [ ] All formats match Copilot spec from Task 6

---

Write each file using the exact format documented in `docs/provider-specs/copilot-cli.md`.

### Task 19: Create Zed fixtures

**Files:**
- Create: `.rules`

**Depends on:** Task 7 (Zed spec research)

**Success Criteria:**
- [ ] .rules file uses correct flat format
- [ ] Contains security rule content
- [ ] Format matches Zed spec from Task 7

---

Write the .rules file using the exact format documented in `docs/provider-specs/zed.md`.

### Task 20: Create Cline fixtures

**Files:**
- Create: `.clinerules/security.md`
- Create: `.clinerules/code-review.md`

**Depends on:** Task 8 (Cline spec research)

**Success Criteria:**
- [ ] Rule files have correct format (paths: field if applicable)
- [ ] Formats match Cline spec from Task 8

---

Write each file using the exact format documented in `docs/provider-specs/cline.md`.

### Task 21: Create Roo Code fixtures

**Files:**
- Create: `.roorules`
- Create: `.roo/rules/security.md`
- Create: `.roo/rules-code/code-review.md`
- Create: `.roo/mcp.json`

**Depends on:** Task 9 (Roo Code spec research)

**Success Criteria:**
- [ ] .roorules top-level file exists
- [ ] Mode-specific directory (rules-code/) used correctly
- [ ] mcp.json has correct schema
- [ ] All formats match Roo Code spec from Task 9

---

Write each file using the exact format documented in `docs/provider-specs/roo-code.md`.

### Task 22: Create OpenCode fixtures

**Files:**
- Create: `opencode.json`
- Create: `.opencode/skill/greeting/SKILL.md`
- Create: `.opencode/agents/code-reviewer.md`
- Create: `.opencode/commands/summarize.md`

**Depends on:** Task 10 (OpenCode spec research)

**Success Criteria:**
- [ ] opencode.json is valid JSONC with mcp key
- [ ] .opencode/skill/ (singular) used, not skills/
- [ ] AGENTS.md (created in Task 17) also works for OpenCode discovery
- [ ] All formats match OpenCode spec from Task 10

---

Write each file using the exact format documented in `docs/provider-specs/opencode.md`. Note: AGENTS.md is shared with Codex (created in Task 17).

### Task 23: Create Kiro fixtures

**Files:**
- Create: `.kiro/steering/security.md`
- Create: `.kiro/steering/greeting.md`
- Create: `.kiro/agents/code-reviewer.json`
- Create: `.kiro/settings/mcp.json`

**Depends on:** Task 11 (Kiro spec research)

**Success Criteria:**
- [ ] Steering files handle both rules and skills
- [ ] Agent JSON has correct schema (including hooks embedding)
- [ ] settings/mcp.json has correct schema
- [ ] All formats match Kiro spec from Task 11

---

Write each file using the exact format documented in `docs/provider-specs/kiro.md`.

## Phase 3: Golden Files

Create the expected-output reference files for install and convert validation. Golden files represent what syllago SHOULD produce when installing/converting content TO a given provider. They are hand-written from the Phase 0 provider spec research — not generated from syllago output.

Each golden file task depends on both:
1. Its corresponding Phase 0 research task (for format spec)
2. The canonical content definitions from Phase 2 (for body content)

**Important distinction:**
- Phase 2 fixtures = what the provider's native files look like in-situ (source content syllago reads)
- Phase 3 golden files = what syllago should output when converting/installing TO that provider

These may differ in subtle ways. For example, a fixture file might have extra provider-specific fields that syllago doesn't preserve during round-trip conversion. The golden file captures what syllago's output should look like, built from the spec's format requirements applied to the canonical content.

**Verification process:** After each golden file is written, a dedicated verification sub-agent independently researches that provider's format spec and validates the golden file against it. The verification agent:
1. Fetches the provider's official documentation (independent of the Phase 0 research -- fresh eyes)
2. Checks every structural element: frontmatter fields, field names, field types, required vs optional, file extension, directory conventions
3. Validates that the canonical content body is correctly embedded in the format
4. Produces a verification report with PASS/FAIL per file and cited source URLs
5. If any file fails: flags the specific issue, the verifier's source, and what it should be

This is a two-person rule -- the writer and verifier never share research context. If both independently arrive at the same format, confidence is high. If they disagree, we investigate before proceeding.

**Execution pattern:** For each provider (Tasks 24-34):
1. Write the golden files from the Phase 0 spec doc
2. Spawn a verification sub-agent that independently researches and validates
3. Resolve any discrepancies before marking the task complete

### Task 24: Create Claude Code golden files

**Files:**
- Create: `tests/golden/claude-code/rules/security.md`
- Create: `tests/golden/claude-code/skills/greeting/SKILL.md`
- Create: `tests/golden/claude-code/agents/code-reviewer.md`
- Create: `tests/golden/claude-code/commands/summarize.md`
- Create: `tests/golden/claude-code/settings.json`

**Depends on:** Task 1 (spec research), Task 13 (fixtures for content reference)

**Success Criteria:**
- [ ] Each golden file represents what syllago should produce when installing TO claude-code
- [ ] Format matches Claude Code spec (correct frontmatter, structure)
- [ ] Canonical content body is present in each file
- [ ] settings.json golden shows expected hooks + MCP merge result

---

Hand-write each file from the Claude Code spec. These represent the expected output when running `syllago install <type> --to claude-code`.

### Task 25: Create Gemini CLI golden files

**Files:**
- Create: `tests/golden/gemini-cli/rules/GEMINI.md`
- Create: `tests/golden/gemini-cli/skills/greeting/SKILL.md`
- Create: `tests/golden/gemini-cli/agents/code-reviewer.md`
- Create: `tests/golden/gemini-cli/commands/summarize.md`
- Create: `tests/golden/gemini-cli/settings.json`

**Depends on:** Task 2 (spec research), Task 14 (fixtures)

**Success Criteria:**
- [ ] Each golden file matches Gemini CLI expected output format
- [ ] Canonical content present
- [ ] settings.json golden shows expected MCP merge result

---

Hand-write from Gemini CLI spec.

### Task 26: Create Cursor golden files

**Files:**
- Create: `tests/golden/cursor/rules/security.mdc`

**Depends on:** Task 3 (spec research), Task 15 (fixtures)

**Success Criteria:**
- [ ] .mdc has correct frontmatter (description, globs, alwaysApply)
- [ ] Body content matches canonical security rule
- [ ] Format matches what syllago's Cursor renderer produces per spec

---

Hand-write from Cursor spec. The .mdc frontmatter fields and their values should match what syllago renders when converting a canonical rule to Cursor format.

### Task 27: Create Windsurf golden files

**Files:**
- Create: `tests/golden/windsurf/rules/.windsurfrules`

**Depends on:** Task 4 (spec research), Task 16 (fixtures)

**Success Criteria:**
- [ ] trigger: field has correct value for the security rule
- [ ] description: field present
- [ ] Body content matches canonical rule
- [ ] Format matches Windsurf spec

---

Hand-write from Windsurf spec.

### Task 28: Create Codex golden files

**Files:**
- Create: `tests/golden/codex/agents/code-reviewer.toml`
- Create: `tests/golden/codex/AGENTS.md`

**Depends on:** Task 5 (spec research), Task 17 (fixtures)

**Success Criteria:**
- [ ] TOML agent has correct structure per Codex spec
- [ ] AGENTS.md matches expected rendered format
- [ ] Canonical agent content present

---

Hand-write from Codex spec.

### Task 29: Create Copilot CLI golden files

**Files:**
- Create: `tests/golden/copilot-cli/rules/copilot-instructions.md`
- Create: `tests/golden/copilot-cli/agents/code-reviewer.md`
- Create: `tests/golden/copilot-cli/commands/summarize.md`
- Create: `tests/golden/copilot-cli/mcp.json`
- Create: `tests/golden/copilot-cli/hooks.json`

**Depends on:** Task 6 (spec research), Task 18 (fixtures)

**Success Criteria:**
- [ ] Each file matches Copilot CLI expected output format
- [ ] hooks.json matches flat hooks schema
- [ ] mcp.json matches expected schema
- [ ] Canonical content present in all files

---

Hand-write from Copilot CLI spec.

### Task 30: Create Zed golden files

**Files:**
- Create: `tests/golden/zed/rules/.rules`

**Depends on:** Task 7 (spec research), Task 19 (fixtures)

**Success Criteria:**
- [ ] .rules file matches Zed flat format
- [ ] Canonical rule content present

---

Hand-write from Zed spec.

### Task 31: Create Cline golden files

**Files:**
- Create: `tests/golden/cline/rules/security.md`

**Depends on:** Task 8 (spec research), Task 20 (fixtures)

**Success Criteria:**
- [ ] Rule file matches Cline format (paths: field if applicable)
- [ ] Canonical content present

---

Hand-write from Cline spec.

### Task 32: Create Roo Code golden files

**Files:**
- Create: `tests/golden/roo-code/rules/security.md`
- Create: `tests/golden/roo-code/rules-code/code-review.md`
- Create: `tests/golden/roo-code/mcp.json`

**Depends on:** Task 9 (spec research), Task 21 (fixtures)

**Success Criteria:**
- [ ] Rule files match Roo Code format
- [ ] Mode-specific dir structure correct
- [ ] mcp.json matches expected merge result
- [ ] Canonical content present

---

Hand-write from Roo Code spec.

### Task 33: Create OpenCode golden files

**Files:**
- Create: `tests/golden/opencode/skill/greeting/SKILL.md`
- Create: `tests/golden/opencode/agents/code-reviewer.md`
- Create: `tests/golden/opencode/commands/summarize.md`
- Create: `tests/golden/opencode/opencode.json`

**Depends on:** Task 10 (spec research), Task 22 (fixtures)

**Success Criteria:**
- [ ] skill/ (singular) path used
- [ ] opencode.json shows expected MCP merge result
- [ ] Canonical content present in all files

---

Hand-write from OpenCode spec. Note: skill/ not skills/.

### Task 34: Create Kiro golden files

**Files:**
- Create: `tests/golden/kiro/steering/security.md`
- Create: `tests/golden/kiro/steering/greeting.md`
- Create: `tests/golden/kiro/agents/code-reviewer.json`
- Create: `tests/golden/kiro/settings/mcp.json`

**Depends on:** Task 11 (spec research), Task 23 (fixtures)

**Success Criteria:**
- [ ] Steering files match Kiro format for rules and skills
- [ ] Agent JSON matches expected schema
- [ ] settings/mcp.json matches expected merge result
- [ ] Canonical content present

---

Hand-write from Kiro spec.

## Phase 4: Test Harness

Build the shared test infrastructure and entry point. These are the shell scripts that set up isolation, provide assertion helpers, and orchestrate test suites.

### Task 35: Create lib.sh -- shared test infrastructure

**Files:**
- Create: `tests/lib.sh`

**Depends on:** Task 12 (repo scaffolding)

**Success Criteria:**
- [ ] `setup_sandbox` creates isolated temp dir with HOME override
- [ ] `teardown_sandbox` cleans up temp dir
- [ ] `seed_library` runs syllago add to pre-populate library
- [ ] All assertion helpers work: assert_exit_zero, assert_exit_nonzero, assert_output_contains, assert_file_exists, assert_file_contains, assert_json_key, assert_golden
- [ ] `assert_golden` uses normalized diff (strip trailing whitespace, normalize line endings, ignore trailing blank lines)
- [ ] `pass` and `fail` track counts globally
- [ ] `print_summary` shows pass/fail counts and exits with correct code
- [ ] Script is sourceable (no side effects on load)

---

### Step 1: Write tests/lib.sh

```bash
#!/usr/bin/env bash
# Kitchen Sink test library -- sourced by test scripts
# Provides: sandbox setup/teardown, assertion helpers, summary reporting

# -- State --

TESTS_PASSED=0
TESTS_FAILED=0
FAILURES=()
SANDBOX_DIR=""
PROJECT_DIR=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GOLDEN_DIR="$SCRIPT_DIR/golden"

# -- Sandbox --

setup_sandbox() {
  SANDBOX_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ks-test-XXXXXX")"
  export HOME="$SANDBOX_DIR"
  PROJECT_DIR="$SANDBOX_DIR/project"
  mkdir -p "$PROJECT_DIR"

  # Copy fixture files into the sandbox project
  # Use rsync to preserve dotfiles and directory structure
  rsync -a --exclude='tests/' --exclude='docs/' --exclude='.git/' \
    "$REPO_DIR/" "$PROJECT_DIR/"

  cd "$PROJECT_DIR" || exit 1
  echo "Sandbox: $SANDBOX_DIR"
}

teardown_sandbox() {
  if [[ -n "$SANDBOX_DIR" && -d "$SANDBOX_DIR" ]]; then
    rm -rf "$SANDBOX_DIR"
  fi
}

seed_library() {
  echo "Seeding library from claude-code fixtures..."
  syllago add --from claude-code --all --force --quiet 2>/dev/null || {
    fail "seed_library: syllago add --from claude-code failed"
    return 1
  }
  echo "Library seeded."
}

# -- Assertions --

pass() {
  local description="$1"
  TESTS_PASSED=$((TESTS_PASSED + 1))
  printf "  \033[32m+\033[0m %s\n" "$description"
}

fail() {
  local description="$1"
  TESTS_FAILED=$((TESTS_FAILED + 1))
  FAILURES+=("$description")
  printf "  \033[31mX\033[0m %s\n" "$description"
}

assert_exit_zero() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "$description"
  else
    fail "$description (exit code: $?)"
  fi
}

assert_exit_nonzero() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    fail "$description (expected non-zero exit, got 0)"
  else
    pass "$description"
  fi
}

assert_output_contains() {
  local description="$1"
  local expected="$2"
  shift 2
  local output
  output=$("$@" 2>&1) || true
  if echo "$output" | grep -qF "$expected"; then
    pass "$description"
  else
    fail "$description (expected output to contain: $expected)"
  fi
}

assert_file_exists() {
  local description="$1"
  local filepath="$2"
  if [[ -f "$filepath" ]]; then
    pass "$description"
  else
    fail "$description (file not found: $filepath)"
  fi
}

assert_file_contains() {
  local description="$1"
  local filepath="$2"
  local expected="$3"
  if [[ ! -f "$filepath" ]]; then
    fail "$description (file not found: $filepath)"
    return
  fi
  if grep -qF "$expected" "$filepath"; then
    pass "$description"
  else
    fail "$description (file does not contain: $expected)"
  fi
}

assert_json_key() {
  local description="$1"
  local filepath="$2"
  local key="$3"
  local expected="$4"
  if [[ ! -f "$filepath" ]]; then
    fail "$description (file not found: $filepath)"
    return
  fi
  local actual
  actual=$(jq -r "$key" "$filepath" 2>/dev/null) || {
    fail "$description (invalid JSON or key not found: $key)"
    return
  }
  if [[ "$actual" == "$expected" ]]; then
    pass "$description"
  else
    fail "$description (expected $key=$expected, got $actual)"
  fi
}

# Normalized diff: convert CRLF to LF, strip trailing whitespace,
# remove trailing blank lines
_normalize() {
  sed 's/\r$//' "$1" | sed 's/[[:space:]]*$//' | sed -e :a -e '/^\n*$/{$d;N;ba}'
}

assert_golden() {
  local description="$1"
  local actual_file="$2"
  local golden_file="$3"

  if [[ ! -f "$actual_file" ]]; then
    fail "$description (actual file not found: $actual_file)"
    return
  fi
  if [[ ! -f "$golden_file" ]]; then
    fail "$description (golden file not found: $golden_file)"
    return
  fi

  local actual_norm golden_norm
  actual_norm=$(_normalize "$actual_file")
  golden_norm=$(_normalize "$golden_file")

  if [[ "$actual_norm" == "$golden_norm" ]]; then
    pass "$description"
  else
    fail "$description (output differs from golden file)"
    echo "    --- expected (golden)"
    echo "    +++ actual"
    diff <(echo "$golden_norm") <(echo "$actual_norm") | head -20 | sed 's/^/    /'
  fi
}

# -- Summary --

print_summary() {
  local total=$((TESTS_PASSED + TESTS_FAILED))
  echo ""
  echo "==========================================="
  if [[ $TESTS_FAILED -eq 0 ]]; then
    printf "\033[32mPASSED: %d/%d\033[0m\n" "$TESTS_PASSED" "$total"
  else
    printf "\033[31mFAILED: %d/%d\033[0m\n" "$TESTS_FAILED" "$total"
    echo ""
    echo "Failures:"
    for f in "${FAILURES[@]}"; do
      echo "  - $f"
    done
  fi
  echo "==========================================="

  teardown_sandbox
  [[ $TESTS_FAILED -eq 0 ]]
}
```

### Task 36: Create run.sh -- test entry point

**Files:**
- Create: `tests/run.sh`

**Depends on:** Task 35 (lib.sh)

**Success Criteria:**
- [ ] Parses --seed and --suite flags correctly
- [ ] Sources lib.sh and sets up sandbox
- [ ] Runs all suites by default or single suite with --suite
- [ ] Seeds library when --seed is passed
- [ ] Calls print_summary at end
- [ ] Is executable (chmod +x)

---

### Step 1: Write tests/run.sh

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# -- Parse flags --

SEED=false
SUITE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed)  SEED=true; shift ;;
    --suite) SUITE="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: run.sh [--seed] [--suite <name>]"
      echo ""
      echo "Flags:"
      echo "  --seed          Pre-seed library (skip add tests for faster runs)"
      echo "  --suite <name>  Run only one suite: discovery, add, install, convert"
      exit 0
      ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done

# -- Setup --

echo "Kitchen Sink E2E Tests"
echo "==========================================="

setup_sandbox

if [[ "$SEED" == "true" ]]; then
  seed_library
fi

# -- Run suites --

run_suite() {
  local name="$1"
  local file="$SCRIPT_DIR/test_${name}.sh"
  if [[ ! -f "$file" ]]; then
    echo "Suite not found: $file"
    exit 1
  fi
  echo ""
  echo "-- Suite: $name --"
  source "$file"
}

if [[ -z "$SUITE" ]]; then
  run_suite discovery
  run_suite add
  run_suite install
  run_suite convert
else
  run_suite "$SUITE"
fi

# -- Summary --

print_summary
```

### Step 2: Make executable

```bash
chmod +x tests/run.sh tests/lib.sh
```

## Phase 5: Test Suites

Write the four test scripts that exercise syllago's content lifecycle. Each suite is sourced by run.sh (not executed as a subprocess) so it shares the sandbox and assertion state.

**CLI reference (from `syllago --help`):**
- `syllago add [type[/name]] --from <provider>` -- discovery (no args) or add to library
- `syllago install [name] --to <provider> --type <type> --method copy` -- install from library to provider location
- `syllago convert <name> --to <provider>` -- render to stdout in target format

**Note:** Install tests must use `--method copy` because the sandbox uses a temp HOME. Symlinks would point to the original fixture paths, not the sandbox library.

### Task 37: Create test_discovery.sh

**Files:**
- Create: `tests/test_discovery.sh`

**Depends on:** Task 35 (lib.sh), Task 36 (run.sh), all Phase 2 fixture tasks

**Success Criteria:**
- [ ] Tests discovery mode for all 11 providers
- [ ] Verifies expected content types appear in discovery output
- [ ] Includes negative test for unknown provider
- [ ] Each provider test checks for the content types that provider supports

---

### Step 1: Write tests/test_discovery.sh

```bash
#!/usr/bin/env bash
# Suite: discovery
# Tests that "syllago add --from <provider>" (no positional arg) discovers expected content.

# -- Claude Code --
assert_output_contains "claude-code discovers security rule" \
  "security" \
  syllago add --from claude-code --no-input

assert_output_contains "claude-code discovers greeting skill" \
  "greeting" \
  syllago add --from claude-code --no-input

assert_output_contains "claude-code discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from claude-code --no-input

assert_output_contains "claude-code discovers summarize command" \
  "summarize" \
  syllago add --from claude-code --no-input

# -- Gemini CLI --
assert_output_contains "gemini-cli discovers security rule" \
  "security" \
  syllago add --from gemini-cli --no-input

assert_output_contains "gemini-cli discovers greeting skill" \
  "greeting" \
  syllago add --from gemini-cli --no-input

assert_output_contains "gemini-cli discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from gemini-cli --no-input

assert_output_contains "gemini-cli discovers summarize command" \
  "summarize" \
  syllago add --from gemini-cli --no-input

# -- Cursor --
assert_output_contains "cursor discovers security rule" \
  "security" \
  syllago add --from cursor --no-input

assert_output_contains "cursor discovers code-review rule" \
  "code-review" \
  syllago add --from cursor --no-input

# -- Windsurf --
assert_output_contains "windsurf discovers rules" \
  "security" \
  syllago add --from windsurf --no-input

# -- Codex --
assert_output_contains "codex discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from codex --no-input

# -- Copilot CLI --
assert_output_contains "copilot-cli discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from copilot-cli --no-input

assert_output_contains "copilot-cli discovers summarize command" \
  "summarize" \
  syllago add --from copilot-cli --no-input

# -- Zed --
assert_output_contains "zed discovers rules" \
  "security" \
  syllago add --from zed --no-input

# -- Cline --
assert_output_contains "cline discovers security rule" \
  "security" \
  syllago add --from cline --no-input

# -- Roo Code --
assert_output_contains "roo-code discovers security rule" \
  "security" \
  syllago add --from roo-code --no-input

# -- OpenCode --
assert_output_contains "opencode discovers greeting skill" \
  "greeting" \
  syllago add --from opencode --no-input

assert_output_contains "opencode discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from opencode --no-input

# -- Kiro --
assert_output_contains "kiro discovers security steering" \
  "security" \
  syllago add --from kiro --no-input

assert_output_contains "kiro discovers code-reviewer agent" \
  "code-reviewer" \
  syllago add --from kiro --no-input

# -- Negative tests --
assert_exit_nonzero "unknown provider fails" \
  syllago add --from nonexistent-provider --no-input
```

### Task 38: Create test_add.sh

**Files:**
- Create: `tests/test_add.sh`

**Depends on:** Task 35 (lib.sh), Task 36 (run.sh), all Phase 2 fixture tasks

**Success Criteria:**
- [ ] Tests adding content from each provider into the library
- [ ] Verifies files exist at expected library paths after add
- [ ] Verifies canonical content body survived the add (format conversion to canonical)
- [ ] Tests --all flag for providers with multiple content types
- [ ] Includes negative test for unknown provider and unsupported content type

---

### Step 1: Write tests/test_add.sh

```bash
#!/usr/bin/env bash
# Suite: add
# Tests that "syllago add <type> --from <provider>" writes content to the library.
#
# NOTE: Each provider's add overwrites the same library paths (e.g., rules/security.md).
# Assertions run immediately after each add, before the next provider overwrites.
# The canonical body text ("validate user input", "security vulnerabilities") should
# survive conversion from any provider format, so checking for it after each add
# validates that specific provider's canonicalization.

LIBRARY="$HOME/.syllago/content"

# -- Claude Code (most content types) --
syllago add --all --from claude-code --force --no-input 2>/dev/null || true

assert_file_exists "claude-code: security rule in library" \
  "$LIBRARY/rules/security.md"
assert_file_contains "claude-code: security rule has body" \
  "$LIBRARY/rules/security.md" "validate user input"

assert_file_exists "claude-code: greeting skill in library" \
  "$LIBRARY/skills/greeting/SKILL.md"
assert_file_contains "claude-code: greeting skill has body" \
  "$LIBRARY/skills/greeting/SKILL.md" "greeting"

assert_file_exists "claude-code: code-reviewer agent in library" \
  "$LIBRARY/agents/code-reviewer.md"
assert_file_contains "claude-code: agent has body" \
  "$LIBRARY/agents/code-reviewer.md" "security vulnerabilities"

assert_file_exists "claude-code: summarize command in library" \
  "$LIBRARY/commands/summarize.md"

# -- Cursor (rules only, .mdc conversion) --
syllago add rules --from cursor --force --no-input 2>/dev/null || true

assert_file_exists "cursor: security rule in library" \
  "$LIBRARY/rules/security.md"
assert_file_contains "cursor: rule body survived mdc conversion" \
  "$LIBRARY/rules/security.md" "validate user input"

# -- Windsurf --
syllago add rules --from windsurf --force --no-input 2>/dev/null || true

assert_file_contains "windsurf: rule body survived conversion" \
  "$LIBRARY/rules/security.md" "validate user input"

# -- Codex (TOML agent) --
syllago add agents --from codex --force --no-input 2>/dev/null || true

assert_file_exists "codex: agent in library" \
  "$LIBRARY/agents/code-reviewer.md"
assert_file_contains "codex: agent body survived TOML conversion" \
  "$LIBRARY/agents/code-reviewer.md" "security vulnerabilities"

# -- Copilot CLI --
syllago add --all --from copilot-cli --force --no-input 2>/dev/null || true

assert_file_exists "copilot-cli: agent in library" \
  "$LIBRARY/agents/code-reviewer.md"
assert_file_exists "copilot-cli: command in library" \
  "$LIBRARY/commands/summarize.md"

# -- Cline --
syllago add rules --from cline --force --no-input 2>/dev/null || true

assert_file_contains "cline: rule body survived conversion" \
  "$LIBRARY/rules/security.md" "validate user input"

# -- Roo Code --
syllago add rules --from roo-code --force --no-input 2>/dev/null || true

assert_file_contains "roo-code: rule body survived conversion" \
  "$LIBRARY/rules/security.md" "validate user input"

# -- OpenCode --
syllago add --all --from opencode --force --no-input 2>/dev/null || true

assert_file_exists "opencode: skill in library" \
  "$LIBRARY/skills/greeting/SKILL.md"
assert_file_exists "opencode: agent in library" \
  "$LIBRARY/agents/code-reviewer.md"

# -- Kiro --
syllago add --all --from kiro --force --no-input 2>/dev/null || true

assert_file_exists "kiro: agent in library" \
  "$LIBRARY/agents/code-reviewer.md"

# -- Gemini CLI --
syllago add --all --from gemini-cli --force --no-input 2>/dev/null || true

assert_file_exists "gemini-cli: rule in library" \
  "$LIBRARY/rules/security.md"
assert_file_exists "gemini-cli: skill in library" \
  "$LIBRARY/skills/greeting/SKILL.md"

# -- Zed --
syllago add rules --from zed --force --no-input 2>/dev/null || true

assert_file_contains "zed: rule body survived conversion" \
  "$LIBRARY/rules/security.md" "validate user input"

# -- Negative tests --
assert_exit_nonzero "add from unknown provider fails" \
  syllago add --all --from nonexistent-provider --force --no-input

assert_exit_nonzero "add unsupported content type fails" \
  syllago add skills --from zed --force --no-input
```

### Task 39: Create test_install.sh

**Files:**
- Create: `tests/test_install.sh`

**Depends on:** Task 35 (lib.sh), Task 36 (run.sh), all Phase 2 fixtures, all Phase 3 golden files

**Success Criteria:**
- [ ] Ensures library is populated first (via add or seed)
- [ ] Installs content to each provider using --method copy
- [ ] Diffs installed files against golden files using assert_golden
- [ ] Covers all content types each provider supports
- [ ] Includes negative test for unknown provider and unsupported content type

---

### Step 1: Write tests/test_install.sh

```bash
#!/usr/bin/env bash
# Suite: install
# Tests that "syllago install --to <provider>" produces provider-native files
# that match the golden file reference.
#
# Requires library to be populated (via add suite or --seed flag).

# Ensure library is populated
if [[ ! -d "$HOME/.syllago/content/rules" ]]; then
  echo "  Library not populated. Running add from claude-code first..."
  syllago add --all --from claude-code --force --no-input 2>/dev/null || true
fi

# -- Claude Code --
syllago install security --to claude-code --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to claude-code" \
  "$PROJECT_DIR/.claude/rules/security.md" \
  "$GOLDEN_DIR/claude-code/rules/security.md"

syllago install greeting --to claude-code --type skills --method copy --no-input 2>/dev/null || true
assert_golden "install skill to claude-code" \
  "$PROJECT_DIR/.claude/skills/greeting/SKILL.md" \
  "$GOLDEN_DIR/claude-code/skills/greeting/SKILL.md"

syllago install code-reviewer --to claude-code --type agents --method copy --no-input 2>/dev/null || true
assert_golden "install agent to claude-code" \
  "$PROJECT_DIR/.claude/agents/code-reviewer.md" \
  "$GOLDEN_DIR/claude-code/agents/code-reviewer.md"

syllago install summarize --to claude-code --type commands --method copy --no-input 2>/dev/null || true
assert_golden "install command to claude-code" \
  "$PROJECT_DIR/.claude/commands/summarize.md" \
  "$GOLDEN_DIR/claude-code/commands/summarize.md"

# -- Cursor --
syllago install security --to cursor --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to cursor" \
  "$PROJECT_DIR/.cursor/rules/security.mdc" \
  "$GOLDEN_DIR/cursor/rules/security.mdc"

# -- Windsurf --
syllago install security --to windsurf --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to windsurf" \
  "$PROJECT_DIR/.windsurfrules" \
  "$GOLDEN_DIR/windsurf/rules/.windsurfrules"

# -- Codex --
syllago install code-reviewer --to codex --type agents --method copy --no-input 2>/dev/null || true
assert_golden "install agent to codex" \
  "$PROJECT_DIR/.codex/agents/code-reviewer.toml" \
  "$GOLDEN_DIR/codex/agents/code-reviewer.toml"

# -- Copilot CLI --
syllago install code-reviewer --to copilot-cli --type agents --method copy --no-input 2>/dev/null || true
assert_golden "install agent to copilot-cli" \
  "$PROJECT_DIR/.copilot/agents/code-reviewer.md" \
  "$GOLDEN_DIR/copilot-cli/agents/code-reviewer.md"

syllago install summarize --to copilot-cli --type commands --method copy --no-input 2>/dev/null || true
assert_golden "install command to copilot-cli" \
  "$PROJECT_DIR/.copilot/commands/summarize.md" \
  "$GOLDEN_DIR/copilot-cli/commands/summarize.md"

# -- Zed --
syllago install security --to zed --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to zed" \
  "$PROJECT_DIR/.rules" \
  "$GOLDEN_DIR/zed/rules/.rules"

# -- Cline --
syllago install security --to cline --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to cline" \
  "$PROJECT_DIR/.clinerules/security.md" \
  "$GOLDEN_DIR/cline/rules/security.md"

# -- Roo Code --
syllago install security --to roo-code --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to roo-code" \
  "$PROJECT_DIR/.roo/rules/security.md" \
  "$GOLDEN_DIR/roo-code/rules/security.md"

# -- OpenCode --
syllago install greeting --to opencode --type skills --method copy --no-input 2>/dev/null || true
assert_golden "install skill to opencode" \
  "$PROJECT_DIR/.opencode/skill/greeting/SKILL.md" \
  "$GOLDEN_DIR/opencode/skill/greeting/SKILL.md"

syllago install code-reviewer --to opencode --type agents --method copy --no-input 2>/dev/null || true
assert_golden "install agent to opencode" \
  "$PROJECT_DIR/.opencode/agents/code-reviewer.md" \
  "$GOLDEN_DIR/opencode/agents/code-reviewer.md"

# -- Kiro --
syllago install security --to kiro --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to kiro (steering)" \
  "$PROJECT_DIR/.kiro/steering/security.md" \
  "$GOLDEN_DIR/kiro/steering/security.md"

syllago install code-reviewer --to kiro --type agents --method copy --no-input 2>/dev/null || true
assert_golden "install agent to kiro" \
  "$PROJECT_DIR/.kiro/agents/code-reviewer.json" \
  "$GOLDEN_DIR/kiro/agents/code-reviewer.json"

# -- Gemini CLI --
syllago install security --to gemini-cli --type rules --method copy --no-input 2>/dev/null || true
assert_golden "install rule to gemini-cli" \
  "$PROJECT_DIR/GEMINI.md" \
  "$GOLDEN_DIR/gemini-cli/rules/GEMINI.md"

syllago install greeting --to gemini-cli --type skills --method copy --no-input 2>/dev/null || true
assert_golden "install skill to gemini-cli" \
  "$PROJECT_DIR/.gemini/skills/greeting/SKILL.md" \
  "$GOLDEN_DIR/gemini-cli/skills/greeting/SKILL.md"

# -- MCP configs --
syllago install --to claude-code --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to claude-code" \
  "$PROJECT_DIR/.claude/settings.json" \
  "$GOLDEN_DIR/claude-code/settings.json"

syllago install --to copilot-cli --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to copilot-cli" \
  "$PROJECT_DIR/.copilot/mcp.json" \
  "$GOLDEN_DIR/copilot-cli/mcp.json"

syllago install --to roo-code --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to roo-code" \
  "$PROJECT_DIR/.roo/mcp.json" \
  "$GOLDEN_DIR/roo-code/mcp.json"

syllago install --to kiro --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to kiro" \
  "$PROJECT_DIR/.kiro/settings/mcp.json" \
  "$GOLDEN_DIR/kiro/settings/mcp.json"

# -- Hooks --
syllago install --to copilot-cli --type hooks --method copy --no-input 2>/dev/null || true
assert_golden "install hooks to copilot-cli" \
  "$PROJECT_DIR/.copilot/hooks.json" \
  "$GOLDEN_DIR/copilot-cli/hooks.json"

# -- Gemini MCP --
syllago install --to gemini-cli --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to gemini-cli" \
  "$PROJECT_DIR/.gemini/settings.json" \
  "$GOLDEN_DIR/gemini-cli/settings.json"

# -- OpenCode MCP --
syllago install --to opencode --type mcp --method copy --no-input 2>/dev/null || true
assert_golden "install MCP to opencode" \
  "$PROJECT_DIR/opencode.json" \
  "$GOLDEN_DIR/opencode/opencode.json"

# -- Claude Code Hooks --
syllago install --to claude-code --type hooks --method copy --no-input 2>/dev/null || true
assert_golden "install hooks to claude-code" \
  "$PROJECT_DIR/.claude/settings.json" \
  "$GOLDEN_DIR/claude-code/settings.json"

# -- Negative tests --
assert_exit_nonzero "install to unknown provider fails" \
  syllago install security --to nonexistent-provider --type rules --method copy --no-input

assert_exit_nonzero "install unsupported type to provider fails" \
  syllago install greeting --to zed --type skills --method copy --no-input
```

### Task 40: Create test_convert.sh

**Files:**
- Create: `tests/test_convert.sh`

**Depends on:** Task 35 (lib.sh), Task 36 (run.sh), all Phase 2 fixture tasks, all Phase 3 golden files

**Success Criteria:**
- [ ] Converts library content to each provider format
- [ ] Captures output via --output flag and diffs against golden files
- [ ] Covers rules, agents, skills across multiple target providers
- [ ] Includes negative test for nonexistent item

---

### Step 1: Write tests/test_convert.sh

```bash
#!/usr/bin/env bash
# Suite: convert
# Tests that "syllago convert <name> --to <provider>" produces correct output.
# Captures output via --output flag and diffs against golden files.
#
# Requires library to be populated (via add suite or --seed flag).

# Ensure library is populated
if [[ ! -d "$HOME/.syllago/content/rules" ]]; then
  echo "  Library not populated. Running add from claude-code first..."
  syllago add --all --from claude-code --force --no-input 2>/dev/null || true
fi

CONVERT_TMP="$SANDBOX_DIR/convert-output"
mkdir -p "$CONVERT_TMP"

# Helper: convert to file, then assert_golden
convert_and_check() {
  local description="$1"
  local name="$2"
  local provider="$3"
  local golden="$4"
  local outfile="$CONVERT_TMP/${provider}-${name}"

  syllago convert "$name" --to "$provider" --output "$outfile" 2>/dev/null || true
  assert_golden "$description" "$outfile" "$golden"
}

# -- Rules to various providers --
convert_and_check "convert security rule to cursor" \
  security cursor "$GOLDEN_DIR/cursor/rules/security.mdc"

convert_and_check "convert security rule to windsurf" \
  security windsurf "$GOLDEN_DIR/windsurf/rules/.windsurfrules"

convert_and_check "convert security rule to cline" \
  security cline "$GOLDEN_DIR/cline/rules/security.md"

convert_and_check "convert security rule to zed" \
  security zed "$GOLDEN_DIR/zed/rules/.rules"

convert_and_check "convert security rule to roo-code" \
  security roo-code "$GOLDEN_DIR/roo-code/rules/security.md"

convert_and_check "convert security rule to kiro" \
  security kiro "$GOLDEN_DIR/kiro/steering/security.md"

# -- Agents to various providers --
convert_and_check "convert code-reviewer agent to codex" \
  code-reviewer codex "$GOLDEN_DIR/codex/agents/code-reviewer.toml"

convert_and_check "convert code-reviewer agent to kiro" \
  code-reviewer kiro "$GOLDEN_DIR/kiro/agents/code-reviewer.json"

convert_and_check "convert code-reviewer agent to copilot-cli" \
  code-reviewer copilot-cli "$GOLDEN_DIR/copilot-cli/agents/code-reviewer.md"

convert_and_check "convert code-reviewer agent to opencode" \
  code-reviewer opencode "$GOLDEN_DIR/opencode/agents/code-reviewer.md"

# -- Skills to various providers --
convert_and_check "convert greeting skill to kiro" \
  greeting kiro "$GOLDEN_DIR/kiro/steering/greeting.md"

convert_and_check "convert greeting skill to opencode" \
  greeting opencode "$GOLDEN_DIR/opencode/skill/greeting/SKILL.md"

convert_and_check "convert greeting skill to gemini-cli" \
  greeting gemini-cli "$GOLDEN_DIR/gemini-cli/skills/greeting/SKILL.md"

# -- Commands --
convert_and_check "convert summarize command to copilot-cli" \
  summarize copilot-cli "$GOLDEN_DIR/copilot-cli/commands/summarize.md"

convert_and_check "convert summarize command to gemini-cli" \
  summarize gemini-cli "$GOLDEN_DIR/gemini-cli/commands/summarize.md"

# -- MCP to various providers --
convert_and_check "convert MCP config to copilot-cli" \
  mcp copilot-cli "$GOLDEN_DIR/copilot-cli/mcp.json"

convert_and_check "convert MCP config to roo-code" \
  mcp roo-code "$GOLDEN_DIR/roo-code/mcp.json"

convert_and_check "convert MCP config to kiro" \
  mcp kiro "$GOLDEN_DIR/kiro/settings/mcp.json"

convert_and_check "convert MCP config to gemini-cli" \
  mcp gemini-cli "$GOLDEN_DIR/gemini-cli/settings.json"

convert_and_check "convert MCP config to opencode" \
  mcp opencode "$GOLDEN_DIR/opencode/opencode.json"

# -- Hooks --
convert_and_check "convert hooks to copilot-cli" \
  hooks copilot-cli "$GOLDEN_DIR/copilot-cli/hooks.json"

convert_and_check "convert hooks to kiro" \
  hooks kiro "$GOLDEN_DIR/kiro/agents/code-reviewer.json"

# -- Negative tests --
assert_exit_nonzero "convert nonexistent item fails" \
  syllago convert nonexistent-item --to cursor
```


## Phase 6: Validation and Push

Run the full test suite against the current syllago binary, fix any failures, and push.

### Task 41: Run full test suite and fix failures

**Files:**
- Modify: Any fixture, golden, or test file that needs adjustment

**Depends on:** All previous tasks

**Success Criteria:**
- [ ] `./tests/run.sh` exits 0
- [ ] All discovery tests pass (all providers find expected content)
- [ ] All add tests pass (content lands in library)
- [ ] All install tests pass (golden file diffs match)
- [ ] All convert tests pass (golden file diffs match)
- [ ] No test is skipped or commented out

---

### Step 1: Run the full suite

```bash
cd syllago-kitchen-sink
./tests/run.sh
```

### Step 2: Analyze failures

For each failure:
1. Check the diff output -- does the golden file need updating, or does syllago have a bug?
2. If golden file is wrong: update it and re-run verification agent
3. If syllago has a bug: file a bead in the main syllago repo and note it in the test as a known failure
4. If test assertion is wrong: fix the test

### Step 3: Re-run until clean

```bash
./tests/run.sh
# Expected: PASSED: N/N
```

### Task 42: Commit and push

**Depends on:** Task 41

**Success Criteria:**
- [ ] All files committed with descriptive messages
- [ ] Pushed to OpenScribbler/syllago-kitchen-sink main branch
- [ ] README accurately reflects the test suite

---

```bash
git add -A
git commit -m "feat: complete kitchen-sink E2E test repo with fixtures, golden files, and test harness"
git push origin main
```

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| Phase 0 | 1-11 | Provider spec research (11 parallel agents) |
| Phase 1 | 12 | Repo scaffolding (GitHub repo, README) |
| Phase 2 | 13-23 | Provider fixtures (11 providers, native format) |
| Phase 3 | 24-34 | Golden files (11 providers, with independent verification) |
| Phase 4 | 35-36 | Test harness (lib.sh, run.sh) |
| Phase 5 | 37-40 | Test suites (discovery, add, install, convert) |
| Phase 6 | 41-42 | Validation and push |

**Total: 42 tasks**

**Dependency graph:**
- Phase 0 and Phase 1 run in parallel (no dependencies on each other)
- Phase 2 depends on Phase 0 (each fixture task depends on its research task)
- Phase 3 depends on Phase 0 + Phase 2 (golden files need specs and content reference)
- Phase 4 depends on Phase 1 only (harness code is content-independent)
- Phase 5 depends on Phase 2 + Phase 3 + Phase 4 (tests need fixtures, golden files, and harness)
- Phase 6 depends on everything
