// Package capmon implements the syllago capability monitor pipeline.
// Four-stage pipeline: fetch → extract → diff → review.
package capmon

import "time"

// Exit class constants for RunManifest.ExitClass.
const (
	ExitClean                 = 0 // All providers extracted, no drift
	ExitDrifted               = 1 // Drift detected, PR/issue opened
	ExitPartialFailure        = 2 // Some providers failed, others succeeded
	ExitInfrastructureFailure = 3 // Chromedp/network/selector broken
	ExitFatal                 = 4 // Config corrupt, schema validation failed
	ExitPaused                = 5 // .capmon-pause sentinel present
)

// SelectorConfig describes how to locate content within a fetched source.
type SelectorConfig struct {
	Primary          string `yaml:"primary"`
	Fallback         string `yaml:"fallback,omitempty"`
	ExpectedContains string `yaml:"expected_contains,omitempty"`
	MinResults       int    `yaml:"min_results,omitempty"`
	UpdatedAt        string `yaml:"updated_at,omitempty"`
}

// FieldValue is an extracted scalar with its SHA-256 fingerprint.
type FieldValue struct {
	Value     string `json:"value"`
	ValueHash string `json:"value_hash"`
}

// ExtractedSource is the Stage 2 output for one source document.
// capmon: pipeline-internal volatile state, no schema_version
type ExtractedSource struct {
	ExtractorVersion string                `json:"extractor_version"`
	Provider         string                `json:"provider"`
	SourceID         string                `json:"source_id"`
	Format           string                `json:"format"`
	ExtractedAt      time.Time             `json:"extracted_at"`
	Partial          bool                  `json:"partial"`
	Fields           map[string]FieldValue `json:"fields"`
	Landmarks        []string              `json:"landmarks"`
}

// FieldChange describes a single field mutation detected in Stage 3.
type FieldChange struct {
	FieldPath string `json:"field_path"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
}

// CapabilityDiff is the Stage 3 output: structured diff + proposed YAML patch.
type CapabilityDiff struct {
	Provider          string        `json:"provider"`
	RunID             string        `json:"run_id"`
	Changes           []FieldChange `json:"changes"`
	StructuralDrift   []string      `json:"structural_drift,omitempty"`
	ProposedYAMLPatch string        `json:"proposed_yaml_patch,omitempty"`
}

// ProviderStatus tracks per-provider pipeline state for the RunManifest.
type ProviderStatus struct {
	FetchStatus    string `json:"fetch_status"`
	ExtractStatus  string `json:"extract_status"`
	DiffStatus     string `json:"diff_status"`
	ActionTaken    string `json:"action_taken"`
	FixtureAgeDays *int   `json:"fixture_age_days"`
}

// RunManifest is write-only observability output — never a pipeline input.
// capmon: never-read-as-input
type RunManifest struct {
	RunID                         string                    `json:"run_id"`
	StartedAt                     time.Time                 `json:"started_at"`
	FinishedAt                    time.Time                 `json:"finished_at"`
	ExitClass                     int                       `json:"exit_class"`
	SourcesAllCached              bool                      `json:"sources_all_cached"`
	Providers                     map[string]ProviderStatus `json:"providers"`
	Warnings                      []string                  `json:"warnings"`
	FingerprintDivergenceWarnings []string                  `json:"fingerprint_divergence_warnings"`
}
