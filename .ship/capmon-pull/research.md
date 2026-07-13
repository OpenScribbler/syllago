# Research: capmon-pull

## Q1: What Sigstore verification entry points exist in cli/internal/moat/ — which functions verify a bundle, check Fulcio signer identity, and verify Rekor inclusion; what are their signatures and inputs; and do any of them handle DSSE/in-toto attestation statements (SLSA provenance) as opposed to plain artifact signatures?

### Bundle verification

- `VerifyManifest(manifestBytes []byte, bundleBytes []byte, pinnedProfile *SigningProfile, trustedRootJSON []byte) (VerificationResult, error)` — `cli/internal/moat/manifest_verify.go:125`. Verifies a MOAT registry manifest against a Sigstore **v0.3 bundle** parsed natively by sigstore-go (`bundle.UnmarshalJSON`, `cli/internal/moat/manifest_verify.go:159-163`). Builds a verifier with `verify.NewVerifier(tr, verify.WithTransparencyLog(1), verify.WithIntegratedTimestamps(1))` (`manifest_verify.go:168-171`) and a policy of `verify.WithArtifact(bytes.NewReader(manifestBytes))` + `verify.WithCertificateIdentity(certID)` (`manifest_verify.go:194-197`). Fulcio identity is pinned via `verify.NewShortCertificateIdentity(pinnedProfile.Issuer, pinnedProfile.IssuerRegex, pinnedProfile.Subject, pinnedProfile.SubjectRegex)` (`manifest_verify.go:183-188`). GitHub numeric-ID OID binding is enforced separately by `matchNumericIDs(b *sgbundle.Bundle, pinned *SigningProfile) (bool, error)` (`manifest_verify.go:283`) and `extensionsFromBundle` (`manifest_verify.go:325`).
- `BuildBundle(rekorRaw []byte, canonicalPayload []byte) (*sgbundle.Bundle, error)` — `cli/internal/moat/bundle_builder.go:46`. Converts a raw Rekor API response into a sigstore-go bundle whose subject is a **MessageSignature over the canonical payload** (an artifact signature) with media type `application/vnd.dev.sigstore.bundle.v0.3+json` (`bundle_builder.go:32`).

### Per-item Rekor verification (no bundle on the wire)

- `VerifyAttestationItem(item AttestationItem, profile *SigningProfile, rekorRaw []byte, trustedRootJSON []byte) (VerificationResult, error)` — `cli/internal/moat/item_verify.go:102-107`. Inputs: the attestation row (content hash + Rekor coordinates), the pinned signing profile, the exact Rekor API JSON, and `trusted_root.json` bytes. Performs an 8-step chain documented at `item_verify.go:3-25`:
  1. hashedrekord v0.0.1 shape + LogIndex match (`item_verify.go:127-136`)
  2. canonical payload hash match against `body.Spec.Data.Hash.Value` (`item_verify.go:152-159`)
  3. ECDSA signature over the canonical payload via `verifySignature(cert, body, payload)` (`cli/internal/moat/signature.go:22`, called at `item_verify.go:166`)
  4. Rekor SET verification: `tlog.VerifySET(tlogEntry, rekorLogs)` (`item_verify.go:202`)
  5. Rekor inclusion proof: `tlog.VerifyInclusion(tlogEntry, sigVerifier)` (`item_verify.go:220`), with the shard-local vs. global logIndex handling documented at `item_verify.go:363-371` and `item_verify.go:405`
  6. Fulcio chain validation at integrated time: `verifyFulcioChain(cert *x509.Certificate, cas []root.CertificateAuthority, at time.Time) error` (`item_verify.go:278`)
  7. Exact-equality Fulcio identity match against the pinned profile via `extractIdentity(cert)` (`cli/internal/moat/cert.go:66`, checked at `item_verify.go:236-250`)
  8. GitHub numeric-ID OID binding (OIDs 1.3.6.1.4.1.57264.1.15 / .1.17, `item_verify.go:76-79`) via `matchNumericIDsFromCert` (`item_verify.go:299`) / `readNumericIDsFromCert` (`item_verify.go:325`)

### Fetch + identity helpers

- `FetchRekorEntry(ctx context.Context, logIndex int64) ([]byte, error)` — `cli/internal/moat/rekor_fetch.go:57`; base URL `https://rekor.sigstore.dev/api/v1/log/entries` (`rekor_fetch.go:28`), 30s client timeout (`rekor_fetch.go:34`), 10 MiB response cap (`rekor_fetch.go:40`).
- `FetchPublisherAttestation(ctx context.Context, sourceURI string) ([]byte, error)` — `cli/internal/moat/publisher_attestation.go:60`; fetches `moat-attestation.json` from `raw.githubusercontent.com/<owner>/<repo>/moat-attestation/` (`publisher_attestation.go:40,67`). `FindPublisherEntry(rawJSON []byte, contentHash string) (int64, error)` — `publisher_attestation.go:106`.
- `ExtractIdentityFromRekorRaw(rekorRaw []byte) (issuer, subject string, err error)` — `cli/internal/moat/cert.go:46` (TOFU capture path).
- Both verify entry points return `VerificationResult` with fields `SignatureValid, CertificateChainValid, RekorProofValid, IdentityMatches, NumericIDMatched, RevocationChecked` (`cli/internal/moat/manifest_verify.go:77`, populated at `item_verify.go:264-271`).
- Trusted root loading: `root.NewTrustedRootFromJSON(trustedRootJSON)` from bundled `cli/internal/moat/trusted_root.json` via `trusted_root_loader.go` / `trusted_root_path.go` (files listed in `cli/internal/moat/`).

### DSSE / in-toto / SLSA provenance

- **No DSSE or in-toto handling exists.** `grep -n -i "dsse|in-toto|intoto|slsa"` over `cli/internal/moat/*.go` matches only `cli/internal/moat/rekor_test.go:65-67`, a test asserting that a Rekor entry with `kind=intoto` is **rejected**.
- `decodeHashedRekordBody` hard-rejects any kind other than `hashedrekord` and any apiVersion other than `0.0.1` (`cli/internal/moat/rekor.go:96-101`).
- All verification is plain artifact-signature verification: `VerifyManifest` uses `verify.WithArtifact` over raw manifest bytes (`manifest_verify.go:195`); `BuildBundle` produces a MessageSignature bundle (`bundle_builder.go:36-41`). There is no code path that parses an in-toto Statement, DSSE envelope, or SLSA provenance predicate.

## Q2: What does cli/internal/provider/coverage.go define — the CheckCoverage and CoverageDrift shapes, each assertion it performs, the inputs it reads, and what callers or test files reference it anywhere in the module?

### Shapes

- `CoverageDrift{Provider string; ContentType catalog.ContentType; Assertion string; Message string}` with a `String()` renderer — `cli/internal/provider/coverage.go:15-24`.
- `CheckCoverage(repoRoot string) ([]CoverageDrift, error)` — `coverage.go:108`. `repoRoot` must contain `docs/` and `cli/`; `FindRepoRoot(start string) string` walks up looking for `cli/go.mod` + `docs/` (`coverage.go:255-269`).
- Assertion-name constants: `AssertionGoVsSourceManifest` ("go-vs-source-manifest"), `AssertionGoVsFormatYAML` ("go-vs-format-yaml"), `AssertionConfigLocationsVsGo` ("configlocations-vs-supportstype"), `AssertionInstallDirVsSupportsGo` ("installdir-vs-supportstype") — `coverage.go:28-33`.
- `CoverageContentTypes` = Rules, Skills, Agents, Commands, Hooks, MCP; Loadouts intentionally excluded — `coverage.go:38-45`.

### Assertions performed (per provider × content type, skipping providers with nil SupportsType, `coverage.go:125-127`)

1. **Assertion 3** — `ConfigLocations[ct]` set ⇒ `SupportsType(ct) == true` (`coverage.go:133-140`).
2. **Assertion 4** — `InstallDir(home, ct) != ""` ⇔ `SupportsType(ct) == true`, using a fixed `/home/covtestuser` home (`coverage.go:122,143-155`).
3. **Assertion 1** — Go `SupportsType` vs. the source manifest's claim (`supported:` field, or implied true by non-empty `sources:`) (`coverage.go:158-169`; claim extraction at `coverage.go:63-72`).
4. **Assertion 2** — Go `SupportsType` vs. the format YAML's `status: supported|unsupported` (empty status = not asserted) (`coverage.go:172-183`; claim extraction at `coverage.go:88-99`).

### Inputs read

- `docs/provider-sources/*.yaml` (skipping `_template.yaml`), keyed by `slug` — `coverage.go:113,192-220`.
- `docs/provider-formats/*.yaml`, keyed by `provider` — `coverage.go:117,224-249`.
- In-memory `AllProviders` (Go-side `SupportsType`, `ConfigLocations`, `InstallDir`) — `coverage.go:125-155`.
- It does **not** read `docs/provider-capabilities/` at all.

### Callers / references

- Only test callers, all in `cli/internal/provider/invariant_test.go`: `TestCoverageInternalGoConsistency` (`invariant_test.go:21-25`, assertions 3+4 always enforced), `TestCoverageNoDrift` (`invariant_test.go:67-75`, full four-assertion gate skipped unless `SYLLAGO_COVERAGE_STRICT=1`, `invariant_test.go:68-70`), `TestFindRepoRoot` (`invariant_test.go:98`).
- No production (non-test) code calls `CheckCoverage`; a module-wide grep finds only the test file plus prose mentions in `CHANGELOG.md:172` and `releases/v0.9.0.md:50`.

## Q3: Where and how do providers declare per-content-type support in Go (SupportsType or equivalent) — what is the data shape, where is it defined per provider, and which packages consume it?

### Data shape

- Field on the `Provider` struct: `SupportsType func(ct catalog.ContentType) bool` — `cli/internal/provider/provider.go:56-57`. It is a nilable function field; every consumer nil-checks it.
- Each provider is a package-level `Provider` literal whose `SupportsType` is a `switch` over `catalog.ContentType` returning a hardcoded set, e.g. Claude Code returns true for Rules, Skills, Agents, Commands, MCP, Hooks, Loadouts (`cli/internal/provider/claude.go:73-81`); Zed returns true only for Rules and MCP (`cli/internal/provider/zed.go:52-59`).
- All 15 providers declare it: `amp.go:81`, `claude.go:73`, `cline.go:123`, `codex.go:68`, `copilot.go:73`, `crush.go:55`, `cursor.go:72`, `factory_droid.go:65`, `gemini.go:69`, `kiro.go:75`, `opencode.go:71`, `pi.go:60`, `roocode.go:84`, `windsurf.go:72`, `zed.go:52` (all in `cli/internal/provider/`).
- The aggregate registry is `var AllProviders = []Provider{...}` — `provider.go:86-102`.

### Consumers (non-test)

- `cli/internal/provider/coverage.go:126,130` (CheckCoverage)
- `cli/internal/add/add.go:530`
- `cli/internal/installer/moat_provider_install.go:60`
- `cli/internal/loadout/validate.go:28`
- `cli/internal/tui/app.go:369`, `cli/internal/tui/add_wizard.go:756`, `cli/internal/tui/actions.go:754`
- `cli/cmd/syllago/info.go:177-179`, `cli/cmd/syllago/genproviders.go:181`, `cli/cmd/syllago/compat_cmd.go:105`, `cli/cmd/syllago/sync_and_export.go:224-225`

## Q4: What is the current file inventory of docs/provider-capabilities/ (YAML files, schema.json, README, by-content-type/), and what references any of those paths?

### Inventory

- 89 top-level `*.yaml` files: 15 per-provider baseline files (`<slug>.yaml`, e.g. `claude-code.yaml`) plus per-provider-per-content-type files (`<slug>-<type>.yaml`, e.g. `claude-code-hooks.yaml`) (directory listing of `docs/provider-capabilities/`).
- `by-content-type/` with 6 generated YAML views: `agents.yaml`, `commands.yaml`, `hooks.yaml`, `mcp.yaml`, `rules.yaml`, `skills.yaml`; each headed "THIS FILE IS GENERATED. Do not edit directly. / Source: docs/provider-capabilities/*.yaml / Generated at: 2026-04-17T16:13:10Z" (`docs/provider-capabilities/by-content-type/skills.yaml:1-3`).
- `schema.json` (118 lines) — JSON Schema draft 2020-12 for `<slug>.yaml`, `schema_version` enum `["1"]` (`docs/provider-capabilities/schema.json:1-12`).
- `README.md` (64 lines) — says the directory is maintained by the external OpenScribbler/capmon pipeline (`README.md:3`) but still documents `capmon run/seed/verify/generate` CLI commands (`README.md:40-46`), a `.capmon-pause` file (`README.md:48-56`), and points schema changes at `cli/internal/capmon/capyaml/validate.go` (`README.md:64`, a path that no longer exists — see Q8).
- `compatibility-matrix.md` (103 lines, maintained by hand per `README.md:14`).
- Note: the current files are YAML, not JSON (lexicon's Capability Document term describes a per-provider JSON mirror; the on-disk state today is `schema_version: "1"` YAML).

### References to these paths

- **Go code, Makefiles, CI workflows, scripts: none.** `grep -rn "provider-capabilities"` over `cli/`, `Makefile`, `cli/Makefile`, and `.github/` returns zero matches.
- `.gitignore:46` — `!docs/provider-capabilities/` un-ignores the directory from the blanket `docs/*` ignore at `.gitignore:41`.
- Docs: `ARCHITECTURE.md:285-288` (capability data stays in syllago; capmon extracted), `CONTRIBUTING.md:164` (review step), `docs/provider-capabilities/README.md` (self), `docs/guides/adding-a-provider.md` (capmon onboarding section, lines 185-302), `docs/guides/simplification-plan.md`, and historical plan docs `docs/plans/2026-04-08-capability-monitor-pipeline-design.md`, `docs/plans/2026-04-09-...-implementation.md`, `docs/plans/2026-04-10-capmon-seeder-*.md`, `docs/plans/2026-04-21-provmon-drift-detection-design.md`.
- `commands.json` (tracked generated file, `git ls-files commands.json`) still contains `syllago capmon` command entries citing `cli/cmd/syllago/capmon_cmd.go` (`commands.json:161-229`), including descriptions referencing provider-capabilities regeneration.

## Q5: What patterns do the existing .github/workflows/ files use for cron schedules, actions/cache, creating/updating PRs from automation, GITHUB_TOKEN permissions, and action pinning?

- **Workflows present:** `ci.yml`, `claude-code-review.yml`, `claude.yml`, `codeql.yml`, `moat-trusted-root-check.yml`, `pr-policy.yml`, `release.yml`, `scorecard.yml`, `smoke-test-providers.yml`, `vouch-manage.yml` (listing of `.github/workflows/`).
- **Cron schedules:** three workflows use `schedule:` — `codeql.yml:7-8` (`'0 6 * * 1'`, weekly Monday), `scorecard.yml:4-6` (`'30 1 * * 6'`), `moat-trusted-root-check.yml:17-20` (`'0 12 * * 1'`, with an explanatory comment). `moat-trusted-root-check.yml` pairs the cron with `workflow_dispatch: {}` (`:21`) and a `concurrency: group: moat-trusted-root-check / cancel-in-progress: false` block (`:23-25`).
- **actions/cache:** no direct `actions/cache` usage anywhere. Go module caching is delegated to `actions/setup-go` with `cache-dependency-path: cli/go.sum` (`moat-trusted-root-check.yml:44-47`; same pinned setup-go across `ci.yml:43,65`, `codeql.yml:28`, `smoke-test-providers.yml:35,76`, `release.yml:26`).
- **Automation creating PRs:** none. No `peter-evans/create-pull-request`, no `gh pr create` in any workflow. The closest patterns:
  - **Single rolling issue** — `moat-trusted-root-check.yml:88-108` uses `gh issue list --search "<title> in:title" --state open` to find an existing issue, then `gh issue comment` (update) or `gh issue create` (open), with `GH_TOKEN: ${{ github.token }}` (`:91`). Dynamic content flows through files (`--body-file "${RUNNER_TEMP}/issue-body.md"`, built at `:63-86`) rather than shell interpolation, per the security-posture comment at `:9-14`.
  - **Cross-repo push** — `release.yml:190-245` clones `OpenScribbler/homebrew-tap` with an `x-access-token` URL using an Aembit-issued API key (`Aembit/get-credentials`, `release.yml:181-187`), commits as `github-actions[bot]`, and `git push`es the formula (`release.yml:237-245`).
- **Permissions blocks:** every workflow sets top-level `permissions: contents: read` (e.g. `ci.yml:13-14`, `moat-trusted-root-check.yml:27-28`) or `read-all` (`scorecard.yml:9`), then escalates per-job: `issues: write` (`moat-trusted-root-check.yml:34-36`), `contents: write` + `id-token: write` for release (`release.yml:16-18`, also gated behind `environment: release`, `release.yml:15`), `security-events: write` for scorecard/codeql (`scorecard.yml:15-19`).
- **Action pinning:** every `uses:` is pinned to a full commit SHA with a trailing version comment, e.g. `actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2`, `actions/setup-go@4a36011... # v6.4.0`, `ossf/scorecard-action@4eaacf05... # v2.4.3`, `anthropics/claude-code-action@37b464ce... # v1` (grep of `uses:` across `.github/workflows/*.yml` shows no tag-only pins).
- Checkouts consistently pass `persist-credentials: false` (`moat-trusted-root-check.yml:40-41`).

## Q6: What HTTP client code exists for fetching remote resources — conditional GET/ETag, GitHub API auth, timeouts, and httptest patterns?

### Conditional GET / ETag

- `moat.Fetcher` (`cli/internal/moat/fetch.go`) is the only conditional-GET implementation: `Fetch(ctx, url, prevETag string) (*FetchResult, error)` (`fetch.go:80`) sets `If-None-Match` when `prevETag` is non-empty (`fetch.go:91-92`), maps `304 Not Modified` to `FetchResult{NotModified: true}` echoing the ETag (`fetch.go:104-108`), and returns the server ETag on 200 (`fetch.go:130`). Rationale comment: ETag + If-None-Match cuts polling bandwidth ~99% (`fetch.go:9-11`). `DefaultFetchTimeout = 30 * time.Second` (`fetch.go:31-32`); injectable `Client *http.Client` field, nil → default with that timeout (`fetch.go:63-64,143-147`).
- ETag persistence: `moat.Sync` threads `reg.ManifestETag` into `fetcher.Fetch` (`cli/internal/moat/sync.go:220`) and returns `SyncResult.ETag` for the caller to persist into `config.Registry.ManifestETag` (`sync.go:79-82,189`). Round-trip persistence is tested in `cli/cmd/syllago/registry_sync_moat_test.go:96-154,420-447`.

### GitHub API calls and auth

- `cli/internal/updater/updater.go` calls `https://api.github.com/repos/OpenScribbler/syllago/releases/latest` (`updater.go:24`) **unauthenticated**; it sets only `User-Agent: syllago-updater` (required to avoid 403, `updater.go:69-71`) and `Accept: application/vnd.github+json`. Shared client with 15s timeout (`updater.go:31-33`).
- `moat` fetches use `rekorFetchClient` (30s timeout, `cli/internal/moat/rekor_fetch.go:34`) with a 10 MiB `io.LimitReader` cap (`rekor_fetch.go:40,82-87`); `FetchPublisherAttestation` reuses the same client against `raw.githubusercontent.com`, no auth (`cli/internal/moat/publisher_attestation.go:40,67-77`).
- **No Authorization/Bearer header is sent by any HTTP client in the module.** The only `Authorization` matches in non-test Go code are MCP config field mappings in `cli/internal/converter/mcp_codex.go:49-54`, not outbound requests. No `GITHUB_TOKEN` reads exist in Go code.

### Test patterns

- Package-level base-URL variable swapped in tests: `githubAPIURL` is a `var` "so tests can override it with an httptest server URL" (`updater.go:23-24`; swap at `updater_test.go:78-80`); moat exposes `SetRekorBaseURLForTest` / `RekorBaseURLForTest` (`rekor_fetch.go:45-50`) and `SetPublisherAttestationBaseURLForTest` (`publisher_attestation.go:44-48`).
- Direct URL injection: `Fetcher.Fetch` takes the URL as an argument, so tests just pass `srv.URL` from `httptest.NewServer` (`cli/internal/moat/fetch_test.go:19-36`). Tests assert request headers server-side (User-Agent, Accept, `fetch_test.go:20-25`), cover 304, custom UA, unexpected status, malformed body, oversized body, empty URL, and context cancellation (`fetch_test.go:59-239`).
- Other httptest users: `cli/internal/moat/rekor_fetch_test.go` (8 servers), `sync_test.go`, `publisher_attestation_test.go`, `cli/internal/telemetry/telemetry_test.go`, `cli/internal/registry/probe_test.go`, `cli/internal/moatinstall/fetch_test.go`, `cli/cmd/syllago/registry_sync_moat_test.go`, `install_moat_test.go` (grep for `httptest`).

## Q7: What precedent exists for non-user-facing maintainer tools in this module — separate main packages or cmd directories outside the syllago CLI, and how are they built and invoked?

Two patterns exist:

### Pattern A: hidden `_`-prefixed subcommands inside the main syllago binary

- `_gentelemetry` (`cli/cmd/syllago/gentelemetry.go:20-21`), `_genproviders` (`cli/cmd/syllago/genproviders.go:66-67`), `_gencapabilities` (`cli/cmd/syllago/gencapabilities.go:198-199`) — all registered on `rootCmd` (`gentelemetry.go:28`, `genproviders.go:74`, `gencapabilities.go:206`) as hidden cobra commands writing JSON to stdout.
- Invocation in CI: `release.yml:58-66` builds a throwaway binary `go build ... -o syllago-gendocs ./cmd/syllago`, runs `./syllago-gendocs _gendocs > commands.json`, `_genproviders > providers.json`, `_gentelemetry > telemetry.json`, `_gencapabilities > capabilities.json`, `_gencontentformat > content-format.json`, `_genyamlschema > syllago-yaml-schema.json`, then `rm -f syllago-gendocs`.
- Invocation via Makefile: `cli/Makefile:25-26` — `gendocs: build` → `./$(OUTPUT) _gendocs > commands.json`. The root `Makefile` has only `build/clean/build-all/test/fmt/vet/setup` delegating targets (`Makefile:3-21`).

### Pattern B: separate main packages under cli/cmd/

- `cli/cmd/syllago-sign/main.go` — build-time Ed25519 release-signing utility, "Not shipped to end users — invoked from .github/workflows/release.yml via `go run ./cmd/syllago-sign`" (`main.go:1-5`); actual invocation `go run ./cmd/syllago-sign sign checksums.txt > checksums.txt.sig` (`release.yml:107`). Has its own `main_test.go`.
- `cli/cmd/migrate_hooks/main.go` — one-shot content migration, "Usage: go run ... Dry run by default; pass --apply" (`main.go:1-8`). No Makefile target, no CI reference (grep of `Makefile`, `cli/Makefile`, `.github/workflows/` finds only the release.yml syllago-sign line).
- Neither separate main package appears in `build-all` or release artifact lists; both are `go run`/ad-hoc only.

## Q8: What residual references remain to the deleted cli/internal/capmon package or the old docs/provider-capabilities generator — in the Makefile, CI workflows, docs, or Go code — as of the current HEAD?

The package was deleted in commit `ef6fdd2b` ("Simplification: process-gate overhaul, dead-code removal, capmon extraction (Tiers 0–2 + Tier 1) (#483)", 2026-07-08), which touched 127 `cli/internal/capmon` paths (`git show ef6fdd2b --stat`). `git ls-files cli/internal/capmon` returns 0 tracked files; an empty untracked `cli/internal/capmon/` directory still exists on disk.

### Makefile / CI

- **Zero references.** `grep -rn -i capmon Makefile cli/Makefile .github/workflows/*.yml` returns nothing; no `capmon.yml` / `capmon-check.yml` workflow exists in `.github/workflows/` (directory listing).

### Go code (comments only — no imports or symbols)

- `cli/cmd/syllago/gencapabilities.go:262,273` — comments saying quality sidecars (`docs/provider-formats/<slug>.quality.json`) "are written by the capmon check pipeline (jtafb)".
- `cli/cmd/syllago/genproviders.go:83` — comment: parses format-doc YAML "without pulling in the full capmon schema".
- `cli/cmd/syllago/genproviders_test.go:457,474` and `cli/cmd/syllago/gencapabilities_test.go:943` — comments describing tests as mirrors of capmon validators.
- `cli/internal/catalog/trust.go:102` — comment listing "capmon validate" among consumers.

### Docs referencing deleted paths/commands

- `CONTRIBUTING.md:133-166` — full "capmon (capability monitor)" section: claims capmon "runs automatically on CI via `.github/workflows/capmon.yml`" (`:135`, file does not exist), documents `.capmon-pause` (`:139-143`), points at fixtures in `cli/internal/capmon/testdata/fixtures/` and `go test ./internal/capmon/` (`:150-154`), and `syllago capmon run --dry-run` / `syllago capmon generate` (`:163-166`).
- `docs/guides/adding-a-provider.md:185-302` — "Capability Monitoring (capmon) Onboarding" section referencing `capmon run/seed/validate-spec` commands, `cli/internal/capmon/recognize_<slug>.go` (`:224,246`), and `cd cli && go run ./cmd/capmon run ...` (`:302`; `cli/cmd/capmon` does not exist — `cli/cmd/` contains only `migrate_hooks`, `syllago`, `syllago-sign`).
- `docs/provider-capabilities/README.md:40-46` — table of `capmon run/seed/verify/generate` commands; `:48-56` `.capmon-pause`; `:64` references `cli/internal/capmon/capyaml/validate.go`.
- `commands.json:161-229` — tracked generated file still lists the `syllago capmon` command family with `"source": "cli/cmd/syllago/capmon_cmd.go"` (file no longer exists); not regenerated since extraction.
- `.gitignore:42` — `.capmon-cache/` entry; `.gitignore:48-49` — comment "Capmon pipeline sidecars ... written at runtime by jtafb" above `docs/provider-formats/*.quality.json`.
- Current-state (not stale) descriptions: `ARCHITECTURE.md:285-288` (capmon "lives in its own repository ... extracted from `internal/capmon` with history"), `docs/guides/simplification-audit.md:21,54-64` and `docs/guides/simplification-plan.md` (the extraction plan itself), historical `docs/plans/2026-04-*.md`, and `CHANGELOG.md`/`releases/` entries.

## Cross-cutting observations

- `cli/internal/moat/fetch.go` is registry-manifest-specific in its parse step (`FetchResult.Manifest`, `fetch.go:104-130`), and `sync.go:325` contains a comment about the alternative of "exposing a general 'fetch bytes' API" (no such general API exists today).
- `docs/provider-capabilities/by-content-type/*.yaml` carry a generation timestamp of 2026-04-17 (`by-content-type/skills.yaml:3`); the generator command (`syllago capmon generate`) no longer exists in the codebase, so these views have no in-repo regeneration path at HEAD.
- Coverage checking (`CheckCoverage`) currently compares Go against `docs/provider-sources/` and `docs/provider-formats/` only; nothing in the module compares Go `SupportsType` against `docs/provider-capabilities/` data.
- The `moat-trusted-root-check.yml` workflow is the repo's only cron+`gh` automation and uses a "find-existing-then-comment, else create" single-issue idiom (`moat-trusted-root-check.yml:95-108`).
- The repo has no `scripts/` directory; automation logic lives inline in workflow `run:` blocks or in Go (`.github/workflows/` listing, repo root listing).
- Working tree at research time: HEAD `cf047f52`, with local modifications limited to `.beads/`, `.wolf/anatomy.md`, `lexicon.md`, and untracked `.ship/capmon-pull.json`, `panel/` (`git status --short`).
