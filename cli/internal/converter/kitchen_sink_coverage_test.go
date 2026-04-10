package converter

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// assertAllFieldsPopulated uses reflection to verify that every exported
// field of v is non-zero. Fields listed in skipFields are excluded —
// use this for bool fields where false is a valid semantic value
// (e.g., AlwaysApply=false means glob-scoped).
func assertAllFieldsPopulated(t *testing.T, v any, skipFields ...string) {
	t.Helper()
	skip := map[string]bool{}
	for _, f := range skipFields {
		skip[f] = true
	}
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() || skip[field.Name] {
			continue
		}
		if rv.Field(i).IsZero() {
			t.Errorf("field %s is zero — add it to the kitchen-sink example", field.Name)
		}
	}
}

func TestKitchenSinkSkillCoverage(t *testing.T) {
	path := filepath.Join("testdata", "kitchen-sink-skill.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading kitchen-sink skill: %v", err)
	}

	meta, body, err := parseSkillCanonical(data)
	if err != nil {
		t.Fatalf("parsing kitchen-sink skill: %v", err)
	}
	if body == "" {
		t.Fatal("kitchen-sink skill body is empty")
	}

	// DisableModelInvocation: false is a valid default (means model can invoke).
	// The kitchen-sink sets it to true, so it's non-zero and doesn't need skipping,
	// but we skip it defensively since the task calls it out.
	assertAllFieldsPopulated(t, meta, "DisableModelInvocation")
}

func TestKitchenSinkAgentCoverage(t *testing.T) {
	path := filepath.Join("testdata", "kitchen-sink-agent.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading kitchen-sink agent: %v", err)
	}

	meta, body, err := parseAgentCanonical(data)
	if err != nil {
		t.Fatalf("parsing kitchen-sink agent: %v", err)
	}
	if body == "" {
		t.Fatal("kitchen-sink agent body is empty")
	}

	// Background: false means non-background, which is the normal default.
	assertAllFieldsPopulated(t, meta, "Background")
}

func TestKitchenSinkRuleCoverage(t *testing.T) {
	path := filepath.Join("testdata", "kitchen-sink-rule.mdc")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading kitchen-sink rule: %v", err)
	}

	meta, body, err := parseCanonical(data)
	if err != nil {
		t.Fatalf("parsing kitchen-sink rule: %v", err)
	}
	if body == "" {
		t.Fatal("kitchen-sink rule body is empty")
	}

	// AlwaysApply: false means glob-scoped, which is correct for the Cursor variant.
	assertAllFieldsPopulated(t, meta, "AlwaysApply")
}

func TestKitchenSinkCommandCoverage(t *testing.T) {
	path := filepath.Join("testdata", "kitchen-sink-command.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading kitchen-sink command: %v", err)
	}

	meta, body, err := parseCommandCanonical(data)
	if err != nil {
		t.Fatalf("parsing kitchen-sink command: %v", err)
	}
	if body == "" {
		t.Fatal("kitchen-sink command body is empty")
	}

	// DisableModelInvocation: false means the model can invoke — valid default.
	assertAllFieldsPopulated(t, meta, "DisableModelInvocation")
}
