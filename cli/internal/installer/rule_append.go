// Package installer's rule-append path implements D20's byte contract.
package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

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
	return SaveInstalled(projectRoot, inst)
}
