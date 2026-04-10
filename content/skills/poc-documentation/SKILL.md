---
name: poc-documentation
description: |
  Mechanical skill that runs the Aembit POC document assembler to generate PDFs from YAML recipe files. Use when generating POC Guide or Implementation Guide PDFs for a customer.

  Trigger: /poc-documentation <recipe-path-or-directory>

  <example>
  Context: SE has YAML recipes and wants to generate PDFs
  user: "/poc-documentation bcp_poc_guide.yaml"
  assistant: "Generating BCP_POC_Guide.pdf..."
  <commentary>
  Checks venv, runs assembler for the specified recipe, reports output path and offers to open.
  </commentary>
  </example>
---

# POC Documentation

Generates customer POC PDFs by running the Aembit document assembler against YAML recipe files.

## Quick Reference

**Assembler location:** `~/.claude/skills/poc-documentation/assembler.py`

**Recipes location:** SE's current working directory (customer recipes are transitory work product, not stored in the skill)

**Venv:** `~/.claude/skills/poc-documentation/.venv/` (persistent, auto-created if missing)

**Assembler command (single recipe):**
```bash
~/.claude/bin/poc-doc generate <recipe_path>
```

## Usage Patterns

| Invocation | Behavior |
|------------|----------|
| `~/.claude/bin/poc-doc generate bcp_poc_guide.yaml` | Generate one PDF from a recipe in cwd |
| `~/.claude/bin/poc-doc generate /absolute/path/to/recipe.yaml` | Generate one PDF from an absolute path |
| `~/.claude/bin/poc-doc generate .` | Generate all `*.yaml` files in cwd |
| `~/.claude/bin/poc-doc generate . --open` | Generate all and open PDFs |
| `~/.claude/bin/poc-doc validate recipe.yaml` | Validate recipe without generating |

Recipe paths are resolved against the SE's **current working directory**. Absolute paths are also accepted.

## Execution Workflow

### Step 1: Resolve Paths

- If the argument is an absolute path, use it as-is
- If the argument is a relative path or filename, resolve it against the current working directory
- If the argument is `.` or a directory, glob for all `*.yaml` files in that directory
- If no matching files found, report and stop

### Step 2: Run Assembler

For each recipe file, run:

```bash
~/.claude/bin/poc-doc generate <absolute_recipe_path>
```

The wrapper handles venv creation, dependency installation, and pandoc detection automatically.

Report progress per file:

```
Generating <output_filename>...
  Recipe: <recipe_path>
Done: <output_path>
```

If the assembler prints WARNING lines to stderr, surface them to the user:
```
⚠ Unresolved placeholders: {{TIMELINE_CLOSEOUT_DATE}}
```

### Step 3: Offer to Open (macOS)

After all PDFs are generated, offer:

```
Generated <N> PDF(s). Open now?
  - Yes, open all
  - Open <specific file>
  - No
```

If the user says yes (or any variant), run:

```bash
open <pdf_path>
```

## Error Handling

| Error | Action |
|-------|--------|
| Recipe file not found | Report path, verify the file exists, stop |
| No `*.yaml` files in directory | Report "No recipe files found in <directory>", stop |
| `pandoc` not found | Report: `brew install pandoc`, stop |
| Assembler exits non-zero | Show assembler output verbatim, stop |

## What This Skill Does NOT Do

- Does not create or edit YAML recipe files — use the `poc-scoper` skill for that
- Does not install system dependencies (pandoc) — reports what is needed and stops
- Does not modify assembler.py or content modules
