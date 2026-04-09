package capyaml

// ProviderCapabilities is the Go representation of docs/provider-capabilities/<slug>.yaml.
type ProviderCapabilities struct {
	SchemaVersion   string `yaml:"schema_version"`
	Slug            string `yaml:"slug"`
	DisplayName     string `yaml:"display_name"`
	LastVerified    string `yaml:"last_verified"`
	ProviderVersion string `yaml:"provider_version,omitempty"`
	SourceManifest  string `yaml:"source_manifest,omitempty"`
	FormatReference string `yaml:"format_reference,omitempty"`
	// References is auto-maintained by the pipeline. Maps reference ID → ReferenceEntry.
	References        map[string]ReferenceEntry   `yaml:"references,omitempty"`
	ContentTypes      map[string]ContentTypeEntry `yaml:"content_types"`
	ProviderExclusive map[string]interface{}      `yaml:"provider_exclusive,omitempty"`
}

// ReferenceEntry tracks provenance for a source URL used in capability extraction.
// The pipeline auto-updates VerifiedAt and LastContentHash after each successful fetch.
type ReferenceEntry struct {
	URL             string `yaml:"url"`
	FetchMethod     string `yaml:"fetch_method"`
	VerifiedAt      string `yaml:"verified_at,omitempty"`
	LastContentHash string `yaml:"last_content_hash,omitempty"`
}

// ContentTypeEntry is the generic entry for a content type (hooks, rules, etc.)
type ContentTypeEntry struct {
	Supported    bool                       `yaml:"supported"`
	Confidence   string                     `yaml:"confidence,omitempty"`
	Events       map[string]EventEntry      `yaml:"events,omitempty"`
	Capabilities map[string]CapabilityEntry `yaml:"capabilities,omitempty"`
	Tools        map[string]ToolEntry       `yaml:"tools,omitempty"`
}

// EventEntry is one hook event in a capability YAML.
type EventEntry struct {
	NativeName string   `yaml:"native_name"`
	Blocking   string   `yaml:"blocking,omitempty"`
	Refs       []string `yaml:"refs,omitempty"`
}

// CapabilityEntry is one capability (e.g., structured_output) in a capability YAML.
type CapabilityEntry struct {
	Supported bool     `yaml:"supported"`
	Mechanism string   `yaml:"mechanism,omitempty"`
	Refs      []string `yaml:"refs,omitempty"`
}

// ToolEntry maps a canonical tool name to its provider-native name.
type ToolEntry struct {
	Native string   `yaml:"native"`
	Refs   []string `yaml:"refs,omitempty"`
}
