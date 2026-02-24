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

	// Cursor at 0 = "See what's new" → triggers fetchReleaseNotes command
	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected fetchReleaseNotes command")
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
	// When no update available, cursor 1 = "Check for updates"
	app := navigateToUpdate(t)
	app = pressN(app, keyDown, 1) // Move to "Check for updates"

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected check/pull command")
	}
}

func TestUpdateCheckForUpdatesClearsLoading(t *testing.T) {
	// "Check for updates" dispatches checkForUpdate which returns updateCheckMsg.
	// The updater must clear loading when it receives the response.
	app := navigateToUpdate(t)
	app.updater.loading = true // simulate waiting for check

	msg := updateCheckMsg{
		localVersion:  "1.0.0",
		remoteVersion: "1.0.0",
		commitsBehind: 0,
	}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.loading {
		t.Fatal("loading should be false after updateCheckMsg")
	}
	if app.updater.step != stepUpdateMenu {
		t.Fatalf("expected stepUpdateMenu, got %d", app.updater.step)
	}
}

func TestUpdateCheckForUpdatesFindsUpdate(t *testing.T) {
	// When "Check for updates" finds a newer version, updater should reflect it.
	app := navigateToUpdate(t)
	app.updater.loading = true

	msg := updateCheckMsg{
		localVersion:  "1.0.0",
		remoteVersion: "2.0.0",
		commitsBehind: 3,
	}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.loading {
		t.Fatal("loading should be false")
	}
	if !app.updater.updateAvail {
		t.Fatal("expected updateAvail=true after finding newer version")
	}
	if app.updater.remoteVersion != "2.0.0" {
		t.Fatalf("expected remoteVersion 2.0.0, got %s", app.updater.remoteVersion)
	}
}

func TestUpdateMenuViewReleaseNotes(t *testing.T) {
	// When on latest, cursor 0 = "View release notes"
	app := navigateToUpdate(t)

	// Cursor starts at 0 = "View release notes"
	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected fetchLocalReleaseNotes command")
	}
	if app.updater.updateAvail {
		t.Fatal("expected no update available")
	}
}

func TestUpdateMenuNavigationLatest(t *testing.T) {
	// When on latest, two options: "View release notes"(0), "Check for updates"(1)
	app := navigateToUpdate(t)

	if app.updater.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", app.updater.cursor)
	}

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

	// Back up
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.updater.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", app.updater.cursor)
	}
}

// ---------------------------------------------------------------------------
// Preview
// ---------------------------------------------------------------------------

func TestUpdatePreviewScroll(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePreview
	app.updater.fallbackLog = "commit1\ncommit2\ncommit3\ncommit4\ncommit5"
	app.updater.fallbackStat = "file1.go | 5 +++\nfile2.go | 3 ---"

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

func TestUpdatePreviewReleaseNotes(t *testing.T) {
	app := navigateToUpdateWithRemote(t)
	app.updater.step = stepUpdatePreview
	app.updater.releaseNotes = "# v2.0.0\n\nSome rendered content"
	app.updater.versionRange = "v1.0.0 -> v2.0.0"

	view := app.View()
	assertContains(t, view, "v1.0.0 -> v2.0.0")
	assertContains(t, view, "Some rendered content")
}

func TestUpdatePreviewNoUpdateEnterIgnored(t *testing.T) {
	// When viewing local release notes (no update), Enter should be a no-op
	app := navigateToUpdate(t)
	app.updater.step = stepUpdatePreview
	app.updater.releaseNotes = "# v1.0.0\n\nLocal notes"

	m, cmd := app.Update(keyEnter)
	app = m.(App)
	if cmd != nil {
		t.Fatal("expected no command (Enter should be no-op without update)")
	}
	if app.updater.step != stepUpdatePreview {
		t.Fatalf("expected to stay on preview, got step %d", app.updater.step)
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
		fallbackLog:  "abc1234 Fix bug",
		fallbackStat: "file.go | 5 +++",
	}

	m, _ := app.Update(msg)
	app = m.(App)

	if app.updater.step != stepUpdatePreview {
		t.Fatalf("expected stepUpdatePreview, got %d", app.updater.step)
	}
	if app.updater.fallbackLog != "abc1234 Fix bug" {
		t.Fatalf("expected fallback log, got %q", app.updater.fallbackLog)
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

func TestVersionInRange(t *testing.T) {
	tests := []struct {
		v, localV, remoteV string
		want               bool
	}{
		// In range: strictly newer than local, at most remote
		{"v0.2.1", "0.2.0", "0.3.0", true},
		{"v0.3.0", "0.2.0", "0.3.0", true}, // upper bound inclusive
		{"v0.2.2", "0.2.0", "0.3.0", true},

		// At lower boundary (exclusive): not in range
		{"v0.2.0", "0.2.0", "0.3.0", false},

		// Beyond upper boundary: not in range
		{"v0.4.0", "0.2.0", "0.3.0", false},

		// Below lower boundary: not in range
		{"v0.1.0", "0.2.0", "0.3.0", false},

		// Equal to both bounds (local == remote): not in range
		{"v1.0.0", "1.0.0", "1.0.0", false},

		// With v prefix on all
		{"v2.0.0", "v1.0.0", "v2.0.0", true},
	}

	for _, tt := range tests {
		got := versionInRange(tt.v, tt.localV, tt.remoteV)
		if got != tt.want {
			t.Errorf("versionInRange(%q, %q, %q) = %v, want %v",
				tt.v, tt.localV, tt.remoteV, got, tt.want)
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

	msg := updatePreviewMsg{fallbackLog: "commit1", fallbackStat: "file.go | 1 +"}
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

func TestUpdateViewMenuLatest(t *testing.T) {
	app := navigateToUpdate(t)
	view := app.View()
	assertContains(t, view, "Update nesco")
	assertContains(t, view, "View release notes")
	assertContains(t, view, "Check for updates")
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
