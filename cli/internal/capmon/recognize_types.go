package capmon

// RecognitionContext is the read-only input passed to every recognizer.
// Carries extracted fields, landmarks, and source metadata. Recognizers MUST treat
// this struct as read-only and MUST NOT retain references to its slices or maps
// across calls — see the package doc comment for the full conformance contract.
type RecognitionContext struct {
	// Fields is the typed-code field map (e.g., "Skill.Name" → "name") produced by extractors.
	Fields map[string]FieldValue
	// Landmarks is the ordered list of structural anchors (e.g., heading titles, struct type names)
	// produced by extractors. Empty for sources that have no landmark concept.
	Landmarks []string
	// Format is the extractor format identifier ("go", "rust", "typescript", "markdown", "html", …).
	// Recognizers MAY branch on this when they need format-aware behavior, but most do not.
	Format string
	// Provider is the provider slug (e.g., "claude-code"). Present so generic helpers can
	// log without requiring callers to thread a separate parameter.
	Provider string
	// SourceID identifies which extracted source produced these fields when a provider has
	// multiple sources merged into one context. Empty when not applicable.
	SourceID string
	// Partial is true when the upstream extractor flagged this source as incomplete.
	// Recognizers SHOULD downgrade confidence or skip emission when Partial is true.
	Partial bool
}

// RecognitionResult is the structured output of a recognizer.
// Returning a single struct (instead of a bare map) makes every recognizer surface
// the three-state status pipeline ("recognized" | "anchors_missing" | "not_evaluated")
// alongside its capability dot-paths.
//
// Why a struct, not (map, error): recognizers are pure pattern-matching functions —
// there are no recoverable errors. The "did this fire?" signal needs to be surfaced
// to downstream observability without forcing every caller to introspect map size.
type RecognitionResult struct {
	// Capabilities is the dot-path → value map consumed by the YAML writer.
	// Keys MUST match ^[a-z_]+(\.[a-z_]+)*$ (enforced by TestRecognitionConformance_KeyRegex).
	Capabilities map[string]string
	// Status reports the recognizer's pipeline state:
	//   "recognized"      — capabilities emitted (Capabilities non-empty)
	//   "anchors_missing" — required landmark anchors absent; capabilities suppressed
	//   "not_evaluated"   — recognizer ran but produced no signal (e.g., no matching fields)
	Status string
	// MissingAnchors lists the named anchors a landmark-based recognizer required but did
	// not find. Empty for non-landmark recognizers and for "recognized" results.
	MissingAnchors []string
	// MatchedAnchors lists the named anchors a landmark-based recognizer successfully matched.
	// Useful for downstream observability; empty for non-landmark recognizers.
	MatchedAnchors []string
}

// Status constants for RecognitionResult.Status. Use these in helpers and tests
// instead of raw string literals to keep the three-state pipeline grep-able.
const (
	StatusRecognized     = "recognized"
	StatusAnchorsMissing = "anchors_missing"
	StatusNotEvaluated   = "not_evaluated"
)

// RecognizerKind tags the recognition strategy a registered recognizer uses.
// Stored alongside the function in the registry for downstream tooling
// (capmon doctor, drift dashboards) that wants to slice metrics by strategy.
type RecognizerKind int

const (
	// RecognizerKindUnknown is the zero value; a recognizer registered without a kind
	// is treated as Unknown and surfaces a warning in capmon health checks.
	RecognizerKindUnknown RecognizerKind = iota
	// RecognizerKindGoStruct matches typed Go struct fields via StructPrefixes.
	RecognizerKindGoStruct
	// RecognizerKindRustStruct matches typed Rust struct fields via StructPrefixes
	// (or PrefixMatcher for multi-struct allow-lists like codex).
	RecognizerKindRustStruct
	// RecognizerKindTSInterface matches typed TypeScript interface fields via StructPrefixes.
	RecognizerKindTSInterface
	// RecognizerKindDoc matches landmark/heading patterns from documentation-only sources
	// where there is no typed source code to extract.
	RecognizerKindDoc
)

// String returns the lowercase form used in metrics and registry doctor output.
func (k RecognizerKind) String() string {
	switch k {
	case RecognizerKindGoStruct:
		return "gostruct"
	case RecognizerKindRustStruct:
		return "ruststruct"
	case RecognizerKindTSInterface:
		return "tsinterface"
	case RecognizerKindDoc:
		return "doc"
	default:
		return "unknown"
	}
}

// recognizerEntry pairs a registered recognizer function with its declared kind.
type recognizerEntry struct {
	fn   func(RecognitionContext) RecognitionResult
	kind RecognizerKind
}

// wrapCapabilities is the standard helper for recognizers that produce a capability
// map and need to wrap it in a RecognitionResult. Status is "recognized" when the
// map is non-empty, "not_evaluated" when empty. Used by every typed-source recognizer
// (GoStruct/RustStruct/TSInterface) to keep the wrapping uniform.
func wrapCapabilities(m map[string]string) RecognitionResult {
	status := StatusNotEvaluated
	if len(m) > 0 {
		status = StatusRecognized
	}
	return RecognitionResult{Capabilities: m, Status: status}
}
