package tui

import (
	"fmt"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

// navigateToUpdate creates a test app and navigates to the update screen.
func navigateToUpdate(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+2) // Update
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenUpdate)
	return app
}

// navigateToUpdateWithRemote creates an app that has a newer remote version available.
func navigateToUpdateWithRemote(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	app.remoteVersion = "2.0.0"
	app.commitsBehind = 5
	app.sidebar.remoteVersion = "2.0.0"
	app.sidebar.updateAvailable = true

	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+2)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenUpdate)
	return app
}

// ---------------------------------------------------------------------------
// Menu
// ---------------------------------------------------------------------------

func TestUpdateMenuNavigation(t *testing.T) {
	app := navigateToUpdateWithRemote(t)

	if !app.updater.updateAvail {
		t.Fatal("expected update to be available")
	}

	// 2 options when update available: "See what's new"(0), "Update now"(1)
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.updater.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.updater.cursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.updater.cursor != 1 {
		t.Fatal("cursor should clamp at 1")
	}
}

func TestUpdateMenuSeeWhatsNew(t *testing.T) {
	app := navigateToUpdateWithRemote(t)

	// Cursor at 0 = "See what's new" → triggers fetchPreview command
	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected fetchPreview command")
	}
}

func TestUpdateMenuUpdateNow(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app = pressN(app, keyDown, 1) // "Update now"

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected startPull command")
	}
}

func TestUpdateMenuCheckForUpdates(t *testing.T) {
	// When no update available, only "Check for updates" option
	app := navigateToUpdate(t)

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected check/pull command")
	}
}

// ---------------------------------------------------------------------------
// Preview
// ---------------------------------------------------------------------------

func TestUpdatePreviewScroll(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePreview
	app.updater.previewLog = "commit1\ncommit2\ncommit3\ncommit4\ncommit5"
	app.updater.previewStat = "file1.go | 5 +++\nfile2.go | 3 ---"

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.updater.scrollOffset != 1 {
		t.Fatalf("expected scrollOffset 1, got %d", app.updater.scrollOffset)
	}

	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.updater.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset 0, got %d", app.updater.scrollOffset)
	}
}

func TestUpdatePreviewEnterPull(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePreview

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected startPull command from preview")
	}
}

func TestUpdatePreviewEscBack(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePreview

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.updater.step != stepUpdateMenu {
		t.Fatalf("expected stepUpdateMenu after esc, got %d", app.updater.step)
	}
}

// ---------------------------------------------------------------------------
// Pull (step 2)
// ---------------------------------------------------------------------------

func TestUpdatePullIgnoresKeys(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePull

	// Keys should be ignored while pull is running
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.updater.step != stepUpdatePull {
		t.Fatal("keys should be ignored during pull")
	}

	m, _ = app.Update(keyEsc)
	app = m.(App)
	// App-level esc only works from stepUpdateMenu or stepUpdateDone
	// so we should still be on update screen
	assertScreen(t, app, screenUpdate)
}

// ---------------------------------------------------------------------------
// Done (step 3)
// ---------------------------------------------------------------------------

func TestUpdateDoneEsc(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdateDone

	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

func TestUpdateCheckMsgSuccess(t *testing.T) {
	app := testApp(t)
	msg := updateCheckMsg{
		localVersion:  "1.0.0",
		remoteVersion: "2.0.0",
		commitsBehind: 5,
	}

	m, _ := app.Update(msg)
	app = m.(App)

	if !app.sidebar.updateAvailable {
		t.Fatal("expected updateAvailable=true")
	}
	if app.remoteVersion != "2.0.0" {
		t.Fatalf("expected remoteVersion 2.0.0, got %s", app.remoteVersion)
	}
}

func TestUpdateCheckMsgSameVersion(t *testing.T) {
	app := testApp(t)
	msg := updateCheckMsg{
		localVersion:  "1.0.0",
		remoteVersion: "1.0.0",
		commitsBehind: 0,
	}

	m, _ := app.Update(msg)
	app = m.(App)

	if app.sidebar.updateAvailable {
		t.Fatal("expected updateAvailable=false when versions match")
	}
}

func TestUpdateCheckMsgError(t *testing.T) {
	app := testApp(t)
	msg := updateCheckMsg{
		err: fmt.Errorf("network error"),
	}

	m, _ := app.Update(msg)
	app = m.(App)

	// Should not crash, and updateAvailable should remain false
	if app.sidebar.updateAvailable {
		t.Fatal("updateAvailable should be false on error")
	}
}

func TestUpdatePreviewMsgSuccess(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdateMenu // simulate waiting for preview

	msg := updatePreviewMsg{
		log:  "abc1234 Fix bug",
		stat: "file.go | 5 +++",
	}

	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.step != stepUpdatePreview {
		t.Fatalf("expected stepUpdatePreview, got %d", app.updater.step)
	}
	if app.updater.previewLog != "abc1234 Fix bug" {
		t.Fatalf("expected preview log, got %q", app.updater.previewLog)
	}
}

func TestUpdatePreviewMsgError(t *testing.T) {
	app := navigateToUpdateWithRemote(t)

	msg := updatePreviewMsg{err: fmt.Errorf("git log failed")}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.previewErr == nil {
		t.Fatal("expected previewErr to be set")
	}
}

func TestUpdatePullMsgSuccess(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePull

	msg := updatePullMsg{output: "Already up to date."}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.step != stepUpdateDone {
		t.Fatalf("expected stepUpdateDone, got %d", app.updater.step)
	}
	if app.updater.pullErr != nil {
		t.Fatal("expected no pull error")
	}
}

func TestUpdatePullMsgError(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePull

	msg := updatePullMsg{err: fmt.Errorf("merge conflict")}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.step != stepUpdateDone {
		t.Fatalf("expected stepUpdateDone, got %d", app.updater.step)
	}
	if app.updater.pullErr == nil {
		t.Fatal("expected pullErr to be set")
	}
}

// ---------------------------------------------------------------------------
// Auto-update
// ---------------------------------------------------------------------------

func TestUpdateAutoUpdate(t *testing.T) {
	app := testApp(t)
	app.autoUpdate = true

	msg := updateCheckMsg{
		localVersion:  "1.0.0",
		remoteVersion: "2.0.0",
		commitsBehind: 3,
	}

	m, cmd := app.Update(msg)
	app = m.(App)

	// Auto-update should immediately transition to update screen and start pull
	assertScreen(t, app, screenUpdate)
	if cmd == nil {
		t.Fatal("expected startPull command for auto-update")
	}
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func TestVersionNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"2.0.0", "1.0.0", true},
		{"1.1.0", "1.0.0", true},
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "2.0.0", false},
		{"1.0.0", "1.1.0", false},
		{"v2.0.0", "v1.0.0", true},
		{"10.0.0", "9.0.0", true},
	}

	for _, tt := range tests {
		got := versionNewer(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"10.20.30", [3]int{10, 20, 30}},
		{"1.0.0", [3]int{1, 0, 0}},
		{"", [3]int{0, 0, 0}},
	}

	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Spinner / Loading
// ---------------------------------------------------------------------------

func TestUpdateSpinnerDuringPull(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePull
	app.updater.loading = true

	view := app.View()
	assertContains(t, view, "Updating nesco")
}

func TestUpdateSpinnerDuringFetch(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.loading = true

	view := app.View()
	assertContains(t, view, "Fetching update info")
}

func TestUpdateLoadingClearedOnPreviewMsg(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.loading = true

	msg := updatePreviewMsg{log: "commit1", stat: "file.go | 1 +"}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.loading {
		t.Fatal("loading should be false after preview msg")
	}
}

func TestUpdateLoadingClearedOnPullMsg(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePull
	app.updater.loading = true

	msg := updatePullMsg{output: "Already up to date."}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.loading {
		t.Fatal("loading should be false after pull msg")
	}
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func TestUpdateViewMenu(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	view := app.View()
	assertContains(t, view, "Update nesco")
	assertContains(t, view, "See what's new")
	assertContains(t, view, "Update now")
}

func TestUpdateViewDoneSuccess(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdateDone
	app.updater.pullErr = nil

	view := app.View()
	assertContains(t, view, "Updated to")
}

func TestUpdateViewDoneError(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdateDone
	app.updater.pullErr = fmt.Errorf("merge conflict")

	view := app.View()
	assertContains(t, view, "Update failed")
}
