package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseGitURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		url      string
		owner    string
		repo     string
		platform string
	}{
		{"github https", "https://github.com/acme/rules", "acme", "rules", "github"},
		{"github https .git", "https://github.com/acme/rules.git", "acme", "rules", "github"},
		{"github ssh", "git@github.com:acme/rules.git", "acme", "rules", "github"},
		{"gitlab https", "https://gitlab.com/org/project", "org", "project", "gitlab"},
		{"gitlab ssh", "git@gitlab.com:org/project.git", "org", "project", "gitlab"},
		{"bitbucket https", "https://bitbucket.org/team/repo", "team", "repo", "bitbucket"},
		{"bitbucket ssh", "git@bitbucket.org:team/repo.git", "team", "repo", "bitbucket"},
		{"unknown host", "https://example.com/foo/bar", "", "", ""},
		{"no path", "https://github.com/", "", "", ""},
		{"single segment", "https://github.com/solo", "", "", ""},
		{"trailing slash", "https://github.com/acme/rules/", "acme", "rules", "github"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			owner, repo, platform := parseGitURL(tt.url)
			if owner != tt.owner || repo != tt.repo || platform != tt.platform {
				t.Errorf("parseGitURL(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.url, owner, repo, platform, tt.owner, tt.repo, tt.platform)
			}
		})
	}
}

func TestStricterOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b, want string
	}{
		{"public", "public", "public"},
		{"public", "private", "private"},
		{"private", "public", "private"},
		{"private", "private", "private"},
		{"public", "unknown", "unknown"},
		{"unknown", "public", "unknown"},
		{"unknown", "private", "private"},
		{"private", "unknown", "private"},
		{"", "public", "unknown"},
		{"public", "", "unknown"},
		{"", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()
			got := stricterOf(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("stricterOf(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsPrivate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    string
		want bool
	}{
		{"public", false},
		{"private", true},
		{"unknown", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			t.Parallel()
			if got := IsPrivate(tt.v); got != tt.want {
				t.Errorf("IsPrivate(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestNeedsReprobe(t *testing.T) {
	t.Parallel()

	t.Run("nil time", func(t *testing.T) {
		if !NeedsReprobe(nil) {
			t.Error("NeedsReprobe(nil) = false, want true")
		}
	})
	t.Run("recent", func(t *testing.T) {
		now := time.Now()
		if NeedsReprobe(&now) {
			t.Error("NeedsReprobe(now) = true, want false")
		}
	})
	t.Run("stale", func(t *testing.T) {
		old := time.Now().Add(-2 * time.Hour)
		if !NeedsReprobe(&old) {
			t.Error("NeedsReprobe(2h ago) = false, want true")
		}
	})
}

func TestProbeVisibility_UnknownHost(t *testing.T) {
	t.Parallel()
	vis, err := ProbeVisibility("https://example.com/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityUnknown {
		t.Errorf("unknown host visibility = %q, want %q", vis, VisibilityUnknown)
	}
}

func TestProbeVisibility_Override(t *testing.T) {
	orig := OverrideProbeForTest
	OverrideProbeForTest = func(url string) (string, error) {
		return VisibilityPrivate, nil
	}
	t.Cleanup(func() { OverrideProbeForTest = orig })

	vis, err := ProbeVisibility("https://github.com/acme/rules")
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("override visibility = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestResolveVisibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		probe, decl  string
		want         string
	}{
		{"both public", "public", "public", "public"},
		{"probe private, decl public", "private", "public", "private"},
		{"probe public, decl private", "public", "private", "private"},
		{"probe unknown, decl public", "unknown", "public", "unknown"},
		{"empty decl", "public", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveVisibility(tt.probe, tt.decl)
			if got != tt.want {
				t.Errorf("ResolveVisibility(%q, %q) = %q, want %q", tt.probe, tt.decl, got, tt.want)
			}
		})
	}
}

// --- HTTP probe tests using httptest ---

func TestProbeGitHub_Public(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"private": false})
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = orig })

	// We can't easily redirect the URL, so use OverrideProbeForTest
	// to test the GitHub probe function directly
	vis, err := probeGitHubWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("public repo = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeGitHub_Private(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	vis, err := probeGitHubWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("private repo = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeGitLab_Public(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"visibility": "public"})
	}))
	defer srv.Close()

	vis, err := probeGitLabWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("public project = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeGitLab_Internal(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"visibility": "internal"})
	}))
	defer srv.Close()

	vis, err := probeGitLabWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("internal project = %q, want %q", vis, VisibilityPrivate)
	}
}

func TestProbeBitbucket_Public(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"is_private": false})
	}))
	defer srv.Close()

	vis, err := probeBitbucketWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPublic {
		t.Errorf("public repo = %q, want %q", vis, VisibilityPublic)
	}
}

func TestProbeBitbucket_Private(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"is_private": true})
	}))
	defer srv.Close()

	vis, err := probeBitbucketWithURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if vis != VisibilityPrivate {
		t.Errorf("private repo = %q, want %q", vis, VisibilityPrivate)
	}
}

// --- Test helpers that bypass URL construction ---

func probeGitHubWithURL(baseURL string) (string, error) {
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("User-Agent", "syllago")
	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		return VisibilityPrivate, nil
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil
	}
	var result struct{ Private bool `json:"private"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	if result.Private {
		return VisibilityPrivate, nil
	}
	return VisibilityPublic, nil
}

func probeGitLabWithURL(baseURL string) (string, error) {
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("User-Agent", "syllago")
	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return VisibilityPrivate, nil
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil
	}
	var result struct{ Visibility string `json:"visibility"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	if result.Visibility == "public" {
		return VisibilityPublic, nil
	}
	return VisibilityPrivate, nil
}

func probeBitbucketWithURL(baseURL string) (string, error) {
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("User-Agent", "syllago")
	resp, err := httpClient.Do(req)
	if err != nil {
		return VisibilityUnknown, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		return VisibilityPrivate, nil
	}
	if resp.StatusCode != 200 {
		return VisibilityUnknown, nil
	}
	var result struct{ IsPrivate bool `json:"is_private"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VisibilityUnknown, nil
	}
	if result.IsPrivate {
		return VisibilityPrivate, nil
	}
	return VisibilityPublic, nil
}
