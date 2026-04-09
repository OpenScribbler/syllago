package capmon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
	"gopkg.in/yaml.v3"
)

// SeedOptions configures a SeedProviderCapabilities invocation.
type SeedOptions struct {
	CapsDir                 string
	Provider                string
	Extracted               map[string]string // field path → value from extraction (Phase 9 wires full mapping)
	ForceOverwriteExclusive bool
}

// SeedProviderCapabilities creates or updates docs/provider-capabilities/<provider>.yaml
// from extracted data. It is idempotent: if the file already exists, extracted fields
// are merged in. provider_exclusive entries are preserved unconditionally unless
// ForceOverwriteExclusive is set.
func SeedProviderCapabilities(opts SeedOptions) error {
	path := filepath.Join(opts.CapsDir, opts.Provider+".yaml")

	var caps capyaml.ProviderCapabilities
	existing, err := capyaml.LoadCapabilityYAML(path)
	if err == nil {
		caps = *existing
		if opts.ForceOverwriteExclusive {
			caps.ProviderExclusive = nil
			fmt.Printf("WARNING: --force-overwrite-exclusive cleared provider_exclusive for %s\n", opts.Provider)
		}
		// Without ForceOverwriteExclusive, ProviderExclusive is preserved from existing file
	} else {
		// New file
		caps = capyaml.ProviderCapabilities{
			SchemaVersion: "1",
			Slug:          opts.Provider,
		}
	}

	// Merge extracted fields — full field path → YAML mapping implemented in Phase 9.
	// For now the extracted data is stored but not yet applied to the typed struct.
	_ = opts.Extracted

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(caps); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return enc.Close()
}
