# Syllago Docs Site — Implementation Plan

**Goal:** Create the `syllago-docs` repository from scratch: scaffold Astro Starlight with the Flexoki theme, configure the full IA sidebar, create placeholder `.mdx` pages for every section, set up GitHub Pages deployment via GitHub Actions, and build the hero landing page. The site must build and deploy successfully to `openscribbler.github.io/syllago-docs`.

**Framework:** Astro Starlight with `starlight-theme-flexoki`
**Hosting:** GitHub Pages via `withastro/action`
**Repo:** `OpenScribbler/syllago-docs` (new, separate from CLI repo)

**Design doc:** `docs/plans/2026-02-26-docs-site-design.md`

---

## Phase 1 — Repo and Project Scaffold

---

### Task 1.1 — Create the GitHub repo

**Dependencies:** None

**What it does:** Creates the empty `syllago-docs` repo under the `OpenScribbler` org on GitHub. GitHub Pages will serve from this repo. The repo is public because the docs are public.

**Commands:**
```bash
gh repo create OpenScribbler/syllago-docs \
  --public \
  --description "Documentation site for syllago — the package manager for AI coding tool content"
```

Then clone it locally:
```bash
git clone git@github.com:OpenScribbler/syllago-docs.git ~/.local/src/syllago-docs
cd ~/.local/src/syllago-docs
```

**Verification:** `gh repo view OpenScribbler/syllago-docs` returns the repo info. The local clone directory exists and contains a `.git/` folder.

---

### Task 1.2 — Scaffold Astro Starlight project

**Dependencies:** Task 1.1

**What it does:** Uses the official Astro CLI to scaffold a new Starlight project in the cloned repo directory. This creates `astro.config.mjs`, `package.json`, `src/content/docs/`, `src/content/config.ts`, and `public/` with a working Starlight baseline.

**Why `create astro` with the Starlight template vs. manual setup:** The template wires up all the Starlight-specific Astro integrations, TypeScript config, and content collection schema that are easy to miss if done by hand.

**Commands (run from inside the `syllago-docs` clone directory):**
```bash
npm create astro@latest . -- --template starlight --no-install --no-git --yes --skip-houston
npm install
```

The `--no-git` flag is used because we already have a git repo from the clone. The `.` installs into the current directory.

**Files created by scaffold:**
```
astro.config.mjs
package.json
tsconfig.json
src/
  content/
    config.ts
    docs/
      index.mdx
      guides/
        example.md
  env.d.ts
public/
  favicon.svg
```

**Verification:** `npm run build` completes without errors. `dist/` directory is created.

---

### Task 1.3 — Install Flexoki theme and configure it

**Dependencies:** Task 1.2

**What it does:** Installs `starlight-theme-flexoki` as an npm package and registers it in the Starlight `plugins` array in `astro.config.mjs`. This replaces Starlight's default blue theme with the warm Flexoki palette.

**Install command:**
```bash
npm install starlight-theme-flexoki
```

**Replace** the entire contents of `astro.config.mjs` with:

```js
// astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightThemeFlexoki from 'starlight-theme-flexoki';

export default defineConfig({
  site: 'https://openscribbler.github.io',
  base: '/syllago-docs',
  integrations: [
    starlight({
      title: 'syllago',
      description: 'The package manager for AI coding tool content',
      plugins: [starlightThemeFlexoki()],
      social: {
        github: 'https://github.com/OpenScribbler/syllago',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Quick Start', slug: 'getting-started/quick-start' },
            { label: 'Core Concepts', slug: 'getting-started/core-concepts' },
          ],
        },
        {
          label: 'Using Syllago',
          items: [
            { label: 'The TUI', slug: 'using-syllago/tui' },
            { label: 'CLI Reference', slug: 'using-syllago/cli-reference' },
            {
              label: 'Supported Providers',
              items: [
                { label: 'Overview', slug: 'using-syllago/providers/index' },
                { label: 'Claude Code', slug: 'using-syllago/providers/claude-code' },
                { label: 'Cursor', slug: 'using-syllago/providers/cursor' },
                { label: 'Windsurf', slug: 'using-syllago/providers/windsurf' },
                { label: 'Copilot', slug: 'using-syllago/providers/copilot' },
                { label: 'Cline', slug: 'using-syllago/providers/cline' },
                { label: 'Roo Code', slug: 'using-syllago/providers/roo-code' },
                { label: 'Kiro', slug: 'using-syllago/providers/kiro' },
                { label: 'Zed', slug: 'using-syllago/providers/zed' },
                { label: 'Gemini CLI', slug: 'using-syllago/providers/gemini-cli' },
                { label: 'OpenCode', slug: 'using-syllago/providers/opencode' },
                { label: 'Codex', slug: 'using-syllago/providers/codex' },
              ],
            },
            { label: 'Content Types', slug: 'using-syllago/content-types' },
          ],
        },
        {
          label: 'Creating Content',
          items: [
            { label: 'Authoring Guide', slug: 'creating-content/authoring-guide' },
            { label: '.syllago.yaml Format', slug: 'creating-content/syllago-yaml' },
            { label: 'Registries', slug: 'creating-content/registries' },
            { label: 'Format Conversion', slug: 'creating-content/format-conversion' },
          ],
        },
        {
          label: 'Advanced',
          items: [
            { label: 'Sandbox', slug: 'advanced/sandbox' },
            { label: 'Team Setup', slug: 'advanced/team-setup' },
            { label: 'Troubleshooting', slug: 'advanced/troubleshooting' },
          ],
        },
      ],
    }),
  ],
});
```

**Why `site` + `base`:** GitHub Pages hosts at `openscribbler.github.io/syllago-docs` (not the root domain), so both `site` and `base` are required for internal links and assets to resolve correctly. This is the most common Astro GitHub Pages gotcha.

**Verification:** `npm run build` completes. `dist/syllago-docs/index.html` exists (the `base` prefix appears in the output path).

---

### Task 1.4 — Delete scaffold example content

**Dependencies:** Task 1.3

**What it does:** The Astro Starlight scaffold creates example pages (`guides/example.md`, etc.) that conflict with our IA. Delete them before creating our own pages. Also delete the scaffold's `index.mdx` since we will replace it completely in Phase 3.

**Commands:**
```bash
rm -rf src/content/docs/guides/
rm src/content/docs/index.mdx
```

**Verification:** `src/content/docs/` contains only `config.ts` (in `src/content/`) and is otherwise empty. `npm run build` will now fail with missing slug errors — that is expected and will be resolved by Phase 2.

---

## Phase 2 — Placeholder Pages for Full IA

---

### Task 2.1 — Getting Started pages (3 files)

**Dependencies:** Task 1.4

**What it does:** Creates the three placeholder pages for the Getting Started section. Each uses Starlight's `title` frontmatter and a one-sentence stub body. The slug in the frontmatter is implicit from the file path — no explicit `slug` field is needed.

**Create** `src/content/docs/getting-started/installation.mdx`:
```mdx
---
title: Installation
description: How to install syllago on your system.
---

Installation instructions coming soon.
```

**Create** `src/content/docs/getting-started/quick-start.mdx`:
```mdx
---
title: Quick Start
description: Get up and running with syllago in minutes.
---

Quick start guide coming soon.
```

**Create** `src/content/docs/getting-started/core-concepts.mdx`:
```mdx
---
title: Core Concepts
description: The mental model behind syllago — content types, providers, and registries.
---

Core concepts documentation coming soon.
```

**Verification:** `npm run build` no longer errors on these three slugs.

---

### Task 2.2 — Using Syllago pages (3 non-provider files)

**Dependencies:** Task 1.4

**What it does:** Creates placeholder pages for the TUI, CLI Reference, and Content Types pages.

**Create** `src/content/docs/using-syllago/tui.mdx`:
```mdx
---
title: The TUI
description: How to navigate the syllago terminal user interface.
---

TUI documentation coming soon.
```

**Create** `src/content/docs/using-syllago/cli-reference.mdx`:
```mdx
---
title: CLI Reference
description: Complete reference for all syllago commands and flags.
---

CLI reference documentation coming soon.
```

**Create** `src/content/docs/using-syllago/content-types.mdx`:
```mdx
---
title: Content Types
description: Rules, skills, agents, commands, hooks, MCP, prompts, and apps — what each is and when to use it.
---

Content types documentation coming soon.
```

**Verification:** `npm run build` no longer errors on these three slugs.

---

### Task 2.3 — Providers index page

**Dependencies:** Task 1.4

**What it does:** Creates the providers overview page. This page will eventually contain a comparison table across all 11 providers. The stub uses Starlight's `title` frontmatter.

Note on file path: The sidebar config uses slug `using-syllago/providers/index`. In Astro Starlight, a file at `src/content/docs/using-syllago/providers/index.mdx` maps to that slug.

**Create** `src/content/docs/using-syllago/providers/index.mdx`:
```mdx
---
title: Supported Providers
description: An overview of all 11 AI coding tool providers supported by syllago.
---

Provider comparison table and overview coming soon.
```

**Verification:** `npm run build` resolves the `using-syllago/providers/index` slug without error.

---

### Task 2.4 — Provider detail pages (11 files)

**Dependencies:** Task 2.3

**What it does:** Creates one placeholder `.mdx` file per provider. Each page will eventually document config locations, supported content types, and provider-specific notes. For now, each is a minimal stub.

**Create** `src/content/docs/using-syllago/providers/claude-code.mdx`:
```mdx
---
title: Claude Code
description: Using syllago with Claude Code.
---

Claude Code provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/cursor.mdx`:
```mdx
---
title: Cursor
description: Using syllago with Cursor.
---

Cursor provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/windsurf.mdx`:
```mdx
---
title: Windsurf
description: Using syllago with Windsurf.
---

Windsurf provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/copilot.mdx`:
```mdx
---
title: GitHub Copilot
description: Using syllago with GitHub Copilot.
---

GitHub Copilot provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/cline.mdx`:
```mdx
---
title: Cline
description: Using syllago with Cline.
---

Cline provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/roo-code.mdx`:
```mdx
---
title: Roo Code
description: Using syllago with Roo Code.
---

Roo Code provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/kiro.mdx`:
```mdx
---
title: Kiro
description: Using syllago with Kiro.
---

Kiro provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/zed.mdx`:
```mdx
---
title: Zed
description: Using syllago with Zed.
---

Zed provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/gemini-cli.mdx`:
```mdx
---
title: Gemini CLI
description: Using syllago with Gemini CLI.
---

Gemini CLI provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/opencode.mdx`:
```mdx
---
title: OpenCode
description: Using syllago with OpenCode.
---

OpenCode provider documentation coming soon.
```

**Create** `src/content/docs/using-syllago/providers/codex.mdx`:
```mdx
---
title: Codex
description: Using syllago with OpenAI Codex CLI.
---

Codex provider documentation coming soon.
```

**Verification:** All 11 provider slugs resolve. `npm run build` completes the `using-syllago/providers/` subtree without errors.

---

### Task 2.5 — Creating Content pages (4 files)

**Dependencies:** Task 1.4

**What it does:** Creates placeholder pages for the four Creating Content section pages.

**Create** `src/content/docs/creating-content/authoring-guide.mdx`:
```mdx
---
title: Authoring Guide
description: How to create syllago content items from scratch.
---

Authoring guide coming soon.
```

**Create** `src/content/docs/creating-content/syllago-yaml.mdx`:
```mdx
---
title: .syllago.yaml Format
description: Complete reference for the .syllago.yaml frontmatter schema.
---

.syllago.yaml format reference coming soon.
```

**Create** `src/content/docs/creating-content/registries.mdx`:
```mdx
---
title: Registries
description: How to create, publish, and consume syllago registries.
---

Registries documentation coming soon.
```

**Create** `src/content/docs/creating-content/format-conversion.mdx`:
```mdx
---
title: Format Conversion
description: How syllago embeds intent across provider formats.
---

Format conversion documentation coming soon.
```

**Verification:** All four slugs resolve. `npm run build` completes the `creating-content/` subtree without errors.

---

### Task 2.6 — Advanced pages (3 files)

**Dependencies:** Task 1.4

**What it does:** Creates placeholder pages for the three Advanced section pages.

**Create** `src/content/docs/advanced/sandbox.mdx`:
```mdx
---
title: Sandbox
description: How to run AI coding tools in an isolated bubblewrap sandbox.
---

Sandbox documentation coming soon.
```

**Create** `src/content/docs/advanced/team-setup.mdx`:
```mdx
---
title: Team Setup
description: Using syllago registries for team standardization.
---

Team setup documentation coming soon.
```

**Create** `src/content/docs/advanced/troubleshooting.mdx`:
```mdx
---
title: Troubleshooting
description: Common issues and solutions.
---

Troubleshooting documentation coming soon.
```

**Verification:** All three slugs resolve. After this task, `npm run build` must complete fully without any missing slug errors — all 24 content pages are now present (excluding the landing page, which is Phase 3).

---

## Phase 3 — Hero Landing Page

---

### Task 3.1 — Create the hero landing page

**Dependencies:** Phase 2 complete (all placeholder pages exist)

**What it does:** Creates `src/content/docs/index.mdx` with Starlight's `hero` frontmatter layout. This is the page served at the root of the site. The hero layout renders a full-width hero section with tagline, description, CTA buttons, and a feature grid below.

**Why `index.mdx` at the docs root:** Starlight maps `src/content/docs/index.mdx` to the `/` route of the docs site. The `template: splash` frontmatter switches from the default sidebar+content layout to the full-width hero layout.

**Create** `src/content/docs/index.mdx`:
```mdx
---
title: syllago
description: The package manager for AI coding tool content. Install rules, skills, agents, and more across 11 AI coding tool providers.
template: splash
hero:
  tagline: The package manager for AI coding tool content.
  actions:
    - text: Get Started
      link: /syllago-docs/getting-started/installation/
      icon: right-arrow
      variant: primary
    - text: View on GitHub
      link: https://github.com/OpenScribbler/syllago
      icon: external
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';

## Why syllago?

<CardGrid>
  <Card title="11 providers, one workflow" icon="puzzle">
    Install rules, skills, agents, and more to Claude Code, Cursor, Windsurf, Copilot, and 7 others — all from a single command.
  </Card>
  <Card title="Automatic format conversion" icon="translate">
    Write content once. Syllago handles the translation to each provider's format. Intent carries forward — nothing is lost.
  </Card>
  <Card title="Team registries" icon="group">
    Publish your org's standards to a git registry. Everyone on the team installs the same content from the same source.
  </Card>
  <Card title="No lock-in" icon="open-book">
    Content you author works across providers. Switch tools, keep your content. Syllago manages the format differences.
  </Card>
</CardGrid>
```

**Note on link paths:** The CTA button link uses the full `/syllago-docs/getting-started/installation/` path (including the `base` prefix) because Starlight's hero `actions` links are not automatically prefixed. All internal links in `.mdx` content outside of the sidebar config must include the base prefix.

**Verification:** `npm run build` completes. `dist/syllago-docs/index.html` exists and contains "The package manager for AI coding tool content."

---

### Task 3.2 — Smoke-test the build locally

**Dependencies:** Task 3.1

**What it does:** Runs the Astro preview server against the built output to verify the site renders correctly before wiring up deployment.

**Commands:**
```bash
npm run build && npm run preview
```

**What to check:**
- Root URL (`http://localhost:4321/syllago-docs/`) shows the hero landing page with tagline and both CTA buttons.
- "Get Started" button links to `/syllago-docs/getting-started/installation/`.
- The sidebar renders all four top-level sections: Getting Started, Using Syllago, Creating Content, Advanced.
- The Providers subsection shows the index + 11 provider entries.
- Clicking any sidebar link loads the corresponding placeholder page.
- The Flexoki theme is applied (warm color palette, not the default Starlight blue).
- Dark/light mode toggle works.

**Verification:** All checks pass. No broken links in the sidebar. No 404s for any page in the IA.

---

## Phase 4 — GitHub Actions Deployment

---

### Task 4.1 — Configure GitHub Pages on the repo

**Dependencies:** Task 1.1

**What it does:** Enables GitHub Pages on the `syllago-docs` repo and sets the source to "GitHub Actions" (not the legacy branch-based deployment). This must be done before the workflow can deploy.

**Commands:**
```bash
gh api \
  --method POST \
  -H "Accept: application/vnd.github+json" \
  repos/OpenScribbler/syllago-docs/pages \
  -f build_type=workflow
```

If the above API call fails because Pages has not yet been initialized, use the GitHub web UI instead:
1. Go to `https://github.com/OpenScribbler/syllago-docs/settings/pages`
2. Under "Build and deployment", set Source to "GitHub Actions"
3. Save

**Verification:** `gh api repos/OpenScribbler/syllago-docs/pages` returns `"build_type": "workflow"`.

---

### Task 4.2 — Create the GitHub Actions deploy workflow

**Dependencies:** Task 4.1

**What it does:** Creates `.github/workflows/deploy.yml`. This workflow uses the official `withastro/action` GitHub Action, which handles installing Node.js, running `npm install`, running `npm run build`, and uploading the `dist/` directory as a Pages artifact in one step.

**Why `withastro/action`:** It handles the Node setup, build, and artifact upload in a single action maintained by the Astro team. The alternative (manual steps) requires wiring `actions/upload-pages-artifact` separately and is more brittle.

**Create** `.github/workflows/deploy.yml`:
```yaml
name: Deploy to GitHub Pages

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Build with Astro
        uses: withastro/action@v5
        with:
          path: .
          node-version: 20

  deploy:
    name: Deploy
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

**Why `concurrency: cancel-in-progress: true`:** If two pushes land in quick succession, the first deploy job is cancelled when the second starts. This prevents a stale build from overwriting a newer one mid-flight.

**Why `workflow_dispatch`:** Allows manually triggering a deploy from the GitHub Actions UI without a code push. Useful for first deployment verification and for doc-only fixes that don't touch code.

**Verification:** File exists at `.github/workflows/deploy.yml`.

---

### Task 4.3 — Commit all files and push to trigger first deployment

**Dependencies:** Task 4.2, Phase 3 complete

**What it does:** Stages all files, creates the initial commit, and pushes to `main`. This triggers the deploy workflow for the first time.

**Commands:**
```bash
git add .
git commit -m "feat: initial Astro Starlight scaffold with Flexoki theme and full IA"
git push origin main
```

**Verification:** `gh run list --repo OpenScribbler/syllago-docs` shows a workflow run in progress or completed. `gh run watch --repo OpenScribbler/syllago-docs` streams the build and deploy logs.

---

### Task 4.4 — Verify the live deployment

**Dependencies:** Task 4.3

**What it does:** Confirms the site is live and correctly served at the GitHub Pages URL.

**Wait for deployment:** Typically 1-3 minutes after the push. Check status:
```bash
gh run list --repo OpenScribbler/syllago-docs --limit 5
```

**Check the live site:**
- Open `https://openscribbler.github.io/syllago-docs/` in a browser.
- Verify the hero page renders with the tagline "The package manager for AI coding tool content."
- Verify the sidebar is present and shows all four top-level sections.
- Click through two or three sidebar links and verify they load without 404.
- Verify the Flexoki theme is applied (not default Starlight blue).
- Verify the GitHub link in the social nav links to `https://github.com/OpenScribbler/syllago`.

**Expected URL:** `https://openscribbler.github.io/syllago-docs/`

**Verification:** All checks pass. No 404s. The site is publicly accessible.

---

## Phase 5 — Repo Hygiene

---

### Task 5.1 — Add `.gitignore`

**Dependencies:** Task 1.2

**What it does:** Creates the standard `.gitignore` for Astro projects. The `dist/` and `.astro/` directories are build artifacts and must not be committed. `node_modules/` is obvious but included for completeness.

**Note:** This task MUST be completed before Task 4.3 (initial commit). Without .gitignore, `git add .` will stage dist/, .astro/, and node_modules/. Do this immediately after the scaffold (Task 1.2).

**Create** `.gitignore`:
```
# Build output
dist/
.astro/

# Dependencies
node_modules/

# Environment
.env
.env.*
!.env.example

# OS
.DS_Store
Thumbs.db
```

**Verification:** `git status` does not show `dist/`, `.astro/`, or `node_modules/` as untracked files.

---

### Task 5.2 — Tag the initial release

**Dependencies:** Task 4.4 (live deployment confirmed)

**What it does:** Creates the initial version tag `v0.0.1` on the `syllago-docs` repo. This establishes the tagging pattern that the release-gate mechanism (the `/release` skill in the `syllago` CLI repo) will check against in the future.

**Why `v0.0.1` not `v1.0.0`:** The docs site is scaffolding only at this point — no real content. The tag matches the intent: infrastructure in place, content pending. The gate mechanism requires a matching tag for each CLI release, so the first content-complete tag will be whatever version syllago ships at v1.0.0.

**Commands:**
```bash
git tag v0.0.1
git push origin v0.0.1
```

**Verification:** `git ls-remote --tags origin` shows the `v0.0.1` tag. (Note: `gh release list` won't show this — it only lists GitHub Releases, not lightweight git tags.)

---

## Complete File Tree

After all phases, the `syllago-docs` repo has this structure:

```
syllago-docs/
├── .github/
│   └── workflows/
│       └── deploy.yml
├── .gitignore
├── astro.config.mjs
├── package.json
├── tsconfig.json
├── public/
│   └── favicon.svg
└── src/
    ├── env.d.ts
    └── content/
        ├── config.ts
        └── docs/
            ├── index.mdx                          ← hero landing page
            ├── getting-started/
            │   ├── installation.mdx
            │   ├── quick-start.mdx
            │   └── core-concepts.mdx
            ├── using-syllago/
            │   ├── tui.mdx
            │   ├── cli-reference.mdx
            │   ├── content-types.mdx
            │   └── providers/
            │       ├── index.mdx
            │       ├── claude-code.mdx
            │       ├── cursor.mdx
            │       ├── windsurf.mdx
            │       ├── copilot.mdx
            │       ├── cline.mdx
            │       ├── roo-code.mdx
            │       ├── kiro.mdx
            │       ├── zed.mdx
            │       ├── gemini-cli.mdx
            │       ├── opencode.mdx
            │       └── codex.mdx
            ├── creating-content/
            │   ├── authoring-guide.mdx
            │   ├── syllago-yaml.mdx
            │   ├── registries.mdx
            │   └── format-conversion.mdx
            └── advanced/
                ├── sandbox.mdx
                ├── team-setup.mdx
                └── troubleshooting.mdx
```

Total content pages: 25 (1 landing + 3 getting-started + 3 using-syllago + 12 providers + 4 creating-content + 3 advanced)

---

## Task Dependency Map

```
1.1 Create repo
└── 1.2 Scaffold Astro Starlight
    └── 1.3 Install + configure Flexoki
        └── 1.4 Delete scaffold example content
            ├── 2.1 Getting Started pages
            ├── 2.2 Using Syllago pages
            ├── 2.3 Providers index
            │   └── 2.4 Provider detail pages (11)
            ├── 2.5 Creating Content pages
            └── 2.6 Advanced pages
                └── 3.1 Hero landing page
                    └── 3.2 Local smoke test
                        └── 4.3 Commit + push
                            └── 4.4 Verify live deployment
                                └── 5.2 Tag v0.0.1

1.1 → 4.1 Configure GitHub Pages (parallel with scaffold work)
1.2 → 5.1 .gitignore (MUST be done before 4.3)
5.1 → 4.3 (gitignore required before initial commit)
4.1 + 3.2 → 4.2 Create deploy workflow
```

---

## Gotchas

**`base` prefix in internal links:** Astro's `base` config prefixes all asset URLs automatically, but `<a href="...">` links in `.mdx` content and Starlight hero action links are NOT automatically prefixed. Any hardcoded internal link in content must include `/syllago-docs/` at the start. Sidebar `slug` values do NOT need the prefix — Starlight handles those.

**`starlight-theme-flexoki` version pinning:** Verify the package exists on npm before Task 1.3 (`npm info starlight-theme-flexoki`). If the package name has changed, check the Starlight community themes page for the correct name.

**GitHub Pages environment:** The `deploy` job references `environment: github-pages`. This environment is created automatically by GitHub when Pages is enabled, but the workflow will fail if Pages is not yet enabled on the repo (Task 4.1 must come first).

**`withastro/action` version:** Task 4.2 uses `@v5`. Verify the latest stable version at `https://github.com/withastro/action/releases` before running if significant time has passed since this plan was written.

**`index.mdx` slug collision:** Starlight treats `providers/index.mdx` as the `providers/` route (no `index` in the URL). The sidebar config uses `slug: 'using-syllago/providers/index'` — this matches Starlight's internal slug representation for index files even though the URL is `/using-syllago/providers/`.

**Node version:** `withastro/action@v3` defaults to Node 20. The `node-version: 20` field in the workflow is explicit for clarity but matches the default. If a newer LTS is required, update the field.
