package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestPathAliasGapDetected(t *testing.T) {
	tmp := t.TempDir()

	// tsconfig.json with path aliases defined
	tsconfig := `{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  }
}`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	// Create .ts files that mostly use relative imports
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)

	// 1 alias import, 9 relative imports → 10% alias usage, well below 30%
	os.WriteFile(filepath.Join(srcDir, "a.ts"), []byte(`import { foo } from "@/utils"
import { bar } from "./bar"
import { baz } from "../baz"
import { qux } from "./qux"
import { quux } from "./quux"
`), 0644)

	os.WriteFile(filepath.Join(srcDir, "b.ts"), []byte(`import { x } from "./x"
import { y } from "../y"
import { z } from "./z"
import { w } from "../w"
import { v } from "./v"
`), 0644)

	det := PathAliasGap{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a surprise section for low alias usage")
	}
	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestPathAliasGapAdequateUsage(t *testing.T) {
	tmp := t.TempDir()

	tsconfig := `{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  }
}`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(srcDir, 0755)

	// 4 alias imports, 1 relative → 80% alias usage, well above 30%
	os.WriteFile(filepath.Join(srcDir, "a.ts"), []byte(`import { foo } from "@/utils"
import { bar } from "@/bar"
import { baz } from "@/baz"
import { qux } from "@/qux"
import { quux } from "./local"
`), 0644)

	det := PathAliasGap{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for adequate alias usage, got %d", len(sections))
	}
}
