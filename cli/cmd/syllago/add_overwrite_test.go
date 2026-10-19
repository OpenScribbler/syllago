package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

type addOverwriteFixture struct {
	configDir string
	dests     []string
	ct        catalog.ContentType
	filename  string
	run       func(force, dryRun bool) error
}

func newAddOverwriteFixture(t *testing.T, writer string) addOverwriteFixture {
	t.Helper()
	f := addOverwriteFixture{configDir: withInstallRecordConfigDir(t)}
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	names := []string{"held", "replace", "fresh"}
	switch writer {
	case "hooks", "mcp":
		settings := `{"mcpServers":{"held":{"command":"new-held"},"replace":{"command":"new-replace"},"fresh":{"command":"new-fresh"}}}`
		f.ct, f.filename = catalog.MCP, "config.json"
		if writer == "hooks" {
			settings = `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"echo new-held","statusMessage":"held"}]},{"hooks":[{"type":"command","command":"echo new-replace","statusMessage":"replace"}]},{"hooks":[{"type":"command","command":"echo new-fresh","statusMessage":"fresh"}]}]}}`
			f.ct, f.filename = catalog.Hooks, "hook.json"
		}
		projectRoot, library := settingsAddEnv(t, settings)
		globalDir = library
		f.run = func(force, dryRun bool) error {
			return runAddSettings(f.ct, projectRoot, "claude-code", dryRun, nil, force, "project", nil, "incoming/registry", "private", "Display Name")
		}
	case "monolithic":
		f.ct, f.filename = catalog.Rules, "rule.md"
		names = []string{"security-rules", "testing-rules", "logging-rules", "style-rules", "process-rules"}
		path := setupMonolithicFixture(t, "CLAUDE.md", monolithicH2Source)
		cmd := &cobra.Command{}
		cmd.Flags().String("split", "h2", "")
		f.run = func(bool, bool) error { return runAddFromMonolithicFiles(cmd, "", []string{path}) }
	case "shared":
		f.ct, f.filename = catalog.Skills, "SKILL.md"
		projectRoot := t.TempDir()
		for _, name := range names {
			writeSharedSkill(t, projectRoot, name)
		}
		f.run = func(force, dryRun bool) error { return runAddFromShared(projectRoot, nil, true, dryRun, force) }
	default:
		t.Fatalf("unknown writer %s", writer)
	}
	for _, name := range names {
		parent := filepath.Join(globalDir, string(f.ct))
		if writer != "shared" {
			parent = filepath.Join(parent, "claude-code")
		}
		f.dests = append(f.dests, filepath.Join(parent, name))
	}
	return f
}

func seedAddOverwriteRecord(t *testing.T, f addOverwriteFixture, dest string, pinned bool) installstore.Record {
	t.Helper()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, f.filename), []byte("old content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(dest)
	if err := metadata.Save(dest, &metadata.Meta{Name: name, SourceType: "registry", SourceRegistry: "old/registry", SourceSHA: "sha-old"}); err != nil {
		t.Fatal(err)
	}
	coord := installstore.Coord{Registry: "old/registry", Type: string(f.ct), Name: name}
	storePath := filepath.Join(f.configDir, "installs.json")
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if err := installstore.RecordInstallMeta(storePath, coord, dest, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismCopy, Path: filepath.Join(t.TempDir(), name),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, now); err != nil {
		t.Fatal(err)
	}
	if pinned {
		if err := installstore.SetPinned(storePath, coord, true, now); err != nil {
			t.Fatal(err)
		}
	}
	return *mustLoadInstallRecordStore(t, f.configDir).Find(coord)
}

func snapshotAddFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			rel, _ := filepath.Rel(dir, path)
			files[rel] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestAddWritersOverwriteSkipsPinsAndRotatesRecords(t *testing.T) {
	for _, writer := range []string{"hooks", "mcp", "monolithic", "shared"} {
		t.Run(writer, func(t *testing.T) {
			f := newAddOverwriteFixture(t, writer)
			stdout, stderr := output.SetForTest(t)
			held := seedAddOverwriteRecord(t, f, f.dests[0], true)
			old := seedAddOverwriteRecord(t, f, f.dests[1], false)
			before := snapshotAddFiles(t, f.dests[0])

			if err := f.run(true, false); err != nil {
				t.Fatalf("add: %v", err)
			}
			if after := snapshotAddFiles(t, f.dests[0]); !reflect.DeepEqual(after, before) {
				t.Errorf("pinned files changed: before=%v after=%v", before, after)
			}
			store := mustLoadInstallRecordStore(t, f.configDir)
			if got := store.Find(held.Coord); !reflect.DeepEqual(got, &held) {
				t.Errorf("pinned record changed: %+v", got)
			}
			rec := store.Find(old.Coord)
			if rec == nil || rec.Previous == nil || rec.Previous.ContentHash != old.ContentHash || rec.Previous.SourceSHA != "sha-old" {
				t.Fatalf("replaced record = %+v, want old version in Previous", rec)
			}
			hash, err := installstore.HashContent(f.dests[1])
			if err != nil || rec.ContentHash != hash || rec.ContentHash == old.ContentHash {
				t.Errorf("new record hash = %s, content hash = %s (err=%v)", rec.ContentHash, hash, err)
			}
			for _, dest := range f.dests[2:] {
				if _, err := os.Stat(filepath.Join(dest, f.filename)); err != nil {
					t.Errorf("new item was not written: %v", err)
				}
			}
			if !strings.Contains(stderr.String(), "pinned "+string(f.ct)+"/"+held.Coord.Name) || !strings.Contains(stderr.String(), "syllago unpin "+held.Coord.Name) {
				t.Errorf("stderr = %q, want the pinned item and unpin hint", stderr.String())
			}
			if !strings.Contains(stdout.String(), "Added ") {
				t.Errorf("stdout = %q, want add summary", stdout.String())
			}
		})
	}
}

func TestAddWritersOverwriteAllPinnedIsConflict(t *testing.T) {
	for _, writer := range []string{"hooks", "mcp", "monolithic", "shared"} {
		t.Run(writer, func(t *testing.T) {
			f := newAddOverwriteFixture(t, writer)
			_, stderr := output.SetForTest(t)
			before := make([]map[string]string, len(f.dests))
			for i, dest := range f.dests {
				seedAddOverwriteRecord(t, f, dest, true)
				before[i] = snapshotAddFiles(t, dest)
			}
			storeBefore := snapshotAddFiles(t, f.configDir)["installs.json"]
			err := f.run(true, false)
			var se output.StructuredError
			if !errors.As(err, &se) || se.Code != output.ErrInstallConflict {
				t.Errorf("err = %v, want %s", err, output.ErrInstallConflict)
			}
			for i, dest := range f.dests {
				if after := snapshotAddFiles(t, dest); !reflect.DeepEqual(after, before[i]) {
					t.Errorf("pinned %s files changed", dest)
				}
				if !strings.Contains(stderr.String(), "syllago unpin "+filepath.Base(dest)) {
					t.Errorf("stderr = %q, want unpin hint for %s", stderr.String(), dest)
				}
			}
			if after := snapshotAddFiles(t, f.configDir)["installs.json"]; after != storeBefore {
				t.Error("install records changed")
			}
		})
	}
}

func TestAddWritersOverwriteHoldsInstallLock(t *testing.T) {
	for _, writer := range []string{"hooks", "mcp", "monolithic", "shared"} {
		t.Run(writer, func(t *testing.T) {
			f := newAddOverwriteFixture(t, writer)
			output.SetForTest(t)
			release, err := syllagolock.Acquire(time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			original := syllagolock.DefaultTimeout
			syllagolock.DefaultTimeout = time.Millisecond
			defer func() { syllagolock.DefaultTimeout = original }()

			err = f.run(true, false)
			var se output.StructuredError
			if !errors.As(err, &se) || se.Code != output.ErrSystemLocked {
				t.Errorf("err = %v, want %s", err, output.ErrSystemLocked)
			}
			for _, dest := range f.dests {
				if _, err := os.Stat(dest); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("%s was written while the install lock was held: %v", dest, err)
				}
			}
		})
	}
}

func TestAddWritersOverwritePreviewAndNoForceLeavePins(t *testing.T) {
	for _, writer := range []string{"hooks", "mcp", "shared"} {
		for _, dryRun := range []bool{false, true} {
			t.Run(writer+map[bool]string{false: "/no-force", true: "/dry-run"}[dryRun], func(t *testing.T) {
				f := newAddOverwriteFixture(t, writer)
				_, stderr := output.SetForTest(t)
				for _, dest := range f.dests {
					seedAddOverwriteRecord(t, f, dest, true)
				}
				before := snapshotAddFiles(t, filepath.Dir(f.dests[0]))
				storeBefore := snapshotAddFiles(t, f.configDir)["installs.json"]
				if err := f.run(dryRun, dryRun); err != nil {
					t.Fatalf("add: %v", err)
				}
				if after := snapshotAddFiles(t, filepath.Dir(f.dests[0])); !reflect.DeepEqual(after, before) {
					t.Error("Library files changed")
				}
				if after := snapshotAddFiles(t, f.configDir)["installs.json"]; after != storeBefore {
					t.Error("install records changed")
				}
				if strings.Contains(stderr.String(), "pinned ") {
					t.Errorf("stderr = %q, want no overwrite decision", stderr.String())
				}
			})
		}
	}
}

func TestAddMonolithicOverwriteSameDestinationRotatesOnce(t *testing.T) {
	configDir := withInstallRecordConfigDir(t)
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	output.SetForTest(t)
	dest := filepath.Join(globalDir, "rules", "claude-code", "claude")
	f := addOverwriteFixture{configDir: configDir, ct: catalog.Rules, filename: "rule.md"}
	old := seedAddOverwriteRecord(t, f, dest, false)
	first := setupMonolithicFixture(t, "CLAUDE.md", "# First version\n")
	last := setupMonolithicFixture(t, "CLAUDE.md", "# Last version\n")
	cmd := &cobra.Command{}
	cmd.Flags().String("split", "single", "")

	if err := runAddFromMonolithicFiles(cmd, "", []string{first, last}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "rule.md")); string(got) != "# Last version\n" {
		t.Errorf("rule.md = %q, want the last source's content", got)
	}
	rec := mustLoadInstallRecordStore(t, configDir).Find(old.Coord)
	if rec == nil || rec.Previous == nil || rec.Previous.SourceSHA != old.SourceSHA || rec.Previous.ContentHash != old.ContentHash {
		t.Errorf("record = %+v, want the original version in Previous", rec)
	}
}
