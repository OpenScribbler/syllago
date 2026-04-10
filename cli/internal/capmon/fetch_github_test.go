package capmon_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestFetchGitHubFile(t *testing.T) {
	fileContent := []byte("# VS Code docs content")
	encoded := base64.StdEncoding.EncodeToString(fileContent)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := map[string]string{
			"content":  encoded + "\n",
			"encoding": "base64",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	capmon.SetGitHubBaseURLForTest(ts.URL)
	defer capmon.SetGitHubBaseURLForTest("")

	entry, err := capmon.FetchGitHubFile(context.Background(), cacheDir,
		"copilot", "hooks-docs",
		"microsoft", "vscode-docs", "main", "docs/copilot/hooks.md")
	if err != nil {
		t.Fatalf("FetchGitHubFile: %v", err)
	}
	if string(entry.Raw) != string(fileContent) {
		t.Errorf("content: got %q, want %q", string(entry.Raw), string(fileContent))
	}
}
