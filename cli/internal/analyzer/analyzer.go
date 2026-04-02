package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Analyzer orchestrates content discovery for a repository.
type Analyzer struct {
	config    AnalysisConfig
	detectors []ContentDetector
}

// New creates an Analyzer with all built-in detectors registered.
func New(config AnalysisConfig) *Analyzer {
	return &Analyzer{
		config: config,
		detectors: []ContentDetector{
			&SyllagoDetector{},
			&ClaudeCodeDetector{},
			&ClaudeCodePluginDetector{},
			&CursorDetector{},
			&CopilotDetector{},
			&WindsurfDetector{},
			&ClineDetector{},
			&RooCodeDetector{},
			&CodexDetector{},
			&GeminiDetector{},
			&TopLevelDetector{},
		},
	}
}

// Analyze examines repoDir and returns classified content items.
// repoDir is resolved via filepath.EvalSymlinks before analysis.
func (a *Analyzer) Analyze(repoDir string) (*AnalysisResult, error) {
	repoRoot, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		return nil, fmt.Errorf("resolving repo root: %w", err)
	}
	if err := validateRepoRoot(repoRoot); err != nil {
		return nil, err
	}

	result := &AnalysisResult{}

	// Step 1: Walk filesystem.
	walkResult := Walk(repoRoot, a.config.ExcludeDirs)
	result.Warnings = append(result.Warnings, walkResult.Warnings...)

	// Step 2: Pattern matching.
	candidates := MatchPatterns(walkResult.Paths, a.detectors)

	// Build set of matched paths for content-signal fallback exclusion.
	matchedPaths := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		matchedPaths[c.Path] = true
	}

	// Step 3: Classify candidates.
	var allItems []*DetectedItem
	totalBytes := int64(0)
	const maxTotalBytes = 50 * 1024 * 1024 // 50 MB per-repo limit

	for _, c := range candidates {
		if totalBytes > maxTotalBytes {
			result.Warnings = append(result.Warnings, "per-repo read limit reached; some items may not be classified")
			break
		}
		items, classifyErr := c.Detector.Classify(c.Path, repoRoot)
		if classifyErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("classify %s: %s", c.Path, classifyErr))
			continue
		}
		if info, statErr := os.Stat(filepath.Join(repoRoot, c.Path)); statErr == nil {
			totalBytes += info.Size()
		}
		for _, item := range items {
			if item == nil {
				continue
			}
			if item.Type == catalog.Skills || item.Type == catalog.Agents {
				item.References = ResolveReferences(item.Path, repoRoot)
			}
			SanitizeItem(item)
			allItems = append(allItems, item)
		}
	}

	// Step 3b: Content-signal fallback for unmatched files.
	if !a.config.Strict {
		var unmatchedPaths []string
		for _, p := range walkResult.Paths {
			if !matchedPaths[p] {
				unmatchedPaths = append(unmatchedPaths, p)
			}
		}
		csd := &ContentSignalDetector{}
		fallbackItems, skipEntries, fallbackErr := csd.ClassifyUnmatched(unmatchedPaths, repoRoot, a.config.ScanAsPaths, a.config.DebugSkips)
		if fallbackErr != nil {
			result.Warnings = append(result.Warnings, "content-signal fallback: "+fallbackErr.Error())
		}
		if a.config.DebugSkips {
			result.SkipReasons = append(result.SkipReasons, skipEntries...)
		}
		for _, item := range fallbackItems {
			if item == nil {
				continue
			}
			SanitizeItem(item)
			allItems = append(allItems, item)
		}
	}

	// Step 4: Dedup + conflict resolution.
	deduped, _ := DeduplicateItems(allItems)

	// Step 5: Partition by confidence.
	for _, item := range deduped {
		// Executable content always requires confirmation regardless of confidence.
		if item.Type == catalog.Hooks || item.Type == catalog.MCP {
			result.Confirm = append(result.Confirm, item)
			continue
		}
		if item.Confidence > a.config.AutoThreshold {
			result.Auto = append(result.Auto, item)
		} else if item.Confidence >= a.config.SkipThreshold {
			result.Confirm = append(result.Confirm, item)
		}
		// Below skip threshold: drop silently.
	}

	// Sort for deterministic output.
	sort.Slice(result.Auto, func(i, j int) bool {
		return result.Auto[i].Name < result.Auto[j].Name
	})
	sort.Slice(result.Confirm, func(i, j int) bool {
		return result.Confirm[i].Name < result.Confirm[j].Name
	})

	return result, nil
}

// ShouldTriggerInteractiveFallback returns true if the result is sparse enough
// to warrant user-directed discovery. Only applicable in interactive mode.
func ShouldTriggerInteractiveFallback(result *AnalysisResult) bool {
	return len(result.AllItems()) <= 5
}

// validateRepoRoot rejects paths that resolve to sensitive system roots.
func validateRepoRoot(resolved string) error {
	dangerous := []string{"/", "/etc", "/home", "/usr", "/var", "/sys", "/proc"}
	for _, d := range dangerous {
		if resolved == d {
			return fmt.Errorf("repo root %q resolves to a sensitive system path; refusing analysis", resolved)
		}
	}
	return nil
}
