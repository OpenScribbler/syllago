// Package moat — manifest freshness / staleness enforcement.
//
// MOAT spec §Freshness Guarantee (v0.6.0, ADR 0007 G-9):
//
//   - Default staleness threshold is 72 hours. The value was 24h in pre-0.6.0
//     drafts; the 72h landed explicitly to survive a weekend (Friday 6pm →
//     Monday 9am = 63h).
//   - Staleness is computed against the client's last *successful* manifest
//     fetch, recorded per-registry in lockfile `registries[url].fetched_at`
//     (see G-7). A failed fetch MUST NOT reset this clock — callers update
//     the timestamp only after verification succeeds.
//   - Manifests MAY carry an OPTIONAL `expires` field (RFC 3339 UTC, renamed
//     from `expires_at` in v0.6.0). After that time, clients MUST NOT trust
//     the manifest regardless of fetched_at. Unlike staleness, expiry is
//     NOT recoverable by refresh alone — the registry must publish a new
//     manifest with a later `expires`.
//   - Checks run at install time, not continuously. A client that opened a
//     manifest an hour ago and is about to install now checks against
//     "now", not "when the manifest was parsed".
//
// Scope of this module: pure functions that classify (last-fetched, expires,
// now) into a StalenessStatus. The registry-sync auto-fetch behavior,
// non-interactive exit-code mapping (G-18), and jitter on the refresh
// schedule (dissent C7) land in the Phase 2 merge (syllago-dsqjz). Keeping
// the primitives pure lets the enforcement path be tested with a fake clock
// without touching the fetch machinery.
package moat

import "time"

// DefaultStalenessThreshold is the spec-normative default from
// §Freshness Guarantee. It is not configurable in this layer — the spec
// fixes it at 72h to prevent clients from loosening the window in a way
// that would undermine the trust-signal guarantee. A future spec change
// would update the constant here in lockstep.
const DefaultStalenessThreshold = 72 * time.Hour

// StalenessStatus classifies a (last-fetched, expires, now) triple against
// the 72h threshold and the manifest's optional `expires`. The three states
// have distinct enforcement meanings and MUST NOT be conflated:
//
//   - StalenessFresh: within both windows — proceed.
//   - StalenessStale: 72h threshold exceeded, but `expires` (if present) is
//     still in the future. Recoverable by a successful refresh. Interactive
//     callers SHOULD auto-fetch; non-interactive MUST exit non-zero if the
//     refresh cannot be attempted or fails (see G-18).
//   - StalenessExpired: manifest.expires is set and now >= expires. NOT
//     recoverable by refresh of the same manifest — the registry must
//     publish a new manifest. Clients MUST NOT trust the current manifest
//     regardless of other state.
type StalenessStatus int

const (
	// StalenessFresh means the manifest is within the 72h window (or the
	// window since fetched_at) AND any declared `expires` is in the future.
	StalenessFresh StalenessStatus = iota

	// StalenessStale means the 72h window from last-successful fetch has
	// elapsed. `expires` (if present) is still in the future — a refresh
	// is expected to restore StalenessFresh without a new manifest.
	StalenessStale

	// StalenessExpired means the manifest's declared `expires` has passed.
	// The spec says clients MUST NOT trust the manifest after that time;
	// a refresh of the same manifest does not help.
	StalenessExpired
)

// String returns a short diagnostic label.
func (s StalenessStatus) String() string {
	switch s {
	case StalenessFresh:
		return "fresh"
	case StalenessStale:
		return "stale"
	case StalenessExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// CheckStaleness classifies a (lastFetched, expires, now) triple against the
// 72h default threshold. It is a pure function — no I/O, no global state,
// `now` is supplied explicitly so tests can pin the clock.
//
// Semantics, in priority order:
//
//  1. If expires != nil AND now >= *expires → StalenessExpired.
//     Expiry wins over every other signal because the spec says clients
//     MUST NOT trust the manifest after that time. This is checked first
//     so that a manifest that is "fresh" by the 72h window but already
//     past its declared expiry still reports Expired.
//
//  2. If lastFetched is the zero time OR now − lastFetched ≥ 72h →
//     StalenessStale. A zero lastFetched means no successful fetch has
//     ever recorded a timestamp for this registry — treated as infinitely
//     stale (fail-closed default; safer than interpreting zero as "just
//     fetched"). The ≥ boundary is inclusive: at exactly 72h elapsed, the
//     manifest is already stale, not still fresh.
//
//  3. Otherwise → StalenessFresh.
//
// `now` normalizes to UTC before comparison so callers passing either UTC
// or a local-zone time produce identical results. The `lastFetched` value
// is expected to already be UTC (SetRegistryFetchedAt enforces this) — if
// it is not, the comparison still works because time.Time.Sub is
// monotonic-aware.
func CheckStaleness(lastFetched time.Time, expires *time.Time, now time.Time) StalenessStatus {
	now = now.UTC()

	if expires != nil {
		exp := expires.UTC()
		if !now.Before(exp) {
			return StalenessExpired
		}
	}

	if lastFetched.IsZero() {
		return StalenessStale
	}
	if now.Sub(lastFetched) >= DefaultStalenessThreshold {
		return StalenessStale
	}
	return StalenessFresh
}

// CheckRegistry is the composed form most callers use: look up the last
// successful fetch for registryURL in the lockfile, pair it with the
// manifest's optional Expires, and return the StalenessStatus. Returns
// StalenessStale for a nil lockfile or an unknown registryURL (fail-closed
// — never observed means never confirmed fresh). Returns StalenessExpired
// if manifest.Expires is in the past, even without a lockfile entry, so a
// freshly-received-but-expired manifest is caught on first inspection.
//
// Mirrors the CheckStaleness priority: expiry is checked first.
func CheckRegistry(lf *Lockfile, registryURL string, m *Manifest, now time.Time) StalenessStatus {
	var expires *time.Time
	if m != nil {
		expires = m.Expires
	}
	var lastFetched time.Time
	if lf != nil {
		if rs, ok := lf.Registries[registryURL]; ok {
			lastFetched = rs.FetchedAt
		}
	}
	return CheckStaleness(lastFetched, expires, now)
}
