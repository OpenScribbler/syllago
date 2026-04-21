// Package moat — non-interactive failure classification (ADR 0007 G-18).
//
// MOAT spec v0.6.0 §Revocation Mechanism defines four conditions under which
// a conforming non-interactive client (CI/CD pipeline, fleet agent, headless
// install script) MUST exit non-zero and MUST produce a machine-distinguishable
// error signal. The spec leaves the signal mechanism open — stderr prefix,
// structured JSON, or distinct exit codes — but requires that an automated
// caller be able to tell the failure classes apart.
//
// Syllago's choice: distinct exit codes. The decision is recorded in the
// project plan's AD-9 and maps each normative MUST-exit condition to a unique
// code in the application-specific band (10–13):
//
//	10  Exit MOAT TOFU acceptance      — first registry add requires human judgment
//	11  Exit MOAT signing-profile change — registry_signing_profile changed
//	12  Exit MOAT publisher revocation  — publisher-source revocation encountered
//	13  Exit MOAT manifest stale        — 72h staleness threshold exceeded (G-9)
//
// Why a new band instead of extending 0–3:
//   - 0–2 are shell/cobra reserved (success, general error, usage).
//   - `ExitDrift = 3` in cli/internal/output is drift-command-specific.
//   - Application-specific failure codes ≥10 is a widely-used convention
//     (git, curl, rsync, nginx). Leaving a gap means a future general exit
//     code can land at 4 without renumbering the MOAT band.
//
// Scope of this module: pure classification and mapping. The registry-sync
// and install-gate paths that detect each condition, set the failure class,
// and call os.Exit via Cobra's RunE return lives in the Phase 2 merge
// (syllago-dsqjz). Keeping the primitives pure lets the enforcement path be
// tested without touching the fetch or install machinery.
//
// Pre-approval mechanism: MOAT defers an out-of-band trust-approval flow to
// upstream ROADMAP Issue 11. Syllago matches that deferral — no pre-approval
// path in this module. A conforming non-interactive client MUST NOT silently
// auto-accept any of these four conditions.
package moat

import "fmt"

// NonInteractiveFailure is the normative classification of the four
// MUST-exit conditions. Zero value (FailureNone) means "no non-interactive
// failure" and maps to ExitSuccess — callers can pass it through without
// special-casing.
type NonInteractiveFailure int

const (
	// FailureNone is the zero value — no MOAT non-interactive failure.
	// Emitted when the check passed, or when the caller has no failure to
	// report. Maps to exit code 0 (success).
	FailureNone NonInteractiveFailure = iota

	// FailureTOFUAcceptance indicates a registry add (or first trust-anchor
	// load) that would require a human Y/n. A trust decision that needs
	// human judgment MUST NOT be made silently by a pipeline. Spec
	// §Revocation Mechanism row 1.
	FailureTOFUAcceptance

	// FailureSigningProfileChange indicates the registry's
	// `registry_signing_profile` moved to a different identity. Could
	// indicate registry-key compromise; non-interactive clients MUST NOT
	// silently trust the new profile. Recover with
	// `syllago registry approve <name>` interactively. Spec row 2.
	FailureSigningProfileChange

	// FailurePublisherRevocation indicates a publisher-source revocation
	// (`revocations[].source == "publisher"`) was encountered during a
	// non-interactive install. Interactive callers get a Y/n; non-interactive
	// callers MUST exit and MUST NOT proceed. Spec row 3 + G-8/G-17.
	FailurePublisherRevocation

	// FailureManifestStale indicates the 72-hour freshness threshold has
	// been exceeded (see G-9 / CheckStaleness). Operating on potentially
	// outdated trust data — refresh the manifest interactively first.
	// Spec row 4.
	FailureManifestStale
)

// Exit codes for the four MOAT non-interactive failure classes. These are
// the machine-distinguishable signal required by spec §Revocation Mechanism.
// Values are part of Syllago's public CLI contract — pipelines grep on
// these — and MUST NOT be reshaped without a deprecation window.
//
// The band (10–13) is reserved exclusively for MOAT non-interactive
// failures. New MUST-exit conditions (e.g., from future spec revisions)
// should be appended to the band rather than colliding with 0–3.
const (
	ExitMoatTOFUAcceptance       = 10
	ExitMoatSigningProfileChange = 11
	ExitMoatPublisherRevocation  = 12
	ExitMoatManifestStale        = 13
)

// ExitCodeFor maps a NonInteractiveFailure to its Syllago exit code.
// FailureNone returns 0 (success). Unknown values also return 0 — a
// deliberately conservative choice for a zero-valued Go enum; callers that
// construct failure values outside this package are expected to use the
// named constants. A panic on unknown would trade a silent pass for a
// crash at a trust boundary; returning 0 means "no failure classified" and
// keeps the mapping total.
//
// Non-MOAT errors (usage, I/O, bug) should use output.ExitError /
// output.ExitUsage — this function is specifically for the four G-18
// normative conditions.
func ExitCodeFor(f NonInteractiveFailure) int {
	switch f {
	case FailureTOFUAcceptance:
		return ExitMoatTOFUAcceptance
	case FailureSigningProfileChange:
		return ExitMoatSigningProfileChange
	case FailurePublisherRevocation:
		return ExitMoatPublisherRevocation
	case FailureManifestStale:
		return ExitMoatManifestStale
	default:
		return 0
	}
}

// String returns a short, stable diagnostic label suitable for structured
// logs and stderr prefixes. Labels are kebab-case to compose with
// log-ingestion pipelines that key off them.
func (f NonInteractiveFailure) String() string {
	switch f {
	case FailureNone:
		return "none"
	case FailureTOFUAcceptance:
		return "tofu-acceptance"
	case FailureSigningProfileChange:
		return "signing-profile-change"
	case FailurePublisherRevocation:
		return "publisher-revocation"
	case FailureManifestStale:
		return "manifest-stale"
	default:
		return fmt.Sprintf("unknown(%d)", int(f))
	}
}

// Message returns a human-readable, actionable stderr line for the failure
// class. Empty string for FailureNone. Callers emitting these messages
// SHOULD prefix with `syllago: ` so pipelines can grep by prefix + failure
// label. The text here intentionally avoids mentioning the exit code — the
// code is the machine signal; the message is the human signal.
func (f NonInteractiveFailure) Message() string {
	switch f {
	case FailureNone:
		return ""
	case FailureTOFUAcceptance:
		return "registry trust requires interactive approval; run `syllago registry add` " +
			"interactively or provide a pre-approved trust anchor"
	case FailureSigningProfileChange:
		return "registry_signing_profile has changed since the last approval; " +
			"re-approve interactively with `syllago registry approve <name>`"
	case FailurePublisherRevocation:
		return "publisher revocation encountered; non-interactive installs " +
			"cannot proceed past publisher advisories"
	case FailureManifestStale:
		return "registry manifest is stale (>72h since last successful fetch); " +
			"refresh with `syllago registry sync` before installing"
	default:
		return fmt.Sprintf("unknown non-interactive failure (%d)", int(f))
	}
}
