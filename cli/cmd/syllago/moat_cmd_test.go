package main

// Tests for `moat trust status`.
//
// We call runMoatTrustStatus directly instead of exercising the cobra RunE
// wrapper because the production path calls os.Exit — not testable without a
// subprocess harness. The factored helper returns the info + exit code so
// tests can assert on both without trapping the exit.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// issuedAtForMoatTest parses the shipped TrustedRootIssuedAtISO so tests can
// build "now" values at known offsets from it. If this fails the binary
// wouldn't boot — safe to t.Fatal.
func issuedAtForMoatTest(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02", moat.TrustedRootIssuedAtISO)
	if err != nil {
		t.Fatalf("TrustedRootIssuedAtISO=%q is not YYYY-MM-DD: %v", moat.TrustedRootIssuedAtISO, err)
	}
	return ts
}

// TestRunMoatTrustStatus_Human_Fresh — day 0 must exit 0 with no stderr
// message and a human-readable key=value dump on stdout.
func TestRunMoatTrustStatus_Human_Fresh(t *testing.T) {
	t.Parallel()
	now := issuedAtForMoatTest(t)
	var stdout, stderr bytes.Buffer
	info, exit := runMoatTrustStatus(&stdout, &stderr, now, false)

	if exit != 0 {
		t.Errorf("exit code for Fresh must be 0, got %d", exit)
	}
	if info.Status != moat.TrustedRootStatusFresh {
		t.Errorf("Status = %s, want fresh", info.Status)
	}
	out := stdout.String()
	if !strings.Contains(out, "moat.trusted_root=bundled") {
		t.Errorf("human output missing acquisition line; got:\n%s", out)
	}
	if !strings.Contains(out, "moat.trusted_root.status=fresh") {
		t.Errorf("human output missing status=fresh; got:\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("Fresh must not write stderr, got: %q", stderr.String())
	}
}

// TestRunMoatTrustStatus_Human_Warn — day 100 must exit 1 and emit a single
// stderr warning line.
func TestRunMoatTrustStatus_Human_Warn(t *testing.T) {
	t.Parallel()
	now := issuedAtForMoatTest(t).AddDate(0, 0, 100)
	var stdout, stderr bytes.Buffer
	info, exit := runMoatTrustStatus(&stdout, &stderr, now, false)

	if exit != 1 {
		t.Errorf("Warn exit code must be 1, got %d", exit)
	}
	if info.Status != moat.TrustedRootStatusWarn {
		t.Errorf("Status = %s, want warn", info.Status)
	}
	se := stderr.String()
	if !strings.Contains(se, "warning") {
		t.Errorf("Warn stderr should mention 'warning', got %q", se)
	}
	if strings.Count(strings.TrimSpace(se), "\n") != 0 {
		t.Errorf("Warn stderr must be one line, got %q", se)
	}
}

// TestRunMoatTrustStatus_Human_Escalated — day 200 must exit 1 and emit a
// multi-line stderr warning.
func TestRunMoatTrustStatus_Human_Escalated(t *testing.T) {
	t.Parallel()
	now := issuedAtForMoatTest(t).AddDate(0, 0, 200)
	var stdout, stderr bytes.Buffer
	info, exit := runMoatTrustStatus(&stdout, &stderr, now, false)

	if exit != 1 {
		t.Errorf("Escalated exit code must be 1, got %d", exit)
	}
	if info.Status != moat.TrustedRootStatusEscalated {
		t.Errorf("Status = %s, want escalated", info.Status)
	}
	se := stderr.String()
	if !strings.Contains(se, "ESCALATED") {
		t.Errorf("Escalated stderr should contain 'ESCALATED', got %q", se)
	}
	if strings.Count(se, "\n") < 2 {
		t.Errorf("Escalated stderr must be multi-line, got %q", se)
	}
}

// TestRunMoatTrustStatus_Human_Expired — past the cliff, exit 2 with an
// actionable stderr message.
func TestRunMoatTrustStatus_Human_Expired(t *testing.T) {
	t.Parallel()
	now := issuedAtForMoatTest(t).AddDate(0, 0, 400)
	var stdout, stderr bytes.Buffer
	info, exit := runMoatTrustStatus(&stdout, &stderr, now, false)

	if exit != 2 {
		t.Errorf("Expired exit code must be 2, got %d", exit)
	}
	if info.Status != moat.TrustedRootStatusExpired {
		t.Errorf("Status = %s, want expired", info.Status)
	}
	se := stderr.String()
	if !strings.Contains(se, "expired") {
		t.Errorf("Expired stderr should mention 'expired', got %q", se)
	}
}

// TestRunMoatTrustStatus_JSON — --json emits a single valid JSON object
// containing all documented fields. Scripts key on this shape.
func TestRunMoatTrustStatus_JSON(t *testing.T) {
	t.Parallel()
	now := issuedAtForMoatTest(t).AddDate(0, 0, 30)
	var stdout, stderr bytes.Buffer
	_, exit := runMoatTrustStatus(&stdout, &stderr, now, true)

	if exit != 0 {
		t.Errorf("expected exit 0, got %d", exit)
	}
	var got trustStatusJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("JSON output must parse, got err=%v output=%q", err, stdout.String())
	}
	if got.Source != "bundled" {
		t.Errorf("source = %q, want bundled", got.Source)
	}
	if got.Status != "fresh" {
		t.Errorf("status = %q, want fresh", got.Status)
	}
	if got.AgeDays != 30 {
		t.Errorf("age_days = %d, want 30", got.AgeDays)
	}
	if got.IssuedAt == "" {
		t.Error("issued_at must be populated for bundled source")
	}
	if got.CliffDate == "" {
		t.Error("cliff_date must be populated for bundled source")
	}
}

// TestMoatTrustStatusCmd_WiredUnderMoatTrust — sanity check that the subcommand
// tree is wired up. If someone moves moatTrustStatusCmd without re-parenting
// it, this test catches it before the binary ships.
func TestMoatTrustStatusCmd_WiredUnderMoatTrust(t *testing.T) {
	t.Parallel()
	trust, _, err := moatCmd.Find([]string{"trust"})
	if err != nil {
		t.Fatalf("moat trust subcommand missing: %v", err)
	}
	status, _, err := trust.Find([]string{"status"})
	if err != nil {
		t.Fatalf("moat trust status subcommand missing: %v", err)
	}
	if status.Use != "status" {
		t.Errorf("expected Use=status, got %q", status.Use)
	}
}
