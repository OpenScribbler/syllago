package moat

import (
	"testing"
	"time"
)

// fixedNow is the anchor for every freshness test. Using a fixed UTC
// instant (not time.Now()) keeps the tests deterministic across time
// zones and across daylight-saving-time boundaries. 2026-03-15 is chosen
// because it falls after a DST transition in most Northern-hemisphere
// jurisdictions, so any accidental local-zone arithmetic would surface
// here rather than lurking silently.
var fixedNow = time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

func TestCheckStaleness_FreshWithin72h(t *testing.T) {
	cases := []struct {
		name string
		age  time.Duration
	}{
		{"just_fetched", 0},
		{"one_hour_ago", 1 * time.Hour},
		{"twenty_four_hours", 24 * time.Hour},
		{"seventy_one_hours_fifty_nine_minutes", 71*time.Hour + 59*time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckStaleness(fixedNow.Add(-tc.age), nil, fixedNow)
			if got != StalenessFresh {
				t.Errorf("%s: got %s, want fresh", tc.name, got)
			}
		})
	}
}

func TestCheckStaleness_StaleAtOrBeyond72h(t *testing.T) {
	// Boundary is inclusive: exactly 72h elapsed MUST classify as stale.
	// The spec's "clients MUST NOT trust after [threshold]" language is
	// satisfied only if the threshold value itself already trips the rule.
	cases := []struct {
		name string
		age  time.Duration
	}{
		{"exactly_72h", 72 * time.Hour},
		{"72h_one_second", 72*time.Hour + time.Second},
		{"one_week", 7 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckStaleness(fixedNow.Add(-tc.age), nil, fixedNow)
			if got != StalenessStale {
				t.Errorf("%s: got %s, want stale", tc.name, got)
			}
		})
	}
}

func TestCheckStaleness_ZeroLastFetchedIsStale(t *testing.T) {
	// A zero timestamp means "no successful fetch has ever been recorded"
	// — fail-closed. Interpreting zero as "just fetched" would silently
	// mark every fresh registry as Fresh before the first fetch completes.
	got := CheckStaleness(time.Time{}, nil, fixedNow)
	if got != StalenessStale {
		t.Errorf("zero lastFetched must fail-closed to stale, got %s", got)
	}
}

func TestCheckStaleness_ExpiresInPastBeatsFreshWindow(t *testing.T) {
	// Expiry priority: even a manifest fetched one second ago is Expired
	// if its declared `expires` is already in the past. The spec says
	// clients MUST NOT trust after `expires` — the 72h window is a floor,
	// not a ceiling.
	expires := fixedNow.Add(-1 * time.Second)
	got := CheckStaleness(fixedNow.Add(-1*time.Second), &expires, fixedNow)
	if got != StalenessExpired {
		t.Errorf("past expires must dominate fresh window, got %s", got)
	}
}

func TestCheckStaleness_ExpiresAtExactlyNowIsExpired(t *testing.T) {
	// Boundary inclusive in the same direction as the staleness threshold:
	// at exactly `expires`, the manifest is already expired. "MUST NOT
	// trust after that time" is interpreted as "at or after".
	expires := fixedNow
	got := CheckStaleness(fixedNow.Add(-1*time.Hour), &expires, fixedNow)
	if got != StalenessExpired {
		t.Errorf("now == expires must classify as expired, got %s", got)
	}
}

func TestCheckStaleness_ExpiresInFutureIrrelevantToFreshness(t *testing.T) {
	// A far-future `expires` does NOT shield a manifest from the 72h
	// staleness window. Registries sometimes set `expires` months out for
	// their TOFU contract, but clients still enforce refresh cadence
	// against fetched_at independently.
	expires := fixedNow.Add(365 * 24 * time.Hour)
	got := CheckStaleness(fixedNow.Add(-100*time.Hour), &expires, fixedNow)
	if got != StalenessStale {
		t.Errorf("future expires must not mask 72h staleness, got %s", got)
	}
}

func TestCheckStaleness_FreshWithFutureExpires(t *testing.T) {
	expires := fixedNow.Add(30 * 24 * time.Hour)
	got := CheckStaleness(fixedNow.Add(-1*time.Hour), &expires, fixedNow)
	if got != StalenessFresh {
		t.Errorf("within 72h + future expires must be fresh, got %s", got)
	}
}

func TestCheckStaleness_NilExpiresDoesNotCrash(t *testing.T) {
	// Guard: the spec marks expires as OPTIONAL. A nil pointer must be
	// indistinguishable from "no expiry set".
	got := CheckStaleness(fixedNow.Add(-1*time.Hour), nil, fixedNow)
	if got != StalenessFresh {
		t.Errorf("nil expires with recent fetch must be fresh, got %s", got)
	}
}

func TestCheckStaleness_StringLabels(t *testing.T) {
	cases := map[StalenessStatus]string{
		StalenessFresh:      "fresh",
		StalenessStale:      "stale",
		StalenessExpired:    "expired",
		StalenessStatus(99): "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("StalenessStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestCheckStaleness_UTCNormalization(t *testing.T) {
	// A lastFetched expressed in a non-UTC zone must produce the same
	// result as the equivalent UTC instant. Catches an accidental
	// wall-clock subtraction that forgets zone conversion.
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("time zone data unavailable: %v", err)
	}
	utcFetched := fixedNow.Add(-1 * time.Hour)
	laFetched := utcFetched.In(la)

	utcRes := CheckStaleness(utcFetched, nil, fixedNow)
	laRes := CheckStaleness(laFetched, nil, fixedNow)
	if utcRes != laRes {
		t.Errorf("zone should not affect result: utc=%s, la=%s", utcRes, laRes)
	}
}

func TestCheckRegistry_FailsClosedOnNilInputs(t *testing.T) {
	// A nil lockfile (no prior fetch observed) OR an unknown registry
	// URL both mean "never confirmed fresh" — stale, not a crash.
	if got := CheckRegistry(nil, "https://r/mf", nil, fixedNow); got != StalenessStale {
		t.Errorf("nil lockfile must report stale, got %s", got)
	}

	lf := NewLockfile()
	if got := CheckRegistry(lf, "https://unknown/mf", nil, fixedNow); got != StalenessStale {
		t.Errorf("unknown registry must report stale, got %s", got)
	}
}

func TestCheckRegistry_ReadsFetchedAtFromLockfile(t *testing.T) {
	lf := NewLockfile()
	lf.SetRegistryFetchedAt("https://r/mf", fixedNow.Add(-1*time.Hour))
	m := &Manifest{} // no expires

	if got := CheckRegistry(lf, "https://r/mf", m, fixedNow); got != StalenessFresh {
		t.Errorf("recent fetched_at should classify as fresh, got %s", got)
	}
}

func TestCheckRegistry_HonorsManifestExpires(t *testing.T) {
	lf := NewLockfile()
	lf.SetRegistryFetchedAt("https://r/mf", fixedNow.Add(-1*time.Hour))
	expired := fixedNow.Add(-1 * time.Minute)
	m := &Manifest{Expires: &expired}

	if got := CheckRegistry(lf, "https://r/mf", m, fixedNow); got != StalenessExpired {
		t.Errorf("past manifest.expires should dominate, got %s", got)
	}
}

func TestCheckRegistry_NilManifestStillChecksFetchedAt(t *testing.T) {
	// Callers sometimes want to staleness-check a registry even without
	// a parsed manifest on hand (e.g., pre-fetch gate). A nil manifest
	// means "no expiry constraint" and the 72h window is the sole signal.
	lf := NewLockfile()
	lf.SetRegistryFetchedAt("https://r/mf", fixedNow.Add(-100*time.Hour))
	if got := CheckRegistry(lf, "https://r/mf", nil, fixedNow); got != StalenessStale {
		t.Errorf("stale fetched_at with nil manifest should be stale, got %s", got)
	}
}

func TestDefaultStalenessThreshold_Is72Hours(t *testing.T) {
	// Spec-normative constant. A change to this value is a spec change
	// (not a tuning knob) — if this test fails, the constant was altered
	// unintentionally or the spec moved. Either way, it needs a review.
	if DefaultStalenessThreshold != 72*time.Hour {
		t.Errorf("DefaultStalenessThreshold drifted from spec: got %s, want 72h",
			DefaultStalenessThreshold)
	}
}
