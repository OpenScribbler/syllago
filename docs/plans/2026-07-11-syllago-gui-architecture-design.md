# Syllago GUI Client — Architecture Design Document

**Goal:** Wire the audited HTML mockup up as a real cross-platform desktop GUI client with full TUI parity, sharing the existing Go business logic.

**Decision Date:** 2026-07-11

---

## Problem Statement

The GUI design phase is complete: a high-fidelity interactive HTML mockup has full functional parity with the TUI (audited feature-by-feature against the Go code). The mockup is the design contract — 9 pages, install/add full-page wizard flows, an 11-dialog modal family, trust surfaces (Dual-Attested chains, TOFU, publisher warnings), toast tiers, and composable library filters.

What's missing is the client architecture: shell technology, how the frontend reaches the Go business logic, where the code lives, how state stays fresh, and what v1 ships. Long-term direction is desktop-first with SaaS-ready patterns, so reversibility of the backend boundary matters.

Design artifacts:

- Interactive mockup (design contract): `docs/plans/syllago-gui-mockups.html` (durable copy, committed); artifact URL `https://claude.ai/code/artifact/a3b6619e-3c4b-4116-8172-c9fa6430bd00`
- Flexoki tokens sourced from `cli/internal/tui/styles.go`

## Proposed Solution

A **Wails v3** desktop application with a **React + TypeScript** frontend, living in the existing repo and Go module as a second binary. The frontend talks only to a new **service layer** (`cli/internal/guiapi`) — typed request/response DTOs shaped like a future HTTP API — never to raw internal packages. Catalog freshness comes from a debounced fsnotify watcher pushing typed Wails events to React. v1 ships full mockup parity.

This is the idiomatic Go "core library + thin adapters" shape (Syncthing, Tailscale, gh, Hugo): the cobra commands and TUI are two existing adapters over `cli/internal/*`; `guiapi` is the third.

## Architecture

```
syllago/                        (existing repo, existing Go module at cli/)
├── cli/
│   ├── cmd/syllago/            existing CLI+TUI binary (unchanged)
│   ├── cmd/syllago-gui/        NEW: Wails v3 entry point + app wiring
│   │   └── frontend/           NEW: React+TS+Vite app (dist/ embedded via go:embed)
│   └── internal/
│       ├── guiapi/             NEW: service layer — the only package Wails binds
│       └── catalog, installer, converter, loadout, moat, ...   (unchanged)
```

- **Two binaries, one module.** `syllago-gui` is a separate binary; the CLI stays free of webview/CGO baggage. Both compile from the same module, so `guiapi` imports internals directly and no public Go API is exported.
- **Frontend inside `cmd/syllago-gui/`** because `go:embed` cannot reach outside the package tree; this is also Wails' standard layout, so its tooling works unmodified.
- **`guiapi` is a strict boundary**: services grouped by domain (Catalog, Install, Add, Loadout, Registry, Config, Trust), each exposing typed request/response DTOs. Wails generates the TypeScript types from these DTOs — the frontend contract is machine-derived, never hand-maintained. Raw internal types never cross. Logic never accretes in the adapter: anything the GUI needs that lives in cobra command funcs or TUI update loops gets pushed down into internals first.
- **Build/release**: `make gui-build` wraps `wails3` tasks alongside the existing `make build`. Release CI gains macOS + Windows runners and Wails packaging (.app + notarization, NSIS installer, AppImage + webkit2gtk-4.1-only native Linux packages); artifacts join the existing cosign signing flow.

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Shell technology | Wails | Only shell with native Go bindings + typed Go→JS events — no sidecar process, no second toolchain, ~15MB installs, OS patches the webview. Accepts WebKitGTK risk on Linux with explicit mitigations (below). |
| Wails version | v3 alpha, pinned exact versions | Built-in auto-updater (delta patches), first-class typed events, built-in MCP server for agent-driven e2e testing. v2 would guarantee a whole-app migration later; pinned alphas absorb churn incrementally while the surface is small. |
| Frontend framework | React + TypeScript (strict) + Vite + Vitest | Most reliable dialect for delegated implementation (model fluency), first-class Wails template, component reuse if a web/SaaS frontend ever happens. Runtime weight is negligible in a desktop webview at this UI scale. |
| Repo layout | Same repo, same Go module (`cli/cmd/syllago-gui`) | Effectively forced: `cli/internal/*` packages are only importable within the `cli` module. Alternatives (public API export, logic duplication) are worse commitments. |
| Backend boundary | Service layer (`cli/internal/guiapi`), in-process | Stateless typed DTOs shaped like a future HTTP API. Internal refactors stop at the DTO layer; lifts into `syllago serve` if SaaS gets real — only the transport changes. |
| State/refresh model | fsnotify → debounce (~300ms) → rescan → Wails event → React refetch | Push, not polling. Covers GUI-initiated changes, CLI writes while GUI is open, and manual edits. Debouncing is a correctness requirement (fsnotify delivers raw bursts). |
| Interactive prompts | Two-phase `DecisionRequired{kind, context}` | Services can't block on user callbacks. Attempt → typed decision-required result → GUI renders matching modal → re-invoke with decision attached. The mockup's dialog family maps onto `kind` nearly one-to-one. |
| v1 scope | Full mockup parity (all 9 pages incl. loadout cards) | The mockup is already the audited spec. Note: loadout apply/try has no TUI surface today — the GUI is first to expose it, requiring service endpoints over loadout internals currently exercised only by the CLI. |
| Analytics | Reuse `cli/internal/telemetry` Go-side; no PostHog JS SDK | One opt-in consent state across CLI/TUI/GUI; no-PII discipline stays in one audited place; existing EventCatalog + gendocs drift detection keeps working. `guiapi` services fire the same events as their CLI equivalents with a `client: gui` property. GUI first-run shows the mockup's consent modal (same opt-in semantics as the TUI overlay; legacy force-disabled state respected). |
| Feedback mechanism | "Report an issue" → prefilled GitHub issue URL in browser | No token management, no server. Prefills version, OS, webview version, and the structured error envelope when launched from an error toast. User reviews/edits the full body in their own GitHub session before submitting — that review is the privacy story. Content names/paths never prefilled. |
| About modal | Standard About dialog | App name/logo, version + commit + build date (ldflags, same as CLI), Wails/webview versions, license, links (repo, docs, release notes), telemetry status, and "check for updates" via the Wails v3 updater. |

### Shell decision — research findings (July 2026)

Live research drove the shell choice; recording the load-bearing facts:

- **Linux is the battleground.** Tauri and Wails both use WebKitGTK, which is in a rough patch: webkit2gtk-4.0 was terminally removed upstream (WebKitGTK 2.52.0, March 2026) and distros are purging it, so 4.1 is mandatory; 4.1 links libsoup3, which hard-crashes any process also loading libsoup2. An open, unresolved NVIDIA + Wayland DMABUF crash on transparent surfaces (Tauri #14924) has no clean fix — only workarounds that trade one defect for another.
- **Mitigations adopted**: target webkit2gtk-4.1 only; ship AppImage alongside native packages; keep the window opaque (sidesteps the worst DMABUF bug class); document NVIDIA env-var workarounds.
- **Electron rejected**: immune to the WebKitGTK class (bundles Chromium) but forces a Go sidecar boundary, 150–200MB installs, and — decisive for a security-focused project — we would ship our own browser and own its CVEs on Chromium's 8-week cadence, a standing re-release treadmill.
- **Tauri v2 rejected**: for a Go backend it is strictly dominated — all of Wails' Linux liabilities plus the sidecar boundary, with no Go bindings. Its Servo-based escape hatch (Verso runtime) is real but explicitly pre-production.
- **Wails status**: v2 stable and maintained (v2.13.0, 2026-07-06); v3 alpha with near-daily releases, production users, built-in updater, typed events, MCP server.

## Data Flow

**Request/response:** React component → generated TS binding → `guiapi` service → internal packages → DTO back. Services are stateless; every call is a complete request. Wizard state (current step, selections) lives entirely in React, matching the mockup's `FLOWS`/`FLOW_CFG` structure.

**Push:** fsnotify watches the catalog roots → debounced rescan → `catalog:changed` Wails event → React invalidates and refetches affected queries. Long operations (install, registry sync) additionally emit progress events for toast/progress surfaces.

**Interactive prompts (two-phase):** operations that prompt mid-flight in the CLI/TUI (TOFU trust, publisher-warn install-anyway, private-content, D17 reinstall/stale-record) instead return a typed `DecisionRequired{kind, context}` result; the GUI renders the matching modal from the dialog family; the operation is re-invoked with the decision attached. No cross-boundary callbacks, no server-side session state. This is the API-ification of the CLI's existing attempt → retry-with-`--force`/`--trust` pattern.

## Error Handling

- **Errors cross as data, not strings.** `guiapi` responses carry the existing structured error envelope (`cli/internal/output/errors.go` — error codes, `{code, message, suggestion}`) inside the DTO, so codes and suggestions survive into TypeScript. Frontend maps them onto the mockup's toast tiers: recoverable → warning toast; fatal → persistent error toast with Copy and Report-an-issue actions.
- **Panics** in bound methods are recovered and surfaced as persistent error toasts.
- **fsnotify failure** (exotic filesystems, watch limits) degrades to the manual refresh button — same affordance as the TUI's `R` key, already in the mockup.
- **Version skew:** none within the app (one binary). A separately installed CLI writing to the same catalog is the benign case the watcher handles by design; the on-disk formats are the contract.

## Testing

Per the repo's existing testing requirements:

- **`guiapi` is a core logic package** → 80% coverage target, table-driven tests, hand-crafted stubs, no mocking libraries. Happy path + at least one error path per service; every `DecisionRequired` kind covered.
- **Frontend:** Vitest + Testing Library for the wizard and modal state machines — the highest-correctness-risk frontend code.
- **End-to-end:** Wails v3's built-in MCP server lets an agent drive the real running app for smoke flows (real software, not fixtures, per existing smoke-test policy).
- **Visual verification** is delegated to a Sonnet subagent per model-routing rules (high token volume, low judgment per step).

## Success Criteria

- All 9 mockup pages, both full-page wizards, and the full modal family function against real catalog data on Linux, macOS, and Windows.
- Full TUI parity: every operation the TUI supports works in the GUI, including trust flows (TOFU, publisher-warn install-anyway, Dual-Attested chains, stale `!` state).
- Catalog changes made by the CLI while the GUI is open appear without manual refresh.
- The frontend imports zero internal Go types — only generated DTO bindings.
- Telemetry events from the GUI appear in PostHog segmented by `client: gui`, gated by the shared opt-in consent.
- Release pipeline produces signed artifacts for all three platforms (notarized .app, NSIS installer, AppImage + 4.1-only native packages).

## Resolved During Design

| Question | Decision | Reasoning |
|----------|----------|-----------|
| Shell: Wails vs Tauri vs Electron vs Go-native? | Wails | Native Go integration; Tauri dominated for Go backends; Electron's Chromium-CVE ownership rejected; Go-native discards the HTML design contract. Linux WebKitGTK risk accepted with mitigations. |
| Wails v2 (stable) or v3 (alpha)? | v3, pinned | Updater, typed events, MCP server; avoids a guaranteed whole-app v2→v3 migration; churn absorbed incrementally. |
| Frontend framework? | React + TS + Vite + Vitest | Model fluency for delegated implementation, Wails template support, SaaS component reuse. Vanilla rejected: 9 pages + wizards + live events means hand-rolling an ad-hoc framework (largest correctness risk). |
| Same repo or separate? | Same repo, same module | Go `internal` visibility makes this effectively forced; alternatives commit to a public API or duplicate logic. |
| In-process bindings, daemon, or CLI shell-out? | In-process service layer | Daemon pays lifecycle/auth/versioning costs now for a SaaS that doesn't exist; shell-out has no event push and partial `--json` coverage; direct internal binding leaks TUI-shaped types into the frontend contract. |
| How do blocking prompts cross a service boundary? | Two-phase `DecisionRequired` | No callbacks across the boundary; stateless services; mockup dialog family maps onto decision kinds. |
| GUI state freshness? | fsnotify + debounce + event push | Push beats polling; covers external (CLI/manual) writes; manual refresh as degraded fallback. |
| v1 scope? | Full mockup parity | Mockup is the audited spec; includes GUI-first loadout apply/try surface. |
| Analytics? | Reuse Go telemetry package, `client: gui` property | Single consent state, single PII-discipline point, existing catalog/gendocs machinery. No frontend PostHog SDK. |
| Feedback mechanism? | Prefilled GitHub issue URL | Zero auth/server surface; user reviews full body before submitting (privacy); error toast integration prefills the structured envelope. |
| About modal? | Yes — 12th dialog | Version/commit/build date, webview + Wails versions, license, links, telemetry status, update check (Wails updater). |

**Design-contract addendums** — both now present in the mockup (added post-signoff, browser-verified): the About modal (statusbar version + help overlay entry points, check-for-updates via the Wails updater) and Report-an-issue (help overlay footer + a Report action on persistent error toasts).

**Library scope amendment (2026-07-11):** GUI v1's Library targets the **Library Unified View** state specced in bead `syllago-mol-2hw`, not the current TUI behavior — registry items inline, in-library/not-in-library State filter, one-time two-step-model explainer, and Add vs. Add + Install actions. All of these are in the mockup. Consequence: the GUI is the first implementation of unified-view behavior; the later TUI work follows the GUI's decisions here, inverting the usual TUI-decides-first parity direction for this one surface.

**Out of scope for this design** (tracked separately): the TUI-side gaps the parity audit surfaced — unreachable monolithic rule-file import (`addSourceMonolithic`), loadout apply/try/create/status/export having no TUI surface, and the documented `n` (Create) no-op. These are candidate beads independent of GUI work.

---

## Environment Spike Results (2026-07-11)

The Wails v3 environment spike (throwaway project at `.develop/wails-spike/`) **passed** on midbiscuit (WSLg, Ubuntu 24.04):

- **Rendering confirmed**: Wails v3.0.0-alpha2.117 window renders correctly under WSLg with webkit2gtk-4.1 (2.52.3). Interactive round-trip verified: button click → Go service → table of real detection results (claude-code, gemini-cli, codex, kiro, amp all detected ✓).
- **cli module binding confirmed**: the spike imports `cli/internal/provider` and binds a DTO-mapping service — the exact shape planned for `guiapi`. (Spike used module path `github.com/OpenScribbler/syllago/cli/spike` + `replace` to satisfy Go's `internal` visibility; production `cli/cmd/syllago-gui/` lives inside the cli module and needs neither.)
- **Toolchain**: `GOTOOLCHAIN=auto` cleanly downloads go1.26 despite the broken system Go (1.23.4); no blocker.
- **Build deps** (needed on build machines + CI Linux runners, NOT end users): `pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`.
- **New finding — GTK4 is now the Wails v3 default on Linux**: alpha2.117 builds against `gtk4 + webkitgtk-6.0` by default; the webkit2gtk-4.1 path this design assumes requires the `-tags gtk3` build tag (upstream labels it "legacy").
- **Decision (2026-07-11, Holden)**: stay on **GTK3 + webkit2gtk-4.1 via `-tags gtk3`** for v1. Rationale: verified on real hardware today; broader distro reach (Ubuntu 22.04 / Debian 12 have webkit2gtk-4.1 but not webkitgtk-6.0); all Linux crash mitigations in this design were researched against 4.1. The tag must be baked into the GUI Taskfile/Make targets and CI so no invocation can forget it. Revisit at Wails v3 stable — upstream momentum is on GTK4, so plan for an eventual migration.

---

## Next Steps

Ready for implementation planning with the `Plan` skill (or `/ship` for tracked feature work): slice v1 into vertical slices — suggested first slice is the walking skeleton (Wails shell + one `guiapi` service + Library page rendering real catalog data + fsnotify push), which retires every architectural risk in one pass.

## Sequencing and Follow-on Work Items (added 2026-07-17)

- **Prerequisites before GUI implementation starts** (decided 2026-07-17): `syllago-l44f3` (ADR-0020 Phase 1b hook-adapter routing) and `syllago-5us6u` (Install Modal) must land first; `syllago-cxrc1` (installer → ACIF install-entry-points matrix migration) blocks `syllago-5us6u`, so the effective chain is cxrc1 → 5us6u → GUI, with l44f3 in parallel. Dependency edges recorded in bd on `syllago-mol-1qm`.
- **GUI/TUI parity process** (`syllago-mol-poa.1`, follows GUI v1): once the GUI exists, syllago has two interactive surfaces exposing overlapping operations (library management, install/add wizards, trust flows, loadouts). A defined process must keep relevant/applicable functions in parity — candidate shape: a parity matrix of operations × surfaces with intentional gaps marked, a definition-of-done rule that shared-behavior changes update all applicable surfaces or file a parity bead, and an enforcement mechanism (advisory hook / PR checklist / drift test). Business logic lives in the shared service layer, so parity pressure is at the UI/interaction level.
