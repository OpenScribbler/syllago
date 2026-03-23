---
name: syllago-quickstart
description: Getting started with syllago in your first 5 minutes
---

# Your First 5 Minutes with Syllago

Syllago is a package manager for AI coding tool content. This guide walks you through the basics — from discovering what you already have to sharing content with your team.

## Step 1: Discover What You Have

If you already use an AI coding tool (Claude Code, Cursor, Gemini CLI, etc.), you probably have rules, skills, or configs worth managing. Import them:

```bash
syllago add --from claude-code     # Import from Claude Code
syllago add --from cursor          # Import from Cursor
syllago add --from gemini-cli      # Import from Gemini CLI
```

This scans the provider's default location, finds content, and copies it into your syllago library. Nothing is modified in the original location.

Run `syllago list` to see what was imported.

## Step 2: Browse Your Library

Launch the interactive TUI:

```bash
syllago
```

Navigate with:
- **Tab** to move between the sidebar and content area
- **Arrow keys** to browse items
- **Enter** to inspect an item's details
- **?** to see all keyboard shortcuts

The sidebar shows content types (Skills, Rules, Agents, etc.) and collections (Library, Loadouts, Registries). Click or arrow to any section to explore.

## Step 3: Install to Another Provider

Content in your library can be installed to any supported provider. Syllago handles format conversion automatically.

```bash
syllago install skills/my-skill --to cursor       # Install a skill to Cursor
syllago install rules/my-rule --to windsurf        # Install a rule to Windsurf
syllago install --all --to gemini-cli              # Install everything to Gemini CLI
```

A Claude Code skill becomes a Kiro steering file, a Cursor rule becomes a Windsurf rule — syllago translates formats, tool names, and metadata.

## Step 4: Add a Registry

Registries are git repos containing shared content. Add one to browse community or team content:

```bash
syllago registry add https://github.com/your-team/ai-configs.git
syllago registry sync
```

Registry content appears in the TUI with a badge showing its source. Browse with `syllago registry items` or in the TUI's Registries section.

## Step 5: Create a Loadout

Loadouts are curated bundles of content for a specific provider. Create one for your team:

```bash
syllago loadout create
```

The interactive wizard walks you through selecting a provider, naming the loadout, and choosing which items to include. Share the loadout via a registry so teammates can apply it in one step:

```bash
syllago loadout apply my-team-loadout
```

## Next Steps

- Run `syllago info` to see all supported providers and content types
- Run `syllago --help` for the full command reference
- Use `syllago-guide` for the complete reference (all commands, flags, and features)
- Try `syllago sandbox run <provider>` to test provider configs in isolation (Linux)
