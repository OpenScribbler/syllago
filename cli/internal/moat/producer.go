package moat

// MOAT catalog-enrichment producer (ADR 0007 Phase 2c, bead syllago-lqas0).
//
// This file is the PRODUCER side of the enrichment pipeline: it reads cached
// MOAT manifests off the filesystem, feeds them to EnrichCatalog, and emits
// non-fatal warnings for any registry whose cache is stale, missing, or
// unparseable. There is no network I/O here — live manifest fetches flow
// through `syllago registry sync`, which is the only component authorized
// to cryptographically verify a manifest.
//
// Trust boundary: enrich-time re-verifies the cached manifest against its
// cached bundle once per (file-metadata) tuple per process, closing the
// same-user local-write gap between syncs (ADR 0007 Addendum 1, bead
// syllago-dwjcy). The first enrich call in a fresh `syllago` process runs
// full sigstore verification; subsequent calls hit a process-local memo
// keyed on (manifest mtime+size, bundle mtime+size) and skip the crypto
// work. File changes flip the key automatically. See
// enrich_verify_cache.go for the cache implementation and cache-scope
// rationale.
//
// Fail-closed posture (G-9): any condition that could produce incorrect or
// outdated trust state results in "no badge" for the affected registry's
// items, never a fabricated or optimistic one. Concretely:
//   - Missing cache file → skip enrich + warning.
//   - Corrupt cache file → skip enrich + warning.
//   - StalenessStale or StalenessExpired → skip enrich + warning.
//
// Skipped registries leave their items at TrustTier=Unknown, which
// collapses to TrustBadgeNone per AD-7 Panel C9. The operator-facing
// `syllago trust-status` CLI surfaces the underlying reason; the TUI only
// shows the collapsed signal.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// manifestCacheDirName and manifestFileName place MOAT manifests in a
// dedicated tree under cacheDir. A separate "moat" namespace keeps these
// artifacts distinct from the git-backed registry clones at the top-level
// registry cache (`~/.syllago/registries/<name>/`) — those are working-tree
// checkouts the user browses, while `moat/registries/<name>/` is a private
// cache of signed artifacts.
const (
	manifestCacheDirName = "moat"
	manifestCacheSubDir  = "registries"
	manifestFileName     = "manifest.json"
	bundleFileName       = "signature.bundle"
)

// EnrichFromMOATManifests iterates MOAT-type registries in cfg, loads each
// registry's cached manifest, checks staleness via CheckRegistry (which
// consults lockfile fetched_at + manifest Expires), and calls EnrichCatalog
// on Fresh. Stale, Expired, missing cache, or unparseable cache all result
// in a warning appended to cat.Warnings and enrichment skipped for that
// registry — items stay TrustTier=Unknown (fail-closed per G-9).
//
// Per-registry read/parse failures never abort the whole producer; the
// next registry is processed on its own merits. The only way this returns
// an error is programmer error (nil cat or nil cfg) — a caller passing
// those is broken and we surface the bug rather than silently producing a
// half-enriched catalog.
//
// MUST be called inside a tea.Cmd, never from Update() or View()
// (BubbleTea Elm rule: no I/O in the update/view path). Enforced by
// convention + `.claude/rules/tui-elm.md` rule #3.
//
// Cache layout (committed per spec §6):
//   - <cacheDir>/moat/registries/<name>/manifest.json
//   - <cacheDir>/moat/registries/<name>/signature.bundle
//
// The registry-name path segment is re-validated here (defense in depth)
// even though config-load already enforces catalog.IsValidRegistryName,
// and a filepath.Rel check confirms the resolved path does not escape
// cacheDir. A registry whose name fails this check contributes a warning
// and is skipped — never a panic and never a read from outside cacheDir.
//
// A missing signature.bundle is ALSO a skip condition: sync-time
// verification requires both files to land atomically, so a manifest
// without a bundle indicates either an interrupted sync or manual
// tampering. Either way, trust decisions against it are unsafe.
func EnrichFromMOATManifests(
	cat *catalog.Catalog,
	cfg *config.Config,
	lf *Lockfile,
	cacheDir string,
	now time.Time,
) error {
	if cat == nil {
		return fmt.Errorf("EnrichFromMOATManifests: catalog is nil")
	}
	if cfg == nil {
		return fmt.Errorf("EnrichFromMOATManifests: config is nil")
	}

	// Short-circuit when the bundled trusted root is unusable: every
	// per-registry verify would hit the same failure, and rendering N
	// identical warnings per rescan drowns the signal. One warning at the
	// top of the loop is louder and correct.
	trInfo := BundledTrustedRoot(now)
	switch trInfo.Status {
	case TrustedRootStatusExpired, TrustedRootStatusMissing, TrustedRootStatusCorrupt:
		cat.Warnings = append(cat.Warnings,
			fmt.Sprintf("MOAT producer: %s; trust decisions disabled for all MOAT registries", StalenessMessage(trInfo)))
		return nil
	}

	// Pre-resolve the absolute cacheDir once so the per-registry
	// filepath.Rel escape check is stable across symlinks. An unreachable
	// cacheDir (non-existent, unreadable) produces a single warning and
	// short-circuits — every subsequent registry would hit the same
	// failure, and we don't want N identical warnings in one rescan.
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		cat.Warnings = append(cat.Warnings,
			fmt.Sprintf("MOAT producer: cannot resolve cache directory %q: %v; trust decisions disabled", cacheDir, err))
		return nil
	}

	for i := range cfg.Registries {
		reg := &cfg.Registries[i]
		if !reg.IsMOAT() {
			continue
		}

		if !catalog.IsValidRegistryName(reg.Name) {
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("MOAT producer: registry %q has invalid name; skipping", reg.Name))
			continue
		}

		manifestPath, bundlePath, err := manifestCachePathsFor(absCache, reg.Name)
		if err != nil {
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("MOAT producer: registry %q cache path rejected: %v", reg.Name, err))
			continue
		}

		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				cat.Warnings = append(cat.Warnings,
					fmt.Sprintf("MOAT cache missing for registry %q; trust decisions disabled (run `syllago registry sync`)", reg.Name))
			} else {
				cat.Warnings = append(cat.Warnings,
					fmt.Sprintf("MOAT cache unreadable for registry %q: %v; trust decisions disabled", reg.Name, err))
			}
			continue
		}

		// signature.bundle presence is a gating check (its contents are
		// verified at sync time, not here). A manifest without a bundle
		// means an interrupted or tampered sync — skip enrichment and
		// wait for the next clean sync to repopulate.
		if _, err := os.Stat(bundlePath); err != nil {
			if os.IsNotExist(err) {
				cat.Warnings = append(cat.Warnings,
					fmt.Sprintf("MOAT cache incomplete for registry %q (missing %s); trust decisions disabled", reg.Name, bundleFileName))
			} else {
				cat.Warnings = append(cat.Warnings,
					fmt.Sprintf("MOAT cache bundle unreadable for registry %q: %v; trust decisions disabled", reg.Name, err))
			}
			continue
		}

		m, err := ParseManifest(manifestBytes)
		if err != nil {
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("MOAT cache unparseable for registry %q: %v; trust decisions disabled", reg.Name, err))
			continue
		}

		// Enrich-time verification (ADR 0007 Addendum 1). Re-verify the
		// cached manifest+bundle against the pinned signing profile,
		// memoized per process to amortize the crypto cost across
		// rescans. Unpinned profiles (SigningProfile nil or zero) cannot
		// be verified safely at enrich time — no user is present to
		// approve a TOFU-captured identity — so we fail-closed with
		// MOAT_IDENTITY_UNPINNED. The sync-time path does TOFU self-match
		// because the user is present to accept; enrich-time does not.
		pinned := pinnedProfileForEnrich(reg)
		if pinned == nil {
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("%s: registry %q has no pinned signing profile; run `syllago registry approve %s`", CodeIdentityUnpinned, reg.Name, reg.Name))
			continue
		}
		if _, err := enrichVerifyFn(manifestPath, bundlePath, pinned, trInfo.Bytes); err != nil {
			code := CodeInvalid
			var ve *VerifyError
			if errors.As(err, &ve) && ve.Code != "" {
				code = ve.Code
			}
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("%s: MOAT manifest verification failed for registry %q: %v; trust decisions disabled", code, reg.Name, err))
			continue
		}

		status := CheckRegistry(lf, reg.ManifestURI, m, now)
		switch status {
		case StalenessFresh:
			EnrichCatalog(cat, reg.Name, m)
		case StalenessStale, StalenessExpired:
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("MOAT cache %s for registry %q; trust decisions disabled", status, reg.Name))
		default:
			cat.Warnings = append(cat.Warnings,
				fmt.Sprintf("MOAT cache returned unknown staleness status for registry %q; trust decisions disabled", reg.Name))
		}

		// Populate registry-level trust aggregate regardless of staleness so
		// the TUI can render a card glyph and preview panel for every MOAT
		// registry. Stale/Expired downgrade Tier to Unsigned (trust claim is
		// not currently valid) but keep the rest of the manifest metadata so
		// users can still see the registry identity.
		attachRegistryTrust(cat, reg, m, lf, status)
	}

	return nil
}

// attachRegistryTrust records the registry-level aggregate on the catalog.
// Counts iterate cat.Items post-EnrichCatalog so verified/recalled/private
// reflect the same state the per-row glyphs will show. A Stale or Expired
// registry gets Tier=Unsigned on the assumption that "cache says Signed but
// the claim is too old to trust" is a Unsigned situation for UI purposes —
// the Staleness string carries the nuance for the inspector.
func attachRegistryTrust(
	cat *catalog.Catalog,
	reg *config.Registry,
	m *Manifest,
	lf *Lockfile,
	status StalenessStatus,
) {
	if cat.RegistryTrusts == nil {
		cat.RegistryTrusts = make(map[string]*catalog.RegistryTrust)
	}

	tier := catalog.TrustTierSigned
	if status != StalenessFresh {
		tier = catalog.TrustTierUnsigned
	}

	rt := &catalog.RegistryTrust{
		Name:        reg.Name,
		Tier:        tier,
		Issuer:      m.RegistrySigningProfile.Issuer,
		Subject:     m.RegistrySigningProfile.Subject,
		Operator:    m.Operator,
		ManifestURI: reg.ManifestURI,
		Staleness:   status.String(),
	}
	if lf != nil {
		if lockState, ok := lf.Registries[reg.ManifestURI]; ok {
			rt.FetchedAt = lockState.FetchedAt
		}
	}

	for _, item := range cat.Items {
		if item.Registry != reg.Name {
			continue
		}
		rt.TotalItems++
		switch item.TrustTier {
		case catalog.TrustTierSigned, catalog.TrustTierDualAttested:
			rt.VerifiedItems++
		}
		if item.Recalled {
			rt.RecalledItems++
		}
		if item.PrivateRepo {
			rt.PrivateItems++
		}
	}

	cat.RegistryTrusts[reg.Name] = rt
}

// pinnedProfileForEnrich translates a config.SigningProfile to the moat
// package shape for enrich-time verification, returning nil if the
// registry has no pinned profile.
//
// Unlike pinnedProfileForVerify (sync.go), this helper has NO wire-profile
// fallback. Sync-time is interactive — the user is present to approve a
// TOFU-captured identity — so falling back to the manifest's own claimed
// profile still yields a well-typed verify run whose result is then
// surfaced for human approval. Enrich-time has no interactive escape
// hatch, so the fallback would be equivalent to self-matching the cert
// against itself: a tautological pass. The correct fail-closed behavior
// for an unpinned profile at enrich time is to skip verification entirely
// and emit MOAT_IDENTITY_UNPINNED.
func pinnedProfileForEnrich(reg *config.Registry) *SigningProfile {
	if reg == nil || reg.SigningProfile == nil || reg.SigningProfile.IsZero() {
		return nil
	}
	return &SigningProfile{
		Issuer:            reg.SigningProfile.Issuer,
		Subject:           reg.SigningProfile.Subject,
		ProfileVersion:    reg.SigningProfile.ProfileVersion,
		SubjectRegex:      reg.SigningProfile.SubjectRegex,
		IssuerRegex:       reg.SigningProfile.IssuerRegex,
		RepositoryID:      reg.SigningProfile.RepositoryID,
		RepositoryOwnerID: reg.SigningProfile.RepositoryOwnerID,
	}
}

// manifestCachePathsFor constructs (manifestPath, bundlePath) under
// <absCacheDir>/moat/registries/<name>/ and verifies the resolved path
// stays under absCacheDir. Returns an error if the registry name resolves
// outside the cache tree — a redundant check given IsValidRegistryName
// already disallows `..` segments, but retained because a future loosening
// of the validator should not silently open a traversal.
func manifestCachePathsFor(absCacheDir, name string) (string, string, error) {
	regCacheDir := filepath.Join(absCacheDir, manifestCacheDirName, manifestCacheSubDir, name)

	// filepath.Rel returns a path with no ".." only when child is under
	// parent. On Windows this would also catch drive-letter escapes.
	rel, err := filepath.Rel(absCacheDir, regCacheDir)
	if err != nil {
		return "", "", fmt.Errorf("compute rel path: %w", err)
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", "", fmt.Errorf("path escapes cache directory: %s", regCacheDir)
	}

	return filepath.Join(regCacheDir, manifestFileName),
		filepath.Join(regCacheDir, bundleFileName),
		nil
}

// ScanAndEnrich composes a fresh catalog scan with MOAT enrichment. This is
// the production pipeline the TUI rescan tea.Cmd and every CLI command that
// materializes a live catalog should call. Living in moat (not catalog)
// lets this function import both packages — the reverse direction would
// cycle.
//
// The returned catalog is always fresh-constructed (never a mutated
// existing catalog), so callers can atomically swap it into the model
// under lock via catalogReadyMsg. A scan error propagates up; enrichment
// errors attach to cat.Warnings but do not fail the whole pipeline.
//
// Contract mirrors catalog.ScanWithGlobalAndRegistries plus the MOAT
// inputs (lf, cacheDir, now) needed for staleness enforcement. Callers
// that previously called ScanWithGlobalAndRegistries directly should
// migrate to this function — a non-MOAT config passes through with no
// observable difference (EnrichFromMOATManifests is a no-op when no
// registries have Type=="moat").
func ScanAndEnrich(
	cfg *config.Config,
	root, projectRoot string,
	regSources []catalog.RegistrySource,
	lf *Lockfile,
	cacheDir string,
	now time.Time,
) (*catalog.Catalog, error) {
	cat, err := catalog.ScanWithGlobalAndRegistries(root, projectRoot, regSources)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return cat, nil
	}
	if err := EnrichFromMOATManifests(cat, cfg, lf, cacheDir, now); err != nil {
		return cat, err
	}
	return cat, nil
}
