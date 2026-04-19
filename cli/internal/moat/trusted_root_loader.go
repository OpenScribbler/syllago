package moat

// Bundled Sigstore trusted root + staleness policy per ADR 0007 D1.
//
// The trusted root (Fulcio CA bundle + Rekor public keys + timestamp
// authorities) is committed as a versioned asset and embedded at build time
// via go:embed. Two reasons this beats runtime TUF for slice 1:
//
//   1. syllago is distributed as a standalone binary — no persistent state
//      at first run. TUF bootstrapping adds a second trust root, a
//      filesystem cache, and network dependency before any verify works.
//   2. Every release is a reproducible signed artifact of what the client
//      trusts at that moment. Operators can pin a release by SHA.
//
// The cost is staleness: the public-good Sigstore instance rotates Fulcio
// CA and Rekor keys every 6–12 months, so the bundled root has a shelf
// life. CheckStaleness enforces the 90 / 180 / 365-day cliff so operators
// see the problem before verification starts silently failing against
// rotated keys.
//
// Refresh procedure:
//   1. Copy the latest trusted_root.json from the upstream Sigstore repo
//      (github.com/sigstore/sigstore-go/examples, or fetch via TUF offline).
//   2. Update the TrustedRootIssuedAtISO constant below to today's date.
//   3. Bump the syllago release so the new bundled root ships.

import (
	_ "embed"
	"fmt"
	"time"
)

// bundledTrustedRoot is the Sigstore public-good trust root — Fulcio CA
// certificates + Rekor public keys + timestamp authorities. Committed
// verbatim from sigstore-go@v1.1.4/examples on 2026-04-17.
//
//go:embed trusted_root.json
var bundledTrustedRoot []byte

// TrustedRootIssuedAtISO is the date the bundled trusted root was refreshed
// in this repository, in YYYY-MM-DD form. Update this EVERY time
// trusted_root.json is regenerated — the staleness policy reads it.
//
// Must match the commit date of the trusted_root.json update. Drift
// between this constant and the file mtime is a process bug, not a trust
// signal, so we don't consult the filesystem.
const TrustedRootIssuedAtISO = "2026-04-17"

// Staleness thresholds per ADR 0007 D1. Durations measured in whole days
// from TrustedRootIssuedAtISO to wall-clock now.
const (
	TrustedRootFreshDays     = 90
	TrustedRootWarnDays      = 180
	TrustedRootEscalatedDays = 365
)

// TrustedRootSource labels where the trusted root bytes came from. Emitted
// on every verify path so auditors can see what root is in effect. Silent
// override is the attack surface; loud override is the defense.
type TrustedRootSource string

const (
	TrustedRootSourceBundled  TrustedRootSource = "bundled"
	TrustedRootSourcePathFlag TrustedRootSource = "path" // reserved for slice 2+ --trusted-root
)

// TrustedRootStatus is the staleness bucket a given (issuedAt, now) pair
// falls into. The exit-code contract for `moat trust status` is anchored
// on this enum: Fresh → 0, Warn/Escalated → 1, Expired/Missing/Corrupt → 2.
type TrustedRootStatus int

const (
	TrustedRootStatusFresh TrustedRootStatus = iota
	TrustedRootStatusWarn
	TrustedRootStatusEscalated
	TrustedRootStatusExpired
	TrustedRootStatusMissing
	TrustedRootStatusCorrupt
)

// String renders the status for logs and human output. Operational vocabulary
// per ADR 0007: the slice-1 three-state output is signed / unsigned / invalid,
// but the trust root status is its own vocabulary tracking calendar freshness.
func (s TrustedRootStatus) String() string {
	switch s {
	case TrustedRootStatusFresh:
		return "fresh"
	case TrustedRootStatusWarn:
		return "warn"
	case TrustedRootStatusEscalated:
		return "escalated"
	case TrustedRootStatusExpired:
		return "expired"
	case TrustedRootStatusMissing:
		return "missing"
	case TrustedRootStatusCorrupt:
		return "corrupt"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// TrustedRootInfo is the caller-facing snapshot: where the root came from,
// when it was issued, how old it is, where the cliff is, and which
// staleness bucket it falls into.
type TrustedRootInfo struct {
	Source    TrustedRootSource
	IssuedAt  time.Time
	AgeDays   int
	CliffDate time.Time
	Status    TrustedRootStatus
	Bytes     []byte // the trusted-root JSON bytes, empty iff Status is Missing/Corrupt
}

// BundledTrustedRoot returns the embedded trusted-root bytes plus a status
// snapshot computed against the supplied wall-clock time. Pass time.Now()
// in production; tests inject a fixed clock to exercise staleness bands.
//
// The function always returns Bytes populated (the embed is a build-time
// guarantee). The returned Status may be Expired — callers deciding
// whether to proceed with verification should gate on Status != Expired,
// not on the nil-ness of err (there is no err return).
func BundledTrustedRoot(now time.Time) TrustedRootInfo {
	issued, parseErr := time.Parse("2006-01-02", TrustedRootIssuedAtISO)
	if parseErr != nil {
		// A parse failure here is a build-time bug (someone edited the
		// constant incorrectly). Surface it as Corrupt so CI catches it
		// loudly rather than silently treating the root as fresh.
		return TrustedRootInfo{
			Source: TrustedRootSourceBundled,
			Status: TrustedRootStatusCorrupt,
			Bytes:  bundledTrustedRoot,
		}
	}

	age := daysBetween(issued, now)
	cliff := issued.AddDate(0, 0, TrustedRootEscalatedDays)

	return TrustedRootInfo{
		Source:    TrustedRootSourceBundled,
		IssuedAt:  issued,
		AgeDays:   age,
		CliffDate: cliff,
		Status:    classifyStaleness(age),
		Bytes:     bundledTrustedRoot,
	}
}

// classifyStaleness buckets the age (days) against the threshold constants.
// Negative ages (clock skew into the past) are treated as Fresh — the
// alternative is hard-failing on a brittle wall-clock assumption, which is
// worse than an optimistic answer.
func classifyStaleness(ageDays int) TrustedRootStatus {
	switch {
	case ageDays < TrustedRootFreshDays:
		return TrustedRootStatusFresh
	case ageDays < TrustedRootWarnDays:
		return TrustedRootStatusWarn
	case ageDays < TrustedRootEscalatedDays:
		return TrustedRootStatusEscalated
	default:
		return TrustedRootStatusExpired
	}
}

// daysBetween returns whole calendar days from start to end, truncated
// toward zero. A negative result means end is before start (clock skew).
func daysBetween(start, end time.Time) int {
	return int(end.Sub(start) / (24 * time.Hour))
}

// ExitCodeForStatus maps a TrustedRootStatus to the `moat trust status`
// exit code. Stable contract per ADR 0007: 0 fresh, 1 warn/escalated,
// 2 expired/missing/corrupt. CI pipelines grep on these — do not reshape.
func ExitCodeForStatus(s TrustedRootStatus) int {
	switch s {
	case TrustedRootStatusFresh:
		return 0
	case TrustedRootStatusWarn, TrustedRootStatusEscalated:
		return 1
	default:
		return 2
	}
}

// StalenessMessage produces a human-readable one-line summary suitable for
// stderr warnings on verification paths. Empty string when Status is Fresh
// (don't spam on the common case).
//
// Escalated status emits a multi-line message because the operator is
// within one rotation of hard-failure; a single-line warn is not forceful
// enough. Expired/Missing/Corrupt emit an actionable error.
func StalenessMessage(info TrustedRootInfo) string {
	switch info.Status {
	case TrustedRootStatusFresh:
		return ""
	case TrustedRootStatusWarn:
		return fmt.Sprintf(
			"warning: bundled Sigstore trusted root is %d days old (issued %s). "+
				"Hard-fail at %s. Run `syllago self-update` to refresh.",
			info.AgeDays,
			info.IssuedAt.Format("2006-01-02"),
			info.CliffDate.Format("2006-01-02"),
		)
	case TrustedRootStatusEscalated:
		return fmt.Sprintf(
			"ESCALATED: bundled Sigstore trusted root is %d days old (issued %s).\n"+
				"             Hard-fail at %s — only %d days remain.\n"+
				"             Action: run `syllago self-update` to pick up the latest bundled root.",
			info.AgeDays,
			info.IssuedAt.Format("2006-01-02"),
			info.CliffDate.Format("2006-01-02"),
			TrustedRootEscalatedDays-info.AgeDays,
		)
	case TrustedRootStatusExpired:
		return fmt.Sprintf(
			"bundled Sigstore trusted root expired %d days ago (cliff %s, issued %s). "+
				"Verification refuses to proceed. Run `syllago self-update`.",
			info.AgeDays-TrustedRootEscalatedDays,
			info.CliffDate.Format("2006-01-02"),
			info.IssuedAt.Format("2006-01-02"),
		)
	case TrustedRootStatusMissing:
		return "trusted root is missing — cannot verify signatures"
	case TrustedRootStatusCorrupt:
		return "trusted root is corrupt — cannot verify signatures"
	default:
		return ""
	}
}
