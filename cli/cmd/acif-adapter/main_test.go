package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func runLines(t *testing.T, input string) []map[string]any {
	t.Helper()
	var out strings.Builder
	if err := run(strings.NewReader(input), &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	var responses []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatalf("response is not JSON: %q: %v", scanner.Text(), err)
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan responses: %v", err)
	}
	return responses
}

func writeAdapterFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, data := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

func TestHello(t *testing.T) {
	t.Parallel()
	responses := runLines(t, `{"op":"hello","runner_protocol":1}`+"\n")
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	want := map[string]any{
		"ok": true,
		"result": map[string]any{
			"implementation":   "syllago",
			"version":          "0.0.0-dev",
			"adapter_protocol": float64(1),
			"scopes":           []any{"core"},
		},
	}
	if !reflect.DeepEqual(responses[0], want) {
		t.Fatalf("hello response = %#v, want %#v", responses[0], want)
	}
}

func TestIngestSidecarTV9(t *testing.T) {
	t.Parallel()
	request := `{"op":"ingest","input":{"kind":"command","sidecar":{"kind":"command","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Review PR","version":"1.2.0"}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	result, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", responses[0])
	}
	if responses[0]["ok"] != true {
		t.Fatalf("ok = %#v, want true", responses[0]["ok"])
	}
	if got, want := result["canonical_bytes"], `{"display_name":"Review PR","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","kind":"command","version":"1.2.0"}`; got != want {
		t.Fatalf("canonical_bytes = %#v, want %q", got, want)
	}
	if got, want := result["metadata_hash"], "ceb0cf9212c530e85444020aeb3cbae8865fdc16d91ee63fe6f5cb374d67b5c6"; got != want {
		t.Fatalf("metadata_hash = %#v, want %q", got, want)
	}
	if result["conformant"] != true || result["installable"] != true {
		t.Fatalf("conformant/installable = %#v/%#v, want true/true", result["conformant"], result["installable"])
	}
	publisherSection, ok := result["publisher_section"].(map[string]any)
	if !ok {
		t.Fatalf("publisher_section missing or wrong type: %#v", result["publisher_section"])
	}
	if publisherSection["display_name"] != "Review PR" || publisherSection["kind"] != "command" {
		t.Fatalf("publisher_section echo = %#v", publisherSection)
	}
}

func TestIngestBodyTV1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAdapterFixture(t, dir, map[string]string{
		"SKILL.md": "---\ndescription: demo\n---\nUse this skill to demo hashing.\n",
	})
	request := `{"op":"ingest","input":{"kind":"skill","body_root":` + mustJSON(t, dir) + `,"entry_file":"SKILL.md"}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	result, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", responses[0])
	}
	if got, want := result["body_hash"], "916e570331167c16f8112573d1b6020c134cc3d4019e8011693676d019b88ffe"; got != want {
		t.Fatalf("body_hash = %#v, want %q", got, want)
	}
	if got := result["classification"]; got != "single-file" {
		t.Fatalf("classification = %#v, want single-file", got)
	}
}

func TestIngestBodyRejectError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAdapterFixture(t, dir, map[string]string{"SKILL.md": "Body prose.\n"})
	if err := os.Symlink("SKILL.md", filepath.Join(dir, "link.md")); err != nil {
		t.Skipf("creating symlink: %v", err)
	}
	request := `{"op":"ingest","input":{"kind":"skill","body_root":` + mustJSON(t, dir) + `,"entry_file":"SKILL.md"}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	if responses[0]["ok"] != false || responses[0]["error"] != "acif.body.symlink" {
		t.Fatalf("response = %#v, want acif.body.symlink error", responses[0])
	}
}

func TestUnknownOpUnsupported(t *testing.T) {
	t.Parallel()
	responses := runLines(t, `{"op":"not_real","input":{}}`+"\n")
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	if !reflect.DeepEqual(responses[0], map[string]any{"unsupported": true}) {
		t.Fatalf("response = %#v, want unsupported", responses[0])
	}
}

func TestMalformedJSONLineContinues(t *testing.T) {
	t.Parallel()
	responses := runLines(t, "{bad json\n"+`{"op":"hello","runner_protocol":1}`+"\n")
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	if responses[0]["ok"] != false {
		t.Fatalf("first response = %#v, want ok=false", responses[0])
	}
	errText, _ := responses[0]["error"].(string)
	if !strings.HasPrefix(errText, "adapter: ") {
		t.Fatalf("malformed error = %q, want adapter prefix", errText)
	}
	if responses[1]["ok"] != true {
		t.Fatalf("second response = %#v, want hello success", responses[1])
	}
}

func TestStatelessnessSmoke(t *testing.T) {
	t.Parallel()
	request := `{"op":"ingest","input":{"kind":"command","sidecar":{"kind":"command","id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","display_name":"Review PR","version":"1.2.0"}}}` + "\n"
	var out strings.Builder
	if err := run(strings.NewReader(request+request), &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("response lines = %d, want 2: %q", len(lines), out.String())
	}
	if lines[0] != lines[1] {
		t.Fatalf("identical requests produced different responses:\n%s\n%s", lines[0], lines[1])
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return string(data)
}
