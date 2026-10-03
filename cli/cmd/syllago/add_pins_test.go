package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// seedPinnedProviderRule puts a rule added from claude-code in the Library
// and pins its install record.
func seedPinnedProviderRule(t *testing.T, configDir, globalDir, name string) string {
	t.Helper()
	libraryPath := add.DestDir(catalog.Rules, "claude-code", name, globalDir)
	if err := os.MkdirAll(libraryPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libraryPath, "rule.md"), []byte("# held\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(libraryPath, &metadata.Meta{Name: name, SourceType: "provider", SourceProvider: "claude-code"}); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(configDir, "installs.json")
	coord := installstore.Coord{Type: string(catalog.Rules), Name: name}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: filepath.Join(t.TempDir(), name),
	}, installstore.InstallMeta{}, now); err != nil {
		t.Fatal(err)
	}
	if err := installstore.SetPinned(storePath, coord, true, now); err != nil {
		t.Fatal(err)
	}
	return libraryPath
}

func sourceRule(t *testing.T, name string) add.DiscoveryItem {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".md")
	if err := os.WriteFile(path, []byte("# new "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return add.DiscoveryItem{Name: name, Type: catalog.Rules, Path: path, Status: add.StatusOutdated}
}

// Regression: an add from a provider wrote over pinned Library items.
func TestAddRespectingPins_SkipsPinnedAddsTheRest(t *testing.T) {
	configDir := withInstallRecordConfigDir(t)
	globalDir := t.TempDir()
	_, stderr := output.SetForTest(t)
	held := seedPinnedProviderRule(t, configDir, globalDir, "held")

	results, err := addRespectingPins(
		[]add.DiscoveryItem{sourceRule(t, "held"), sourceRule(t, "fresh")},
		add.AddOptions{Provider: "claude-code", Force: true}, globalDir, nil, "test")
	if err != nil {
		t.Fatalf("addRespectingPins: %v", err)
	}
	if len(results) != 1 || results[0].Name != "fresh" {
		t.Errorf("results = %+v, want only fresh", results)
	}
	if got, _ := os.ReadFile(filepath.Join(held, "rule.md")); string(got) != "# held\n" {
		t.Errorf("pinned rule = %q, want it unchanged", got)
	}
	if !strings.Contains(stderr.String(), "pinned rules/held; unpin to update: syllago unpin held") {
		t.Errorf("stderr = %q, want the pinned item reported", stderr.String())
	}
}

func TestAddRespectingPins_AllPinnedIsAConflict(t *testing.T) {
	configDir := withInstallRecordConfigDir(t)
	globalDir := t.TempDir()
	output.SetForTest(t)
	seedPinnedProviderRule(t, configDir, globalDir, "held")

	_, err := addRespectingPins([]add.DiscoveryItem{sourceRule(t, "held")},
		add.AddOptions{Provider: "claude-code", Force: true}, globalDir, nil, "test")
	var se output.StructuredError
	if !errors.As(err, &se) || se.Code != output.ErrInstallConflict {
		t.Fatalf("err = %v, want %s", err, output.ErrInstallConflict)
	}
}
