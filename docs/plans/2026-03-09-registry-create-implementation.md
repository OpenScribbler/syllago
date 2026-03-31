# Implementation Plan: `syllago registry create` Improvements

## Phase 1: `.gitignore` generation

### Task 1.1: Add gitignore content to `Scaffold()`

**File:** `cli/internal/registry/scaffold.go`

Add a package-level string constant:

```go
const gitignoreContent = `# OS files
.DS_Store
Thumbs.db

# Editor files
*.swp
*~
.vscode/
.idea/

# Misc
*.bak
*.tmp
`
```

Write it in `Scaffold()` after the README:

```go
if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
    return fmt.Errorf("writing .gitignore: %w", err)
}
```

---

## Phase 2: Example content generation

### Task 2.1: Add example content constants

**File:** `cli/internal/registry/scaffold.go`

Add string constants for:
- `exampleSkillContent` — SKILL.md with frontmatter (name, description) and placeholder instructions
- `exampleSkillMetaContent` — `.syllago.yaml` with `tags: [example]`
- `exampleRuleContent` — `rule.md` with placeholder rule
- `exampleRuleReadmeContent` — `README.md` for the rule
- `exampleRuleMetaContent` — `.syllago.yaml` with `tags: [example]`

### Task 2.2: Write example files in `Scaffold()`

**File:** `cli/internal/registry/scaffold.go`

After gitignore, create:
- `skills/hello-world/SKILL.md` + `skills/hello-world/.syllago.yaml`
- `rules/claude-code/example-rule/rule.md` + `rules/claude-code/example-rule/README.md` + `rules/claude-code/example-rule/.syllago.yaml`

These directories already have parent dirs created by the content-type loop. Use `os.MkdirAll` for the nested paths.

---

## Phase 3: CONTRIBUTING.md generation

### Task 3.1: Add `contributingContent()` function

**File:** `cli/internal/registry/scaffold.go`

Function (not constant) because it includes the registry name. Covers:
- Universal content layout (skills, agents, MCP)
- Provider-specific layout (rules, hooks, commands)
- Naming conventions
- registry.yaml explanation
- Note about example content

### Task 3.2: Write CONTRIBUTING.md in `Scaffold()`

After example content:

```go
if err := os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte(contributingContent(name)), 0644); err != nil {
    return fmt.Errorf("writing CONTRIBUTING.md: %w", err)
}
```

---

## Phase 4: Auto git init + initial commit

### Task 4.1: Add git functions to gitutil package

**File:** `cli/internal/gitutil/gitutil.go`

Check what already exists in this file. Add two functions:

**`IsInsideGitRepo(dir string) bool`** — runs `git -C dir rev-parse --is-inside-work-tree`. Returns true if exit code 0.

**`InitAndCommit(dir, message string) error`** — runs `git init`, `git add .`, `git commit -m message` sequentially. Checks `exec.LookPath("git")` first. Returns error on any failure.

Key design decisions:
- These are separate from `Scaffold()` — Scaffold is pure filesystem, git is orchestration
- `IsInsideGitRepo` checks the parent `cwd`, not the new registry dir (which doesn't exist yet when we check)
- `InitAndCommit` failure is non-fatal in the cobra command (warning, not error) — git may not be configured

---

## Phase 5: Updated CLI output and `--no-git` flag

### Task 5.1: Add `--no-git` flag

**File:** `cli/cmd/syllago/registry_cmd.go`

In `init()`:
```go
registryCreateCmd.Flags().Bool("no-git", false, "Skip git init and initial commit")
```

### Task 5.2: Update `registryCreateCmd.RunE`

**File:** `cli/cmd/syllago/registry_cmd.go`

Updated flow:
1. Read `--no-git` flag
2. Check `gitutil.IsInsideGitRepo(cwd)` before calling Scaffold
3. Call `registry.Scaffold(cwd, name, desc)` (unchanged)
4. Print structure (unchanged)
5. If `!noGit && !alreadyInGit`: call `gitutil.InitAndCommit(dir, "Initial registry scaffold")`
   - On success: print "Initialized git repository and created initial commit."
   - On failure: print warning to stderr, continue (non-fatal)
6. If `alreadyInGit && !noGit`: print "Note: already inside a git repo — skipping git init."
7. Print updated "next steps":
   - If git was initialized: `git remote add origin <url>`, `git push -u origin main`
   - If not: show manual `git init && git add . && git commit` commands
   - Always show: `syllago registry add <your-git-url>`

Add import for `gitutil` package.

---

## Phase 6: Tests

### Task 6.1: Test `.gitignore` creation

**File:** `cli/internal/registry/scaffold_test.go`

Verify `.gitignore` exists and contains `.DS_Store`, `Thumbs.db`, `*.swp`, `.vscode/`, `.idea/`.

### Task 6.2: Test example skill creation

Verify `skills/hello-world/SKILL.md` exists with frontmatter containing `name:`.
Verify `skills/hello-world/.syllago.yaml` exists with `example` tag.

### Task 6.3: Test example rule creation

Verify `rules/claude-code/example-rule/rule.md`, `README.md`, `.syllago.yaml` all exist.

### Task 6.4: Test CONTRIBUTING.md creation

Verify `CONTRIBUTING.md` exists and contains key strings: registry name, `SKILL.md`, `rule.md`, `hook.json`, `claude-code`.

### Task 6.5: Test gitutil functions

**File:** `cli/internal/gitutil/gitutil_test.go` (new or extend existing)

- `TestIsInsideGitRepo_False` — fresh temp dir returns false
- `TestInitAndCommit` — creates file, runs InitAndCommit, verifies IsInsideGitRepo returns true. Skip if git not on PATH.

---

## Sequencing

Phases 1-3 are independent changes to `Scaffold()` in scaffold.go.
Phase 4 is independent new code in gitutil.
Phase 5 depends on Phase 4 (imports gitutil).
Phase 6 tests should be written alongside each phase.

Recommended order: 1 → 2 → 3 → 4 → 5 → 6

## Key Design Decisions

| Decision | Rationale |
|---|---|
| `InitAndCommit` in gitutil, not Scaffold | Scaffold is pure filesystem; mixing git breaks testability |
| Check `cwd` not `dir` for existing repo | Registry dir doesn't exist yet when we check |
| Git failure is non-fatal (warning) | Git may lack user config; scaffold still succeeded |
| String constants, not embed.FS | Simpler build, content changes rarely |
| Example content tagged `example` | Scanner already supports this tag via `.syllago.yaml` |
