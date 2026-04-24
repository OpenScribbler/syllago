// Package installer's rule-append path implements D20's byte contract.
package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// cacheInvalidator is an optional hook set by packages that want to be
// notified when a target file's bytes change (e.g., the installcheck mtime
// cache). Kept as a var-hook to avoid an import cycle between installer and
// installcheck. Package installcheck sets this in its init; non-installcheck
// callers see a no-op.
var cacheInvalidator func(target string)

// SetCacheInvalidator registers the hook. Called by installcheck in its init.
func SetCacheInvalidator(fn func(target string)) {
	cacheInvalidator = fn
}

func invalidateCache(target string) {
	if cacheInvalidator != nil {
		cacheInvalidator(target)
	}
}

// AppendRuleToTarget appends canonicalBody to targetFile per D20:
//  1. If target does not exist or is empty, write "\n<canonicalBody>".
//  2. Otherwise ensure target ends with "\n", then append "\n<canonicalBody>".
//
// canonicalBody must already be in D12 canonical form (single trailing \n).
// The function re-normalizes defensively.
func AppendRuleToTarget(targetFile string, canonicalBody []byte) error {
	existing, err := os.ReadFile(targetFile)
	if errors.Is(err, fs.ErrNotExist) {
		existing = nil
	} else if err != nil {
		return err
	}
	cb := canonical.Normalize(canonicalBody)
	var out []byte
	if len(existing) == 0 {
		out = append([]byte{'\n'}, cb...)
	} else {
		if existing[len(existing)-1] != '\n' {
			existing = append(existing, '\n')
		}
		out = append(existing, '\n')
		out = append(out, cb...)
	}
	return os.WriteFile(targetFile, out, 0644)
}

// InstallRuleAppend performs a monolithic-file append install. The library
// rule's current_version bytes are the canonical body (already normalized
// per D11's "files on disk = canonical"). D14 uniqueness: callers must
// route repeats through the update flow (D17); this function assumes
// Fresh state.
func InstallRuleAppend(projectRoot, homeDir, providerSlug, targetFile, source string, rule *rulestore.Loaded) error {
	canonBody := rule.History[rule.Meta.CurrentVersion]
	if canonBody == nil {
		return fmt.Errorf("install: no history entry for current_version %s", rule.Meta.CurrentVersion)
	}
	if err := AppendRuleToTarget(targetFile, canonBody); err != nil {
		return err
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		return err
	}
	inst.RuleAppends = append(inst.RuleAppends, InstalledRuleAppend{
		Name:        rule.Meta.Name,
		LibraryID:   rule.Meta.ID,
		Provider:    providerSlug,
		TargetFile:  targetFile,
		VersionHash: rule.Meta.CurrentVersion,
		Source:      source,
		Scope:       ResolveAppendScope(targetFile, homeDir, projectRoot),
		InstalledAt: time.Now().UTC(),
	})
	invalidateCache(targetFile)
	return SaveInstalled(projectRoot, inst)
}

// UninstallRuleAppend searches targetFile for any historical version's bytes
// (canonical "\n<body>" pattern per D20) and removes the matched range if
// found exactly once. Zero or multiple matches return an error that callers
// should surface as D17 Modified state — actual decision routing lives in
// the Update flow (Phase 7).
func UninstallRuleAppend(projectRoot, libraryID, targetFile string, library map[string]*rulestore.Loaded) error {
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		return err
	}
	idx := inst.FindRuleAppend(libraryID, targetFile)
	if idx < 0 {
		return fmt.Errorf("no rule-append record for %s at %s", libraryID, targetFile)
	}
	// Read target. ENOENT uninstall semantics (D7): silent success + drop record.
	raw, err := os.ReadFile(targetFile)
	if errors.Is(err, fs.ErrNotExist) {
		inst.RemoveRuleAppend(idx)
		invalidateCache(targetFile)
		return SaveInstalled(projectRoot, inst)
	}
	if err != nil {
		// D7: unreadable target errors out and leaves record intact.
		return fmt.Errorf("reading %s: %w", targetFile, err)
	}
	rule := library[libraryID]
	if rule == nil {
		return fmt.Errorf("no library rule for %s", libraryID)
	}
	normalized := canonical.Normalize(raw)
	// Try the full history in reverse order (newest first) — D7 full-history search.
	for i := len(rule.Meta.Versions) - 1; i >= 0; i-- {
		body := rule.History[rule.Meta.Versions[i].Hash]
		pattern := append([]byte{'\n'}, canonical.Normalize(body)...)
		count := bytes.Count(normalized, pattern)
		if count == 1 {
			cut := bytes.Index(normalized, pattern)
			out := append([]byte{}, normalized[:cut]...)
			out = append(out, normalized[cut+len(pattern):]...)
			if err := os.WriteFile(targetFile, out, 0644); err != nil {
				return err
			}
			invalidateCache(targetFile)
			inst.RemoveRuleAppend(idx)
			return SaveInstalled(projectRoot, inst)
		}
		if count > 1 {
			return fmt.Errorf("multiple matches for rule %s in %s — resolve via update flow", libraryID, targetFile)
		}
	}
	return fmt.Errorf("rule %s not found in %s — file may have been edited", libraryID, targetFile)
}
