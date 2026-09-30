// Package librarymigrate moves library content stored under a retired
// provider slug to the provider's current slug.
package librarymigrate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Run migrates the library at libDir away from retired provider slugs. It
// renames each <type>/<retired>/ folder to <type>/<current>/ when no
// <type>/<current>/ sibling exists, repoints the provider-directory
// symlinks under home and the install records in storePath that referenced
// the old folder, and rewrites retired slugs in the library's metadata and
// loadout files. When both folders exist it leaves them and warns. It
// writes one notice line to w when it changes anything, and does nothing
// on a library that holds no retired slug, so it is safe to run on every
// command.
func Run(libDir, home, storePath string, w io.Writer) error {
	if info, err := os.Stat(libDir); err != nil || !info.IsDir() {
		return nil
	}

	var errs []error
	moved := map[string]string{} // old folder -> new folder
	var movedNames []string
	for _, prov := range provider.AllProviders {
		for _, old := range catalog.RetiredProviderSlugs(prov.Slug) {
			for _, ct := range catalog.AllContentTypes() {
				oldDir := filepath.Join(libDir, string(ct), old)
				newDir := filepath.Join(libDir, string(ct), prov.Slug)
				if !isProviderFolder(ct, oldDir) {
					continue
				}
				if _, err := os.Lstat(newDir); err == nil {
					fmt.Fprintf(w, "warning: library has both %s and %s; move the items from the first into the second, then delete the first\n", oldDir, newDir)
					continue
				}
				links := installer.ScanProviderLinks(provider.AllProviders, home, []string{oldDir})
				if err := os.Rename(oldDir, newDir); err != nil {
					errs = append(errs, err)
					continue
				}
				moved[oldDir] = newDir
				movedNames = append(movedNames, filepath.Join(string(ct), old)+" -> "+filepath.Join(string(ct), prov.Slug))
				for _, link := range links {
					if err := installer.CreateSymlink(movedPath(link.Target, moved), link.Path); err != nil {
						errs = append(errs, fmt.Errorf("repointing %s: %w", link.Path, err))
					}
				}
			}
		}
	}

	if len(moved) > 0 {
		if err := repointRecords(storePath, moved); err != nil {
			errs = append(errs, err)
		}
	}

	rewritten, err := rewriteLibraryFiles(libDir)
	if err != nil {
		errs = append(errs, err)
	}

	var parts []string
	parts = append(parts, movedNames...)
	if rewritten > 0 {
		parts = append(parts, fmt.Sprintf("%d metadata and loadout files updated", rewritten))
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "notice: migrated library content to renamed provider slugs: %s\n", strings.Join(parts, "; "))
	}
	return errors.Join(errs...)
}

// isProviderFolder reports whether dir is a provider folder of content type
// ct. Provider-specific types keep every item under a provider folder. MCP
// servers sit either directly under mcp/ or grouped by provider, so
// mcp/<slug>/ counts only when it holds server directories, and not when it
// is a server that happens to share the slug's name. Other universal types
// have no provider folders.
func isProviderFolder(ct catalog.ContentType, dir string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	if !ct.IsUniversal() {
		return true
	}
	if ct != catalog.MCP {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "config.json")); err == nil {
				return true
			}
		}
	}
	return false
}

// movedPath maps a path inside a moved folder to its new location.
func movedPath(path string, moved map[string]string) string {
	for oldDir, newDir := range moved {
		if rel, err := filepath.Rel(oldDir, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.Join(newDir, rel)
		}
	}
	return path
}

// repointRecords rewrites install-record library paths that pointed into a
// moved folder.
func repointRecords(storePath string, moved map[string]string) error {
	store, err := installstore.Load(storePath)
	if err != nil {
		return err
	}
	changed := false
	for i := range store.Records {
		rec := &store.Records[i]
		if p := movedPath(rec.LibraryPath, moved); p != rec.LibraryPath {
			rec.LibraryPath = p
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return store.Save()
}

// rewriteLibraryFiles replaces retired provider slugs in the provider fields
// of every metadata and loadout file in the library, and returns how many
// files it changed. A file it cannot parse is left alone; the catalog scan
// reports it.
func rewriteLibraryFiles(libDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		var fields func(root *yaml.Node) bool
		name := d.Name()
		switch {
		case name == "loadout.yaml":
			fields = rewriteLoadoutSlugs
		case strings.HasPrefix(name, ".syllago.") && strings.HasSuffix(name, ".yaml"):
			fields = rewriteMetadataSlugs
		default:
			return nil
		}
		changed, err := rewriteYAML(path, fields)
		if err != nil {
			return err
		}
		if changed {
			count++
		}
		return nil
	})
	return count, err
}

// rewriteYAML applies fields to the top-level mapping of the YAML file at
// path and writes the file back when fields reports a change.
func rewriteYAML(path string, fields func(root *yaml.Node) bool) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !mentionsRetiredSlug(data) {
		return false, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	if !fields(doc.Content[0]) {
		return false, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(&doc); err != nil {
		return false, fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return false, fmt.Errorf("encoding %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, buf.Bytes(), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// mentionsRetiredSlug is a cheap filter that skips parsing files that
// cannot hold a retired slug.
func mentionsRetiredSlug(data []byte) bool {
	for _, prov := range provider.AllProviders {
		for _, old := range catalog.RetiredProviderSlugs(prov.Slug) {
			if bytes.Contains(data, []byte(old)) {
				return true
			}
		}
	}
	return false
}

// rewriteMetadataSlugs handles both metadata shapes: the flat
// source_provider field and the rule shape's nested source.provider.
func rewriteMetadataSlugs(root *yaml.Node) bool {
	changed := resolveScalar(mapValue(root, "source_provider"))
	if src := mapValue(root, "source"); src != nil && src.Kind == yaml.MappingNode {
		changed = resolveScalar(mapValue(src, "provider")) || changed
	}
	return changed
}

// rewriteLoadoutSlugs handles a loadout manifest's provider and providers
// fields.
func rewriteLoadoutSlugs(root *yaml.Node) bool {
	changed := resolveScalar(mapValue(root, "provider"))
	if seq := mapValue(root, "providers"); seq != nil && seq.Kind == yaml.SequenceNode {
		for _, n := range seq.Content {
			changed = resolveScalar(n) || changed
		}
	}
	return changed
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func resolveScalar(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	c, aliased := catalog.ResolveProviderSlug(n.Value)
	if !aliased {
		return false
	}
	n.Value = c
	return true
}
