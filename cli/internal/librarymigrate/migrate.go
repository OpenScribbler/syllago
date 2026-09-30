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
				if err := moveFolder(oldDir, newDir, home, storePath, ct == catalog.Hooks); err != nil {
					errs = append(errs, err)
					continue
				}
				movedNames = append(movedNames, filepath.Join(string(ct), old)+" -> "+filepath.Join(string(ct), prov.Slug))
			}
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
// servers sit either directly under mcp/ or grouped by provider, so, as in
// the catalog scanner, mcp/<slug>/ counts only when it has no config.json of
// its own and holds server directories that do. Other universal types have
// no provider folders.
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
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
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

// moveFolder renames oldDir to newDir and repoints every symlink and
// install record that referenced oldDir. The symlinks come from a scan of
// the global provider directories plus the symlink placements the install
// store recorded, which covers installs to configured or custom paths. If
// any repointing fails, it restores the links it changed and renames the
// folder back, so the next command retries from the same state instead of
// finding a moved folder with stale links.
//
// With keepOldPath set, it leaves oldDir as a relative symlink to newDir.
// Hooks need it: applying a loadout writes each hook's command into the
// provider's settings as an absolute path inside the library item, and
// those settings files live in project folders the migration cannot
// enumerate. The catalog scanner skips symlinked provider folders, so the
// link does not surface the items twice.
func moveFolder(oldDir, newDir, home, storePath string, keepOldPath bool) error {
	store, err := installstore.Load(storePath)
	if err != nil {
		return err
	}
	links := map[string]string{} // link path -> old target
	for _, l := range installer.ScanProviderLinks(provider.AllProviders, home, []string{oldDir}) {
		links[l.Path] = l.Target
	}
	for _, rec := range store.Records {
		for _, pl := range rec.Placements {
			if pl.Mechanism != installstore.MechanismSymlink {
				continue
			}
			target, err := readLink(pl.Path)
			if errors.Is(err, fs.ErrPermission) {
				return fmt.Errorf("reading recorded link %s: %w", pl.Path, err)
			}
			if err == nil && within(target, oldDir) {
				links[pl.Path] = target
			}
		}
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	var done []string
	linkedOld := false
	fail := func(err error) error {
		for _, path := range done {
			_ = installer.CreateSymlink(links[path], path)
		}
		if linkedOld {
			_ = os.Remove(oldDir)
		}
		if rbErr := os.Rename(newDir, oldDir); rbErr != nil {
			return fmt.Errorf("%w; restoring %s also failed: %v", err, oldDir, rbErr)
		}
		return err
	}
	for path, target := range links {
		if err := installer.CreateSymlink(rebase(target, oldDir, newDir), path); err != nil {
			return fail(fmt.Errorf("repointing %s: %w", path, err))
		}
		done = append(done, path)
	}
	if keepOldPath {
		if err := os.Symlink(filepath.Base(newDir), oldDir); err != nil {
			return fail(fmt.Errorf("linking %s to %s: %w", oldDir, newDir, err))
		}
		linkedOld = true
	}
	changed := false
	for i := range store.Records {
		rec := &store.Records[i]
		if within(rec.LibraryPath, oldDir) {
			rec.LibraryPath = rebase(rec.LibraryPath, oldDir, newDir)
			changed = true
		}
	}
	if changed {
		if err := store.Save(); err != nil {
			return fail(err)
		}
	}
	return nil
}

func readLink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), nil
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// rebase maps path, which lies inside oldDir, to the same place in newDir.
func rebase(path, oldDir, newDir string) string {
	rel, _ := filepath.Rel(oldDir, path)
	return filepath.Join(newDir, rel)
}

// rewriteLibraryFiles replaces retired provider slugs in the provider fields
// of every metadata and loadout file in the library, and returns how many
// files it changed. A file it cannot parse is left alone; the catalog scan
// reports it.
func rewriteLibraryFiles(libDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		// A symlinked file may point outside the library, so only regular
		// files are rewritten.
		if err != nil || !d.Type().IsRegular() {
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
