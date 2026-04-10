# Adding a Provider to Syllago

Step-by-step guide for adding a new AI coding tool as a supported provider. Based on the process used to add Amp (March 2026).

---

## Prerequisites

Before starting, research and document the provider's format for each content type:

- **Rules:** File format, frontmatter fields, file locations (project + global)
- **Skills:** SKILL.md support, frontmatter fields, install directories
- **Commands:** File format, argument placeholders, frontmatter fields
- **Agents:** Structured format or instruction-only, frontmatter fields
- **MCP:** Top-level key, per-server fields, transport types, config file location
- **Hooks:** Event names, configuration format, hook features
- **Detection:** How to detect if the provider is installed

Create a format reference doc at `docs/provider-formats/<slug>.md` using existing docs as templates.

---

## Step 1: Provider Definition

**File:** `cli/internal/provider/<slug>.go`

Create a `Provider` struct with all required fields:

```go
var MyProvider = Provider{
    Name:      "My Provider",
    Slug:      "my-provider",           // Stable identifier, used in CLI flags and switch cases
    ConfigDir: ".my-provider",           // Config directory name (relative to home)
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        // Return directory path for each content type
        // Return JSONMergeSentinel for JSON-merge types (MCP, Hooks)
        // Return "" for unsupported types
    },
    Detect: func(homeDir string) bool {
        // Check for config directory or binary
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        // Return paths to scan for existing content
    },
    FileFormat: func(ct catalog.ContentType) Format {
        // FormatMarkdown, FormatJSON, FormatYAML, FormatMDC, FormatTOML, FormatJSONC
    },
    EmitPath: func(projectRoot string) string {
        // Where syllago writes scan output for this provider
    },
    SupportsType: func(ct catalog.ContentType) bool {
        // Which content types this provider handles
    },
    SymlinkSupport: map[catalog.ContentType]bool{
        // true for filesystem types, false for JSON-merge types
    },
}
```

**Register:** Add the provider to `AllProviders` in `provider.go`:

```go
var AllProviders = []Provider{
    // ... existing providers ...
    MyProvider,
}
```

---

## Step 2: Converter Paths

For each supported content type, add canonicalize and/or render functions.

### Files to Modify

| Content Type | File |
|-------------|------|
| Rules | `cli/internal/converter/rules.go` |
| Skills | `cli/internal/converter/skills.go` |
| Commands | `cli/internal/converter/commands.go` |
| Agents | `cli/internal/converter/agents.go` |
| MCP | `cli/internal/converter/mcp.go` |
| Hooks | `cli/internal/converter/hooks.go` |

### Pattern

Each converter has `Canonicalize(content, sourceProvider)` and `Render(content, targetProvider)` methods.

**Canonicalize** (provider format -> canonical):
```go
// In Canonicalize() switch:
case "my-provider":
    return canonicalizeMyProviderRule(content)
```

**Render** (canonical -> provider format):
```go
// In Render() switch:
case "my-provider":
    return renderMyProviderRule(meta, body)
```

### Common Patterns

**Provider supports full canonical fields:** Use `renderClaudeSkill` pattern (emit all frontmatter).

**Provider supports subset of fields:** Create a provider-specific meta struct with only supported fields. Embed unsupported fields as prose notes via `BuildConversionNotes` / `AppendNotes`. See `renderGeminiSkill`, `renderCursorSkill`, `renderAmpSkill`.

**Provider uses different field names:** Parse provider-specific struct, map to canonical fields. See `canonicalizeOpenCodeAgent` (steps -> maxTurns), `canonicalizeWindsurfMCP` (serverUrl -> url).

**Provider uses different format (TOML, JSONC, etc.):** Use appropriate unmarshaler. See `canonicalizeGeminiCommand` (TOML), `canonicalizeOpencodeMCP` (JSONC).

**Provider uses different top-level key:** Create wrapper struct. See `ampMCPSettings` (amp.mcpServers), `vscodeMCPConfig` (servers), `opencodeMCPConfig` (mcp).

---

## Step 3: Tool Name Translation

If the provider uses different tool names, add mappings to `cli/internal/converter/toolmap.go`.

---

## Step 4: Hook Capabilities (if applicable)

If the provider supports hooks, add entries to:
- `HookCapabilities` in `cli/internal/converter/compat.go`
- `HookOutputCapabilities` in the same file
- `HookProviders()` function
- `hookConfigHints` and `hookScopingNotes` in `cli/internal/converter/skills.go`

---

## Step 5: Build and Verify

```bash
cd cli
make fmt              # Format code
make build            # Build binary
make test             # Run tests
golangci-lint run ./... # Lint check (CI enforces this)
```

### What to Check

1. **Provider appears in list:** `syllago providers`
2. **Detection works:** Provider shows as detected when config dir exists
3. **Import/export works:** Content round-trips through canonical format
4. **TUI shows provider:** Provider cards appear in the TUI
5. **Inspect output:** `syllago inspect` shows the provider

### TUI Golden Files

If TUI visual output changes (provider list, cards, etc.), regenerate golden files:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

---

## Step 6: Documentation

1. **Format reference:** `docs/provider-formats/<slug>.md`
2. **Conversion reference:** Update `docs/cross-provider-conversion-reference.md` if it exists
3. **README:** Update supported providers list if applicable

---

## Checklist

- [ ] Research provider formats for all content types
- [ ] Create `docs/provider-formats/<slug>.md`
- [ ] Create `cli/internal/provider/<slug>.go`
- [ ] Add to `AllProviders` in `provider.go`
- [ ] Add converter paths for each supported content type
- [ ] Add tool name translations (if different tool names)
- [ ] Add hook capabilities (if provider supports hooks)
- [ ] `make fmt && make build && make test && golangci-lint run ./...`
- [ ] Verify CLI: `syllago providers`, `syllago inspect`
- [ ] Update TUI golden files if visual changes
- [ ] Update documentation

---

# Capability Monitoring (capmon) Onboarding

This section covers the capmon-specific workflow for registering a provider's capabilities in the capability monitoring system. Run this after completing the Go implementation steps above.

## Prerequisites

- Provider slug (kebab-case, matches filesystem convention)
- Source documentation URLs

## Steps

### 1. Create the provider source manifest

Write `docs/provider-sources/<slug>.yaml` manually. Use `docs/provider-sources/claude-code.yaml` as a template.

### 2. Create or verify the format reference doc

Write `docs/provider-formats/<slug>.md` — the human-authored ground truth for this provider's skills format.
If you don't have one yet, the inspection bead (Step 4) will generate a draft.

### 3. Fetch and extract

```bash
syllago capmon run --stage=fetch-extract --provider=<slug>
```

### 4. Run the inspection bead

Follow the workflow in `docs/workflows/inspect-provider-skills.md` with `--provider=<slug>`.
This produces `.develop/seeder-specs/<slug>-skills.yaml`.

### 5. Review and approve the seeder spec

Open `.develop/seeder-specs/<slug>-skills.yaml`.
Review `proposed_mappings`. Set `human_action: approve` and `reviewed_at: <ISO timestamp>`.
Optionally run: `syllago capmon validate-spec --provider=<slug>`

### 6. Implement the recognizer

Implement `recognizeXxxSkills()` in `cli/internal/capmon/recognize_<slug_underscored>.go`
using the approved seeder spec as the source of truth.

### 7. Seed the provider

```bash
syllago capmon seed --provider=<slug>
```

### 8. Verify output

Check `docs/provider-capabilities/<slug>.yaml` for a populated `content_types.skills` section
with `confidence: confirmed` entries.

## capmon Checklist

- [ ] Create `docs/provider-sources/<slug>.yaml`
- [ ] Create or verify `docs/provider-formats/<slug>.md`
- [ ] `syllago capmon run --stage=fetch-extract --provider=<slug>`
- [ ] Run inspection bead workflow (`docs/workflows/inspect-provider-skills.md`)
- [ ] Review and approve `.develop/seeder-specs/<slug>-skills.yaml` (`human_action: approve`, `reviewed_at`)
- [ ] `syllago capmon validate-spec --provider=<slug>` (confirm spec passes gate)
- [ ] Implement `cli/internal/capmon/recognize_<slug_underscored>.go`
- [ ] `syllago capmon seed --provider=<slug>`
- [ ] Verify `docs/provider-capabilities/<slug>.yaml` has `confidence: confirmed` entries
- [ ] Confirm `TestAllProviderSlugsRegistered` passes (auto-detects from filesystem)

## Troubleshooting

**Spec gate blocking (`seeder spec for <slug> has not been reviewed`):**
The `validate-spec` command enforces that `human_action` is set to `approve` and `reviewed_at` is a non-empty ISO timestamp. Open `.develop/seeder-specs/<slug>-skills.yaml` and set both fields before re-running.

**Missing cache (fetch-extract returns no output):**
The source manifest at `docs/provider-sources/<slug>.yaml` must contain valid `documentation_urls`. Verify the URLs are reachable and re-run `--stage=fetch-extract`.

**`TestAllProviderSlugsRegistered` fails after adding a new provider:**
This test auto-detects providers from the filesystem (`docs/provider-sources/` and `docs/provider-capabilities/`). Ensure both files exist and the slug matches exactly (kebab-case). No code change is needed — the test picks up new files automatically.

**Inspection bead produces empty `proposed_mappings`:**
The format reference doc (`docs/provider-formats/<slug>.md`) may be missing or incomplete. Add field definitions and re-run the inspection bead workflow.

## Smoke-Testing with a Scratch Provider

To verify that the validate-spec command handles an unknown provider gracefully (no panic, clear error):

```bash
# Create a minimal scratch spec
mkdir -p .develop/seeder-specs
cat > .develop/seeder-specs/scratch-test-skills.yaml << 'EOF'
provider: scratch-test
content_type: skills
format: markdown
format_doc_provenance: human
extraction_gaps: []
source_excerpt: ""
proposed_mappings: []
human_action: ""
reviewed_at: ""
notes: ""
EOF

# Validate with empty human_action — should error
syllago capmon validate-spec --provider=scratch-test

# Set approval and re-validate — should pass
# (edit human_action and reviewed_at in the file, then re-run)

# Clean up
rm .develop/seeder-specs/scratch-test-skills.yaml
```

Expected: first run errors with "seeder spec for scratch-test has not been reviewed"; approved run succeeds.
