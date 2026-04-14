# Canonical Keys Expansion — Implementation Plan

**Goal:** Expand `docs/spec/canonical-keys.yaml` from skills-only (13 keys) to all 6 content types (44 keys total), and populate `canonical_mappings` in all 14 provider-format YAMLs for every content type with `status: supported`.

**Architecture:** Pure YAML data work — no Go code changes required. `gencapabilities.go` reads `canonical_mappings` as `map[string]capMappingYAML` and `canonical-keys.yaml` as a nested map; both are fully schema-agnostic. Adding new content type sections and key names to the YAMLs is picked up automatically. After all YAML edits, run `./syllago _gencapabilities` from `cli/` to regenerate `capabilities.json`. The integration test `TestGencapabilities_AllRealProviders` in `gencapabilities_test.go` validates the real files on every `make test` run.

**Tech Stack:** YAML (data editing only), Go (`make test` verification)

**Design Doc:** `docs/plans/2026-04-13-canonical-keys-expansion-design.md`

---

## Provider Coverage Reference

Providers with `status: supported` per content type (determines which YAMLs get canonical_mappings):

| Provider | rules | hooks | mcp | agents | commands |
|----------|-------|-------|-----|--------|----------|
| amp | yes | yes | yes | — | — |
| claude-code | yes | yes | yes | yes | yes |
| cline | yes | yes | yes | — | yes |
| codex | yes | yes | yes | yes | yes |
| copilot-cli | yes | yes | yes | yes | yes |
| cursor | — | — | — | — | — |
| factory-droid | yes | yes | yes | yes | yes |
| gemini-cli | yes | yes | yes | — | yes |
| kiro | yes | yes | yes | yes | — |
| opencode | — | — | — | — | — |
| pi | yes | yes | — | — | yes |
| roo-code | — | — | — | — | — |
| windsurf | yes | yes | yes | yes | yes |
| zed | — | — | — | — | — |

Providers not appearing in any column (cursor, opencode, roo-code, zed) have no supported non-skills content types and are not modified in this work.

---

## Task T1: Add Normative Definitions Header to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** nothing

### Success Criteria
- `cd cli && make test` → pass — `TestGencapabilities_AllRealProviders` parses the updated file without error
- `grep -q "Confidence Enum" docs/spec/canonical-keys.yaml` → pass — Normative definitions comment block present
- `grep -q "Key Lifecycle Policy" docs/spec/canonical-keys.yaml` → pass — Lifecycle policy documented

### Step 1: Insert comment block after `---` document start

Insert the following comment block immediately after the `---` document start marker and before the existing `# Canonical key vocabulary` comment:

```yaml
# Normative Definitions
#
# Confidence Enum
#   confirmed  — Verified directly against provider documentation, first-party test, or
#                authoritative source code. The mapping author can cite a specific primary source.
#   inferred   — Extrapolated from related behavior, secondary sources, partial evidence, or
#                behavioral observation. No primary citation available, but reasoning is
#                documented in the mechanism field.
#   unknown    — Default when a capmon sweep cannot establish either of the above. Indicates
#                the mapping has not been actively verified and should be treated as a research
#                lead, not ground truth.
#
#   Two contributors applying the same evidence to the same provider capability must reach the
#   same confidence tier. Capmon sweeps may downgrade confidence to unknown when a sweep cannot
#   verify current provider behavior.
#
# Key Lifecycle Policy
#   Add:        A new key may be introduced when 2+ providers demonstrate the same semantic
#               concept (graduation threshold: same semantic meaning, not just same name).
#   Rename:     The old name remains as an alias for at least one capmon cycle, recorded in an
#               aliases field on the new key entry.
#   Deprecate:  A key may be deprecated when the underlying concept no longer reflects current
#               provider reality. Deprecated keys are marked deprecated: true with a replacement
#               pointer for at least one capmon cycle before removal.
#   Split:      The original name is retained as an alias to the most common interpretation,
#               with deprecation applied to the alias after one cycle.
#   Remove:     Only after the alias/deprecation period. Recorded in a changelog comment here.
#
#   Post-publication: Once a key is published in a community-owned spec capability registry,
#   changes are governed by that spec's change process. Syllago canonical-keys.yaml becomes a
#   derived reference synchronized from upstream. Syllago-specific lifecycle rules apply only
#   to keys that remain internal.
#
# Staleness and Maintenance
#   Canonical mappings are a point-in-time snapshot. Capmon (syllago-jtafb) runs scheduled
#   sweeps that re-verify provider documentation and detect changes. When a sweep cannot verify
#   a mapping, confidence is downgraded to unknown and the entry is flagged for human review.
#
# Public-Spec Inheritance
#   When canonical keys feed into public interchange specs, those keys inherit the normative
#   precision requirements of the destination spec. Ambiguity tolerated in internal vocabulary
#   must be resolved before publication in a community-owned spec capability registry. The
#   minimum-qualification statements and boundary predicates in this file are written with that
#   future migration in mind.
```

### Step 2: Verify

```bash
cd cli && make test
```

Expected: all tests pass. If `TestGencapabilities_AllRealProviders` fails, the comment block broke YAML syntax — review indentation.

---

## Task T2: Add `rules` Section to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** T1

### Success Criteria
- `cd cli && make test` → pass — file parses without error
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['canonical_keys']['rules']))"` → pass — prints `5`
- `grep -q "activation_mode:" docs/spec/canonical-keys.yaml` → pass — first rules key present

### Step 1: Append rules section after the `skills:` section under `content_types:`

```yaml
  rules:
    activation_mode:
      description: >
        How rules decide when to load into context. Providers implement varying modes:
        always-on, conditional/glob-based, model-decision, and manual activation.
        Tracks which modes the provider supports and how they are configured.
      type: object

    file_imports:
      description: >
        Whether rules can reference or include content from other files. Mechanisms vary:
        @-import syntax, @file.md directives, @path/to/file references.
      type: bool

    cross_provider_recognition:
      description: >
        Which rule file formats from other providers this provider recognizes.
        Contents: recognized_formats (list of filenames like AGENTS.md, .cursorrules,
        .windsurfrules). Minimum qualification: supported when the provider reads at least
        one rule-file format defined by a different provider.
      type: object

    auto_memory:
      description: >
        Whether the provider auto-generates persistent rules from conversation context.
        Distinct from user-authored rules — these are AI-created and stored separately.
      type: bool

    hierarchical_loading:
      description: >
        Whether rules load from multiple directory levels with defined precedence.
        Enables project-root rules plus subdirectory-scoped overrides.
      type: bool
```

### Step 2: Verify

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted(d['canonical_keys']['rules'].keys()))"
```

Expected: `['activation_mode', 'auto_memory', 'cross_provider_recognition', 'file_imports', 'hierarchical_loading']`

---

## Task T3: Add `hooks` Section to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** T2

### Success Criteria
- `cd cli && make test` → pass — file parses without error
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['canonical_keys']['hooks']))"` → pass — prints `9`
- `grep -q "decision_control:" docs/spec/canonical-keys.yaml` → pass — object-type hooks key present

### Step 1: Append hooks section after the `rules:` section

```yaml
  hooks:
    handler_types:
      description: >
        What kinds of executors hooks support beyond shell commands. May include HTTP
        endpoints, LLM prompt evaluation, multi-turn agent handlers, or TypeScript
        extensions.
      type: object

    matcher_patterns:
      description: >
        Whether hooks can filter which tools or events they respond to using name
        matching, regex patterns, or structured criteria.
      type: bool

    decision_control:
      description: >
        Which decision actions hooks can take on the triggering action. Contents:
        {block: bool, allow: bool, modify: bool}. Mechanisms include exit code
        contracts, JSON decision fields, or cancel flags. Boundary: decision_control
        governs whether a tool invocation proceeds; see permission_control for whether
        a tool is available at all.
      type: object

    input_modification:
      description: >
        Whether hooks can modify tool input arguments before the tool executes.
        Safety-critical capability — silent degradation creates false security.
        Minimum qualification: supported when the provider provides any mechanism to
        modify tool input arguments before execution (e.g., hookSpecificOutput.updatedInput,
        plugin-mutable args, or equivalent).
      type: bool

    async_execution:
      description: >
        Whether hooks can run asynchronously without blocking the agent's execution
        loop. Fire-and-forget semantics.
      type: bool

    hook_scopes:
      description: >
        Where hooks can be configured and the precedence model when multiple scopes
        define hooks for the same event. Common scopes: global/user, project, workspace,
        managed/enterprise.
      type: object

    json_io_protocol:
      description: >
        Whether hooks communicate with the host via structured JSON on stdin/stdout
        rather than plain text or exit codes alone.
      type: bool

    context_injection:
      description: >
        Whether hooks can inject messages, system prompts, or conversation context
        into the agent's active session.
      type: bool

    permission_control:
      description: >
        Whether hooks can make or influence permission decisions determining whether
        a tool is available for invocation. Minimum qualification: supported when the
        provider allows hooks to return permission decisions of any kind (grant, deny,
        or ask). Boundary: permission_control governs whether a tool is available;
        see decision_control for invocation-flow control.
      type: bool
```

### Step 2: Verify

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted(d['canonical_keys']['hooks'].keys()))"
```

Expected: `['async_execution', 'context_injection', 'decision_control', 'handler_types', 'hook_scopes', 'input_modification', 'json_io_protocol', 'matcher_patterns', 'permission_control']`

---

## Task T4: Add `mcp` Section to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** T3

### Success Criteria
- `cd cli && make test` → pass — file parses without error
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['canonical_keys']['mcp']))"` → pass — prints `8`
- `grep -q "tool_filtering:" docs/spec/canonical-keys.yaml` → pass — object-type mcp key present

### Step 1: Append mcp section after the `hooks:` section

```yaml
  mcp:
    transport_types:
      description: >
        Which MCP transport protocols the provider supports. Common transports: stdio
        (local process), SSE (Server-Sent Events), HTTP/streamable-HTTP (stateless or
        streaming).
      type: object

    oauth_support:
      description: >
        Whether the provider supports OAuth 2.0 authentication for remote MCP servers,
        including token storage and automatic refresh.
      type: bool

    env_var_expansion:
      description: >
        Whether MCP server configuration supports environment variable expansion.
        Reduces the need for hardcoded secrets in config files.
      type: bool

    tool_filtering:
      description: >
        Which per-server tool filtering mechanisms the provider supports. Contents:
        {allowlist: bool, blocklist: bool, disable_flag: bool}. Controls which server
        tools are exposed to the agent. Boundary: tool_filtering governs tool visibility
        (what appears in the agent's available tool set); see auto_approve for execution
        gating of visible tools.
      type: object

    auto_approve:
      description: >
        Whether specific MCP tools or entire servers can be configured for automatic
        approval without per-invocation user confirmation. Boundary: auto_approve governs
        execution gating (user prompt suppression) for tools that are already visible;
        see tool_filtering for tool visibility control.
      type: bool

    marketplace:
      description: >
        Whether the provider offers an in-IDE MCP server discovery and installation
        experience.
      type: bool

    resource_referencing:
      description: >
        Whether MCP resources (not just tools) can be accessed, typically via @-mention
        syntax or similar referencing mechanisms.
      type: bool

    enterprise_management:
      description: >
        Whether the provider supports organization-level MCP configuration management,
        including managed server lists, allowlists, and enterprise registries.
      type: bool
```

### Step 2: Verify

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted(d['canonical_keys']['mcp'].keys()))"
```

Expected: `['auto_approve', 'enterprise_management', 'env_var_expansion', 'marketplace', 'oauth_support', 'resource_referencing', 'tool_filtering', 'transport_types']`

---

## Task T5: Add `agents` Section to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** T4

### Success Criteria
- `cd cli && make test` → pass — file parses without error
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['canonical_keys']['agents']))"` → pass — prints `7`
- `grep -q "tool_aliases" docs/spec/canonical-keys.yaml` → fail — dropped key must not be present

### Step 1: Append agents section after the `mcp:` section

Note: `tool_aliases` is intentionally absent — dropped during design review as a false graduation.

```yaml
  agents:
    definition_format:
      description: >
        What format agent definitions use. Varies widely: Markdown with YAML frontmatter,
        JSON config files, TOML sections, or AGENTS.md plain markdown.
      type: string

    tool_restrictions:
      description: >
        Whether agents can be restricted to specific tools via allowlists, denylists,
        tool categories, or per-tool configuration maps.
      type: bool

    invocation_patterns:
      description: >
        How agents are triggered. Mechanisms include natural language detection,
        @-mention syntax, slash commands, CLI flags, or automatic delegation.
      type: object

    agent_scopes:
      description: >
        Where agent definitions can live and the priority ordering when definitions
        exist at multiple levels. Common scopes: project, user/personal,
        managed/enterprise, CLI-defined.
      type: object

    model_selection:
      description: >
        Whether per-agent model overrides are supported, allowing different agents
        to use different AI models.
      type: bool

    per_agent_mcp:
      description: >
        Whether agents can have their own MCP server configuration, scoping which
        external tools each agent can access.
      type: bool

    subagent_spawning:
      description: >
        Whether agents can spawn, delegate to, or resume other agents. Enables
        multi-agent coordination patterns.
      type: bool
```

### Step 2: Verify

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted(d['canonical_keys']['agents'].keys()))"
```

Expected: `['agent_scopes', 'definition_format', 'invocation_patterns', 'model_selection', 'per_agent_mcp', 'subagent_spawning', 'tool_restrictions']`

---

## Task T6: Add `commands` Section to canonical-keys.yaml

**Files:**
- Modify: `docs/spec/canonical-keys.yaml`

**Depends on:** T5

### Success Criteria
- `cd cli && make test` → pass — all tests pass including integration test
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); total=sum(len(v) for v in d['canonical_keys'].values()); print(total)"` → pass — prints `44`
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted(d['canonical_keys'].keys()))"` → pass — prints `['agents', 'commands', 'hooks', 'mcp', 'rules', 'skills']`

### Step 1: Append commands section after the `agents:` section

```yaml
  commands:
    argument_substitution:
      description: >
        How user-provided arguments are injected into command templates. Mechanisms
        vary: $ARGUMENTS, {{args}}, positional $1/$2/${@:N}, and other interpolation
        syntaxes.
      type: object

    builtin_commands:
      description: >
        Whether the provider ships default/built-in commands alongside user-defined
        custom commands.
      type: bool
```

### Step 2: Verify full canonical-keys.yaml is complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
total = sum(len(v) for v in d['canonical_keys'].values())
print(f'Content types: {sorted(d[\"canonical_keys\"].keys())}')
print(f'Total keys: {total}')
assert total == 44, f'Expected 44, got {total}'
print('PASS')
"
```

---

## Checkpoint A: canonical-keys.yaml Complete

After T6, before any provider-format work:

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities > /dev/null && echo "YAML parse: PASS"
```

Both must succeed before proceeding to T7.

---

## Task T7: Populate rules canonical_mappings — claude-code

**Files:**
- Modify: `docs/provider-formats/claude-code.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T6

### Success Criteria
- `cd cli && make test` → pass — integration test sees non-empty rules mappings
- `grep -c "confidence:" docs/provider-formats/claude-code.yaml` → pass — prints a number greater than the pre-task count (new entries present)
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['claude-code']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: true
        mechanism: "Always-on (CLAUDE.md loads at every session start); conditional via paths frontmatter in .claude/rules/*.md files (glob-based, triggers on file access)"
        confidence: confirmed
      file_imports:
        supported: true
        mechanism: "@path/to/file syntax in CLAUDE.md files; relative and absolute paths; recursive up to 5 hops; requires per-project approval"
        confidence: confirmed
      cross_provider_recognition:
        supported: false
        mechanism: "Claude Code does not natively read rule files defined by other providers"
        confidence: confirmed
      auto_memory:
        supported: true
        mechanism: "Auto memory saves Claude's accumulated notes to ~/.claude/projects/<project>/memory/MEMORY.md; requires CC v2.1.59+; enabled by default"
        confidence: confirmed
      hierarchical_loading:
        supported: true
        mechanism: "CLAUDE.md files load from all ancestor directories up to repo root; .claude/rules/ files discovered recursively; user-level rules load before project rules"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T8: Populate rules canonical_mappings — windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T7

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['windsurf']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: true
        mechanism: "activation_modes extension: always (unconditional), conditional (glob patterns), model (AI decides), manual (explicit toggle)"
        confidence: confirmed
      file_imports:
        supported: false
        mechanism: "No @-import or file inclusion syntax documented for Windsurf rule files"
        confidence: confirmed
      cross_provider_recognition:
        supported: true
        mechanism: "agents_md_auto_scoping: Windsurf reads AGENTS.md files from other providers and applies them with workspace-aware scoping"
        confidence: confirmed
      auto_memory:
        supported: true
        mechanism: "auto_generated_memories: Windsurf AI generates persistent memories stored in .windsurf/memories/ as rule-like files"
        confidence: confirmed
      hierarchical_loading:
        supported: false
        mechanism: "Windsurf rule files are global or workspace-scoped; no subdirectory precedence model documented"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T9: Populate rules canonical_mappings — cline

**Files:**
- Modify: `docs/provider-formats/cline.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T8

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['cline']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: true
        mechanism: "conditional_rules_paths: Cline supports path-conditional rules alongside always-on rules; conditions use glob patterns"
        confidence: confirmed
      file_imports:
        supported: false
        mechanism: "No file import/include syntax documented for Cline rule files"
        confidence: inferred
      cross_provider_recognition:
        supported: true
        mechanism: "multi_source_rule_detection: Cline reads .clinerules/, .cursorrules, CLAUDE.md, and other provider rule files automatically"
        confidence: confirmed
      auto_memory:
        supported: false
        mechanism: "Cline does not have an AI-generated auto-memory system distinct from user-authored rules"
        confidence: confirmed
      hierarchical_loading:
        supported: false
        mechanism: "Cline rules are project-scoped or global; no per-subdirectory loading hierarchy documented"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T10: Populate rules canonical_mappings — codex

**Files:**
- Modify: `docs/provider-formats/codex.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T9

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['codex']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Codex AGENTS.md files load unconditionally; no conditional or model-decision activation documented"
        confidence: inferred
      file_imports:
        supported: false
        mechanism: "No file import syntax documented for Codex AGENTS.md files"
        confidence: inferred
      cross_provider_recognition:
        supported: true
        mechanism: "agents_md_filename: Codex reads AGENTS.md files using the standard filename used by multiple providers"
        confidence: confirmed
      auto_memory:
        supported: false
        mechanism: "Codex does not have an AI-generated auto-memory system"
        confidence: confirmed
      hierarchical_loading:
        supported: true
        mechanism: "hierarchical_agents_md: Codex loads AGENTS.md from subdirectories with defined precedence (deeper overrides higher)"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T11: Populate rules canonical_mappings — copilot-cli

**Files:**
- Modify: `docs/provider-formats/copilot-cli.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T10

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['copilot-cli']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Copilot CLI rules load unconditionally at session start; no conditional or manual activation modes documented"
        confidence: inferred
      file_imports:
        supported: false
        mechanism: "No @-import or file inclusion syntax documented for Copilot CLI rule files"
        confidence: inferred
      cross_provider_recognition:
        supported: true
        mechanism: "agents_md_instructions: Copilot CLI reads AGENTS.md files following the multi-provider shared format"
        confidence: confirmed
      auto_memory:
        supported: false
        mechanism: "Copilot CLI does not have an AI-generated persistent memory system for rules"
        confidence: confirmed
      hierarchical_loading:
        supported: true
        mechanism: "Copilot CLI supports repo-wide instructions plus path-specific and personal instruction layers with defined precedence"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T12: Populate rules canonical_mappings — factory-droid

**Files:**
- Modify: `docs/provider-formats/factory-droid.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T11

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['factory-droid']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Factory Droid rule files load unconditionally; no conditional or model-decision activation documented"
        confidence: inferred
      file_imports:
        supported: false
        mechanism: "No file import syntax documented for Factory Droid rule files"
        confidence: inferred
      cross_provider_recognition:
        supported: true
        mechanism: "agents_md_format: Factory Droid reads AGENTS.md files in the shared multi-provider format"
        confidence: confirmed
      auto_memory:
        supported: false
        mechanism: "Factory Droid does not have an AI-generated auto-memory system"
        confidence: confirmed
      hierarchical_loading:
        supported: true
        mechanism: "Factory Droid loads global and project rule layers; innermost (most specific) layer takes precedence"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T13: Populate rules canonical_mappings — gemini-cli

**Files:**
- Modify: `docs/provider-formats/gemini-cli.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T12

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['gemini-cli']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Gemini CLI rule files load unconditionally; no conditional activation syntax documented"
        confidence: inferred
      file_imports:
        supported: true
        mechanism: "file_imports extension: Gemini CLI supports including external file content into rule files via import directives"
        confidence: confirmed
      cross_provider_recognition:
        supported: false
        mechanism: "Gemini CLI reads its own GEMINI.md format; no cross-provider rule file recognition documented"
        confidence: inferred
      auto_memory:
        supported: true
        mechanism: "memory_command: Gemini CLI /memory command saves notes that persist across sessions as rule-like content"
        confidence: confirmed
      hierarchical_loading:
        supported: true
        mechanism: "hierarchical_context_loading: Gemini CLI loads GEMINI.md files from multiple directory levels with defined precedence"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T14: Populate rules canonical_mappings — kiro

**Files:**
- Modify: `docs/provider-formats/kiro.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T13

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['kiro']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: true
        mechanism: "kiro_steering_inclusion_mode: Kiro steering files support always, conditional, and manual inclusion modes"
        confidence: confirmed
      file_imports:
        supported: true
        mechanism: "kiro_steering_file_references: Kiro steering files can reference other files using @file syntax"
        confidence: confirmed
      cross_provider_recognition:
        supported: true
        mechanism: "kiro_agents_md_recognition: Kiro reads AGENTS.md files from other providers in addition to its own steering files"
        confidence: confirmed
      auto_memory:
        supported: false
        mechanism: "Kiro does not have an AI-generated auto-memory system separate from user-authored steering files"
        confidence: confirmed
      hierarchical_loading:
        supported: false
        mechanism: "Kiro steering files are project-scoped; no multi-level subdirectory hierarchy documented"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T15: Populate rules canonical_mappings — amp

**Files:**
- Modify: `docs/provider-formats/amp.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T14

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['amp']['rules']['canonical_mappings']))"` → pass — prints `5`

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Amp rule files load unconditionally; no conditional or model-decision activation documented"
        confidence: inferred
      file_imports:
        supported: false
        mechanism: "No file import syntax documented for Amp rule files"
        confidence: inferred
      cross_provider_recognition:
        supported: false
        mechanism: "Amp reads its own rule format; no cross-provider rule file recognition documented"
        confidence: inferred
      auto_memory:
        supported: false
        mechanism: "Amp does not have an AI-generated auto-memory system"
        confidence: confirmed
      hierarchical_loading:
        supported: false
        mechanism: "Amp rule files are project-scoped; no subdirectory hierarchy documented"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T16: Populate rules canonical_mappings — pi

**Files:**
- Modify: `docs/provider-formats/pi.yaml` (replace `canonical_mappings: {}` under `rules:`)

**Depends on:** T15

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted([p for p,cts in d['providers'].items() if 'rules' in cts and cts['rules']['canonical_mappings']]))"` → pass — prints all 10 rules-supported providers

### Step 1: Replace `canonical_mappings: {}` under `rules:` with

```yaml
    canonical_mappings:
      activation_mode:
        supported: false
        mechanism: "Pi rule files load unconditionally; no conditional or model-decision activation documented"
        confidence: inferred
      file_imports:
        supported: false
        mechanism: "No file import syntax documented for Pi rule files"
        confidence: inferred
      cross_provider_recognition:
        supported: false
        mechanism: "Pi reads its own rule format; no cross-provider rule file recognition documented"
        confidence: inferred
      auto_memory:
        supported: false
        mechanism: "Pi does not have an AI-generated auto-memory system"
        confidence: confirmed
      hierarchical_loading:
        supported: false
        mechanism: "Pi rule files are not documented as supporting multi-level directory loading"
        confidence: inferred
```

### Step 2: Verify batch complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
providers = sorted([p for p, cts in d['providers'].items() if 'rules' in cts and cts['rules']['canonical_mappings']])
print(f'rules batch complete: {providers}')
assert len(providers) == 10
"
```

---

## Task T17: Populate hooks canonical_mappings — claude-code

**Files:**
- Modify: `docs/provider-formats/claude-code.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T16

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['claude-code']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: true
        mechanism: "Four types: command (shell), http (POST to URL), prompt (LLM evaluation), agent (subagent with tools); prompt/agent restricted to subset of events"
        confidence: confirmed
      matcher_patterns:
        supported: true
        mechanism: "hook_matcher_patterns: exact string, pipe-separated list, or JavaScript regex; matches on tool_name or event-specific fields"
        confidence: confirmed
      decision_control:
        supported: true
        mechanism: "block: exit code 2 or decision=block; allow: permissionDecision=allow in hookSpecificOutput; modify: updatedInput replaces tool input before execution"
        confidence: confirmed
      input_modification:
        supported: true
        mechanism: "hook_input_modification: PreToolUse returns updatedInput in hookSpecificOutput; entire input object replaced; compatible with all permissionDecision values except defer"
        confidence: confirmed
      async_execution:
        supported: true
        mechanism: "hook_async_execution: async: true on command handlers runs hook in background without blocking; decisions ignored; systemMessage delivered on next turn"
        confidence: confirmed
      hook_scopes:
        supported: true
        mechanism: "Six scopes: user (~/.claude/settings.json), project (.claude/settings.json), local (.claude/settings.local.json), managed policy, plugin (hooks/hooks.json), component frontmatter"
        confidence: confirmed
      json_io_protocol:
        supported: true
        mechanism: "Command hooks receive event JSON on stdin; respond with JSON on stdout (exit 0); structured fields: continue, stopReason, suppressOutput, systemMessage, hookSpecificOutput"
        confidence: confirmed
      context_injection:
        supported: true
        mechanism: "Hooks return systemMessage field in JSON output to inject context into agent's active session"
        confidence: confirmed
      permission_control:
        supported: true
        mechanism: "hook_permission_update_entries: PermissionRequest hooks return updatedPermissions with addRules/replaceRules/removeRules/setMode entries"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T18: Populate hooks canonical_mappings — windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T17

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['windsurf']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Windsurf hooks execute shell commands only; no HTTP, LLM prompt, or agent handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: false
        mechanism: "Windsurf hooks fire on all events of their configured type; no per-tool or regex matching documented"
        confidence: inferred
      decision_control:
        supported: false
        mechanism: "Windsurf hooks are observational; no mechanism to block, allow, or modify the triggering action documented"
        confidence: inferred
      input_modification:
        supported: false
        mechanism: "Windsurf hooks cannot modify tool input arguments before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Windsurf hooks run synchronously; no async/background execution documented"
        confidence: inferred
      hook_scopes:
        supported: true
        mechanism: "three_config_scopes: global (user-wide), workspace, and managed/enterprise hook configuration scopes"
        confidence: confirmed
      json_io_protocol:
        supported: true
        mechanism: "json_stdin_context: Windsurf hooks receive event context as JSON on stdin"
        confidence: confirmed
      context_injection:
        supported: false
        mechanism: "Windsurf hooks cannot inject context into the agent's active session"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Windsurf hooks do not participate in permission decisions"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T19: Populate hooks canonical_mappings — cline

**Files:**
- Modify: `docs/provider-formats/cline.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T18

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['cline']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Cline hooks execute shell commands only; no HTTP, LLM prompt, or agent handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: false
        mechanism: "Cline hooks fire on configured lifecycle events; no per-tool pattern matching documented"
        confidence: inferred
      decision_control:
        supported: true
        mechanism: "pre_tool_use_cancellation: PreToolUse hooks can cancel (block) the tool invocation; no allow or modify sub-capabilities documented"
        confidence: confirmed
      input_modification:
        supported: true
        mechanism: "context_modification_output: Cline PreToolUse hooks can return modified context/input before tool execution"
        confidence: confirmed
      async_execution:
        supported: false
        mechanism: "Cline hooks run synchronously before/after tool events; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: true
        mechanism: "global_and_project_hooks: Cline supports global (user-wide) and project-level hook configuration"
        confidence: confirmed
      json_io_protocol:
        supported: false
        mechanism: "Cline hooks communicate via exit codes and output text; no structured JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: true
        mechanism: "context_modification_output: Cline hooks can inject additional context into the agent's session via hook output"
        confidence: confirmed
      permission_control:
        supported: false
        mechanism: "Cline hooks can cancel tool invocations (decision_control) but cannot grant/deny tool availability"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T20: Populate hooks canonical_mappings — codex

**Files:**
- Modify: `docs/provider-formats/codex.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T19

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['codex']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: true
        mechanism: "hook_handler_types: Codex supports shell command and LLM prompt handler types; TypeScript extension type also documented"
        confidence: confirmed
      matcher_patterns:
        supported: true
        mechanism: "hook_matcher: Codex hooks support pattern matching to filter which tools or events trigger the hook"
        confidence: confirmed
      decision_control:
        supported: true
        mechanism: "hook_result_abort: Codex hooks can abort (block) the triggering action; allow and modify sub-capabilities documented via hook result schema"
        confidence: confirmed
      input_modification:
        supported: true
        mechanism: "hook_updated_input: Codex PreToolUse hooks return modified input arguments that replace the original tool input"
        confidence: confirmed
      async_execution:
        supported: true
        mechanism: "hook_execution_mode: Codex supports async hook execution mode for fire-and-forget background hook runs"
        confidence: confirmed
      hook_scopes:
        supported: true
        mechanism: "hook_scope: Codex hooks can be scoped to global/user or project configuration"
        confidence: confirmed
      json_io_protocol:
        supported: false
        mechanism: "Codex hooks communicate via exit codes and stdout text; no structured JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: true
        mechanism: "hook_system_message: Codex hooks can inject a system message into the agent's active session context"
        confidence: confirmed
      permission_control:
        supported: true
        mechanism: "hook_permission_decision: Codex hooks can return permission decisions that grant or deny tool availability"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T21: Populate hooks canonical_mappings — copilot-cli

**Files:**
- Modify: `docs/provider-formats/copilot-cli.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T20

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['copilot-cli']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Copilot CLI hooks execute shell commands only; no HTTP, LLM prompt, or agent handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: false
        mechanism: "Copilot CLI hooks fire on all events of their configured type; no per-tool filtering documented"
        confidence: inferred
      decision_control:
        supported: true
        mechanism: "pre_tool_use_deny: Copilot CLI PreToolUse hooks can deny (block) the tool invocation; no allow or modify documented"
        confidence: confirmed
      input_modification:
        supported: false
        mechanism: "Copilot CLI hooks cannot modify tool input before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Copilot CLI hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Copilot CLI hooks are configured at a single scope level; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: false
        mechanism: "Copilot CLI hooks communicate via exit codes; no JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: false
        mechanism: "Copilot CLI hooks cannot inject context into the agent session"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Copilot CLI hooks can deny tool invocations (decision_control) but do not govern tool availability"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T22: Populate hooks canonical_mappings — factory-droid

**Files:**
- Modify: `docs/provider-formats/factory-droid.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T21

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['factory-droid']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Factory Droid hooks execute shell commands only; no other handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: false
        mechanism: "Factory Droid hooks fire on configured lifecycle events; no per-tool pattern matching documented"
        confidence: inferred
      decision_control:
        supported: true
        mechanism: "hook_exit_code_behavior: Factory Droid hooks use exit codes to signal block (non-zero) or allow (zero) decisions on the triggering action"
        confidence: confirmed
      input_modification:
        supported: false
        mechanism: "Factory Droid hooks cannot modify tool input before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Factory Droid hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Factory Droid hooks are project-scoped; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: false
        mechanism: "Factory Droid hooks communicate via exit codes; no JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: false
        mechanism: "Factory Droid hooks cannot inject context into the agent session"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Factory Droid hooks cannot make permission decisions about tool availability"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T23: Populate hooks canonical_mappings — gemini-cli

**Files:**
- Modify: `docs/provider-formats/gemini-cli.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T22

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['gemini-cli']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Gemini CLI hooks execute shell commands only; no HTTP, LLM prompt, or agent handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: true
        mechanism: "hook_matchers: Gemini CLI hooks support event and tool name matching to filter when hooks fire"
        confidence: confirmed
      decision_control:
        supported: true
        mechanism: "exit_code_semantics: Gemini CLI uses exit codes to signal block (non-zero) or allow (zero) decisions; no modify sub-capability documented"
        confidence: confirmed
      input_modification:
        supported: false
        mechanism: "Gemini CLI hooks cannot modify tool input before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Gemini CLI hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Gemini CLI hooks are project-scoped; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: true
        mechanism: "hook_io_protocol: Gemini CLI hooks receive event data as JSON on stdin and return structured JSON responses on stdout"
        confidence: confirmed
      context_injection:
        supported: false
        mechanism: "Gemini CLI hooks cannot inject context into the agent session"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Gemini CLI hooks control invocation flow via exit codes but do not govern tool availability"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T24: Populate hooks canonical_mappings — kiro

**Files:**
- Modify: `docs/provider-formats/kiro.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T23

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['kiro']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Kiro hooks execute shell commands only; no HTTP, LLM prompt, or agent handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: false
        mechanism: "Kiro hooks fire on configured lifecycle events; no per-tool filtering documented"
        confidence: inferred
      decision_control:
        supported: false
        mechanism: "Kiro hooks are observational; no mechanism to block, allow, or modify tool invocations documented"
        confidence: inferred
      input_modification:
        supported: false
        mechanism: "Kiro hooks cannot modify tool input before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Kiro hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Kiro hooks are project-scoped; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: false
        mechanism: "Kiro hooks communicate via exit codes; no JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: false
        mechanism: "Kiro hooks cannot inject context into the agent session"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Kiro hooks do not participate in permission decisions"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T25: Populate hooks canonical_mappings — amp

**Files:**
- Modify: `docs/provider-formats/amp.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T24

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['amp']['hooks']['canonical_mappings']))"` → pass — prints `9`

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: false
        mechanism: "Amp hooks execute shell commands only; no other handler types documented"
        confidence: inferred
      matcher_patterns:
        supported: true
        mechanism: "hook_match_input_contains: Amp hooks can match on input content containing specific strings"
        confidence: confirmed
      decision_control:
        supported: false
        mechanism: "Amp hooks are observational; no mechanism to block, allow, or modify tool invocations documented"
        confidence: inferred
      input_modification:
        supported: false
        mechanism: "Amp hooks cannot modify tool input before execution"
        confidence: inferred
      async_execution:
        supported: false
        mechanism: "Amp hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Amp hooks are project-scoped; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: false
        mechanism: "Amp hooks communicate via exit codes; no JSON stdin/stdout protocol documented"
        confidence: inferred
      context_injection:
        supported: false
        mechanism: "Amp hooks cannot inject context into the agent session"
        confidence: inferred
      permission_control:
        supported: true
        mechanism: "permissions_system: Amp has a permissions system that hooks interact with to make tool availability decisions"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T26: Populate hooks canonical_mappings — pi

**Files:**
- Modify: `docs/provider-formats/pi.yaml` (replace `canonical_mappings: {}` under `hooks:`)

**Depends on:** T25

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted([p for p,cts in d['providers'].items() if 'hooks' in cts and cts['hooks']['canonical_mappings']]))"` → pass — prints all 10 hooks-supported providers

### Step 1: Replace `canonical_mappings: {}` under `hooks:` with

```yaml
    canonical_mappings:
      handler_types:
        supported: true
        mechanism: "pi_extension_typescript_native: Pi hooks support TypeScript/native extension handler type beyond shell commands"
        confidence: confirmed
      matcher_patterns:
        supported: false
        mechanism: "Pi hooks fire on configured lifecycle events; no per-tool filtering documented"
        confidence: inferred
      decision_control:
        supported: false
        mechanism: "Pi hooks are observational; no mechanism to block, allow, or modify tool invocations documented"
        confidence: inferred
      input_modification:
        supported: true
        mechanism: "pi_extension_tool_call_blocking: Pi extensions can intercept and modify tool call inputs before execution"
        confidence: confirmed
      async_execution:
        supported: false
        mechanism: "Pi hooks run synchronously; no async execution documented"
        confidence: inferred
      hook_scopes:
        supported: false
        mechanism: "Pi hooks are project-scoped; no multi-scope configuration documented"
        confidence: inferred
      json_io_protocol:
        supported: false
        mechanism: "Pi hooks communicate via TypeScript extension API; no JSON stdin/stdout protocol for shell hooks"
        confidence: inferred
      context_injection:
        supported: false
        mechanism: "Pi hooks cannot inject context into the agent session via hook output"
        confidence: inferred
      permission_control:
        supported: false
        mechanism: "Pi hooks cannot make permission decisions about tool availability"
        confidence: inferred
```

### Step 2: Verify hooks batch complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
providers = sorted([p for p, cts in d['providers'].items() if 'hooks' in cts and cts['hooks']['canonical_mappings']])
print(f'hooks batch complete: {providers}')
assert len(providers) == 10
"
```

---

## Task T27: Populate mcp canonical_mappings — claude-code

**Files:**
- Modify: `docs/provider-formats/claude-code.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T26

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['claude-code']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: true
        mechanism: "mcp_transport_types: stdio (local process), SSE (deprecated, still supported), HTTP/streamable-HTTP (recommended for remote)"
        confidence: confirmed
      oauth_support:
        supported: true
        mechanism: "mcp_oauth_authentication: OAuth 2.0 for HTTP MCP servers; dynamic client registration; token storage in macOS keychain or credentials file; auto-refresh"
        confidence: confirmed
      env_var_expansion:
        supported: true
        mechanism: "mcp_env_var_expansion: ${VAR} and ${VAR:-default} syntax in command, args, env, url, and headers fields of .mcp.json"
        confidence: confirmed
      tool_filtering:
        supported: false
        mechanism: "Claude Code exposes all tools from connected MCP servers; no per-server tool allowlist or blocklist for individual tool visibility"
        confidence: confirmed
      auto_approve:
        supported: false
        mechanism: "Claude Code does not support pre-configured auto-approval for specific MCP tools; all tool calls require user confirmation unless covered by session permission rules"
        confidence: confirmed
      marketplace:
        supported: false
        mechanism: "Claude Code does not have an in-IDE MCP marketplace for discovery and installation"
        confidence: confirmed
      resource_referencing:
        supported: true
        mechanism: "mcp_resources: MCP resources accessible via @server:protocol://path syntax; appear in @ autocomplete; auto-fetched when referenced"
        confidence: confirmed
      enterprise_management:
        supported: true
        mechanism: "mcp_managed_config: managed-mcp.json for exclusive control, or allowedMcpServers/deniedMcpServers in managed settings for policy-based control"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T28: Populate mcp canonical_mappings — windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T27

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['windsurf']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: true
        mechanism: "three_transport_types: Windsurf supports stdio, SSE, and HTTP/streamable-HTTP transports"
        confidence: confirmed
      oauth_support:
        supported: false
        mechanism: "Windsurf MCP does not document OAuth 2.0 authentication for remote servers"
        confidence: inferred
      env_var_expansion:
        supported: true
        mechanism: "config_interpolation: Windsurf MCP configuration supports ${VAR} interpolation for environment variables"
        confidence: confirmed
      tool_filtering:
        supported: false
        mechanism: "Windsurf exposes all tools from connected MCP servers; no per-server tool allowlist or blocklist documented"
        confidence: inferred
      auto_approve:
        supported: false
        mechanism: "Windsurf does not support pre-configured auto-approval for specific MCP tools"
        confidence: inferred
      marketplace:
        supported: true
        mechanism: "mcp_marketplace: Windsurf has an in-IDE MCP server discovery and one-click installation experience"
        confidence: confirmed
      resource_referencing:
        supported: false
        mechanism: "Windsurf does not document @-mention or equivalent access to MCP resources"
        confidence: inferred
      enterprise_management:
        supported: true
        mechanism: "enterprise_whitelist_and_registry: Windsurf supports organization-level MCP server allowlists and enterprise registries"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T29: Populate mcp canonical_mappings — cline

**Files:**
- Modify: `docs/provider-formats/cline.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T28

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['cline']['mcp']['canonical_mappings']))"` → pass — prints `8`
- `grep -A2 "tool_filtering:" docs/provider-formats/cline.yaml | grep "always_allow_tools"` → pass — visibility-scope mechanism present
- `grep -A2 "auto_approve:" docs/provider-formats/cline.yaml | grep "always_allow_tools"` → pass — execution-gating mechanism present

Note: Cline's `always_allow_tools` extension maps to both `tool_filtering` and `auto_approve`. The mechanism strings are distinct to clarify the boundary: tool_filtering describes visibility control; auto_approve describes prompt-suppression.

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: true
        mechanism: "sse_transport_support: Cline supports SSE transport alongside stdio; HTTP transport support follows MCP protocol adoption"
        confidence: confirmed
      oauth_support:
        supported: false
        mechanism: "Cline MCP does not document OAuth 2.0 authentication for remote servers"
        confidence: inferred
      env_var_expansion:
        supported: false
        mechanism: "Cline MCP configuration does not document environment variable expansion syntax"
        confidence: inferred
      tool_filtering:
        supported: true
        mechanism: "always_allow_tools (visibility scope): Cline's always_allow list controls which MCP tools are exposed in the agent's available tool set without per-invocation prompts"
        confidence: confirmed
      auto_approve:
        supported: true
        mechanism: "always_allow_tools (execution-gating scope): Cline's always_allow list suppresses per-invocation confirmation prompts for listed tools that are already visible"
        confidence: confirmed
      marketplace:
        supported: true
        mechanism: "mcp_marketplace: Cline has an in-IDE MCP server discovery and installation experience"
        confidence: confirmed
      resource_referencing:
        supported: false
        mechanism: "Cline does not document @-mention or equivalent access to MCP resources"
        confidence: inferred
      enterprise_management:
        supported: false
        mechanism: "Cline does not document organization-level MCP configuration management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T30: Populate mcp canonical_mappings — codex

**Files:**
- Modify: `docs/provider-formats/codex.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T29

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['codex']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: false
        mechanism: "Codex MCP transport types not documented beyond basic MCP support; likely stdio only"
        confidence: inferred
      oauth_support:
        supported: true
        mechanism: "mcp_oauth_support: Codex supports OAuth 2.0 authentication for remote MCP servers"
        confidence: confirmed
      env_var_expansion:
        supported: false
        mechanism: "Codex MCP configuration does not document environment variable expansion"
        confidence: inferred
      tool_filtering:
        supported: true
        mechanism: "mcp_enabled_disabled_tools: Codex supports per-server tool enable/disable configuration controlling which tools are exposed to the agent"
        confidence: confirmed
      auto_approve:
        supported: true
        mechanism: "mcp_per_tool_approval: Codex supports per-tool approval configuration to suppress confirmation prompts for trusted tools"
        confidence: confirmed
      marketplace:
        supported: false
        mechanism: "Codex does not have an in-IDE MCP marketplace"
        confidence: confirmed
      resource_referencing:
        supported: false
        mechanism: "Codex does not document MCP resource referencing"
        confidence: inferred
      enterprise_management:
        supported: false
        mechanism: "Codex does not document organization-level MCP management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T31: Populate mcp canonical_mappings — copilot-cli

**Files:**
- Modify: `docs/provider-formats/copilot-cli.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T30

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['copilot-cli']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: false
        mechanism: "Copilot CLI MCP transport types not documented"
        confidence: inferred
      oauth_support:
        supported: false
        mechanism: "Copilot CLI MCP does not document OAuth 2.0 authentication"
        confidence: inferred
      env_var_expansion:
        supported: false
        mechanism: "Copilot CLI MCP configuration does not document environment variable expansion"
        confidence: inferred
      tool_filtering:
        supported: true
        mechanism: "mcp_tool_allow_deny_flags: Copilot CLI supports per-tool allow and deny flags to control which MCP tools are exposed"
        confidence: confirmed
      auto_approve:
        supported: false
        mechanism: "Copilot CLI does not document pre-configured auto-approval for MCP tools"
        confidence: inferred
      marketplace:
        supported: false
        mechanism: "Copilot CLI does not have an in-IDE MCP marketplace"
        confidence: inferred
      resource_referencing:
        supported: false
        mechanism: "Copilot CLI does not document MCP resource referencing"
        confidence: inferred
      enterprise_management:
        supported: false
        mechanism: "Copilot CLI does not document organization-level MCP management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T32: Populate mcp canonical_mappings — factory-droid

**Files:**
- Modify: `docs/provider-formats/factory-droid.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T31

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['factory-droid']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: false
        mechanism: "Factory Droid MCP transport types not documented beyond basic MCP support"
        confidence: inferred
      oauth_support:
        supported: false
        mechanism: "Factory Droid MCP does not document OAuth 2.0 authentication"
        confidence: inferred
      env_var_expansion:
        supported: false
        mechanism: "Factory Droid MCP configuration does not document environment variable expansion"
        confidence: inferred
      tool_filtering:
        supported: false
        mechanism: "Factory Droid does not document per-server MCP tool filtering"
        confidence: inferred
      auto_approve:
        supported: false
        mechanism: "Factory Droid does not document pre-configured auto-approval for MCP tools"
        confidence: inferred
      marketplace:
        supported: false
        mechanism: "Factory Droid does not have an in-IDE MCP marketplace"
        confidence: inferred
      resource_referencing:
        supported: false
        mechanism: "Factory Droid does not document MCP resource referencing"
        confidence: inferred
      enterprise_management:
        supported: false
        mechanism: "Factory Droid does not document organization-level MCP management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T33: Populate mcp canonical_mappings — gemini-cli

**Files:**
- Modify: `docs/provider-formats/gemini-cli.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T32

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['gemini-cli']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: true
        mechanism: "transport_types: Gemini CLI documents supported MCP transport protocols including stdio and HTTP"
        confidence: confirmed
      oauth_support:
        supported: false
        mechanism: "Gemini CLI MCP does not document OAuth 2.0 authentication for remote servers"
        confidence: inferred
      env_var_expansion:
        supported: true
        mechanism: "env_variable_expansion: Gemini CLI MCP configuration supports environment variable expansion for secrets and paths"
        confidence: confirmed
      tool_filtering:
        supported: true
        mechanism: "tool_filtering: Gemini CLI supports per-server tool filtering to control which MCP tools are exposed to the agent"
        confidence: confirmed
      auto_approve:
        supported: false
        mechanism: "Gemini CLI does not document pre-configured auto-approval for specific MCP tools"
        confidence: inferred
      marketplace:
        supported: false
        mechanism: "Gemini CLI does not have an in-IDE MCP marketplace"
        confidence: confirmed
      resource_referencing:
        supported: true
        mechanism: "resource_referencing: Gemini CLI supports accessing MCP resources in addition to tools"
        confidence: confirmed
      enterprise_management:
        supported: false
        mechanism: "Gemini CLI does not document organization-level MCP management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T34: Populate mcp canonical_mappings — kiro

**Files:**
- Modify: `docs/provider-formats/kiro.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T33

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['kiro']['mcp']['canonical_mappings']))"` → pass — prints `8`

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: false
        mechanism: "Kiro MCP transport types not documented beyond basic MCP support"
        confidence: inferred
      oauth_support:
        supported: false
        mechanism: "Kiro MCP does not document OAuth 2.0 authentication"
        confidence: inferred
      env_var_expansion:
        supported: false
        mechanism: "Kiro MCP configuration does not document environment variable expansion"
        confidence: inferred
      tool_filtering:
        supported: true
        mechanism: "kiro_mcp_disabled_tools: Kiro supports per-server tool disable configuration to control which MCP tools are available to the agent"
        confidence: confirmed
      auto_approve:
        supported: true
        mechanism: "kiro_mcp_auto_approve: Kiro supports per-server or per-tool auto-approval configuration to suppress confirmation prompts"
        confidence: confirmed
      marketplace:
        supported: false
        mechanism: "Kiro does not have an in-IDE MCP marketplace"
        confidence: inferred
      resource_referencing:
        supported: false
        mechanism: "Kiro does not document MCP resource referencing"
        confidence: inferred
      enterprise_management:
        supported: false
        mechanism: "Kiro does not document organization-level MCP management"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T35: Populate mcp canonical_mappings — amp

**Files:**
- Modify: `docs/provider-formats/amp.yaml` (replace `canonical_mappings: {}` under `mcp:`)

**Depends on:** T34

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted([p for p,cts in d['providers'].items() if 'mcp' in cts and cts['mcp']['canonical_mappings']]))"` → pass — prints all 9 mcp-supported providers

### Step 1: Replace `canonical_mappings: {}` under `mcp:` with

```yaml
    canonical_mappings:
      transport_types:
        supported: false
        mechanism: "Amp MCP transport types not documented beyond basic MCP support"
        confidence: inferred
      oauth_support:
        supported: true
        mechanism: "oauth_support: Amp supports OAuth 2.0 authentication for remote MCP servers"
        confidence: confirmed
      env_var_expansion:
        supported: false
        mechanism: "Amp MCP configuration does not document environment variable expansion"
        confidence: inferred
      tool_filtering:
        supported: true
        mechanism: "per_tool_enable_disable: Amp supports per-tool enable/disable configuration to control which MCP tools are exposed to the agent"
        confidence: confirmed
      auto_approve:
        supported: false
        mechanism: "Amp does not document pre-configured auto-approval for specific MCP tools separate from tool filtering"
        confidence: inferred
      marketplace:
        supported: false
        mechanism: "Amp does not have an in-IDE MCP marketplace"
        confidence: inferred
      resource_referencing:
        supported: false
        mechanism: "Amp does not document MCP resource referencing"
        confidence: inferred
      enterprise_management:
        supported: true
        mechanism: "enterprise_registry_allowlist: Amp supports organization-level MCP configuration with an enterprise registry and allowlist"
        confidence: confirmed
```

### Step 2: Verify mcp batch complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
providers = sorted([p for p, cts in d['providers'].items() if 'mcp' in cts and cts['mcp']['canonical_mappings']])
print(f'mcp batch complete: {providers}')
assert len(providers) == 9
"
```

---

## Task T36: Populate agents canonical_mappings — claude-code

**Files:**
- Modify: `docs/provider-formats/claude-code.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T35

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['claude-code']['agents']['canonical_mappings']))"` → pass — prints `7`

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: true
        mechanism: "Markdown files with YAML frontmatter (.md); file body is the system prompt; stored in .claude/agents/ (project) or ~/.claude/agents/ (user)"
        confidence: confirmed
      tool_restrictions:
        supported: true
        mechanism: "agent_tool_restrictions: tools field (allowlist) and disallowedTools field (denylist) in frontmatter; Agent(type) syntax restricts which subagent types can be spawned"
        confidence: confirmed
      invocation_patterns:
        supported: true
        mechanism: "agent_invocation_patterns: natural language delegation, @agent-<name> mention, session-wide --agent flag at startup"
        confidence: confirmed
      agent_scopes:
        supported: true
        mechanism: "agent_scopes: five scopes — managed settings, CLI --agents flag (session-only), project (.claude/agents/), user (~/.claude/agents/), plugin; highest priority wins"
        confidence: confirmed
      model_selection:
        supported: true
        mechanism: "model frontmatter field: sonnet/opus/haiku alias, full model ID, or inherit; defaults to inherit session model"
        confidence: confirmed
      per_agent_mcp:
        supported: true
        mechanism: "agent_mcp_scoping: mcpServers frontmatter field; inline definitions connected/disconnected with agent lifecycle; string references share parent connection"
        confidence: confirmed
      subagent_spawning:
        supported: true
        mechanism: "agent_resume: SendMessage tool with agent ID for resumption (experimental); main agents spawn subagents via Agent tool with Agent(type) restriction syntax"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T37: Populate agents canonical_mappings — windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T36

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['windsurf']['agents']['canonical_mappings']))"` → pass — prints `7`

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: true
        mechanism: "agents_md_format: Windsurf agents are defined in AGENTS.md files using the shared multi-provider Markdown format"
        confidence: confirmed
      tool_restrictions:
        supported: false
        mechanism: "Windsurf agent definitions do not document per-agent tool allowlist or denylist configuration"
        confidence: inferred
      invocation_patterns:
        supported: false
        mechanism: "Windsurf agents load based on file presence; no explicit invocation pattern syntax documented"
        confidence: inferred
      agent_scopes:
        supported: false
        mechanism: "Windsurf agents are workspace-scoped; no multi-level scope priority model documented"
        confidence: inferred
      model_selection:
        supported: false
        mechanism: "Windsurf agents do not document per-agent model override"
        confidence: inferred
      per_agent_mcp:
        supported: false
        mechanism: "Windsurf agents do not document per-agent MCP server scoping"
        confidence: inferred
      subagent_spawning:
        supported: false
        mechanism: "Windsurf agents do not document spawning or delegating to other agents"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T38: Populate agents canonical_mappings — codex

**Files:**
- Modify: `docs/provider-formats/codex.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T37

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['codex']['agents']['canonical_mappings']))"` → pass — prints `7`

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: true
        mechanism: "agent_roles: Codex agents use role-based configuration format, distinct from AGENTS.md plain markdown"
        confidence: confirmed
      tool_restrictions:
        supported: false
        mechanism: "Codex agent definitions do not document per-agent tool restriction configuration"
        confidence: inferred
      invocation_patterns:
        supported: false
        mechanism: "Codex agents are invoked by role assignment; no @-mention or slash command invocation documented"
        confidence: inferred
      agent_scopes:
        supported: false
        mechanism: "Codex agents are project-scoped; no multi-level scope priority model documented"
        confidence: inferred
      model_selection:
        supported: true
        mechanism: "role_config_layer: Codex supports per-role model configuration to assign different AI models to different agent roles"
        confidence: confirmed
      per_agent_mcp:
        supported: false
        mechanism: "Codex agents do not document per-agent MCP server scoping"
        confidence: inferred
      subagent_spawning:
        supported: true
        mechanism: "spawn_tool_spec: Codex agents can spawn sub-agents using the spawn tool specification"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T39: Populate agents canonical_mappings — copilot-cli

**Files:**
- Modify: `docs/provider-formats/copilot-cli.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T38

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['copilot-cli']['agents']['canonical_mappings']))"` → pass — prints `7`

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: true
        mechanism: "agent_profile_format: Copilot CLI agents use profile files with structured configuration format"
        confidence: confirmed
      tool_restrictions:
        supported: true
        mechanism: "tool_aliases: Copilot CLI agent profiles support tool restriction and exposure configuration"
        confidence: confirmed
      invocation_patterns:
        supported: true
        mechanism: "agent_invocation_modes: Copilot CLI supports multiple agent invocation modes including natural language and explicit selection"
        confidence: confirmed
      agent_scopes:
        supported: true
        mechanism: "agent_scopes: Copilot CLI supports project and personal/user agent scope levels"
        confidence: confirmed
      model_selection:
        supported: false
        mechanism: "Copilot CLI agent profiles do not document per-agent model override"
        confidence: inferred
      per_agent_mcp:
        supported: false
        mechanism: "Copilot CLI agents do not document per-agent MCP server scoping"
        confidence: inferred
      subagent_spawning:
        supported: true
        mechanism: "subagent_execution: Copilot CLI supports spawning and delegating to sub-agents"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T40: Populate agents canonical_mappings — factory-droid

**Files:**
- Modify: `docs/provider-formats/factory-droid.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T39

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['factory-droid']['agents']['canonical_mappings']))"` → pass — prints `7`

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: true
        mechanism: "droid_format: Factory Droid agents (Droids) use a custom format distinct from AGENTS.md"
        confidence: confirmed
      tool_restrictions:
        supported: true
        mechanism: "tool_categories: Factory Droid Droids support tool category configuration to restrict available tools"
        confidence: confirmed
      invocation_patterns:
        supported: false
        mechanism: "Factory Droid Droids are invoked by name; no multi-mode invocation pattern documented"
        confidence: inferred
      agent_scopes:
        supported: true
        mechanism: "droid_precedence: Factory Droid supports global and project Droid definitions; innermost (most specific) layer takes precedence"
        confidence: confirmed
      model_selection:
        supported: true
        mechanism: "model_selection_per_droid: Factory Droid supports per-Droid model selection"
        confidence: confirmed
      per_agent_mcp:
        supported: false
        mechanism: "Factory Droid Droids do not document per-agent MCP server scoping"
        confidence: inferred
      subagent_spawning:
        supported: false
        mechanism: "Factory Droid Droids do not document spawning or delegating to other Droids"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T41: Populate agents canonical_mappings — kiro

**Files:**
- Modify: `docs/provider-formats/kiro.yaml` (replace `canonical_mappings: {}` under `agents:`)

**Depends on:** T40

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted([p for p,cts in d['providers'].items() if 'agents' in cts and cts['agents']['canonical_mappings']]))"` → pass — prints all 6 agents-supported providers

### Step 1: Replace `canonical_mappings: {}` under `agents:` with

```yaml
    canonical_mappings:
      definition_format:
        supported: false
        mechanism: "Kiro does not use a separate agent definition file format; agent behavior is configured through steering files"
        confidence: inferred
      tool_restrictions:
        supported: true
        mechanism: "kiro_agent_tools_settings: Kiro supports per-agent tool restriction configuration"
        confidence: confirmed
      invocation_patterns:
        supported: false
        mechanism: "Kiro does not document explicit agent invocation patterns"
        confidence: inferred
      agent_scopes:
        supported: false
        mechanism: "Kiro agent configuration is project-scoped; no multi-level scope model documented"
        confidence: inferred
      model_selection:
        supported: false
        mechanism: "Kiro does not document per-agent model selection"
        confidence: inferred
      per_agent_mcp:
        supported: true
        mechanism: "kiro_agent_inline_mcp_servers: Kiro supports per-agent inline MCP server definitions scoped to that agent's lifecycle"
        confidence: confirmed
      subagent_spawning:
        supported: false
        mechanism: "Kiro does not document agent spawning or delegation"
        confidence: inferred
```

### Step 2: Verify agents batch complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
providers = sorted([p for p, cts in d['providers'].items() if 'agents' in cts and cts['agents']['canonical_mappings']])
print(f'agents batch complete: {providers}')
assert len(providers) == 6
"
```

---

## Task T42: Populate commands canonical_mappings — claude-code

**Files:**
- Modify: `docs/provider-formats/claude-code.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T41

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['claude-code']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: true
        mechanism: "$ARGUMENTS (all args), $ARGUMENTS[N] / $N (positional, 0-based), ${CLAUDE_SESSION_ID}, ${CLAUDE_SKILL_DIR}; multi-word args must be quoted"
        confidence: confirmed
      builtin_commands:
        supported: true
        mechanism: "builtin_commands: approximately 60 built-in commands coded into the CLI; plus bundled skill commands (/batch, /simplify, /loop, etc.)"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T43: Populate commands canonical_mappings — cline

**Files:**
- Modify: `docs/provider-formats/cline.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T42

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['cline']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: false
        mechanism: "Cline custom commands do not document argument substitution syntax"
        confidence: inferred
      builtin_commands:
        supported: true
        mechanism: "builtin_slash_commands: Cline ships built-in slash commands (/help, /settings, /clear, etc.) alongside user-defined commands"
        confidence: confirmed
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T44: Populate commands canonical_mappings — codex

**Files:**
- Modify: `docs/provider-formats/codex.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T43

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['codex']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: false
        mechanism: "Codex custom commands do not document argument substitution syntax"
        confidence: inferred
      builtin_commands:
        supported: false
        mechanism: "Codex does not document a set of built-in commands distinct from user-defined commands"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T45: Populate commands canonical_mappings — copilot-cli

**Files:**
- Modify: `docs/provider-formats/copilot-cli.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T44

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['copilot-cli']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: false
        mechanism: "Copilot CLI custom commands do not document argument substitution syntax"
        confidence: inferred
      builtin_commands:
        supported: false
        mechanism: "Copilot CLI does not document built-in commands separate from user-defined commands"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T46: Populate commands canonical_mappings — factory-droid

**Files:**
- Modify: `docs/provider-formats/factory-droid.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T45

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['factory-droid']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: true
        mechanism: "two_command_types: Factory Droid supports $ARGUMENTS substitution in command templates for user-provided arguments"
        confidence: confirmed
      builtin_commands:
        supported: false
        mechanism: "Factory Droid does not document built-in commands separate from user-defined commands"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T47: Populate commands canonical_mappings — gemini-cli

**Files:**
- Modify: `docs/provider-formats/gemini-cli.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T46

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['gemini-cli']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: true
        mechanism: "args_placeholder: Gemini CLI command templates support an argument placeholder syntax for user-provided arguments"
        confidence: confirmed
      builtin_commands:
        supported: false
        mechanism: "Gemini CLI does not document built-in commands separate from user-defined commands"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T48: Populate commands canonical_mappings — pi

**Files:**
- Modify: `docs/provider-formats/pi.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T47

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d['providers']['pi']['commands']['canonical_mappings']))"` → pass — prints `2`

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: true
        mechanism: "pi_prompt_template_arguments: Pi supports argument substitution in prompt templates using a documented template syntax"
        confidence: confirmed
      builtin_commands:
        supported: false
        mechanism: "Pi does not document built-in commands separate from user-defined commands"
        confidence: inferred
```

### Step 2: Verify

```bash
cd cli && make test
```

---

## Task T49: Populate commands canonical_mappings — windsurf

**Files:**
- Modify: `docs/provider-formats/windsurf.yaml` (replace `canonical_mappings: {}` under `commands:`)

**Depends on:** T48

### Success Criteria
- `cd cli && make test` → pass
- `cd cli && ./syllago _gencapabilities | python3 -c "import json,sys; d=json.load(sys.stdin); print(sorted([p for p,cts in d['providers'].items() if 'commands' in cts and cts['commands']['canonical_mappings']]))"` → pass — prints all 8 commands-supported providers

### Step 1: Replace `canonical_mappings: {}` under `commands:` with

```yaml
    canonical_mappings:
      argument_substitution:
        supported: false
        mechanism: "Windsurf custom commands do not document argument substitution syntax"
        confidence: inferred
      builtin_commands:
        supported: false
        mechanism: "Windsurf does not document built-in commands separate from user-defined commands"
        confidence: inferred
```

### Step 2: Verify commands batch complete

```bash
cd cli && make test
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
d = json.load(sys.stdin)
providers = sorted([p for p, cts in d['providers'].items() if 'commands' in cts and cts['commands']['canonical_mappings']])
print(f'commands batch complete: {providers}')
assert len(providers) == 8
"
```

---

## Task T50: Final Regeneration and Coverage Validation

**Files:** none modified (read-only verification)

**Depends on:** T49

### Success Criteria
- `cd cli && make test` → pass — all tests pass including `TestGencapabilities_AllRealProviders`
- key count check → pass — prints `44`
- coverage check → pass — no supported non-skills content type has empty canonical_mappings
- content type check → pass — exactly 6 content types in canonical_keys

### Step 1: Run full test suite

```bash
cd cli && make test
```

### Step 2: Verify all design doc success criteria

```bash
cd cli && ./syllago _gencapabilities > /tmp/capabilities-check.json

# Criterion 1: 44 keys across 6 content types
python3 -c "
import json
with open('/tmp/capabilities-check.json') as f:
    d = json.load(f)
total = sum(len(v) for v in d['canonical_keys'].values())
types = sorted(d['canonical_keys'].keys())
assert types == ['agents', 'commands', 'hooks', 'mcp', 'rules', 'skills'], f'Wrong types: {types}'
assert total == 44, f'Expected 44 keys, got {total}'
print(f'Criterion 1 PASS: {total} keys across {types}')
"

# Criterion 2: No supported non-skills content type has empty canonical_mappings
python3 -c "
import json
with open('/tmp/capabilities-check.json') as f:
    d = json.load(f)
failures = []
for provider, cts in d['providers'].items():
    for ct_name, ct_data in cts.items():
        if ct_name == 'skills':
            continue
        if ct_data['status'] == 'supported' and not ct_data['canonical_mappings']:
            failures.append(f'{provider}/{ct_name}')
assert not failures, f'Empty canonical_mappings in supported types: {failures}'
print('Criterion 2 PASS: all supported non-skills types have canonical_mappings')
"

# Criterion 3: _gencapabilities produces output for all 6 content types
python3 -c "
import json
with open('/tmp/capabilities-check.json') as f:
    d = json.load(f)
assert len(d['canonical_keys']) == 6, f'Expected 6 content types in canonical_keys'
print('Criterion 3 PASS: capabilities.json has data for all 6 content types')
"

rm /tmp/capabilities-check.json
```

### Step 3: Verify no forbidden internal fields in output

```bash
cd cli && ./syllago _gencapabilities | python3 -c "
import json, sys
raw = sys.stdin.read()
for forbidden in ['\"confidence\"', '\"graduation_candidate\"', '\"generation_method\"', '\"content_hash\"', '\"fetch_method\"']:
    assert forbidden not in raw, f'Forbidden field {forbidden} found in output'
print('Field filtering PASS: no internal fields in output')
"
```

---

## Task T51: Format Check and Commit Preparation

**Files:** `docs/spec/canonical-keys.yaml`, all 11 modified provider-format YAMLs

**Depends on:** T50

### Success Criteria
- `cd cli && make fmt` → pass — no formatting changes (no Go files modified)
- `cd cli && make test` → pass — final clean test run
- `cd cli && ./syllago _gencapabilities > /dev/null` → pass — clean parse of all modified YAMLs

### Step 1: Final format and test

```bash
cd cli && make fmt && make test
cd cli && ./syllago _gencapabilities > /dev/null && echo "All YAML files parse cleanly"
```

### Step 2: Stage only the files modified in this work

The following files were modified (do not stage cursor.yaml, opencode.yaml, roo-code.yaml, zed.yaml — those were not changed):

```bash
git add docs/spec/canonical-keys.yaml
git add docs/provider-formats/amp.yaml
git add docs/provider-formats/claude-code.yaml
git add docs/provider-formats/cline.yaml
git add docs/provider-formats/codex.yaml
git add docs/provider-formats/copilot-cli.yaml
git add docs/provider-formats/factory-droid.yaml
git add docs/provider-formats/gemini-cli.yaml
git add docs/provider-formats/kiro.yaml
git add docs/provider-formats/pi.yaml
git add docs/provider-formats/windsurf.yaml
```

### Step 3: Commit

```bash
git commit -m "feat: expand canonical-keys.yaml to all 6 content types with 44 keys

Add 31 new canonical keys across rules (5), hooks (9), mcp (8), agents (7),
and commands (2) content types. Populate canonical_mappings in all 11 provider-format
YAMLs that have supported non-skills content types. Add Normative Definitions
header documenting confidence enum, key lifecycle policy, staleness/maintenance,
and public-spec inheritance. No Go code changes — gencapabilities.go is
schema-agnostic and picks up new content types automatically."
```

---

## Files Modified Summary

**Spec:**
- `docs/spec/canonical-keys.yaml` — Normative Definitions header + 5 new content type sections (31 new keys)

**Provider formats (11 files, in order of task work):**
- `docs/provider-formats/claude-code.yaml` — rules, hooks, mcp, agents, commands
- `docs/provider-formats/windsurf.yaml` — rules, hooks, mcp, agents, commands
- `docs/provider-formats/cline.yaml` — rules, hooks, mcp, commands
- `docs/provider-formats/codex.yaml` — rules, hooks, mcp, agents, commands
- `docs/provider-formats/copilot-cli.yaml` — rules, hooks, mcp, agents, commands
- `docs/provider-formats/factory-droid.yaml` — rules, hooks, mcp, agents, commands
- `docs/provider-formats/gemini-cli.yaml` — rules, hooks, mcp, commands
- `docs/provider-formats/kiro.yaml` — rules, hooks, mcp, agents
- `docs/provider-formats/amp.yaml` — rules, hooks, mcp
- `docs/provider-formats/pi.yaml` — rules, hooks, commands

**Not modified** (no supported non-skills content types):
- `docs/provider-formats/cursor.yaml`
- `docs/provider-formats/opencode.yaml`
- `docs/provider-formats/roo-code.yaml`
- `docs/provider-formats/zed.yaml`

**No Go files modified.** `cli/cmd/syllago/gencapabilities.go` and `cli/cmd/syllago/gencapabilities_test.go` require zero changes.

---

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| No Go code changes | Zero changes | `gencapabilities.go` is fully schema-agnostic; new content type keys are picked up automatically |
| Object-type mapping format | Flat `{supported, mechanism, confidence}` with sub-capability detail in mechanism prose | Matches existing pattern for skills `compatibility` and `metadata_map` keys; no YAML schema extension needed |
| `tool_aliases` omission | Not included anywhere | Dropped during design review — false graduation; Kiro and Copilot CLI use the same name for semantically different concepts |
| Cline `always_allow_tools` dual-reference | Both `tool_filtering` and `auto_approve` reference it with distinct mechanism strings | Explicitly required by design doc implementer note; mechanism text must scope the distinction |
| Unsupported entries | Include `supported: false` with mechanism and `confidence: inferred` | Explicit false entries document what was checked; gaps that look unreviewed are more harmful than documented negatives |
| Task sequencing | Sequential within content-type batch | YAML edits to the same file cannot be parallelized safely; checkpoints after each batch verify completeness before the next |
