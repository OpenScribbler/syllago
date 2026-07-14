package acif

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"gopkg.in/yaml.v3"
)

type ItemResult struct {
	Canonical   map[string]any
	Diagnostics []Diagnostic
	Verdict     *HookVerdict
}

type RecordResult struct {
	Conformant       bool
	Installable      bool
	Reason           string
	Classification   string
	BodyHash         string
	Canonical        map[string]any
	PublisherSection map[string]any
	MetadataHash     string
	CanonicalBytes   string
	Diagnostics      []Diagnostic
}

type frontmatterDocument struct {
	Frontmatter map[string]any
	Body        string
	Present     bool
}

var envelopeKeys = map[string]bool{
	"kind":         true,
	"id":           true,
	"display_name": true,
	"version":      true,
	"description":  true,
	"license":      true,
	"pack_id":      true,
}

func IngestFrontmatterFile(kind, bodyRoot, entryFile string) (*RecordResult, error) {
	entryPath := filepath.Join(bodyRoot, filepath.FromSlash(entryFile))
	data, err := os.ReadFile(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			if _, bodyErr := BodyHash(bodyRoot, entryFile); bodyErr != nil {
				return nil, bodyErr
			}
		}
		return nil, fmt.Errorf("reading entry file: %w", err)
	}
	doc, err := parseFrontmatterDocument(data)
	if err != nil {
		return nil, err
	}

	body := doc.Body
	var diagnostics []Diagnostic
	if kind == "command" {
		var rewriteDiagnostics []Diagnostic
		body, rewriteDiagnostics = RewriteCommandPlaceholders(body)
		diagnostics = append(diagnostics, rewriteDiagnostics...)
	}

	bodyResult, err := BodyHashWithEntryBytes(bodyRoot, entryFile, []byte(body))
	if err != nil {
		return nil, err
	}

	extension := extensionBlockFromFrontmatter(doc.Frontmatter)
	item, err := canonicalizeExtension(kind, extension, "")
	if err != nil {
		return nil, err
	}
	item.Diagnostics = append(diagnostics, item.Diagnostics...)
	if item.Canonical == nil {
		item.Canonical = map[string]any{}
	}
	item.Canonical["body"] = body

	record := map[string]any{
		"kind":           kind,
		"classification": bodyResult.Classification,
		kindKey(kind):    item.Canonical,
	}
	result := &RecordResult{
		Conformant:     true,
		Installable:    true,
		Classification: bodyResult.Classification,
		BodyHash:       bodyResult.HashHex,
		Canonical:      record,
		Diagnostics:    item.Diagnostics,
	}
	if item.Verdict != nil {
		result.Conformant = false
		result.Installable = false
		result.Reason = item.Verdict.Reason
	}

	if doc.Present {
		section := publisherSection(kind, doc.Frontmatter)
		result.PublisherSection = section
		raw, err := json.Marshal(section)
		if err != nil {
			return nil, err
		}
		hashHex, _, err := MetadataHash(raw)
		if err != nil {
			return nil, err
		}
		result.MetadataHash = hashHex
	}

	return result, nil
}

func IngestExtensionBlock(kind string, sidecar map[string]any) (*RecordResult, error) {
	key := kindKey(kind)
	block, _ := sidecar[key].(map[string]any)
	if block == nil {
		block = map[string]any{}
	}
	item, err := canonicalizeExtension(kind, block, "")
	if err != nil {
		return nil, err
	}
	result := &RecordResult{
		Conformant:  true,
		Installable: true,
		Canonical: map[string]any{
			"kind": kind,
			key:    item.Canonical,
		},
		Diagnostics: item.Diagnostics,
	}
	if item.Verdict != nil {
		result.Conformant = false
		result.Installable = false
		result.Reason = item.Verdict.Reason
	}
	return result, nil
}

func parseFrontmatterDocument(data []byte) (frontmatterDocument, error) {
	text := moat.CanonicalText(data)
	yamlText, body, present := splitFrontmatter(text)
	doc := frontmatterDocument{Body: string(body), Present: present}
	if !present || len(bytes.TrimSpace(yamlText)) == 0 {
		return doc, nil
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(yamlText, &parsed); err != nil {
		return doc, fmt.Errorf("parsing frontmatter: %w", err)
	}
	doc.Frontmatter = normalizeYAMLValue(parsed).(map[string]any)
	return doc, nil
}

func splitFrontmatter(text []byte) (yamlText []byte, body []byte, present bool) {
	if !bytes.HasPrefix(text, []byte("---\n")) {
		return nil, text, false
	}
	const closing = "\n---\n"
	if idx := bytes.Index(text[3:], []byte(closing)); idx >= 0 {
		start := idx + 3
		return text[4:start], text[start+len(closing):], true
	}
	if bytes.HasSuffix(text, []byte("\n---")) {
		return text[4 : len(text)-4], nil, true
	}
	return nil, text, false
}

func normalizeYAMLValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeYAMLValue(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[fmt.Sprint(k)] = normalizeYAMLValue(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = normalizeYAMLValue(v)
		}
		return out
	default:
		return x
	}
}

func extensionBlockFromFrontmatter(frontmatter map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range frontmatter {
		if envelopeKeys[k] {
			continue
		}
		out[k] = cloneJSONValue(v)
	}
	return out
}

func publisherSection(kind string, frontmatter map[string]any) map[string]any {
	section := make(map[string]any)
	extension := make(map[string]any)
	for k, v := range frontmatter {
		if envelopeKeys[k] {
			section[k] = cloneJSONValue(v)
			continue
		}
		extension[k] = cloneJSONValue(v)
	}
	if len(extension) > 0 {
		section[kindKey(kind)] = extension
	}
	return section
}

func canonicalizeExtension(kind string, block map[string]any, provider string) (*ItemResult, error) {
	switch kind {
	case "skill":
		return CanonicalizeSkill(block)
	case "rule":
		return CanonicalizeRule(block)
	case "command":
		return CanonicalizeCommand(block)
	case "agent":
		return CanonicalizeAgent(block, provider)
	default:
		return nil, fmt.Errorf("unsupported extension kind %q", kind)
	}
}

func applyRequiresVerdict(block map[string]any) *HookVerdict {
	requires, ok := block["requires"]
	if !ok {
		return nil
	}
	if reqMap, ok := requires.(map[string]any); ok && len(reqMap) == 0 {
		delete(block, "requires")
		return nil
	}
	return &HookVerdict{Reason: ReasonRequiresOrphanKey}
}

func kindKey(kind string) string {
	if kind == "mcp_config" {
		return "mcp"
	}
	return kind
}
