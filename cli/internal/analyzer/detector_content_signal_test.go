package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestContentSignalDetector_PreFilter_ExtensionReject(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	if d.passesPreFilter("agents/foo/bar.go", false) {
		t.Error("expected .go to be rejected by pre-filter")
	}
	if d.passesPreFilter("agents/foo/bar.rs", false) {
		t.Error("expected .rs to be rejected by pre-filter")
	}
}

func TestContentSignalDetector_PreFilter_ExtensionAccept(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	exts := []string{".md", ".yaml", ".yml", ".json", ".toml"}
	for _, ext := range exts {
		path := "agents/foo/bar" + ext
		if !d.passesPreFilter(path, false) {
			t.Errorf("expected %q to pass pre-filter", path)
		}
	}
}

func TestContentSignalDetector_PreFilter_DirectoryKeyword(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	if !d.passesPreFilter("skills/foo/bar.md", false) {
		t.Error("path with 'skills' keyword should pass")
	}
	if d.passesPreFilter("docs/api/overview.md", false) {
		t.Error("path with no keyword should fail without userDirected")
	}
	if !d.passesPreFilter("docs/api/overview.md", true) {
		t.Error("userDirected path should bypass directory keyword check")
	}
}

func TestContentSignalDetector_ScoreFilename_SKILL(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	signals := d.scoreFilename("SKILL.md", catalog.Skills)
	total := sumWeights(signals)
	if total < 0.25 {
		t.Errorf("SKILL.md filename should score >= 0.25 for skills, got %.2f", total)
	}
}

func TestContentSignalDetector_ScoreFilename_AgentYAML(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	signals := d.scoreFilename("my-agent.agent.yaml", catalog.Agents)
	total := sumWeights(signals)
	if total < 0.20 {
		t.Errorf("*.agent.yaml should score >= 0.20 for agents, got %.2f", total)
	}
}

func TestContentSignalDetector_ScoreJSON_MCPServers(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	data := []byte(`{"mcpServers": {"myserver": {"command": "npx"}}}`)
	signals, ct := d.scoreJSON(data)
	if ct != catalog.MCP {
		t.Errorf("mcpServers JSON should detect MCP type, got %v", ct)
	}
	total := sumWeights(signals)
	if total < 0.30 {
		t.Errorf("mcpServers signal should score >= 0.30, got %.2f", total)
	}
}

func TestContentSignalDetector_ScoreJSON_HooksWiring(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	data := []byte(`{
		"hooks": {
			"PreToolUse": [{"command": "bash hooks/lint.sh"}],
			"PostToolUse": [{"command": "echo done"}]
		}
	}`)
	signals, ct := d.scoreJSON(data)
	if ct != catalog.Hooks {
		t.Errorf("hooks JSON with event names should detect Hooks type, got %v", ct)
	}
	total := sumWeights(signals)
	if total < 0.25 {
		t.Errorf("hooks wiring should score >= 0.25, got %.2f", total)
	}
}

func TestContentSignalDetector_ScoreJSON_HooksOnlyOneEvent(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	data := []byte(`{"hooks": {"PreToolUse": [], "onSave": []}}`)
	_, ct := d.scoreJSON(data)
	if ct == catalog.Hooks {
		t.Error("single known event name should not trigger hooks classification")
	}
}

func TestContentSignalDetector_ScoreFrontmatter_CommandFields(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	data := []byte("---\nallowed-tools: [Bash, Read]\nargument-hint: \"<filename>\"\n---\nDo something.\n")
	signals, ct := d.scoreFrontmatter(data)
	if ct != catalog.Commands {
		t.Errorf("allowed-tools + argument-hint should detect Commands, got %v", ct)
	}
	total := sumWeights(signals)
	if total < 0.35 {
		t.Errorf("command frontmatter signals should total >= 0.35, got %.2f", total)
	}
}

func TestContentSignalDetector_ScoreFrontmatter_RuleFields(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	data := []byte("---\nalwaysApply: true\nglobs: [\"*.go\"]\n---\nRule content.\n")
	signals, ct := d.scoreFrontmatter(data)
	if ct != catalog.Rules {
		t.Errorf("alwaysApply + globs should detect Rules, got %v", ct)
	}
	_ = signals
}

func TestContentSignalDetector_FinalScore_Floor(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	// Base 0.40 + directory keyword 0.10 = 0.50 — must be dropped (below 0.55 floor).
	score := d.computeScore([]signalEntry{{weight: 0.10}})
	if score >= 0.55 {
		t.Errorf("base(0.40)+keyword(0.10)=0.50 must be below 0.55 floor, got %.2f", score)
	}
}

func TestContentSignalDetector_FinalScore_Cap(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	// Many signals should be capped at 0.70.
	score := d.computeScore([]signalEntry{
		{weight: 0.25},
		{weight: 0.25},
		{weight: 0.20},
		{weight: 0.15},
		{weight: 0.10},
	})
	if score > 0.70 {
		t.Errorf("score should be capped at 0.70, got %.2f", score)
	}
}

func TestContentSignalDetector_TypePriority_CommandsOverSkills(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	priority := d.typePriority(catalog.Commands)
	skillPriority := d.typePriority(catalog.Skills)
	if priority >= skillPriority {
		t.Errorf("Commands should have lower priority value (higher specificity) than Skills: commands=%d skills=%d", priority, skillPriority)
	}
}

func TestContentSignalDetector_GlobalREADMEExcluded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "agents/README.md", "# Agents\nThis is documentation.\n")
	d := &ContentSignalDetector{}
	items, _, err := d.ClassifyUnmatched([]string{"agents/README.md"}, root, nil, false)
	if err != nil {
		t.Fatalf("ClassifyUnmatched error: %v", err)
	}
	if len(items) > 0 {
		t.Errorf("README.md in agents/ should be excluded, got %d items", len(items))
	}
}

// sumWeights is a test helper.
func sumWeights(signals []signalEntry) float64 {
	var total float64
	for _, s := range signals {
		total += s.weight
	}
	return total
}
