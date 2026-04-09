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
