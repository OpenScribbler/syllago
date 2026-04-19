package moat

// Clock-injected tests for the bundled trusted-root staleness policy. The
// BundledTrustedRoot function takes `now` so we can exercise the Fresh / Warn
// / Escalated / Expired bands at exact day boundaries (0, 89, 90, 179, 180,
// 364, 365) without mocking time.Now.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// issuedAt parses the package-level TrustedRootIssuedAtISO constant once per
// test via the same path production uses. If this fails, the constant is
// malformed — a build-time bug, not a runtime failure.
func issuedAt(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02", TrustedRootIssuedAtISO)
	if err != nil {
		t.Fatalf("TrustedRootIssuedAtISO=%q is not YYYY-MM-DD: %v", TrustedRootIssuedAtISO, err)
	}
	return ts
}

// TestBundledTrustedRoot_BytesNonEmpty — the go:embed directive must produce
// non-zero bytes, and those bytes must be valid JSON. A silent regression to
// an empty file would let verification pass with no trust anchor.
func TestBundledTrustedRoot_BytesNonEmpty(t *testing.T) {
	t.Parallel()
	info := BundledTrustedRoot(issuedAt(t))
	if len(info.Bytes) == 0 {
		t.Fatal("bundled trusted root bytes must be non-empty")
	}
	var probe map[string]any
	if err := json.Unmarshal(info.Bytes, &probe); err != nil {
		t.Fatalf("bundled trusted root must be valid JSON: %v", err)
	}
}

// TestBundledTrustedRoot_StalenessBands walks the boundary days and asserts
// the status bucket is what the policy promises. A change to
// TrustedRootFreshDays/WarnDays/EscalatedDays will break this and force a
// deliberate update — we don't want a silent one-day shift.
func TestBundledTrustedRoot_StalenessBands(t *testing.T) {
	t.Parallel()
	base := issuedAt(t)
	cases := []struct {
		name    string
		ageDays int
		want    TrustedRootStatus
	}{
		{"day-0", 0, TrustedRootStatusFresh},
		{"day-89", 89, TrustedRootStatusFresh},
		{"day-90", 90, TrustedRootStatusWarn},
		{"day-179", 179, TrustedRootStatusWarn},
		{"day-180", 180, TrustedRootStatusEscalated},
		{"day-364", 364, TrustedRootStatusEscalated},
		{"day-365", 365, TrustedRootStatusExpired},
		{"day-700", 700, TrustedRootStatusExpired},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			now := base.AddDate(0, 0, tc.ageDays)
			info := BundledTrustedRoot(now)
			if info.Status != tc.want {
				t.Errorf("age=%d: got Status=%s want=%s", tc.ageDays, info.Status, tc.want)
			}
			if info.AgeDays != tc.ageDays {
				t.Errorf("age=%d: got AgeDays=%d want=%d", tc.ageDays, info.AgeDays, tc.ageDays)
			}
			if info.Source != TrustedRootSourceBundled {
				t.Errorf("age=%d: got Source=%s want=%s", tc.ageDays, info.Source, TrustedRootSourceBundled)
			}
			wantCliff := base.AddDate(0, 0, TrustedRootEscalatedDays)
			if !info.CliffDate.Equal(wantCliff) {
				t.Errorf("age=%d: got CliffDate=%s want=%s", tc.ageDays, info.CliffDate, wantCliff)
			}
		})
	}
}

// TestBundledTrustedRoot_ClockSkewIsFresh — a wall clock that's set before the
// issuedAt date must not be treated as stale. Negative ages collapse to Fresh
// so a user whose clock is wrong by a week doesn't get spurious warnings.
func TestBundledTrustedRoot_ClockSkewIsFresh(t *testing.T) {
	t.Parallel()
	before := issuedAt(t).AddDate(0, 0, -7)
	info := BundledTrustedRoot(before)
	if info.Status != TrustedRootStatusFresh {
		t.Errorf("clock skew into past should be Fresh, got %s", info.Status)
	}
}

// TestExitCodeForStatus pins the 0/1/2 contract that `moat trust status`
// emits. CI pipelines grep on these — do not reshape. The test covers every
// enum value so a new status added without an explicit mapping defaults to 2
// (fail-closed) rather than silently returning 0.
func TestExitCodeForStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status TrustedRootStatus
		want   int
	}{
		{TrustedRootStatusFresh, 0},
		{TrustedRootStatusWarn, 1},
		{TrustedRootStatusEscalated, 1},
		{TrustedRootStatusExpired, 2},
		{TrustedRootStatusMissing, 2},
		{TrustedRootStatusCorrupt, 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.status.String(), func(t *testing.T) {
			t.Parallel()
			if got := ExitCodeForStatus(tc.status); got != tc.want {
				t.Errorf("ExitCodeForStatus(%s) = %d, want %d", tc.status, got, tc.want)
			}
		})
	}
}

// TestStalenessMessage_ShapeByBand asserts the human-readable message
// follows the promised structure per band:
//   - Fresh      → empty (no stderr spam in the common case)
//   - Warn       → single line mentioning "warning" + self-update action
//   - Escalated  → multi-line, mentions "ESCALATED" + days-remaining
//   - Expired    → mentions "expired" + refuses-to-proceed + action
//   - Missing    → mentions "missing"
//   - Corrupt    → mentions "corrupt"
func TestStalenessMessage_ShapeByBand(t *testing.T) {
	t.Parallel()
	base := issuedAt(t)

	fresh := BundledTrustedRoot(base)
	if got := StalenessMessage(fresh); got != "" {
		t.Errorf("Fresh message must be empty, got %q", got)
	}

	warn := BundledTrustedRoot(base.AddDate(0, 0, 100))
	warnMsg := StalenessMessage(warn)
	if !strings.Contains(warnMsg, "warning") {
		t.Errorf("Warn message must contain 'warning', got %q", warnMsg)
	}
	if !strings.Contains(warnMsg, "syllago self-update") {
		t.Errorf("Warn message must point operator to self-update, got %q", warnMsg)
	}
	if strings.Count(warnMsg, "\n") != 0 {
		t.Errorf("Warn must be single line, got %d newlines: %q", strings.Count(warnMsg, "\n"), warnMsg)
	}

	esc := BundledTrustedRoot(base.AddDate(0, 0, 200))
	escMsg := StalenessMessage(esc)
	if !strings.Contains(escMsg, "ESCALATED") {
		t.Errorf("Escalated message must contain 'ESCALATED', got %q", escMsg)
	}
	if !strings.Contains(escMsg, "days remain") {
		t.Errorf("Escalated message must mention days remaining, got %q", escMsg)
	}
	if strings.Count(escMsg, "\n") < 2 {
		t.Errorf("Escalated must be multi-line, got %q", escMsg)
	}

	expired := BundledTrustedRoot(base.AddDate(0, 0, 400))
	expMsg := StalenessMessage(expired)
	if !strings.Contains(expMsg, "expired") {
		t.Errorf("Expired message must contain 'expired', got %q", expMsg)
	}
	if !strings.Contains(expMsg, "Verification refuses to proceed") {
		t.Errorf("Expired message must say it refuses to proceed, got %q", expMsg)
	}

	missing := TrustedRootInfo{Status: TrustedRootStatusMissing}
	if got := StalenessMessage(missing); !strings.Contains(got, "missing") {
		t.Errorf("Missing message must contain 'missing', got %q", got)
	}

	corrupt := TrustedRootInfo{Status: TrustedRootStatusCorrupt}
	if got := StalenessMessage(corrupt); !strings.Contains(got, "corrupt") {
		t.Errorf("Corrupt message must contain 'corrupt', got %q", got)
	}
}

// TestTrustedRootStatus_String asserts the operational vocabulary. Structured
// logs and human output both key on these strings — reshape breaks scrapers.
func TestTrustedRootStatus_String(t *testing.T) {
	t.Parallel()
	cases := map[TrustedRootStatus]string{
		TrustedRootStatusFresh:     "fresh",
		TrustedRootStatusWarn:      "warn",
		TrustedRootStatusEscalated: "escalated",
		TrustedRootStatusExpired:   "expired",
		TrustedRootStatusMissing:   "missing",
		TrustedRootStatusCorrupt:   "corrupt",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(status), got, want)
		}
	}
	// Unknown enum value falls through to the default branch — that branch
	// must be reachable so future log-reading code sees unknown(<int>) rather
	// than an empty string.
	const unknown TrustedRootStatus = 99
	if got := unknown.String(); !strings.HasPrefix(got, "unknown(") {
		t.Errorf("unknown enum must render as 'unknown(N)', got %q", got)
	}
}

// TestDaysBetween covers the arithmetic — small inputs, hour-precision
// truncation, and negative results for clock skew.
func TestDaysBetween(t *testing.T) {
	t.Parallel()
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		end  time.Time
		want int
	}{
		{"same-instant", anchor, 0},
		{"+23h", anchor.Add(23 * time.Hour), 0},
		{"+24h", anchor.Add(24 * time.Hour), 1},
		{"+48h", anchor.Add(48 * time.Hour), 2},
		{"-24h", anchor.Add(-24 * time.Hour), -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := daysBetween(anchor, tc.end); got != tc.want {
				t.Errorf("daysBetween(%s, %s) = %d, want %d", anchor, tc.end, got, tc.want)
			}
		})
	}
}
