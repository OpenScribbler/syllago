package reconcile

import (
	"strings"
	"testing"
)

func TestParseHTMLMarkers(t *testing.T) {
	content := `# Project Context

<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->

<!-- nesco:human:architecture -->
## Architecture
Our custom architecture notes.
<!-- /nesco:human:architecture -->
`

	result := Parse(content, FormatHTML)
	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.Sections))
	}
	if !result.Sections[0].IsAuto {
		t.Error("first section should be auto")
	}
	if !result.Sections[1].IsHuman {
		t.Error("second section should be human")
	}
	if !strings.Contains(result.Sections[1].Content, "custom architecture") {
		t.Error("human section content not preserved")
	}
}

func TestReconcileReplacesAuto(t *testing.T) {
	existing := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.21
<!-- /nesco:auto:tech-stack -->

<!-- nesco:human:architecture -->
## Architecture
Keep this.
<!-- /nesco:human:architecture -->
`

	fresh := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->
`

	result := Reconcile(existing, fresh, FormatHTML)
	if !strings.Contains(result.Output, "Go 1.22") {
		t.Error("auto section not updated")
	}
	if strings.Contains(result.Output, "Go 1.21") {
		t.Error("old auto content should be replaced")
	}
	if !strings.Contains(result.Output, "Keep this.") {
		t.Error("human section not preserved")
	}
}

func TestReconcileAppendsNewSections(t *testing.T) {
	existing := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->
`

	fresh := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->

<!-- nesco:auto:surprise -->
## Competing Frameworks
Both Jest and Vitest found.
<!-- /nesco:auto:surprise -->
`

	result := Reconcile(existing, fresh, FormatHTML)
	if !strings.Contains(result.Output, "Competing Frameworks") {
		t.Error("new section should be appended")
	}
}

func TestReconcileEmptyExisting(t *testing.T) {
	result := Reconcile("", "# Fresh content\n", FormatHTML)
	if result.Output != "# Fresh content\n" {
		t.Errorf("empty existing should pass through fresh: %q", result.Output)
	}
}
