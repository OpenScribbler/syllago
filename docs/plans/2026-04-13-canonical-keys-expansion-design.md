# Canonical Keys Expansion — Design Document

**Goal:** Expand canonical-keys.yaml from skills-only (13 keys) to all 6 content types (44 keys total), and populate canonical_mappings in all 14 provider-format YAMLs.

**Decision Date:** 2026-04-13

---

## Problem Statement

canonical-keys.yaml only defines 13 keys for skills. The other 5 content types (rules, hooks, mcp, agents, commands) have empty canonical_mappings in all 14 provider-format YAMLs, despite having rich provider_extensions data documenting cross-provider capability differences.

This blocks:
- **syllago-docs**: Auto-generated per-content-type pages (canonical support tables, provider comparisons) for non-skills types
- **syllago-jtafb (capmon)**: Monitoring capability changes across all content types, not just skills
- **Public interchange specs**: Each content type needs a capability registry; canonical keys are the starting vocabulary

## Proposed Solution

Add 31 new canonical keys across 5 content types. Each key represents a cross-provider concept that 2+ providers implement with different names/mechanisms. The canonical key provides a vendor-neutral name for the concept.

**Key selection rule:** A canonical key is created when 2+ providers have provider_extensions with the same semantic meaning, even if named differently. The canonical name is vendor-neutral. If all providers already use the same name, we use that name. Otherwise, we pick the best name that covers the concept without being tied to any specific provider.

## Architecture

### Three-Layer Model

```
Public Spec Repo (interchange formats, community-owned)
  Each content type spec has a capability registry
  Canonical keys are candidates for those registries

Syllago (reference implementation)
  docs/spec/canonical-keys.yaml     ← vocabulary (this work)
  docs/provider-formats/*.yaml      ← provider mappings (this work)
  capabilities.json                 ← generated output

syllago-docs (documentation site)
  sync-capabilities.ts pipeline     ← reads capabilities.json
  Auto-generates per-content-type pages (future work, separate repo)
```

### Content Types Are Independent

Each content type has its own complete key set. No shared base vocabulary. Even if key names overlap across types (e.g., a hypothetical "description" in both skills and rules), they are independently defined. Rationale: the same concept name may have different semantics, mechanisms, and provider support per content type.

### Relationship to Public Interchange Specs

The hooks interchange spec (docs/spec/hooks/) demonstrates the pattern:
- **Capability registry** (capabilities.md) lists features with support matrices, inference rules, and degradation strategies
- **Our canonical keys** map to capability registry entries — each key identifies a capability dimension

When interchange specs are created for rules, MCP, agents, and commands, their capability registries should be seeded from these canonical keys. The specs are community-owned and live in a separate public repo; syllago implements them.

## Key Definitions

### Rules (5 new keys)

| Key | Type | Description |
|-----|------|-------------|
| `activation_mode` | object | How rules decide when to load into context. Providers implement varying modes: always-on, conditional/glob-based, model-decision, and manual activation. Tracks which modes the provider supports and how they are configured. |
| `file_imports` | bool | Whether rules can reference or include content from other files. Mechanisms vary: @-import syntax, @file.md directives, @path/to/file references. |
| `cross_provider_recognition` | object | Which rule file formats from other providers this provider recognizes. Contents: `recognized_formats` (list of filenames like `AGENTS.md`, `.cursorrules`, `.windsurfrules`). **Minimum qualification:** supported when the provider reads at least one rule-file format defined by a different provider. |
| `auto_memory` | bool | Whether the provider auto-generates persistent rules from conversation context. Distinct from user-authored rules — these are AI-created and stored separately. |
| `hierarchical_loading` | bool | Whether rules load from multiple directory levels with defined precedence. Enables project-root rules plus subdirectory-scoped overrides. |

**Provider evidence:**

| Key | Providers with this concept |
|-----|---------------------------|
| `activation_mode` | Windsurf (`activation_modes`), Kiro (`kiro_steering_inclusion_mode`), Cline (`conditional_rules_paths`) |
| `file_imports` | Claude Code (`file_imports`), Gemini CLI (`file_imports`), Kiro (`kiro_steering_file_references`) |
| `cross_provider_recognition` | Codex (`agents_md_filename`), Copilot CLI (`agents_md_instructions`), Factory Droid (`agents_md_format`), Kiro (`kiro_agents_md_recognition`), Windsurf (`agents_md_auto_scoping`), Cline (`multi_source_rule_detection`) |
| `auto_memory` | Claude Code (`auto_memory`), Windsurf (`auto_generated_memories`), Gemini CLI (`memory_command`) |
| `hierarchical_loading` | Gemini CLI (`hierarchical_context_loading`), Codex (`hierarchical_agents_md`), Copilot CLI (repo-wide + path-specific + personal), Factory Droid (global + project, innermost precedence) |

### Hooks (9 new keys)

| Key | Type | Description |
|-----|------|-------------|
| `handler_types` | object | What kinds of executors hooks support beyond shell commands. May include HTTP endpoints, LLM prompt evaluation, multi-turn agent handlers, or TypeScript extensions. |
| `matcher_patterns` | bool | Whether hooks can filter which tools or events they respond to using name matching, regex patterns, or structured criteria. |
| `decision_control` | object | Which decision actions hooks can take on the triggering action. Contents: `{block: bool, allow: bool, modify: bool}`. Mechanisms include exit code contracts, JSON decision fields, or cancel flags. **Boundary:** `decision_control` governs whether a tool invocation proceeds; see `permission_control` for whether a tool is available at all. |
| `input_modification` | bool | Whether hooks can modify tool input arguments before the tool executes. Safety-critical capability — silent degradation creates false security. **Minimum qualification:** supported when the provider provides any mechanism to modify tool input arguments before execution (e.g., `hookSpecificOutput.updatedInput`, plugin-mutable args, or equivalent). |
| `async_execution` | bool | Whether hooks can run asynchronously without blocking the agent's execution loop. Fire-and-forget semantics. |
| `hook_scopes` | object | Where hooks can be configured and the precedence model when multiple scopes define hooks for the same event. Common scopes: global/user, project, workspace, managed/enterprise. |
| `json_io_protocol` | bool | Whether hooks communicate with the host via structured JSON on stdin/stdout rather than plain text or exit codes alone. |
| `context_injection` | bool | Whether hooks can inject messages, system prompts, or conversation context into the agent's active session. |
| `permission_control` | bool | Whether hooks can make or influence permission decisions determining whether a tool is available for invocation. **Minimum qualification:** supported when the provider allows hooks to return permission decisions of any kind (grant, deny, or ask). **Boundary:** `permission_control` governs whether a tool is available; see `decision_control` for invocation-flow control. |

**Provider evidence:**

| Key | Providers with this concept |
|-----|---------------------------|
| `handler_types` | Claude Code (`hook_handler_types`), Codex (`hook_handler_types`), Pi (`pi_extension_typescript_native`) |
| `matcher_patterns` | Claude Code (`hook_matcher_patterns`), Gemini CLI (`hook_matchers`), Codex (`hook_matcher`), Amp (`hook_match_input_contains`) |
| `decision_control` | Claude Code (`hook_decision_control`), Cline (`pre_tool_use_cancellation`), Copilot CLI (`pre_tool_use_deny`), Codex (`hook_result_abort`), Gemini CLI (`exit_code_semantics`), Factory Droid (`hook_exit_code_behavior`) |
| `input_modification` | Claude Code (`hook_input_modification`), Codex (`hook_updated_input`), Pi (`pi_extension_tool_call_blocking`), Cline (`context_modification_output`) |
| `async_execution` | Claude Code (`hook_async_execution`), Codex (`hook_execution_mode`) |
| `hook_scopes` | Claude Code (`hook_scopes`), Windsurf (`three_config_scopes`), Cline (`global_and_project_hooks`), Codex (`hook_scope`) |
| `json_io_protocol` | Windsurf (`json_stdin_context`), Gemini CLI (`hook_io_protocol`) |
| `context_injection` | Codex (`hook_system_message`), Cline (`context_modification_output`) |
| `permission_control` | Claude Code (`hook_permission_update_entries`), Codex (`hook_permission_decision`), Amp (`permissions_system`) |

### MCP (8 new keys)

| Key | Type | Description |
|-----|------|-------------|
| `transport_types` | object | Which MCP transport protocols the provider supports. Common transports: stdio (local process), SSE (Server-Sent Events), HTTP/streamable-HTTP (stateless or streaming). |
| `oauth_support` | bool | Whether the provider supports OAuth 2.0 authentication for remote MCP servers, including token storage and automatic refresh. |
| `env_var_expansion` | bool | Whether MCP server configuration supports environment variable expansion. Reduces the need for hardcoded secrets in config files. |
| `tool_filtering` | object | Which per-server tool filtering mechanisms the provider supports. Contents: `{allowlist: bool, blocklist: bool, disable_flag: bool}`. Controls which server tools are exposed to the agent. **Boundary:** `tool_filtering` governs tool visibility (what appears in the agent's available tool set); see `auto_approve` for execution gating of visible tools. |
| `auto_approve` | bool | Whether specific MCP tools or entire servers can be configured for automatic approval without per-invocation user confirmation. **Boundary:** `auto_approve` governs execution gating (user prompt suppression) for tools that are already visible; see `tool_filtering` for tool visibility control. |
| `marketplace` | bool | Whether the provider offers an in-IDE MCP server discovery and installation experience. |
| `resource_referencing` | bool | Whether MCP resources (not just tools) can be accessed, typically via @-mention syntax or similar referencing mechanisms. |
| `enterprise_management` | bool | Whether the provider supports organization-level MCP configuration management, including managed server lists, allowlists, and enterprise registries. |

**Provider evidence:**

| Key | Providers with this concept |
|-----|---------------------------|
| `transport_types` | Claude Code (`mcp_transport_types`), Windsurf (`three_transport_types`), Gemini CLI (`transport_types`), Cline (`sse_transport_support`) |
| `oauth_support` | Claude Code (`mcp_oauth_authentication`), Amp (`oauth_support`), Codex (`mcp_oauth_support`) |
| `env_var_expansion` | Claude Code (`mcp_env_var_expansion`), Gemini CLI (`env_variable_expansion`), Windsurf (`config_interpolation`) |
| `tool_filtering` | Gemini CLI (`tool_filtering`), Kiro (`kiro_mcp_disabled_tools`), Codex (`mcp_enabled_disabled_tools`), Amp (`per_tool_enable_disable`), Cline (`always_allow_tools`), Copilot CLI (`mcp_tool_allow_deny_flags`) |
| `auto_approve` | Kiro (`kiro_mcp_auto_approve`), Cline (`always_allow_tools`), Codex (`mcp_per_tool_approval`) |

**Implementer note — Cline overlap:** Cline's `always_allow_tools` extension appears as evidence for both `tool_filtering` and `auto_approve`. Cline conflates visibility control and execution gating into a single config surface. When populating Cline's provider-format YAML, both canonical_mappings will reference `always_allow_tools`, and the `mechanism` field must clarify the scope distinction: under `tool_filtering` it should describe the allowlist behavior; under `auto_approve` it should describe the prompt-suppression behavior. This is the expected resolution per the boundary predicates, not a failure case.
| `marketplace` | Windsurf (`mcp_marketplace`), Cline (`mcp_marketplace`) |
| `resource_referencing` | Claude Code (`mcp_resources`), Gemini CLI (`resource_referencing`) |
| `enterprise_management` | Claude Code (`mcp_managed_config`), Amp (`enterprise_registry_allowlist`), Windsurf (`enterprise_whitelist_and_registry`) |

### Agents (7 new keys)

| Key | Type | Description |
|-----|------|-------------|
| `definition_format` | string | What format agent definitions use. Varies widely: Markdown with YAML frontmatter, JSON config files, TOML sections, or AGENTS.md plain markdown. |
| `tool_restrictions` | bool | Whether agents can be restricted to specific tools via allowlists, denylists, tool categories, or per-tool configuration maps. |
| `invocation_patterns` | object | How agents are triggered. Mechanisms include natural language detection, @-mention syntax, slash commands, CLI flags, or automatic delegation. |
| `agent_scopes` | object | Where agent definitions can live and the priority ordering when definitions exist at multiple levels. Common scopes: project, user/personal, managed/enterprise, CLI-defined. |
| `model_selection` | bool | Whether per-agent model overrides are supported, allowing different agents to use different AI models. |
| `per_agent_mcp` | bool | Whether agents can have their own MCP server configuration, scoping which external tools each agent can access. |
| `subagent_spawning` | bool | Whether agents can spawn, delegate to, or resume other agents. Enables multi-agent coordination patterns. |

**Provider evidence:**

| Key | Providers with this concept |
|-----|---------------------------|
| `definition_format` | Claude Code (`agent_file_format`), Copilot CLI (`agent_profile_format`), Factory Droid (`droid_format`), Windsurf (`agents_md_format`), Codex (`agent_roles`) |
| `tool_restrictions` | Claude Code (`agent_tool_restrictions`), Kiro (`kiro_agent_tools_settings`), Factory Droid (`tool_categories`), Copilot CLI (`tool_aliases`) |
| `invocation_patterns` | Claude Code (`agent_invocation_patterns`), Copilot CLI (`agent_invocation_modes`) |
| `agent_scopes` | Claude Code (`agent_scopes`), Copilot CLI (`agent_scopes`), Factory Droid (`droid_precedence`) |
| `model_selection` | Factory Droid (`model_selection_per_droid`), Codex (`role_config_layer`) |
| `per_agent_mcp` | Claude Code (`agent_mcp_scoping`), Kiro (`kiro_agent_inline_mcp_servers`) |
| `subagent_spawning` | Claude Code (`agent_resume`), Codex (`spawn_tool_spec`), Copilot CLI (`subagent_execution`) |

**Dropped during review:** `tool_aliases` was initially proposed but flagged by panel review as false graduation — Kiro's `kiro_agent_tool_aliases` (remapping tool names within agent config) and Copilot CLI's `tool_aliases` (exposing tools under alternative names) describe different concepts that happened to use the same string. Held until a third provider confirms a shared semantic, or narrowed to one of the two implementations explicitly.

### Commands (2 new keys)

| Key | Type | Description |
|-----|------|-------------|
| `argument_substitution` | object | How user-provided arguments are injected into command templates. Mechanisms vary: $ARGUMENTS, {{args}}, positional $1/$2/${@:N}, and other interpolation syntaxes. |
| `builtin_commands` | bool | Whether the provider ships default/built-in commands alongside user-defined custom commands. |

**Provider evidence:**

| Key | Providers with this concept |
|-----|---------------------------|
| `argument_substitution` | Gemini CLI (`args_placeholder`), Pi (`pi_prompt_template_arguments`), Factory Droid (`two_command_types` — includes $ARGUMENTS) |
| `builtin_commands` | Claude Code (`builtin_commands`), Cline (`builtin_slash_commands`) |

## Data Flow

### What changes in this repo

1. **docs/spec/canonical-keys.yaml** — Add 5 new content_types sections (rules, hooks, mcp, agents, commands) with the 32 key definitions above. Format matches existing skills section: description + type per key.

2. **docs/provider-formats/*.yaml** — For each of the 14 provider-format YAMLs, populate `canonical_mappings` under each content type where the provider has status: supported. Each mapping entry follows the existing format:
   ```yaml
   canonical_mappings:
     <key_name>:
       supported: true|false
       mechanism: "How the provider implements this"
       confidence: confirmed|inferred|unknown
   ```

3. **capabilities.json** — Running `./syllago _gencapabilities` will automatically pick up the new canonical_mappings and include them in the generated output.

### What this enables downstream

- **syllago-docs**: `sync-capabilities.ts` can generate per-content-type canonical support tables, provider comparison pages, and provider-specific mapping sections for all 6 content types (not just skills). This is a separate design doc in the docs repo.
- **capmon (syllago-jtafb)**: New monitoring targets for each canonical key. Change detection across all content types.
- **Public specs**: When interchange specs are created for rules, MCP, agents, and commands, their capability registries can be seeded from these canonical keys.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Independent key sets per type | Yes — no shared base vocabulary | Same concept name may have different semantics per content type. "description" for a hook is not the same as "description" for a skill. |
| Graduation threshold | 2+ providers with same semantic concept | Canonical keys normalize cross-provider terminology. One provider's unique feature stays in provider_extensions until another provider adds something similar. |
| Key naming | Vendor-neutral snake_case | Consistent with existing skills keys. Never tied to a specific provider's terminology. |
| Type assignments | bool for yes/no capabilities, string for descriptive values, object for structured multi-faceted data | Follows existing skills pattern. Most keys are bool (capability check); a few are object (structured capability data like transport types, scopes, activation modes). |

## Normative Definitions

### Confidence Enum

Every `canonical_mapping` entry carries a `confidence` field with one of three values. These tiers are normatively defined as follows:

| Value | Definition |
|-------|------------|
| `confirmed` | Verified directly against provider documentation, first-party test, or authoritative source code. The mapping author can cite a specific primary source. |
| `inferred` | Extrapolated from related behavior, secondary sources, partial evidence, or behavioral observation. No primary citation available, but reasoning is documented in the `mechanism` field. |
| `unknown` | Default when a capmon sweep cannot establish either of the above. Indicates the mapping has not been actively verified and should be treated as a research lead, not ground truth. |

Two contributors applying the same evidence to the same provider capability must reach the same confidence tier. Capmon sweeps may downgrade confidence to `unknown` when a sweep cannot verify current provider behavior.

### Key Lifecycle Policy

The canonical keys vocabulary evolves. The following rules govern changes:

- **Add:** A new key may be introduced when 2+ providers demonstrate the same semantic concept (see graduation threshold in "Proposed Solution").
- **Rename:** When a canonical key is renamed, the old name remains as an alias for at least one capmon cycle. The alias is recorded in canonical-keys.yaml under an `aliases` field on the new key.
- **Deprecate:** A key may be deprecated when the underlying concept no longer reflects current provider reality (e.g., all providers converge on a different abstraction). Deprecated keys are marked with a `deprecated: true` flag and a `replacement` pointer for at least one capmon cycle before removal.
- **Split:** When a canonical key is split into multiple keys, the original name is retained as an alias mapping to the most common interpretation, with deprecation applied to the alias after one cycle.
- **Remove:** Removal occurs only after the alias/deprecation period and is recorded in the canonical-keys.yaml changelog.

Lifecycle changes are coordinated edits to canonical-keys.yaml and the provider-format YAMLs in the same commit — there is no external party holding a stale reference.

**Post-publication:** Once a key is published in a community-owned spec capability registry (see Public-Spec Inheritance below), changes to that key are governed by the spec's change process, not this lifecycle policy. The syllago canonical-keys.yaml entry becomes a derived reference synchronized from the upstream spec, not the authoritative source. Syllago-specific lifecycle rules continue to apply only to keys that remain internal.

### Staleness and Maintenance

Canonical mappings are a snapshot of provider behavior at a point in time. Maintenance is handled by capmon (syllago-jtafb) via scheduled sweeps that re-verify provider documentation and detect changes. When capmon cannot verify a mapping, confidence is downgraded to `unknown` and the entry is flagged for human review.

### Public-Spec Inheritance

When canonical keys feed into public interchange specs (as this design anticipates for rules, MCP, agents, and commands), those keys inherit the normative precision requirements of the destination spec. Any ambiguity tolerated in an internal-only vocabulary must be resolved before the key is published in a community-owned spec capability registry. This design's minimum-qualification statements and boundary predicates are written with that future migration in mind.

## Success Criteria

1. canonical-keys.yaml has 44 keys across 6 content types
2. All 14 provider-format YAMLs have populated canonical_mappings for every supported content type
3. `./syllago _gencapabilities` produces capabilities.json with data for all 6 content types
4. No content type has empty canonical_mappings where the provider reports status: supported
5. Confidence enum definitions, key lifecycle policy, and public-spec inheritance clause are present in canonical-keys.yaml header documentation

## Resolved During Design

| Question | Decision | Reasoning |
|----------|----------|-----------|
| Shared base keys vs independent? | Independent per content type | Same concept name may differ in semantics across types. Holden's call: "just because something has a display name or description does not mean it will behave the same." |
| What threshold for inclusion? | 2+ providers with same semantic meaning | Canonical keys exist to normalize cross-provider terminology. The graduation criterion is semantic overlap, not feature count. |
| How do canonical keys relate to public specs? | Keys are vocabulary candidates for spec capability registries | Specs are community-owned, live in separate repo. Syllago implements them. Canonical keys inform spec design but don't replace it. |
| Scope of this work vs docs-site expansion? | Separate — this repo first, docs-site follows | Docs-site pipeline expansion depends on this YAML data. This work is the prerequisite. |
| Should composite bool keys be promoted to object type? | Per-key decision, 3 promoted / 2 kept with min-qualification | Panel review (round 3 unanimous approve) surfaced that several bool keys collapsed meaningful sub-capabilities. Promoted: `decision_control`, `tool_filtering`, `cross_provider_recognition`. Kept bool with minimum-qualification: `input_modification`, `permission_control`. |
| Should `tool_aliases` ship in v1? | No — dropped | Panel flagged false graduation: Kiro and Copilot CLI use the same name for different concepts (within-agent remapping vs alternative name exposure). Holds until a third provider confirms a shared semantic. |
| How are boundary-ambiguous keys disambiguated? | Testable boundary predicates in key descriptions | Panel flagged `permission_control`/`decision_control` and `tool_filtering`/`auto_approve` as classification-ambiguous. Added explicit boundary predicates distinguishing invocation-flow vs tool-availability, and visibility vs execution-gating. |
| Is the confidence enum normatively defined? | Yes — added Normative Definitions section | Panel flagged `confirmed`/`inferred`/`unknown` as producing divergent classifications without a written definition. Defined by evidence source type (primary citation vs behavioral inference vs unverified). |
| Is there a key lifecycle policy? | Yes — add/rename/deprecate/split/remove rules added | Panel flagged the absence of explicit evolution rules. Added a light-touch policy: aliases on rename, deprecation pointer before removal, one capmon cycle minimum before removal. |

### Panel Review Record

The design was reviewed by a 5-agent panel (Remy, solo-publisher, platform-vendor, registry-operator, spec-purist) over 3 rounds, reaching unanimous approve in round 3 conditional on the fixes recorded above. Panel transcript is preserved at `.scratch/panel/bus.jsonl` for audit.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
