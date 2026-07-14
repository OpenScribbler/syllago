package acif

import (
	"reflect"
	"testing"
)

func TestEvaluateFreshnessBattery(t *testing.T) {
	t.Parallel()

	baseRecord := map[string]any{
		"fetched_at": "2026-05-01T00:00:00Z",
		"expires":    "2026-05-02T00:00:00Z",
	}

	fresh, err := EvaluateFreshness(FreshnessInput{
		Record:                cloneMap(baseRecord),
		ConsumerClock:         "2026-05-01T12:00:00Z",
		AttestationEvaluation: "valid",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(fresh) error: %v", err)
	}
	if fresh["conformant"] != true || fresh["staleness"] != "fresh" || fresh["trust_tier"] != "attested" || fresh["install"] != "proceed" {
		t.Fatalf("fresh result = %#v", fresh)
	}
	if fresh["response_hash"] == "" {
		t.Fatalf("fresh response_hash missing: %#v", fresh)
	}
	if _, ok := fresh["warnings"]; ok {
		t.Fatalf("fresh warnings present: %#v", fresh)
	}
	if _, ok := fresh["combined_scalar"]; ok {
		t.Fatalf("combined_scalar present: %#v", fresh)
	}

	stale, err := EvaluateFreshness(FreshnessInput{
		Record:        cloneMap(baseRecord),
		ConsumerClock: "2026-05-02T00:00:01Z",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(stale) error: %v", err)
	}
	if stale["staleness"] != "stale" || stale["trust_tier"] != "unattested" || stale["install"] != "proceed" {
		t.Fatalf("stale result = %#v", stale)
	}
	wantWarnings := []any{map[string]any{
		"id":     ErrRegistryStale,
		"params": map[string]any{"expires": "2026-05-02T00:00:00Z"},
	}}
	if !reflect.DeepEqual(stale["warnings"], wantWarnings) {
		t.Fatalf("stale warnings = %#v, want %#v", stale["warnings"], wantWarnings)
	}

	noClockA, err := EvaluateFreshness(FreshnessInput{
		Record:            cloneMap(baseRecord),
		AttestationSystem: "system-a",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(no clock a) error: %v", err)
	}
	noClockB, err := EvaluateFreshness(FreshnessInput{
		Record:            cloneMap(baseRecord),
		AttestationSystem: "system-b",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(no clock b) error: %v", err)
	}
	if noClockA["staleness"] != "fresh" || !reflect.DeepEqual(noClockA, noClockB) {
		t.Fatalf("attestation_system or absent clock changed result:\na=%#v\nb=%#v", noClockA, noClockB)
	}

	defaultFresh, err := EvaluateFreshness(FreshnessInput{
		Record:        map[string]any{"fetched_at": "2026-05-01T00:00:00Z"},
		ConsumerClock: "2026-05-03T23:00:00Z",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(default fresh) error: %v", err)
	}
	defaultStale, err := EvaluateFreshness(FreshnessInput{
		Record:        map[string]any{"fetched_at": "2026-05-01T00:00:00Z"},
		ConsumerClock: "2026-05-04T01:00:00Z",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(default stale) error: %v", err)
	}
	if defaultFresh["staleness"] != "fresh" || defaultStale["staleness"] != "stale" {
		t.Fatalf("default 72h results: fresh=%#v stale=%#v", defaultFresh, defaultStale)
	}

	_, err = EvaluateFreshness(FreshnessInput{
		Record: map[string]any{
			"fetched_at": "2026-05-02T00:00:00Z",
			"expires":    "2026-05-01T00:00:00Z",
		},
	})
	assertRejectID(t, err, ErrRegistryExpiresBeforeFetchedAt)

	missingOffset, err := EvaluateFreshness(FreshnessInput{
		Record: map[string]any{"fetched_at": "2026-05-01T00:00:00"},
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(missing offset) error: %v", err)
	}
	if missingOffset["conformant"] != false || missingOffset["reason"] != ReasonRegistryTimestampOffsetMissing {
		t.Fatalf("missing offset verdict = %#v", missingOffset)
	}

	tolerance, err := EvaluateFreshness(FreshnessInput{
		Record:                   cloneMap(baseRecord),
		ConsumerClock:            "2026-05-01T23:59:45Z",
		DeclaredToleranceSeconds: 30,
		AttestationEvaluation:    "lapsed",
		ImplementationBehavior:   "",
		Policies:                 nil,
		AttestationSystem:        "ignored",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(tolerance) error: %v", err)
	}
	if tolerance["staleness"] != "stale" || tolerance["trust_tier"] != "unattested" {
		t.Fatalf("tolerance result = %#v", tolerance)
	}

	enforced, err := EvaluateFreshness(FreshnessInput{
		Record:        cloneMap(baseRecord),
		ConsumerClock: "2026-05-03T00:00:00Z",
		Policies:      []string{"freshness-enforcement-opt-in"},
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(enforced) error: %v", err)
	}
	if enforced["install"] != "refuse" {
		t.Fatalf("enforced install = %#v", enforced)
	}

	generatedIgnored, err := EvaluateFreshness(FreshnessInput{
		Record: map[string]any{
			"fetched_at":    "2026-05-01T00:00:00Z",
			"expires":       "2026-05-02T00:00:00Z",
			"generated_at":  "1999-01-01T00:00:00Z",
			"max_staleness": "PT1S",
			"body_hash":     "unchanged",
		},
		ConsumerClock: "2026-05-01T00:01:00Z",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(generated ignored) error: %v", err)
	}
	if generatedIgnored["staleness"] != "fresh" {
		t.Fatalf("generated_at/max_staleness affected staleness: %#v", generatedIgnored)
	}

	generatedBehavior, err := EvaluateFreshness(FreshnessInput{
		Record:                 cloneMap(baseRecord),
		ImplementationBehavior: "staleness-from-generated_at",
	})
	if err != nil {
		t.Fatalf("EvaluateFreshness(generated behavior) error: %v", err)
	}
	if generatedBehavior["conformant"] != false || generatedBehavior["reason"] != ReasonResponseEnvelopeClockNotStalenessInput {
		t.Fatalf("generated behavior verdict = %#v", generatedBehavior)
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
