package capmon

import (
	"context"
	"fmt"
)

// Extractor extracts structured fields from raw source bytes.
// Each format (html, markdown, typescript, etc.) registers its own implementation.
type Extractor interface {
	Extract(ctx context.Context, raw []byte, cfg SelectorConfig) (*ExtractedSource, error)
}

// extractors maps format strings to Extractor implementations.
// Format packages register themselves via init().
var extractors = map[string]Extractor{}

// RegisterExtractor registers an extractor for a named format.
// Called from init() in each format package.
func RegisterExtractor(format string, ext Extractor) {
	extractors[format] = ext
}

// Extract dispatches to the appropriate Extractor for the given format.
func Extract(ctx context.Context, format string, raw []byte, cfg SelectorConfig) (*ExtractedSource, error) {
	ext, ok := extractors[format]
	if !ok {
		return nil, fmt.Errorf("no extractor for format %q", format)
	}
	return ext.Extract(ctx, raw, cfg)
}
