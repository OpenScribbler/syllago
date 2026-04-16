# Local Dashboard Design

**Date:** 2026-04-15
**Status:** Design complete — awaiting aesthetic direction (see `local-dashboard-mockups/`)
**Feature:** `syllago dashboard` — local web UI served by the syllago binary

## Context

Syllago currently ships a TUI (`syllago tui`) for interactive use. Non-terminal users and teammates evaluating syllago need a friendlier entry point. A hosted webapp is the long-term goal, but building one requires product validation, infra decisions, and auth. A **local dashboard** served by the syllago binary itself is the stepping stone: it lets us validate UX patterns, content displays, and interaction flows in a zero-infrastructure form factor that also serves real day-to-day users.

Inspired by [openwolf](https://github.com/cytostack/openwolf), which bundles a React dashboard into a Node CLI via `go:embed`-equivalent patterns. Syllago's Go binary can do the same thing.

## Goals

- **Zero install friction.** `syllago dashboard` opens a browser; no separate server, no Docker, no dependencies.
- **Reuses the eventual webapp.** The frontend code built here becomes the foundation for the hosted product.
- **Read-only v1.** Prove plumbing with safe operations. Write operations come later.
- **Live updates.** File watcher pushes catalog/provider changes to the UI via WebSocket.
- **Flexoki-themed, TUI-parity.** Same color palette and visual language as the TUI so users don't context-switch.

## Non-Goals (v1)

- Editing, installing, removing content through the UI
- Authentication or multi-user support
- Remote access (localhost only, 127.0.0.1 bind)
- Mobile optimization (desktop-first; responsive comes later)
- Offline PWA behavior
- Analytics beyond existing telemetry

## Architecture

```
cli/
├── cmd/syllago/
│   └── dashboard_cmd.go              # cobra command: syllago dashboard [--port N] [--no-open]
└── internal/dashboard/
    ├── server.go                     # net/http server, embed FS, graceful shutdown
    ├── websocket.go                  # coder/websocket hub, client tracking, broadcast
    ├── watcher.go                    # fsnotify wrapper, debounce, emit events
    ├── api/
    │   ├── catalog.go                # GET /api/catalog — list content
    │   ├── providers.go              # GET /api/providers — installed providers + counts
    │   └── health.go                 # GET /api/health — liveness, uptime
    └── web/
        ├── dist/                     # build output (gitignored, embedded at compile time)
        ├── src/                      # Astro source
        │   ├── pages/
        │   │   ├── index.astro       # redirect to /catalog
        │   │   ├── catalog.astro
        │   │   └── providers.astro
        │   ├── layouts/
        │   │   └── Dashboard.astro   # shared shell (sidebar, header, main)
        │   ├── components/
        │   │   ├── Sidebar.astro
        │   │   ├── Header.astro
        │   │   ├── MetaPanel.astro
        │   │   ├── CatalogTable.astro
        │   │   └── ProviderCard.astro
        │   ├── lib/
        │   │   ├── api.ts            # typed fetch helpers
        │   │   ├── ws.ts             # WebSocket client with reconnect
        │   │   └── types.ts          # shared types (mirror Go structs)
        │   └── styles/
        │       ├── tokens.css        # Flexoki CSS custom properties
        │       ├── reset.css
        │       └── components/*.css  # per-component styles
        ├── astro.config.mjs          # output: 'static'
        ├── package.json
        └── tsconfig.json
```

## Key Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Astro, static output | Matches user's existing expertise. Multi-page structure fits content-browser shape better than React SPA. Zero JS by default, islands when needed. |
| 2 | No Tailwind — custom CSS variables | User preference. Flexoki palette maps cleanly to custom properties. Easier to keep TUI/dashboard visual parity. |
| 3 | No client framework in v1; Svelte for islands when needed | Lightest runtime (~2KB) for Astro islands. Only reach for it when interactivity demands it (modals, filters). |
| 4 | `net/http` + `coder/websocket` | Stdlib HTTP is sufficient. `coder/websocket` is the modern, maintained choice over `gorilla/websocket` (archived 2024). |
| 5 | `fsnotify` for filesystem watching | Cross-platform, widely used, same pattern as openwolf's chokidar. |
| 6 | `//go:embed all:web/dist` | Single-binary distribution. The `all:` prefix includes dotfiles (Astro generates some). |
| 7 | Port 19099 default, fallback to `:0` on collision | Clean gap in registered range (mid-19000s). Below 32768 for cross-platform safety. OS-picked fallback avoids user friction. |
| 8 | Bind 127.0.0.1 only | No remote access. Reduces threat surface and avoids firewall prompts on macOS/Windows. |
| 9 | 2 panels in v1: Catalog + Providers | Smallest surface that proves the stack. Add Registries, Loadouts, Settings once patterns are locked in. |
| 10 | Read-only v1 | Write operations (install, edit, remove) require careful UX + confirmation flows. Defer until v2. |
| 11 | Vite dev proxy for local development | `astro dev` runs on its own port, proxies `/api` + `/ws` to the Go server. Standard pattern. |
| 12 | No build tag gymnastics | The `web/dist/` embed is always present (empty file shim committed for `go build` without the frontend). |
| 13 | `web/dist/` gitignored | Build artifact. CI builds Astro before `go build`. Local dev uses dev proxy. |

## Flow

### Startup (`syllago dashboard`)
1. Parse flags (`--port`, `--no-open`)
2. Try to bind requested port; on failure, bind `:0` and log the chosen port
3. Start file watcher on `~/.syllago/` + project-local `.syllago/`
4. Start WebSocket hub
5. Start HTTP server
6. Unless `--no-open`, open `http://127.0.0.1:<port>` in default browser
7. Block on shutdown signal; drain WebSocket clients; stop watcher; exit clean

### Live Updates
1. `fsnotify` emits event (create/modify/delete in watched dir)
2. Debouncer coalesces rapid bursts (e.g., editor saves)
3. Dashboard server rescans affected catalog portion
4. Delta broadcast over WebSocket: `{type: "catalog.changed", items: [...]}`
5. Frontend receives message, updates local state (island reactivity or full reload depending on view)

### HTTP Endpoints (v1)
- `GET /api/catalog` — all catalog items (syllago's canonical format)
- `GET /api/providers` — installed providers + item counts + health
- `GET /api/health` — `{status, uptime, version}`
- `GET /ws` — WebSocket upgrade

## Success Criteria

1. `syllago dashboard` opens a browser to the dashboard in <2s from a warm cache
2. Single binary: no separate assets directory to ship
3. Catalog panel renders all content types with correct icons/colors matching TUI
4. Providers panel shows every detected provider with installed item counts
5. File changes propagate to the UI in <500ms (debounced)
6. Port collisions self-heal (fallback `:0` with logged port)
7. Dashboard survives `Ctrl-C` cleanly (no orphaned goroutines or leaked fd's)
8. Light/dark mode follows system preference, overridable via UI toggle
9. Keyboard shortcuts match TUI where applicable (`g` then `c` for catalog, `g` then `p` for providers)
10. Visual parity: a TUI user opening the dashboard recognizes the palette, typography hierarchy, and info density

## Resolved During Design

| Question | Decision | Why |
|----------|----------|-----|
| Framework? | Astro static + custom CSS | User preference; content-shaped app fits MPA better than SPA |
| Tailwind? | No — CSS custom properties | User preference; simpler TUI parity |
| Client JS? | None by default, Svelte when needed | Lightest runtime for islands |
| Build flow? | Astro → `web/dist/` → `//go:embed` | Single-binary shipping |
| Server lib? | `net/http` + `coder/websocket` | Stdlib + modern WS lib; gorilla archived |
| Watcher? | `fsnotify` | Cross-platform standard |
| Port? | 19099 (clean gap, below 32768) with `:0` fallback | Research of IANA + common collisions |
| Bind? | 127.0.0.1 | Local-only, no firewall prompts |
| Scope v1? | 2 panels: Catalog + Providers | Prove plumbing, smallest surface |
| Writes? | Read-only in v1 | Defer UX complexity to v2 |
| Dev flow? | `astro dev` + Vite proxy | Standard Astro + API server pattern |
| Build tags? | None | Embed shim keeps `go build` green without frontend |
| `dist/` in git? | No, gitignored | CI builds before `go build`; committed shim preserved |

## Next Steps

1. **Pick aesthetic direction.** 10 HTML mockups in `docs/plans/local-dashboard-mockups/` — open `index.html` in a browser, compare, pick one (or a blend).
2. **Resume `/develop` workflow.** State file `.develop/local-dashboard.json`, dispatch plan-writing subagent to produce the implementation plan with task breakdown.
3. **Bead creation.** Convert plan into beads with test→impl pairs per the established pattern.
4. **Execute.** Three-agent model (executor + verifier) per the develop skill.

## References

- openwolf: https://github.com/cytostack/openwolf — Node CLI with embedded React dashboard (reference architecture)
- Flexoki palette: https://stephango.com/flexoki — color system (already in TUI)
- `coder/websocket`: https://github.com/coder/websocket — successor to gorilla/websocket
- `fsnotify`: https://github.com/fsnotify/fsnotify — filesystem watcher
- Astro: https://astro.build — static output mode, islands architecture
