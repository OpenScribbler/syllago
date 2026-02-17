package tui

import (
	"fmt"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

// navigateToImport creates a test app and navigates to the import screen.
func navigateToImport(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+1) // Import
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenImport)
	return app
}

// ---------------------------------------------------------------------------
// Source selection (step 0)
// ---------------------------------------------------------------------------

func TestImportSourceNavigation(t *testing.T) {
	app := navigateToImport(t)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource, got %d", app.importer.step)
	}

	// 3 options: Local(0), Git(1), Create(2)
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 1 {
		t.Fatalf("expected sourceCursor 1, got %d", app.importer.sourceCursor)
	}

	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 2 {
		t.Fatalf("expected sourceCursor 2, got %d", app.importer.sourceCursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 2 {
		t.Fatal("sourceCursor should clamp at 2")
	}
}

func TestImportSourceSelectLocal(t *testing.T) {
	app := navigateToImport(t)
	// Local = cursor 0, just Enter
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after local, got %d", app.importer.step)
	}
	if app.importer.isCreate {
		t.Fatal("isCreate should be false for local")
	}
}

func TestImportSourceSelectGit(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1) // Git
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL after git, got %d", app.importer.step)
	}
}

func TestImportSourceSelectCreate(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2) // Create
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after create, got %d", app.importer.step)
	}
	if !app.importer.isCreate {
		t.Fatal("isCreate should be true for create")
	}
}

func TestImportSourceEscBack(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEsc) // Esc from stepSource at app level → category
	app = m.(App)
	assertScreen(t, app, screenCategory)
}

// ---------------------------------------------------------------------------
// Type selection (step 1)
// ---------------------------------------------------------------------------

func TestImportTypeNavigation(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)

	nTypes := len(app.importer.types)
	app = pressN(app, keyDown, nTypes+5)
	if app.importer.typeCursor != nTypes-1 {
		t.Fatalf("expected typeCursor clamped at %d, got %d", nTypes-1, app.importer.typeCursor)
	}
}

func TestImportTypeUniversalToBrowse(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)

	// Skills is universal and at cursor 0
	m, _ = app.Update(keyEnter) // select Skills
	app = m.(App)
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart for universal type, got %d", app.importer.step)
	}
}

func TestImportTypeProviderSpecificToProvider(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)

	// Find a provider-specific type (e.g., Rules)
	for i, ct := range app.importer.types {
		if !ct.IsUniversal() {
			app = pressN(app, keyDown, i)
			break
		}
	}

	m, _ = app.Update(keyEnter)
	app = m.(App)

	// Should go to stepProvider or show "no provider dirs" message
	if app.importer.step == stepProvider {
		// Good — found provider directories
	} else if app.importer.message != "" {
		// Also acceptable — no provider directories found
	} else {
		t.Fatalf("expected stepProvider or error message, got step %d", app.importer.step)
	}
}

func TestImportTypeCreateToName(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2) // Create
	m, _ := app.Update(keyEnter)  // → stepType (create flow)
	app = m.(App)

	// Select any type
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName for create flow, got %d", app.importer.step)
	}
}

func TestImportTypeEscBack(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)

	m, _ = app.Update(keyEsc) // back to source
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Browse start (step 3)
// ---------------------------------------------------------------------------

func TestImportBrowseStartOptions(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)
	m, _ = app.Update(keyEnter) // → stepBrowseStart (Skills)
	app = m.(App)

	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart, got %d", app.importer.step)
	}

	// 3 options: cwd(0), home(1), custom(2)
	app = pressN(app, keyDown, 2)
	if app.importer.browseCursor != 2 {
		t.Fatalf("expected browseCursor 2, got %d", app.importer.browseCursor)
	}
}

func TestImportBrowseStartEsc(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)
	m, _ = app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)

	m, _ = app.Update(keyEsc) // back
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after esc from browseStart, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Path input (step 6)
// ---------------------------------------------------------------------------

func TestImportPathInput(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)
	m, _ = app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)

	// Select custom path (option 2)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	if app.importer.step != stepPath {
		t.Fatalf("expected stepPath, got %d", app.importer.step)
	}

	// Enter with empty path stays
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepPath {
		t.Fatalf("expected to stay at stepPath with empty input, got %d", app.importer.step)
	}
}

func TestImportPathInvalid(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)
	m, _ = app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	// Type an invalid path
	app.importer.pathInput.SetValue("/nonexistent/path/xyz")
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if app.importer.message == "" {
		t.Fatal("expected error message for invalid path")
	}
}

func TestImportPathEsc(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEnter) // → stepType
	app = m.(App)
	m, _ = app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Git URL input (step 7)
// ---------------------------------------------------------------------------

func TestImportGitURLEmpty(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1) // Git
	m, _ := app.Update(keyEnter)  // → stepGitURL
	app = m.(App)

	// Enter with empty URL stays
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected to stay at stepGitURL with empty URL, got %d", app.importer.step)
	}
}

func TestImportGitURLInvalid(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // → stepGitURL
	app = m.(App)

	// Type invalid URL
	app.importer.urlInput.SetValue("not-a-valid-url")
	m, _ = app.Update(keyEnter)
	app = m.(App)

	assertContains(t, app.importer.message, "Invalid URL")
}

func TestImportGitURLEsc(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // → stepGitURL
	app = m.(App)

	m, _ = app.Update(keyEsc) // back to source
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Clone done message handling
// ---------------------------------------------------------------------------

func TestImportCloneDoneSuccess(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitURL

	// Simulate successful clone with scanned items
	tmpDir := t.TempDir()
	// Create a scannable skill in the temp dir
	makeSkill(t, tmpDir, "cloned-skill", "From git", false)

	msg := importCloneDoneMsg{err: nil, path: tmpDir}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepGitPick {
		t.Fatalf("expected stepGitPick after successful clone, got %d", app.importer.step)
	}
	if len(app.importer.clonedItems) == 0 {
		t.Fatal("expected cloned items after scan")
	}
}

func TestImportCloneDoneError(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitURL

	msg := importCloneDoneMsg{err: fmt.Errorf("clone failed"), path: ""}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepGitURL {
		t.Fatalf("expected to stay at stepGitURL on error, got %d", app.importer.step)
	}
	assertContains(t, app.importer.message, "Clone failed")
}

// ---------------------------------------------------------------------------
// Git pick (step 8)
// ---------------------------------------------------------------------------

func TestImportGitPickNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitPick
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "item-1", Type: catalog.Skills},
		{Name: "item-2", Type: catalog.Agents},
	}
	app.importer.pickCursor = 0

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.pickCursor != 1 {
		t.Fatalf("expected pickCursor 1, got %d", app.importer.pickCursor)
	}
}

func TestImportGitPickSelect(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitPick
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "picked-item", Type: catalog.Skills, Path: "/tmp/picked"},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", app.importer.step)
	}
	if app.importer.itemName != "picked-item" {
		t.Fatalf("expected itemName 'picked-item', got %q", app.importer.itemName)
	}
}

func TestImportGitPickEscCleanup(t *testing.T) {
	app := navigateToImport(t)
	tmpDir := t.TempDir()
	app.importer.step = stepGitPick
	app.importer.clonedPath = tmpDir
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "temp-item", Type: catalog.Skills},
	}

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL after esc, got %d", app.importer.step)
	}
	if app.importer.clonedPath != "" {
		t.Fatal("expected clonedPath to be cleaned up")
	}
}

// ---------------------------------------------------------------------------
// Name input (step 10 - create flow)
// ---------------------------------------------------------------------------

func TestImportNameEnter(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = true
	app.importer.nameInput.SetValue("my-new-skill")

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", app.importer.step)
	}
	if app.importer.itemName != "my-new-skill" {
		t.Fatalf("expected itemName 'my-new-skill', got %q", app.importer.itemName)
	}
}

func TestImportNameEmpty(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName
	app.importer.nameInput.SetValue("")

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatal("expected to stay at stepName with empty name")
	}
}

func TestImportNameEsc(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Validate (step 5)
// ---------------------------------------------------------------------------

func TestImportValidateNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: true},
		{name: "item-2", included: true},
		{name: "item-3", included: true},
	}

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.validateCursor != 1 {
		t.Fatalf("expected validateCursor 1, got %d", app.importer.validateCursor)
	}
}

func TestImportValidateToggle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: true},
	}

	m, _ := app.Update(keySpace)
	app = m.(App)
	if app.importer.validationItems[0].included {
		t.Fatal("space should toggle included to false")
	}

	m, _ = app.Update(keySpace)
	app = m.(App)
	if !app.importer.validationItems[0].included {
		t.Fatal("space should toggle included back to true")
	}
}

func TestImportValidateNoneSelected(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: false},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertContains(t, app.importer.message, "No items selected")
}

func TestImportValidateSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills
	app.importer.validationItems = []validationItem{
		{path: "/tmp/single-item", name: "single-item", included: true},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm for single selection, got %d", app.importer.step)
	}
}

func TestImportValidateEsc(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepBrowse {
		t.Fatalf("expected stepBrowse after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Confirm (step 9)
// ---------------------------------------------------------------------------

func TestImportConfirmEnter(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.sourcePath = app.catalog.Items[0].Path
	app.importer.itemName = "confirm-test"
	app.importer.contentType = catalog.Skills

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected import command on confirm")
	}
}

func TestImportConfirmEscLocal(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.isCreate = false
	app.importer.clonedPath = "" // not from git

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepValidate {
		t.Fatalf("expected stepValidate after esc from confirm (local flow), got %d", app.importer.step)
	}
}

func TestImportConfirmEscGit(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.isCreate = false
	app.importer.clonedPath = "/tmp/fake-clone"

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepGitPick {
		t.Fatalf("expected stepGitPick after esc from confirm (git flow), got %d", app.importer.step)
	}
}

func TestImportConfirmEscCreate(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.isCreate = true

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName after esc from confirm (create flow), got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Import done message
// ---------------------------------------------------------------------------

func TestImportDoneSuccess(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "imported-item", err: nil}
	m, _ := app.Update(msg)
	app = m.(App)

	assertScreen(t, app, screenCategory)
	assertContains(t, app.category.message, "imported-item")
}

func TestImportDoneError(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "", err: fmt.Errorf("copy failed")}
	m, _ := app.Update(msg)
	app = m.(App)

	assertScreen(t, app, screenImport) // stays on import screen
	assertContains(t, app.importer.message, "Import failed")
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func TestImportViewSource(t *testing.T) {
	app := navigateToImport(t)
	view := app.View()
	assertContains(t, view, "Import Content")
	assertContains(t, view, "Local Path")
	assertContains(t, view, "Git URL")
	assertContains(t, view, "Create New")
}

func TestImportShowsStepIndicator(t *testing.T) {
	app := navigateToImport(t)
	view := app.View()
	assertContains(t, view, "Step 1")
}

func TestImportShowsBreadcrumb(t *testing.T) {
	app := navigateToImport(t)
	view := app.View()
	assertContains(t, view, "nesco >")
}

// ---------------------------------------------------------------------------
// Git URL validation: secure transports only
// ---------------------------------------------------------------------------

func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://github.com/user/repo.git", true},
		{"ssh://git@github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"git://github.com/user/repo.git", false}, // insecure
		{"http://github.com/user/repo.git", false}, // insecure
		{"ext::sh -c 'evil'", false},               // command injection
		{"-u flag injection", false},                // argument injection
		{"", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isValidGitURL(tt.url)
			if got != tt.valid {
				t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, got, tt.valid)
			}
		})
	}
}
