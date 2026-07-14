package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

var (
	mcpServerNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	mcpEnvVarRe     = regexp.MustCompile(`\$\{[^}]+\}`)
)

func CanonicalizeMCP(block map[string]any) (*RecordResult, error) {
	canonicalBlock, diagnostics, verdict, err := canonicalizeMCPBlock(block)
	if err != nil {
		return nil, err
	}
	record := map[string]any{
		"kind": "mcp_config",
		"mcp":  canonicalBlock,
	}
	recordRaw, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	canonicalBytes, err := CanonicalJSON(recordRaw)
	if err != nil {
		return nil, err
	}
	bodyHash, err := mcpBodyHash(canonicalBlock)
	if err != nil {
		return nil, err
	}
	result := &RecordResult{
		Conformant:     true,
		Installable:    true,
		Canonical:      record,
		CanonicalBytes: string(canonicalBytes),
		BodyHash:       bodyHash,
		Diagnostics:    diagnostics,
	}
	if verdict != nil {
		result.Conformant = false
		result.Installable = false
		result.Reason = verdict.Reason
		result.Params = verdict.Params
	}
	return result, nil
}

func canonicalizeMCPBlock(block map[string]any) (map[string]any, []Diagnostic, *HookVerdict, error) {
	block = unwrapKindBlock(block, "mcp")
	rawServers, ok := block["servers"].(map[string]any)
	if !ok || len(rawServers) == 0 {
		return nil, nil, nil, &RejectError{ID: ErrMCPServersMissing}
	}

	names := make([]string, 0, len(rawServers))
	for name := range rawServers {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make(map[string]any, len(rawServers))
	var diagnostics []Diagnostic
	for _, name := range names {
		rawServer, _ := rawServers[name].(map[string]any)
		if rawServer == nil {
			rawServer = map[string]any{}
		}
		server, err := canonicalizeMCPServer(rawServer)
		if err != nil {
			return nil, nil, nil, err
		}
		if !mcpServerNameRe.MatchString(name) {
			diagnostics = append(diagnostics, Diagnostic{
				ID:     DiagMCPServerNameUnconventional,
				Params: map[string]any{"server": name},
			})
		}
		servers[name] = server
	}

	out := map[string]any{"servers": servers}
	if requires, ok := block["requires"]; ok {
		out["requires"] = cloneJSONValue(requires)
	}
	verdict := applyRequiresVerdict(out)
	return out, diagnostics, verdict, nil
}

func canonicalizeMCPServer(server map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(server)+1)
	for k, v := range server {
		switch k {
		case "type", "includeTools", "excludeTools", "disabledTools", "autoApprove":
			continue
		default:
			out[k] = cloneJSONValue(v)
		}
	}

	transport, hasType := server["type"].(string)
	if hasType {
		switch transport {
		case "sse", "stdio", "streamable-http":
		default:
			return nil, &RejectError{ID: ErrMCPTransportTypeInvalid, Detail: transport}
		}
	} else {
		_, hasCommand := server["command"]
		_, hasURL := server["url"]
		switch {
		case hasCommand && !hasURL:
			transport = "stdio"
		case hasURL && !hasCommand:
			transport = "streamable-http"
		case hasCommand && hasURL:
			return nil, &RejectError{ID: ErrMCPTransportDefaultAmbiguous}
		default:
			return nil, &RejectError{ID: ErrMCPTransportDefaultUndetermined}
		}
	}
	out["type"] = transport

	for _, key := range []string{"includeTools", "excludeTools", "disabledTools", "autoApprove"} {
		if raw, ok := server[key]; ok {
			out[key] = sortedDedupedArray(raw)
		}
	}
	return out, nil
}

func sortedDedupedArray(raw any) any {
	items, ok := raw.([]any)
	if !ok {
		return cloneJSONValue(raw)
	}
	type entry struct {
		key   string
		value any
	}
	seen := make(map[string]bool, len(items))
	entries := make([]entry, 0, len(items))
	for _, item := range items {
		key := canonicalSortKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry{key: key, value: cloneJSONValue(item)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	out := make([]any, len(entries))
	for i, entry := range entries {
		out[i] = entry.value
	}
	return out
}

func canonicalSortKey(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return string(raw)
	}
	return string(canonical)
}

func mcpBodyHash(block map[string]any) (string, error) {
	raw, err := json.Marshal(block)
	if err != nil {
		return "", err
	}
	wire, err := CanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	digestHeader := hookSHA256Prefix + sha256Hex(nil)
	h := sha256.New()
	_, _ = h.Write([]byte(digestHeader))
	_, _ = h.Write([]byte{'\n'})
	_, _ = h.Write(wire)
	_, _ = h.Write([]byte{'\n'})
	return hex.EncodeToString(h.Sum(nil)), nil
}

func CanonicalizeMCPProviderConfig(config map[string]any) (*RecordResult, error) {
	if provider, _ := config["provider"].(string); provider != "no-type-format" {
		return nil, &RejectError{ID: ErrMCPTransportDefaultUndetermined}
	}
	var block map[string]any
	switch content := config["content"].(type) {
	case string:
		if err := decodeJSONUseNumberBytes([]byte(content), &block); err != nil {
			return nil, err
		}
	case map[string]any:
		block = content
	default:
		return nil, fmt.Errorf("mcp provider_config content must be JSON string")
	}
	return CanonicalizeMCP(block)
}

func mcpDerivedCapabilities(item map[string]any) (map[string]bool, error) {
	block, _, _, err := canonicalizeMCPBlock(item)
	if err != nil {
		return nil, err
	}
	servers, _ := block["servers"].(map[string]any)
	caps := map[string]bool{
		"transport_types":   true,
		"oauth_support":     false,
		"env_var_expansion": false,
		"tool_filtering":    false,
		"auto_approve":      false,
	}
	for _, rawServer := range servers {
		server, _ := rawServer.(map[string]any)
		if _, ok := server["oauth"]; ok {
			caps["oauth_support"] = true
		}
		if _, ok := server["autoApprove"]; ok {
			caps["auto_approve"] = true
		}
		for _, key := range []string{"includeTools", "excludeTools", "disabledTools"} {
			if _, ok := server[key]; ok {
				caps["tool_filtering"] = true
			}
		}
		if serverHasEnvVarExpansion(server) {
			caps["env_var_expansion"] = true
		}
	}
	return caps, nil
}

func serverHasEnvVarExpansion(server map[string]any) bool {
	for _, key := range []string{"command", "url"} {
		if s, ok := server[key].(string); ok && mcpEnvVarRe.MatchString(s) {
			return true
		}
	}
	for _, arg := range anyStringSlice(server["args"]) {
		if mcpEnvVarRe.MatchString(arg) {
			return true
		}
	}
	for _, key := range []string{"env", "headers"} {
		values, _ := server[key].(map[string]any)
		for _, rawValue := range values {
			if s, ok := rawValue.(string); ok && mcpEnvVarRe.MatchString(s) {
				return true
			}
		}
	}
	return false
}

func RenderMCP(item map[string]any, target string) (*RenderResult, error) {
	if target != "no-type-format" {
		return &RenderResult{Unsupported: true}, nil
	}
	block, _, _, err := canonicalizeMCPBlock(item)
	if err != nil {
		return nil, err
	}
	servers, _ := block["servers"].(map[string]any)
	outServers := make(map[string]any, len(servers))
	for name, rawServer := range servers {
		server, _ := rawServer.(map[string]any)
		clean := make(map[string]any, len(server))
		for k, v := range server {
			if k == "type" {
				continue
			}
			clean[k] = cloneJSONValue(v)
		}
		outServers[name] = clean
	}
	raw, err := json.Marshal(map[string]any{"servers": outServers})
	if err != nil {
		return nil, err
	}
	return &RenderResult{Output: string(raw)}, nil
}
