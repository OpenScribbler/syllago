package librarymigrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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
		if target, err := os.Readlink(filepath.Join(lib, p)); err != nil || target != "devin" {
			t.Errorf("%s is not a link to devin: %q, %v", p, target, err)
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

func recordSymlink(t *testing.T, storePath, libPath, linkPath string) {
	t.Helper()
	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	store.Upsert(installstore.Record{
		Coord:       installstore.Coord{Type: "rules", Name: filepath.Base(libPath)},
		LibraryPath: libPath,
		Placements:  []installstore.Placement{{Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: linkPath}},
	})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestRun_RepointsRecordedLinkAtCustomPath(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	storePath := filepath.Join(tmp, "installs.json")
	src := filepath.Join(lib, "rules", "windsurf", "r1")
	write(t, filepath.Join(src, "rule.md"), "# r1\n")
	link := filepath.Join(tmp, "custom", "r1")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	recordSymlink(t, storePath, src, link)

	if err := Run(lib, filepath.Join(tmp, "home"), storePath, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(lib, "rules", "devin", "r1") {
		t.Errorf("custom link target = %q, %v", target, err)
	}
}

func TestRun_FailedRepointRestoresFolderAndLinks(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	storePath := filepath.Join(tmp, "installs.json")
	oldDir := filepath.Join(lib, "rules", "windsurf")
	write(t, filepath.Join(oldDir, "r1", "rule.md"), "# r1\n")
	write(t, filepath.Join(oldDir, "r2", "rule.md"), "# r2\n")
	okLink := filepath.Join(tmp, "ok", "r1")
	lockedLink := filepath.Join(tmp, "locked", "r2")
	for link, src := range map[string]string{okLink: filepath.Join(oldDir, "r1"), lockedLink: filepath.Join(oldDir, "r2")} {
		if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
		recordSymlink(t, storePath, src, link)
	}
	if err := os.Chmod(filepath.Dir(lockedLink), 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(lockedLink), 0755) })

	var out bytes.Buffer
	if err := Run(lib, filepath.Join(tmp, "home"), storePath, &out); err == nil {
		t.Fatal("expected an error when a link cannot be repointed")
	}
	if !exists(filepath.Join(oldDir, "r1", "rule.md")) || exists(filepath.Join(lib, "rules", "devin")) {
		t.Error("folder was not restored")
	}
	for link, want := range map[string]string{okLink: filepath.Join(oldDir, "r1"), lockedLink: filepath.Join(oldDir, "r2")} {
		if target, err := os.Readlink(link); err != nil || target != want {
			t.Errorf("%s target = %q, %v; want %q", link, target, err, want)
		}
	}
	if strings.Contains(out.String(), "notice:") {
		t.Errorf("printed a notice for a failed move: %q", out.String())
	}
}

func TestRun_FlatMCPServerNamedAfterSlugStays(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	write(t, filepath.Join(lib, "mcp", "windsurf", "config.json"), "{}\n")
	write(t, filepath.Join(lib, "mcp", "windsurf", "extra", "config.json"), "{}\n")

	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(lib, "mcp", "windsurf", "config.json")) {
		t.Error("flat MCP server named windsurf was moved")
	}
}

func TestRun_UnreadableRecordedLinkStopsMove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	storePath := filepath.Join(tmp, "installs.json")
	src := filepath.Join(lib, "rules", "windsurf", "r1")
	write(t, filepath.Join(src, "rule.md"), "body\n")
	parent := filepath.Join(tmp, "locked")
	link := filepath.Join(parent, "r1.md")
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	recordSymlink(t, storePath, src, link)
	if err := os.Chmod(parent, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0755) })

	if err := Run(lib, tmp, storePath, &bytes.Buffer{}); err == nil {
		t.Fatal("expected an error for an unreadable recorded link")
	}
	if !exists(src) || exists(filepath.Join(lib, "rules", "devin")) {
		t.Error("folder moved despite an unreadable recorded link")
	}
}

func TestRun_OldPathKeepsResolving(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	storePath := filepath.Join(tmp, "installs.json")
	write(t, filepath.Join(lib, "hooks", "windsurf", "check", "hook.json"), `{"hooks":{"PostToolUse":[{"matcher":"Write","command":"./check.sh"}]}}`)
	write(t, filepath.Join(lib, "hooks", "windsurf", "check", "check.sh"), "#!/bin/sh\n")
	os.MkdirAll(filepath.Join(lib, "skills"), 0755)

	if err := Run(lib, tmp, storePath, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(lib, "hooks", "devin", "check", "hook.json")) {
		t.Fatal("hook folder not moved")
	}
	// A loadout-applied hook command holds the old absolute script path.
	if read(t, filepath.Join(lib, "hooks", "windsurf", "check", "check.sh")) != "#!/bin/sh\n" {
		t.Error("old script path no longer resolves")
	}

	var out bytes.Buffer
	if err := Run(lib, tmp, storePath, &out); err != nil || out.Len() != 0 {
		t.Errorf("second run: err %v, printed %q", err, out.String())
	}
	cat, err := catalog.Scan(lib, lib)
	if err != nil {
		t.Fatal(err)
	}
	if n := cat.CountByType()[catalog.Hooks]; n != 1 {
		t.Errorf("scan found %d hooks, want 1", n)
	}
}

func TestRun_SymlinkedMetadataOutsideLibraryUntouched(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	outside := filepath.Join(tmp, "outside.yaml")
	body := "id: a\nname: r1\nsource_provider: windsurf\n"
	write(t, outside, body)
	write(t, filepath.Join(lib, "rules", "claude-code", "r1", "rule.md"), "body\n")
	if err := os.Symlink(outside, filepath.Join(lib, "rules", "claude-code", "r1", ".syllago.yaml")); err != nil {
		t.Fatal(err)
	}

	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, outside); got != body {
		t.Errorf("file outside the library rewritten to %q", got)
	}
}

func TestRun_UnrecordedProjectLinkKeepsResolving(t *testing.T) {
	tmp := t.TempDir()
	lib := filepath.Join(tmp, "content")
	src := filepath.Join(lib, "rules", "windsurf", "r1")
	write(t, filepath.Join(src, "rule.md"), "body\n")
	// A loadout applied with --base-dir records this link only in the
	// project's install records, which the migration never sees.
	link := filepath.Join(tmp, "project", ".devin", "rules", "r1.md")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "rule.md"), link); err != nil {
		t.Fatal(err)
	}

	if err := Run(lib, tmp, filepath.Join(tmp, "installs.json"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(lib, "rules", "devin", "r1", "rule.md")) {
		t.Fatal("rule folder not moved")
	}
	if got := read(t, link); got != "body\n" {
		t.Errorf("project link reads %q", got)
	}
}
