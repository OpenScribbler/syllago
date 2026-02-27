package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
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
	assertContains(t, app.statusMessage, "imported-item")
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
	assertContains(t, view, "Import AI Tools")
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
	assertContains(t, view, "Home")
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
		{"git://github.com/user/repo.git", false},  // insecure
		{"http://github.com/user/repo.git", false}, // insecure
		{"ext::sh -c 'evil'", false},               // command injection
		{"-u flag injection", false},               // argument injection
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

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestCollectRelativeFiles(t *testing.T) {
	tmp := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)
	os.WriteFile(filepath.Join(tmp, "a.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "sub", "b.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(tmp, ".nesco.yaml"), []byte("meta"), 0o644)
	// Create symlink (should be skipped)
	os.Symlink(filepath.Join(tmp, "a.md"), filepath.Join(tmp, "link.md"))

	files := collectRelativeFiles(tmp)

	// Should contain a.md and sub/b.md, sorted
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != "a.md" {
		t.Errorf("expected files[0]='a.md', got %q", files[0])
	}
	if files[1] != filepath.Join("sub", "b.md") {
		t.Errorf("expected files[1]='sub/b.md', got %q", files[1])
	}
}

func TestBuildConflictInfoDirectories(t *testing.T) {
	existing := t.TempDir()
	source := t.TempDir()

	// A only in existing, B in both, C only in new
	os.WriteFile(filepath.Join(existing, "a.md"), []byte("old-a"), 0o644)
	os.WriteFile(filepath.Join(existing, "b.md"), []byte("old-b"), 0o644)
	os.WriteFile(filepath.Join(source, "b.md"), []byte("new-b"), 0o644)
	os.WriteFile(filepath.Join(source, "c.md"), []byte("new-c"), 0o644)

	m := importModel{}
	ci := m.buildConflictInfo(existing, source, "test-item")

	if len(ci.onlyExisting) != 1 || ci.onlyExisting[0] != "a.md" {
		t.Errorf("expected onlyExisting=[a.md], got %v", ci.onlyExisting)
	}
	if len(ci.inBoth) != 1 || ci.inBoth[0] != "b.md" {
		t.Errorf("expected inBoth=[b.md], got %v", ci.inBoth)
	}
	if len(ci.onlyNew) != 1 || ci.onlyNew[0] != "c.md" {
		t.Errorf("expected onlyNew=[c.md], got %v", ci.onlyNew)
	}
}

func TestBuildConflictInfoSingleFile(t *testing.T) {
	existing := t.TempDir()
	os.WriteFile(filepath.Join(existing, "tool.sh"), []byte("old"), 0o644)

	// Source is a single file (universal type wrapping)
	sourceFile := filepath.Join(t.TempDir(), "tool.sh")
	os.WriteFile(sourceFile, []byte("new"), 0o644)

	m := importModel{}
	ci := m.buildConflictInfo(existing, sourceFile, "tool")

	// tool.sh exists in both
	if len(ci.inBoth) != 1 || ci.inBoth[0] != "tool.sh" {
		t.Errorf("expected inBoth=[tool.sh], got %v", ci.inBoth)
	}
}

func TestBuildConflictInfoCreateFlow(t *testing.T) {
	existing := t.TempDir()
	os.WriteFile(filepath.Join(existing, "SKILL.md"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(existing, "README.md"), []byte("readme"), 0o644)

	m := importModel{isCreate: true}
	ci := m.buildConflictInfo(existing, "", "test-item")

	// All existing files should be in onlyExisting
	if len(ci.onlyExisting) != 2 {
		t.Errorf("expected 2 onlyExisting, got %d: %v", len(ci.onlyExisting), ci.onlyExisting)
	}
	if len(ci.onlyNew) != 0 {
		t.Errorf("expected empty onlyNew, got %v", ci.onlyNew)
	}
	if len(ci.inBoth) != 0 {
		t.Errorf("expected empty inBoth, got %v", ci.inBoth)
	}
}

// ---------------------------------------------------------------------------
// Single import conflict tests
// ---------------------------------------------------------------------------

func TestConflictDetectionOnConfirm(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false

	// Create source
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0o644)
	app.importer.sourcePath = srcDir
	app.importer.itemName = "conflict-test"

	// Create the destination so it conflicts
	dest := filepath.Join(app.catalog.RepoRoot, "local", "skills", "conflict-test")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("existing"), 0o644)

	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.importer.step != stepConflict {
		t.Fatalf("expected stepConflict, got %d", app.importer.step)
	}
}

func TestNoConflictPassesThrough(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false
	app.importer.sourcePath = app.catalog.Items[0].Path
	app.importer.itemName = "no-conflict-unique-name"

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected import command (no conflict), got nil")
	}
}

func TestConflictOverwriteSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false

	// Set up source and destination
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("new"), 0o644)
	app.importer.sourcePath = srcDir
	app.importer.itemName = "overwrite-test"

	dest := filepath.Join(app.catalog.RepoRoot, "local", "skills", "overwrite-test")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("old"), 0o644)

	app.importer.conflict = conflictInfo{
		existingPath: dest,
		sourcePath:   srcDir,
		itemName:     "overwrite-test",
		inBoth:       []string{"SKILL.md"},
	}

	// Press 'y' to overwrite
	_, cmd := app.Update(keyRune('y'))
	if cmd == nil {
		t.Fatal("expected overwrite import command, got nil")
	}
}

func TestConflictCancelSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		existingPath: "/fake/dest",
		sourcePath:   "/fake/src",
		itemName:     "cancel-test",
	}
	// No batch conflicts (single mode)
	app.importer.batchConflicts = nil

	m, _ := app.Update(keyEsc)
	app = m.(App)

	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm after esc, got %d", app.importer.step)
	}
}

func TestConflictScrolling(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		diffText: "--- a/file.md\n+++ b/file.md\n@@ -1,1 +1,1 @@\n-old\n+new",
	}

	// Scroll down
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 1 {
		t.Fatalf("expected scrollOffset 1, got %d", app.importer.conflict.scrollOffset)
	}

	// Scroll back up
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset 0, got %d", app.importer.conflict.scrollOffset)
	}

	// Clamp at top
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 0 {
		t.Fatal("scrollOffset should clamp at 0")
	}
}

// ---------------------------------------------------------------------------
// Batch conflict tests
// ---------------------------------------------------------------------------

func TestBatchConflictDetection(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills

	// Create 3 source items
	src1 := filepath.Join(t.TempDir(), "item-1")
	src2 := filepath.Join(t.TempDir(), "item-2")
	src3 := filepath.Join(t.TempDir(), "item-3")
	for _, d := range []string{src1, src2, src3} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("content"), 0o644)
	}

	app.importer.validationItems = []validationItem{
		{path: src1, name: "item-1", included: true},
		{path: src2, name: "item-2", included: true},
		{path: src3, name: "item-3", included: true},
	}

	// Create destinations for item-1 and item-3 so they conflict
	for _, name := range []string{"item-1", "item-3"} {
		dest := filepath.Join(app.catalog.RepoRoot, "local", "skills", name)
		os.MkdirAll(dest, 0o755)
		os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("existing"), 0o644)
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.importer.step != stepConflict {
		t.Fatalf("expected stepConflict, got %d", app.importer.step)
	}
	if len(app.importer.batchConflicts) != 2 {
		t.Fatalf("expected 2 batch conflicts, got %d", len(app.importer.batchConflicts))
	}
}

func TestBatchNoConflicts(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "unique-1")
	src2 := filepath.Join(t.TempDir(), "unique-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("content"), 0o644)
	}

	app.importer.validationItems = []validationItem{
		{path: src1, name: "unique-1", included: true},
		{path: src2, name: "unique-2", included: true},
	}

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected batch import command (no conflicts)")
	}
}

func TestBatchConflictStepThrough(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "batch-1")
	src2 := filepath.Join(t.TempDir(), "batch-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("new"), 0o644)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "batch-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "batch-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("old"), 0o644)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "batch-1")

	// Press 'y' on first conflict → overwrite marked, advances
	m, _ := app.Update(keyRune('y'))
	app = m.(App)

	if !app.importer.batchOverwrite[src1] {
		t.Fatal("expected batch-1 marked for overwrite")
	}
	if app.importer.batchConflictIdx != 1 {
		t.Fatalf("expected batchConflictIdx=1, got %d", app.importer.batchConflictIdx)
	}

	// Press 'n' on second conflict → skip, triggers batch import
	_, cmd := app.Update(keyRune('n'))
	if cmd == nil {
		t.Fatal("expected batch import command after resolving all conflicts")
	}
	if app.importer.batchOverwrite[src2] {
		t.Fatal("batch-2 should not be marked for overwrite")
	}
}

func TestBatchConflictAllOverwrite(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "all-ow-1")
	src2 := filepath.Join(t.TempDir(), "all-ow-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("new"), 0o644)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "all-ow-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "all-ow-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "all-ow-1")

	// y, y
	m, _ := app.Update(keyRune('y'))
	app = m.(App)
	_, cmd := app.Update(keyRune('y'))

	if !app.importer.batchOverwrite[src1] || !app.importer.batchOverwrite[src2] {
		t.Fatal("expected both paths marked for overwrite")
	}
	if cmd == nil {
		t.Fatal("expected batch import command")
	}
}

func TestBatchConflictAllSkip(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "skip-1")
	src2 := filepath.Join(t.TempDir(), "skip-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "skip-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "skip-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "skip-1")

	// n, n (Esc also works for skip)
	m, _ := app.Update(keyRune('n'))
	app = m.(App)
	app.Update(keyEsc) // Esc also skips in batch

	if len(app.importer.batchOverwrite) != 0 {
		t.Fatalf("expected empty batchOverwrite, got %v", app.importer.batchOverwrite)
	}
}

// ---------------------------------------------------------------------------
// View rendering tests
// ---------------------------------------------------------------------------

func TestConflictViewDiff(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		existingPath: "/dest/test",
		sourcePath:   "/src/test",
		itemName:     "test",
		onlyExisting: []string{"old-file.md"},
		onlyNew:      []string{"new-file.md"},
		inBoth:       []string{"shared.md"},
		diffText:     "--- a/old-file.md\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-old content\n\n--- /dev/null\n+++ b/new-file.md\n@@ -0,0 +1,1 @@\n+new content\n\n--- a/shared.md\n+++ b/shared.md\n@@ -1,1 +1,1 @@\n-old shared\n+new shared",
	}

	view := app.importer.viewConflict()

	assertContains(t, view, "Destination already exists")
	assertContains(t, view, "old-file.md")
	assertContains(t, view, "new-file.md")
	assertContains(t, view, "shared.md")
	assertContains(t, view, "-old content")
	assertContains(t, view, "+new content")
	assertContains(t, view, "y overwrite")
	assertContains(t, view, "1 removed")
	assertContains(t, view, "1 added")
	assertContains(t, view, "1 modified")
}

func TestConflictViewBatchHeader(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.batchConflicts = []string{"/a", "/b", "/c"}
	app.importer.batchConflictIdx = 1
	app.importer.conflict = conflictInfo{
		existingPath: "/dest",
		sourcePath:   "/src",
	}

	label := app.importer.stepLabel()
	if !strings.Contains(label, "2 of 3") {
		t.Fatalf("expected step label to contain '2 of 3', got %q", label)
	}
}

func TestConflictDiffComputation(t *testing.T) {
	existing := t.TempDir()
	source := t.TempDir()

	os.WriteFile(filepath.Join(existing, "keep.md"), []byte("line one.\nline two."), 0o644)
	os.WriteFile(filepath.Join(source, "keep.md"), []byte("line one\nline two."), 0o644) // removed period

	m := importModel{}
	ci := m.buildConflictInfo(existing, source, "diff-test")

	if ci.diffText == "" {
		t.Fatal("expected non-empty diffText for files with differences")
	}
	if !strings.Contains(ci.diffText, "-line one.") {
		t.Error("diffText should contain removed line with period")
	}
	if !strings.Contains(ci.diffText, "+line one") {
		t.Error("diffText should contain added line without period")
	}
	assertContains(t, ci.diffText, "--- a/keep.md")
	assertContains(t, ci.diffText, "+++ b/keep.md")
}
