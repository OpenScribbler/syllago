package main

import (
	"bufio"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
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
			"scopes":           []any{"core", "hook", "skill", "rule", "command", "agent", "mcp", "publisher", "registry"},
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

func TestHookIngestSidecar(t *testing.T) {
	t.Parallel()
	request := `{"op":"ingest","input":{"kind":"hook","sidecar":{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"inline","content":"#!/bin/sh\r\nexit 0\r\n"}]}]}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	if responses[0]["ok"] != true {
		t.Fatalf("response = %#v, want ok true", responses[0])
	}
	result, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %#v", responses[0])
	}
	if result["conformant"] != true || result["installable"] != true {
		t.Fatalf("conformant/installable = %#v/%#v", result["conformant"], result["installable"])
	}
	if got, want := result["body_hash"], "9c8ab2d7f2465728140264d725011daad97054aceaaedfa9dd0a03d68d06b629"; got != want {
		t.Fatalf("body_hash = %#v, want %q", got, want)
	}
	if got, want := result["canonical_bytes"], `{"blocking":false,"event":"before_tool_execute","handlers":[{"async":false,"scripts":[{"content":"#!/bin/sh\nexit 0\n","type":"inline"}],"type":"command"}]}`; got != want {
		t.Fatalf("canonical_bytes = %#v, want %q", got, want)
	}
}

func TestHookIngestProviderConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAdapterFixture(t, dir, map[string]string{
		"hooks/base.sh": "#!/bin/sh\necho base\n",
		"hooks/win.cmd": "@echo off\r\necho win\r\n",
		"hooks/lin.sh":  "#!/bin/sh\necho lin\n",
		"hooks/mac.sh":  "#!/bin/sh\necho mac\n",
	})
	request := `{"op":"ingest","input":{"kind":"hook","body_root":` + mustJSON(t, dir) + `,"provider_config":{"provider":"per-os-key-map","path":"settings.json","content":{"command":"hooks/base.sh","windows":"hooks/win.cmd","linux":"hooks/lin.sh","osx":"hooks/mac.sh"}}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	if responses[0]["ok"] != true {
		t.Fatalf("response = %#v, want ok true", responses[0])
	}
	result := responses[0]["result"].(map[string]any)
	if got, want := result["body_hash"], "11f1e91480d2fdfd247311cc52240bf0fb2293febeccaeeadafacd74707cb32d"; got != want {
		t.Fatalf("body_hash = %#v, want %q", got, want)
	}
	if result["provenance"] != "declared" {
		t.Fatalf("provenance = %#v, want declared", result["provenance"])
	}
}

func TestHookIngestReject(t *testing.T) {
	t.Parallel()
	request := `{"op":"ingest","input":{"kind":"hook","sidecar":{"event":"before_tool_execute"}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	if responses[0]["ok"] != false || responses[0]["error"] != "acif.hook.handlers_missing" {
		t.Fatalf("response = %#v, want handlers_missing error", responses[0])
	}
}

func TestHookProjectOps(t *testing.T) {
	t.Parallel()
	request := `{"op":"project","input":{"projection":"script_selection","targets":["linux","windows"],"item":{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}]}]}}}` + "\n" +
		`{"op":"project","input":{"projection":"derived_capabilities","item":{"event":"before_tool_execute","matcher":"shell","handlers":[{"async":true,"scripts":[{"type":"inline","content":"x"}]}]}}}` + "\n" +
		`{"op":"project","input":{"projection":"not-real","item":{}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(responses))
	}
	result := responses[0]["result"].(map[string]any)
	selection := result["selection"].(map[string]any)
	if selection["linux"] != "hooks/unix.sh" || selection["windows"] != "none" {
		t.Fatalf("selection = %#v", selection)
	}
	diags := result["diagnostics"].([]any)
	if len(diags) != 1 || diags[0].(map[string]any)["id"] != "acif.hook.script_no_platform_match" {
		t.Fatalf("diagnostics = %#v", diags)
	}
	caps := responses[1]["result"].(map[string]any)["derived_capabilities"].(map[string]any)
	if caps["handler_types"] != true || caps["matcher_patterns"] != true || caps["async_execution"] != true {
		t.Fatalf("derived_capabilities = %#v", caps)
	}
	if !reflect.DeepEqual(responses[2], map[string]any{"unsupported": true}) {
		t.Fatalf("unknown projection = %#v, want unsupported", responses[2])
	}
}

func TestHookRenderOp(t *testing.T) {
	t.Parallel()
	request := `{"op":"render","input":{"target":"no-mechanism-provider","invocation":{"target_os":"linux"},"canonical":{"event":"before_tool_execute","handlers":[{"scripts":[{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}]}]}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	result := responses[0]["result"].(map[string]any)
	if !strings.Contains(result["output"].(string), `"command":"hooks/unix.sh"`) {
		t.Fatalf("render output = %#v", result["output"])
	}
}

func TestHookEvaluateOps(t *testing.T) {
	t.Parallel()
	request := `{"op":"evaluate_install","input":{"install_target_os":"windows","item":{"event":"before_tool_execute","blocking":true,"handlers":[{"scripts":[{"type":"file","path":"hooks/unix.sh","os":["darwin","linux"]}]}]}}}` + "\n" +
		`{"op":"evaluate_requires","input":{"item_requires":{"handler_types":["command"]},"consumer_recognizes":["matcher_patterns"]}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	if got := responses[0]["result"].(map[string]any)["install"]; got != "refuse-unless-operator-opt-in" {
		t.Fatalf("evaluate_install = %#v", responses[0])
	}
	result := responses[1]["result"].(map[string]any)
	if result["evaluation"] != "unknown" || result["install"] != "refuse-unless-operator-opt-in" {
		t.Fatalf("evaluate_requires = %#v", result)
	}
}

func TestStage2IngestProjectRenderAndResolveOps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeAdapterFixture(t, dir, map[string]string{
		"COMMAND.md": "---\ndescription: review\n---\nReview PR {{args}} carefully.\n",
	})
	request := `{"op":"ingest","input":{"kind":"command","body_root":` + mustJSON(t, dir) + `,"entry_file":"COMMAND.md"}}` + "\n" +
		`{"op":"ingest","input":{"kind":"skill","sidecar":{"skill":{"activation":{"type":"manual"}}}}}` + "\n" +
		`{"op":"ingest","input":{"kind":"mcp_config","sidecar":{"servers":{"demo":{"command":"npx","args":["-y","@demo/mcp-server"]}}}}}` + "\n" +
		`{"op":"project","input":{"projection":"derived_capabilities","item":{"agent":{"tools":["spawn_agent"],"model":"gpt-5-codex","mcp_servers":["demo"]}}}}` + "\n" +
		`{"op":"project","input":{"projection":"rule_activation","item":{"rule":{"activation":{"mode":"glob","globs":["*.go","cmd/**","internal/**"]}}}}}` + "\n" +
		`{"op":"project","input":{"projection":"advisory","item":{"command":{"body":"Review $ARGUMENTS."}}}}` + "\n" +
		`{"op":"project","input":{"projection":"builtin_shadowing_advisory","item":{"command":{"body":"x"}}}}` + "\n" +
		`{"op":"render","input":{"target":"input-form","canonical":{"command":{"body":"Review $ARGUMENTS."}}}}` + "\n" +
		`{"op":"resolve_reference","input":{"item":{"skill":{"activation":{"type":"hook","hook_ref":{"id":"550e8400-e29b-41d4-a716-446655440000"}}}},"registry_state":{"known_hooks":["550e8400-e29b-41d4-a716-446655440000"]}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 9 {
		t.Fatalf("responses = %d, want 9", len(responses))
	}

	commandIngest := responses[0]["result"].(map[string]any)
	if commandIngest["body_hash"] != "eb6f4eb9bc130773a45fb39b20e9ad8e8a05fdabc011d85eeaf47dec08fa5cea" {
		t.Fatalf("command ingest = %#v", commandIngest)
	}
	skillIngest := responses[1]["result"].(map[string]any)
	if skillIngest["conformant"] != true || skillIngest["installable"] != true {
		t.Fatalf("skill sidecar = %#v", skillIngest)
	}
	mcpIngest := responses[2]["result"].(map[string]any)
	if mcpIngest["body_hash"] != "26387bc7f0b779925f2d6e704f3dfe590fd381893aa301bc28d2ae399f5e3b52" {
		t.Fatalf("mcp ingest = %#v", mcpIngest)
	}

	agentCaps := responses[3]["result"].(map[string]any)["derived_capabilities"].(map[string]any)
	if agentCaps["subagent_spawning"] != true || agentCaps["per_agent_mcp"] != true {
		t.Fatalf("agent caps = %#v", agentCaps)
	}
	ruleProjection := responses[4]["result"].(map[string]any)["projection"].(map[string]any)
	if ruleProjection["mode"] != "glob" {
		t.Fatalf("rule projection = %#v", ruleProjection)
	}
	advisory := responses[5]["result"].(map[string]any)["projection"].(map[string]any)
	token := advisory["argument_substitution_token"].(map[string]any)
	if token["present"] != true || token["method"] != "substring-canonical-v1" {
		t.Fatalf("advisory = %#v", advisory)
	}
	if _, ok := responses[6]["result"].(map[string]any)["projection"]; ok {
		t.Fatalf("builtin shadowing advisory should be vacuous-pass: %#v", responses[6])
	}
	rendered := responses[7]["result"].(map[string]any)
	if rendered["output"] != "Review ${input:args}." {
		t.Fatalf("render output = %#v", rendered)
	}
	cross := responses[8]["result"].(map[string]any)["cross_reference"].(map[string]any)
	if !reflect.DeepEqual(cross, map[string]any{
		"source_path": "skill.activation.hook_ref",
		"target_kind": "hook",
		"resolution":  "resolved",
	}) {
		t.Fatalf("skill cross reference = %#v", cross)
	}
}

func TestPublisherStage3NDJSON(t *testing.T) {
	t.Parallel()

	request := `{"op":"reconcile_frontmatter","input":{"sidecar_value":{"description":"canonical"},"source_frontmatter":{"description":"declared"},"mode":"default"}}` + "\n" +
		`{"op":"reconcile_frontmatter","input":{"sidecar_value":{"description":"canonical"},"source_frontmatter":{"description":"declared"},"mode":"overwrite"}}` + "\n" +
		`{"op":"ingest","input":{"kind":"pack","manifests":[{"source":"package.json","name":"superpowers"},{"source":"gemini-extension.json","name":"super-powers"}]}}` + "\n" +
		`{"op":"ingest","input":{"kind":"pack","sidecar":{"source_kind":"declared"}}}` + "\n" +
		`{"op":"ingest","input":{"kind":"pack","sidecar":{"source_kind":"inferred"}}}` + "\n" +
		`{"op":"ingest","input":{"kind":"agent","provider_config":{"provider":"provider-native-frontmatter","content":{"frontmatter":{"tools":["Read","Task"]}}}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 6 {
		t.Fatalf("responses = %d, want 6", len(responses))
	}

	defaultReconcile := responses[0]["result"].(map[string]any)
	if defaultReconcile["action"] != "block" {
		t.Fatalf("default reconcile = %#v", defaultReconcile)
	}
	overwriteReconcile := responses[1]["result"].(map[string]any)
	if overwriteReconcile["action"] != "overwrite" {
		t.Fatalf("overwrite reconcile = %#v", overwriteReconcile)
	}

	manifest := responses[2]["result"].(map[string]any)
	if manifest["canonical_source"] != "package.json" || manifest["canonical_display_name"] != "superpowers" {
		t.Fatalf("pack manifest result = %#v", manifest)
	}
	manifestDiag := manifest["diagnostics"].([]any)[0].(map[string]any)
	params := manifestDiag["params"].(map[string]any)
	if !reflect.DeepEqual(params["sources"], []any{"package.json", "gemini-extension.json"}) {
		t.Fatalf("manifest sources = %#v", params["sources"])
	}
	if !reflect.DeepEqual(params["values"], []any{"superpowers", "super-powers"}) {
		t.Fatalf("manifest values = %#v", params["values"])
	}

	declaredPack := responses[3]["result"].(map[string]any)
	if declaredPack["metadata_hash"] == "" || declaredPack["publisher_section"] == nil {
		t.Fatalf("declared pack missing publisher hash fields: %#v", declaredPack)
	}
	if _, ok := declaredPack["body_hash"]; ok {
		t.Fatalf("declared pack body_hash present: %#v", declaredPack)
	}

	inferredPack := responses[4]["result"].(map[string]any)
	if inferredPack["conformant"] != true || inferredPack["installable"] != true {
		t.Fatalf("inferred pack = %#v", inferredPack)
	}
	for _, key := range []string{"publisher_section", "metadata_hash", "body_hash"} {
		if _, ok := inferredPack[key]; ok {
			t.Fatalf("inferred pack has %s: %#v", key, inferredPack)
		}
	}

	agent := responses[5]["result"].(map[string]any)
	publisherTools := agent["publisher_section"].(map[string]any)["agent"].(map[string]any)["tools"]
	if !reflect.DeepEqual(publisherTools, []any{"Read", "Task"}) {
		t.Fatalf("publisher tools = %#v", publisherTools)
	}
	canonicalTools := agent["canonical"].(map[string]any)["agent"].(map[string]any)["tools"]
	if !reflect.DeepEqual(canonicalTools, []any{"file_read", "agent"}) {
		t.Fatalf("canonical tools = %#v", canonicalTools)
	}
	if agent["metadata_hash"] == "" {
		t.Fatalf("agent metadata_hash missing: %#v", agent)
	}
	if _, ok := agent["body_hash"]; ok {
		t.Fatalf("agent body_hash present: %#v", agent)
	}
}

func TestRegistryStage4NDJSONOps(t *testing.T) {
	t.Parallel()

	request := `{"op":"normalize_uri","input":{"uri":"HTTPS://GitHub.IO/A%3ab"}}` + "\n" +
		`{"op":"normalize_uri","input":{"uri":"https://example.com/x?token=secret"}}` + "\n" +
		`{"op":"derive_url_name","input":{"uri":"https://example.com/skills/my-skill.md","body_classification":"single-file","frontmatter_name":"declared"}}` + "\n" +
		`{"op":"evaluate_freshness","input":{"record":{"fetched_at":"2026-05-01T00:00:00Z","expires":"2026-05-02T00:00:00Z"},"consumer_clock":"2026-05-03T00:00:00Z","policies":["freshness-enforcement-opt-in"]}}` + "\n" +
		`{"op":"project","input":{"projection":"tuple_endpoint","item":{"pack_members":[{"item_id":"one","publisher_section":"present"},{"item_id":"two","publisher_section":"absent"}]}}}` + "\n" +
		`{"op":"project","input":{"projection":"install_scope_capabilities","item":{"filesystem":{"read":true}}}}` + "\n" +
		`{"op":"project","input":{"projection":"advisory","item":{"warning":{"text":"needs review"}}}}` + "\n" +
		`{"op":"evaluate_install","input":{"item":{"cross_references":[{"resolution":"revoked"}]}}}` + "\n" +
		`{"op":"evaluate_install","input":{"item":{"registry_section":{"source_uri":"https://example.com/skill.md"}}}}` + "\n" +
		`{"op":"ingest","input":{"kind":"skill","sidecar":{"registry_section":{"source_uri":"https://example.com/skill.md"}}}}` + "\n" +
		`{"op":"ingest","input":{"kind":"skill","sidecar":{"registry_section":{}}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 11 {
		t.Fatalf("responses = %d, want 11", len(responses))
	}

	normalized := responses[0]["result"].(map[string]any)
	if normalized["source_uri"] != "https://github.io/A%3Ab" {
		t.Fatalf("normalize_uri success = %#v", normalized)
	}
	if responses[1]["ok"] != false || responses[1]["error"] != "acif.source_uri.query_present" {
		t.Fatalf("normalize_uri query error = %#v", responses[1])
	}

	name := responses[2]["result"].(map[string]any)
	if name["url_derived_name"] != "my-skill" {
		t.Fatalf("derive_url_name = %#v", name)
	}
	diag := name["diagnostics"].([]any)[0].(map[string]any)
	if diag["id"] != "acif.source_uri.filename_conflict" {
		t.Fatalf("derive_url_name diagnostics = %#v", name["diagnostics"])
	}

	freshness := responses[3]["result"].(map[string]any)
	if freshness["staleness"] != "stale" || freshness["install"] != "refuse" || freshness["response_hash"] == "" {
		t.Fatalf("evaluate_freshness = %#v", freshness)
	}
	if _, ok := freshness["combined_scalar"]; ok {
		t.Fatalf("evaluate_freshness combined_scalar present: %#v", freshness)
	}

	tuple := responses[4]["result"].(map[string]any)["projection"].(map[string]any)
	if tuple["member_1"].(map[string]any)["metadata_hash"] != "present" || tuple["member_2"].(map[string]any)["metadata_hash"] != nil {
		t.Fatalf("tuple endpoint = %#v", tuple)
	}
	scopeVerdict := responses[5]["result"].(map[string]any)
	if scopeVerdict["conformant"] != false || scopeVerdict["reason"] != "acif.registry.provenance_tag_missing" {
		t.Fatalf("install scope capabilities = %#v", scopeVerdict)
	}
	advisoryVerdict := responses[6]["result"].(map[string]any)
	if advisoryVerdict["conformant"] != false || advisoryVerdict["reason"] != "acif.registry.method_stamp_missing" {
		t.Fatalf("registry advisory = %#v", advisoryVerdict)
	}

	if got := responses[7]["result"].(map[string]any)["install"]; got != "refuse-unless-operator-opt-in" {
		t.Fatalf("evaluate_install cross refs = %#v", responses[7])
	}
	if got := responses[8]["result"].(map[string]any)["install"]; got != "proceed" {
		t.Fatalf("evaluate_install non-hook = %#v", responses[8])
	}
	if result := responses[9]["result"].(map[string]any); result["conformant"] != true {
		t.Fatalf("registry emit valid = %#v", result)
	}
	if responses[10]["ok"] != false || responses[10]["error"] != "acif.source_uri.missing" {
		t.Fatalf("registry emit missing source_uri = %#v", responses[10])
	}
}

func TestRegistryStage4FetchURINDJSON(t *testing.T) {
	t.Parallel()

	server, trustCA := newAdapterTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			w.Header().Set("Location", "/target")
			w.WriteHeader(http.StatusMovedPermanently)
		case "/target":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		default:
			http.NotFound(w, r)
		}
	})
	defer server.Close()

	request := `{"op":"fetch_uri","input":{"url":"https://127.0.0.1/start","trust_ca":` + mustJSON(t, trustCA) + `,"resolve":{"127.0.0.1":` + mustJSON(t, server.Listener.Addr().String()) + `}}}` + "\n"
	responses := runLines(t, request)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	result := responses[0]["result"].(map[string]any)
	if result["source_uri"] != "https://127.0.0.1/target" {
		t.Fatalf("fetch_uri = %#v", responses[0])
	}
	if _, ok := result["source_status"]; ok {
		t.Fatalf("fetch_uri emitted source_status: %#v", result)
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

func newAdapterTLSServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox blocks local sockets: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.StartTLS()

	cert := server.Certificate()
	if cert == nil {
		t.Fatal("test TLS server has no certificate")
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if pemBytes == nil {
		t.Fatal("encoding test TLS certificate PEM")
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatalf("write trust CA: %v", err)
	}
	return server, path
}
