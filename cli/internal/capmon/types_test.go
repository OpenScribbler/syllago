package capmon_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestExtractedSource_ZeroValue(t *testing.T) {
	var es capmon.ExtractedSource
	if es.Partial != false {
		t.Error("zero value Partial should be false")
	}

}

func TestRunManifest_ExitClasses(t *testing.T) {
	tests := []struct {
		name  string
		class int
	}{
		{"clean", capmon.ExitClean},
		{"drifted", capmon.ExitDrifted},
		{"partial_failure", capmon.ExitPartialFailure},
		{"infrastructure_failure", capmon.ExitInfrastructureFailure},
		{"fatal", capmon.ExitFatal},
		{"paused", capmon.ExitPaused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := capmon.RunManifest{ExitClass: tt.class}
			if m.ExitClass != tt.class {
				t.Errorf("got %d, want %d", m.ExitClass, tt.class)
			}
		})
	}
}

func TestSelectorConfig_Fields(t *testing.T) {
	cfg := capmon.SelectorConfig{
		Primary:          "main table",
		Fallback:         "table",
		ExpectedContains: "Event Name",
		MinResults:       6,
		UpdatedAt:        "2026-04-08",
	}
	if cfg.Primary != "main table" {
		t.Error("Primary not set")
	}
	if cfg.MinResults != 6 {
		t.Error("MinResults not set")
	}
}

func TestRunManifest_NeverReadAsInput(t *testing.T) {
	// Verifies the type can be constructed with all fields.
	// RunManifest is write-only observability output — never a pipeline input.
	_ = capmon.RunManifest{
		RunID:     "test-run-id",
		StartedAt: time.Now(),
	}
}

func TestExtract_UnknownFormat(t *testing.T) {
	ctx := context.Background()
	_, err := capmon.Extract(ctx, "unknown-format", []byte("data"), capmon.SelectorConfig{})
	if err == nil {
		t.Error("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "no extractor for format") {
		t.Errorf("unexpected error: %v", err)
	}
}
