// The roundtrip test sits in the installer_test external test package so it
// can import both installer and installcheck. Keeping it in package installer
// would create an import cycle (installcheck depends on installer).
package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// TestRoundtrip_NormalizationChain is the D21 ship gate: 5 fixtures × 2
// pre-states = 10 cells. Each cell exercises the full byte path: splitter →
// normalize → rulestore write → rulestore load → installer append → scan →
// uninstall, and asserts the target file's pre-install bytes are restored
// byte-for-byte after uninstall. If any cell fails, a normalization-layer
// mismatch is silent in production and must be fixed at the root.
func TestRoundtrip_NormalizationChain(t *testing.T) {
	// Locate the fixtures dir relative to this test file's package.
	fixturesDir := filepath.Join("..", "converter", "testdata", "splitter")

	fixtures := []string{
		"h2-clean.md",
		"crlf-line-endings.md",
		"bom-prefix.md",
		"no-trailing-newline.md",
		"trailing-whitespace.md",
	}
	preStates := []string{"empty", "non-empty"}

	for _, fx := range fixtures {
		for _, ps := range preStates {
			name := fx + "/" + ps
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				runRoundtripCell(t, filepath.Join(fixturesDir, fx), ps)
			})
		}
	}
}

// runRoundtripCell executes the 8-step chain for one (fixture, pre-state) cell.
func runRoundtripCell(t *testing.T, fixturePath, preState string) {
	t.Helper()
	// Step 1: load fixture bytes.
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}

	// Step 2: splitter.Split(canonical.Normalize(raw), HeuristicSingle).
	normalized := canonical.Normalize(raw)
	cands, skip := splitter.Split(normalized, splitter.Options{Heuristic: splitter.HeuristicSingle})
	if skip != nil {
		t.Fatalf("splitter: unexpected skip signal %+v", skip)
	}
	if len(cands) != 1 {
		t.Fatalf("splitter: expected 1 candidate, got %d", len(cands))
	}
	cand := cands[0]

	// Step 3: normalize candidate body (already canonical from normalized input
	// going through HeuristicSingle which returns Body = string(source)).
	candBody := canonical.Normalize([]byte(cand.Body))

	// Step 4: rulestore.WriteRuleWithSource → <contentRoot>/<provider>/<slug>.
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	libraryRoot := filepath.Join(projectRoot, "syllago-library")
	meta := metadata.RuleMetadata{
		ID:   "lib-id-d21",
		Name: "d21-rule",
	}
	// The filename for .source/ is the fixture basename; the sourceBytes are
	// the raw fixture bytes (pre-canonicalization) — D11's original-source
	// storage is for lossless same-provider install, not part of the canonical
	// roundtrip.
	if err := rulestore.WriteRuleWithSource(libraryRoot, "claude-code", "d21-rule", meta, candBody, filepath.Base(fixturePath), raw); err != nil {
		t.Fatalf("WriteRuleWithSource: %v", err)
	}

	// Step 5: rulestore.LoadRule → Loaded (history map is canonical-hash keyed).
	ruleDir := filepath.Join(libraryRoot, "claude-code", "d21-rule")
	loaded, err := rulestore.LoadRule(ruleDir)
	if err != nil {
		t.Fatalf("LoadRule: %v", err)
	}
	library := map[string]*rulestore.Loaded{loaded.Meta.ID: loaded}

	// Step 6: seed target per pre-state + snapshot, then InstallRuleAppend.
	target := filepath.Join(projectRoot, "CLAUDE.md")
	var preSnapshot []byte
	switch preState {
	case "empty":
		if err := os.WriteFile(target, nil, 0644); err != nil {
			t.Fatalf("seed empty target: %v", err)
		}
		preSnapshot = []byte{}
	case "non-empty":
		preSnapshot = []byte("preamble content\nmore preamble\n")
		if err := os.WriteFile(target, preSnapshot, 0644); err != nil {
			t.Fatalf("seed non-empty target: %v", err)
		}
	default:
		t.Fatalf("unknown pre-state %q", preState)
	}
	if err := installer.InstallRuleAppend(projectRoot, homeDir, "claude-code", target, "manual", loaded); err != nil {
		t.Fatalf("InstallRuleAppend: %v", err)
	}

	// Step 7: scan → assert Clean/None.
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	result := installcheck.Scan(inst, library)
	key := installcheck.RecordKey{LibraryID: loaded.Meta.ID, TargetFile: target}
	got := result.PerRecord[key]
	if got.State != installcheck.StateClean || got.Reason != installcheck.ReasonNone {
		t.Errorf("PerRecord[%v] = %+v, want {StateClean, ReasonNone}; warnings=%v", key, got, result.Warnings)
	}

	// Step 8: UninstallRuleAppend → target bytes byte-equal to pre-install.
	if err := installer.UninstallRuleAppend(projectRoot, loaded.Meta.ID, target, library); err != nil {
		t.Fatalf("UninstallRuleAppend: %v", err)
	}
	post, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after uninstall: %v", err)
	}
	if string(post) != string(preSnapshot) {
		t.Errorf("roundtrip bytes mismatch\n got  %q\nwant %q", post, preSnapshot)
	}
}
