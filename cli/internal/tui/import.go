package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/gitutil"
	"github.com/OpenScribbler/nesco/cli/internal/installer"
	"github.com/OpenScribbler/nesco/cli/internal/metadata"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
	"github.com/OpenScribbler/nesco/cli/internal/readme"
)

type importStep int

const (
	stepSource      importStep = iota // pick Local Path / Git URL / Create New
	stepType                          // (local/create) pick content type
	stepProvider                      // (local + provider-specific only) pick provider
	stepBrowseStart                   // (local only) pick starting directory
	stepBrowse                        // (local only) navigate filesystem
	stepValidate                      // (local only) review selections before import
	stepPath                          // (local only) custom path text input for browser start
	stepGitURL                        // (git only) enter git URL
	stepGitPick                       // (git only) pick item from scanned clone
	stepConfirm                       // review and execute
	stepName                          // (create only) enter item name
	stepConflict                      // conflict resolution (overwrite/skip)
)

type importCloneDoneMsg struct {
	err  error
	path string // temp dir with cloned repo
}

type importDoneMsg struct {
	name     string
	err      error
	warnings []string
}

type validationItem struct {
	path        string
	name        string
	detection   string
	description string
	isWarning   bool
	included    bool
}

// conflictInfo holds file comparison data for a single conflicting item.
type conflictInfo struct {
	existingPath string   // absolute path to existing destination
	sourcePath   string   // absolute path to source being imported
	itemName     string   // name of the conflicting item
	onlyExisting []string // relative paths only in existing (will be removed)
	onlyNew      []string // relative paths only in new (will be added)
	inBoth       []string // relative paths in both (may differ)
	diffText     string   // precomputed unified diff for all files
	scrollOffset int      // vertical scroll position
	hOffset      int      // horizontal scroll position
}

type importModel struct {
	repoRoot  string
	providers []provider.Provider
	step      importStep

	// Source picker (0=local, 1=git, 2=create)
	sourceCursor int

	// Type picker
	types      []catalog.ContentType
	typeCursor int

	// Provider picker (for provider-specific types)
	providerNames []string
	provCursor    int

	// Text inputs
	pathInput textinput.Model // local path
	urlInput  textinput.Model // git URL
	nameInput textinput.Model // create new: item name

	// Git clone results
	clonedItems []catalog.ContentItem
	clonedPath  string // temp dir to clean up
	pickCursor  int

	// File browser (local path flow)
	browser       fileBrowserModel
	browseCursor  int      // cursor for stepBrowseStart (0=cwd, 1=home, 2=custom)
	selectedPaths []string // paths selected for batch import

	// Validation results
	validationItems []validationItem
	validateCursor  int

	// Resolved import target
	sourcePath   string
	contentType  catalog.ContentType
	providerName string
	itemName     string
	isCreate     bool // true if using "Create New" flow

	// Conflict resolution
	conflict         conflictInfo    // current conflict being shown
	batchConflicts   []string        // source paths that have conflicts
	batchConflictIdx int             // index into batchConflicts
	batchOverwrite   map[string]bool // srcPath → true means overwrite

	// Result messaging
	message      string
	messageIsErr bool

	width, height int
}

func newImportModel(providers []provider.Provider, repoRoot string) importModel {
	pi := textinput.New()
	pi.Prompt = labelStyle.Render("Path: ")
	pi.Placeholder = "/path/to/content"
	pi.CharLimit = 500

	ui := textinput.New()
	ui.Prompt = labelStyle.Render("URL: ")
	ui.Placeholder = "https://github.com/user/repo.git"
	ui.CharLimit = 500

	ni := textinput.New()
	ni.Prompt = labelStyle.Render("Name: ")
	ni.Placeholder = "my-new-tool"
	ni.CharLimit = 100

	return importModel{
		repoRoot:  repoRoot,
		providers: providers,
		types:     catalog.AllContentTypes(),
		pathInput: pi,
		urlInput:  ui,
		nameInput: ni,
	}
}

func (m importModel) Update(msg tea.Msg) (importModel, tea.Cmd) {
	switch msg := msg.(type) {
	case fileBrowserDoneMsg:
		m.selectedPaths = msg.paths
		m.validationItems = m.validateSelections(msg.paths)
		m.validateCursor = 0
		m.step = stepValidate
		return m, nil

	case importCloneDoneMsg:
		if msg.err != nil {
			os.RemoveAll(msg.path) // clean up failed clone
			m.message = fmt.Sprintf("Clone failed: %s", msg.err)
			m.messageIsErr = true
			m.step = stepGitURL
			m.urlInput.Focus()
			return m, nil
		}
		m.clonedPath = msg.path
		// Scan the cloned repo for content (cloned repo is self-contained; both roots are the same)
		cat, err := catalog.Scan(msg.path, msg.path)
		if err != nil {
			m.cleanup()
			m.message = fmt.Sprintf("Scan failed: %s", err)
			m.messageIsErr = true
			m.step = stepGitURL
			m.urlInput.Focus()
			return m, nil
		}
		if len(cat.Items) == 0 {
			m.cleanup()
			m.message = "No content found in cloned repository"
			m.messageIsErr = true
			m.step = stepGitURL
			m.urlInput.Focus()
			return m, nil
		}
		m.clonedItems = cat.Items
		m.pickCursor = 0
		m.step = stepGitPick
		m.message = ""
		return m, nil

	case tea.MouseMsg:
		if m.step == stepBrowse {
			m.browser, _ = m.browser.Update(msg)
		}
		if m.step == stepConflict {
			if msg.Button == tea.MouseButtonWheelUp && m.conflict.scrollOffset > 0 {
				m.conflict.scrollOffset--
			}
			if msg.Button == tea.MouseButtonWheelDown {
				m.conflict.scrollOffset++
			}
		}
		// Handle left-clicks on zone-marked list items
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			return m.handleMouseClick(msg)
		}
		return m, nil

	case tea.KeyMsg:
		// Clear any previous message on keypress
		if msg.Type != tea.KeyEsc {
			m.message = ""
			m.messageIsErr = false
		}

		switch m.step {
		case stepSource:
			return m.updateSource(msg)
		case stepType:
			return m.updateType(msg)
		case stepProvider:
			return m.updateProvider(msg)
		case stepBrowseStart:
			return m.updateBrowseStart(msg)
		case stepBrowse:
			return m.updateBrowse(msg)
		case stepValidate:
			return m.updateValidate(msg)
		case stepPath:
			return m.updatePath(msg)
		case stepGitURL:
			return m.updateGitURL(msg)
		case stepGitPick:
			return m.updateGitPick(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		case stepName:
			return m.updateName(msg)
		case stepConflict:
			return m.updateConflict(msg)
		}
	}
	return m, nil
}

func (m importModel) updateSource(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.sourceCursor < 2 {
			m.sourceCursor++
		}
	case key.Matches(msg, keys.Enter):
		switch m.sourceCursor {
		case 0: // Local path
			m.isCreate = false
			m.step = stepType
			m.typeCursor = 0
		case 1: // Git URL
			m.isCreate = false
			m.step = stepGitURL
			m.urlInput.SetValue("")
			m.urlInput.Focus()
		case 2: // Create New
			m.isCreate = true
			m.step = stepType
			m.typeCursor = 0
		}
	}
	return m, nil
}

func (m importModel) updateType(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepSource
	case key.Matches(msg, keys.Up):
		if m.typeCursor > 0 {
			m.typeCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.typeCursor < len(m.types)-1 {
			m.typeCursor++
		}
	case key.Matches(msg, keys.Enter):
		ct := m.types[m.typeCursor]
		m.contentType = ct
		if m.isCreate {
			// Create flow: go to name input (skip provider for all types)
			m.step = stepName
			m.nameInput.SetValue("")
			m.nameInput.Focus()
		} else if ct.IsUniversal() {
			// Universal types skip provider selection
			m.browseCursor = 0
			m.step = stepBrowseStart
		} else {
			// Provider-specific: need to pick provider
			m.providerNames = m.discoverProviderDirs(ct)
			if len(m.providerNames) == 0 {
				m.message = "No provider directories found for " + ct.Label()
				m.messageIsErr = true
			} else {
				m.provCursor = 0
				m.step = stepProvider
			}
		}
	}
	return m, nil
}

func (m importModel) updateProvider(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepType
	case key.Matches(msg, keys.Up):
		if m.provCursor > 0 {
			m.provCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.provCursor < len(m.providerNames)-1 {
			m.provCursor++
		}
	case key.Matches(msg, keys.Enter):
		m.providerName = m.providerNames[m.provCursor]
		m.browseCursor = 0
		m.step = stepBrowseStart
	}
	return m, nil
}

func (m importModel) updateBrowseStart(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		if m.contentType.IsUniversal() {
			m.step = stepType
		} else {
			m.step = stepProvider
		}
	case key.Matches(msg, keys.Up):
		if m.browseCursor > 0 {
			m.browseCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.browseCursor < 2 {
			m.browseCursor++
		}
	case key.Matches(msg, keys.Enter):
		switch m.browseCursor {
		case 0: // Current working directory
			cwdPath, err := os.Getwd()
			if err != nil {
				m.message = "Cannot determine working directory"
				m.messageIsErr = true
				return m, nil
			}
			m.browser = newFileBrowser(cwdPath, m.contentType)
			m.browser.width = m.width
			m.browser.height = m.height
			m.step = stepBrowse
		case 1: // Home directory
			home, err := os.UserHomeDir()
			if err != nil {
				m.message = "Cannot determine home directory"
				m.messageIsErr = true
				return m, nil
			}
			m.browser = newFileBrowser(home, m.contentType)
			m.browser.width = m.width
			m.browser.height = m.height
			m.step = stepBrowse
		case 2: // Custom path
			m.pathInput.SetValue("")
			m.pathInput.Focus()
			m.step = stepPath
		}
	}
	return m, nil
}

func (m importModel) updateBrowse(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		// If previewing a file, exit preview first
		if m.browser.previewing {
			m.browser.previewing = false
			m.browser.previewContent = ""
			m.browser.previewOffset = 0
			return m, nil
		}
		// If selections exist, clear them first
		if len(m.browser.selected) > 0 {
			m.browser.selected = make(map[string]bool)
			return m, nil
		}
		// Navigate to parent directory; exit browser only at filesystem root
		parent := filepath.Dir(m.browser.currentDir)
		if parent != m.browser.currentDir {
			m.browser.loadDir(parent)
			return m, nil
		}
		m.step = stepBrowseStart
		return m, nil
	}
	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

func (m importModel) updatePath(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.pathInput.Blur()
		m.step = stepBrowseStart
		return m, nil
	case msg.Type == tea.KeyEnter:
		path := m.pathInput.Value()
		if path == "" {
			return m, nil
		}
		// Expand ~ prefix
		expanded, err := expandHome(path)
		if err != nil {
			m.message = err.Error()
			m.messageIsErr = true
			return m, nil
		}
		path = expanded
		// Validate source exists
		_, err = os.Stat(path)
		if err != nil {
			m.message = fmt.Sprintf("Path not found: %s", path)
			m.messageIsErr = true
			return m, nil
		}
		m.pathInput.Blur()
		m.browser = newFileBrowser(path, m.contentType)
		m.browser.width = m.width
		m.browser.height = m.height
		m.step = stepBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m importModel) updateValidate(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepBrowse
	case key.Matches(msg, keys.Up):
		if m.validateCursor > 0 {
			m.validateCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.validateCursor < len(m.validationItems)-1 {
			m.validateCursor++
		}
	case key.Matches(msg, keys.Space):
		if m.validateCursor >= 0 && m.validateCursor < len(m.validationItems) {
			m.validationItems[m.validateCursor].included = !m.validationItems[m.validateCursor].included
		}
	case key.Matches(msg, keys.Enter):
		// Collect included paths and proceed to batch import
		var included []string
		for _, vi := range m.validationItems {
			if vi.included {
				included = append(included, vi.path)
			}
		}
		if len(included) == 0 {
			m.message = "No items selected for import"
			m.messageIsErr = true
			return m, nil
		}
		m.selectedPaths = included
		// For single selection, set sourcePath/itemName for existing confirm flow
		if len(included) == 1 {
			m.sourcePath = included[0]
			m.itemName = filepath.Base(included[0])
			m.step = stepConfirm
			return m, nil
		}
		// For batch, check for conflicts before importing
		var conflicts []string
		for _, srcPath := range included {
			dest := m.batchDestForSource(srcPath)
			if _, err := os.Stat(dest); err == nil {
				conflicts = append(conflicts, srcPath)
			}
		}
		if len(conflicts) > 0 {
			m.batchConflicts = conflicts
			m.batchConflictIdx = 0
			m.batchOverwrite = make(map[string]bool)
			// Build conflict info for first conflict
			srcPath := conflicts[0]
			dest := m.batchDestForSource(srcPath)
			itemName := filepath.Base(srcPath)
			m.conflict = m.buildConflictInfo(dest, srcPath, itemName)
			m.step = stepConflict
			return m, nil
		}
		// No conflicts — go directly to import
		return m, func() tea.Msg {
			name, warnings, err := m.doBatchImportWithOverwrite(included, nil)
			return importDoneMsg{name: name, warnings: warnings, err: err}
		}
	}
	return m, nil
}

func (m importModel) updateGitURL(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.urlInput.Blur()
		m.step = stepSource
		m.message = ""
		return m, nil
	case msg.Type == tea.KeyEnter:
		url := m.urlInput.Value()
		if url == "" {
			return m, nil
		}
		if !isValidGitURL(url) {
			m.message = "Invalid URL. Must start with https://, http://, git://, ssh://, or git@"
			m.messageIsErr = true
			return m, nil
		}
		m.urlInput.Blur()
		m.message = "Cloning repository..."
		m.messageIsErr = false
		return m, m.startClone(url)
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m importModel) updateGitPick(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.step = stepGitURL
		m.urlInput.Focus()
		m.clonedItems = nil
		m.cleanup()
	case key.Matches(msg, keys.Up):
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.pickCursor < len(m.clonedItems)-1 {
			m.pickCursor++
		}
	case key.Matches(msg, keys.Enter):
		item := m.clonedItems[m.pickCursor]
		m.contentType = item.Type
		m.providerName = item.Provider
		m.sourcePath = item.Path
		m.itemName = item.Name
		m.step = stepConfirm
	}
	return m, nil
}

func (m importModel) updateConfirm(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		if m.isCreate {
			m.step = stepName
			m.nameInput.Focus()
		} else if m.clonedPath != "" {
			m.step = stepGitPick
		} else {
			// Came from browse flow
			m.step = stepValidate
		}
	case key.Matches(msg, keys.Enter):
		// Check if destination already exists (conflict detection)
		dest := m.destinationPath()
		if _, err := os.Stat(dest); err == nil {
			m.conflict = m.buildConflictInfo(dest, m.sourcePath, m.itemName)
			m.batchConflicts = nil
			m.batchOverwrite = nil
			m.step = stepConflict
			return m, nil
		}
		if m.isCreate {
			return m, func() tea.Msg {
				name, warnings, err := m.doScaffold()
				return importDoneMsg{name: name, warnings: warnings, err: err}
			}
		}
		// Run the copy in the Cmd closure so it doesn't block the UI.
		// m is a value copy, so doImport reads from an immutable snapshot.
		return m, func() tea.Msg {
			name, warnings, err := m.doImport()
			return importDoneMsg{name: name, warnings: warnings, err: err}
		}
	}
	return m, nil
}

// stepLabel returns a human-readable step indicator for the import flow.
func (m importModel) stepLabel() string {
	switch m.step {
	case stepSource:
		return "Step 1 of 4: Source"
	case stepType:
		return "Step 2 of 4: Content Type"
	case stepProvider:
		return "Step 2b of 4: Provider"
	case stepBrowseStart, stepBrowse, stepPath:
		return "Step 3 of 4: Browse"
	case stepValidate:
		return "Step 3b of 4: Review"
	case stepGitURL:
		return "Step 2 of 3: Repository URL"
	case stepGitPick:
		return "Step 3 of 3: Select Item"
	case stepName:
		return "Step 2 of 3: Name"
	case stepConfirm:
		return "Confirm"
	case stepConflict:
		if len(m.batchConflicts) > 0 {
			return fmt.Sprintf("Conflict %d of %d", m.batchConflictIdx+1, len(m.batchConflicts))
		}
		return "Conflict"
	}
	return ""
}

// View renders the current step's UI.
func (m importModel) View() string {
	s := zone.Mark("crumb-home", helpStyle.Render("Home")) + " " + helpStyle.Render(">") + " " + titleStyle.Render("Import AI Tools") + "\n"
	if label := m.stepLabel(); label != "" {
		if m.step == stepConflict {
			s += "\n" + warningStyle.Render(label) + "\n"
		} else {
			s += helpStyle.Render(label) + "\n"
		}
	}

	switch m.step {
	case stepSource:
		s += "\n" + helpStyle.Render("Bring in skills, agents, prompts, rules, hooks, commands, and MCP configs") + "\n"
		s += helpStyle.Render("from your filesystem or a git repository. Create New scaffolds a blank template.") + "\n\n"
		options := []string{"Local Path", "Git URL", "Create New"}
		for i, opt := range options {
			prefix := "   "
			style := itemStyle
			if i == m.sourceCursor {
				prefix = " > "
				style = selectedItemStyle
			}
			row := prefix + style.Render(opt)
			s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
		}
		s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")

	case stepType:
		s += helpStyle.Render("Select content type") + "\n\n"
		for i, ct := range m.types {
			prefix := "   "
			style := itemStyle
			if i == m.typeCursor {
				prefix = " > "
				style = selectedItemStyle
			}
			label := ct.Label()
			row := prefix + style.Render(label)
			s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
		}
		s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")

	case stepProvider:
		s += helpStyle.Render("Select provider for "+m.contentType.Label()) + "\n\n"
		for i, name := range m.providerNames {
			prefix := "   "
			style := itemStyle
			if i == m.provCursor {
				prefix = " > "
				style = selectedItemStyle
			}
			row := prefix + style.Render(name)
			s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
		}
		s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")

	case stepBrowseStart:
		s += helpStyle.Render("Where do you want to browse?") + "\n\n"
		options := []struct{ label, desc string }{
			{"Current directory", cwd()},
			{"Home directory", homeDir()},
			{"Custom path...", "Enter a path to start from"},
		}
		for i, opt := range options {
			prefix := "   "
			style := itemStyle
			if i == m.browseCursor {
				prefix = " > "
				style = selectedItemStyle
			}
			row := prefix + style.Render(opt.label) + " " + countStyle.Render(opt.desc)
			s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
		}
		s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")

	case stepBrowse:
		s += m.browser.View()

	case stepPath:
		s += helpStyle.Render("Enter starting path for browser") + "\n\n"
		s += m.pathInput.View() + "\n"
		s += "\n" + helpStyle.Render("enter open browser • esc back")

	case stepValidate:
		s += m.viewValidate()

	case stepGitURL:
		s += helpStyle.Render("Enter git repository URL") + "\n\n"
		s += m.urlInput.View() + "\n"
		s += "\n" + helpStyle.Render("enter clone • esc back")

	case stepGitPick:
		s += helpStyle.Render("Select item to import") + "\n\n"

		// Calculate visible window for scrolling
		visibleRows := m.height - 6 // header + subtitle + blank + footer
		if visibleRows < 1 {
			visibleRows = len(m.clonedItems)
		}
		offset := 0
		if m.pickCursor >= visibleRows {
			offset = m.pickCursor - visibleRows + 1
		}
		end := offset + visibleRows
		if end > len(m.clonedItems) {
			end = len(m.clonedItems)
		}

		if offset > 0 {
			s += helpStyle.Render("  (more items above)") + "\n"
		}
		for i := offset; i < end; i++ {
			item := m.clonedItems[i]
			prefix := "   "
			style := itemStyle
			if i == m.pickCursor {
				prefix = " > "
				style = selectedItemStyle
			}
			label := item.Name
			typeTag := countStyle.Render("(" + item.Type.Label() + ")")
			if item.Provider != "" {
				typeTag = countStyle.Render("(" + item.Type.Label() + "/" + item.Provider + ")")
			}
			row := prefix + style.Render(label) + " " + typeTag
			s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
		}
		if end < len(m.clonedItems) {
			s += helpStyle.Render("  (more items below)") + "\n"
		}
		s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")

	case stepName:
		s += helpStyle.Render("Enter a name for your new "+m.contentType.Label()+" item") + "\n\n"
		s += m.nameInput.View() + "\n"
		s += "\n" + helpStyle.Render("enter confirm • esc back")

	case stepConfirm:
		if m.isCreate {
			s += helpStyle.Render("Confirm creation") + "\n\n"
			dest := m.destinationPath()
			s += labelStyle.Render("Item:  ") + valueStyle.Render(m.itemName) + "\n"
			s += labelStyle.Render("Type:  ") + valueStyle.Render(m.contentType.Label()) + "\n"
			s += labelStyle.Render("To:    ") + valueStyle.Render(dest) + " " + helpStyle.Render("(local, not git-tracked)") + "\n"
			s += "\n" + helpStyle.Render("Scaffolds from template with LLM prompt for content creation.")
			s += "\n\n" + helpStyle.Render("enter create • esc back")
		} else {
			s += helpStyle.Render("Confirm import") + "\n\n"
			dest := m.destinationPath()
			s += labelStyle.Render("Item:  ") + valueStyle.Render(m.itemName) + "\n"
			s += labelStyle.Render("Type:  ") + valueStyle.Render(m.contentType.Label()) + "\n"
			if m.providerName != "" {
				s += labelStyle.Render("Provider: ") + valueStyle.Render(m.providerName) + "\n"
			}
			s += labelStyle.Render("From:  ") + valueStyle.Render(m.sourcePath) + "\n"
			s += labelStyle.Render("To:    ") + valueStyle.Render(dest) + " " + helpStyle.Render("(local, not git-tracked)") + "\n"
			s += "\n" + helpStyle.Render("enter import • esc back")
		}

	case stepConflict:
		s += m.viewConflict()
	}

	// Status message
	if m.message != "" {
		if m.messageIsErr {
			s += "\n" + errorMsgStyle.Render("Error: "+m.message)
		} else {
			s += "\n" + successMsgStyle.Render("Done: "+m.message)
		}
	}

	return s
}

// destinationPath computes where the content will be copied to (always local/).
func (m importModel) destinationPath() string {
	if m.contentType.IsUniversal() {
		return filepath.Join(m.repoRoot, "local", string(m.contentType), m.itemName)
	}
	return filepath.Join(m.repoRoot, "local", string(m.contentType), m.providerName, m.itemName)
}

// doImport executes the copy, generates metadata, and returns the item name.
func (m importModel) doImport() (string, []string, error) {
	dest := m.destinationPath()
	var warnings []string

	// When importing a single file for a universal type, wrap it in a
	// directory so metadata and README can coexist alongside the content.
	srcInfo, err := os.Stat(m.sourcePath)
	if err != nil {
		return "", nil, fmt.Errorf("stat source: %w", err)
	}
	if !srcInfo.IsDir() && m.contentType.IsUniversal() {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return "", nil, fmt.Errorf("creating directory: %w", err)
		}
		fileDest := filepath.Join(dest, filepath.Base(m.sourcePath))
		if err := installer.CopyContent(m.sourcePath, fileDest); err != nil {
			return "", nil, fmt.Errorf("copy failed: %w", err)
		}
	} else {
		if err := installer.CopyContent(m.sourcePath, dest); err != nil {
			return "", nil, fmt.Errorf("copy failed: %w", err)
		}
	}

	// Generate .nesco.yaml metadata
	now := time.Now()
	source := m.sourcePath
	if m.clonedPath != "" {
		source = m.urlInput.Value()
	}
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       m.itemName,
		Type:       string(m.contentType),
		Source:     source,
		ImportedAt: &now,
		ImportedBy: gitutil.Username(),
	}
	// For universal types, save in the item directory.
	// For provider-specific types, save as provider-specific metadata.
	if m.contentType.IsUniversal() {
		if err := metadata.Save(dest, meta); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to save metadata for %s: %s", m.itemName, err))
		}
	} else {
		destDir := filepath.Dir(dest)
		if err := metadata.SaveProvider(destDir, m.itemName, meta); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to save metadata for %s: %s", m.itemName, err))
		}
	}

	// Generate placeholder README if the source didn't include one
	if created, _ := readme.EnsureReadme(dest, m.itemName, string(m.contentType), ""); created {
		warnings = append(warnings, fmt.Sprintf("Generated placeholder README.md for %s — an LLM can improve it", m.itemName))
	}

	return m.itemName, warnings, nil
}

func (m importModel) updateName(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.nameInput.Blur()
		m.step = stepType
		return m, nil
	case msg.Type == tea.KeyEnter:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return m, nil
		}
		m.itemName = name
		m.nameInput.Blur()
		m.step = stepConfirm
		return m, nil
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// doScaffold creates a new content item from templates.
func (m importModel) doScaffold() (string, []string, error) {
	dest := m.destinationPath()
	var warnings []string

	// Find the templates directory relative to repo root
	templateDir := filepath.Join(m.repoRoot, "templates", string(m.contentType))

	// Copy template files (if template dir exists)
	if _, err := os.Stat(templateDir); err == nil {
		if err := installer.CopyContent(templateDir, dest); err != nil {
			return "", nil, fmt.Errorf("scaffold failed: %w", err)
		}
	} else {
		// No template — just create the directory
		if err := os.MkdirAll(dest, 0755); err != nil {
			return "", nil, fmt.Errorf("creating directory: %w", err)
		}
	}

	// Replace {{NAME}} placeholder in LLM-PROMPT.md if it exists
	llmPath := filepath.Join(dest, "LLM-PROMPT.md")
	if data, err := os.ReadFile(llmPath); err == nil {
		replaced := strings.ReplaceAll(string(data), "{{NAME}}", m.itemName)
		os.WriteFile(llmPath, []byte(replaced), 0644) // non-fatal
	}

	// Generate .nesco.yaml
	now := time.Now()
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       m.itemName,
		Type:       string(m.contentType),
		Source:     "created",
		ImportedAt: &now,
		ImportedBy: gitutil.Username(),
	}
	if err := metadata.Save(dest, meta); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to save metadata for %s: %s", m.itemName, err))
	}

	return m.itemName, warnings, nil
}

// startClone creates a temp dir and returns a tea.ExecProcess command for git clone.
func (m importModel) startClone(url string) tea.Cmd {
	tmpDir, err := os.MkdirTemp("", "nesco-import-*")
	if err != nil {
		return func() tea.Msg {
			return importCloneDoneMsg{err: fmt.Errorf("creating temp dir: %w", err)}
		}
	}

	cmd := exec.Command("git", "clone", "--depth", "1", url, tmpDir)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return importCloneDoneMsg{err: err, path: tmpDir}
	})
}

// cleanup removes the cloned temp directory if it exists.
func (m *importModel) cleanup() {
	if m.clonedPath != "" {
		os.RemoveAll(m.clonedPath)
		m.clonedPath = ""
	}
}

// validateSelections checks each selected path and returns validation results.
func (m importModel) validateSelections(paths []string) []validationItem {
	var items []validationItem
	for _, p := range paths {
		name := filepath.Base(p)
		det, ok := catalog.DetectContent(p)

		vi := validationItem{
			path:      p,
			name:      name,
			detection: det,
			isWarning: !ok,
			included:  true,
		}

		// Try to extract description from frontmatter
		info, err := os.Stat(p)
		if err == nil && info.IsDir() {
			for _, marker := range []string{"SKILL.md", "AGENT.md", "PROMPT.md"} {
				data, err := os.ReadFile(filepath.Join(p, marker))
				if err == nil {
					fm, fmErr := catalog.ParseFrontmatter(data)
					if fmErr == nil && fm.Description != "" {
						vi.description = fm.Description
					}
					break
				}
			}
		}

		items = append(items, vi)
	}
	return items
}

// doBatchImport imports multiple selected paths, generating metadata for each.
// Returns a comma-separated list of imported names.
func (m importModel) doBatchImport(paths []string) (string, []string, error) {
	return m.doBatchImportWithOverwrite(paths, nil)
}

// doBatchImportWithOverwrite imports multiple paths, with an optional overwrite map.
// Paths in the overwrite map are removed before importing. Paths not in the map
// that already exist are skipped.
func (m importModel) doBatchImportWithOverwrite(paths []string, overwrite map[string]bool) (string, []string, error) {
	var imported []string
	var errs []string
	var warnings []string

	for _, srcPath := range paths {
		itemName := filepath.Base(srcPath)
		dest := m.batchDestForSource(srcPath)

		// Check if destination exists
		if _, err := os.Stat(dest); err == nil {
			if overwrite[srcPath] {
				// Overwrite: remove existing destination
				if err := os.RemoveAll(dest); err != nil {
					errs = append(errs, fmt.Sprintf("%s (remove failed: %s)", itemName, err))
					continue
				}
			} else {
				// Skip non-overwritten conflicts
				errs = append(errs, fmt.Sprintf("%s (already exists)", itemName))
				continue
			}
		}

		// When importing a single file for a universal type, wrap it in a
		// directory so metadata and README can coexist alongside the content.
		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s (%s)", itemName, err))
			continue
		}
		if !srcInfo.IsDir() && m.contentType.IsUniversal() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				errs = append(errs, fmt.Sprintf("%s (%s)", itemName, err))
				continue
			}
			fileDest := filepath.Join(dest, filepath.Base(srcPath))
			if err := installer.CopyContent(srcPath, fileDest); err != nil {
				errs = append(errs, fmt.Sprintf("%s (%s)", itemName, err))
				continue
			}
		} else {
			if err := installer.CopyContent(srcPath, dest); err != nil {
				errs = append(errs, fmt.Sprintf("%s (%s)", itemName, err))
				continue
			}
		}

		// Generate metadata
		now := time.Now()
		meta := &metadata.Meta{
			ID:         metadata.NewID(),
			Name:       itemName,
			Type:       string(m.contentType),
			Source:     srcPath,
			ImportedAt: &now,
			ImportedBy: gitutil.Username(),
		}
		if m.contentType.IsUniversal() {
			if err := metadata.Save(dest, meta); err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to save metadata for %s: %s", itemName, err))
			}
		} else {
			if err := metadata.SaveProvider(filepath.Dir(dest), itemName, meta); err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to save metadata for %s: %s", itemName, err))
			}
		}

		// Generate placeholder README if source didn't include one
		if _, err := readme.EnsureReadme(dest, itemName, string(m.contentType), ""); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to generate README for %s: %s", itemName, err))
		}

		imported = append(imported, itemName)
	}

	result := strings.Join(imported, ", ")
	if len(errs) > 0 {
		if len(imported) == 0 {
			return "", nil, fmt.Errorf("all imports failed: %s", strings.Join(errs, "; "))
		}
		result += fmt.Sprintf(" (skipped: %s)", strings.Join(errs, "; "))
	}
	return result, warnings, nil
}

func (m importModel) updateConflict(msg tea.KeyMsg) (importModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.conflict.scrollOffset > 0 {
			m.conflict.scrollOffset--
		}
	case key.Matches(msg, keys.Down):
		m.conflict.scrollOffset++
	case key.Matches(msg, keys.PageUp):
		pageSize := m.height - 10
		if pageSize < 1 {
			pageSize = 10
		}
		m.conflict.scrollOffset -= pageSize
		if m.conflict.scrollOffset < 0 {
			m.conflict.scrollOffset = 0
		}
	case key.Matches(msg, keys.PageDown):
		pageSize := m.height - 10
		if pageSize < 1 {
			pageSize = 10
		}
		m.conflict.scrollOffset += pageSize
	case key.Matches(msg, keys.Left):
		if m.conflict.hOffset > 0 {
			m.conflict.hOffset -= 8
			if m.conflict.hOffset < 0 {
				m.conflict.hOffset = 0
			}
		}
	case key.Matches(msg, keys.Right):
		m.conflict.hOffset += 8
	case msg.Type == tea.KeyRunes && string(msg.Runes) == "y":
		// Overwrite
		if len(m.batchConflicts) > 0 {
			m.batchOverwrite[m.batchConflicts[m.batchConflictIdx]] = true
			return m.advanceConflict()
		}
		return m, func() tea.Msg {
			name, warnings, err := m.doImportOverwrite()
			return importDoneMsg{name: name, warnings: warnings, err: err}
		}
	case msg.Type == tea.KeyRunes && string(msg.Runes) == "n":
		if len(m.batchConflicts) > 0 {
			return m.advanceConflict()
		}
	case msg.Type == tea.KeyEsc:
		if len(m.batchConflicts) > 0 {
			return m.advanceConflict()
		}
		m.step = stepConfirm
	}
	return m, nil
}

func (m importModel) viewConflict() string {
	s := warningStyle.Render("Destination already exists!") + "\n"
	s += labelStyle.Render("Path: ") + valueStyle.Render(m.conflict.existingPath) + "\n"

	// Summary line
	var parts []string
	if n := len(m.conflict.onlyExisting); n > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", n))
	}
	if n := len(m.conflict.onlyNew); n > 0 {
		parts = append(parts, fmt.Sprintf("%d added", n))
	}
	if n := len(m.conflict.inBoth); n > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", n))
	}
	if len(parts) > 0 {
		s += helpStyle.Render(strings.Join(parts, ", ")) + "\n"
	}
	s += "\n"

	if m.conflict.diffText == "" {
		s += helpStyle.Render("(no differences found)") + "\n"
	} else {
		lines := strings.Split(m.conflict.diffText, "\n")
		visibleHeight := m.height - 10
		if visibleHeight < 5 {
			visibleHeight = len(lines)
		}

		// Clamp scroll
		maxOffset := len(lines) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.conflict.scrollOffset
		if offset > maxOffset {
			offset = maxOffset
		}

		end := offset + visibleHeight
		if end > len(lines) {
			end = len(lines)
		}

		if offset > 0 {
			s += helpStyle.Render(fmt.Sprintf("(%d lines above)", offset)) + "\n"
		}

		hOff := m.conflict.hOffset
		for i := offset; i < end; i++ {
			s += "  " + renderDiffLine(lines[i], hOff) + "\n"
		}

		if end < len(lines) {
			s += helpStyle.Render(fmt.Sprintf("(%d lines below)", len(lines)-end)) + "\n"
		}
	}

	// Footer
	s += "\n"
	footer := "↑↓ scroll • "
	if m.conflict.hOffset > 0 {
		footer += fmt.Sprintf("←→ scroll (col %d) • ", m.conflict.hOffset)
	} else {
		footer += "←→ scroll • "
	}
	if len(m.batchConflicts) > 0 {
		footer += "y overwrite • n skip"
	} else {
		footer += "y overwrite • esc cancel"
	}
	s += helpStyle.Render(footer)

	return s
}

// renderDiffLine applies unified-diff syntax coloring and horizontal offset.
func renderDiffLine(line string, hOff int) string {
	// Apply horizontal offset to display text
	display := line
	runes := []rune(display)
	if hOff > 0 && hOff < len(runes) {
		display = string(runes[hOff:])
	} else if hOff >= len(runes) {
		display = ""
	}

	// Color based on original line prefix (coloring is always correct even when scrolled)
	switch {
	case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
		return labelStyle.Render(display)
	case strings.HasPrefix(line, "@@"):
		return helpStyle.Render(display)
	case strings.HasPrefix(line, "+"):
		return installedStyle.Render(display)
	case strings.HasPrefix(line, "-"):
		return errorMsgStyle.Render(display)
	default:
		return valueStyle.Render(display)
	}
}

// collectRelativeFiles walks a directory and returns sorted relative file paths.
// Skips symlinks and .nesco.yaml (metadata noise).
func collectRelativeFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // skip symlinks
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == ".nesco.yaml" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files
}

// buildConflictInfo creates a conflictInfo comparing the existing destination
// with the source being imported.
func (m importModel) buildConflictInfo(dest, source, name string) conflictInfo {
	ci := conflictInfo{
		existingPath: dest,
		sourcePath:   source,
		itemName:     name,
	}

	existingFiles := collectRelativeFiles(dest)

	var newFiles []string
	if m.isCreate {
		// Create flow: source files are empty (all existing → "will be removed")
		newFiles = nil
	} else {
		// Check if source is a single file (universal type wrapping)
		srcInfo, err := os.Stat(source)
		if err == nil && !srcInfo.IsDir() {
			// Single file will be wrapped in a directory
			newFiles = []string{filepath.Base(source)}
		} else {
			newFiles = collectRelativeFiles(source)
		}
	}

	// Classify files
	existingSet := make(map[string]bool, len(existingFiles))
	for _, f := range existingFiles {
		existingSet[f] = true
	}
	newSet := make(map[string]bool, len(newFiles))
	for _, f := range newFiles {
		newSet[f] = true
	}

	for _, f := range existingFiles {
		if newSet[f] {
			ci.inBoth = append(ci.inBoth, f)
		} else {
			ci.onlyExisting = append(ci.onlyExisting, f)
		}
	}
	for _, f := range newFiles {
		if !existingSet[f] {
			ci.onlyNew = append(ci.onlyNew, f)
		}
	}

	ci.diffText = computeDiffText(ci)
	return ci
}

// sourceFilePath returns the full path to a source file given its relative path.
// Handles the single-file wrapping case where sourcePath is a file, not a directory.
func (ci conflictInfo) sourceFilePath(relPath string) string {
	info, err := os.Stat(ci.sourcePath)
	if err == nil && !info.IsDir() {
		return ci.sourcePath
	}
	return filepath.Join(ci.sourcePath, relPath)
}

// computeDiffText generates a unified diff for all changed files in the conflict.
func computeDiffText(ci conflictInfo) string {
	var buf strings.Builder

	// Files only in existing (will be removed)
	for _, f := range ci.onlyExisting {
		data, err := os.ReadFile(filepath.Join(ci.existingPath, f))
		if err != nil {
			continue
		}
		buf.WriteString(fmt.Sprintf("--- a/%s\n", f))
		buf.WriteString("+++ /dev/null\n")
		content := strings.TrimRight(string(data), "\n")
		if content == "" {
			buf.WriteString("@@ -0,0 +0,0 @@\n")
		} else {
			lines := strings.Split(content, "\n")
			buf.WriteString(fmt.Sprintf("@@ -1,%d +0,0 @@\n", len(lines)))
			for _, l := range lines {
				buf.WriteString("-" + l + "\n")
			}
		}
		buf.WriteString("\n")
	}

	// Files only in new (will be added)
	for _, f := range ci.onlyNew {
		filePath := ci.sourceFilePath(f)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		buf.WriteString("--- /dev/null\n")
		buf.WriteString(fmt.Sprintf("+++ b/%s\n", f))
		content := strings.TrimRight(string(data), "\n")
		if content == "" {
			buf.WriteString("@@ -0,0 +0,0 @@\n")
		} else {
			lines := strings.Split(content, "\n")
			buf.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
			for _, l := range lines {
				buf.WriteString("+" + l + "\n")
			}
		}
		buf.WriteString("\n")
	}

	// Files in both — run diff -u to get unified diff
	for _, f := range ci.inBoth {
		existFile := filepath.Join(ci.existingPath, f)
		newFile := ci.sourceFilePath(f)
		cmd := exec.Command("diff", "-u", existFile, newFile)
		out, _ := cmd.Output()
		if len(out) == 0 {
			continue // identical files — skip
		}
		// Replace header lines with clean relative paths
		diffStr := string(out)
		diffLines := strings.SplitN(diffStr, "\n", 3)
		if len(diffLines) >= 3 {
			diffLines[0] = "--- a/" + f
			diffLines[1] = "+++ b/" + f
			diffStr = strings.Join(diffLines, "\n")
		}
		buf.WriteString(diffStr)
		if !strings.HasSuffix(diffStr, "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return strings.TrimRight(buf.String(), "\n")
}

// doImportOverwrite removes the destination then delegates to doImport or doScaffold.
func (m importModel) doImportOverwrite() (string, []string, error) {
	dest := m.destinationPath()
	if err := os.RemoveAll(dest); err != nil {
		return "", nil, fmt.Errorf("removing existing: %w", err)
	}
	if m.isCreate {
		return m.doScaffold()
	}
	return m.doImport()
}

// batchDestForSource computes the destination path for a batch source path.
func (m importModel) batchDestForSource(srcPath string) string {
	itemName := filepath.Base(srcPath)
	if m.contentType.IsUniversal() {
		return filepath.Join(m.repoRoot, "local", string(m.contentType), itemName)
	}
	return filepath.Join(m.repoRoot, "local", string(m.contentType), m.providerName, itemName)
}

// advanceConflict moves to the next batch conflict, or launches the batch import
// if all conflicts have been resolved.
func (m importModel) advanceConflict() (importModel, tea.Cmd) {
	m.batchConflictIdx++
	if m.batchConflictIdx < len(m.batchConflicts) {
		// Build conflict info for the next conflict
		srcPath := m.batchConflicts[m.batchConflictIdx]
		dest := m.batchDestForSource(srcPath)
		itemName := filepath.Base(srcPath)
		m.conflict = m.buildConflictInfo(dest, srcPath, itemName)
		return m, nil
	}
	// All conflicts resolved — launch batch import with overwrite decisions
	overwrite := m.batchOverwrite
	included := m.selectedPaths
	return m, func() tea.Msg {
		name, warnings, err := m.doBatchImportWithOverwrite(included, overwrite)
		return importDoneMsg{name: name, warnings: warnings, err: err}
	}
}

// handleMouseClick processes a left-click on zone-marked import options.
// Clicking an option sets the cursor and triggers Enter (select).
func (m importModel) handleMouseClick(msg tea.MouseMsg) (importModel, tea.Cmd) {
	// Check how many options are in the current step
	var maxItems int
	switch m.step {
	case stepSource:
		maxItems = 3
	case stepType:
		maxItems = len(m.types)
	case stepProvider:
		maxItems = len(m.providerNames)
	case stepBrowseStart:
		maxItems = 3
	case stepGitPick:
		maxItems = len(m.clonedItems)
	case stepValidate:
		maxItems = len(m.validationItems)
	default:
		return m, nil
	}

	for i := 0; i < maxItems; i++ {
		if zone.Get(fmt.Sprintf("import-opt-%d", i)).InBounds(msg) {
			switch m.step {
			case stepSource:
				m.sourceCursor = i
			case stepType:
				m.typeCursor = i
			case stepProvider:
				m.provCursor = i
			case stepBrowseStart:
				m.browseCursor = i
			case stepGitPick:
				m.pickCursor = i
			case stepValidate:
				m.validateCursor = i
			}
			// Synthesize Enter to activate the selection
			return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
	return m, nil
}

// hasTextInput returns true if the import model has an active text input.
func (m importModel) hasTextInput() bool {
	return m.step == stepGitURL || m.step == stepPath || m.step == stepName
}

// isValidGitURL checks that a URL looks like a legitimate git remote.
// Only allows secure transports (https://, ssh://, git@). Rejects insecure
// transports (git://, http://), argument injection (-), and ext:: (command injection).
func isValidGitURL(url string) bool {
	for _, prefix := range []string{"https://", "ssh://", "git@"} {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}

// discoverProviderDirs reads the existing provider subdirectories for a content type.
// Checks both the shared directory and local/ for provider dirs.
func (m importModel) discoverProviderDirs(ct catalog.ContentType) []string {
	seen := make(map[string]bool)
	var names []string

	// Check shared directory
	dirs := []string{
		filepath.Join(m.repoRoot, string(ct)),
		filepath.Join(m.repoRoot, "local", string(ct)),
	}
	for _, typeDir := range dirs {
		entries, err := os.ReadDir(typeDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && e.Name() != ".gitkeep" && !seen[e.Name()] {
				seen[e.Name()] = true
				names = append(names, e.Name())
			}
		}
	}
	return names
}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "(unknown)"
	}
	return dir
}

func homeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "(unknown)"
	}
	return dir
}

func (m importModel) viewValidate() string {
	s := helpStyle.Render("Review selections before import") + "\n\n"

	for i, vi := range m.validationItems {
		prefix := "   "
		nameStyle := itemStyle
		if i == m.validateCursor {
			prefix = " > "
			nameStyle = selectedItemStyle
		} else if vi.included {
			nameStyle = installedStyle
		}

		// Inclusion checkbox
		check := " "
		if vi.included {
			check = installedStyle.Render("✓")
		}

		// Status indicator
		status := installedStyle.Render("[✓] " + vi.detection)
		if vi.isWarning {
			status = errorMsgStyle.Render("[!] No recognized content")
		}

		row := prefix + "[" + check + "] " + nameStyle.Render(vi.name) + " " + status
		if vi.description != "" {
			row += "\n       " + countStyle.Render(vi.description)
		}
		s += zone.Mark(fmt.Sprintf("import-opt-%d", i), row) + "\n"
	}

	includedCount := 0
	for _, vi := range m.validationItems {
		if vi.included {
			includedCount++
		}
	}

	s += "\n" + helpStyle.Render(fmt.Sprintf("  %d of %d items will be imported", includedCount, len(m.validationItems)))
	s += "\n" + helpStyle.Render("up/down navigate • space toggle • enter import • esc back")
	return s
}
