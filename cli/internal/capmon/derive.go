package capmon

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// DeriveSeederSpec produces a SeederSpec from a FormatDoc deterministically.
// Identical inputs always produce identical outputs (no timestamps, no UUIDs).
// Content types with status "unsupported" are skipped.
// Returns an error if any canonical_mappings key is not in canonical-keys.yaml.
func DeriveSeederSpec(doc *FormatDoc, canonicalKeysPath string) (*SeederSpec, error) {
	canonicalKeys, err := loadCanonicalKeys(canonicalKeysPath)
	if err != nil {
		return nil, fmt.Errorf("derive seeder spec: %w", err)
	}

	spec := &SeederSpec{
		Provider:    doc.Provider,
		ContentType: "skills",
	}

	// Collect all proposed mappings across content types, skipping unsupported ones.
	// Sort canonical keys for determinism.
	for ct, ctDoc := range doc.ContentTypes {
		if ctDoc.Status == "unsupported" {
			continue
		}

		validKeys := canonicalKeys[ct]

		// Sort canonical mapping keys for deterministic output.
		keys := make([]string, 0, len(ctDoc.CanonicalMappings))
		for k := range ctDoc.CanonicalMappings {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			mapping := ctDoc.CanonicalMappings[key]

			// Unknown key — hard error.
			if validKeys != nil && !validKeys[key] {
				return nil, fmt.Errorf("derive seeder spec for %q: canonical_mappings key %q not in canonical-keys.yaml", doc.Provider, key)
			}

			spec.ProposedMappings = append(spec.ProposedMappings, ProposedMapping{
				CanonicalKey: key,
				Supported:    mapping.Supported,
				Mechanism:    mapping.Mechanism,
				Confidence:   mapping.Confidence,
			})
		}
	}

	return spec, nil
}

// WriteSeederSpec writes a SeederSpec to the given path using an atomic
// write (temp file → rename). This prevents partial writes from leaving
// a corrupt spec file if the process is interrupted.
func WriteSeederSpec(spec *SeederSpec, path string) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal seeder spec: %w", err)
	}

	// Write to a temp file in the same directory so rename is atomic
	// (same filesystem, no cross-device move).
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".seederspec-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}
	return nil
}
