package detectors

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestDependenciesNode(t *testing.T) {
	det := Dependencies{}
	sections, err := det.Detect("testdata/techstack/node-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected dependency section for Node project")
	}
	ds, ok := sections[0].(model.DependencySection)
	if !ok {
		t.Fatalf("expected DependencySection, got %T", sections[0])
	}
	if len(ds.Groups) < 2 {
		t.Errorf("expected at least 2 groups (prod + dev), got %d", len(ds.Groups))
	}
}

func TestDependenciesGo(t *testing.T) {
	det := Dependencies{}
	sections, err := det.Detect("testdata/techstack/go-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected dependency section for Go project")
	}
	ds, ok := sections[0].(model.DependencySection)
	if !ok {
		t.Fatalf("expected DependencySection, got %T", sections[0])
	}
	if len(ds.Groups[0].Items) != 2 {
		t.Errorf("expected 2 Go deps, got %d", len(ds.Groups[0].Items))
	}
}
