# Hybrid Hook Distribution

**Bead:** syllago-3wza
**Status:** Plan
**Depends on:** Hook canonical spec (hooks-v1), adapter refactor (Phase 3)
**Addresses:** Hook author review section 4 (distribution), section 6 items 2+7

---

## Problem

Hook authors like Sam distribute via GitHub today. Their users clone the repo and manually copy hooks into provider settings files. Adopting syllago's canonical format means their users now need syllago installed -- an extra dependency that gates adoption.

The core tension: canonical format is better for portability and install UX, but provider-native format is better for zero-dependency manual consumption. Authors need to serve both audiences without maintaining two separate codebases.

## Solution: Dual-Format Distribution

A hook directory ships **both** the canonical `hook.json` (source of truth) and pre-generated provider-native configs (for manual users). syllago generates the native configs from canonical. Authors edit canonical only; native files are derived artifacts.

---

## 1. Directory Layout

```
hooks/safety-check/
  hook.json                    # canonical manifest (source of truth)
  hook.yaml                    # optional YAML authoring source (compiles to hook.json)
  check.sh                     # handler script (Unix)
  check.ps1                    # handler script (Windows)
  lib/
    helpers.sh                 # helper scripts
  .syllago.yaml                # syllago metadata (version, tags, tested_on)
  native/                      # generated provider-native configs
    claude-code/
      README.md                # manual install instructions for Claude Code
      hooks.json               # ready-to-merge JSON for .claude/settings.json
    gemini-cli/
      README.md                # manual install instructions for Gemini CLI
      hooks.json               # ready-to-merge JSON for .gemini/settings.json
    cursor/
      README.md
      hooks.json
    windsurf/
      README.md
      hooks.json
  README.md                    # top-level: what this hook does, provider support table
```

### Key decisions

- **`native/` directory, not top-level.** Native configs are derived artifacts. Keeping them in a subdirectory makes the hierarchy clear: `hook.json` is the source, `native/*` is output. Also avoids filename collisions (every provider's config is called something like `hooks.json`).

- **One subdirectory per provider.** Each contains the provider-native config plus a README with copy-paste install instructions. A user who only cares about Cursor navigates to `native/cursor/` and follows the README -- no syllago needed.

- **Scripts stay at hook root, not duplicated per provider.** Native configs reference scripts via relative paths (`../../check.sh`). This avoids script duplication and keeps the single-edit property: change `check.sh` once, all providers get it.

- **`native/` is gitignored by default in registries.** Registry hooks ship canonical only; syllago generates native on install. For GitHub-distributed hooks (Sam's use case), `native/` IS committed so manual users can consume it.

---

## 2. `syllago export --dual` Command

### Interface

```
syllago export --dual [--providers claude-code,cursor,gemini-cli] [--out-dir ./hooks/safety-check]
```

Flags:
- `--dual` -- generate native configs alongside canonical (without this flag, export behaves as today)
- `--providers` -- comma-separated list of target providers (default: all providers the hook is portable to, based on capability analysis)
- `--out-dir` -- output directory (default: current hook directory)

### Behavior

1. Read `hook.json` (or `hook.yaml`, compiling to canonical first)
2. Validate canonical format against spec
3. For each target provider:
   a. Run the conversion pipeline: validate capabilities, apply degradation, encode to native format
   b. Write native config to `native/<provider>/hooks.json`
   c. Generate `native/<provider>/README.md` with provider-specific install instructions
4. Generate top-level `README.md` with provider support table (if not already present or if `--update-readme` is set)
5. Print summary: which providers succeeded, which had warnings, which were skipped (non-portable)

### README Generation

Each `native/<provider>/README.md` is templated:

```markdown
# safety-check -- Claude Code

## Install with syllago (recommended)

    syllago install safety-check

## Manual install

1. Copy `check.sh` to your project (or add this repo as a submodule)
2. Add the following to `.claude/settings.json` under the `hooks` key:

    ```json
    { ... provider-native hook config ... }
    ```

3. Adjust the script path in `command` to point to where you placed `check.sh`

## Limitations

- This hook uses `structured_output` which is fully supported on Claude Code
```

The limitations section is generated from the capability validation step -- same warnings the conversion pipeline already produces. If the hook is non-portable to a provider, no `native/<provider>/` directory is created (instead of generating a broken config with a disclaimer).

### Top-level README

Generated from `.syllago.yaml` metadata + capability analysis:

```markdown
# safety-check

Block dangerous shell commands across all AI coding tools.

## Provider Support

| Provider | Status | Install |
|----------|--------|---------|
| Claude Code | Full support | `native/claude-code/README.md` |
| Gemini CLI | Full support | `native/gemini-cli/README.md` |
| Cursor | Limited (no structured output) | `native/cursor/README.md` |
| Windsurf | Limited (no structured output) | `native/windsurf/README.md` |
| OpenCode | Not supported | -- |

## Install with syllago

    syllago install safety-check

## Manual install

See the `native/<provider>/` directory for your provider.
```

---

## 3. Install Flow with Auto-Detection

When `syllago install` encounters a hook directory (local path or registry), it needs to decide which format to consume. The logic:

```
1. Does `hook.json` (or `hook.yaml`) exist?
   YES -> canonical install path (current flow, preferred)
   NO  -> check for native configs

2. Does `native/<target-provider>/hooks.json` exist?
   YES -> native install path (direct merge, skip conversion)
   NO  -> error: no compatible format found

3. Canonical install path:
   a. Parse hook.json
   b. Validate against spec
   c. Run conversion pipeline for target provider
   d. Merge into provider settings
   e. Copy scripts, set permissions

4. Native install path:
   a. Parse native config directly
   b. Merge into provider settings (no conversion needed)
   c. Copy scripts, set permissions
   d. Print info: "Installed from pre-generated native config.
      For cross-provider portability, the canonical hook.json is recommended."
```

### Why auto-detect matters

- **Forward compatibility.** Future hook directories might ship native-only (authored directly for one provider, not yet canonicalized). syllago should still install them.
- **Registry flexibility.** Registries can ship canonical-only (syllago generates native on the fly) or dual-format (pre-generated for faster install). The install flow handles both.
- **Gradual adoption.** Sam can convert 5 hooks to canonical, keep 25 as native Claude Code configs, and distribute all 30 through the same mechanism.

### Priority order

Canonical is always preferred over native when both exist. This ensures the conversion pipeline runs (with its capability validation, degradation strategies, and warnings). The native path is a fallback for directories that only have native configs.

---

## 4. Provider-Native README Instructions

Each generated README follows a consistent template but with provider-specific details:

### Template variables

| Variable | Source |
|----------|--------|
| Hook name, description | `.syllago.yaml` |
| Provider name | Provider registry |
| Settings file path | Provider adapter (e.g., `.claude/settings.json`, `.gemini/settings.json`) |
| Hook config JSON | Encoded output from adapter |
| Script files | File listing from hook directory |
| Limitations | Conversion warnings from capability validation |
| Install command | `syllago install <name>` |

### Provider-specific sections

**Claude Code / Gemini CLI** (JSON merge into settings):
- Show the JSON blob to merge under the `hooks` array
- Note: "Add to the `hooks` array in `<settings-file>`"

**Cursor / Windsurf** (separate rules file or settings):
- Show the config to add to the appropriate file
- Provider-specific file paths and structure

**Copilot CLI** (hooks.json):
- Show the hooks.json format
- Note working directory behavior

### Keeping READMEs current

READMEs are regenerated every time `syllago export --dual` runs. They are derived artifacts, not hand-maintained. The README includes a header comment:

```markdown
<!-- Generated by syllago export --dual. Do not edit manually. -->
<!-- Re-generate: syllago export --dual --providers claude-code -->
```

---

## 5. Registry Support for Dual-Format Hooks

### Registry storage

Registries store **canonical only**. Native configs are generated at install time by the user's local syllago. This avoids:
- Registry bloat (N providers x M hooks = lots of duplicate data)
- Stale native configs when syllago's adapters improve
- Version skew between canonical and native

```
registry/
  hooks/
    safety-check/
      hook.json          # canonical (stored)
      check.sh           # scripts (stored)
      .syllago.yaml      # metadata (stored)
      # NO native/ directory -- generated on install
```

### Registry metadata

`.syllago.yaml` gains a `distribution` field for registry entries:

```yaml
name: safety-check
version: 1.0.0
distribution:
  format: canonical           # "canonical", "native", or "dual"
  native_providers: []        # only populated if format is "dual" or "native"
  source_provider: ""         # only populated if format is "native" (original provider)
```

For GitHub-distributed hooks (outside registries), authors run `syllago export --dual` and commit the `native/` directory. The registry never stores native configs; GitHub repos optionally do.

### Registry install flow

```
syllago install safety-check
  1. Fetch hook directory from registry
  2. hook.json exists -> canonical install path
  3. Convert to target provider on the fly
  4. Merge into settings, copy scripts
```

### Native-only registry entries

For the gradual adoption story (Sam converts 5 hooks, keeps 25 native), registries accept native-only entries:

```yaml
distribution:
  format: native
  source_provider: claude-code
```

syllago can install these on the source provider directly. For other providers, syllago attempts to canonicalize first (import from source provider, then export to target). This is the existing import/export pipeline -- no new code needed, just wiring.

---

## 6. Update Workflow

### Author workflow: edit canonical, regenerate native

```
1. Author edits hook.json (or hook.yaml)
2. Author runs: syllago export --dual
3. native/ directory is regenerated
4. Author commits both hook.json and native/ changes
5. Version bump in .syllago.yaml
```

This is the "single source of truth" model. The canonical manifest is the only thing the author edits. Native configs are always derived.

### Consumer workflow: check for updates

For registry-installed hooks:

```
syllago update safety-check
  1. Check registry for newer version (compare .syllago.yaml version)
  2. Fetch updated hook directory
  3. Re-run install flow (canonical -> convert -> merge)
  4. Report: "Updated safety-check from 1.0.0 to 1.1.0"
```

For GitHub-distributed hooks (manual install):
- No automatic update mechanism. Users pull the repo and re-copy.
- The generated README can include a "check for updates" note linking to the GitHub repo.

### Staleness detection

When a hook directory has both `hook.json` and `native/` configs, syllago can detect staleness:

```
syllago validate hooks/safety-check/
  1. Parse hook.json
  2. For each native/<provider>/hooks.json:
     a. Generate what the native config SHOULD be (from canonical)
     b. Compare with what's on disk
     c. If different: "native/claude-code/hooks.json is stale. Run: syllago export --dual"
```

This runs as part of `syllago validate` (existing validation command) and could also be a pre-commit hook or CI check for hook authors.

### CI integration for hook authors

Hook authors can add a CI step:

```yaml
# .github/workflows/hooks.yml
- name: Check native configs are current
  run: |
    syllago export --dual
    git diff --exit-code native/
```

This fails if native configs are out of date, enforcing the "edit canonical, regenerate native" workflow.

---

## 7. Test Cases

### Unit tests

**Dual export generation:**
- Export a core-only hook (no capabilities) -> native configs generated for all providers
- Export a hook with `input_rewrite` capability -> non-supporting providers excluded from native output
- Export a hook with `llm_evaluated` capability -> default degradation `exclude` means most providers skipped
- Export with `--providers` flag -> only specified providers generated
- Export YAML source -> compiles to canonical JSON, then generates native
- Export with `degradation` overrides -> respected in native output

**Directory layout:**
- `native/` directory created with correct provider subdirectories
- Scripts referenced via correct relative paths from native configs
- README files generated with correct provider-specific content
- Top-level README includes accurate provider support table

**Auto-detection install:**
- Directory with `hook.json` only -> canonical path
- Directory with `hook.json` + `native/` -> canonical path (preferred)
- Directory with `native/claude-code/` only -> native path for claude-code target
- Directory with `native/cursor/` only -> error for claude-code target
- Empty directory -> error

**Staleness detection:**
- Fresh export -> validate reports no staleness
- Edit `hook.json`, don't re-export -> validate reports stale native configs
- Edit script file only (no hook.json change) -> native configs still valid (they reference scripts by path)

### Integration tests

**Round-trip fidelity:**
- Create canonical hook -> export dual -> install from native -> compare with direct canonical install
- The installed result should be identical regardless of which path was taken

**Multi-provider export:**
- Hook with 4 portable providers -> 4 native directories, each with valid provider config
- Verify each native config parses correctly with its provider's adapter (read-back verification)

**Registry vs GitHub flow:**
- Registry install (canonical only, no native/) -> converts on the fly, installs correctly
- GitHub install (dual format, native/ present) -> uses canonical path, same result

**Gradual adoption:**
- Directory with 3 canonical hooks and 2 native-only hooks -> all 5 installable
- Native-only hook installed on source provider -> works
- Native-only hook installed on different provider -> attempts canonicalize-then-convert

### CLI output tests

- `syllago export --dual` prints provider support summary
- `syllago export --dual` warns when providers are skipped (non-portable)
- `syllago validate` on stale native configs prints actionable message
- `syllago install` from native path prints info about preferring canonical

---

## Sequencing

This feature depends on:
1. **Hooks-v1 spec** -- canonical format must be defined
2. **Adapter refactor (Phase 3)** -- per-provider `Encode()` needed to generate native configs
3. **Capability validation** -- degradation strategies needed to decide which providers to generate for

Implementation order:
1. Native config generation (`export --dual` core logic) -- uses existing adapter `Encode()`
2. README templating (provider-specific install instructions)
3. Auto-detect install flow (canonical vs native path selection)
4. Staleness detection (`validate` integration)
5. Registry metadata (`distribution` field in `.syllago.yaml`)

Each step is independently testable and shippable.
