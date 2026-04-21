# MOAT TUI Trust Surfacing Spec — Expert Panel Findings (Synthesis)

**Date:** 2026-04-20
**Bead:** syllago-nyhgy (spec + expert panel review)
**Reviewed spec:** `docs/plans/2026-04-20-moat-tui-trust-surfacing-spec.md`
**Panel:** TUI UX + Accessibility, Go/BubbleTea, Security, Design System (all parallel, general-purpose agents)

## Scorecard

| Expert         | MUST-FIX | SHOULD-FIX |
|----------------|---------:|-----------:|
| TUI UX + A11y  |        8 |         12 |
| Go/BubbleTea   |        7 |         13 |
| Security       |        3 |         12 |
| Design System  |        7 |         11 |
| **Raw total**  |   **25** |     **48** |
| **Deduplicated** | **~15** | **~30** |

Many findings overlap. This doc consolidates by theme, flags conflicts, and prioritizes so Holden can make a single scope decision before the spec is fixed.

---

## Conflicts to resolve before applying fixes

| # | Conflict | Experts | Recommendation |
|---|----------|---------|----------------|
| C1 | Row glyph for Recalled: keep `✗` (AD-7 collapse) vs switch to `[R]` (color-blind shape distinction) | UX says change; Design says AD-7 preserves `✗` | **Change to `[R]`.** UX wins — shape distinction is load-bearing for color-blind users, and `[R]` matches the established bracket convention (`[P]`, `[1]`/`[2]`). AD-7's three-state collapse rule is about which *states* map to badges, not the specific glyph used for Recalled. Update AD-7 reference in spec to note glyph change. |
| C2 | `AttestationHashMismatch` surfacing: surface in drill-down, or remove field entirely | Design says surface; Go says remove (YAGNI); Security says document coherence | **Remove from `ContentItem`.** Tier is already downgraded authoritatively. Carrying a second display-only signal invites drift. If future drill-down wants to say "tier downgraded because of hash mismatch," derive from tier + a computed-on-render flag at that time. |
| C3 | `MarkPublisherConfirmed` scope: session-only vs persistent | Security asks; spec silent | **Session-only.** Persistence would silently bypass future revocation-reason escalations (`deprecated` → `malicious`). Document and add session-scoped cache keyed on `(content_hash, reason)`. |
| C4 | Nested `MOATState` struct vs 8 flat fields on `ContentItem` | Go prefers nested; others silent | **Keep flat for this phase.** `ContentItem` is already 22 fields; 8 more is noisy but consistent. Refactor into `MOATState` is a separate bead once MOAT field count stabilizes. Not a blocker. |

---

## MUST-FIX (blocks implementation)

### Correctness / compile

1. **Import cycle: producer helper cannot live in `catalog` package.**
   - **Source:** Go
   - **Spec location:** §6 "new file `cli/internal/catalog/moat_enrich.go` — imports both `catalog` and `moat`"
   - **Why it blocks:** `moat` already imports `catalog` (see `moat/enrich.go:23`). Catalog-importing-moat breaks the direction and will fail to compile.
   - **Fix:** Put `EnrichFromMOATManifests` in the `moat` package (e.g., `cli/internal/moat/producer.go`) alongside `EnrichCatalog`. Call sites become `moat.EnrichFromMOATManifests(...)`. Update §6 and §5.

2. **Modal message contract: drop `Confirmed()` getter.**
   - **Source:** Go
   - **Spec location:** §10 "`func (m *PublisherWarnModel) Confirmed() bool`"
   - **Why it blocks:** Mixing state-getter with message-emission invites stale reads and ambiguous routing. Messages aren't addressed in BubbleTea — every Update sees every msg.
   - **Fix:** Remove `Confirmed()`. Define message types in `modals/` or `tuimsg/` package. Name which sub-model consumes the message (install wizard).

3. **Update() routing when modal is active.**
   - **Source:** Go
   - **Spec location:** §10 — silent on routing
   - **Why it blocks:** Reconciles `tui-modals.md` ("modal consumes ALL input") with `tui-elm.md` #5 ("always propagate to active sub-models"). Undefined routing → keys leak to library/install wizard mid-modal.
   - **Fix:** Add §10 "Message routing" subsection:
     - Key + mouse events during modal activity → modal only.
     - `WindowSizeMsg` → modal AND background sub-models.
     - `tickMsg`/`toastExpireMsg`/async results propagate normally.

4. **Catalog mutation concurrency during rescan.**
   - **Source:** Go
   - **Spec location:** §6 (not addressed)
   - **Why it blocks:** `EnrichFromMOATManifests` must run *inside* the rescan Cmd on a freshly-built catalog, not mutate `App.catalog`. Violates `tui-elm.md` ("no state mutation outside Update()").
   - **Fix:** §6 sub-note: pipeline is `Cmd = scan + enrich → catalogReadyMsg{cat}` → `Update()` swaps atomically. Never mutate live catalog.

5. **`RecallIssuer` registry-source uses wrong `Manifest` field.**
   - **Source:** Go
   - **Spec location:** §4, §5 "registry-source: Manifest.Name (the registry operator)"
   - **Why it blocks:** `Manifest.Name` is the registry *handle*, not operator. `Manifest.Operator` is the human-readable identity field. Using `Name` is both incorrect and misleading.
   - **Fix:** Use `m.Operator` for registry-source. Fallback to `m.RegistrySigningProfile.Subject` if `Operator` empty. Sanitize (see MUST-FIX #6).

### Security

6. **Terminal injection: publisher-controlled strings rendered verbatim.**
   - **Source:** Security (MUST-FIX), also raised by UX + Go + Design
   - **Spec location:** §4, §8, §13 — listed as "open question"
   - **Why it blocks:** Trojan Source (CVE-2021-42574) applied to a trust UI. `RecallReason`, `RecallIssuer`, `RecallDetailsURL`, `item.Name`, `DisplayName`, registry `m.Name` all flow into render. ANSI escapes can clear terminal, rewrite title, bidi overrides can invert text meaning. Worst in revocation banner — user's only danger signal.
   - **Fix:** Normative requirement in §4/§5 (not §13):
     - Add `catalog.SanitizeForDisplay(s) string` (or reuse/extend `tui.sanitizeLine`).
     - Strip: C0 (`\x00-\x08`, `\x0a-\x1f` except tab), DEL (`\x7f`), C1 (`\x80-\x9f`), bidi overrides (U+202A–U+202E, U+2066–U+2069), line separators (U+2028, U+2029), ANSI escape sequences.
     - Apply at enrich boundary so sanitized values live on `ContentItem`.
     - Add golden test asserting ANSI-laden fields render as visible text.

7. **Staleness warn-band drifts from fail-closed.**
   - **Source:** Security
   - **Spec location:** §6 "StalenessWarn (24h – 7d) → enrich but mark staleness state ... This phase does not render warn state."
   - **Why it blocks:** Spec claims fail-closed but the warn band enriches as Verified + TUI doesn't surface it → stale-Verified badge attack window. A 6-day-old cache displays as green-checkmark DUAL-ATTESTED with no visible staleness signal. This is the exact window MOAT revocations are designed to close.
   - **Fix:** Pick one:
     - **(a, recommended)** Drop the warn band. Anything > 24h → `StalenessStale` → no enrichment → `TrustTier=Unknown`.
     - **(b)** Render visible degraded badge in this phase for warn-band items.
   - Staleness calc must use most conservative of: `cache_mtime`, `Manifest.UpdatedAt`, `Manifest.Expires`. A manifest whose author-stated `Expires` is past is stale regardless of mtime.

8. **Cache path traversal: registry name unsanitized.**
   - **Source:** Security
   - **Spec location:** §6 cache location uses `<registry-name>` literally
   - **Why it blocks:** Registry names could contain `../../etc/passwd`, Windows reserved names, etc. User-configured via imported loadouts or future auto-discovery.
   - **Fix:**
     - Enforce regex `^[a-z0-9][a-z0-9._-]{0,63}$` at config-load AND cache-path construction (defense in depth).
     - Verify resulting path stays under `cacheDir` via `filepath.Rel`.
     - Reject or deterministically rename on fail.
     - Test: configure registry `../escape`, assert rejection or containment.

9. **Cache layout must commit, not "TBD".**
   - **Source:** Go, Security
   - **Spec location:** §6 "Layout TBD in expert review (§13)"
   - **Why it blocks:** Every consumer hardcodes these paths. Can't implement with layout open.
   - **Fix:** Commit to `filepath.Join(cacheDir, "moat", "registries", <sanitized-name>, {"manifest.json","signature.bundle"})`. Move §13 question to "Confirmed."

### UX / accessibility

10. **Color-only differentiation for `✓` vs `✗` fails color-blind users.**
    - **Source:** UX (resolves conflict C1)
    - **Spec location:** §7, §13
    - **Why it blocks:** `✓` and `✗` differ only in thin diagonal strokes + color; deuteranopes can't distinguish. `tui/CLAUDE.md` separates color from meaning.
    - **Fix:** Verified → `✓`. Recalled → `[R]` (brackets, shape-distinct from `✓`). Matches `[P]`/`[1]` conventions. See also conflict C1 resolution.

11. **Narrow-terminal (60x20) behavior "TBD" blocks row layout + goldens.**
    - **Source:** UX, Go, Design
    - **Spec location:** §7 "TBD in expert review (§13)"
    - **Why it blocks:** 60x20 is declared minimum + golden target. Implementer's first guess becomes de facto spec.
    - **Fix:** Explicit precedence: trust glyph > name > `[P]` > registry column. Trust glyph never drops (it's the safety signal). Recalled items always render their `[R]` at every supported size. `[P]` drops first when space tight.

12. **Metapanel 5-line layout not validated at 60x20.**
    - **Source:** UX
    - **Spec location:** §8
    - **Why it blocks:** Adding 2 lines to a 3-line panel at 20 rows total leaves ~6 rows for content below. Consumes ~45% of content pane on Recalled items.
    - **Fix:** Collapse rule at narrow heights: Line 4 (Trust) + Line 5 (banner) merge into single line `RECALLED (pub) — <reason>`. Verified items hide Line 4 on narrow terminals (row badge already shows `✓`). Layout diagram in §8 for each state.

13. **Line 4 chip pattern mismatch.**
    - **Source:** Design
    - **Spec location:** §8 example `"Trust:  ✓  Verified ..."`
    - **Why it blocks:** Lines 1-2 use `chip(key, val, width)` helper. Line 4 breaks the pattern.
    - **Fix:** Restructure as `chip("Trust", glyph+" "+description, 60) + chip("Visibility", "Private", 15)`. Glyph becomes part of value, not separator.

14. **Row-badge 2-char prefix breaks existing alignment.**
    - **Source:** Design
    - **Spec location:** §7 "Fixed 2-char prefix"
    - **Why it blocks:** No gap between glyphs and name; overflows name column without budget adjustment.
    - **Fix:** 3-char prefix: `[trust][P/-][space]`. Explicitly reduce name column max-width by 3 chars. Audit library.go + content tab renderers.

15. **`🔒` emoji in §8 example violates no-emoji rule.**
    - **Source:** Design
    - **Spec location:** §8 Line 4 example `"🔒 Private"`
    - **Why it blocks:** Internal contradiction with §9's `[P]` proposal and project rule.
    - **Fix:** Replace `🔒 Private` with `[P] Private`.

16. **`┃` (U+2503) heavy vertical: monospace alignment risk.**
    - **Source:** Design (MUST-FIX), UX (SHOULD-FIX)
    - **Spec location:** §8 Line 5 "left border `┃`"
    - **Why it blocks:** Heavy box-drawing renders at 1.5× in some terminal fonts (Windows Terminal Cascadia Mono, iTerm2 ligature fonts). Mixing heavy + light (`│`) in same panel breaks golden alignment.
    - **Fix:** Use `│` (U+2502) with `dangerColor` foreground. If emphasis needed, add `Bold(true)` on whole banner.

17. **Publisher-warn amber `!` + red `RECALLED` visual collision.**
    - **Source:** UX (resolves ambiguity from §13)
    - **Spec location:** §8 Line 5, §3 table
    - **Why it blocks:** Amber `!` prefixing red RECALLED produces two-color glyph cluster. Cognitive load + color-blind users see only "RECALLED" + unexplained prefix.
    - **Fix:** Replace amber `!` prefix with text suffix:
      - Registry-source: `RECALLED (registry) — <reason>. Issued by <issuer>.`
      - Publisher-source: `RECALLED (publisher) — <reason>. Issued by <issuer>.`
    - Text is self-describing, color-independent.

18. **URL truncation: middle-ellipsis leaves truncated URL unusable.**
    - **Source:** UX, Design
    - **Spec location:** §8 Line 5, §11
    - **Why it blocks:** Truncated URL can't be copied; spec says "full URL available in future modal" — accessibility dead-end in the exact feature designed to surface it.
    - **Fix:** Land a details modal in Phase 2c OR add a zone-marked `[y] Yank URL to clipboard` action (with sanitization per MUST-FIX #6). "Future modal" is not acceptable.

19. **Publisher-warn modal missing title/orientation line.**
    - **Source:** UX
    - **Spec location:** §10 Modal structure
    - **Why it blocks:** User mid-install sees dialog with URL + Confirm, no headline. May tab-through assuming next wizard step.
    - **Fix:** Mandate title line `"Publisher revoked this content"` in `warningColor` as first rendered line. Second line restates action: `"Installing <item> from <registry> will proceed despite publisher revocation."`

20. **HardBlock path has no keyboard-accessible exit.**
    - **Source:** UX
    - **Spec location:** §10 "non-modal error banner"
    - **Why it blocks:** Keyboard-only users stuck; screen reader users have no orientation.
    - **Fix:** Specify banner position, persistence, exit contract:
      - Esc aborts install, returns to tab wizard launched from.
      - Enter acknowledges + aborts.
      - Banner text includes `[esc] Abort install` visible help.
      - Zone-mark banner's dismiss affordance.

21. **Keyboard focus cycling bindings unspecified.**
    - **Source:** UX
    - **Spec location:** §10 `focus int // 0 = Cancel, 1 = Confirm`
    - **Why it blocks:** Implementation could ship Tab-only or arrow-only; golden tests won't know which key to press.
    - **Fix:** Enumerate: Tab/Shift+Tab AND Left/Right arrows cycle. Enter activates focused button. Esc ≡ Cancel. No hotkey letters (mis-press risk on dangerous surface).

### Cleanup / YAGNI

22. **Remove `Catalog.MOATStalenessByRegistry` field.**
    - **Source:** Go
    - **Spec location:** §6 proposes adding it "so the TUI can surface it in a later bead"
    - **Why it blocks:** Written-but-never-read field defers real design, invites nil-map-write panic. YAGNI.
    - **Fix:** Delete from this bead. Add when the consumer ships.

23. **Remove `AttestationHashMismatch` from `ContentItem`.**
    - **Source:** Go, Design, Security (conflict C2)
    - **Spec location:** §4
    - **Why it blocks:** Field preserved "for drill-down surfacing" but §8 never renders it. Tier is already authoritatively downgraded. Two-source-of-truth drift risk.
    - **Fix:** Delete field. If future bead wants to surface the downgrade reason, derive from `TrustTier == Signed` + a computed boolean at render time.

---

## SHOULD-FIX (quality, not blocking)

Grouped by theme. Apply what fits the current scope; defer rest to follow-up beads.

### Styling & design system

- Add named styles (`trustVerifiedStyle`, `trustRecalledStyle`, `trustWarnStyle`, `revocationBannerStyle`) to `styles.go` matching the existing pattern. Don't scatter inline `lipgloss.NewStyle()` calls.
- Add `successColor` vs `logoMint` disambiguation note in §7.
- Cite `renderModalButtons()` (tui-modals.md §4) in §10 — don't reinvent button rendering.
- `[Cancel] [Confirm]` in spec = shorthand; actual labels have no brackets. Focus stays on Cancel across resize (don't reset focus to dangerous default).
- Drill-down "Verified" vs "DUAL-ATTESTED/SIGNED" label ambiguity — decide: tier distinction carried by parenthetical (keep), OR tier is the leading label.

### Wizard-pattern compliance

- Install-wizard integration adds a new step `stepPublisherWarn`. Add `validateStep()` entry-precondition (`gateDecision == DecisionPublisherWarn`). Add forward + Esc/back tests to `wizard_invariant_test.go`. Exit on Cancel returns to review step (per Phase B learning).

### Screen reader / accessibility fallback

- Add `SYLLAGO_TUI_ACCESSIBLE=1` env var that switches glyphs to bracketed text tokens across all indicators. Defer implementation to follow-up bead but document in spec out-of-scope list.
- Every row with a badge should have a text-readable trust token accessible via screen reader capture.

### Producer wiring

- Define `ScanAndEnrich(cfg, ...) (*Catalog, error)` factory in `catalog` package (consumer calls `moat.EnrichFromMOATManifests` internally since moat imports catalog, not the other way — or put factory in a higher-level package).
- Enumerate ALL call sites: `cmd/syllago/root.go`, `tui/app.go#rescanCatalog`, individual CLI commands. Migrate each.
- Document: `EnrichFromMOATManifests` must be called inside a `tea.Cmd`, never from `Update()` or `View()`. Cross-reference `tui-elm.md` #2.
- Error handling: per-registry parse/read failures append to `cat.Warnings`, continue, never return error. Return error only on programmer errors (nil catalog/cfg) OR make infallible like `EnrichCatalog`.

### Modal lifecycle

- App holds `publisherWarn *modals.PublisherWarnModel`; non-nil means active. On dismiss, App sets `nil`. Document in §10.
- Modal at width < 56: render at `width - 2` fits-to-screen. Test: resize to 50x20 with modal open.

### Signature bundle verification

- Either verify manifest against `signature.bundle` on every enrich-time read (CPU cost but correct) OR document threat model ("we trust user's filesystem under cacheDir").
- If skipping re-verification, store a sync-time verification receipt the enrich path checks against.
- Add edge-case rows: bundle missing, bundle corrupt, bundle verification fails.

### Publisher identity display

- Publisher-source `RecallIssuer` from `SigningProfile.Subject` is a URL. Prefix display: `signed-by: <subject>` so users know it's a signing identity not a human name.
- Truncate long URLs to `OWNER/REPO` in banner; full URL in drill-down (blocked by MUST-FIX #18 details modal).
- Replace fallback `"publisher"` with unambiguous `"(publisher — identity not provided)"` to avoid collision with a malicious `Subject: "publisher"`.

### Publisher-warn state hygiene

- `MarkPublisherConfirmed` session-scoped (resolves C3). Key on `(content_hash, reason)`. Reason change invalidates confirmation.
- Bulk install: one aggregated modal listing all publisher-source revocations, one Confirm for all. Prevents modal spam + reflexive click-through.
- `PreInstallCheck` runs at install commit, not wizard-entry. Fresh gate decision drives modal.
- Audit log: each `Confirm` writes to `audit` package with item name + reason.

### Data handling

- Document zero-value collision: `TrustTier=Unknown` conflates "not MOAT-sourced" with "MOAT-sourced but stale cache." Both render identically in TUI; distinguishing is out-of-scope.
- Private-repo label: change `[P] Private` to `[P] Publisher-declared private` or similar — signals "claim not proof."
- Empty `RecallDetailsURL`: banner conditionally omits the URL segment (avoid trailing space/double-period). Golden test with empty URL.
- Copy-to-clipboard of `details_url`: either sanitize bytes + reject URLs with `\n`/`\r`/shell metacharacters, OR explicitly disallow programmatic access. Make decision testable.

### Test coverage

- Trim golden files: 3 sizes × 11 variants = 33 files is high maintenance. Keep 80x30 canonical for variant tests; use narrow/wide only on layout-sensitive components (full-app Library). Target 11–13 goldens.
- Add security tests: ANSI-laden fields, path-traversal registry names, stale-cache rendering, bundle-missing, bundle-verification-fails.
- Truth table for trust states (8–10 representative permutations) covering AD-7 collapse branches + MUST-FIX #23 `AttestationHashMismatch` removal.
- Golden: Recalled + Private row (most crowded prefix combo).
- Rescan persistence test: press `R`, verify trust state survives.

### Threat model

- Add §2 Threat Model subsection: what the trust display prevents, what it doesn't (same-user compromise), invariants on `ContentItem` MOAT fields, authorized writers.

---

## Recommended spec update plan

If Holden accepts these findings, the spec update is roughly:

1. **§2 In/Out of scope** — add threat model, defer `SYLLAGO_TUI_ACCESSIBLE` + nested `MOATState` struct + in-phase-independent signature re-verification to follow-up beads (explicitly noted).
2. **§3 Design decisions table** — update row glyphs (`✗` → `[R]`), note `MarkPublisherConfirmed` is session-scoped, add banner-text source notation `(registry)`/`(publisher)`.
3. **§4 ContentItem fields** — remove `AttestationHashMismatch`. Remove `Catalog.MOATStalenessByRegistry`. Note zero-value collision. Add normative sanitization requirement.
4. **§5 EnrichCatalog extension** — correct `RecallIssuer` registry-source to `m.Operator`. Add `SanitizeForDisplay` at enrich boundary. Define `moatTierToCatalogTier` explicitly. Note package: `moat`, not `catalog`.
5. **§6 Producer wiring** — producer lives in `moat` package. Commit to cache layout. Registry-name regex + path containment check. Staleness calc uses most-conservative of 3 sources. Drop warn band (or spec visible degraded badge). Factory `ScanAndEnrich` + enumerated call sites. Run inside Cmd only. Error-handling contract.
6. **§7 Row badge rendering** — 3-char prefix. Name-column budget reduction. Glyph `[R]` for Recalled. Narrow-terminal precedence rule.
7. **§8 Metapanel drill-down** — `│` not `┃`. Chip pattern for Line 4. Remove `🔒`. Layout diagram per state. Narrow-height collapse rule. Banner text `(registry)`/`(publisher)`. Conditional segments for empty fields. Named style references.
8. **§9 Private-repo glyph** — committed `[P]`. "Publisher-declared private" label.
9. **§10 Publisher-warn modal** — pointer receiver (closed). Drop `Confirmed()`. Message types in `modals/` or `tuimsg/`. Message routing subsection. Title line mandate. Key bindings enumerated. Width-fallback when < 56. Batch-aggregation rule. Fresh gate at commit. Audit log. `stepPublisherWarn` wizard integration. `renderModalButtons()` citation. HardBlock banner spec.
10. **§11 Edge cases** — rows for: missing `signature.bundle`, bundle verification fail, stale-then-sync race, narrow terminal (60 cols, 50 cols).
11. **§12 Testing strategy** — trim goldens. Truth table. Security test cases. Rescan persistence. Sanitization golden.
12. **§13 Open questions** — collapses to: (a) details modal vs clipboard (pick one), (b) bundle re-verification vs sync-time receipt. All other items closed above.

---

## Ready for human review

Holden, decide:

- **A. Scope.** Apply all MUST-FIX + selected SHOULD-FIX? Or MUST-FIX only, defer SHOULD-FIX to follow-up beads? (Recommend: MUST-FIX + the wizard-pattern + producer-wiring SHOULD-FIXes, since those are the integration-critical ones.)
- **B. Details modal vs clipboard (MUST-FIX #18).** Both are reasonable. Details modal is heavier but reusable; clipboard is simpler but terminal-dependent.
- **C. Staleness warn band (MUST-FIX #7).** Drop it (recommended — simpler, safer), or add visible degraded badge in this phase?
- **D. Conflict C1.** Agree with `[R]` row glyph over `✗`? This changes AD-7 prose slightly but fixes accessibility.

Once A–D are decided, I'll apply the spec updates inline and close `syllago-nyhgy`, unlocking `syllago-lqas0` (plumbing + producer wiring).
