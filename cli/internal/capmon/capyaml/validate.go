package capyaml

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// supportedSchemaVersions lists all accepted schema_version values.
// The first entry is current; the second (if present) is the previous version
// accepted during migration windows.
var supportedSchemaVersions = []string{"1"}

// ValidateAgainstSchema validates a capability YAML file against the schema version policy.
// If migrationWindow is true, the immediately previous schema version (index 1 in
// supportedSchemaVersions) is also accepted. Returns an error if schema_version is
// unknown or the file cannot be parsed.
func ValidateAgainstSchema(path string, migrationWindow bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var header struct {
		SchemaVersion string `yaml:"schema_version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("parse schema_version from %s: %w", path, err)
	}
	accepted := make(map[string]bool)
	accepted[supportedSchemaVersions[0]] = true
	if migrationWindow && len(supportedSchemaVersions) > 1 {
		accepted[supportedSchemaVersions[1]] = true
	}
	if !accepted[header.SchemaVersion] {
		return fmt.Errorf("unknown schema_version %q in %s: supported versions are %v",
			header.SchemaVersion, path, supportedSchemaVersions)
	}
	// Full struct parse to catch type errors
	var caps ProviderCapabilities
	if err := yaml.Unmarshal(data, &caps); err != nil {
		return fmt.Errorf("validate %s: %w", path, err)
	}
	return nil
}
