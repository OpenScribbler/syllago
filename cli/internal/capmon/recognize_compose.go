package capmon

// mergeRecognitionResults composes multiple RecognitionResult values into one.
// The composition is content-type-agnostic — it is the standard way for a per-
// provider recognizer that handles two or more content types (skills + rules,
// rules + hooks, …) to produce a single RecognitionResult.
//
// Composition rules:
//
//   - Capabilities maps are merged. Keys do not collide in normal operation
//     because every dot-path begins with a content-type prefix (skills.*,
//     rules.*) — the prefix discipline is enforced upstream by
//     <ContentType>LandmarkOptions and recognizeLandmarks.
//   - MissingAnchors and MatchedAnchors are unioned (de-duplicated, sorted).
//     Anchor names are content-type-agnostic strings; mixing them mirrors
//     the merged Landmarks input the recognizer received.
//   - Status follows a precedence ladder:
//     1. recognized       — any input emitted capabilities (the strongest signal)
//     2. anchors_missing  — no caps but at least one input had missing anchors
//     3. not_evaluated    — no signal at all
//     This matches recognizeLandmarks's per-result decision applied to the
//     union of all inputs.
//
// Empty input returns the zero-value not_evaluated result (with non-nil maps),
// matching the convention used by RecognizeWithContext.
func mergeRecognitionResults(results ...RecognitionResult) RecognitionResult {
	merged := RecognitionResult{
		Capabilities: make(map[string]string),
		Status:       StatusNotEvaluated,
	}
	missingSet := make(map[string]struct{})
	matchedSet := make(map[string]struct{})
	anyAnchorsMissing := false

	for _, r := range results {
		for k, v := range r.Capabilities {
			merged.Capabilities[k] = v
		}
		for _, m := range r.MissingAnchors {
			missingSet[m] = struct{}{}
		}
		for _, m := range r.MatchedAnchors {
			matchedSet[m] = struct{}{}
		}
		if len(r.MissingAnchors) > 0 {
			anyAnchorsMissing = true
		}
	}

	switch {
	case len(merged.Capabilities) > 0:
		merged.Status = StatusRecognized
	case anyAnchorsMissing:
		merged.Status = StatusAnchorsMissing
	}

	merged.MissingAnchors = sortedSetKeys(missingSet)
	merged.MatchedAnchors = sortedSetKeys(matchedSet)
	return merged
}
