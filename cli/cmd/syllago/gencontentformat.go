package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/contentformat"
	"github.com/spf13/cobra"
)

// ContentFormatManifest is the top-level structure of content-format.json.
// Downstream consumers (primarily syllago-docs) fetch this artifact to
// codegen reference tables and eliminate manual-maintenance drift.
type ContentFormatManifest struct {
	Version        string             `json:"version"`
	GeneratedAt    string             `json:"generated_at"`
	SyllagoVersion string             `json:"syllago_version"`
	Enums          ContentFormatEnums `json:"enums"`
}

// ContentFormatEnums groups the canonical enum lists.
// Field order is stable; new enums should be appended, not interleaved.
type ContentFormatEnums struct {
	Effort           []string `json:"effort"`
	PermissionMode   []string `json:"permission_mode"`
	SourceType       []string `json:"source_type"`
	SourceVisibility []string `json:"source_visibility"`
	SourceScope      []string `json:"source_scope"`
	ContentType      []string `json:"content_type"`
	HookHandlerType  []string `json:"hook_handler_type"`
}

var gencontentformatCmd = &cobra.Command{
	Use:    "_gencontentformat",
	Short:  "Generate content-format.json manifest",
	Hidden: true,
	RunE:   runGencontentformat,
}

func init() {
	rootCmd.AddCommand(gencontentformatCmd)
}

func runGencontentformat(_ *cobra.Command, _ []string) error {
	v := version
	if v == "" {
		v = "dev"
	}

	manifest := ContentFormatManifest{
		Version:        "1",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion: v,
		Enums: ContentFormatEnums{
			Effort:           contentformat.Effort,
			PermissionMode:   contentformat.PermissionMode,
			SourceType:       contentformat.SourceType,
			SourceVisibility: contentformat.SourceVisibility,
			SourceScope:      contentformat.SourceScope,
			ContentType:      contentformat.ContentType,
			HookHandlerType:  contentformat.HookHandlerType,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}
