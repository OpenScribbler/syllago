package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// installWriterGlobally installs item to prov under HOME and returns the
// placed path.
func installWriterGlobally(t *testing.T, app App, item catalog.ContentItem, prov provider.Provider) string {
	t.Helper()
	msg := app.doInstallCmd(installResultMsg{item: item, provider: prov, method: installer.MethodSymlink, location: "global"})().(installDoneMsg)
	if msg.err != nil {
		t.Fatalf("install: %v", msg.err)
	}
	return msg.targetPath
}

func TestDoUninstallCmd_RemovesPlacementAndRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	app := testApp(t)
	prov := skillProvider("test-prov")
	item := registrySkill(t)
	placed := installWriterGlobally(t, app, item, prov)

	msg := app.doUninstallCmd(confirmResultMsg{item: item, uninstallProviders: []provider.Provider{prov}})().(uninstallDoneMsg)

	if msg.err != nil || len(msg.warnings) != 0 {
		t.Fatalf("err = %v, warnings = %q", msg.err, msg.warnings)
	}
	if len(msg.uninstalledFrom) != 1 || msg.uninstalledFrom[0] != "test-prov" {
		t.Errorf("uninstalledFrom = %q", msg.uninstalledFrom)
	}
	if _, err := os.Lstat(placed); !os.IsNotExist(err) {
		t.Errorf("%s still exists", placed)
	}
	if rec := mustLoadTUIInstallRecordStore(t, configDir).Find(writerCoord); rec != nil {
		t.Errorf("record = %+v, want it gone", rec)
	}
}

// A provider the item never reached no longer vanishes from a partly
// successful uninstall: the toast names it.
func TestDoUninstallCmd_PartialFailureWarns(t *testing.T) {
	withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	app := testApp(t)
	good := skillProvider("good")
	item := registrySkill(t)
	installWriterGlobally(t, app, item, good)

	msg := app.doUninstallCmd(confirmResultMsg{
		item:               item,
		uninstallProviders: []provider.Provider{good, skillProvider("absent")},
	})().(uninstallDoneMsg)

	if msg.err != nil {
		t.Fatalf("err = %v, want nil: good was uninstalled", msg.err)
	}
	if len(msg.warnings) != 1 || !strings.Contains(msg.warnings[0], "not uninstalled from absent") {
		t.Errorf("warnings = %q, want one naming absent", msg.warnings)
	}
}

func TestDoUninstallCmd_WaitsForInstallLock(t *testing.T) {
	withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	app := testApp(t)
	prov := skillProvider("test-prov")
	item := registrySkill(t)
	placed := installWriterGlobally(t, app, item, prov)
	origTimeout := syllagolock.DefaultTimeout
	syllagolock.DefaultTimeout = 50 * time.Millisecond
	t.Cleanup(func() { syllagolock.DefaultTimeout = origTimeout })
	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)

	msg := app.doUninstallCmd(confirmResultMsg{item: item, uninstallProviders: []provider.Provider{prov}})().(uninstallDoneMsg)

	var se output.StructuredError
	if !errors.As(msg.err, &se) || se.Code != output.ErrSystemLocked {
		t.Fatalf("err = %v, want %s", msg.err, output.ErrSystemLocked)
	}
	if _, err := os.Lstat(placed); err != nil {
		t.Errorf("placement removed while the lock was held: %v", err)
	}
}

func TestActions_HandleUninstallDone_WarningsShow(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.handleUninstallDone(uninstallDoneMsg{
		itemName:        "my-skill",
		uninstalledFrom: []string{"Claude Code"},
		warnings:        []string{"could not record install state: disk full"},
	})
	result := m.(App)
	cur := result.toast.Current()
	if cur == nil || cur.level != toastWarning || len(cur.details) != 1 {
		t.Fatalf("uninstall with warnings should raise a warning toast with details, got %+v", cur)
	}
}
