# Team Setup Guide

Set up syllago for a team: create a shared registry, populate it with content, and onboard engineers with loadouts.

## Overview

The team workflow has four stages:

1. **Create** a registry (git repo) for shared content
2. **Populate** it with rules, skills, and other content
3. **Configure** engineer machines to use the registry
4. **Distribute** content via loadouts or individual installs

## 1. Create a Registry

A registry is a git repo with syllago content. Any git host works (GitHub, GitLab, Bitbucket, self-hosted).

```bash
# Create a new repo on GitHub (or your git host)
gh repo create my-org/ai-coding-standards --private

# Clone it locally
git clone git@github.com:my-org/ai-coding-standards.git
cd ai-coding-standards

# Initialize as a syllago repo
syllago init
```

After init, the repo has the standard directory structure:

```
ai-coding-standards/
  skills/
  agents/
  rules/
  hooks/
  commands/
  mcp/
  loadouts/
```

## 2. Populate with Content

### Add content from your own setup

If you already have rules and skills configured in Claude Code, Cursor, or other providers:

```bash
# See what content you have
syllago add --from claude-code

# Add everything to your library
syllago add --all --from claude-code

# Share specific items to the team registry
syllago share my-coding-rules --to ai-coding-standards
syllago share security-policy --to ai-coding-standards
```

### Create content directly

```bash
# Create a new rule in your library
syllago create rule team-conventions

# Edit it, then share
syllago share team-conventions --to ai-coding-standards
```

### Organize with loadouts

Loadouts are curated bundles. Create one for your team's standard setup:

```bash
syllago create loadout onboarding --to claude-code
# The wizard walks you through selecting which items to include
```

## 3. Configure Engineer Machines

Each engineer adds the registry to their syllago config:

```bash
# Add the team registry
syllago registry add ai-coding-standards git@github.com:my-org/ai-coding-standards.git

# Verify it's configured
syllago doctor
```

The `syllago doctor` command confirms the registry is reachable and shows its visibility (private/public).

### Private registry access

For private registries, engineers need git credentials (SSH key or personal access token) for the host. syllago uses standard git authentication -- no separate credential system.

## 4. Distribute Content

### Option A: Loadouts (recommended for onboarding)

If you created a loadout, engineers apply it in one command:

```bash
# Browse available loadouts from the registry
syllago list --type loadouts

# Apply the onboarding loadout
syllago loadout apply onboarding --to claude-code
```

This installs all bundled content (rules, skills, hooks, MCP configs) in one step.

### Option B: Individual installs

Engineers can browse and install individual items:

```bash
# See what's available
syllago list

# Install specific items
syllago install team-conventions --to claude-code
syllago install security-policy --to cursor
```

## Updating Content

When team standards change:

```bash
# Update content in your library
# (edit the files, or re-add from provider)

# Share the update to the registry
syllago share updated-rule --to ai-coding-standards

# Engineers pull updates
syllago registry sync ai-coding-standards
```

## Verifying Setup

Use `syllago doctor` to check that everything is working:

```bash
syllago doctor
```

This checks:
- Library exists and is accessible
- Config files are valid
- Providers are detected
- Installed content integrity (no drift)
- Registry connectivity and visibility

## Tips

- **Start small**: Share 2-3 high-value rules first, then expand. Adoption is easier when the initial set is curated.
- **Use loadouts for onboarding**: New engineers get a consistent setup without manual steps.
- **Private by default**: Use private registries for proprietary conventions. Public registries are for community sharing.
- **Version with git**: Registry content is versioned through git. Use branches for experimental content, tags for releases.
- **Cross-provider**: Content added from one provider can be installed to any other. syllago handles format conversion automatically.
