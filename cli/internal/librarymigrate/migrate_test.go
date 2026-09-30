package librarymigrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestRun_MissingLibraryIsLeftAbsent(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	var out bytes.Buffer
	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &out); err != nil {
		t.Fatal(err)
	}
	if exists(lib) || out.Len() != 0 {
		t.Errorf("created library or printed %q", out.String())
	}
}

func TestRun_FreshLibraryIsUnchangedAndSilent(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	meta := "id: a\nname: r1\nsource_provider: claude-code\n"
	write(t, filepath.Join(lib, "rules", "claude-code", "r1", ".syllago.yaml"), meta)
	var out bytes.Buffer
	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("printed %q", out.String())
	}
	if got := read(t, filepath.Join(lib, "rules", "claude-code", "r1", ".syllago.yaml")); got != meta {
		t.Errorf("metadata changed: %q", got)
	}
}

func TestRun_MigratesRetiredSlugOnce(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	lib := filepath.Join(tmp, "content")
	storePath := filepath.Join(tmp, "installs.json")

	// Directory item, single-file item, rule-shape metadata, grouped MCP
	// server, universal skill, and loadout, all naming windsurf.
	write(t, filepath.Join(lib, "rules", "windsurf", "r1", "rule.md"), "# r1\n")
	write(t, filepath.Join(lib, "rules", "windsurf", "r1", ".syllago.yaml"), "id: a\nname: r1\nsource_provider: windsurf\n")
	write(t, filepath.Join(lib, "rules", "windsurf", "single.md"), "# single\n")
	write(t, filepath.Join(lib, "rules", "windsurf", ".syllago.single.md.yaml"), "id: b\nsource_provider: windsurf\n")
	write(t, filepath.Join(lib, "rules", "windsurf", "r2", ".syllago.yaml"), "format_version: 1\nid: c\ntype: rule\nsource:\n    provider: windsurf\n    path: /proj/.windsurfrules\n")
	write(t, filepath.Join(lib, "mcp", "windsurf", "srv", "config.json"), "{}\n")
	write(t, filepath.Join(lib, "skills", "sk", ".syllago.yaml"), "id: d\nsource_provider: windsurf\n")
	write(t, filepath.Join(lib, "loadouts", "windsurf", "lo", "loadout.yaml"), "kind: loadout\nprovider: windsurf\nproviders:\n    - windsurf\n    - claude-code\n")
	// A skill named after the slug is an item, not a provider folder.
	write(t, filepath.Join(lib, "skills", "windsurf", "SKILL.md"), "# skill\n")

	// A provider link into the old folder, and an install record for it.
	link := filepath.Join(home, ".claude", "rules", "r1")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(lib, "rules", "windsurf", "r1"), link); err != nil {
		t.Fatal(err)
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	store.Upsert(installstore.Record{Coord: installstore.Coord{Type: "rules", Name: "r1"}, LibraryPath: filepath.Join(lib, "rules", "windsurf", "r1")})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Run(lib, home, storePath, &out); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), "\n"); n != 1 || !strings.HasPrefix(out.String(), "notice: ") {
		t.Errorf("want one notice line, got %q", out.String())
	}
	for _, p := range []string{"rules/windsurf", "mcp/windsurf", "loadouts/windsurf"} {
		if exists(filepath.Join(lib, p)) {
			t.Errorf("%s still exists", p)
		}
	}
	if !exists(filepath.Join(lib, "skills", "windsurf", "SKILL.md")) {
		t.Error("skill named windsurf was moved")
	}
	if !exists(filepath.Join(lib, "mcp", "devin", "srv", "config.json")) {
		t.Error("grouped MCP server not moved")
	}
	for path, want := range map[string]string{
		"rules/devin/r1/.syllago.yaml":        "source_provider: devin",
		"rules/devin/.syllago.single.md.yaml": "source_provider: devin",
		"rules/devin/r2/.syllago.yaml":        "provider: devin",
		"skills/sk/.syllago.yaml":             "source_provider: devin",
		"loadouts/devin/lo/loadout.yaml":      "provider: devin",
	} {
		got := read(t, filepath.Join(lib, path))
		if !strings.Contains(got, want) || strings.Contains(got, ": windsurf") || strings.Contains(got, "- windsurf") {
			t.Errorf("%s = %q, want %q and no windsurf slug", path, got, want)
		}
	}
	if got := read(t, filepath.Join(lib, "rules", "devin", "r2", ".syllago.yaml")); !strings.Contains(got, "/proj/.windsurfrules") {
		t.Errorf("rule source path was rewritten: %q", got)
	}
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(lib, "rules", "devin", "r1") {
		t.Errorf("link target = %q, %v", target, err)
	}
	store, err = installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Records[0].LibraryPath; got != filepath.Join(lib, "rules", "devin", "r1") {
		t.Errorf("record library path = %q", got)
	}

	// A migrated library is left alone and prints nothing.
	out.Reset()
	if err := Run(lib, home, storePath, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("second run printed %q", out.String())
	}
}

func TestRun_BothFoldersWarnAndStayPut(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	oldDir := filepath.Join(lib, "rules", "windsurf")
	newDir := filepath.Join(lib, "rules", "devin")
	write(t, filepath.Join(oldDir, "r1", "rule.md"), "# old\n")
	write(t, filepath.Join(newDir, "r2", "rule.md"), "# new\n")

	var out bytes.Buffer
	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &out); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(oldDir, "r1", "rule.md")) || !exists(filepath.Join(newDir, "r2", "rule.md")) {
		t.Error("a folder was moved")
	}
	if got := out.String(); !strings.HasPrefix(got, "warning: ") || !strings.Contains(got, oldDir) || !strings.Contains(got, newDir) {
		t.Errorf("warning = %q, want both paths", got)
	}
}
