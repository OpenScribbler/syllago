package capmon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// githubBaseURL is overridable for tests.
var githubBaseURL = "https://api.github.com"

// SetGitHubBaseURLForTest overrides the GitHub API base URL in tests.
func SetGitHubBaseURLForTest(url string) {
	if url == "" {
		githubBaseURL = "https://api.github.com"
	} else {
		githubBaseURL = url
	}
}

type githubContentsResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// FetchGitHubFile fetches a file from a GitHub repo via the Contents API,
// decodes base64 content, writes to cache, and returns the entry.
func FetchGitHubFile(ctx context.Context, cacheRoot, provider, sourceID, owner, repo, ref, path string) (*CacheEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", githubBaseURL, owner, repo, path, ref)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := httpDoer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // closing HTTP response body in defer; error not actionable

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp githubContentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}

	if apiResp.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding %q, want base64", apiResp.Encoding)
	}

	// GitHub base64 content includes newlines — strip them before decoding
	cleaned := strings.ReplaceAll(apiResp.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("decode base64 content: %w", err)
	}

	meta := CacheMeta{
		FetchedAt:   time.Now().UTC(),
		ContentHash: SHA256Hex(decoded),
		FetchStatus: "ok",
		FetchMethod: "gh-api",
	}
	entry := CacheEntry{
		Provider: provider,
		SourceID: sourceID,
		Raw:      decoded,
		Meta:     meta,
	}
	if err := WriteCacheEntry(cacheRoot, entry); err != nil {
		return nil, fmt.Errorf("write cache entry: %w", err)
	}
	return &entry, nil
}
