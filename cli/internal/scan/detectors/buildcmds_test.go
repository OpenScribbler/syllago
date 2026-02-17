package detectors

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestBuildCommandsMakefile(t *testing.T) {
	t.Parallel()
	det := BuildCommands{}
	sections, err := det.Detect("testdata/buildcmds/makefile-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected build commands section")
	}
	bc, ok := sections[0].(model.BuildCommandSection)
	if !ok {
		t.Fatalf("expected BuildCommandSection, got %T", sections[0])
	}
	if len(bc.Commands) < 3 {
		t.Errorf("expected at least 3 Makefile targets, got %d", len(bc.Commands))
	}
}

func TestBuildCommandsEmpty(t *testing.T) {
	t.Parallel()
	det := BuildCommands{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
