package main

import (
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// TelemetryManifest is the top-level JSON structure output by _gentelemetry.
type TelemetryManifest struct {
	Version            string                   `json:"version"`
	GeneratedAt        string                   `json:"generatedAt"`
	SyllagoVersion     string                   `json:"syllagoVersion"`
	Events             []telemetry.EventDef     `json:"events"`
	StandardProperties []telemetry.PropertyDef  `json:"standardProperties"`
	NeverCollected     []telemetry.PrivacyEntry `json:"neverCollected"`
}

var gentelemetryCmd = &cobra.Command{
	Use:    "_gentelemetry",
	Short:  "Generate telemetry.json manifest",
	Hidden: true,
	RunE:   runGentelemetry,
}

func init() {
	rootCmd.AddCommand(gentelemetryCmd)
}

func runGentelemetry(_ *cobra.Command, _ []string) error {
	v := buildVersionOrDev()

	manifest := TelemetryManifest{
		Version:            "1",
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion:     v,
		Events:             telemetry.EventCatalog(),
		StandardProperties: telemetry.StandardProperties(),
		NeverCollected:     telemetry.NeverCollected(),
	}

	return emitManifestJSON(manifest)
}
