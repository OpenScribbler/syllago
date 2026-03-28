package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	return buf.Bytes()
}

func TestGenproviders(t *testing.T) {
	raw := captureStdout(t, func() {
		if err := genprovidersCmd.RunE(genprovidersCmd, nil); err != nil {
			t.Fatalf("_genproviders failed: %v", err)
		}
	})

	var manifest ProviderManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v\nfirst 200 bytes: %s", err, string(raw[:min(200, len(raw))]))
	}

	// Must have version and content types.
	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if len(manifest.ContentTypes) == 0 {
		t.Error("contentTypes is empty")
	}

	// Must have all providers.
	if len(manifest.Providers) != len(provider.AllProviders) {
		t.Errorf("providers count = %d, want %d", len(manifest.Providers), len(provider.AllProviders))
	}

	// Build a lookup for validation.
	provBySlug := make(map[string]ProviderCapEntry)
	for _, p := range manifest.Providers {
		provBySlug[p.Slug] = p
	}

	// Validate each provider matches the Go definition.
	for _, goProv := range provider.AllProviders {
		t.Run(goProv.Slug, func(t *testing.T) {
			jsonProv, ok := provBySlug[goProv.Slug]
			if !ok {
				t.Fatalf("provider %q missing from JSON output", goProv.Slug)
			}

			if jsonProv.Name != goProv.Name {
				t.Errorf("name = %q, want %q", jsonProv.Name, goProv.Name)
			}
			if jsonProv.ConfigDir != goProv.ConfigDir {
				t.Errorf("configDir = %q, want %q", jsonProv.ConfigDir, goProv.ConfigDir)
			}

			// Verify content type support matches.
			for _, ct := range catalog.AllContentTypes() {
				ctName := string(ct)
				goSupports := goProv.SupportsType != nil && goProv.SupportsType(ct)
				jsonCap, exists := jsonProv.Content[ctName]

				if !exists {
					t.Errorf("content type %q missing from JSON", ctName)
					continue
				}

				if jsonCap.Supported != goSupports {
					t.Errorf("content[%q].supported = %v, Go says %v", ctName, jsonCap.Supported, goSupports)
				}

				if !goSupports {
					continue
				}

				// Verify file format matches.
				if goProv.FileFormat != nil {
					goFormat := string(goProv.FileFormat(ct))
					if jsonCap.FileFormat != goFormat {
						t.Errorf("content[%q].fileFormat = %q, Go says %q", ctName, jsonCap.FileFormat, goFormat)
					}
				}

				// Verify install method.
				if goProv.InstallDir != nil {
					dir := goProv.InstallDir("{home}", ct)
					switch dir {
					case provider.JSONMergeSentinel:
						if jsonCap.InstallMethod != "json-merge" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "json-merge")
						}
					case provider.ProjectScopeSentinel:
						if jsonCap.InstallMethod != "project-scope" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "project-scope")
						}
					case "":
						// No install dir — skip.
					default:
						if jsonCap.InstallMethod != "filesystem" {
							t.Errorf("content[%q].installMethod = %q, want %q", ctName, jsonCap.InstallMethod, "filesystem")
						}
						if jsonCap.InstallPath != dir {
							t.Errorf("content[%q].installPath = %q, want %q", ctName, jsonCap.InstallPath, dir)
						}
					}
				}

				// Verify symlink support.
				if goProv.SymlinkSupport != nil {
					goSymlink := goProv.SymlinkSupport[ct]
					if jsonCap.SymlinkSupport != goSymlink {
						t.Errorf("content[%q].symlinkSupport = %v, Go says %v", ctName, jsonCap.SymlinkSupport, goSymlink)
					}
				}

				// Verify discovery paths count.
				if goProv.DiscoveryPaths != nil {
					goPaths := goProv.DiscoveryPaths("{project}", ct)
					if len(goPaths) != len(jsonCap.DiscoveryPaths) {
						t.Errorf("content[%q].discoveryPaths count = %d, Go has %d", ctName, len(jsonCap.DiscoveryPaths), len(goPaths))
					}
				}
			}
		})
	}
}
