package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// RiskIndicator represents a security-relevant characteristic of a content item.
// These are informational — they help users make informed install decisions.
type RiskIndicator struct {
	Label       string // e.g. "Runs commands"
	Description string // e.g. "Hook executes shell commands on your machine"
}

// RiskIndicators analyzes a ContentItem and returns any applicable risk indicators.
func RiskIndicators(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator

	switch item.Type {
	case Hooks:
		risks = append(risks, hookRisks(item)...)
	case MCP:
		risks = append(risks, mcpRisks(item)...)
	case Skills, Agents:
		risks = append(risks, skillAgentRisks(item)...)
	}

	return risks
}

func hookRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		// Claude Code hooks format: {"hooks": {"PostToolUse": [{"matcher":"...", "command":"..."}]}}
		// Command hooks have a "command" field; HTTP hooks have a "url" field.
		gjson.GetBytes(data, "hooks").ForEach(func(_, eventHooks gjson.Result) bool {
			eventHooks.ForEach(func(_, hook gjson.Result) bool {
				if hook.Get("command").Exists() {
					risks = appendIfMissing(risks, RiskIndicator{
						Label:       "Runs commands",
						Description: "Hook executes shell commands on your machine",
					})
				}
				if hook.Get("url").Exists() {
					risks = appendIfMissing(risks, RiskIndicator{
						Label:       "Network access",
						Description: "Hook makes HTTP requests",
					})
				}
				return true
			})
			return true
		})
	}
	return risks
}

func mcpRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	risks = appendIfMissing(risks, RiskIndicator{
		Label:       "Network access",
		Description: "MCP server communicates over network",
	})
	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		// MCP config format: {"mcpServers": {"name": {"env": {...}}}}
		gjson.GetBytes(data, "mcpServers").ForEach(func(_, srv gjson.Result) bool {
			if srv.Get("env").Exists() {
				risks = appendIfMissing(risks, RiskIndicator{
					Label:       "Environment variables",
					Description: "MCP server reads environment variables",
				})
			}
			return true
		})
	}
	return risks
}

func skillAgentRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	for _, f := range item.Files {
		if filepath.Ext(f) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "Bash") {
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Bash access",
				Description: "Content references the Bash tool — can execute arbitrary commands",
			})
			break
		}
	}
	return risks
}

func appendIfMissing(risks []RiskIndicator, r RiskIndicator) []RiskIndicator {
	for _, existing := range risks {
		if existing.Label == r.Label {
			return risks
		}
	}
	return append(risks, r)
}
