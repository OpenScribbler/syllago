package rulestore

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// WriteRule creates a new rule directory under contentRoot/<sourceProvider>/<slug>
// with rule.md + .syllago.yaml + .history/<algo>-<hex>.md per D11. Body is
// normalized once (D12) and that normalized form is what is hashed, written
// to rule.md, and written to .history. meta.Versions and meta.CurrentVersion
// are overwritten by this call to match the produced hash.
func WriteRule(contentRoot, sourceProvider, slug string, meta metadata.RuleMetadata, body []byte) error {
	canon := canonical.Normalize(body)
	hash := HashBody(canon)
	dir := filepath.Join(contentRoot, sourceProvider, slug)
	if err := os.MkdirAll(filepath.Join(dir, ".history"), 0755); err != nil {
		return err
	}
	// rule.md — canonical body, mirrored into .history/ per D11.
	if err := os.WriteFile(filepath.Join(dir, "rule.md"), canon, 0644); err != nil {
		return err
	}
	// .history/<algo>-<hex>.md — byte-equal to rule.md so scan has one code path.
	if err := os.WriteFile(filepath.Join(dir, ".history", hashToFilename(hash)), canon, 0644); err != nil {
		return err
	}
	// .syllago.yaml — versions + current_version are authoritative here.
	meta.Versions = []metadata.RuleVersionEntry{{Hash: hash, WrittenAt: time.Now().UTC()}}
	meta.CurrentVersion = hash
	if meta.FormatVersion == 0 {
		meta.FormatVersion = metadata.CurrentFormatVersion
	}
	if meta.Type == "" {
		meta.Type = "rule"
	}
	data, err := yaml.Marshal(&meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, metadata.FileName), data, 0644)
}
