// Package installer's rule-append path implements D20's byte contract.
package installer

import (
	"errors"
	"io/fs"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
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
