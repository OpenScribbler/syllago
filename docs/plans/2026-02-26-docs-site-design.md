# Syllago Documentation Site - Design Document

**Goal:** Build a public documentation site for syllago using Astro Starlight, hosted in a separate repo with release-gated sync to the CLI repo.

**Decision Date:** 2026-02-26

---

## Problem Statement

Syllago needs user-facing documentation beyond a GitHub README. The tool manages content across 11 AI coding tool providers, has a TUI, CLI, registry system, content authoring workflow, and a sandbox feature. A README can't serve the getting started, reference, and content authoring stories at the quality level syllago needs to make a good first impression.

## Proposed Solution

A standalone Astro Starlight documentation site in a separate `syllago-docs` repository, deployed to GitHub Pages. Release-gated sync ensures docs stay current with CLI releases without coupling the repos.

### Repo Landscape

| Repo | Purpose |
|---|---|
| `syllago` | CLI source, meta-content, design docs |
| `syllago-docs` | Public documentation site (Astro Starlight) |
| `syllago-tools` | Reference registry content |

## Architecture

### Framework: Astro Starlight

- Purpose-built for documentation sites
- Sidebar navigation, search, dark/light mode, responsive out of the box
- Hero landing page support
- Tabs, callouts, code blocks, and other doc components built in

### Visual Theme: Flexoki

- Community theme plugin: `starlight-theme-flexoki`
- Warm, friendly color palette based on the [Flexoki](https://stephango.com/flexoki) system
- Distinctive without being flashy — readable and professional
- Install via plugin, then customize accent colors if needed to match syllago branding

### Hosting: GitHub Pages

- Deployed via GitHub Actions on push to `main`
- Default GitHub Pages URL for now (`openscribbler.github.io/syllago-docs`)
- Custom domain can be added later with a single config change (no URL breakage with proper redirects)

### Separate Repo (Not Monorepo)

**Why separate:**
- Clean toolchain separation (Go CLI vs Node.js/Astro docs)
- Independent deploy cadence — content fixes and improvements ship without touching the CLI repo
- No CI overhead — docs changes don't trigger Go builds, CLI changes don't trigger Astro builds
- Docs drift prevented by active release gating, not passive proximity

**Why not monorepo:**
- Monorepo gives passive sync (things are nearby, might stay in sync). Release gates give active sync (the process requires them to be in sync). Active sync is strictly better.
- Mixed toolchains in one repo add CI complexity for no real benefit
- AI partners can update docs in minutes — the "solo maintainer drift" argument doesn't apply

## Information Architecture

User-journey based — organized by what users need to DO, mirroring the onboarding path.

```
Getting Started
  ├─ Installation
  ├─ Quick Start
  └─ Core Concepts

Using Syllago
  ├─ The TUI
  ├─ CLI Reference
  ├─ Supported Providers (index + 11 detail pages)
  └─ Content Types

Creating Content
  ├─ Authoring Guide
  ├─ .syllago.yaml Format
  ├─ Registries
  └─ Format Conversion

Advanced
  ├─ Sandbox
  ├─ Team Setup
  └─ Troubleshooting
```

### Page Details

**Landing Page (Hero)**
- Tagline: "The package manager for AI coding tool content"
- CTA buttons: Get Started, GitHub
- Feature highlights: 11 providers, automatic format conversion, team registries, no lock-in

**Getting Started**
- **Installation:** curl one-liner, Homebrew, manual binary download
- **Quick Start:** Provider-agnostic walkthrough: install → `syllago init` → browse in TUI → install an item → verify in your AI tool. Syllago abstracts provider differences, so the quick start should too.
- **Core Concepts:** What content types are, how providers work, what registries do. Mental model, not reference.

**Using Syllago**
- **The TUI:** How to navigate, keyboard shortcuts, what each panel does
- **CLI Reference:** All commands and flags
- **Supported Providers:** Index page with summary comparison table (all 11 at a glance) + one detail page per provider. Each detail page covers: config locations, supported content types, provider-specific notes.
- **Content Types:** Rules, skills, agents, commands, hooks, MCP, prompts, apps. What each is, when to use it.

**Creating Content**
- **Authoring Guide:** Walk through creating a content item from scratch. Shows `.syllago.yaml` frontmatter, directory structure, how syllago discovers items, how to test locally with `syllago add`, and export to a provider. Mentions the built-in `syllago-author` skill as a convenience alternative for those who prefer guided creation.
- **.syllago.yaml Format:** Reference page for the frontmatter schema. Fields, required vs optional, valid values, examples per content type. (Detailed scope TBD — depends on format finalization in the tool-coverage implementation work.)
- **Registries:** How to create a registry repo, directory structure, how items are discovered, how to publish, how others consume it.
- **Format Conversion:** How syllago embeds intent across provider formats. Not "what gets lost" — syllago doesn't lose things. This page explains how intent carries forward across different provider representations.

**Advanced**
- **Sandbox:** Setup (bubblewrap + socat), usage, security model, domain allowlisting
- **Team Setup:** How to use registries for team standardization, per-project config
- **Troubleshooting:** Common issues and solutions

## Release-Gated Sync

The `/release` skill in the syllago CLI repo enforces that docs are current before a CLI release can proceed.

### Mechanism: Hard Version Tag Check

```
/release 1.0.0

1. Bump VERSION in syllago
2. Check: does syllago-docs repo have git tag v1.0.0?
3. If NO → hard stop
   "Tag v1.0.0 not found in syllago-docs. Create the docs release first."
4. If YES → proceed with CLI release
```

- No escape hatch. Docs are a release requirement, not a suggestion.
- Both repos end up with matching version tags for every feature release.
- Content-only fixes (typos, clarifications) deploy independently to syllago-docs at any time — they don't require a CLI release.

### Workflow in Practice

1. Feature work lands in `syllago` CLI repo
2. Corresponding docs work lands in `syllago-docs`
3. When ready to release, tag `syllago-docs` with the version first
4. Run `/release <version>` in `syllago` — skill verifies the docs tag exists
5. Both releases proceed, both repos have matching version tags

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Repo structure | Separate `syllago-docs` repo | Independent deploys, clean toolchain separation, active sync via release gate |
| Framework | Astro Starlight | Holden's expertise, purpose-built for docs |
| Visual theme | Flexoki (starlight-theme-flexoki) | Warm, readable, distinctive. Customize accent colors for syllago brand later. |
| Hosting | GitHub Pages | Free, same ecosystem, no vendor |
| Domain | Default GitHub Pages URL | Custom domain later, no premature purchase |
| Information architecture | User-journey based | Mirrors onboarding path, tells a story |
| Provider pages | Index + one per provider | Quick-scan overview + detailed reference |
| Quick start | Provider-agnostic | Syllago abstracts providers — docs should too |
| Landing page | Hero layout | First impression with tagline, CTA, highlights |
| Content authoring docs | Manual guide + skill reference | Docs are complete reference, skill is convenience |
| Format conversion framing | "Embeds intent" | Core philosophy — nothing is lost in conversion |
| Release gate | Hard version tag check | No skip option. Docs are a release requirement. |

## Open Questions

- **Exact `.syllago.yaml` schema:** Depends on format finalization in tool-coverage implementation. Docs page will be written once the schema is stable.
- **Content authoring wizard skill:** Should this be part of `syllago-author` or a separate built-in skill? Design TBD.
- **TUI screenshots/recordings:** Should the docs include static screenshots, embedded asciinema recordings, or both? Decision deferred to implementation.
- **Search:** Starlight includes Pagefind by default. Is that sufficient, or do we need Algolia DocSearch? Likely Pagefind is fine for v1.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
