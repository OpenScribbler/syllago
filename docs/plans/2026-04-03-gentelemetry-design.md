# _gentelemetry Command — Design Document

**Goal:** A hidden CLI command that generates `telemetry.json` — a machine-readable event catalog for the docs site, keeping telemetry documentation automatically in sync with the code.

**Decision Date:** 2026-04-03

---

## Problem Statement

Telemetry events and their properties are defined across 12 command files as `telemetry.Enrich()` calls. The telemetry docs page is hand-maintained and can drift from the actual code. There's no machine-readable reference for the docs site to consume, unlike `commands.json` (from `_gendocs`) and `providers.json` (from `_genproviders`) which are auto-generated and included in releases.

## Proposed Solution

A `_gentelemetry` hidden command that reads from a Go-defined event catalog and outputs structured JSON to stdout. The catalog is the single source of truth — a drift-detection test ensures it stays in sync with actual `Enrich()` calls in the codebase. A rule file reminds developers to add telemetry when creating new commands.

## Architecture

### Component 1: Event Catalog (`cli/internal/telemetry/catalog.go`)

Go structs and functions that define the complete telemetry surface:

```go
type EventDef struct {
    Name        string        `json:"name"`
    Description string        `json:"description"`
    FiredWhen   string        `json:"firedWhen"`
    Properties  []PropertyDef `json:"properties"`
}

type PropertyDef struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"`        // "string", "int", "bool"
    Description string   `json:"description"`
    Example     any      `json:"example"`
    Commands    []string `json:"commands"`     // which commands set this property
}

type PrivacyEntry struct {
    Category string `json:"category"`
    Examples string `json:"examples"`
}

// EventCatalog returns the complete event definition list.
func EventCatalog() []EventDef { ... }

// StandardProperties returns properties included in every event automatically.
func StandardProperties() []PropertyDef { ... }

// NeverCollected returns the privacy guarantees (what is never tracked).
func NeverCollected() []PrivacyEntry { ... }
```

### Component 2: Hidden Command (`cli/cmd/syllago/gentelemetry.go`)

Follows the exact pattern of `gendocs.go` and `genproviders.go`:

- Hidden command registered in `init()`
- Calls catalog functions, wraps in manifest struct
- Outputs indented JSON to stdout

### Component 3: Output Format (`telemetry.json`)

Event-centric structure — one entry per event, properties list which commands use them:

```json
{
  "version": "1",
  "generatedAt": "2026-04-03T...",
  "syllagoVersion": "0.7.0",
  "events": [
    {
      "name": "command_executed",
      "description": "Fired when a CLI command completes successfully",
      "firedWhen": "PersistentPostRun (every non-telemetry command)",
      "properties": [
        {
          "name": "command",
          "type": "string",
          "description": "Command name (cobra command path)",
          "example": "install",
          "commands": ["*"]
        },
        {
          "name": "provider",
          "type": "string",
          "description": "Target provider slug",
          "example": "claude-code",
          "commands": ["install", "uninstall", "loadout_apply", "sandbox_run", "sync-and-export"]
        },
        {
          "name": "content_type",
          "type": "string",
          "description": "Content type filter or specific type",
          "example": "rules",
          "commands": ["install", "add", "convert", "uninstall", "remove", "list", "create", "share", "sync-and-export", "registry_items"]
        },
        {
          "name": "content_count",
          "type": "int",
          "description": "Number of content items affected",
          "example": 3,
          "commands": ["install", "add", "list", "registry_items"]
        },
        {
          "name": "dry_run",
          "type": "bool",
          "description": "Whether --dry-run flag was used",
          "example": false,
          "commands": ["install", "add", "uninstall", "remove", "sync-and-export"]
        },
        {
          "name": "from",
          "type": "string",
          "description": "Source provider slug",
          "example": "cursor",
          "commands": ["add"]
        },
        {
          "name": "from_provider",
          "type": "string",
          "description": "Source provider for conversion",
          "example": "cursor",
          "commands": ["convert"]
        },
        {
          "name": "to_provider",
          "type": "string",
          "description": "Target provider for conversion",
          "example": "claude-code",
          "commands": ["convert"]
        },
        {
          "name": "source_filter",
          "type": "string",
          "description": "Content source filter (library, shared, registry)",
          "example": "library",
          "commands": ["list"]
        },
        {
          "name": "mode",
          "type": "string",
          "description": "Loadout application mode",
          "example": "try",
          "commands": ["loadout_apply"]
        },
        {
          "name": "action_count",
          "type": "int",
          "description": "Number of actions performed by loadout",
          "example": 5,
          "commands": ["loadout_apply"]
        },
        {
          "name": "registry_count",
          "type": "int",
          "description": "Number of registries involved",
          "example": 2,
          "commands": ["registry_sync"]
        },
        {
          "name": "item_count",
          "type": "int",
          "description": "Number of items in result set",
          "example": 12,
          "commands": ["list", "registry_items"]
        }
      ]
    },
    {
      "name": "tui_session_started",
      "description": "Fired when the TUI exits normally",
      "firedWhen": "After tea.Program.Run() completes without error",
      "properties": [
        {
          "name": "success",
          "type": "bool",
          "description": "TUI exited normally",
          "example": true,
          "commands": ["(root)"]
        }
      ]
    }
  ],
  "standardProperties": [
    {"name": "version", "type": "string", "description": "Syllago version", "example": "0.7.0"},
    {"name": "os", "type": "string", "description": "Operating system", "example": "linux"},
    {"name": "arch", "type": "string", "description": "CPU architecture", "example": "amd64"}
  ],
  "neverCollected": [
    {"category": "File contents", "examples": "Rule text, skill prompts, hook commands, MCP configs"},
    {"category": "File paths", "examples": "/home/user/.claude/rules/my-secret-rule"},
    {"category": "User identity", "examples": "Usernames, hostnames, IP addresses, email"},
    {"category": "Registry URLs", "examples": "Git clone URLs, registry names"},
    {"category": "Content names", "examples": "Names of rules, skills, agents you manage"},
    {"category": "Interaction details", "examples": "Keystrokes, mouse clicks, TUI navigation"}
  ]
}
```

### Component 4: Tests (`cli/cmd/syllago/gentelemetry_test.go`)

Following `genproviders_test.go` patterns:

| Test | What it checks |
|------|---------------|
| `TestGentelemetry` | Valid JSON, version "1", syllagoVersion set, events non-empty |
| `TestGentelemetry_EventsComplete` | Every event has name, description, firedWhen, non-empty properties |
| `TestGentelemetry_PropertiesComplete` | Every property has name, type (string/int/bool), description, example, commands |
| `TestGentelemetry_StandardProperties` | version, os, arch present |
| `TestGentelemetry_PrivacyGuarantees` | At least 5 entries covering file contents, paths, identity, URLs, content names |
| `TestGentelemetry_CatalogMatchesEnrichCalls` | Scans `cmd/syllago/*.go` for `telemetry.Enrich("key"` patterns, verifies each key exists in the catalog. **Strict failure** — same behavior as genproviders hook event completeness tests. |

### Component 5: Integration

**Makefile** — extend `gendocs` target:
```makefile
gendocs: build
    ./$(OUTPUT) _gendocs > commands.json
    ./$(OUTPUT) _genproviders > providers.json
    ./$(OUTPUT) _gentelemetry > telemetry.json
```

**Release workflow** — add to the Generate step:
```yaml
./syllago-gendocs _gentelemetry > telemetry.json
```

Include `telemetry.json` in checksums and release assets.

**Pre-push hook** — extend existing freshness check to also verify `telemetry.json`.

### Component 6: Rule File (`.claude/rules/telemetry-enrichment.md`)

Scoped to `cli/cmd/syllago/*_cmd.go` and `cli/cmd/syllago/*.go` files with `RunE` functions. Reminds developers to:

1. Add `telemetry.Enrich()` calls for relevant properties (provider slugs, content types, counts, boolean flags — never paths, names, or PII)
2. Update the event catalog in `cli/internal/telemetry/catalog.go` if introducing new property keys
3. Run `make gendocs` to regenerate `telemetry.json`

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Source of truth | Go catalog in telemetry package | Matches runtime-data pattern of _gendocs/_genproviders; drift test catches forgotten updates |
| Output format | Event-centric JSON | Clean for docs rendering — one table per event, properties list which commands use them |
| Privacy guarantees | Structured data in JSON | Same source powers CLI status output + docs site, stays in sync |
| Drift detection | Strict test failure | Same as genproviders hook event completeness — CI blocks on mismatch |
| Freshness check | Pre-push hook | Same as commands.json — can't push stale telemetry docs |
| Rule file | Scoped to CLI command files | Catches forgotten telemetry at authoring time, not just at test time |

## Data Flow

```
Developer adds Enrich() call → updates catalog.go → runs make gendocs
                                                        ↓
                                              _gentelemetry reads catalog
                                                        ↓
                                              telemetry.json written to cli/
                                                        ↓
                                         pre-push hook verifies freshness
                                                        ↓
                                    release workflow includes in assets + checksums
                                                        ↓
                                        docs site consumes telemetry.json
```

## Error Handling

- If catalog has duplicate property names within an event: test catches it
- If Enrich() key not in catalog: `TestGentelemetry_CatalogMatchesEnrichCalls` fails
- If telemetry.json is stale: pre-push hook blocks push
- If developer forgets Enrich() on a new command: rule file reminds; PersistentPostRun still tracks command name automatically (baseline)

## Success Criteria

1. `make gendocs` produces valid `telemetry.json` alongside `commands.json` and `providers.json`
2. All tests pass including the drift-detection scan
3. Pre-push hook blocks on stale `telemetry.json`
4. Release workflow includes `telemetry.json` in assets
5. Rule file triggers when editing command files

---

## Next Steps

Ready for implementation planning with `Plan` skill.
