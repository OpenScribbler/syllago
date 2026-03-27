package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// RiskLevel indicates severity of a risk indicator.
type RiskLevel int

const (
	RiskMedium RiskLevel = iota
	RiskHigh
)

// RiskLine identifies a specific line in a source file where a risk was detected.
type RiskLine struct {
	File string // relative path within item (e.g., "hooks.json")
	Line int    // 1-based line number
}

// RiskIndicator represents a security-relevant characteristic of a content item.
// These are informational — they help users make informed install decisions.
type RiskIndicator struct {
	Label       string // e.g. "Runs commands"
	Description string // e.g. "Hook executes shell commands on your machine"
	Level       RiskLevel
	Lines       []RiskLine
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
		lines := strings.Split(string(data), "\n")

		// Claude Code hooks format: {"hooks": {"PostToolUse": [{"matcher":"...", "command":"..."}]}}
		// Command hooks have a "command" field; HTTP hooks have a "url" field.
		hasCommand := false
		hasURL := false
		gjson.GetBytes(data, "hooks").ForEach(func(_, eventHooks gjson.Result) bool {
			eventHooks.ForEach(func(_, hook gjson.Result) bool {
				if hook.Get("command").Exists() {
					hasCommand = true
				}
				if hook.Get("url").Exists() {
					hasURL = true
				}
				return true
			})
			return true
		})

		if hasCommand {
			var riskLines []RiskLine
			var firstCmd string
			for lineNum, line := range lines {
				if strings.Contains(line, `"command"`) {
					riskLines = append(riskLines, RiskLine{File: f, Line: lineNum + 1})
				}
			}
			// Extract the first command value for the description.
			gjson.GetBytes(data, "hooks").ForEach(func(_, eventHooks gjson.Result) bool {
				eventHooks.ForEach(func(_, hook gjson.Result) bool {
					if firstCmd == "" {
						firstCmd = hook.Get("command").String()
					}
					return firstCmd == ""
				})
				return firstCmd == ""
			})
			desc := "Hook executes shell commands on your machine"
			if firstCmd != "" {
				if len(firstCmd) > 60 {
					firstCmd = firstCmd[:57] + "..."
				}
				desc = "Runs: " + firstCmd
			}
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Runs commands",
				Description: desc,
				Level:       RiskHigh,
				Lines:       riskLines,
			})
		}
		if hasURL {
			var riskLines []RiskLine
			for lineNum, line := range lines {
				if strings.Contains(line, `"url"`) {
					riskLines = append(riskLines, RiskLine{File: f, Line: lineNum + 1})
				}
			}
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Network access",
				Description: "Hook makes HTTP requests",
				Level:       RiskMedium,
				Lines:       riskLines,
			})
		}
	}
	return risks
}

func mcpRisks(item ContentItem) []RiskIndicator {
	var risks []RiskIndicator
	risks = appendIfMissing(risks, RiskIndicator{
		Label:       "Network access",
		Description: "MCP server communicates over network",
		Level:       RiskMedium,
	})
	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")

		// MCP config format: {"mcpServers": {"name": {"env": {...}}}}
		hasEnv := false
		gjson.GetBytes(data, "mcpServers").ForEach(func(_, srv gjson.Result) bool {
			if srv.Get("env").Exists() {
				hasEnv = true
			}
			return true
		})
		if hasEnv {
			var riskLines []RiskLine
			for lineNum, line := range lines {
				if strings.Contains(line, `"env"`) {
					riskLines = append(riskLines, RiskLine{File: f, Line: lineNum + 1})
				}
			}
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Environment variables",
				Description: "MCP server reads environment variables",
				Level:       RiskMedium,
				Lines:       riskLines,
			})
		}
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
		content := string(data)
		if strings.Contains(content, "Bash") {
			var riskLines []RiskLine
			lines := strings.Split(content, "\n")
			for lineNum, line := range lines {
				if strings.Contains(line, "Bash") {
					riskLines = append(riskLines, RiskLine{File: f, Line: lineNum + 1})
				}
			}
			risks = appendIfMissing(risks, RiskIndicator{
				Label:       "Bash access",
				Description: "Content references the Bash tool — can execute arbitrary commands",
				Level:       RiskHigh,
				Lines:       riskLines,
			})
		}
	}
	return risks
}

func appendIfMissing(risks []RiskIndicator, r RiskIndicator) []RiskIndicator {
	for i, existing := range risks {
		if existing.Label == r.Label {
			// Merge lines from the new indicator into the existing one.
			risks[i].Lines = append(risks[i].Lines, r.Lines...)
			return risks
		}
	}
	return append(risks, r)
}
