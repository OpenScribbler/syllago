package capmon_test

import (
	"context"
	"encoding/json"
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

func TestHealEvent_JSONRoundTrip(t *testing.T) {
	// HealEvent is persisted in the run manifest. Verify CandidateOutcomes
	// survives JSON marshal/unmarshal so reviewers see the full diagnostic
	// history when reading capmon-run.json.
	original := capmon.HealEvent{
		ContentType: "skills",
		SourceIndex: 0,
		OldURL:      "https://example.com/old.md",
		Success:     false,
		FailReason:  "2 candidates: 2 http_error",
		CandidateOutcomes: []capmon.CandidateOutcome{
			{URL: "https://example.com/v1.md", Strategy: "variant", Outcome: capmon.OutcomeHTTPError, StatusCode: 404, Detail: "status 404"},
			{URL: "https://example.com/v2.md", Strategy: "variant", Outcome: capmon.OutcomeHTTPError, StatusCode: 410, Detail: "status 410"},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"candidate_outcomes"`) {
		t.Errorf("JSON missing candidate_outcomes key:\n%s", data)
	}
	var round capmon.HealEvent
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(round.CandidateOutcomes) != 2 {
		t.Fatalf("CandidateOutcomes len = %d, want 2", len(round.CandidateOutcomes))
	}
	if round.CandidateOutcomes[0].URL != original.CandidateOutcomes[0].URL {
		t.Errorf("URL[0] = %q, want %q", round.CandidateOutcomes[0].URL, original.CandidateOutcomes[0].URL)
	}
	if round.CandidateOutcomes[1].StatusCode != 410 {
		t.Errorf("StatusCode[1] = %d, want 410", round.CandidateOutcomes[1].StatusCode)
	}
}

func TestHealEvent_OmitEmptyCandidateOutcomes(t *testing.T) {
	// When no candidates were probed (e.g. healing disabled), CandidateOutcomes
	// must be omitted from JSON to keep run manifests lean.
	evt := capmon.HealEvent{
		ContentType: "skills",
		SourceIndex: 0,
		OldURL:      "https://example.com/old.md",
		Success:     true,
		NewURL:      "https://example.com/new.md",
		Strategy:    "redirect",
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "candidate_outcomes") {
		t.Errorf("expected candidate_outcomes to be omitted; got:\n%s", data)
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
