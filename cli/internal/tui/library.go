package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// libraryMode tracks whether we're browsing the table or viewing item detail.
type libraryMode int

const (
	libraryBrowse libraryMode = iota // full-width table
	libraryDetail                    // file tree + preview drill-in
)

// libraryDrillMsg is sent when the user drills into an item from the Library table.
type libraryDrillMsg struct {
	item *catalog.ContentItem
}

// libraryEditMsg is sent when the edit button is clicked in the metadata bar.
type libraryEditMsg struct{}

// libraryInstallMsg is sent when the install button is clicked in the metadata bar.
type libraryInstallMsg struct{}

// libraryRemoveMsg is sent when the remove button is clicked in the metadata bar.
type libraryRemoveMsg struct{}

// libraryUninstallMsg is sent when the uninstall button is clicked in the metadata bar.
type libraryUninstallMsg struct{}

// libraryCloseMsg is sent when the user closes the detail view.
type libraryCloseMsg struct{}

// libraryTrustInspectMsg is sent when the user opens the Trust Inspector
// for the currently selected library item (via [t] or clicking the Trust
// field in the metapanel).
type libraryTrustInspectMsg struct {
	item *catalog.ContentItem
}

// libraryModel manages the Library tab: full-width table with drill-in detail view.
type libraryModel struct {
	table   tableModel
	tree    fileTreeModel
	preview previewModel
	mode    libraryMode
	focus   explorerPane // paneItems=tree, panePreview=preview (reuse enum)
	width   int
	height  int

	// The item currently being viewed in detail
	detailItem *catalog.ContentItem

	// D16 verification state — used to render the per-target breakdown in
	// the rule metapanel. verification.PerRecord is keyed by
	// (LibraryID, TargetFile); installed.RuleAppends enumerates which
	// records to show (filtered by LibraryID for the selected rule).
	verification *installcheck.VerificationResult
	installed    *installer.Installed
}

func newLibraryModel(items []catalog.ContentItem, provs []provider.Provider, repoRoot string) libraryModel {
	return libraryModel{
		table:   newTableModel(items, provs, repoRoot),
		preview: newPreviewModel(),
		mode:    libraryBrowse,
	}
}

// currentMetaItem returns the item whose metadata is displayed for the
// current mode: the selected row in browse mode, or the drill-in item in
// detail mode. Used by metaBarLines() for per-item height computation.
func (l libraryModel) currentMetaItem() *catalog.ContentItem {
	switch l.mode {
	case libraryDetail:
		return l.detailItem
	default:
		return l.table.Selected()
	}
}

// metaBarLines returns the dynamic number of metadata content lines for
// the currently-selected item (varies with TrustTier/Revoked/PrivateRepo).
// Non-MOAT items return metaBarLinesBase (3) so existing layouts are stable.
// For rule items with active installs, the D16 per-target breakdown adds
// extra lines beyond the base via metaPanelExtraLines.
func (l libraryModel) metaBarLines() int {
	item := l.currentMetaItem()
	base := metaBarLinesFor(item)
	extra := metaPanelExtraLines(metaPanelData{ruleRecords: l.ruleRecordsFor(item)})
	return base + extra
}

// metaBarTotal returns metaBarLines + 1 for the shared separator line
// drawn between the metadata section and the content pane.
func (l libraryModel) metaBarTotal() int {
	return l.metaBarLines() + 1
}

// SetSize updates layout dimensions.
func (l *libraryModel) SetSize(width, height int) {
	l.width = width
	l.height = height

	switch l.mode {
	case libraryBrowse:
		innerH := height - borderSize
		if l.table.Len() > 0 {
			innerH = max(3, innerH-l.metaBarTotal())
		}
		l.table.SetSize(width-borderSize, innerH)
	case libraryDetail:
		l.sizeDetailPanes()
	}
}

// SetItems replaces the table data and returns to browse mode.
func (l *libraryModel) SetItems(items []catalog.ContentItem) {
	l.table.SetItems(items)
	l.mode = libraryBrowse
	l.detailItem = nil
}

// SetVerification stores the D16 rule-append verification result on the
// underlying table so the Installed column can render binary Installed/Not-
// Installed for rules based on MatchSet lookups by library ID.
//
// Passing nil clears the verification state — the table falls back to the
// pre-D16 provider-abbreviation rendering for rules (e.g. "CC,GC"). The
// non-rule rows are unaffected regardless of whether verification is set.
func (l *libraryModel) SetVerification(v *installcheck.VerificationResult) {
	l.table.SetVerification(v)
	l.verification = v
}

// SetInstalled stores the installed.json snapshot so the metapanel can
// enumerate per-target records (InstalledRuleAppend entries filtered by
// LibraryID) when rendering the D16 breakdown for a rule item. Nil is a
// valid value — metapanel falls back to the base 4 lines without the
// breakdown section.
func (l *libraryModel) SetInstalled(inst *installer.Installed) {
	l.installed = inst
}

// Update handles input for the current mode.
func (l libraryModel) Update(msg tea.Msg) (libraryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return l.updateMouse(msg)
	case tea.KeyMsg:
		switch l.mode {
		case libraryBrowse:
			return l.updateBrowse(msg)
		case libraryDetail:
			return l.updateDetail(msg)
		}
	}
	return l, nil
}

// updateBrowse handles keys in table browse mode.
func (l libraryModel) updateBrowse(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	// When actively searching, route most keys to the search input
	if l.table.searching {
		return l.updateSearch(msg)
	}

	switch msg.String() {
	case keyDown, "down":
		l.table.CursorDown()
	case keyUp, "up":
		l.table.CursorUp()
	case "enter":
		if item := l.table.Selected(); item != nil {
			l.drillIn(item)
			return l, func() tea.Msg { return libraryDrillMsg{item: item} }
		}
	case keyTrust:
		// [t] opens the Trust Inspector for the focused row. No-op for
		// non-MOAT items — the inspector shows "Unknown / No trust claim"
		// which is honest about the absence rather than hiding the affordance.
		if item := l.table.Selected(); item != nil {
			return l, func() tea.Msg { return libraryTrustInspectMsg{item: item} }
		}
	case keySearch:
		l.table.StartSearch()
	case "s":
		l.table.CycleSort()
	case "S":
		l.table.ReverseSort()
	case "esc":
		if l.table.searchQuery != "" {
			l.table.CancelSearch()
		}
	case "pgup", "ctrl+u":
		l.table.PageUp()
	case "pgdown", "ctrl+d":
		l.table.PageDown()
	case "g", "home":
		l.table.cursor = 0
		l.table.offset = 0
	case "G", "end":
		if len(l.table.items) > 0 {
			l.table.cursor = len(l.table.items) - 1
			l.table.offset = max(0, len(l.table.items)-l.table.viewHeight())
		}
	}
	return l, nil
}

// updateSearch handles keys when the search input is active.
func (l libraryModel) updateSearch(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		l.table.CancelSearch()
	case tea.KeyEnter:
		l.table.SearchConfirm()
	case tea.KeyBackspace:
		l.table.SearchBackspace()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			l.table.SearchType(r)
		}
	}
	return l, nil
}

// updateDetail handles keys in file tree + preview detail mode.
func (l libraryModel) updateDetail(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "x":
		l.mode = libraryBrowse
		l.detailItem = nil
		l.table.SetSize(l.width-borderSize, l.height-borderSize)
		return l, func() tea.Msg { return libraryCloseMsg{} }

	case keyLeft, "left":
		l.setDetailFocus(paneItems)
		return l, nil
	case keyRight, "right":
		l.setDetailFocus(panePreview)
		return l, nil
	case keyTrust:
		// [t] opens the Trust Inspector for the item in detail view. Pane
		// focus is irrelevant — the inspector is a modal over the whole
		// library, and the user's mental model is "inspect this item," not
		// "inspect this pane." Routed here instead of per-pane so the key
		// works whether the tree or the preview is focused.
		if l.detailItem != nil {
			item := l.detailItem
			return l, func() tea.Msg { return libraryTrustInspectMsg{item: item} }
		}
	}

	switch l.focus {
	case paneItems:
		return l.updateTree(msg)
	case panePreview:
		return l.updatePreviewKeys(msg)
	}
	return l, nil
}

// updateTree handles keys when file tree is focused.
func (l libraryModel) updateTree(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		l.tree.CursorDown()
		l.loadSelectedFile()
	case keyUp, "up":
		l.tree.CursorUp()
		l.loadSelectedFile()
	case "pgup", "ctrl+u":
		l.tree.PageUp()
		l.loadSelectedFile()
	case "pgdown", "ctrl+d":
		l.tree.PageDown()
		l.loadSelectedFile()
	case "enter", " ":
		if l.tree.cursor >= 0 && l.tree.cursor < len(l.tree.nodes) {
			if l.tree.nodes[l.tree.cursor].isDir {
				l.tree.ToggleDir()
			} else {
				l.loadSelectedFile()
			}
		}
	}
	return l, nil
}

// updatePreviewKeys handles keys when preview is focused.
func (l libraryModel) updatePreviewKeys(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		l.preview.ScrollDown()
	case keyUp, "up":
		l.preview.ScrollUp()
	case "pgdown", "ctrl+d":
		l.preview.PageDown()
	case "pgup", "ctrl+u":
		l.preview.PageUp()
	}
	return l, nil
}

// updateMouse handles mouse events.
func (l libraryModel) updateMouse(msg tea.MouseMsg) (libraryModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		switch l.mode {
		case libraryBrowse:
			// Metadata bar button clicks
			if zone.Get("meta-install").InBounds(msg) {
				return l, func() tea.Msg { return libraryInstallMsg{} }
			}
			if zone.Get("meta-edit").InBounds(msg) {
				return l, func() tea.Msg { return libraryEditMsg{} }
			}
			if zone.Get("meta-remove").InBounds(msg) {
				return l, func() tea.Msg { return libraryRemoveMsg{} }
			}
			if zone.Get("meta-uninstall").InBounds(msg) {
				return l, func() tea.Msg { return libraryUninstallMsg{} }
			}
			if zone.Get("meta-trust").InBounds(msg) {
				if item := l.table.Selected(); item != nil {
					return l, func() tea.Msg { return libraryTrustInspectMsg{item: item} }
				}
			}

			// Column header clicks for sorting
			colZones := []struct {
				id  string
				col sortColumn
			}{
				{"col-name", sortByName},
				{"col-type", sortByType},
				{"col-scope", sortByScope},
				{"col-files", sortByFiles},
				{"col-installed", sortByInstalled},
				{"col-desc", sortByDesc},
			}
			for _, cz := range colZones {
				if zone.Get(cz.id).InBounds(msg) {
					l.table.SortByColumn(cz.col)
					return l, nil
				}
			}

			// Row clicks — double-click drills in
			for i := range l.table.items {
				if zone.Get("tbl-" + itoa(i)).InBounds(msg) {
					if l.table.cursor == i {
						// Second click on same row — drill in
						if item := l.table.Selected(); item != nil {
							l.drillIn(item)
							return l, func() tea.Msg { return libraryDrillMsg{item: item} }
						}
					}
					l.table.cursor = i
					return l, nil
				}
			}
		case libraryDetail:
			// Metadata bar button clicks
			if zone.Get("meta-install").InBounds(msg) {
				return l, func() tea.Msg { return libraryInstallMsg{} }
			}
			if zone.Get("meta-edit").InBounds(msg) {
				return l, func() tea.Msg { return libraryEditMsg{} }
			}
			if zone.Get("meta-remove").InBounds(msg) {
				return l, func() tea.Msg { return libraryRemoveMsg{} }
			}
			if zone.Get("meta-uninstall").InBounds(msg) {
				return l, func() tea.Msg { return libraryUninstallMsg{} }
			}
			if zone.Get("meta-trust").InBounds(msg) {
				if l.detailItem != nil {
					item := l.detailItem
					return l, func() tea.Msg { return libraryTrustInspectMsg{item: item} }
				}
			}
			// Click on file tree nodes
			for i := range l.tree.nodes {
				if zone.Get("ftnode-" + itoa(i)).InBounds(msg) {
					l.tree.cursor = i
					l.setDetailFocus(paneItems)
					if l.tree.nodes[i].isDir {
						l.tree.ToggleDir()
					} else {
						l.loadSelectedFile()
					}
					return l, nil
				}
			}
			// Click on pane areas for focus
			if zone.Get("lib-tree").InBounds(msg) {
				l.setDetailFocus(paneItems)
				return l, nil
			}
			if zone.Get("lib-preview").InBounds(msg) {
				l.setDetailFocus(panePreview)
				return l, nil
			}
		}
	}

	// Scroll wheel
	if msg.Action == tea.MouseActionPress {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if l.mode == libraryBrowse {
				l.table.CursorUp()
			} else if l.focus == paneItems {
				l.tree.CursorUp()
				l.loadSelectedFile()
			} else {
				l.preview.ScrollUp()
			}
		case tea.MouseButtonWheelDown:
			if l.mode == libraryBrowse {
				l.table.CursorDown()
			} else if l.focus == paneItems {
				l.tree.CursorDown()
				l.loadSelectedFile()
			} else {
				l.preview.ScrollDown()
			}
		}
	}

	return l, nil
}

// drillIn enters detail mode for the given item.
func (l *libraryModel) drillIn(item *catalog.ContentItem) {
	l.mode = libraryDetail
	l.detailItem = item
	l.tree = newFileTreeModel(item.Files)
	l.focus = paneItems
	l.tree.focused = true
	l.preview.focused = false
	l.sizeDetailPanes()
	l.loadSelectedFile()
}

// loadSelectedFile loads the file at the tree cursor into the preview.
func (l *libraryModel) loadSelectedFile() {
	if l.detailItem == nil {
		return
	}
	// MOAT-materialized registry item: the cache holds only manifest.json +
	// signature.bundle, not the content tree (content is fetched at install
	// time via SourceURI). Without this branch the preview pane silently
	// renders "(no preview)" with no explanation, which reads as a bug.
	if isUnstagedRegistryItem(l.detailItem) {
		l.preview.fileName = "(not staged)"
		l.preview.offset = 0
		l.preview.lines = unstagedRegistryItemPreview(l.detailItem)
		return
	}
	path := l.tree.SelectedPath()
	if path == "" {
		// Directory selected or empty — show primary file
		primary := catalog.PrimaryFileName(l.detailItem.Files, l.detailItem.Type)
		if primary != "" {
			path = primary
		}
	}
	if path == "" {
		l.preview.lines = nil
		l.preview.fileName = ""
		return
	}
	l.preview.fileName = path
	l.preview.offset = 0
	content, err := catalog.ReadFileContent(l.detailItem.Path, path, 10000)
	if err != nil {
		l.preview.lines = []string{"Error reading file:", err.Error()}
		return
	}
	l.preview.lines = strings.Split(content, "\n")
}

// isUnstagedRegistryItem reports whether an item was synthesized from a MOAT
// manifest (Source is a registry name, but the cache has no on-disk content
// tree to read). The discriminator is empty Path with non-empty Source —
// scanner-discovered items always have a Path; materialized items never do.
func isUnstagedRegistryItem(item *catalog.ContentItem) bool {
	return item != nil && item.Path == "" && item.Source != ""
}

// unstagedRegistryItemPreview returns the placeholder body shown in the
// preview pane for a MOAT-materialized item that has no on-disk files yet.
// Spelled out so the user understands the empty pane is by design and knows
// the next step.
func unstagedRegistryItemPreview(item *catalog.ContentItem) []string {
	return []string{
		"This item is published by the registry but its files have not",
		"been fetched yet.",
		"",
		"Source registry: " + item.Source,
		"",
		"Press [i] to install — the registry will fetch the content blob,",
		"verify its attestation, and stage it under the appropriate",
		"provider directory.",
	}
}

// sizeDetailPanes calculates sizes for the detail mode (tree + preview).
func (l *libraryModel) sizeDetailPanes() {
	treeOuterW := l.detailTreeWidth()
	previewOuterW := l.width - treeOuterW
	paneH := max(0, l.height-l.metaBarTotal())
	innerH := max(0, paneH-borderSize)

	l.tree.SetSize(max(0, treeOuterW-borderSize), innerH)
	l.preview.SetSize(max(0, previewOuterW-borderSize), innerH)
}

// setDetailFocus switches focus between tree and preview in detail mode.
func (l *libraryModel) setDetailFocus(pane explorerPane) {
	l.focus = pane
	l.tree.focused = pane == paneItems
	l.preview.focused = pane == panePreview
}

// detailTreeWidth returns the outer width of the file tree pane.
func (l libraryModel) detailTreeWidth() int {
	if l.width >= 120 {
		return 35
	}
	return max(22, l.width*30/100)
}

// View renders the Library view based on current mode.
func (l libraryModel) View() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}

	switch l.mode {
	case libraryDetail:
		return l.viewDetail()
	default:
		return l.viewBrowse()
	}
}

// metaBarLinesBase is the fixed number of content lines every metadata
// panel emits. The layout is stable across content types and trust state:
//
//	Line 1: name, type, files, origin, installed
//	Line 2: scope, registry, path
//	Line 3: type-specific handler detail (blank when not applicable)
//	Line 4: trust + visibility + action buttons (on one row)
//
// Line 3 is reserved whether or not the current content type supplies a
// handler row — skills/commands emit a blank line there so buttons stay
// pinned to Line 4. MOAT trust state (tier, revoked, private) collapses
// into Line 4 alongside the buttons via fixed-position chips; extended
// revocation details live in the Trust Inspector modal (opened via [t] or a
// click on the Trust value). The shared border separator (├────┤) is
// drawn by the view, not counted here.
const metaBarLinesBase = 4

// viewBrowse renders a unified panel: metadata section + separator + table.
func (l libraryModel) viewBrowse() string {
	l.table.focused = true
	innerW := l.width - borderSize
	innerH := l.height - borderSize

	if l.table.Len() == 0 {
		l.table.SetSize(innerW, innerH)
		return borderedPanel(l.table.View(), innerW, innerH, focusedBorderFg)
	}

	// metadata (3-5 lines) + separator (1 line) + table (rest)
	sepLines := 1
	tableH := max(3, innerH-l.metaBarLines()-sepLines)
	l.table.SetSize(innerW, tableH)

	metaContent := l.renderMetadataContent(innerW)
	separator := sectionRuleStyle.Render("├" + strings.Repeat("─", innerW) + "┤")
	tableContent := l.table.View()

	// Build unified panel manually: top border + meta + separator + table + bottom border
	topBorder := sectionRuleStyle.Render("╭" + strings.Repeat("─", innerW) + "╮")
	bottomBorder := sectionRuleStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	// Wrap each metadata and table line with side borders
	wrapLine := func(s string) string {
		s = lipgloss.NewStyle().MaxWidth(innerW).Render(s)
		if gap := innerW - lipgloss.Width(s); gap > 0 {
			s += strings.Repeat(" ", gap)
		}
		return sectionRuleStyle.Render("│") + s + sectionRuleStyle.Render("│")
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, wrapLine(ml))
	}
	lines = append(lines, separator)
	for _, tl := range strings.Split(tableContent, "\n") {
		lines = append(lines, wrapLine(tl))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// viewDetail renders a unified frame: metadata + separator with T-junction + tree|preview.
func (l libraryModel) viewDetail() string {
	innerW := l.width - borderSize
	totalInnerH := l.height - borderSize

	// Metadata gets metaBarLines (3-5 per item), separator gets 1, panes get the rest
	paneH := max(3, totalInnerH-l.metaBarLines()-1)

	treeOuterW := l.detailTreeWidth()
	treeInnerW := max(0, treeOuterW-1) // -1 for the vertical divider
	previewInnerW := max(0, innerW-treeInnerW-1)

	metaContent := l.renderMetadataContent(innerW)

	// Build separator with T-junction: ├──────┬──────────────────┤
	sepLeft := strings.Repeat("─", treeInnerW)
	sepRight := strings.Repeat("─", previewInnerW)
	separator := sectionRuleStyle.Render("├" + sepLeft + "┬" + sepRight + "┤")

	// Build tree content
	treeContentH := max(0, paneH)
	closeBtn := zone.Mark("lib-close", mutedStyle.Render("[x] Close"))
	treeViewH := max(0, treeContentH-1) // -1 for close button
	l.tree.SetSize(treeInnerW, treeViewH)
	treeLines := strings.Split(l.tree.View(), "\n")
	// Pad tree to exact height and append close button
	for len(treeLines) < treeViewH {
		treeLines = append(treeLines, strings.Repeat(" ", treeInnerW))
	}
	if len(treeLines) > treeViewH {
		treeLines = treeLines[:treeViewH]
	}
	treeLines = append(treeLines, closeBtn)

	// Build preview content
	fileCount := ""
	if l.detailItem != nil {
		fileCount = fmt.Sprintf(" (%d files)", len(l.detailItem.Files))
	}
	previewHeader := renderSectionTitle(l.preview.fileName+fileCount, previewInnerW)
	previewViewH := max(0, paneH-1) // -1 for header
	l.preview.SetSize(previewInnerW, previewViewH)
	previewBody := l.renderPreviewBody(previewViewH, previewInnerW)
	previewLines := []string{previewHeader}
	previewLines = append(previewLines, strings.Split(previewBody, "\n")...)
	for len(previewLines) < paneH {
		previewLines = append(previewLines, strings.Repeat(" ", previewInnerW))
	}
	if len(previewLines) > paneH {
		previewLines = previewLines[:paneH]
	}

	// Assemble the frame
	border := sectionRuleStyle.Render
	topBorder := border("╭" + strings.Repeat("─", innerW) + "╮")
	bottomLeft := strings.Repeat("─", treeInnerW)
	bottomRight := strings.Repeat("─", previewInnerW)
	bottomBorder := border("╰" + bottomLeft + "┴" + bottomRight + "╯")

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if g := w - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, border("│")+wrapLine(ml, innerW)+border("│"))
	}
	lines = append(lines, separator)
	for i := 0; i < paneH; i++ {
		tl := ""
		if i < len(treeLines) {
			tl = treeLines[i]
		}
		pl := ""
		if i < len(previewLines) {
			pl = previewLines[i]
		}
		lines = append(lines, border("│")+wrapLine(tl, treeInnerW)+border("│")+wrapLine(pl, previewInnerW)+border("│"))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// renderMetadataContent returns exactly metaBarLines (3) lines of metadata text.
// Delegates to the shared renderMetaPanel function.
func (l libraryModel) renderMetadataContent(width int) string {
	item := l.table.Selected()
	if item == nil {
		return renderMetaPanel(nil, metaPanelData{}, width)
	}
	row := l.table.rows[l.table.cursor]
	canInstall := false
	if item.Library || item.Registry == "" || isUnstagedRegistryItem(item) {
		for _, prov := range l.table.providers {
			if prov.Detected && installer.CheckStatus(*item, prov, l.table.repoRoot) != installer.StatusInstalled {
				canInstall = true
				break
			}
		}
	}
	return renderMetaPanel(item, metaPanelData{
		installed:   row.installed,
		typeDetail:  row.typeDetail,
		canInstall:  canInstall,
		ruleRecords: l.ruleRecordsFor(item),
	}, width)
}

// ruleRecordsFor builds the D16 per-target breakdown for a rule item by
// filtering installed.RuleAppends to records matching the item's LibraryID
// and looking up each record's PerTargetState in verification.PerRecord.
// Returns nil for non-rule items, nil installed, or items with an empty ID
// (metapanel renders the base 4 lines with no breakdown section).
func (l libraryModel) ruleRecordsFor(item *catalog.ContentItem) []ruleTargetStatus {
	if item == nil || item.Type != catalog.Rules || l.installed == nil {
		return nil
	}
	if item.Meta == nil || item.Meta.ID == "" {
		return nil
	}
	libID := item.Meta.ID
	var out []ruleTargetStatus
	for _, r := range l.installed.RuleAppends {
		if r.LibraryID != libID {
			continue
		}
		var state installcheck.PerTargetState
		if l.verification != nil {
			state = l.verification.PerRecord[installcheck.RecordKey{LibraryID: libID, TargetFile: r.TargetFile}]
		}
		out = append(out, ruleTargetStatus{TargetFile: r.TargetFile, State: state})
	}
	return out
}

// truncateMiddle keeps the first 2 path segments and last 3 segments,
// replacing the middle with "…". Returns the path unchanged if it fits.
func truncateMiddle(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	sep := "/"
	parts := strings.Split(path, sep)
	if len(parts) <= 5 {
		// Not enough segments to split meaningfully — truncate end
		return truncate(path, maxWidth)
	}

	head := strings.Join(parts[:2], sep)            // first 2 segments
	tail := strings.Join(parts[len(parts)-3:], sep) // last 3 segments
	result := head + "/…/" + tail

	if len(result) <= maxWidth {
		return result
	}
	// Still too long — truncate the tail
	return truncate(result, maxWidth)
}

// homeDir returns the user's home directory path, cached for rendering.
func homeDir() (string, error) {
	return os.UserHomeDir()
}

// renderPreviewBody renders just the preview content lines (no header).
func (l libraryModel) renderPreviewBody(height, width int) string {
	if len(l.preview.lines) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Select a file to preview")
	}

	linesAbove := l.preview.offset
	lastVisible := min(l.preview.offset+height, len(l.preview.lines))
	linesBelow := max(0, len(l.preview.lines)-lastVisible)
	showAbove := linesAbove > 0
	showBelow := linesBelow > 0

	contentStart := l.preview.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++
	}
	if showBelow && contentEnd > contentStart {
		contentEnd--
	}

	lineNumW := len(fmt.Sprintf("%d", len(l.preview.lines)))
	if lineNumW < 2 {
		lineNumW = 2
	}

	lines := make([]string, 0, height)

	if showAbove {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more above)", linesAbove)))
	}

	for i := contentStart; i < contentEnd; i++ {
		num := mutedStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		numW := lipgloss.Width(num)
		lineW := width - numW
		line := truncateLine(l.preview.lines[i], lineW)
		lines = append(lines, num+line)
	}

	if showBelow {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more below)", linesBelow)))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}
