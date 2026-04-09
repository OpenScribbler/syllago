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

// ErrNoExtractor is returned by Extract when no extractor is registered for the format.
// Callers should treat this as a capability gap (skip + warn), not a failure.
type ErrNoExtractor struct {
	Format string
}

func (e *ErrNoExtractor) Error() string {
	return fmt.Sprintf("no extractor for format %q", e.Format)
}

// Extract dispatches to the appropriate Extractor for the given format.
func Extract(ctx context.Context, format string, raw []byte, cfg SelectorConfig) (*ExtractedSource, error) {
	ext, ok := extractors[format]
	if !ok {
		return nil, &ErrNoExtractor{Format: format}
	}
	return ext.Extract(ctx, raw, cfg)
}
