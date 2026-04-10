package extract_go_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_go"
)

func TestGoExtractor_StringConsts(t *testing.T) {
	raw := []byte(`package hooks

const (
	PreToolUse  = "PreToolUse"
	PostToolUse = "PostToolUse"
	unexported  = "hidden"
)
`)
	cfg := capmon.SelectorConfig{MinResults: 1}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	// Exported consts should appear with their string values
	found := false
	for _, fv := range result.Fields {
		if fv.Value == "PreToolUse" {
			found = true
		}
	}
	if !found {
		t.Error("expected PreToolUse string value in extracted fields")
	}
	// Unexported const should not appear
	for _, fv := range result.Fields {
		if fv.Value == "hidden" {
			t.Error("unexported const 'hidden' should not appear in fields")
		}
	}
}

func TestGoExtractor_IotaEnum(t *testing.T) {
	raw := []byte(`package capmon

type ExitClass int

const (
	ExitClean ExitClass = iota
	ExitChanged
	ExitPartial
)
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	// Iota consts should be extracted with their identifier names
	wantFields := []string{"ExitClean", "ExitChanged", "ExitPartial"}
	for _, want := range wantFields {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected iota const %q in fields", want)
		}
	}
}

func TestGoExtractor_TypeLandmarks(t *testing.T) {
	raw := []byte(`package capmon

type ExitClass int
type SelectorConfig struct{}
type unexportedType struct{}
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	wantLandmarks := []string{"ExitClass", "SelectorConfig"}
	for _, want := range wantLandmarks {
		found := false
		for _, got := range result.Landmarks {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("landmark %q not found in %v", want, result.Landmarks)
		}
	}
	// unexportedType should not appear as a landmark
	for _, got := range result.Landmarks {
		if got == "unexportedType" {
			t.Error("unexported type should not appear as landmark")
		}
	}
}

func TestGoExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`package hooks

const PreToolUse = "PreToolUse"
`)
	cfg := capmon.SelectorConfig{
		ExpectedContains: "NonExistentAnchor",
	}
	_, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}

func TestGoExtractor_Generics(t *testing.T) {
	raw := []byte(`package mylib

type Set[T comparable] struct {
	m map[T]struct{}
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]struct{})}
}

const Version = "1.0.0"
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("modern Go with generics should parse without error: %v", err)
	}
	// Set should be a landmark
	found := false
	for _, got := range result.Landmarks {
		if got == "Set" {
			found = true
		}
	}
	if !found {
		t.Errorf("generic type Set not found in landmarks %v", result.Landmarks)
	}
	// Version const should be extracted
	versionFound := false
	for _, fv := range result.Fields {
		if fv.Value == "1.0.0" {
			versionFound = true
		}
	}
	if !versionFound {
		t.Error("expected Version = \"1.0.0\" in fields")
	}
}

func TestGoExtractor_ParseError(t *testing.T) {
	raw := []byte(`this is not valid go source code!!!`)
	cfg := capmon.SelectorConfig{}
	_, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err == nil {
		t.Error("expected parse error for invalid Go source")
	}
}

func TestGoExtractor_IntConsts(t *testing.T) {
	raw := []byte(`package skills

const (
	MaxNameLength    = 64
	MaxDescLength    = 512
	unexportedLimit  = 8
)
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	// Exported int consts should have the literal value, not the name
	cases := []struct{ key, wantVal string }{
		{"MaxNameLength", "64"},
		{"MaxDescLength", "512"},
	}
	for _, tc := range cases {
		fv, ok := result.Fields[tc.key]
		if !ok {
			t.Errorf("field %q not found in %v", tc.key, result.Fields)
			continue
		}
		if fv.Value != tc.wantVal {
			t.Errorf("field %q: got value %q, want %q", tc.key, fv.Value, tc.wantVal)
		}
	}
	// unexported const should not appear
	if _, ok := result.Fields["unexportedLimit"]; ok {
		t.Error("unexported int const should not appear in fields")
	}
}

func TestGoExtractor_StructFields(t *testing.T) {
	raw := []byte(`package skills

type Skill struct {
	Name          string            ` + "`" + `yaml:"name"` + "`" + `
	Description   string            ` + "`" + `yaml:"description"` + "`" + `
	License       string            ` + "`" + `yaml:"license,omitempty"` + "`" + `
	Compatibility []string          ` + "`" + `yaml:"compatibility,omitempty"` + "`" + `
	Metadata      map[string]string ` + "`" + `yaml:"metadata,omitempty"` + "`" + `
	internal      string
}
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	// Struct fields should be extracted with yaml tag names as values
	wantFields := map[string]string{
		"Skill.Name":          "name",
		"Skill.Description":   "description",
		"Skill.License":       "license",
		"Skill.Compatibility": "compatibility",
		"Skill.Metadata":      "metadata",
	}
	for key, wantVal := range wantFields {
		fv, ok := result.Fields[key]
		if !ok {
			t.Errorf("field %q not found in extracted fields", key)
			continue
		}
		if fv.Value != wantVal {
			t.Errorf("field %q: got value %q, want %q", key, fv.Value, wantVal)
		}
	}
	// Unexported field should not appear
	if _, ok := result.Fields["Skill.internal"]; ok {
		t.Error("unexported struct field should not appear")
	}
	// Skill should be a landmark
	foundLandmark := false
	for _, l := range result.Landmarks {
		if l == "Skill" {
			foundLandmark = true
		}
	}
	if !foundLandmark {
		t.Errorf("Skill not found in landmarks %v", result.Landmarks)
	}
}

func TestGoExtractor_StructFieldYamlDash(t *testing.T) {
	raw := []byte(`package config

type Config struct {
	Name     string ` + "`" + `yaml:"name"` + "`" + `
	Internal string ` + "`" + `yaml:"-"` + "`" + `
	NoTag    string
}
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("Extract go: %v", err)
	}
	// yaml:"-" field should be skipped
	if _, ok := result.Fields["Config.Internal"]; ok {
		t.Error("yaml:\"-\" field should be excluded")
	}
	// Name should be extracted with its yaml tag
	fv, ok := result.Fields["Config.Name"]
	if !ok {
		t.Error("Config.Name should be extracted")
	} else if fv.Value != "name" {
		t.Errorf("Config.Name: got %q, want %q", fv.Value, "name")
	}
	// NoTag field should be extracted with lowercased field name as fallback
	fv, ok = result.Fields["Config.NoTag"]
	if !ok {
		t.Error("Config.NoTag should be extracted (no yaml tag → lowercase field name)")
	} else if fv.Value != "notag" {
		t.Errorf("Config.NoTag: got %q, want %q", fv.Value, "notag")
	}
}

func TestGoExtractor_Partial(t *testing.T) {
	raw := []byte(`package p

const OnlyOne = "one"
`)
	cfg := capmon.SelectorConfig{MinResults: 5}
	result, err := capmon.Extract(context.Background(), "go", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be Partial when below MinResults")
	}
}
