package analyzer

// ConfidenceTier classifies a content-signal item's confidence level.
type ConfidenceTier string

const (
	TierLow    ConfidenceTier = "Low confidence"
	TierMedium ConfidenceTier = "Medium confidence"
	TierHigh   ConfidenceTier = "High confidence"
	TierUser   ConfidenceTier = "User-asserted, no content signals"
)

// TierForItem returns the confidence tier label for a DetectedItem.
// User-directed items with zero content signals get a special label.
func TierForItem(item *DetectedItem) ConfidenceTier {
	// User-directed zero-signal items: base(0.40) + boost(0.20) = 0.60 exactly.
	if item.Provider == "content-signal" && item.InternalLabel == "content-signal" && item.Confidence == 0.60 {
		return TierUser
	}
	switch {
	case item.Confidence < 0.60:
		return TierLow
	case item.Confidence < 0.70:
		return TierMedium
	default:
		return TierHigh
	}
}
