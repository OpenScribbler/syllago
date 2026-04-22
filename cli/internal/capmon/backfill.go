package capmon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SourceFetcher abstracts how BackfillFormatDoc fetches a single source URI.
// Production callers use DefaultSourceFetcher; tests pass a stub.
type SourceFetcher interface {
	Fetch(ctx context.Context, uri, fetchMethod string) ([]byte, error)
}

// FetchURI fetches a URI using HTTP GET, or a headless Chromium browser when
// fetchMethod is "chromedp". Returns the raw body.
func FetchURI(ctx context.Context, uri, fetchMethod string) ([]byte, error) {
	if fetchMethod == "chromedp" {
		tmpCache, err := os.MkdirTemp("", "capmon-fetchuri-*")
		if err != nil {
			return nil, fmt.Errorf("chromedp temp cache: %w", err)
		}
		defer os.RemoveAll(tmpCache)
		entry, err := FetchChromedp(ctx, tmpCache, "fetchuri", "0", uri)
		if err != nil {
			return nil, err
		}
		return entry.Raw, nil
	}
	body, _, _, err := fetchForCheck(ctx, uri)
	return body, err
}

// DefaultSourceFetcher routes by fetch_method: "chromedp" goes through a
// headless browser, everything else through HTTP GET.
type DefaultSourceFetcher struct{}

// Fetch implements SourceFetcher.
func (DefaultSourceFetcher) Fetch(ctx context.Context, uri, fetchMethod string) ([]byte, error) {
	return FetchURI(ctx, uri, fetchMethod)
}

// BackfillOptions controls BackfillFormatDoc behavior.
type BackfillOptions struct {
	// Force re-fetches and overwrites content_hash even when already populated.
	Force bool
}

// BackfillResult reports the outcome of a BackfillFormatDoc run.
type BackfillResult struct {
	// Updated is the number of sources whose content_hash was written during this run.
	Updated int
	// Errors holds per-source fetch failures. A non-empty Errors slice does NOT
	// abort the run — other sources still get backfilled — but the slice is also
	// returned as a joined error from BackfillFormatDoc.
	Errors []error
}

// BackfillFormatDoc reads the FormatDoc YAML at path, fetches each source whose
// content_hash is missing or empty (or every source when opts.Force is true),
// and writes the computed sha256 back in place using byte-level surgery guided
// by yaml.Node positions. This preserves comments, blank lines, folded-scalar
// wrapping, and key order — only the touched lines change.
//
// When at least one source is updated, the top-level last_fetched_at is also
// refreshed to the current UTC time.
func BackfillFormatDoc(ctx context.Context, path string, fetcher SourceFetcher, opts BackfillOptions) (BackfillResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("read %s: %w", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return BackfillResult{}, fmt.Errorf("parse YAML %s: %w", path, err)
	}
	if len(root.Content) == 0 {
		return BackfillResult{}, fmt.Errorf("empty YAML document: %s", path)
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return BackfillResult{}, fmt.Errorf("expected top-level mapping in %s, got kind %d", path, top.Kind)
	}

	contentTypes := findMappingValue(top, "content_types")
	if contentTypes == nil || contentTypes.Kind != yaml.MappingNode {
		return BackfillResult{}, nil
	}

	// Plan phase: collect what needs to happen for each source.
	type sourcePlan struct {
		uri                string
		fetchMethod        string
		existingHash       string
		hashValueNode      *yaml.Node // nil if content_hash key is absent
		fetchedAtValueNode *yaml.Node // nil if fetched_at key is absent
		uriKeyNode         *yaml.Node // for indent column
		uriValueNode       *yaml.Node // for line position (fallback insert target)
	}

	var plans []*sourcePlan
	for i := 0; i+1 < len(contentTypes.Content); i += 2 {
		ctValue := contentTypes.Content[i+1]
		if ctValue.Kind != yaml.MappingNode {
			continue
		}
		sources := findMappingValue(ctValue, "sources")
		if sources == nil || sources.Kind != yaml.SequenceNode {
			continue
		}
		for _, src := range sources.Content {
			if src.Kind != yaml.MappingNode {
				continue
			}
			uriKey, uriVal := findMappingKeyValue(src, "uri")
			if uriVal == nil {
				continue
			}
			p := &sourcePlan{
				uri:          uriVal.Value,
				uriKeyNode:   uriKey,
				uriValueNode: uriVal,
			}
			if _, hashVal := findMappingKeyValue(src, "content_hash"); hashVal != nil {
				p.hashValueNode = hashVal
				p.existingHash = hashVal.Value
			}
			if _, faVal := findMappingKeyValue(src, "fetched_at"); faVal != nil {
				p.fetchedAtValueNode = faVal
			}
			if fmVal := findMappingValue(src, "fetch_method"); fmVal != nil {
				p.fetchMethod = fmVal.Value
			}
			if p.existingHash != "" && !opts.Force {
				continue
			}
			plans = append(plans, p)
		}
	}

	type editKind int
	const (
		editReplace editKind = iota
		editInsertBefore
		editInsertAfter
	)
	type edit struct {
		kind          editKind
		line          int // 0-based line index in the original file
		replaceColumn int // 1-based column where the scalar value begins (for replace)
		newValue      string
		indent        int    // for insert kinds
		newKeyValue   string // for insert kinds
	}

	var edits []edit
	now := time.Now().UTC().Format(time.RFC3339)
	result := BackfillResult{}

	// Fetch + build per-source edits.
	for _, p := range plans {
		body, ferr := fetcher.Fetch(ctx, p.uri, p.fetchMethod)
		if ferr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", p.uri, ferr))
			continue
		}
		newHash := SHA256Hex(body)
		indent := p.uriKeyNode.Column - 1

		if p.hashValueNode != nil {
			edits = append(edits, edit{
				kind:          editReplace,
				line:          p.hashValueNode.Line - 1,
				replaceColumn: p.hashValueNode.Column,
				newValue:      fmt.Sprintf("%q", newHash),
			})
		} else if p.fetchedAtValueNode != nil {
			edits = append(edits, edit{
				kind:        editInsertBefore,
				line:        p.fetchedAtValueNode.Line - 1,
				indent:      indent,
				newKeyValue: fmt.Sprintf("content_hash: %q", newHash),
			})
		} else {
			edits = append(edits, edit{
				kind:        editInsertAfter,
				line:        p.uriValueNode.Line - 1,
				indent:      indent,
				newKeyValue: fmt.Sprintf("content_hash: %q", newHash),
			})
		}

		if p.fetchedAtValueNode != nil {
			edits = append(edits, edit{
				kind:          editReplace,
				line:          p.fetchedAtValueNode.Line - 1,
				replaceColumn: p.fetchedAtValueNode.Column,
				newValue:      fmt.Sprintf("%q", now),
			})
		} else {
			edits = append(edits, edit{
				kind:        editInsertAfter,
				line:        p.uriValueNode.Line - 1,
				indent:      indent,
				newKeyValue: fmt.Sprintf("fetched_at: %q", now),
			})
		}
		result.Updated++
	}

	if result.Updated == 0 {
		if len(result.Errors) > 0 {
			return result, errors.Join(result.Errors...)
		}
		return result, nil
	}

	// Update top-level last_fetched_at if present.
	if lfVal := findMappingValue(top, "last_fetched_at"); lfVal != nil {
		edits = append(edits, edit{
			kind:          editReplace,
			line:          lfVal.Line - 1,
			replaceColumn: lfVal.Column,
			newValue:      fmt.Sprintf("%q", now),
		})
	}

	// Bucket edits by line, then rebuild the file one line at a time.
	lineEdits := map[int][]edit{}
	for _, e := range edits {
		lineEdits[e.line] = append(lineEdits[e.line], e)
	}

	origLines := strings.Split(string(data), "\n")
	newLines := make([]string, 0, len(origLines)+result.Updated*2)
	for i, ln := range origLines {
		currentLine := ln
		var beforeLines []string
		var afterLines []string
		for _, e := range lineEdits[i] {
			switch e.kind {
			case editReplace:
				if e.replaceColumn-1 <= len(currentLine) {
					currentLine = currentLine[:e.replaceColumn-1] + e.newValue
				}
			case editInsertBefore:
				beforeLines = append(beforeLines, strings.Repeat(" ", e.indent)+e.newKeyValue)
			case editInsertAfter:
				afterLines = append(afterLines, strings.Repeat(" ", e.indent)+e.newKeyValue)
			}
		}
		newLines = append(newLines, beforeLines...)
		newLines = append(newLines, currentLine)
		newLines = append(newLines, afterLines...)
	}

	if err := os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return result, fmt.Errorf("write %s: %w", path, err)
	}

	if len(result.Errors) > 0 {
		return result, errors.Join(result.Errors...)
	}
	return result, nil
}

// findMappingKeyValue returns (keyNode, valueNode) for the given key, or (nil, nil).
func findMappingKeyValue(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}
