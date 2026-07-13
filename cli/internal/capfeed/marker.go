package capfeed

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Marker is the decoded provenance.json: the last verified feed snapshot
// this mirror reflects. Its DataRevision is the change-detection comparison
// point.
type Marker struct {
	DataRevision string    `json:"data_revision"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// ReadMarker tolerantly decodes a provenance marker. A missing file is a
// zero marker with a nil error — the first-ever pull has no marker and must
// proceed. A malformed marker is an error: silently re-pulling over corrupt
// state would hide the corruption.
func ReadMarker(path string) (*Marker, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Marker{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("capfeed marker: %w", err)
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("capfeed marker %s: decode: %w", path, err)
	}
	return &m, nil
}
