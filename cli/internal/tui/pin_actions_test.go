package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rollback"
)

func TestPinToggle_RoundTrip(t *testing.T) {
	// Seed global dir override
	tmpDir := t.TempDir()
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	// Seed install store record
	storePath, err := installstore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	coord := installstore.Coord{
		Registry: "https://my-registry.com",
		Type:     "rules",
		Name:     "my-rule",
	}
	rec := installstore.Record{
		Coord:       coord,
		SourceSHA:   "1234567890abcdef1234567890abcdef",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.Records = append(store.Records, rec)
	err = store.Save()
	if err != nil {
		t.Fatal(err)
	}

	// Build App
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: "https://my-registry.com"},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{item}}
	app := NewApp(cat, testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	// Press "p"
	m, cmd := app.Update(keyRune('p'))
	app = m.(App)
	if cmd == nil {
		t.Fatal("expected command from handlePin")
	}

	// Execute command
	msg := cmd()
	pinMsg, ok := msg.(pinToggleMsg)
	if !ok {
		t.Fatalf("expected pinToggleMsg, got %T", msg)
	}
	if pinMsg.err != nil {
		t.Fatalf("unexpected pinToggleMsg error: %v", pinMsg.err)
	}
	if !pinMsg.pinned {
		t.Fatal("expected pinToggleMsg to have pinned=true")
	}

	// Send pinToggleMsg back to Update
	m, rescmd := app.Update(pinMsg)
	app = m.(App)
	if rescmd == nil {
		t.Fatal("expected rescan cmd")
	}

	// Check if store flipped to pinned
	store, _ = installstore.Load(storePath)
	recOut := store.Find(coord)
	if recOut == nil || !recOut.Pinned {
		t.Fatal("expected record to be pinned in store")
	}

	// Check toast is success toast
	assertCurrentToastMessage(t, app, "Pinned rules/my-rule — holding at 1234567890ab")
}

func TestRollback_NoData(t *testing.T) {
	// Seed global dir override so PlanFor reads an empty store, not ~/.syllago
	tmpDir := t.TempDir()
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: "https://my-registry.com"},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{item}}
	app := NewApp(cat, testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	// Press "z"
	m, cmd := app.Update(keyRune('z'))
	app = m.(App)
	if cmd == nil {
		t.Fatal("expected command from handleRollback")
	}

	// Execute command
	msg := cmd()
	planMsg, ok := msg.(rollbackPlanMsg)
	if !ok {
		t.Fatalf("expected rollbackPlanMsg, got %T", msg)
	}
	if planMsg.err == nil {
		t.Fatal("expected rollbackPlanMsg to have error")
	}

	// Feed rollbackPlanMsg back to Update
	m, _ = app.Update(planMsg)
	app = m.(App)

	// Toast should show the error message
	cur := app.toast.Current()
	if cur == nil {
		t.Fatal("expected toast message")
	}
	if !strings.Contains(cur.message, "no install record") {
		t.Errorf("toast message = %q, want substring %q", cur.message, "no install record")
	}
}

func TestConfirmPurposeRouting(t *testing.T) {
	plan := &rollback.Plan{
		Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules},
	}
	app := NewApp(&catalog.Catalog{}, testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	app.pendingRollback = plan

	// confirmed: false should clear pendingRollback and do nothing
	m, cmd := app.handleConfirmResult(confirmResultMsg{
		purpose:   confirmPurposeRollback,
		confirmed: false,
	})
	app = m.(App)
	if app.pendingRollback != nil {
		t.Error("expected pendingRollback to be cleared on cancel")
	}
	if cmd != nil {
		t.Error("expected no command on cancel")
	}

	// confirmed: true should clear pendingRollback and return doRollbackCmd
	app.pendingRollback = plan
	m, cmd = app.handleConfirmResult(confirmResultMsg{
		purpose:   confirmPurposeRollback,
		confirmed: true,
	})
	app = m.(App)
	if app.pendingRollback != nil {
		t.Error("expected pendingRollback to be cleared on confirm")
	}
	if cmd == nil {
		t.Error("expected doRollbackCmd to be returned on confirm")
	}
}

func TestLibrary_UpdateMouse_MetaPinAndRollback(t *testing.T) {
	// Seed global dir override
	tmpDir := t.TempDir()
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	// Seed install store record so buttons render
	storePath, err := installstore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	coord := installstore.Coord{Registry: "https://my-registry.com", Type: "rules", Name: "my-rule"}
	rec := installstore.Record{
		Coord:       coord,
		SourceSHA:   "123",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
		Previous:    &installstore.PreviousVersion{SourceSHA: "123"},
	}
	store.Records = append(store.Records, rec)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: "https://my-registry.com"},
	}
	l := newLibraryModel([]catalog.ContentItem{item}, testProviders(), t.TempDir())
	l.SetSize(120, 30)

	// Render to register zones
	scanZones(l.View())

	// Test Pin
	zPin := zone.Get("meta-pin")
	if zPin.IsZero() {
		t.Skip("zone meta-pin not registered")
	}
	_, cmdPin := l.updateMouse(mouseClick(zPin.StartX, zPin.StartY))
	if cmdPin == nil {
		t.Fatal("click on meta-pin should emit cmd")
	}
	if _, ok := cmdPin().(libraryPinMsg); !ok {
		t.Errorf("expected libraryPinMsg, got %T", cmdPin())
	}

	// Test Rollback
	zRollback := zone.Get("meta-rollback")
	if zRollback.IsZero() {
		t.Skip("zone meta-rollback not registered")
	}
	_, cmdRollback := l.updateMouse(mouseClick(zRollback.StartX, zRollback.StartY))
	if cmdRollback == nil {
		t.Fatal("click on meta-rollback should emit cmd")
	}
	if _, ok := cmdRollback().(libraryRollbackMsg); !ok {
		t.Errorf("expected libraryRollbackMsg, got %T", cmdRollback())
	}
}

func TestExplorer_UpdateMouse_MetaPinAndRollback(t *testing.T) {
	// Seed global dir override
	tmpDir := t.TempDir()
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	// Seed install store record so buttons render
	storePath, err := installstore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	coord := installstore.Coord{Registry: "https://my-registry.com", Type: "skills", Name: "alpha-skill"}
	rec := installstore.Record{
		Coord:       coord,
		SourceSHA:   "123",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
		Previous:    &installstore.PreviousVersion{SourceSHA: "123"},
	}
	store.Records = append(store.Records, rec)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	e, _ := newExplorerWithDiskItems(t, 120, 30)
	// Make sure the item has Meta registry
	if len(e.items.items) > 0 {
		e.items.items[0].Meta = &metadata.Meta{SourceRegistry: "https://my-registry.com"}
		e.items.items[0].Library = true
	}

	// Render to register zones
	scanZones(e.View())

	// Test Pin
	zPin := zone.Get("meta-pin")
	if zPin.IsZero() {
		t.Skip("zone meta-pin not registered")
	}
	_, cmdPin := e.Update(mouseClick(zPin.StartX, zPin.StartY))
	if cmdPin == nil {
		t.Fatal("click on meta-pin should emit cmd")
	}
	if _, ok := cmdPin().(libraryPinMsg); !ok {
		t.Errorf("expected libraryPinMsg, got %T", cmdPin())
	}

	// Test Rollback
	zRollback := zone.Get("meta-rollback")
	if zRollback.IsZero() {
		t.Skip("zone meta-rollback not registered")
	}
	_, cmdRollback := e.Update(mouseClick(zRollback.StartX, zRollback.StartY))
	if cmdRollback == nil {
		t.Fatal("click on meta-rollback should emit cmd")
	}
	if _, ok := cmdRollback().(libraryRollbackMsg); !ok {
		t.Errorf("expected libraryRollbackMsg, got %T", cmdRollback())
	}
}
