package extract_html

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/PuerkitoBio/goquery"
)

func init() {
	capmon.RegisterExtractor("html", &htmlExtractor{})
}

type htmlExtractor struct{}

func (e *htmlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	// Check expected_contains anchor
	if cfg.ExpectedContains != "" {
		found := false
		doc.Find("body").Each(func(_ int, s *goquery.Selection) {
			if strings.Contains(s.Text(), cfg.ExpectedContains) {
				found = true
			}
		})
		if !found {
			return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
		}
	}

	fields := make(map[string]capmon.FieldValue)

	doc.Find(cfg.Primary).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		sanitized := capmon.SanitizeExtractedString(text)
		key := fmt.Sprintf("item_%d", i)
		fields[key] = capmon.FieldValue{
			Value:     sanitized,
			ValueHash: capmon.SHA256Hex([]byte(sanitized)),
		}
	})

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults

	var landmarks []string
	doc.Find("h1, h2, h3, h4").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			landmarks = append(landmarks, text)
		}
	})

	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "html",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}
