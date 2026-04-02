package analyzer

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/tidwall/gjson"
)

// SignalEntry records one matched signal and its weight contribution.
// Exported for audit logging; populated by ContentSignalDetector.
type SignalEntry struct {
	Signal string
	Weight float64
}

// contentSignalBase is the starting score for any file that passes the pre-filter.
const contentSignalBase = 0.40

// contentSignalFloor is the minimum score required to emit an item.
const contentSignalFloor = 0.55

// contentSignalCap is the maximum score content-signal items can achieve.
const contentSignalCap = 0.70

// ContentSignalDetector classifies files that pattern-based detectors did not match.
// It uses weighted signal scoring against static fingerprints only.
type ContentSignalDetector struct{}

func (d *ContentSignalDetector) ProviderSlug() string { return "content-signal" }

// Patterns returns empty — the content-signal detector does not participate in MatchPatterns.
// It operates on unmatched files via ClassifyUnmatched.
func (d *ContentSignalDetector) Patterns() []DetectionPattern { return nil }

// Classify satisfies ContentDetector but is not used by this detector.
func (d *ContentSignalDetector) Classify(path, repoRoot string) ([]*DetectedItem, error) {
	return nil, nil
}

// ClassifyUnmatched inspects files not matched by any pattern-based detector.
// unmatchedPaths are relative to repoRoot. scanAsPaths maps relative path prefixes
// to their user-specified content type (bypasses directory keyword filter).
// When debugSkips is true, skipped files are recorded in the returned skip entries.
func (d *ContentSignalDetector) ClassifyUnmatched(
	unmatchedPaths []string,
	repoRoot string,
	scanAsPaths map[string]catalog.ContentType,
	debugSkips bool,
) ([]*DetectedItem, []SkipEntry, error) {
	var items []*DetectedItem
	var skips []SkipEntry

	for _, p := range unmatchedPaths {
		// Global README exclusion applies to content-signal detector too.
		if globalExcludedBasenames[filepath.Base(p)] {
			if debugSkips {
				skips = append(skips, SkipEntry{Path: p, Reason: "pre_filter_excluded"})
			}
			continue
		}

		userDirected := false
		var typeHint catalog.ContentType
		for prefix, ct := range scanAsPaths {
			if strings.HasPrefix(filepath.ToSlash(p), filepath.ToSlash(prefix)) {
				userDirected = true
				typeHint = ct
				break
			}
		}

		if !d.passesPreFilter(p, userDirected) {
			if debugSkips {
				skips = append(skips, SkipEntry{Path: p, Reason: "pre_filter_excluded"})
			}
			continue
		}

		item := d.classifyFile(p, repoRoot, userDirected, typeHint)
		if item != nil {
			items = append(items, item)
		} else if debugSkips {
			skips = append(skips, SkipEntry{Path: p, Reason: "below_threshold"})
		}
	}
	return items, skips, nil
}

// passesPreFilter returns true if the file should be inspected for content signals.
// userDirected bypasses the directory keyword requirement.
func (d *ContentSignalDetector) passesPreFilter(path string, userDirected bool) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if !contentSignalExtensions[ext] {
		return false
	}
	if userDirected {
		return true
	}
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")
	// Check that at least one path component (including filename) contains a known keyword.
	for _, part := range parts {
		lower := strings.ToLower(part)
		for _, kw := range directoryKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

// classifyFile reads and scores a single file, returning a DetectedItem or nil.
func (d *ContentSignalDetector) classifyFile(
	path, repoRoot string,
	userDirected bool,
	typeHint catalog.ContentType,
) *DetectedItem {
	ext := strings.ToLower(filepath.Ext(path))
	absPath := filepath.Join(repoRoot, path)

	var data []byte
	var readErr error
	if ext == ".json" {
		data, readErr = readFileLimited(absPath, limitJSON)
	} else {
		data, readErr = readFileLimited(absPath, limitMarkdown)
	}
	if readErr != nil || len(data) == 0 {
		return nil
	}

	// Score against all types and pick the winner.
	type typeScore struct {
		ct      catalog.ContentType
		signals []SignalEntry
		score   float64
	}

	var best typeScore
	allTypes := []catalog.ContentType{
		catalog.Commands, catalog.Skills, catalog.Agents,
		catalog.Hooks, catalog.MCP, catalog.Rules,
	}

	if typeHint != "" {
		allTypes = []catalog.ContentType{typeHint}
	}

	for _, ct := range allTypes {
		var signals []SignalEntry

		// Filename fingerprints.
		signals = append(signals, d.scoreFilename(filepath.Base(path), ct)...)

		// Directory keyword signals.
		signals = append(signals, d.scoreDirectory(path)...)

		// Content-based signals.
		if ext == ".json" {
			jsonSigs, detectedType := d.scoreJSON(data)
			if detectedType == ct {
				signals = append(signals, jsonSigs...)
			}
		} else {
			fmSigs, detectedType := d.scoreFrontmatter(data)
			if detectedType == ct || detectedType == "" {
				signals = append(signals, fmSigs...)
			}
		}

		score := d.computeScore(signals)
		if score > best.score || (score == best.score && d.typePriority(ct) < d.typePriority(best.ct)) {
			best = typeScore{ct, signals, score}
		}
	}

	confidence := best.score
	if userDirected {
		confidence = min(confidence+0.20, 0.85)
	}

	if best.ct == "" || confidence < contentSignalFloor {
		return nil
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	// Generic marker filenames (SKILL, AGENT) are not unique — use parent directory name.
	if name == "SKILL" || name == "AGENT" {
		dir := filepath.Dir(filepath.ToSlash(path))
		if dir != "." && dir != "" {
			name = filepath.Base(dir)
		}
	}
	item := &DetectedItem{
		Name:          name,
		Type:          best.ct,
		Provider:      "content-signal",
		Path:          path,
		ContentHash:   hashBytes(data),
		Confidence:    confidence,
		InternalLabel: "content-signal",
		Signals:       best.signals,
	}

	if fm := parseFrontmatterBasic(data); fm != nil {
		item.DisplayName = fm.name
		item.Description = fm.description
	}

	return item
}

// scoreFilename returns signals based on filename patterns for a given content type.
func (d *ContentSignalDetector) scoreFilename(filename string, ct catalog.ContentType) []SignalEntry {
	lower := strings.ToLower(filename)
	var signals []SignalEntry

	switch ct {
	case catalog.Skills:
		if filename == "SKILL.md" {
			signals = append(signals, SignalEntry{"filename_SKILL.md", 0.25})
		}
	case catalog.Agents:
		if filename == "AGENT.md" {
			signals = append(signals, SignalEntry{"filename_AGENT.md", 0.25})
		}
		if strings.HasSuffix(lower, ".agent.yaml") || strings.HasSuffix(lower, ".agent.md") {
			signals = append(signals, SignalEntry{"filename_agent_extension", 0.20})
		}
	}
	return signals
}

// scoreDirectory returns directory-context signals for the file path.
func (d *ContentSignalDetector) scoreDirectory(path string) []SignalEntry {
	var signals []SignalEntry
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 {
		return nil
	}
	dirs := parts[:len(parts)-1]

	for i, part := range dirs {
		lower := strings.ToLower(part)
		for _, kw := range directoryKeywords {
			if strings.Contains(lower, kw) {
				if i == 0 {
					signals = append(signals, SignalEntry{"directory_keyword_" + kw, 0.10})
				} else {
					signals = append(signals, SignalEntry{"subdirectory_keyword_" + kw, 0.05})
				}
				break
			}
		}
	}
	return signals
}

// scoreJSON scores a JSON file and returns signals + detected content type.
func (d *ContentSignalDetector) scoreJSON(data []byte) ([]SignalEntry, catalog.ContentType) {
	if !json.Valid(data) {
		return nil, ""
	}

	// MCP: top-level mcpServers key.
	if gjson.GetBytes(data, "mcpServers").IsObject() {
		return []SignalEntry{{"json_mcpServers", 0.30}}, catalog.MCP
	}

	// Hooks: hooks key with ≥2 known event name subkeys.
	hooks := gjson.GetBytes(data, "hooks")
	if hooks.IsObject() {
		var matchCount int
		hooks.ForEach(func(key, _ gjson.Result) bool {
			if knownHookEventNames[key.String()] {
				matchCount++
			}
			return true
		})
		if matchCount >= 2 {
			return []SignalEntry{{"json_hooks_event_names", 0.25}}, catalog.Hooks
		}
	}

	return nil, ""
}

// scoreFrontmatter scores YAML frontmatter and returns signals + detected content type.
func (d *ContentSignalDetector) scoreFrontmatter(data []byte) ([]SignalEntry, catalog.ContentType) {
	keys := parseFrontmatterKeys(data)
	if len(keys) == 0 {
		return nil, ""
	}

	var signals []SignalEntry
	typeCounts := make(map[catalog.ContentType]float64)

	for _, k := range keys {
		switch k {
		case "allowed-tools":
			signals = append(signals, SignalEntry{"frontmatter_allowed-tools", 0.20})
			typeCounts[catalog.Commands] += 0.20
		case "argument-hint":
			signals = append(signals, SignalEntry{"frontmatter_argument-hint", 0.15})
			typeCounts[catalog.Commands] += 0.15
		case "alwaysApply":
			signals = append(signals, SignalEntry{"frontmatter_alwaysApply", 0.15})
			typeCounts[catalog.Rules] += 0.15
		case "globs":
			signals = append(signals, SignalEntry{"frontmatter_globs", 0.10})
			typeCounts[catalog.Rules] += 0.10
		}
	}

	var bestType catalog.ContentType
	var bestWeight float64
	for ct, w := range typeCounts {
		if w > bestWeight {
			bestWeight = w
			bestType = ct
		}
	}

	return signals, bestType
}

// parseFrontmatterKeys extracts YAML frontmatter key names from file data.
func parseFrontmatterKeys(data []byte) []string {
	s := string(data)
	if !strings.HasPrefix(strings.TrimSpace(s), "---") {
		return nil
	}
	rest := s[strings.Index(s, "---")+3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	block := rest[:end]
	var keys []string
	for _, line := range strings.Split(block, "\n") {
		if k, _, ok := strings.Cut(line, ":"); ok {
			k = strings.TrimSpace(k)
			if k != "" && !strings.HasPrefix(k, "#") {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// computeScore returns base + sum of signal weights, capped at contentSignalCap.
func (d *ContentSignalDetector) computeScore(signals []SignalEntry) float64 {
	total := contentSignalBase
	for _, s := range signals {
		total += s.Weight
	}
	if total > contentSignalCap {
		return contentSignalCap
	}
	return total
}

// typePriority returns a rank for tie-breaking. Lower = higher priority.
// Commands > Skills > Agents > Hooks > MCP > Rules.
func (d *ContentSignalDetector) typePriority(ct catalog.ContentType) int {
	switch ct {
	case catalog.Commands:
		return 0
	case catalog.Skills:
		return 1
	case catalog.Agents:
		return 2
	case catalog.Hooks:
		return 3
	case catalog.MCP:
		return 4
	case catalog.Rules:
		return 5
	default:
		return 99
	}
}
