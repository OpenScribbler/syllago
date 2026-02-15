// cli/internal/catalog/detect_test.go
package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectContent(t *testing.T) {
	tmp := t.TempDir()

	// Create a valid skill directory
	skillDir := filepath.Join(tmp, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\ndescription: A test skill\n---\n# Test"), 0644)

	// Create a valid agent directory
	agentDir := filepath.Join(tmp, "my-agent")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("---\nname: test-agent\n---\n"), 0644)

	// Create a valid prompt directory
	promptDir := filepath.Join(tmp, "my-prompt")
	os.MkdirAll(promptDir, 0755)
	os.WriteFile(filepath.Join(promptDir, "PROMPT.md"), []byte("---\nname: test-prompt\n---\n"), 0644)

	// Create a valid app directory (README.md with frontmatter)
	appDir := filepath.Join(tmp, "my-app")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "README.md"), []byte("---\nname: test-app\ndescription: A test app\n---\n# App"), 0644)

	// Create a directory with no recognized content
	emptyDir := filepath.Join(tmp, "random")
	os.MkdirAll(emptyDir, 0755)
	os.WriteFile(filepath.Join(emptyDir, "notes.txt"), []byte("nothing here"), 0644)

	// Create a plain file
	plainFile := filepath.Join(tmp, "rule.md")
	os.WriteFile(plainFile, []byte("# A rule"), 0644)

	tests := []struct {
		name     string
		path     string
		wantType string
		wantOK   bool
	}{
		{"skill dir", skillDir, "Skill", true},
		{"agent dir", agentDir, "Agent", true},
		{"prompt dir", promptDir, "Prompt", true},
		{"app dir", appDir, "App", true},
		{"no content dir", emptyDir, "", false},
		{"markdown file", plainFile, ".md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotOK := DetectContent(tt.path)
			if gotOK != tt.wantOK {
				t.Errorf("DetectContent(%s) ok = %v, want %v", tt.name, gotOK, tt.wantOK)
			}
			if gotType != tt.wantType {
				t.Errorf("DetectContent(%s) type = %q, want %q", tt.name, gotType, tt.wantType)
			}
		})
	}
}
