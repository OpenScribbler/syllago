package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestRustAsyncRuntimeTokio(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	cargo := `[package]
name = "async-app"
version = "0.1.0"

[dependencies]
tokio = { version = "1", features = ["full"] }
serde = "1.0"
`
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargo), 0644)

	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(`
#[tokio::main]
async fn main() {
    println!("hello async");
}
`), 0644)

	det := RustAsyncRuntime{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for tokio runtime")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if !strings.Contains(ts.Body, "tokio") {
		t.Errorf("body should mention tokio, got: %s", ts.Body)
	}
}

func TestRustAsyncRuntimeNone(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// A sync-only Rust project — no async runtime
	cargo := `[package]
name = "sync-app"
version = "0.1.0"

[dependencies]
serde = "1.0"
`
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargo), 0644)

	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(`
fn main() {
    println!("hello sync");
}
`), 0644)

	det := RustAsyncRuntime{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for sync-only project, got %d", len(sections))
	}
}
