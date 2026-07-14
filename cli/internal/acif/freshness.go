package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type FreshnessInput struct {
	Record                   map[string]any `json:"record"`
	ConsumerClock            string         `json:"consumer_clock,omitempty"`
	Policies                 []string       `json:"policies,omitempty"`
	AttestationEvaluation    string         `json:"attestation_evaluation,omitempty"`
	DeclaredToleranceSeconds any            `json:"declared_tolerance_seconds,omitempty"`
	AttestationSystem        string         `json:"attestation_system,omitempty"`
	ImplementationBehavior   string         `json:"implementation_behavior,omitempty"`
}

func EvaluateFreshness(input FreshnessInput) (map[string]any, error) {
	if input.ImplementationBehavior == "staleness-from-generated_at" {
		return map[string]any{
			"conformant": false,
			"reason":     ReasonResponseEnvelopeClockNotStalenessInput,
		}, nil
	}

	fetchedAt, ok := parseRecordTime(input.Record, "fetched_at")
	if !ok {
		return timestampOffsetMissingVerdict(), nil
	}
	var expires time.Time
	if rawExpires, ok := input.Record["expires"]; ok {
		expiresString, ok := rawExpires.(string)
		if !ok {
			return timestampOffsetMissingVerdict(), nil
		}
		parsed, err := time.Parse(time.RFC3339, expiresString)
		if err != nil {
			return timestampOffsetMissingVerdict(), nil
		}
		expires = parsed
	} else {
		expires = fetchedAt.Add(72 * time.Hour)
	}

	if expires.Before(fetchedAt) {
		return nil, &RejectError{ID: ErrRegistryExpiresBeforeFetchedAt}
	}

	stale := false
	if input.ConsumerClock != "" {
		consumerClock, err := time.Parse(time.RFC3339, input.ConsumerClock)
		if err != nil {
			return timestampOffsetMissingVerdict(), nil
		}
		tolerance := time.Duration(numberAsInt64(input.DeclaredToleranceSeconds)) * time.Second
		stale = consumerClock.After(expires.Add(-tolerance))
	}

	staleness := "fresh"
	if stale {
		staleness = "stale"
	}
	trustTier := "unattested"
	if input.AttestationEvaluation == "valid" {
		trustTier = "attested"
	}
	install := "proceed"
	if stale && stringSliceContains(input.Policies, "freshness-enforcement-opt-in") {
		install = "refuse"
	}

	result := map[string]any{
		"conformant": true,
		"staleness":  staleness,
		"trust_tier": trustTier,
		"install":    install,
	}
	if stale {
		result["warnings"] = []any{map[string]any{
			"id": ErrRegistryStale,
			"params": map[string]any{
				"expires": expires.Format(time.RFC3339),
			},
		}}
	}
	if err := attachResponseHash(result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseRecordTime(record map[string]any, key string) (time.Time, bool) {
	raw, ok := record[key]
	if !ok {
		return time.Time{}, false
	}
	s, ok := raw.(string)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func timestampOffsetMissingVerdict() map[string]any {
	return map[string]any{
		"conformant": false,
		"reason":     ReasonRegistryTimestampOffsetMissing,
	}
}

func numberAsInt64(raw any) int64 {
	switch v := raw.(type) {
	case nil:
		return 0
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return n
		}
		f, err := v.Float64()
		if err == nil {
			return int64(f)
		}
	}
	return 0
}

func attachResponseHash(result map[string]any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(canonical)
	result["response_hash"] = hex.EncodeToString(sum[:])
	return nil
}
