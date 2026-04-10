package extract_markdown

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

func init() {
	capmon.RegisterExtractor("markdown", &markdownExtractor{})
}

type markdownExtractor struct{}

func (e *markdownExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
	}

	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	reader := text.NewReader(raw)
	doc := md.Parser().Parse(reader)

	// scopedMode: when a Primary selector is given, only extract within that heading section.
	// When empty, extract all tables and list items from the whole document.
	scopedMode := cfg.Primary != ""
	targetLevel, targetText := parseHeadingPath(cfg.Primary)

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string
	inTargetSection := !scopedMode // true from start when no selector
	tableRowIdx := 0
	itemIdx := 0

	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			headingText := extractText(node, raw)
			landmarks = append(landmarks, headingText)
			if scopedMode {
				if node.Level == targetLevel && headingText == targetText {
					inTargetSection = true
					tableRowIdx = 0
					itemIdx = 0
				} else if inTargetSection && node.Level <= targetLevel {
					inTargetSection = false
				}
			}
		case *extast.TableRow, *extast.TableHeader:
			if !inTargetSection {
				return ast.WalkContinue, nil
			}
			colIdx := 0
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if child.Kind() == extast.KindTableCell {
					cellText := extractText(child, raw)
					sanitized := capmon.SanitizeExtractedString(cellText)
					key := fmt.Sprintf("row_%d_col_%d", tableRowIdx, colIdx)
					fields[key] = capmon.FieldValue{
						Value:     sanitized,
						ValueHash: capmon.SHA256Hex([]byte(sanitized)),
					}
					colIdx++
				}
			}
			tableRowIdx++
		case *ast.ListItem:
			if !inTargetSection {
				return ast.WalkContinue, nil
			}
			itemText := extractText(node, raw)
			if itemText != "" {
				sanitized := capmon.SanitizeExtractedString(itemText)
				key := fmt.Sprintf("item_%d", itemIdx)
				fields[key] = capmon.FieldValue{
					Value:     sanitized,
					ValueHash: capmon.SHA256Hex([]byte(sanitized)),
				}
				itemIdx++
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return nil, fmt.Errorf("walk doc: %w", err)
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "markdown",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// extractText collects all ast.Text leaf values under n by walking the child tree.
// This replaces the deprecated ast.Node.Text(src) method.
func extractText(n ast.Node, raw []byte) string {
	var sb strings.Builder
	var collect func(ast.Node)
	collect = func(node ast.Node) {
		if t, ok := node.(*ast.Text); ok {
			sb.Write(t.Segment.Value(raw))
			return
		}
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			collect(c)
		}
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		collect(c)
	}
	return strings.TrimSpace(sb.String())
}

// parseHeadingPath parses "## Events" → (2, "Events"), "# Title" → (1, "Title").
func parseHeadingPath(primary string) (int, string) {
	level := 0
	for _, ch := range primary {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level == 0 {
		return 2, strings.TrimSpace(primary)
	}
	return level, strings.TrimSpace(primary[level:])
}
