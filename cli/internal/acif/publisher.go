package acif

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

const (
	DiagPublisherPackSourceConflict  = "acif.publisher.pack_source_conflict"
	DiagPublisherFrontmatterConflict = "acif.publisher.frontmatter_conflict"
)

type PackManifest struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

type PackManifestReconciliation struct {
	CanonicalSource      string       `json:"canonical_source,omitempty"`
	CanonicalDisplayName string       `json:"canonical_display_name,omitempty"`
	Diagnostics          []Diagnostic `json:"diagnostics,omitempty"`
}

func ReconcilePackManifests(manifests []PackManifest) PackManifestReconciliation {
	precedence := map[string]int{
		"package.json":               0,
		".claude-plugin/plugin.json": 1,
		".cursor-plugin/plugin.json": 2,
		".codex-plugin/plugin.json":  3,
		"gemini-extension.json":      4,
	}
	bestRank := len(precedence) + len(manifests) + 1
	bestIndex := -1
	sources := make([]string, 0, len(manifests))
	values := make([]string, 0, len(manifests))
	for i, manifest := range manifests {
		name := strings.TrimSpace(manifest.Name)
		if name == "" {
			continue
		}
		rank, ok := precedence[manifest.Source]
		if !ok {
			rank = len(precedence) + i
		}
		if rank < bestRank {
			bestRank = rank
			bestIndex = i
		}
		sources = append(sources, manifest.Source)
		values = append(values, name)
	}

	result := PackManifestReconciliation{}
	if bestIndex >= 0 {
		result.CanonicalSource = manifests[bestIndex].Source
		result.CanonicalDisplayName = strings.TrimSpace(manifests[bestIndex].Name)
	}
	if len(values) >= 2 {
		first := values[0]
		for _, value := range values[1:] {
			if value != first {
				result.Diagnostics = []Diagnostic{{
					ID: DiagPublisherPackSourceConflict,
					Params: map[string]any{
						"sources": sources,
						"values":  values,
					},
				}}
				break
			}
		}
	}
	return result
}

type FrontmatterReconcileResult struct {
	Action      string       `json:"action"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

func ReconcileFrontmatter(sidecarValue, sourceFrontmatter map[string]any, mode string) FrontmatterReconcileResult {
	if sidecarValue == nil {
		sidecarValue = map[string]any{}
	}
	if sourceFrontmatter == nil {
		sourceFrontmatter = map[string]any{}
	}
	action := "leave-untouched"
	var diagnostics []Diagnostic

	fields := make([]string, 0, len(sidecarValue))
	for field := range sidecarValue {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		canonical := sidecarValue[field]
		declared, present := sourceFrontmatter[field]
		switch {
		case !present:
			action = strongestFrontmatterAction(action, "add-silently")
		case reflect.DeepEqual(declared, canonical):
			action = strongestFrontmatterAction(action, "leave-untouched")
		case mode == "overwrite":
			action = strongestFrontmatterAction(action, "overwrite")
			diagnostics = append(diagnostics, frontmatterConflictDiagnostic(field, declared, canonical))
		default:
			action = strongestFrontmatterAction(action, "block")
			diagnostics = append(diagnostics, frontmatterConflictDiagnostic(field, declared, canonical))
		}
	}

	return FrontmatterReconcileResult{Action: action, Diagnostics: diagnostics}
}

func strongestFrontmatterAction(current, candidate string) string {
	rank := map[string]int{
		"leave-untouched": 0,
		"add-silently":    1,
		"overwrite":       2,
		"block":           3,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func frontmatterConflictDiagnostic(field string, declared, canonical any) Diagnostic {
	return Diagnostic{
		ID: DiagPublisherFrontmatterConflict,
		Params: map[string]any{
			"field":     field,
			"declared":  cloneJSONValue(declared),
			"canonical": cloneJSONValue(canonical),
		},
	}
}

func EnvelopePublisherSection(sidecar map[string]any) map[string]any {
	section := make(map[string]any)
	for key, value := range sidecar {
		if envelopeKeys[key] {
			section[key] = cloneJSONValue(value)
		}
	}
	if len(section) == 0 {
		return nil
	}
	return section
}

func IngestPackSidecar(sidecar map[string]any) (*RecordResult, error) {
	if sidecar == nil {
		sidecar = map[string]any{}
	}
	if packSourceKind(sidecar) == "inferred" {
		return &RecordResult{Conformant: true, Installable: true}, nil
	}

	section := cloneJSONValue(sidecar).(map[string]any)
	raw, err := json.Marshal(section)
	if err != nil {
		return nil, err
	}
	hashHex, _, err := MetadataHash(raw)
	if err != nil {
		return nil, err
	}
	return &RecordResult{
		Conformant:       true,
		Installable:      true,
		PublisherSection: section,
		MetadataHash:     hashHex,
	}, nil
}

func packSourceKind(sidecar map[string]any) string {
	if sourceKind, _ := sidecar["source_kind"].(string); sourceKind != "" {
		return sourceKind
	}
	pack, _ := sidecar["pack"].(map[string]any)
	sourceKind, _ := pack["source_kind"].(string)
	return sourceKind
}

func IngestProviderNativeFrontmatter(kind string, frontmatter map[string]any) (*RecordResult, error) {
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	extension := extensionBlockFromFrontmatter(frontmatter)
	item, err := canonicalizeExtension(kind, extension, "")
	if err != nil {
		return nil, err
	}
	if item.Canonical == nil {
		item.Canonical = map[string]any{}
	}
	section := publisherSection(kind, frontmatter)
	raw, err := json.Marshal(section)
	if err != nil {
		return nil, err
	}
	hashHex, _, err := MetadataHash(raw)
	if err != nil {
		return nil, err
	}

	result := &RecordResult{
		Conformant:  true,
		Installable: true,
		Canonical: map[string]any{
			"kind":        kind,
			kindKey(kind): item.Canonical,
		},
		PublisherSection: section,
		MetadataHash:     hashHex,
		Diagnostics:      item.Diagnostics,
	}
	if item.Verdict != nil {
		result.Conformant = false
		result.Installable = false
		result.Reason = item.Verdict.Reason
		result.Params = item.Verdict.Params
	}
	return result, nil
}
