package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type createLoadoutStep int

const (
	clStepProvider createLoadoutStep = iota // pick provider (skipped if pre-filled)
	clStepTypes                             // checkbox list of content types
	clStepItems                             // per-type item selection (Task 1.3)
	clStepName                              // name + description text inputs
	clStepDest                              // pick destination
	clStepReview                            // summary + confirm
)

// typeCheckEntry represents a content type checkbox in the type selection step.
type typeCheckEntry struct {
	ct      catalog.ContentType
	checked bool
	count   int // number of available items for this type
}

// loadoutItemEntry is one item in the checkbox picker.
type loadoutItemEntry struct {
	item     catalog.ContentItem
	selected bool
}

// buildLoadoutItemEntries creates entry list from catalog items.
func buildLoadoutItemEntries(cat *catalog.Catalog, scopeRegistry string) []loadoutItemEntry {
	var entries []loadoutItemEntry
	for _, item := range cat.Items {
		if scopeRegistry != "" && item.Registry != scopeRegistry {
			continue
		}
		entries = append(entries, loadoutItemEntry{item: item, selected: false})
	}
	return entries
}

// ---------------------------------------------------------------------------
// createLoadoutScreen — full-screen replacement for the old modal wizard.
// Renders in the content pane (sidebar visible) with a persistent split-view.
// ---------------------------------------------------------------------------

type createLoadoutScreen struct {
	step createLoadoutStep

	// Context passed in at creation
	prefilledProvider string
	scopeRegistry    string

	// Step 1: provider picker
	providerList   []provider.Provider
	providerCursor int

	// Step 2: content type selection
	typeEntries []typeCheckEntry
	typeCursor  int

	// Step 3: per-type item selection
	entries        []loadoutItemEntry
	selectedTypes  []catalog.ContentType
	typeStepIndex  int
	typeItemMap    map[catalog.ContentType][]int
	typeItemMapAll map[catalog.ContentType][]int
	showAllCompat  bool
	perTypeCursor  map[catalog.ContentType]int
	perTypeScroll  map[catalog.ContentType]int
	perTypeSearch  map[catalog.ContentType]string
	searchActive   bool
	searchInput    textinput.Model

	// Step 4: name/description
	nameInput textinput.Model
	descInput textinput.Model
	nameFirst bool

	// Step 5: destination
	destOptions  []string
	destCursor   int
	destDisabled []bool
	destHints    []string

	// Step 6: review
	reviewItemCursor int // cursor over the item list in review step
	reviewBtnCursor  int // 0=Back, 1=Create

	// Split-view preview
	splitView      splitViewModel
	previewLoading bool

	// Outcome
	confirmed    bool
	message      string
	messageIsErr bool

	// Layout
	width  int
	height int
}

func newCreateLoadoutScreen(
	prefilledProvider string,
	scopeRegistry string,
	allProviders []provider.Provider,
	cat *catalog.Catalog,
	width, height int,
) createLoadoutScreen {
	si := textinput.New()
	si.Placeholder = "filter items..."
	si.CharLimit = 100

	ni := textinput.New()
	ni.Prompt = labelStyle.Render("Name: ")
	ni.Placeholder = "my-loadout"
	ni.CharLimit = 100
	ni.Focus()

	di := textinput.New()
	di.Prompt = labelStyle.Render("Description: ")
	di.Placeholder = "What this loadout does"
	di.CharLimit = 300

	m := createLoadoutScreen{
		prefilledProvider: prefilledProvider,
		scopeRegistry:    scopeRegistry,
		providerList:     allProviders,
		searchInput:      si,
		nameInput:        ni,
		descInput:        di,
		nameFirst:        true,
		perTypeCursor:    make(map[catalog.ContentType]int),
		perTypeScroll:    make(map[catalog.ContentType]int),
		perTypeSearch:    make(map[catalog.ContentType]string),
		typeItemMap:      make(map[catalog.ContentType][]int),
		typeItemMapAll:   make(map[catalog.ContentType][]int),
		width:            width,
		height:           height,
	}

	m.destOptions = []string{"Project (loadouts/ in repo)", "Library (~/.syllago/content/loadouts/)"}
	m.destDisabled = []bool{false, false}
	m.destHints = []string{"", ""}
	if scopeRegistry != "" {
		m.destOptions = append(m.destOptions, fmt.Sprintf("Registry: %s", scopeRegistry))
		m.destDisabled = append(m.destDisabled, false)
		m.destHints = append(m.destHints, "")
	}

	m.entries = buildLoadoutItemEntries(cat, scopeRegistry)
	m.splitView = newSplitView(nil, "wiz")
	m.splitView.width = width
	m.splitView.height = height - 5 // reserve space for header + help bar

	if prefilledProvider != "" {
		m.buildTypeEntries()
		m.step = clStepTypes
	} else {
		m.step = clStepProvider
	}

	return m
}

// --- Helper methods ---

func (m *createLoadoutScreen) buildTypeEntries() {
	provSlug := m.prefilledProvider
	var prov *provider.Provider
	for i := range m.providerList {
		if m.providerList[i].Slug == provSlug {
			prov = &m.providerList[i]
			break
		}
	}

	typeCounts := make(map[catalog.ContentType]int)
	for _, e := range m.entries {
		typeCounts[e.item.Type]++
	}

	m.typeEntries = nil
	for _, ct := range catalog.AllContentTypes() {
		if ct == catalog.Loadouts {
			continue
		}
		count := typeCounts[ct]
		if count == 0 {
			continue
		}
		if prov != nil && prov.SupportsType != nil && !prov.SupportsType(ct) {
			continue
		}
		m.typeEntries = append(m.typeEntries, typeCheckEntry{
			ct:      ct,
			checked: true,
			count:   count,
		})
	}
	m.typeCursor = 0
}

func (m *createLoadoutScreen) buildTypeItemMaps() {
	m.selectedTypes = nil
	for _, te := range m.typeEntries {
		if te.checked {
			m.selectedTypes = append(m.selectedTypes, te.ct)
		}
	}

	provSlug := m.prefilledProvider
	var prov *provider.Provider
	for i := range m.providerList {
		if m.providerList[i].Slug == provSlug {
			prov = &m.providerList[i]
			break
		}
	}

	m.typeItemMap = make(map[catalog.ContentType][]int)
	m.typeItemMapAll = make(map[catalog.ContentType][]int)
	for i, e := range m.entries {
		ct := e.item.Type
		m.typeItemMapAll[ct] = append(m.typeItemMapAll[ct], i)
		if prov == nil || prov.SupportsType == nil || prov.SupportsType(ct) {
			m.typeItemMap[ct] = append(m.typeItemMap[ct], i)
		}
	}
	m.typeStepIndex = 0
}

func (m createLoadoutScreen) currentTypeItems() []int {
	if m.typeStepIndex >= len(m.selectedTypes) {
		return nil
	}
	ct := m.selectedTypes[m.typeStepIndex]
	if m.showAllCompat {
		return m.typeItemMapAll[ct]
	}
	return m.typeItemMap[ct]
}

func (m createLoadoutScreen) currentType() catalog.ContentType {
	if m.typeStepIndex >= len(m.selectedTypes) {
		return ""
	}
	return m.selectedTypes[m.typeStepIndex]
}

func (m createLoadoutScreen) currentTypeSelectedCount() int {
	count := 0
	ct := m.currentType()
	for _, e := range m.entries {
		if e.selected && e.item.Type == ct {
			count++
		}
	}
	return count
}

func (m createLoadoutScreen) isItemCompatible(idx int) bool {
	ct := m.entries[idx].item.Type
	indices := m.typeItemMap[ct]
	for _, i := range indices {
		if i == idx {
			return true
		}
	}
	return false
}

func (m createLoadoutScreen) filteredTypeItems() []int {
	items := m.currentTypeItems()
	ct := m.currentType()
	query := strings.ToLower(m.perTypeSearch[ct])
	if query == "" {
		return items
	}
	var out []int
	for _, idx := range items {
		e := m.entries[idx]
		if strings.Contains(strings.ToLower(e.item.Name), query) ||
			strings.Contains(strings.ToLower(e.item.Description), query) {
			out = append(out, idx)
		}
	}
	return out
}

func (m createLoadoutScreen) selectedItems() []loadoutItemEntry {
	var out []loadoutItemEntry
	for _, e := range m.entries {
		if e.selected {
			out = append(out, e)
		}
	}
	return out
}

func (m *createLoadoutScreen) updateDestConstraints() {
	if len(m.destOptions) < 3 {
		return
	}
	selected := m.selectedItems()
	registries := make(map[string]bool)
	providers := make(map[string]bool)
	for _, e := range selected {
		if e.item.Registry != "" {
			registries[e.item.Registry] = true
		}
		if e.item.Provider != "" {
			providers[e.item.Provider] = true
		}
	}
	regIdx := 2
	if len(registries) > 1 {
		m.destDisabled[regIdx] = true
		m.destHints[regIdx] = "Items span multiple registries"
	} else if len(providers) > 1 {
		m.destDisabled[regIdx] = true
		m.destHints[regIdx] = "Items target multiple providers"
	} else {
		m.destDisabled[regIdx] = false
		m.destHints[regIdx] = ""
	}
	if m.destDisabled[regIdx] && m.destCursor == regIdx {
		m.destCursor--
	}
}

func (m createLoadoutScreen) currentStepNum() int {
	fixedBefore := 2
	if m.prefilledProvider != "" {
		fixedBefore = 1
	}
	switch m.step {
	case clStepProvider:
		return 1
	case clStepTypes:
		if m.prefilledProvider != "" {
			return 1
		}
		return 2
	case clStepItems:
		return fixedBefore + m.typeStepIndex + 1
	case clStepName:
		return fixedBefore + len(m.selectedTypes) + 1
	case clStepDest:
		return fixedBefore + len(m.selectedTypes) + 2
	case clStepReview:
		return fixedBefore + len(m.selectedTypes) + 3
	}
	return 1
}

func (m createLoadoutScreen) dynamicTotalSteps() int {
	fixedBefore := 2
	if m.prefilledProvider != "" {
		fixedBefore = 1
	}
	nTypes := len(m.selectedTypes)
	if nTypes == 0 {
		nTypes = len(m.typeEntries)
	}
	return fixedBefore + nTypes + 3
}

// primaryFilePath returns the absolute path to the primary file for a content item.
func primaryFilePath(item catalog.ContentItem) string {
	if len(item.Files) == 0 || item.Path == "" {
		return ""
	}
	primary := findPrimaryFile(item)
	if primary == "" {
		return ""
	}
	return filepath.Join(item.Path, primary)
}

// previewCmdForCursor emits a splitViewCursorMsg for the current cursor item.
// App uses this to load the primary file preview into the right pane.
func (m createLoadoutScreen) previewCmdForCursor() tea.Cmd {
	ct := m.currentType()
	filtered := m.filteredTypeItems()
	cursor := m.perTypeCursor[ct]
	if cursor < 0 || cursor >= len(filtered) {
		return nil
	}
	idx := filtered[cursor]
	e := m.entries[idx]
	item := splitViewItem{
		Label: e.item.Name,
		Path:  primaryFilePath(e.item),
	}
	return func() tea.Msg {
		return splitViewCursorMsg{index: cursor, item: item}
	}
}

func (m createLoadoutScreen) Update(msg tea.Msg) (createLoadoutScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""

		switch m.step {
		case clStepProvider:
			switch {
			case msg.Type == tea.KeyEsc:
				// Esc on provider step signals cancellation — App handles navigation
				m.confirmed = false
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.providerCursor > 0 {
					m.providerCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.providerCursor < len(m.providerList)-1 {
					m.providerCursor++
				}
			case msg.Type == tea.KeyEnter:
				m.prefilledProvider = m.providerList[m.providerCursor].Slug
				m.buildTypeEntries()
				m.step = clStepTypes
			}

		case clStepTypes:
			switch {
			case msg.Type == tea.KeyEsc:
				if m.prefilledProvider != "" {
					// Pre-filled provider — Esc signals cancellation
					m.confirmed = false
				} else {
					m.step = clStepProvider
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.typeCursor > 0 {
					m.typeCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.typeCursor < len(m.typeEntries)-1 {
					m.typeCursor++
				}
			case key.Matches(msg, keys.Space):
				if m.typeCursor < len(m.typeEntries) {
					m.typeEntries[m.typeCursor].checked = !m.typeEntries[m.typeCursor].checked
				}
			case key.Matches(msg, keys.ToggleAll):
				allChecked := true
				for _, te := range m.typeEntries {
					if !te.checked {
						allChecked = false
						break
					}
				}
				for i := range m.typeEntries {
					m.typeEntries[i].checked = !allChecked
				}
			case msg.Type == tea.KeyEnter:
				anySelected := false
				for _, te := range m.typeEntries {
					if te.checked {
						anySelected = true
						break
					}
				}
				if !anySelected {
					m.message = "Select at least one content type"
					m.messageIsErr = true
					return m, nil
				}
				m.buildTypeItemMaps()
				m.step = clStepItems
				return m, m.previewCmdForCursor()
			}

		case clStepItems:
			ct := m.currentType()
			if m.searchActive {
				if msg.Type == tea.KeyEsc {
					m.searchActive = false
					m.searchInput.Blur()
					m.searchInput.SetValue("")
					m.perTypeSearch[ct] = ""
					return m, nil
				}
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.perTypeSearch[ct] = m.searchInput.Value()
				return m, cmd
			}
			filtered := m.filteredTypeItems()
			cursor := m.perTypeCursor[ct]
			switch {
			case msg.Type == tea.KeyEsc:
				m.perTypeCursor[ct] = cursor
				if m.typeStepIndex == 0 {
					m.step = clStepTypes
				} else {
					m.typeStepIndex--
					prevCt := m.currentType()
					m.searchInput.SetValue(m.perTypeSearch[prevCt])
				}
				return m, nil
			case key.Matches(msg, keys.Up):
				if cursor > 0 {
					m.perTypeCursor[ct] = cursor - 1
					return m, m.previewCmdForCursor()
				}
			case key.Matches(msg, keys.Down):
				if cursor < len(filtered)-1 {
					m.perTypeCursor[ct] = cursor + 1
					return m, m.previewCmdForCursor()
				}
			case key.Matches(msg, keys.Space):
				if cursor >= 0 && cursor < len(filtered) {
					entryIdx := filtered[cursor]
					if m.isItemCompatible(entryIdx) {
						m.entries[entryIdx].selected = !m.entries[entryIdx].selected
						m.updateDestConstraints()
					}
				}
				return m, nil
			case key.Matches(msg, keys.ToggleAll):
				compatible := m.typeItemMap[ct]
				allSelected := true
				for _, idx := range compatible {
					if !m.entries[idx].selected {
						allSelected = false
						break
					}
				}
				for _, idx := range compatible {
					m.entries[idx].selected = !allSelected
				}
				m.updateDestConstraints()
				return m, nil
			case key.Matches(msg, keys.ToggleCompat):
				m.showAllCompat = !m.showAllCompat
				newFiltered := m.filteredTypeItems()
				if cursor >= len(newFiltered) && len(newFiltered) > 0 {
					m.perTypeCursor[ct] = len(newFiltered) - 1
				}
				return m, nil
			case msg.String() == "/":
				m.searchActive = true
				m.searchInput.SetValue(m.perTypeSearch[ct])
				m.searchInput.Focus()
				return m, nil
			case msg.String() == "l":
				m.splitView.focusedPane = panePreview
				return m, nil
			case msg.String() == "h":
				m.splitView.focusedPane = paneList
				return m, nil
			case msg.Type == tea.KeyEnter:
				m.perTypeCursor[ct] = cursor
				if m.typeStepIndex < len(m.selectedTypes)-1 {
					m.typeStepIndex++
					nextCt := m.currentType()
					m.searchInput.SetValue(m.perTypeSearch[nextCt])
					m.searchInput.Blur()
					m.searchActive = false
				} else {
					m.step = clStepName
				}
				return m, m.previewCmdForCursor()
			}

		case clStepName:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepItems
				return m, nil
			case msg.Type == tea.KeyTab:
				if m.nameFirst {
					m.nameFirst = false
					m.nameInput.Blur()
					m.descInput.Focus()
				} else {
					m.nameFirst = true
					m.descInput.Blur()
					m.nameInput.Focus()
				}
				return m, nil
			case msg.Type == tea.KeyEnter:
				if strings.TrimSpace(m.nameInput.Value()) == "" {
					m.message = "Name is required"
					m.messageIsErr = true
					return m, nil
				}
				m.step = clStepDest
				return m, nil
			}
			var cmd tea.Cmd
			if m.nameFirst {
				m.nameInput, cmd = m.nameInput.Update(msg)
			} else {
				m.descInput, cmd = m.descInput.Update(msg)
			}
			return m, cmd

		case clStepDest:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepName
				return m, nil
			case key.Matches(msg, keys.Up):
				if m.destCursor > 0 {
					m.destCursor--
					for m.destCursor > 0 && m.destDisabled[m.destCursor] {
						m.destCursor--
					}
				}
			case key.Matches(msg, keys.Down):
				if m.destCursor < len(m.destOptions)-1 {
					m.destCursor++
					for m.destCursor < len(m.destOptions)-1 && m.destDisabled[m.destCursor] {
						m.destCursor++
					}
				}
			case msg.Type == tea.KeyEnter:
				if m.destDisabled[m.destCursor] {
					return m, nil
				}
				m.reviewBtnCursor = 1 // default to Create button
				m.step = clStepReview
				return m, nil
			}

		case clStepReview:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = clStepDest
				return m, nil
			case key.Matches(msg, keys.Left):
				if m.reviewBtnCursor > 0 {
					m.reviewBtnCursor--
				}
			case key.Matches(msg, keys.Right):
				if m.reviewBtnCursor < 1 {
					m.reviewBtnCursor++
				}
			case msg.Type == tea.KeyEnter:
				if m.reviewBtnCursor == 0 {
					// Back
					m.step = clStepDest
				} else {
					// Create
					m.confirmed = true
				}
				return m, nil
			}
		}

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}

		switch m.step {
		case clStepProvider:
			for i := range m.providerList {
				if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
					m.providerCursor = i
					m.prefilledProvider = m.providerList[i].Slug
					m.buildTypeEntries()
					m.step = clStepTypes
					return m, nil
				}
			}

		case clStepTypes:
			for i := range m.typeEntries {
				if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
					m.typeEntries[i].checked = !m.typeEntries[i].checked
					return m, nil
				}
			}

		case clStepItems:
			if zone.Get("wiz-field-search").InBounds(msg) {
				m.searchInput.Focus()
				return m, nil
			}
			filtered := m.filteredTypeItems()
			for vi, absIdx := range filtered {
				if zone.Get(fmt.Sprintf("wiz-opt-%d", absIdx)).InBounds(msg) {
					ct := m.currentType()
					m.perTypeCursor[ct] = vi
					if m.isItemCompatible(absIdx) {
						m.entries[absIdx].selected = !m.entries[absIdx].selected
					}
					return m, m.previewCmdForCursor()
				}
			}

		case clStepName:
			if zone.Get("wiz-field-name").InBounds(msg) {
				m.nameFirst = true
				m.nameInput.Focus()
				m.descInput.Blur()
				return m, nil
			}
			if zone.Get("wiz-field-desc").InBounds(msg) {
				m.nameFirst = false
				m.descInput.Focus()
				m.nameInput.Blur()
				return m, nil
			}

		case clStepDest:
			for i := range m.destOptions {
				if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
					if !m.destDisabled[i] {
						m.destCursor = i
					}
					return m, nil
				}
			}

		case clStepReview:
			if zone.Get("wiz-btn-back").InBounds(msg) {
				m.step = clStepDest
				return m, nil
			}
			if zone.Get("wiz-btn-create").InBounds(msg) {
				m.confirmed = true
				return m, nil
			}
		}
	}
	return m, nil
}

func (m createLoadoutScreen) View() string {
	breadcrumbSegments := []BreadcrumbSegment{
		{"Home", "crumb-home"},
		{"Loadouts", "crumb-parent"},
	}
	stepLabel := fmt.Sprintf("(%d of %d)", m.currentStepNum(), m.dynamicTotalSteps())
	if m.step == clStepItems {
		breadcrumbSegments = append(breadcrumbSegments,
			BreadcrumbSegment{m.currentType().Label(), ""})
	} else {
		breadcrumbSegments = append(breadcrumbSegments,
			BreadcrumbSegment{"Create", ""})
	}
	s := renderBreadcrumb(breadcrumbSegments...) + "  " + helpStyle.Render(stepLabel) + "\n\n"
	s += m.renderSplitTitleBar()
	left := m.renderLeftPane()
	s += m.renderSplitView(left)
	return s
}

func (m createLoadoutScreen) renderLeftPane() string {
	leftW := m.splitView.leftWidth()
	if leftW < 20 {
		leftW = 20
	}
	var body string

	switch m.step {
	case clStepProvider:
		body = labelStyle.Render("Pick a provider:") + "\n\n"
		for i, prov := range m.providerList {
			prefix, style := cursorPrefix(i == m.providerCursor)
			detected := ""
			if prov.Detected {
				detected = " " + installedStyle.Render("(detected)")
			}
			row := fmt.Sprintf("  %s%s%s", prefix, style.Render(prov.Name), detected)
			body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
		}

	case clStepTypes:
		body = labelStyle.Render("Uncheck any types to skip.") + "\n\n"
		for i, te := range m.typeEntries {
			checkBox := helpStyle.Render("[x]")
			if !te.checked {
				checkBox = helpStyle.Render("[ ]")
			}
			prefix, style := cursorPrefix(i == m.typeCursor)
			badge := ""
			if te.ct == catalog.Hooks || te.ct == catalog.MCP {
				badge = " " + warningStyle.Render("!!")
			}
			countLabel := helpStyle.Render(fmt.Sprintf("(%d)", te.count))
			row := fmt.Sprintf("  %s%s %s%s %s",
				prefix, checkBox, style.Render(te.ct.Label()), badge, countLabel)
			body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
		}
		body += "\n" + helpStyle.Render("space toggle • a toggle all • enter next")
		if m.message != "" && m.messageIsErr {
			body += "\n" + errorMsgStyle.Render(m.message)
		}

	case clStepItems:
		ct := m.currentType()
		selCount := m.currentTypeSelectedCount()
		body = labelStyle.Render(fmt.Sprintf("Select %s (%d selected)", ct.Label(), selCount)) + "\n"
		if m.searchActive {
			body += zone.Mark("wiz-field-search", m.searchInput.View()) + "\n"
		} else {
			body += "\n"
		}
		filtered := m.filteredTypeItems()
		cursor := m.perTypeCursor[ct]
		visibleH := m.splitView.visibleListRows() - 4
		if visibleH < 3 {
			visibleH = 3
		}
		start := 0
		if len(filtered) > visibleH {
			start = cursor - visibleH/2
			if start < 0 {
				start = 0
			}
			if start+visibleH > len(filtered) {
				start = len(filtered) - visibleH
			}
		}
		end := start + visibleH
		if end > len(filtered) {
			end = len(filtered)
		}
		if start > 0 {
			body += "  " + renderScrollUp(start, false) + "\n"
		}
		maxNameW := leftW - 10
		if maxNameW < 10 {
			maxNameW = 10
		}
		for vi, fi := range filtered[start:end] {
			e := m.entries[fi]
			absIdx := start + vi
			compatible := m.isItemCompatible(fi)
			checkBox := helpStyle.Render("[ ]")
			if e.selected {
				checkBox = helpStyle.Render("[x]")
			}
			prefix, style := cursorPrefix(absIdx == cursor)
			name := e.item.Name
			if len(name) > maxNameW {
				name = name[:maxNameW-3] + "..."
			}
			source := ""
			if e.item.Registry != "" {
				source = " (" + e.item.Registry + ")"
			} else if e.item.Library {
				source = " (library)"
			}
			var row string
			if !compatible {
				row = fmt.Sprintf("  %s%s %s%s (incompatible)",
					prefix, checkBox, helpStyle.Render(name), helpStyle.Render(source))
			} else {
				row = fmt.Sprintf("  %s%s %s%s",
					prefix, checkBox, style.Render(name), helpStyle.Render(source))
			}
			body += zone.Mark(fmt.Sprintf("wiz-opt-%d", absIdx), row) + "\n"
		}
		if end < len(filtered) {
			body += "  " + renderScrollDown(len(filtered)-end, false) + "\n"
		}
		filterMode := "compatible only"
		if m.showAllCompat {
			filterMode = "showing all"
		}
		body += "\n" + helpStyle.Render(fmt.Sprintf("space select • a all • t filter (%s) • / search • enter next", filterMode))

	case clStepName:
		body = labelStyle.Render("Name your loadout:") + "\n\n"
		body += zone.Mark("wiz-field-name", m.nameInput.View()) + "\n"
		body += zone.Mark("wiz-field-desc", m.descInput.View()) + "\n"
		body += "\n" + helpStyle.Render("tab switch field • enter next")
		if m.message != "" && m.messageIsErr {
			body += "\n" + errorMsgStyle.Render(m.message)
		}

	case clStepDest:
		body = labelStyle.Render("Choose destination:") + "\n\n"
		for i, opt := range m.destOptions {
			prefix, style := cursorPrefix(i == m.destCursor)
			if m.destDisabled[i] {
				style = helpStyle
				prefix = "  "
			}
			row := fmt.Sprintf("  %s%s", prefix, style.Render(opt))
			if m.destDisabled[i] && m.destHints[i] != "" {
				row += "\n      " + helpStyle.Render(m.destHints[i])
			}
			body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
		}
		body += "\n" + helpStyle.Render("enter next • esc back")

	case clStepReview:
		body = m.renderReviewLeftPane(leftW)
	}

	return body
}

// renderReviewLeftPane renders the review step left pane.
func (m createLoadoutScreen) renderReviewLeftPane(leftW int) string {
	body := labelStyle.Render("Review & Create") + "\n\n"

	provName := m.prefilledProvider
	for _, p := range m.providerList {
		if p.Slug == m.prefilledProvider {
			provName = p.Name
			break
		}
	}
	body += fmt.Sprintf("  %s %s\n", labelStyle.Render("Provider:"), provName)
	body += fmt.Sprintf("  %s %s\n", labelStyle.Render("Name:"), m.nameInput.Value())
	if desc := m.descInput.Value(); desc != "" {
		body += fmt.Sprintf("  %s %s\n", labelStyle.Render("Description:"), desc)
	}
	body += fmt.Sprintf("  %s %s\n", labelStyle.Render("Destination:"), m.destOptions[m.destCursor])

	selected := m.selectedItems()
	if len(selected) == 0 {
		body += "\n  " + warningStyle.Render("No items selected") + "\n"
	} else {
		body += fmt.Sprintf("\n  %s\n", labelStyle.Render(fmt.Sprintf("Items (%d):", len(selected))))
		byType := make(map[catalog.ContentType][]loadoutItemEntry)
		for _, e := range selected {
			byType[e.item.Type] = append(byType[e.item.Type], e)
		}
		for _, ct := range catalog.AllContentTypes() {
			items := byType[ct]
			if len(items) == 0 {
				continue
			}
			badge := ""
			if ct == catalog.Hooks || ct == catalog.MCP {
				badge = " " + warningStyle.Render("!!")
			}
			body += fmt.Sprintf("\n  %s%s\n", labelStyle.Render(ct.Label()), badge)
			maxNameW := leftW - 6
			if maxNameW < 10 {
				maxNameW = 10
			}
			for _, e := range items {
				name := e.item.Name
				if len(name) > maxNameW {
					name = name[:maxNameW-3] + "..."
				}
				body += fmt.Sprintf("    %s\n", name)
			}
		}

		hasSecurityContent := false
		for _, e := range selected {
			if e.item.Type == catalog.Hooks || e.item.Type == catalog.MCP {
				hasSecurityContent = true
				break
			}
		}
		if hasSecurityContent {
			body += "\n  " + warningStyle.Render("Security Notice: Hooks and MCP configs run code.") + "\n"
			body += "  " + warningStyle.Render("Review content before applying this loadout.") + "\n"
		}
	}

	// Buttons
	body += "\n"
	backStyle := buttonDisabledStyle
	createStyle := buttonDisabledStyle
	if m.reviewBtnCursor == 0 {
		backStyle = buttonStyle
	} else {
		createStyle = buttonStyle
	}
	body += "  " + zone.Mark("wiz-btn-back", backStyle.Render("Back"))
	body += "  " + zone.Mark("wiz-btn-create", createStyle.Render("Create"))
	body += "\n"

	return body
}

// renderSplitTitleBar renders "Items | Preview" tab bar on items and review steps.
func (m createLoadoutScreen) renderSplitTitleBar() string {
	if m.step != clStepItems && m.step != clStepReview {
		return ""
	}
	if m.width < splitViewMinWidth {
		return ""
	}
	listLabel := "Items"
	if m.step == clStepReview {
		listLabel = "Review"
	}
	sep := helpStyle.Render(" | ")
	leftStyle := activeTabStyle
	rightStyle := inactiveTabStyle
	if m.splitView.focusedPane == panePreview {
		leftStyle = inactiveTabStyle
		rightStyle = activeTabStyle
	}
	return leftStyle.Render(listLabel) + sep + rightStyle.Render("Preview") + "\n\n"
}

// renderSplitView composes the left wizard pane and right preview pane.
func (m createLoadoutScreen) renderSplitView(leftContent string) string {
	contentW := m.width
	if contentW < splitViewMinWidth {
		return leftContent
	}

	leftW := m.splitView.leftWidth()
	rightW := contentW - leftW - 1

	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(m.splitView.renderPreviewContent(rightW), "\n")

	displayH := m.height - 5
	if displayH < 5 {
		displayH = 5
	}

	for len(leftLines) < displayH {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < displayH {
		rightLines = append(rightLines, "")
	}

	sep := helpStyle.Render("│")
	var rows []string
	for i := 0; i < displayH; i++ {
		l := leftLines[i]
		r := rightLines[i]
		visW := lipgloss.Width(l)
		if visW < leftW {
			l = l + strings.Repeat(" ", leftW-visW)
		}
		rows = append(rows, l+sep+r)
	}
	return strings.Join(rows, "\n")
}
