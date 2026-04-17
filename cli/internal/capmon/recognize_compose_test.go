package capmon

import (
	"reflect"
	"testing"
)

func TestMergeRecognitionResults_Empty(t *testing.T) {
	got := mergeRecognitionResults()
	if got.Status != StatusNotEvaluated {
		t.Errorf("status = %q, want %q", got.Status, StatusNotEvaluated)
	}
	if len(got.Capabilities) != 0 {
		t.Errorf("capabilities = %v, want empty", got.Capabilities)
	}
	if got.MissingAnchors != nil {
		t.Errorf("missing anchors = %v, want nil", got.MissingAnchors)
	}
}

func TestMergeRecognitionResults_TwoRecognized(t *testing.T) {
	skills := RecognitionResult{
		Capabilities: map[string]string{
			"skills.supported":                          "true",
			"skills.capabilities.frontmatter.supported": "true",
		},
		Status:         StatusRecognized,
		MatchedAnchors: []string{"Frontmatter reference"},
	}
	rules := RecognitionResult{
		Capabilities: map[string]string{
			"rules.supported": "true",
			"rules.capabilities.activation_mode.supported": "true",
		},
		Status:         StatusRecognized,
		MatchedAnchors: []string{"CLAUDE.md files"},
	}
	got := mergeRecognitionResults(skills, rules)
	if got.Status != StatusRecognized {
		t.Errorf("status = %q, want %q", got.Status, StatusRecognized)
	}
	want := map[string]string{
		"skills.supported":                             "true",
		"skills.capabilities.frontmatter.supported":    "true",
		"rules.supported":                              "true",
		"rules.capabilities.activation_mode.supported": "true",
	}
	if !reflect.DeepEqual(got.Capabilities, want) {
		t.Errorf("capabilities = %v, want %v", got.Capabilities, want)
	}
	wantMatched := []string{"CLAUDE.md files", "Frontmatter reference"}
	if !reflect.DeepEqual(got.MatchedAnchors, wantMatched) {
		t.Errorf("matched = %v, want %v", got.MatchedAnchors, wantMatched)
	}
}

// TestMergeRecognitionResults_StatusPrecedence verifies the precedence ladder:
// recognized > anchors_missing > not_evaluated. A single recognized input
// drives the merged status to recognized, even if other inputs failed anchors.
func TestMergeRecognitionResults_StatusPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		inputs []RecognitionResult
		want   string
	}{
		{
			name: "recognized_beats_anchors_missing",
			inputs: []RecognitionResult{
				{Capabilities: map[string]string{"skills.supported": "true"}, Status: StatusRecognized},
				{Status: StatusAnchorsMissing, MissingAnchors: []string{"X"}},
			},
			want: StatusRecognized,
		},
		{
			name: "recognized_beats_not_evaluated",
			inputs: []RecognitionResult{
				{Capabilities: map[string]string{"rules.supported": "true"}, Status: StatusRecognized},
				{Status: StatusNotEvaluated},
			},
			want: StatusRecognized,
		},
		{
			name: "anchors_missing_beats_not_evaluated",
			inputs: []RecognitionResult{
				{Status: StatusAnchorsMissing, MissingAnchors: []string{"X"}},
				{Status: StatusNotEvaluated},
			},
			want: StatusAnchorsMissing,
		},
		{
			name: "all_not_evaluated",
			inputs: []RecognitionResult{
				{Status: StatusNotEvaluated},
				{Status: StatusNotEvaluated},
			},
			want: StatusNotEvaluated,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeRecognitionResults(tc.inputs...)
			if got.Status != tc.want {
				t.Errorf("status = %q, want %q", got.Status, tc.want)
			}
		})
	}
}

func TestMergeRecognitionResults_AnchorsDeduplicated(t *testing.T) {
	a := RecognitionResult{
		MissingAnchors: []string{"Foo", "Bar"},
		MatchedAnchors: []string{"Baz"},
	}
	b := RecognitionResult{
		MissingAnchors: []string{"Bar", "Quux"}, // Bar is duplicate
		MatchedAnchors: []string{"Baz"},         // duplicate
	}
	got := mergeRecognitionResults(a, b)
	wantMissing := []string{"Bar", "Foo", "Quux"}
	wantMatched := []string{"Baz"}
	if !reflect.DeepEqual(got.MissingAnchors, wantMissing) {
		t.Errorf("missing = %v, want %v", got.MissingAnchors, wantMissing)
	}
	if !reflect.DeepEqual(got.MatchedAnchors, wantMatched) {
		t.Errorf("matched = %v, want %v", got.MatchedAnchors, wantMatched)
	}
}
