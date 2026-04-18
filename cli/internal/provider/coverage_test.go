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

// TestCoverageNoDrift is the authoritative full-conformance gate. It fails if
// ANY drift is found across all four assertions. This is the gate that Phase 3
// of the provider-coverage-reconciliation plan closes (by bringing
// provider-sources/ and provider-formats/ YAMLs into sync with Go).
//
// Gated behind SYLLAGO_COVERAGE_STRICT=1 until Phase 3 lands. Once the YAMLs
// are reconciled, delete the env-var gate so this test runs by default on
// every `make test`.
//
// Manual invocation during Phase 2/3 work:
//
//	SYLLAGO_COVERAGE_STRICT=1 go test ./internal/provider/... -run Coverage
func TestCoverageNoDrift(t *testing.T) {
	if os.Getenv("SYLLAGO_COVERAGE_STRICT") != "1" {
		t.Skip("full Go↔YAML conformance gated behind SYLLAGO_COVERAGE_STRICT=1 until Phase 3 of provider-coverage-reconciliation lands")
	}

	repoRoot := mustFindRepoRoot(t)
	drifts, err := CheckCoverage(repoRoot)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}

	if len(drifts) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("provider coverage drift detected (")
	b.WriteString(strconv.Itoa(len(drifts)))
	b.WriteString(" issue")
	if len(drifts) != 1 {
		b.WriteString("s")
	}
	b.WriteString("):\n")
	for _, d := range drifts {
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
