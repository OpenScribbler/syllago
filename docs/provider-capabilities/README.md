# Provider Capabilities

This directory contains the authoritative capability baseline for each AI coding tool provider that syllago supports. These files are maintained by the `syllago capmon` pipeline.

## Directory Structure

```
docs/provider-capabilities/
├── <slug>.yaml              # Per-provider capability baseline (one file per provider)
├── by-content-type/         # Generated views grouped by content type (do not edit)
├── compatibility-matrix.md  # Human-readable summary matrix (maintained by hand)
├── schema.json              # JSON Schema for validating *.yaml files
└── README.md                # This file
```

## File Format

Each `<slug>.yaml` follows `schema_version: "1"` (validated by `syllago capmon verify`).

```yaml
schema_version: "1"
slug: claude-code
display_name: Claude Code
last_verified: "2026-04-09"
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: PreToolUse
        blocking: prevent
      # ...
```

## Updating Baselines

The `syllago capmon` pipeline manages these files automatically:

| Command | Description |
|---------|-------------|
| `syllago capmon run` | Full pipeline: fetch → extract → diff → review |
| `syllago capmon run --stage fetch-extract` | Stages 1–2 only (no write permissions needed) |
| `syllago capmon run --stage report` | Stages 3–4 only (reads cached data, creates PRs) |
| `syllago capmon seed --provider <slug>` | Bootstrap or re-seed a single provider's baseline |
| `syllago capmon verify` | Validate all YAML files against the schema |
| `syllago capmon generate` | Regenerate by-content-type views and spec tables |

## Pausing the Pipeline

Create a `.capmon-pause` file in the repo root to prevent Stage 4 (PR/issue creation) from running. Stages 1–3 still execute. Remove the file to resume.

```bash
touch .capmon-pause    # pause
rm .capmon-pause       # resume
```

## Schema Evolution

The `schema_version` field follows a strict evolution policy:

- Current version: `"1"`
- `syllago capmon verify` validates files against the current schema
- `syllago capmon verify --migration-window` also accepts the immediately previous version (for gradual rollouts)
- Never edit `schema.json` or `schema_version` without a corresponding change to the `ValidateAgainstSchema` function in `cli/internal/capmon/capyaml/validate.go`
