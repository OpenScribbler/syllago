package loadout

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

// ValidationResult describes one validation issue.
type ValidationResult struct {
	Ref     ResolvedRef
	Problem string
}

// Validate checks that:
//  1. Each resolved item's type is supported by the target provider (via provider.SupportsType).
//  2. No two refs have the same (type, name) pair (duplicate detection).
//
// Returns a slice of validation results. Empty slice means the loadout is valid.
//
// This is purely about manifest coherence -- it does NOT check filesystem state.
// Conflict detection (symlink already exists, hook already installed) happens in Preview.
func Validate(refs []ResolvedRef, prov provider.Provider) []ValidationResult {
	var results []ValidationResult

	// Check provider support for each type
	for _, ref := range refs {
		if prov.SupportsType != nil && !prov.SupportsType(ref.Type) {
			results = append(results, ValidationResult{
				Ref:     ref,
				Problem: fmt.Sprintf("%s does not support %s", prov.Name, ref.Type.Label()),
			})
		}
	}

	// Check for duplicate (type, name) pairs
	seen := make(map[string]bool)
	for _, ref := range refs {
		key := string(ref.Type) + ":" + ref.Name
		if seen[key] {
			results = append(results, ValidationResult{
				Ref:     ref,
				Problem: fmt.Sprintf("duplicate %s reference: %s", ref.Type.Label(), ref.Name),
			})
		}
		seen[key] = true
	}

	return results
}
