// One-shot migration: walks content/hooks/<provider>/<name>/hook.json, detects
// the pre-fix "whole settings.json dumped as hook.json" corruption, splits it
// via converter.SplitSettingsHooks, and rewrites each directory as one-or-more
// flat hook.json entries.
//
// Usage: go run /tmp/migrate_hooks.go /path/to/content/hooks
//
// Dry run by default; pass --apply to write changes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"gopkg.in/yaml.v3"
)

type syllagoYAML struct {
	FormatVersion  int    `yaml:"format_version"`
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	SourceProvider string `yaml:"source_provider,omitempty"`
	SourceFormat   string `yaml:"source_format,omitempty"`
	SourceType     string `yaml:"source_type,omitempty"`
	HasSource      bool   `yaml:"has_source,omitempty"`
	SourceHash     string `yaml:"source_hash,omitempty"`
	AddedAt        string `yaml:"added_at,omitempty"`
	AddedBy        string `yaml:"added_by,omitempty"`

	// Catch-all for other fields we want to preserve verbatim.
	Extra map[string]any `yaml:",inline"`
}

func main() {
	apply := flag.Bool("apply", false, "actually write changes")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: migrate_hooks [--apply] <hooks-root>")
		os.Exit(2)
	}
	root := flag.Arg(0)

	entries, err := os.ReadDir(root)
	if err != nil {
		die("reading %s: %v", root, err)
	}

	for _, providerDir := range entries {
		if !providerDir.IsDir() {
			continue
		}
		providerSlug := providerDir.Name()
		providerPath := filepath.Join(root, providerSlug)

		hookDirs, err := os.ReadDir(providerPath)
		if err != nil {
			die("reading %s: %v", providerPath, err)
		}
		for _, hd := range hookDirs {
			if !hd.IsDir() {
				continue
			}
			dirPath := filepath.Join(providerPath, hd.Name())
			if err := migrateDir(dirPath, providerSlug, *apply); err != nil {
				fmt.Fprintf(os.Stderr, "  error %s: %v\n", dirPath, err)
			}
		}
	}

	if !*apply {
		fmt.Println("\n(dry run — rerun with --apply to make changes)")
	}
}

func migrateDir(dirPath, providerSlug string, apply bool) error {
	hookPath := filepath.Join(dirPath, "hook.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return err
	}

	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("parsing hook.json: %w", err)
	}

	// Already flat? Skip.
	if _, hasEvent := probe["event"].(string); hasEvent {
		return nil
	}
	// Not a nested settings.json shape? Skip (unknown format).
	if _, hasHooks := probe["hooks"].(map[string]any); !hasHooks {
		fmt.Printf("skip (unknown shape, no hooks map): %s\n", dirPath)
		return nil
	}

	hooks, err := converter.SplitSettingsHooks(data, providerSlug)
	if err != nil {
		return fmt.Errorf("splitting: %w", err)
	}
	if len(hooks) == 0 {
		fmt.Printf("skip (no hooks after split): %s\n", dirPath)
		return nil
	}

	meta, metaErr := readMeta(filepath.Join(dirPath, ".syllago.yaml"))

	providerPath := filepath.Dir(dirPath)
	origName := filepath.Base(dirPath)

	fmt.Printf("\n%s → %d hook(s):\n", dirPath, len(hooks))
	newNames := make([]string, len(hooks))
	for i, h := range hooks {
		derived := converter.DeriveHookName(h)
		// Disambiguate if multiple hooks derive to the same name
		newName := derived
		for j := 0; j < i; j++ {
			if newNames[j] == newName {
				newName = fmt.Sprintf("%s-%d", derived, i+1)
				break
			}
		}
		newNames[i] = newName
		fmt.Printf("  %s (event=%s, matcher=%q, %d action(s))\n",
			newName, h.Event, h.Matcher, len(h.Hooks))
	}

	if !apply {
		return nil
	}

	// Write each split hook into its own directory.
	for i, h := range hooks {
		newDir := filepath.Join(providerPath, newNames[i])
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			return err
		}
		flat, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return err
		}
		flat = append(flat, '\n')
		if err := os.WriteFile(filepath.Join(newDir, "hook.json"), flat, 0o644); err != nil {
			return err
		}
		// Write metadata (fresh ID per split entry when we had multiple; reuse
		// the original when there's exactly one hook so IDs stay stable).
		if err := writeMeta(filepath.Join(newDir, ".syllago.yaml"), meta, metaErr, newNames[i], providerSlug, len(hooks) == 1); err != nil {
			return err
		}
	}

	// If all new names are different from the original, the original directory
	// is obsolete. If one of the new names matches the original, the other
	// siblings were added alongside it — keep the original slot, remove any
	// stale files other than the ones we just wrote.
	keepOriginal := false
	for _, n := range newNames {
		if n == origName {
			keepOriginal = true
			break
		}
	}
	if !keepOriginal {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
		fmt.Printf("  removed %s\n", dirPath)
	}

	return nil
}

func readMeta(path string) (*syllagoYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m syllagoYAML
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func writeMeta(path string, meta *syllagoYAML, origErr error, name, provider string, reuseID bool) error {
	m := syllagoYAML{
		FormatVersion:  1,
		Name:           name,
		Type:           "hooks",
		SourceProvider: provider,
		SourceFormat:   "json",
		SourceType:     "provider",
		HasSource:      false, // source hash no longer valid after split
		AddedBy:        "syllago-migrate",
	}
	if meta != nil && origErr == nil {
		if reuseID {
			m.ID = meta.ID
		}
		if meta.AddedAt != "" {
			m.AddedAt = meta.AddedAt
		}
	}
	if m.ID == "" {
		m.ID = newID()
	}
	out, err := yaml.Marshal(&m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func newID() string {
	// Lightweight ID — random hex, no external uuid dep needed.
	buf := make([]byte, 16)
	if _, err := os.ReadFile("/dev/urandom"); err == nil {
		f, _ := os.Open("/dev/urandom")
		defer func() { _ = f.Close() }()
		_, _ = f.Read(buf)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}

var _ = strings.TrimSpace
