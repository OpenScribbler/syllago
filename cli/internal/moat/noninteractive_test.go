package moat

import (
	"strings"
	"testing"
)

// TestExitCodeFor_AllFourFailureClasses pins the spec-normative mapping.
// Each of the four G-18 MUST-exit conditions maps to a unique exit code in
// the 10–13 band. Pipelines grep on these — changing a value here is a
// breaking CLI contract change.
func TestExitCodeFor_AllFourFailureClasses(t *testing.T) {
	cases := []struct {
		name string
		f    NonInteractiveFailure
		want int
	}{
		{"tofu_acceptance", FailureTOFUAcceptance, 10},
		{"signing_profile_change", FailureSigningProfileChange, 11},
		{"publisher_revocation", FailurePublisherRevocation, 12},
		{"manifest_stale", FailureManifestStale, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeFor(tc.f); got != tc.want {
				t.Errorf("ExitCodeFor(%s) = %d, want %d", tc.f, got, tc.want)
			}
		})
	}
}

// TestExitCodeFor_NoneReturnsSuccess ensures the zero value of
// NonInteractiveFailure maps to exit 0. Callers should be able to pass
// FailureNone through without special-casing the "no failure" path.
func TestExitCodeFor_NoneReturnsSuccess(t *testing.T) {
	if got := ExitCodeFor(FailureNone); got != 0 {
		t.Errorf("ExitCodeFor(FailureNone) = %d, want 0", got)
	}
}

// TestExitCodeFor_UnknownReturnsZero locks the default-arm contract: an
// out-of-range enum value is treated as "no failure classified" rather
// than a crash. A panic would trade a silent pass for a crash at a trust
// boundary — safer to return 0 and let the caller's other checks decide.
func TestExitCodeFor_UnknownReturnsZero(t *testing.T) {
	if got := ExitCodeFor(NonInteractiveFailure(99)); got != 0 {
		t.Errorf("ExitCodeFor(unknown) = %d, want 0", got)
	}
}

// TestExitCodeFor_AllCodesAreDistinct guards against accidental duplication
// during edits. The four failure classes must each be distinguishable —
// collapsing any two codes would make the failure class un-recoverable by
// an automated caller.
func TestExitCodeFor_AllCodesAreDistinct(t *testing.T) {
	codes := map[int]NonInteractiveFailure{}
	failures := []NonInteractiveFailure{
		FailureTOFUAcceptance,
		FailureSigningProfileChange,
		FailurePublisherRevocation,
		FailureManifestStale,
	}
	for _, f := range failures {
		c := ExitCodeFor(f)
		if prev, seen := codes[c]; seen {
			t.Errorf("exit code %d collides: %s and %s map to the same code",
				c, prev, f)
		}
		codes[c] = f
	}
}

// TestExitCodeFor_CodesInReservedBand ensures each MOAT non-interactive
// exit code falls in the 10–19 reserved band. If someone adds a fifth
// failure class with code 14, this still passes; if they pick code 3,
// they collide with ExitDrift and this test catches it.
func TestExitCodeFor_CodesInReservedBand(t *testing.T) {
	failures := []NonInteractiveFailure{
		FailureTOFUAcceptance,
		FailureSigningProfileChange,
		FailurePublisherRevocation,
		FailureManifestStale,
	}
	for _, f := range failures {
		c := ExitCodeFor(f)
		if c < 10 || c > 19 {
			t.Errorf("%s maps to exit %d, want in 10–19 reserved band", f, c)
		}
	}
}

// TestExitCodeConstants_StableValues pins each named constant. The named
// constants are the primary source of truth; ExitCodeFor indirects through
// them. A typo in the mapping surfaces here even if the switch happens to
// still return the same number.
func TestExitCodeConstants_StableValues(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"ExitMoatTOFUAcceptance", ExitMoatTOFUAcceptance, 10},
		{"ExitMoatSigningProfileChange", ExitMoatSigningProfileChange, 11},
		{"ExitMoatPublisherRevocation", ExitMoatPublisherRevocation, 12},
		{"ExitMoatManifestStale", ExitMoatManifestStale, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestNonInteractiveFailure_StringLabels pins the kebab-case label set.
// These labels ship in structured-log outputs and stderr prefixes; log
// pipelines key off them, so a rename is a breaking change.
func TestNonInteractiveFailure_StringLabels(t *testing.T) {
	cases := map[NonInteractiveFailure]string{
		FailureNone:                 "none",
		FailureTOFUAcceptance:       "tofu-acceptance",
		FailureSigningProfileChange: "signing-profile-change",
		FailurePublisherRevocation:  "publisher-revocation",
		FailureManifestStale:        "manifest-stale",
	}
	for f, want := range cases {
		t.Run(want, func(t *testing.T) {
			if got := f.String(); got != want {
				t.Errorf("%d.String() = %q, want %q", int(f), got, want)
			}
		})
	}
}

// TestNonInteractiveFailure_UnknownStringEmbedsNumber: for diagnostic
// recovery, an unknown value's String includes the raw int. A blank label
// here would make "unknown non-interactive failure" log lines useless for
// bug reports.
func TestNonInteractiveFailure_UnknownStringEmbedsNumber(t *testing.T) {
	got := NonInteractiveFailure(99).String()
	if !strings.Contains(got, "99") {
		t.Errorf("unknown label should embed the number, got %q", got)
	}
	if !strings.Contains(got, "unknown") {
		t.Errorf("unknown label should say \"unknown\", got %q", got)
	}
}

// TestNonInteractiveFailure_MessageNonEmptyForFailures: every declared
// failure class MUST surface a non-empty, actionable message. Callers
// wiring G-18 into stderr rely on Message() to avoid raw cobra errors.
func TestNonInteractiveFailure_MessageNonEmptyForFailures(t *testing.T) {
	failures := []NonInteractiveFailure{
		FailureTOFUAcceptance,
		FailureSigningProfileChange,
		FailurePublisherRevocation,
		FailureManifestStale,
	}
	for _, f := range failures {
		t.Run(f.String(), func(t *testing.T) {
			if got := f.Message(); got == "" {
				t.Errorf("%s.Message() is empty — every failure class needs user text", f)
			}
		})
	}
}

// TestNonInteractiveFailure_MessageEmptyForNone: FailureNone must not emit
// a message — otherwise callers would print chatter on the success path.
func TestNonInteractiveFailure_MessageEmptyForNone(t *testing.T) {
	if got := FailureNone.Message(); got != "" {
		t.Errorf("FailureNone.Message() = %q, want empty", got)
	}
}

// TestNonInteractiveFailure_MessageIsActionable checks that each failure
// class message names the recovery command ("syllago registry ..."). The
// spec does not require a command, but operators parsing CI logs benefit
// from a direct pointer. A message that says only "stale" with no next
// step is the vague-limitations anti-pattern.
func TestNonInteractiveFailure_MessageIsActionable(t *testing.T) {
	cases := map[NonInteractiveFailure]string{
		FailureTOFUAcceptance:       "syllago registry add",
		FailureSigningProfileChange: "syllago registry remove",
		FailureManifestStale:        "syllago registry sync",
	}
	for f, needle := range cases {
		t.Run(f.String(), func(t *testing.T) {
			msg := f.Message()
			if !strings.Contains(msg, needle) {
				t.Errorf("%s.Message() missing actionable reference %q: %s", f, needle, msg)
			}
		})
	}
	// Publisher revocation has no "fix" recovery — the message must at
	// least explain why non-interactive can't proceed, not name a command.
	rev := FailurePublisherRevocation.Message()
	if !strings.Contains(rev, "non-interactive") {
		t.Errorf("publisher revocation message should mention non-interactive constraint: %q", rev)
	}
}
