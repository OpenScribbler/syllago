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

	targetLevel, targetText := parseHeadingPath(cfg.Primary)

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string
	inTargetSection := false
	tableRowIdx := 0
	itemIdx := 0

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			headingText := strings.TrimSpace(string(node.Text(raw)))
			landmarks = append(landmarks, headingText)
			if node.Level == targetLevel && headingText == targetText {
				inTargetSection = true
				tableRowIdx = 0
				itemIdx = 0
			} else if inTargetSection && node.Level <= targetLevel {
				inTargetSection = false
			}
		case *extast.TableRow, *extast.TableHeader:
			if !inTargetSection {
				return ast.WalkContinue, nil
			}
			colIdx := 0
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if child.Kind() == extast.KindTableCell {
					cellText := strings.TrimSpace(string(child.Text(raw)))
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
			itemText := strings.TrimSpace(string(node.Text(raw)))
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
	})

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
