package analyzer

import (
	"math"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// floatsEqual is a test-only near-equality predicate for pinned score
// assertions. Scoring sums IEEE 754 doubles (e.g., 0.40 + 0.20 =
// 0.6000000000000001), so strict == comparison is wrong — the production
// code never compares two summed floats for equality either. Epsilon is
// tight enough to catch real constant shifts (0.01 and larger) while
// tolerating the well-known cumulative-rounding noise.
func floatsEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

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
	score := d.computeScore([]SignalEntry{{Weight: 0.10}})
	if score >= 0.55 {
		t.Errorf("base(0.40)+keyword(0.10)=0.50 must be below 0.55 floor, got %.2f", score)
	}
}

func TestContentSignalDetector_FinalScore_Cap(t *testing.T) {
	t.Parallel()
	d := &ContentSignalDetector{}
	// Many signals should be capped at 0.70.
	score := d.computeScore([]SignalEntry{
		{Weight: 0.25},
		{Weight: 0.25},
		{Weight: 0.20},
		{Weight: 0.15},
		{Weight: 0.10},
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
func sumWeights(signals []SignalEntry) float64 {
	var total float64
	for _, s := range signals {
		total += s.Weight
	}
	return total
}

// TestContentSignalDetector_ScoreConstants_Pinned locks the three numeric
// constants that drive content-signal scoring to their known-good values.
// Any silent drift of these constants flips the Auto/Confirm/Skip partition
// decisions for every unmatched file the analyzer sees — a user-facing
// behavior change that no existing range-based test (uses >=) would catch.
// When intentionally changing a constant, update this test in the same
// commit so the intent is auditable.
func TestContentSignalDetector_ScoreConstants_Pinned(t *testing.T) {
	t.Parallel()

	if contentSignalBase != 0.40 {
		t.Errorf("regression: contentSignalBase must be 0.40 — any shift silently rebases every content-signal score and moves items across partition boundaries; got %v", contentSignalBase)
	}
	if contentSignalFloor != 0.55 {
		t.Errorf("regression: contentSignalFloor must be 0.55 — this is the emit gate; lowering it lets weak-signal junk through, raising it silently drops currently-classified items; got %v", contentSignalFloor)
	}
	if contentSignalCap != 0.70 {
		t.Errorf("regression: contentSignalCap must be 0.70 — raising it past DefaultAutoThreshold (0.80) would let content-signal items auto-install without user confirmation; got %v", contentSignalCap)
	}
}

// TestContentSignalDetector_ScoringInvariants pins the two architectural
// relationships between the detector's emit/cap constants and the
// analyzer-level partition thresholds. These are product guarantees, not
// coincidences: (1) every emitted content-signal item must survive partition
// (floor >= skip), and (2) no content-signal item can ever reach Auto
// (cap < auto). A future tweak that violates either would silently ship
// incorrect partition behavior to every user.
func TestContentSignalDetector_ScoringInvariants(t *testing.T) {
	t.Parallel()

	if contentSignalFloor < DefaultSkipThreshold {
		t.Errorf("invariant: contentSignalFloor (%.2f) must be >= DefaultSkipThreshold (%.2f) — otherwise an item could pass the emit gate only to be silently dropped at partition, leaving users wondering why detection is inconsistent",
			contentSignalFloor, DefaultSkipThreshold)
	}
	if contentSignalCap >= DefaultAutoThreshold {
		t.Errorf("invariant: contentSignalCap (%.2f) must be < DefaultAutoThreshold (%.2f) — content-signal items must always land in Confirm so the user gets a chance to reject false positives; raising the cap would turn Confirm into Auto and break that contract",
			contentSignalCap, DefaultAutoThreshold)
	}
}

// TestContentSignalDetector_ComputeScore_ExactValues pins the output of
// computeScore for N fixed signal combinations. Existing tests use range
// assertions (>= 0.25) which let a shifted weight constant pass; these
// tests use exact equality so any drift is caught. The cases are chosen to
// cover: base-only, each relevant partition boundary (skip 0.50, floor
// 0.55, medium 0.60, high 0.70/cap), and well past the cap.
func TestContentSignalDetector_ComputeScore_ExactValues(t *testing.T) {
	t.Parallel()

	d := &ContentSignalDetector{}

	cases := []struct {
		name    string
		signals []SignalEntry
		want    float64
		// meaning documents why the value matters so a future maintainer
		// reading a failure understands which partition boundary moved.
		meaning string
	}{
		{
			name:    "no signals yields base only",
			signals: nil,
			want:    0.40,
			meaning: "bare base score is below DefaultSkipThreshold (0.50) so this item is dropped at partition",
		},
		{
			name:    "single 0.10 weight lands at skip threshold",
			signals: []SignalEntry{{Weight: 0.10}},
			want:    0.50,
			meaning: "exactly at DefaultSkipThreshold (0.50); still below contentSignalFloor (0.55) so does not emit",
		},
		{
			name:    "single 0.15 weight lands exactly at emit floor",
			signals: []SignalEntry{{Weight: 0.15}},
			want:    0.55,
			meaning: "exactly at contentSignalFloor; this is the just-above-skip / just-at-floor boundary",
		},
		{
			name:    "single 0.20 weight lands at tier-medium boundary",
			signals: []SignalEntry{{Weight: 0.20}},
			want:    0.60,
			meaning: "the TierForMeta Low/Medium boundary (0.60); user-directed base+boost also lands here",
		},
		{
			name:    "signals summing to 0.30 land at cap exactly",
			signals: []SignalEntry{{Weight: 0.15}, {Weight: 0.15}},
			want:    0.70,
			meaning: "exactly at contentSignalCap (0.70); below DefaultAutoThreshold so still needs Confirm",
		},
		{
			name:    "signals summing past cap are clamped",
			signals: []SignalEntry{{Weight: 0.25}, {Weight: 0.25}, {Weight: 0.20}, {Weight: 0.15}, {Weight: 0.10}},
			want:    0.70,
			meaning: "raw total 1.35 must be clamped to cap so content-signal never reaches Auto",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := d.computeScore(tc.signals)
			if !floatsEqual(got, tc.want) {
				t.Errorf("regression: computeScore(%v) = %v, want %v (%s) — a shift in any scoring constant would surface here before it silently reshuffles the Auto/Confirm/Skip partition for users",
					tc.signals, got, tc.want, tc.meaning)
			}
		})
	}
}

// TestContentSignalDetector_ComputeScore_EmitFloorBoundary pins the three
// points that matter for the emit gate inside ClassifyUnmatched (line 186,
// "if confidence < contentSignalFloor"). Without this pin, a rounding
// change or weight tweak could silently move an item from "emits" to
// "silently dropped" and vice versa.
func TestContentSignalDetector_ComputeScore_EmitFloorBoundary(t *testing.T) {
	t.Parallel()

	d := &ContentSignalDetector{}

	justBelow := d.computeScore([]SignalEntry{{Weight: 0.10}}) // 0.50
	atFloor := d.computeScore([]SignalEntry{{Weight: 0.15}})   // 0.55
	justAbove := d.computeScore([]SignalEntry{{Weight: 0.20}}) // 0.60

	if justBelow >= contentSignalFloor {
		t.Errorf("regression: base+0.10=%v must be strictly below contentSignalFloor (%.2f) — this is the documented 'dropped' case in TestContentSignalDetector_FinalScore_Floor and the audit", justBelow, contentSignalFloor)
	}
	if !floatsEqual(atFloor, contentSignalFloor) {
		t.Errorf("regression: base+0.15=%v must equal contentSignalFloor (%.2f) — this is the boundary value that determines emit vs. silent drop", atFloor, contentSignalFloor)
	}
	if justAbove <= contentSignalFloor {
		t.Errorf("regression: base+0.20=%v must be strictly above contentSignalFloor (%.2f) — if this becomes equal, items that currently emit would silently drop", justAbove, contentSignalFloor)
	}
}
