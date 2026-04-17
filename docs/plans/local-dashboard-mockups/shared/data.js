/* Shared mock catalog + provider data.
 * Every mockup renders this identical dataset so aesthetic differences are
 * what you're comparing — not content variation. */

window.MOCK_CATALOG = [
  {
    name: "claude-rules-base",
    type: "rules",
    provider: "claude-code",
    origin: "library",
    files: 4,
    size: "12 KB",
    updated: "2 hours ago",
    description: "Baseline Claude Code rules for enterprise projects — tone, safety, and output conventions.",
  },
  {
    name: "astro-skills",
    type: "skills",
    provider: "claude-code",
    origin: "registry",
    files: 12,
    size: "48 KB",
    updated: "yesterday",
    description: "Astro documentation helpers: component scaffolding, content collection patterns, SSG gotchas.",
  },
  {
    name: "typescript-strict",
    type: "rules",
    provider: "multi",
    origin: "registry",
    files: 2,
    size: "6 KB",
    updated: "3 days ago",
    description: "Strict TS conventions — no implicit any, prefer functional patterns, Vitest for tests.",
  },
  {
    name: "git-commit-helper",
    type: "commands",
    provider: "claude-code",
    origin: "library",
    files: 1,
    size: "3 KB",
    updated: "1 week ago",
    description: "Slash command that drafts conventional-commit messages from staged diffs.",
  },
  {
    name: "mcp-postgres",
    type: "mcp",
    provider: "claude-code",
    origin: "registry",
    files: 1,
    size: "2 KB",
    updated: "2 weeks ago",
    description: "MCP server config for read-only Postgres queries over your dev database.",
  },
  {
    name: "agent-reviewer",
    type: "agents",
    provider: "claude-code",
    origin: "library",
    files: 1,
    size: "4 KB",
    updated: "today",
    description: "Code-review subagent — reads diffs, flags correctness issues, never writes files.",
  },
  {
    name: "python-linter",
    type: "hooks",
    provider: "cursor",
    origin: "shared",
    files: 1,
    size: "1 KB",
    updated: "4 days ago",
    description: "Post-edit hook: run ruff + mypy on any modified .py file, block on errors.",
  },
  {
    name: "loadout-nextjs",
    type: "loadouts",
    provider: "multi",
    origin: "registry",
    files: 7,
    size: "32 KB",
    updated: "5 hours ago",
    description: "Curated bundle: Next.js rules, React skills, ESLint hook, API route agent. Apply to any provider.",
  },
];

window.MOCK_PROVIDERS = [
  { slug: "claude-code", name: "Claude Code", installed: 14, detected: true,  version: "1.4.2" },
  { slug: "cursor",      name: "Cursor",      installed: 3,  detected: true,  version: "0.42" },
  { slug: "gemini",      name: "Gemini CLI",  installed: 0,  detected: true,  version: "0.8.0" },
  { slug: "copilot",     name: "Copilot",     installed: 0,  detected: false, version: null   },
];

/* Type color mapping — consistent across all mockups */
window.TYPE_COLORS = {
  rules:    "cyan",
  skills:   "blue",
  agents:   "purple",
  commands: "green",
  hooks:    "orange",
  mcp:      "magenta",
  prompts:  "yellow",
  loadouts: "red",
};

/* Simple theme toggle — every mockup uses this */
window.initThemeToggle = function (btnId) {
  const btn = document.getElementById(btnId);
  if (!btn) return;
  const saved = localStorage.getItem("syllago-mockup-theme");
  if (saved) document.documentElement.setAttribute("data-theme", saved);
  btn.addEventListener("click", () => {
    const current = document.documentElement.getAttribute("data-theme");
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const currentEffective = current || (prefersDark ? "dark" : "light");
    const next = currentEffective === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    localStorage.setItem("syllago-mockup-theme", next);
  });
};
