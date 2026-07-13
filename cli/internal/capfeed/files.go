package capfeed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// FetchFeedFiles fetches every attested file and verifies each against its
// sha256 from the (already provenance-verified) index BEFORE returning.
// Fail-closed: any fetch failure or hash mismatch returns an error and a nil
// map — callers never see partial results, so all verification precedes all
// writes.
func FetchFeedFiles(ctx context.Context, f *Fetcher, feedBaseURL string, files []IndexFile) (map[string][]byte, error) {
	if !strings.HasSuffix(feedBaseURL, "/") {
		feedBaseURL += "/"
	}

	out := make(map[string][]byte, len(files))
	for _, file := range files {
		res, err := f.Fetch(ctx, feedBaseURL+file.Path, "")
		if err != nil {
			return nil, fmt.Errorf("capfeed files: %s: %w", file.Path, err)
		}
		sum := sha256.Sum256(res.Body)
		if got := hex.EncodeToString(sum[:]); got != file.SHA256 {
			return nil, fmt.Errorf("capfeed files: %s: sha256 mismatch: index attests %s, fetched bytes hash to %s",
				file.Path, file.SHA256, got)
		}
		out[file.Path] = res.Body
	}
	return out, nil
}
