package capmon

// DiffProviderCapabilities compares extracted fields against the current capability YAML values.
// current is a map of dot-delimited field paths to their string values from the YAML.
func DiffProviderCapabilities(provider, runID string, extracted *ExtractedSource, current map[string]string) CapabilityDiff {
	diff := CapabilityDiff{
		Provider: provider,
		RunID:    runID,
	}
	for fieldPath, newFV := range extracted.Fields {
		oldVal, exists := current[fieldPath]
		if !exists {
			// New field — structural addition
			diff.StructuralDrift = append(diff.StructuralDrift, fieldPath)
			continue
		}
		if oldVal != newFV.Value {
			diff.Changes = append(diff.Changes, FieldChange{
				FieldPath: fieldPath,
				OldValue:  oldVal,
				NewValue:  newFV.Value,
			})
		}
	}
	// Check for removed fields (in current but not in extracted)
	for fieldPath := range current {
		if _, ok := extracted.Fields[fieldPath]; !ok {
			diff.StructuralDrift = append(diff.StructuralDrift, "removed: "+fieldPath)
		}
	}
	return diff
}
