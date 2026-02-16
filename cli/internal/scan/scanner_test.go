package scan

import (
	"fmt"
	"testing"
	"time"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

type stubDetector struct {
	name     string
	sections []model.Section
	err      error
	delay    time.Duration
	panics   bool
}

func (d stubDetector) Name() string { return d.name }
func (d stubDetector) Detect(root string) ([]model.Section, error) {
	if d.panics {
		panic("intentional panic for testing")
	}
	if d.delay > 0 {
		time.Sleep(d.delay)
	}
	return d.sections, d.err
}

func TestScannerCollectsResults(t *testing.T) {
	scanner := NewScanner(
		stubDetector{
			name: "tech-stack",
			sections: []model.Section{
				model.TechStackSection{Origin: model.OriginAuto, Title: "Tech Stack", Language: "Go"},
			},
		},
		stubDetector{
			name: "build",
			sections: []model.Section{
				model.BuildCommandSection{Origin: model.OriginAuto, Title: "Build Commands"},
			},
		},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Document.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(result.Document.Sections))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(result.Warnings))
	}
}

func TestScannerHandlesTimeout(t *testing.T) {
	scanner := NewScanner(
		stubDetector{
			name:  "slow",
			delay: 2 * time.Second,
			sections: []model.Section{
				model.TextSection{Category: model.CatConventions, Title: "slow result"},
			},
		},
	)
	scanner.Timeout = 100 * time.Millisecond

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 timeout warning, got %d", len(result.Warnings))
	}
}

func TestScannerHandlesPanic(t *testing.T) {
	scanner := NewScanner(
		stubDetector{name: "panicker", panics: true},
		stubDetector{
			name: "normal",
			sections: []model.Section{
				model.TextSection{Category: model.CatConventions, Title: "normal"},
			},
		},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 panic warning, got %d", len(result.Warnings))
	}
	if len(result.Document.Sections) != 1 {
		t.Errorf("expected 1 section from normal detector, got %d", len(result.Document.Sections))
	}
}

func TestScannerHandlesError(t *testing.T) {
	scanner := NewScanner(
		stubDetector{name: "erroring", err: fmt.Errorf("test error")},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 error warning, got %d", len(result.Warnings))
	}
}
