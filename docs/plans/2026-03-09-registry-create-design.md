# Design: Improve `syllago registry create`

## Goal

Make `registry create` feel like `cargo init` — you run it and you're ready to push. Reduce friction for first-time registry authors by providing structure, examples, and automation.

## Decisions

### 1. Auto `git init` + initial commit

- Run `git init` automatically after scaffolding
- Stage all files and create initial commit: `"Initial registry scaffold"`
- Add `--no-git` flag to opt out
- If already inside a git repo, skip `git init` with a message (avoid nested repos)
- Justification: A registry IS a git repo — unlike most project scaffolders, the output is inherently a git artifact. `cargo init` and `dotnet new` follow this pattern.

### 2. `.gitignore`

Add a standard `.gitignore` covering:
- OS files: `.DS_Store`, `Thumbs.db`
- Editor files: `*.swp`, `*~`, `.vscode/`, `.idea/`
- Other: `*.bak`, `*.tmp`

### 3. Example content

Include two examples showing the two layout patterns:

**Universal content (skill):** `skills/hello-world/SKILL.md` — shows frontmatter with name/description, a simple instruction body.

**Provider-specific content (rule):** `rules/claude-code/example-rule/rule.md` + `rules/claude-code/example-rule/README.md` — shows the provider subdirectory nesting pattern.

Both examples should be clearly marked as examples (comments in the files explaining "replace this with your content"). Tag with `.syllago.yaml` metadata `tags: [example]` so they're identifiable.

### 4. CONTRIBUTING.md

Template covering:
- How to add each content type (directory structure for universal vs provider-specific)
- Naming conventions (letters, numbers, hyphens, underscores)
- Required files per content type (SKILL.md, rule.md, hook.json, etc.)
- Brief explanation of registry.yaml manifest

### 5. Better "next steps" output

After auto `git init` + commit:
- Show "push to a remote" commands with the registry name substituted
- Show how to test locally with `syllago registry add`

When `--no-git` is used:
- Show the existing manual git commands

## Scope

### In scope
- `.gitignore` file generation
- Auto `git init` + initial commit with `--no-git` flag
- Example skill + example rule
- `CONTRIBUTING.md` template
- Updated "next steps" CLI output

### Explicitly deferred
- CI/CD template (no `syllago lint` command exists yet)
- LICENSE file (flag like `--license MIT` for later)
- Interactive wizard mode
- Presets (`--preset team`, `--preset full`, etc.)

## Files to modify

- `cli/internal/registry/scaffold.go` — Main changes: add gitignore, examples, CONTRIBUTING, git init logic
- `cli/cmd/syllago/registry_cmd.go` — Add `--no-git` flag, update "next steps" output
- `cli/internal/registry/scaffold_test.go` — New tests for all additions

## Content type reference (from scanner)

| Content Type | Layout | Key Files |
|---|---|---|
| Skills (universal) | `skills/<name>/` | `SKILL.md` (frontmatter) |
| Agents (universal) | `agents/<name>/` | `AGENT.md` (frontmatter) |
| MCP (universal) | `mcp/<name>/` | `README.md` |
| Rules (provider-specific) | `rules/<provider>/<name>/` | `rule.md`, `README.md` |
| Hooks (provider-specific) | `hooks/<provider>/<name>/` | `hook.json`, `README.md` |
| Commands (provider-specific) | `commands/<provider>/<name>/` | `command.md`, `README.md` |
| Loadouts | `loadouts/<name>/` | `loadout.yaml` |
