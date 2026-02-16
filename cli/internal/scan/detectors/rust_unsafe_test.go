package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestRustUnsafeForbid(t *testing.T) {
	tmp := t.TempDir()

	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(`[package]
name = "safe-app"
version = "0.1.0"
`), 0644)

	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte(`#![forbid(unsafe_code)]

pub fn safe_function() -> i32 {
    42
}
`), 0644)

	det := RustUnsafe{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for forbid(unsafe_code)")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if !strings.Contains(ts.Body, "forbid") {
		t.Errorf("body should mention forbid, got: %s", ts.Body)
	}
}

func TestRustUnsafeUsage(t *testing.T) {
	tmp := t.TempDir()

	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(`[package]
name = "unsafe-app"
version = "0.1.0"
`), 0644)

	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte(`
pub fn do_something() {
    unsafe {
        // raw pointer dereference
        let ptr = 0x1234 as *const i32;
        let _ = *ptr;
    }
}

pub fn another() {
    unsafe {
        std::hint::unreachable_unchecked();
    }
}
`), 0644)

	det := RustUnsafe{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for unsafe usage")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if !strings.Contains(ts.Body, "unsafe block") {
		t.Errorf("body should mention unsafe blocks, got: %s", ts.Body)
	}
}
