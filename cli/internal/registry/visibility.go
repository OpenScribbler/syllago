package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Visibility constants.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
	VisibilityUnknown = "unknown"
)

// CacheTTL is how long a cached visibility result is trusted before re-probing.
const CacheTTL = 1 * time.Hour

// OverrideProbeForTest allows tests to override the HTTP probe with a custom function.
// When non-nil, ProbeVisibility calls this instead of making real HTTP requests.
var OverrideProbeForTest func(url string) (string, error)

// ProbeVisibility detects whether a git remote URL points to a public or private
// repository by probing the hosting platform's API. Returns one of: "public",
// "private", or "unknown".
//
// Detection priority (stricter always wins):
//  1. API probe result (authoritative when reachable)
//  2. Manifest declaration (fallback for unknown hosts)
//  3. Default "unknown" (treated as private)
func ProbeVisibility(gitURL string) (string, error) {
	if OverrideProbeForTest != nil {
		return OverrideProbeForTest(gitURL)
	}

	owner, repo, platform := parseGitURL(gitURL)
	if platform == "" || owner == "" || repo == "" {
		return VisibilityUnknown, nil
	}

	switch platform {
	case "github":
		return probeGitHub(owner, repo)
	case "gitlab":
		return probeGitLab(owner, repo)
	case "bitbucket":
		return probeBitbucket(owner, repo)
	default:
		return VisibilityUnknown, nil
	}
}

// ResolveVisibility determines the effective visibility of a registry by
// combining the API probe result with the manifest declaration.
// The stricter value always wins: private > unknown > public.
func ResolveVisibility(probeResult, manifestDecl string) string {
	return stricterOf(probeResult, manifestDecl)
}

// NeedsReprobe returns true if the cached visibility is stale (older than CacheTTL).
func NeedsReprobe(checkedAt *time.Time) bool {
	if checkedAt == nil {
		return true
	}
	return time.Since(*checkedAt) > CacheTTL
}

// IsPrivate returns true if the visibility value should be treated as private.
// Both "private" and "unknown" are treated as private (fail-safe).
func IsPrivate(visibility string) bool {
	return visibility != VisibilityPublic
}

// stricterOf returns the stricter of two visibility values.
// private > unknown > public. Empty string is treated as unknown.
func stricterOf(a, b string) string {
	rank := func(v string) int {
		switch v {
		case VisibilityPrivate:
			return 2
		case VisibilityUnknown, "":
			return 1
		case VisibilityPublic:
			return 0
		default:
			return 1 // treat unrecognized as unknown
		}
	}
	if rank(a) >= rank(b) {
		return normalizeVisibility(a)
	}
	return normalizeVisibility(b)
}

func normalizeVisibility(v string) string {
	switch v {
	case VisibilityPublic, VisibilityPrivate, VisibilityUnknown:
		return v
	case "":
		return VisibilityUnknown
	default:
		return VisibilityUnknown
	}
}

// parseGitURL extracts owner, repo, and platform from a git URL.
// Supports HTTPS and SSH formats for GitHub, GitLab, and Bitbucket.
func parseGitURL(url string) (owner, repo, platform string) {
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// Map of host patterns to platform names
	hosts := map[string]string{
		"github.com":    "github",
		"gitlab.com":    "gitlab",
		"bitbucket.org": "bitbucket",
	}

	// HTTPS format: https://host/owner/repo
	if i := strings.Index(url, "://"); i >= 0 {
		path := url[i+3:]
		for host, plat := range hosts {
			if strings.HasPrefix(path, host+"/") {
				segments := strings.Split(strings.TrimPrefix(path, host+"/"), "/")
				if len(segments) >= 2 {
					return segments[0], segments[1], plat
				}
			}
		}
		return "", "", ""
	}

	// SSH format: git@host:owner/repo
	if strings.HasPrefix(url, "git@") {
		for host, plat := range hosts {
			prefix := "git@" + host + ":"
			if strings.HasPrefix(url, prefix) {
				path := strings.TrimPrefix(url, prefix)
				segments := strings.Split(path, "/")
				if len(segments) >= 2 {
					return segments[0], segments[1], plat
				}
			}
		}
	}

	return "", "", ""
}

// httpClient is the shared client for API probes. Overridable for tests.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// probeGitHub checks GitHub's REST API. Public repos return 200 with
// {"private": false}. Private repos return 404 without auth.
func probeGitHub(owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VisibilityUnknown, err
	}
	req.Header.Set("User-Agent", "syllago")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil // network error = unknown (fail-safe)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		return VisibilityPrivate, nil // not accessible = private
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil // unexpected status = unknown
	}

	var result struct {
		Private bool `json:"private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	if result.Private {
		return VisibilityPrivate, nil
	}
	return VisibilityPublic, nil
}

// probeGitLab checks GitLab's REST API. Public projects return 200 with
// {"visibility": "public"|"internal"|"private"}.
func probeGitLab(owner, repo string) (string, error) {
	// GitLab uses URL-encoded project path as ID
	projectID := owner + "%2F" + repo
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", projectID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VisibilityUnknown, err
	}
	req.Header.Set("User-Agent", "syllago")

	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return VisibilityPrivate, nil
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil
	}

	var result struct {
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	switch result.Visibility {
	case "public":
		return VisibilityPublic, nil
	default:
		return VisibilityPrivate, nil // "internal" and "private" are both private
	}
}

// probeBitbucket checks Bitbucket's REST API. Public repos return 200 with
// {"is_private": false}. Private repos return 403 without auth.
func probeBitbucket(owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VisibilityUnknown, err
	}
	req.Header.Set("User-Agent", "syllago")

	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		return VisibilityPrivate, nil
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil
	}

	var result struct {
		IsPrivate bool `json:"is_private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	if result.IsPrivate {
		return VisibilityPrivate, nil
	}
	return VisibilityPublic, nil
}
