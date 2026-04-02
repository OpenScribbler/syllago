package analyzer

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// DetectionPattern declares a glob pattern a detector recognizes.
type DetectionPattern struct {
	Glob        string
	ContentType catalog.ContentType
	// InternalLabel overrides ContentType for detector-internal classification.
	// Use for "hook-script", "hook-wiring", "plugin-manifest", "output-style".
	// Empty means use ContentType directly.
	InternalLabel string
	Confidence    float64
}

// DependencyRef references another content item this item depends on.
type DependencyRef struct {
	Registry string
	Type     catalog.ContentType
	Name     string
}

// DetectedItem is the output of a detector's Classify call.
type DetectedItem struct {
	Name          string
	Type          catalog.ContentType
	InternalLabel string // "hook-script", "hook-wiring", etc. Empty = use Type.
	Provider      string
	Path          string // primary file or directory (relative to repoRoot)
	ContentHash   string // SHA-256 of primary file content
	Confidence    float64
	Scripts       []string // referenced script files (relative to repoRoot)
	References    []string // other files needed (relative to repoRoot)
	Dependencies  []DependencyRef
	HookEvent     string
	HookIndex     int
	ConfigSource  string // where wiring was found (e.g., ".claude/settings.json")
	DisplayName   string
	Description   string
	// Providers holds alias paths for deduplicated items — paths of lower-confidence
	// duplicates that were suppressed in favor of this item (same name+type+hash).
	Providers []string
}

// ContentDetector is the interface every provider detector implements.
type ContentDetector interface {
	// ProviderSlug returns the detector's provider identifier.
	ProviderSlug() string
	// Patterns returns the glob patterns this detector recognizes.
	Patterns() []DetectionPattern
	// Classify inspects a candidate path and returns detected items.
	// Returns nil if the path matched a pattern but content inspection
	// shows it is not actually content.
	// repoRoot is filepath.EvalSymlinks-resolved before being passed in.
	Classify(path string, repoRoot string) ([]*DetectedItem, error)
}

// ConfidenceCategory partitions items for UI presentation.
type ConfidenceCategory int

const (
	CategoryAuto    ConfidenceCategory = iota // > autoThreshold
	CategoryConfirm                           // >= skipThreshold and <= autoThreshold
	CategorySkip                              // < skipThreshold (never included in manifest)
)

// DefaultAutoThreshold is the default minimum confidence for auto-detection.
const DefaultAutoThreshold = 0.80

// DefaultSkipThreshold is the default minimum confidence to include at all.
const DefaultSkipThreshold = 0.50

// AnalysisResult holds the output of a full repository analysis.
type AnalysisResult struct {
	Auto     []*DetectedItem // confidence > AutoThreshold (excluding hooks/MCP)
	Confirm  []*DetectedItem // confidence in [SkipThreshold, AutoThreshold], plus ALL hooks/MCP
	Warnings []string
}

// AnalysisConfig controls analyzer behavior.
type AnalysisConfig struct {
	AutoThreshold float64                        // default DefaultAutoThreshold
	SkipThreshold float64                        // default DefaultSkipThreshold
	ExcludeDirs   []string                       // additional per-registry exclusions
	SymlinkPolicy string                         // "ask", "follow", "skip"
	Strict        bool                           // disables content-signal fallback
	ScanAsPaths   map[string]catalog.ContentType // user-directed: path prefix → type
}

// DefaultConfig returns the default analysis configuration.
func DefaultConfig() AnalysisConfig {
	return AnalysisConfig{
		AutoThreshold: DefaultAutoThreshold,
		SkipThreshold: DefaultSkipThreshold,
		SymlinkPolicy: "ask",
	}
}

// AllItems returns Auto and Confirm combined, Auto first.
func (r *AnalysisResult) AllItems() []*DetectedItem {
	all := make([]*DetectedItem, 0, len(r.Auto)+len(r.Confirm))
	all = append(all, r.Auto...)
	all = append(all, r.Confirm...)
	return all
}

// CountByType returns a map of ContentType to item count across Auto+Confirm.
func (r *AnalysisResult) CountByType() map[catalog.ContentType]int {
	counts := make(map[catalog.ContentType]int)
	for _, item := range r.AllItems() {
		counts[item.Type]++
	}
	return counts
}

// IsEmpty returns true if both Auto and Confirm are empty.
func (r *AnalysisResult) IsEmpty() bool {
	return len(r.Auto) == 0 && len(r.Confirm) == 0
}
