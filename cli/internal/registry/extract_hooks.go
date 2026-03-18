package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// UserScopedHook represents a single hook extracted from a user settings file.
type UserScopedHook struct {
	Name         string // derived: event-matcher or event-index
	Event        string
	Index        int
	Definition   json.RawMessage // the matcher group JSON
	Command      string          // first command (for display)
	ScriptPath   string          // if command references a script file
	ScriptInRepo bool            // whether the script exists in the repo
}

// ScanUserHooks reads a provider settings file and extracts individual hooks.
// repoRoot is used to determine if referenced scripts are in-repo.
func ScanUserHooks(settingsPath string, repoRoot string) ([]UserScopedHook, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	hooksObj := gjson.GetBytes(data, "hooks")
	if !hooksObj.Exists() || !hooksObj.IsObject() {
		return nil, nil
	}

	var hooks []UserScopedHook
	hooksObj.ForEach(func(event, entries gjson.Result) bool {
		if !entries.IsArray() {
			return true
		}
		var idx int
		entries.ForEach(func(_, entry gjson.Result) bool {
			matcher := entry.Get("matcher").String()

			name := strings.ToLower(event.String())
			if matcher != "" {
				safe := strings.NewReplacer("|", "-", " ", "-").Replace(strings.ToLower(matcher))
				name += "-" + safe
			} else {
				name += fmt.Sprintf("-%d", idx)
			}

			cmd := entry.Get("hooks.0.command").String()

			h := UserScopedHook{
				Name:       name,
				Event:      event.String(),
				Index:      idx,
				Definition: []byte(entry.Raw),
				Command:    cmd,
			}

			// Check if command references a script file
			if cmd != "" {
				// Strip any leading args/flags — take first token
				firstToken := cmd
				if i := strings.IndexByte(cmd, ' '); i > 0 {
					firstToken = cmd[:i]
				}

				if filepath.IsAbs(firstToken) {
					h.ScriptPath = firstToken
					rel, relErr := filepath.Rel(repoRoot, firstToken)
					h.ScriptInRepo = relErr == nil && !strings.HasPrefix(rel, "..")
				} else if strings.HasPrefix(firstToken, "./") || strings.HasPrefix(firstToken, "../") {
					h.ScriptPath = filepath.Join(repoRoot, firstToken)
					_, statErr := os.Stat(h.ScriptPath)
					h.ScriptInRepo = statErr == nil
				}
			}

			hooks = append(hooks, h)
			idx++
			return true
		})
		return true
	})

	return hooks, nil
}

// ExtractHooksToDir copies selected user-scoped hooks into targetDir.
// Creates targetDir/<hook-name>/hook.json for each hook.
// Copies script files if they're in the repo.
func ExtractHooksToDir(hooks []UserScopedHook, targetDir string) error {
	for _, h := range hooks {
		hookDir := filepath.Join(targetDir, h.Name)
		if err := os.MkdirAll(hookDir, 0755); err != nil {
			return fmt.Errorf("creating hook dir %s: %w", hookDir, err)
		}

		// Build flat-format hook.json with event at top level
		hookJSON := map[string]interface{}{
			"event": h.Event,
		}
		var def map[string]interface{}
		if err := json.Unmarshal(h.Definition, &def); err == nil {
			if m, ok := def["matcher"]; ok {
				hookJSON["matcher"] = m
			}
			if hks, ok := def["hooks"]; ok {
				hookJSON["hooks"] = hks
			}
		}

		data, err := json.MarshalIndent(hookJSON, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling hook %s: %w", h.Name, err)
		}
		if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), data, 0644); err != nil {
			return fmt.Errorf("writing hook %s: %w", h.Name, err)
		}

		// Copy script if in-repo
		if h.ScriptPath != "" && h.ScriptInRepo {
			scriptData, readErr := os.ReadFile(h.ScriptPath)
			if readErr == nil {
				scriptDest := filepath.Join(hookDir, filepath.Base(h.ScriptPath))
				if err := os.WriteFile(scriptDest, scriptData, 0755); err != nil {
					return fmt.Errorf("copying script for %s: %w", h.Name, err)
				}
			}
		}
	}
	return nil
}
