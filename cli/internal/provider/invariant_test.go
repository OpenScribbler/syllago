package provider

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestCoverageInternalGoConsistency enforces the two Go-internal assertions:
//
//  3. configlocations-vs-supportstype — ConfigLocations[ct] set ⇒ SupportsType(ct) == true.
//  4. installdir-vs-supportstype       — InstallDir(home, ct) != "" ⇔ SupportsType(ct) == true.
//
// These drifts indicate programmer errors inside a single provider definition
// and MUST always be zero. Unlike Go↔YAML drift, these cannot be "in flight"
// — either the Go entry is consistent with itself or it isn't.
//
// This test is the hard gate for Phase 1 (provider code cleanup) and any
// future provider additions.
func TestCoverageInternalGoConsistency(t *testing.T) {
	repoRoot := mustFindRepoRoot(t)
	drifts, err := CheckCoverage(repoRoot)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}

	var internal []CoverageDrift
	for _, d := range drifts {
		switch d.Assertion {
		case AssertionConfigLocationsVsGo, AssertionInstallDirVsSupportsGo:
			internal = append(internal, d)
		}
	}
	if len(internal) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("internal Go drift detected (")
	b.WriteString(strconv.Itoa(len(internal)))
	b.WriteString(" issue")
	if len(internal) != 1 {
		b.WriteString("s")
	}
	b.WriteString("):\n")
	for _, d := range internal {
		b.WriteString("  - ")
		b.WriteString(d.String())
		b.WriteString("\n")
	}
	t.Fatal(b.String())
}

// TestCoverageNoDrift is the authoritative full-conformance gate. It fails
// if ANY unaccepted drift is found across all assertions (Go internal
// consistency, Go vs format YAMLs, Go vs the Capability Feed mirror). It
// runs on every `make test`.
//
// Drifts carrying an AcceptedReason (recorded, permanent disagreements —
// see acceptedFeedDrift in coverage_feed.go) are logged but never fail the
// gate, matching TestCoverageFeedDrift's semantics.
func TestCoverageNoDrift(t *testing.T) {
	repoRoot := mustFindRepoRoot(t)
	drifts, err := CheckCoverage(repoRoot)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}

	var failing []CoverageDrift
	for _, d := range drifts {
		if d.AcceptedReason != "" {
			t.Logf("coverage drift (accepted): %s — %s", d, d.AcceptedReason)
			continue
		}
		failing = append(failing, d)
	}
	if len(failing) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("provider coverage drift detected (")
	b.WriteString(strconv.Itoa(len(failing)))
	b.WriteString(" issue")
	if len(failing) != 1 {
		b.WriteString("s")
	}
	b.WriteString("):\n")
	for _, d := range failing {
		b.WriteString("  - ")
		b.WriteString(d.String())
		b.WriteString("\n")
	}
	t.Fatal(b.String())
}

func TestFindRepoRoot(t *testing.T) {
	repoRoot := mustFindRepoRoot(t)
	if _, err := os.Stat(repoRoot + "/cli/go.mod"); err != nil {
		t.Errorf("repo root %q missing cli/go.mod: %v", repoRoot, err)
	}
	if _, err := os.Stat(repoRoot + "/docs"); err != nil {
		t.Errorf("repo root %q missing docs/: %v", repoRoot, err)
	}
}

func mustFindRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := FindRepoRoot(wd)
	if root == "" {
		t.Fatalf("could not locate repo root starting from %s (expected cli/go.mod + docs/ markers)", wd)
	}
	return root
}
