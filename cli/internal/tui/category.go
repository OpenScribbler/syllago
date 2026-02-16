package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

type categoryModel struct {
	types      []catalog.ContentType
	counts     map[catalog.ContentType]int
	localCount int    // total local items across all types
	cursor     int
	message    string // transient success message (e.g., after import)

	// Version and update state
	version         string // local version (from build-time ldflags)
	remoteVersion   string // latest version on origin
	updateAvailable bool
	commitsBehind   int
}

func newCategoryModel(cat *catalog.Catalog, version string) categoryModel {
	return categoryModel{
		types:      catalog.AllContentTypes(),
		counts:     cat.CountByType(),
		localCount: cat.CountLocal(),
		version:    version,
	}
}

func (m categoryModel) Update(msg tea.Msg) (categoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = "" // clear transient message on any keypress
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.types)+3 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m categoryModel) View() string {
	s := titleStyle.Render("nesco")
	if m.version != "" {
		s += " " + versionStyle.Render("v"+m.version)
	}
	s += "\n"
	s += helpStyle.Render("Browse and install AI coding tool content") + "\n\n"

	for i, ct := range m.types {
		count := m.counts[ct]
		prefix := "   "
		style := itemStyle

		if i == m.cursor {
			prefix = " ▸ "
			style = selectedItemStyle
		}

		label := ct.Label()
		line := fmt.Sprintf("%s%s %s", prefix, style.Render(label), countStyle.Render(fmt.Sprintf("(%d)", count)))
		s += line + "\n"
	}

	// My Tools item (visually separated)
	s += "\n"
	myToolsPrefix := "   "
	myToolsStyle := itemStyle
	if m.cursor == len(m.types) {
		myToolsPrefix = " ▸ "
		myToolsStyle = selectedItemStyle
	}
	myToolsLabel := fmt.Sprintf("My Tools (%d)", m.localCount)
	s += myToolsPrefix + myToolsStyle.Render(myToolsLabel) + "\n"

	// Import item
	importPrefix := "   "
	importStyle := itemStyle
	if m.cursor == len(m.types)+1 {
		importPrefix = " ▸ "
		importStyle = selectedItemStyle
	}
	s += importPrefix + importStyle.Render("Import an AI tool...") + "\n"

	// Update item
	updatePrefix := "   "
	updateStyle := itemStyle
	if m.cursor == len(m.types)+2 {
		updatePrefix = " ▸ "
		updateStyle = selectedItemStyle
	}
	updateLabel := "Update nesco..."
	if m.updateAvailable {
		updateLabel = fmt.Sprintf("Update nesco to v%s...", m.remoteVersion)
	}
	s += updatePrefix + updateStyle.Render(updateLabel) + "\n"

	// Settings item
	settingsPrefix := "   "
	settingsStyle := itemStyle
	if m.cursor == len(m.types)+3 {
		settingsPrefix = " ▸ "
		settingsStyle = selectedItemStyle
	}
	s += settingsPrefix + settingsStyle.Render("Settings...") + "\n"

	// Update notification banner
	if m.updateAvailable {
		s += "\n" + updateBannerStyle.Render(fmt.Sprintf("  ✦ A new version is available (v%s)", m.remoteVersion)) + "\n"
	}

	if m.message != "" {
		s += successMsgStyle.Render(m.message) + "\n"
	}

	// Show empty catalog guidance
	allZero := true
	for _, ct := range m.types {
		if m.counts[ct] > 0 {
			allZero = false
			break
		}
	}
	if allZero {
		s += "\n" + helpStyle.Render("Your catalog is empty. Use Import to add content.") + "\n"
	}

	s += "\n" + helpStyle.Render("↑↓ navigate • enter select • / search • q quit")
	return s
}

func (m categoryModel) isMyToolsSelected() bool {
	return m.cursor == len(m.types)
}

func (m categoryModel) isImportSelected() bool {
	return m.cursor == len(m.types)+1
}

func (m categoryModel) isUpdateSelected() bool {
	return m.cursor == len(m.types)+2
}

func (m categoryModel) isSettingsSelected() bool {
	return m.cursor == len(m.types)+3
}

func (m categoryModel) selectedType() catalog.ContentType {
	if m.cursor >= len(m.types) {
		return "" // shouldn't be called when on My Tools or Import
	}
	return m.types[m.cursor]
}
