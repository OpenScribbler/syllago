package capyaml

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadCapabilityYAML parses a provider capability YAML file.
func LoadCapabilityYAML(path string) (*ProviderCapabilities, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var caps ProviderCapabilities
	if err := yaml.Unmarshal(data, &caps); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &caps, nil
}

// WriteCapabilityYAML serializes a ProviderCapabilities to the writer.
// provider_exclusive is preserved as-is (round-trip safe via map[string]interface{}).
func WriteCapabilityYAML(w io.Writer, caps *ProviderCapabilities) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(caps); err != nil {
		return fmt.Errorf("encode capability YAML: %w", err)
	}
	return enc.Close()
}
