package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
)

// ProviderManifest is the top-level JSON structure output by _genproviders.
type ProviderManifest struct {
	Version        string             `json:"version"`
	GeneratedAt    string             `json:"generatedAt"`
	SyllagoVersion string             `json:"syllagoVersion"`
	Providers      []ProviderCapEntry `json:"providers"`
	ContentTypes   []string           `json:"contentTypes"`
}

// ProviderCapEntry represents a single provider's full capability data.
type ProviderCapEntry struct {
	Name      string                       `json:"name"`
	Slug      string                       `json:"slug"`
	ConfigDir string                       `json:"configDir"`
	Content   map[string]ContentCapability `json:"content"`
}

// ContentCapability describes a provider's capability for one content type.
type ContentCapability struct {
	Supported      bool     `json:"supported"`
	FileFormat     string   `json:"fileFormat,omitempty"`
	InstallMethod  string   `json:"installMethod,omitempty"` // filesystem | json-merge | project-scope
	InstallPath    string   `json:"installPath,omitempty"`   // template with {home}
	SymlinkSupport bool     `json:"symlinkSupport"`
	DiscoveryPaths []string `json:"discoveryPaths,omitempty"` // templates with {project}, {home}
}

var genprovidersCmd = &cobra.Command{
	Use:    "_genproviders",
	Short:  "Generate providers.json manifest",
	Hidden: true,
	RunE:   runGenproviders,
}

func init() {
	rootCmd.AddCommand(genprovidersCmd)
}

func runGenproviders(_ *cobra.Command, _ []string) error {
	var entries []ProviderCapEntry

	for _, prov := range provider.AllProviders {
		entries = append(entries, buildProviderEntry(prov))
	}

	v := version
	if v == "" {
		v = "dev"
	}

	var ctNames []string
	for _, ct := range catalog.AllContentTypes() {
		ctNames = append(ctNames, string(ct))
	}

	manifest := ProviderManifest{
		Version:        "1",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion: v,
		Providers:      entries,
		ContentTypes:   ctNames,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

func buildProviderEntry(prov provider.Provider) ProviderCapEntry {
	content := make(map[string]ContentCapability)

	for _, ct := range catalog.AllContentTypes() {
		content[string(ct)] = buildContentCap(prov, ct)
	}

	return ProviderCapEntry{
		Name:      prov.Name,
		Slug:      prov.Slug,
		ConfigDir: prov.ConfigDir,
		Content:   content,
	}
}

func buildContentCap(prov provider.Provider, ct catalog.ContentType) ContentCapability {
	supported := prov.SupportsType != nil && prov.SupportsType(ct)
	if !supported {
		return ContentCapability{Supported: false}
	}

	cap := ContentCapability{
		Supported: true,
	}

	// File format.
	if prov.FileFormat != nil {
		cap.FileFormat = string(prov.FileFormat(ct))
	}

	// Install method and path.
	if prov.InstallDir != nil {
		dir := prov.InstallDir("{home}", ct)
		switch dir {
		case provider.JSONMergeSentinel:
			cap.InstallMethod = "json-merge"
		case provider.ProjectScopeSentinel:
			cap.InstallMethod = "project-scope"
		case "":
			// Not supported via install (shouldn't happen if SupportsType is true).
			cap.InstallMethod = ""
		default:
			cap.InstallMethod = "filesystem"
			cap.InstallPath = dir
		}
	}

	// Symlink support.
	if prov.SymlinkSupport != nil {
		cap.SymlinkSupport = prov.SymlinkSupport[ct]
	}

	// Discovery paths.
	if prov.DiscoveryPaths != nil {
		paths := prov.DiscoveryPaths("{project}", ct)
		if len(paths) > 0 {
			cap.DiscoveryPaths = paths
		}
	}

	return cap
}
