package capmon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// Apply extracted dot-path mappings to the capability struct.
	// Dot-path format: "<content_type>.<section>.<key>.<field>" → value.
	if caps.ContentTypes == nil {
		caps.ContentTypes = make(map[string]capyaml.ContentTypeEntry)
	}
	for path, value := range opts.Extracted {
		parts := strings.SplitN(path, ".", 4)
		if len(parts) < 2 {
			continue
		}
		ct := parts[0]
		ctEntry := caps.ContentTypes[ct]

		switch {
		case len(parts) == 2 && parts[1] == "supported":
			ctEntry.Supported = value == "true"

		case len(parts) == 4 && parts[1] == "capabilities":
			capKey, field := parts[2], parts[3]
			if ctEntry.Capabilities == nil {
				ctEntry.Capabilities = make(map[string]capyaml.CapabilityEntry)
			}
			ce := ctEntry.Capabilities[capKey]
			switch field {
			case "supported":
				ce.Supported = value == "true"
			case "mechanism":
				ce.Mechanism = value
			case "confidence":
				ce.Confidence = value
			}
			ctEntry.Capabilities[capKey] = ce

		case len(parts) == 4 && parts[1] == "events":
			eventKey, field := parts[2], parts[3]
			if ctEntry.Events == nil {
				ctEntry.Events = make(map[string]capyaml.EventEntry)
			}
			ev := ctEntry.Events[eventKey]
			switch field {
			case "native_name":
				ev.NativeName = value
			case "blocking":
				ev.Blocking = value
			}
			ctEntry.Events[eventKey] = ev
		}

		caps.ContentTypes[ct] = ctEntry
	}

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
