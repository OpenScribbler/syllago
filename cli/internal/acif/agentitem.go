package acif

import (
	"fmt"
	"sort"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"gopkg.in/yaml.v3"
)

func CanonicalizeAgent(block map[string]any, provider string) (*ItemResult, error) {
	block = unwrapKindBlock(block, "agent")
	out := make(map[string]any, len(block))
	for k, v := range block {
		switch k {
		case "tools", "disallowed_tools":
			out[k] = translateAgentToolList(v, provider)
		default:
			out[k] = cloneJSONValue(v)
		}
	}
	verdict := applyRequiresVerdict(out)
	return &ItemResult{Canonical: out, Verdict: verdict}, nil
}

func translateAgentToolList(raw any, provider string) any {
	items, ok := raw.([]any)
	if !ok {
		return cloneJSONValue(raw)
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok || name == "" {
			out = append(out, cloneJSONValue(item))
			continue
		}
		out = append(out, canonicalAgentToolName(name, provider))
	}
	return out
}

func canonicalAgentToolName(name, provider string) string {
	if _, ok := converter.ToolNames[name]; ok {
		return name
	}
	if provider != "" {
		if provider == "collapsing-provider" {
			if name == "edit_file" {
				return "file_edit"
			}
		} else {
			reversed := converter.ReverseTranslateTool(name, provider)
			if reversed != name {
				return reversed
			}
			if _, ok := converter.ToolNames[reversed]; ok {
				return reversed
			}
		}
	}
	matches := matchingCanonicalToolsAnyProvider(name)
	for _, match := range matches {
		if match == "file_edit" {
			return "file_edit"
		}
	}
	if len(matches) == 0 && strings.EqualFold(name, "task") {
		return "agent"
	}
	if len(matches) > 0 {
		sort.Strings(matches)
		return matches[0]
	}
	return name
}

func matchingCanonicalToolsAnyProvider(name string) []string {
	var matches []string
	for canonical, providers := range converter.ToolNames {
		for _, native := range providers {
			if native == name {
				matches = append(matches, canonical)
				break
			}
		}
	}
	return matches
}

func agentDerivedCapabilities(item map[string]any) (map[string]bool, error) {
	block, _ := unwrapItemBlock(item, "agent")
	result, err := CanonicalizeAgent(block, "")
	if err != nil {
		return nil, err
	}
	tools := anyStringSlice(result.Canonical["tools"])
	return map[string]bool{
		"tool_restrictions": len(tools) > 0 || len(anyStringSlice(result.Canonical["disallowed_tools"])) > 0,
		"model_selection":   nonEmptyString(result.Canonical["model"]),
		"per_agent_mcp":     len(anyStringSlice(result.Canonical["mcp_servers"])) > 0,
		"subagent_spawning": stringSliceContains(tools, "agent"),
	}, nil
}

func RenderAgent(item map[string]any, target string) (*RenderResult, error) {
	block, _ := unwrapItemBlock(item, "agent")
	result, err := CanonicalizeAgent(block, "")
	if err != nil {
		return nil, err
	}
	if !isAgentRenderTarget(target) {
		return &RenderResult{Unsupported: true}, nil
	}

	frontmatter := make(map[string]any)
	for k, v := range result.Canonical {
		if k == "body" || k == "requires" {
			continue
		}
		switch k {
		case "tools", "disallowed_tools":
			frontmatter[k] = translateAgentToolsForProvider(anyStringSlice(v), target)
		default:
			frontmatter[k] = cloneJSONValue(v)
		}
	}
	body, _ := result.Canonical["body"].(string)
	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return nil, err
	}
	rendered := "---\n" + string(yamlBytes) + "---\n" + body
	out := &RenderResult{Output: rendered}
	if hasWriteEditLoss(result.Canonical, target) {
		out.Lossy = []string{"write-edit-distinction"}
	}
	return out, nil
}

func translateAgentToolsForProvider(tools []string, target string) []any {
	out := make([]any, len(tools))
	for i, tool := range tools {
		if target == "collapsing-provider" {
			switch tool {
			case "file_edit", "file_write":
				out[i] = "edit_file"
			default:
				out[i] = tool
			}
			continue
		}
		out[i] = converter.TranslateTool(tool, target)
	}
	return out
}

func hasWriteEditLoss(block map[string]any, target string) bool {
	tools := anyStringSlice(block["tools"])
	seenNative := make(map[string]string)
	for _, tool := range tools {
		native := tool
		if target == "collapsing-provider" {
			if tool == "file_edit" || tool == "file_write" {
				native = "edit_file"
			}
		} else {
			native = converter.TranslateTool(tool, target)
		}
		if prior, ok := seenNative[native]; ok && prior != tool {
			return true
		}
		seenNative[native] = tool
	}
	return false
}

func CanonicalizeAgentProviderConfig(config map[string]any) (*ItemResult, error) {
	provider, _ := config["provider"].(string)
	content, ok := config["content"].(string)
	if !ok {
		return nil, fmt.Errorf("agent provider_config content must be string")
	}
	doc, err := parseFrontmatterDocument([]byte(content))
	if err != nil {
		return nil, err
	}
	block := map[string]any{}
	for k, v := range doc.Frontmatter {
		block[k] = cloneJSONValue(v)
	}
	if doc.Present || doc.Body != "" {
		block["body"] = doc.Body
	}
	return CanonicalizeAgent(block, provider)
}

func isAgentRenderTarget(target string) bool {
	switch target {
	case "claude-code", "copilot-cli", "opencode", "vs-code-copilot", "zed", "codex", "kiro", "factory-droid", "collapsing-provider":
		return true
	default:
		return false
	}
}

func unwrapKindBlock(block map[string]any, key string) map[string]any {
	if block == nil {
		return map[string]any{}
	}
	if inner, ok := block[key].(map[string]any); ok {
		return inner
	}
	return block
}

func unwrapItemBlock(item map[string]any, key string) (map[string]any, string) {
	if item == nil {
		return map[string]any{}, ""
	}
	classification, _ := item["classification"].(string)
	if inner, ok := item[key].(map[string]any); ok {
		return inner, classification
	}
	return item, classification
}

func nonEmptyString(v any) bool {
	s, ok := v.(string)
	return ok && s != ""
}

func stringSliceContains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
