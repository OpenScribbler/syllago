package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestRustFeaturesDetectsNonDefault(t *testing.T) {
	tmp := t.TempDir()

	cargo := `[package]
name = "myapp"
version = "0.1.0"

[features]
default = ["logging"]
logging = []
grpc = ["dep:tonic"]
metrics = ["dep:prometheus"]
`
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargo), 0644)

	// Create a .rs file that uses the grpc feature
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	rsContent := `
#[cfg(feature = "grpc")]
mod grpc_server {
    pub fn start() {}
}

fn main() {}
`
	os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(rsContent), 0644)

	det := RustFeatures{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for non-default features")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Title != "Non-Default Cargo Features" {
		t.Errorf("title = %q, want %q", ts.Title, "Non-Default Cargo Features")
	}
}

func TestRustFeaturesSkipsNonRust(t *testing.T) {
	tmp := t.TempDir()

	det := RustFeatures{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for non-Rust project, got %d", len(sections))
	}
}
