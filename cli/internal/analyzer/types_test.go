package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.AutoThreshold != 0.80 {
		t.Errorf("AutoThreshold = %v, want 0.80", cfg.AutoThreshold)
	}
	if cfg.SkipThreshold != 0.50 {
		t.Errorf("SkipThreshold = %v, want 0.50", cfg.SkipThreshold)
	}
	if cfg.SymlinkPolicy != "ask" {
		t.Errorf("SymlinkPolicy = %q, want %q", cfg.SymlinkPolicy, "ask")
	}
	if cfg.ExcludeDirs != nil {
		t.Errorf("ExcludeDirs = %v, want nil", cfg.ExcludeDirs)
	}
}

func TestAnalysisResult_AllItems(t *testing.T) {
	t.Parallel()

	r := &AnalysisResult{
		Auto: []*DetectedItem{
			{Name: "auto-1", Type: catalog.Skills},
			{Name: "auto-2", Type: catalog.Rules},
		},
		Confirm: []*DetectedItem{
			{Name: "confirm-1", Type: catalog.Hooks},
		},
	}

	all := r.AllItems()
	if len(all) != 3 {
		t.Fatalf("AllItems() len = %d, want 3", len(all))
	}
	// Auto items come first.
	if all[0].Name != "auto-1" {
		t.Errorf("AllItems()[0].Name = %q, want %q", all[0].Name, "auto-1")
	}
	if all[1].Name != "auto-2" {
		t.Errorf("AllItems()[1].Name = %q, want %q", all[1].Name, "auto-2")
	}
	if all[2].Name != "confirm-1" {
		t.Errorf("AllItems()[2].Name = %q, want %q", all[2].Name, "confirm-1")
	}
}

func TestAnalysisResult_CountByType(t *testing.T) {
	t.Parallel()

	r := &AnalysisResult{
		Auto: []*DetectedItem{
			{Name: "skill-1", Type: catalog.Skills},
			{Name: "skill-2", Type: catalog.Skills},
		},
		Confirm: []*DetectedItem{
			{Name: "hook-1", Type: catalog.Hooks},
			{Name: "rule-1", Type: catalog.Rules},
		},
	}

	counts := r.CountByType()
	if counts[catalog.Skills] != 2 {
		t.Errorf("CountByType[Skills] = %d, want 2", counts[catalog.Skills])
	}
	if counts[catalog.Hooks] != 1 {
		t.Errorf("CountByType[Hooks] = %d, want 1", counts[catalog.Hooks])
	}
	if counts[catalog.Rules] != 1 {
		t.Errorf("CountByType[Rules] = %d, want 1", counts[catalog.Rules])
	}
}

func TestAnalysisResult_IsEmpty(t *testing.T) {
	t.Parallel()

	empty := &AnalysisResult{}
	if !empty.IsEmpty() {
		t.Error("IsEmpty() = false for empty result, want true")
	}

	autoOnly := &AnalysisResult{
		Auto: []*DetectedItem{{Name: "x", Type: catalog.Skills}},
	}
	if autoOnly.IsEmpty() {
		t.Error("IsEmpty() = true with Auto items, want false")
	}

	confirmOnly := &AnalysisResult{
		Confirm: []*DetectedItem{{Name: "y", Type: catalog.Hooks}},
	}
	if confirmOnly.IsEmpty() {
		t.Error("IsEmpty() = true with Confirm items, want false")
	}
}
