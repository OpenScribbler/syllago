package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/spf13/cobra"
)

// YAMLSchemaManifest wraps the raw SchemaDoc with release provenance so
// downstream consumers can key off a version string.
type YAMLSchemaManifest struct {
	Version        string              `json:"version"`
	GeneratedAt    string              `json:"generated_at"`
	SyllagoVersion string              `json:"syllago_version"`
	Schema         *metadata.SchemaDoc `json:"schema"`
}

// yamlSchemaSourcePath is the path to metadata.go that the AST parser reads.
// Overridable in tests; defaults to the relative path used by `make gendocs`
// when invoked from the cli/ directory.
var yamlSchemaSourcePath = filepath.Join("internal", "metadata", "metadata.go")

var genyamlschemaCmd = &cobra.Command{
	Use:    "_genyamlschema",
	Short:  "Generate syllago-yaml-schema.json manifest",
	Hidden: true,
	RunE:   runGenyamlschema,
}

func init() {
	rootCmd.AddCommand(genyamlschemaCmd)
}

func runGenyamlschema(_ *cobra.Command, _ []string) error {
	doc, err := metadata.BuildSchemaDocFromSource(yamlSchemaSourcePath)
	if err != nil {
		return fmt.Errorf("building schema doc: %w", err)
	}

	v := version
	if v == "" {
		v = "dev"
	}

	manifest := YAMLSchemaManifest{
		Version:        "1",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion: v,
		Schema:         doc,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}
