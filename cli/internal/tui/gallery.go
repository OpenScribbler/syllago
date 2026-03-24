package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// sidebarMinWidth is the minimum width for the contents sidebar.
const sidebarMinWidth = 20

// narrowThreshold is the width below which the sidebar is hidden.
const narrowThreshold = 65

// galleryModel combines a card grid (left ~70-75%) with a contents sidebar
// (right ~25-30%). At narrow widths (< 65), the sidebar is hidden and Enter
// drills into the card instead.
type galleryModel struct {
	cards          cardGridModel
	contents       contentsModel
	collectionType catalog.ContentType // Library, Registries, Loadouts
	focusGrid      bool                // true = grid focused, false = sidebar focused
	width          int
	height         int
}

// newGalleryModel creates a gallery layout with card grid and contents sidebar.
//
// Width allocation: the card grid gets ~70-75% of the width, and the contents
// sidebar gets the remainder. At widths < 65, the sidebar is hidden entirely.
func newGalleryModel(cards []cardData, ct catalog.ContentType, width, height int) galleryModel {
	gridW, sidebarW := gallerySplit(width)

	// Build initial contents from the first card (if any)
	var contents contentsModel
	if len(cards) > 0 {
		contents = newContentsModel(
			contentsTitle(cards[0]),
			cards[0].contents,
		)
	} else {
		contents = newContentsModel("Contents (0)", nil)
	}
	contents.width = sidebarW
	contents.height = height

	grid := newCardGridModel(cards)
	grid.width = gridW
	grid.height = height

	return galleryModel{
		cards:          grid,
		contents:       contents,
		collectionType: ct,
		focusGrid:      true,
		width:          width,
		height:         height,
	}
}

// showSidebar returns true if there's enough width for the contents sidebar.
func (m galleryModel) showSidebar() bool {
	return m.width >= narrowThreshold
}

// gallerySplit calculates the width allocation for grid and sidebar.
// Returns (gridWidth, sidebarWidth). If width < narrowThreshold, sidebar gets 0.
func gallerySplit(width int) (int, int) {
	if width < narrowThreshold {
		return width, 0
	}
	// Sidebar gets ~28% of width, minimum sidebarMinWidth
	sidebarW := width * 28 / 100
	if sidebarW < sidebarMinWidth {
		sidebarW = sidebarMinWidth
	}
	gridW := width - sidebarW - 1 // 1 for the separator column
	if gridW < 20 {
		gridW = 20
	}
	return gridW, sidebarW
}

// contentsTitle builds the sidebar title from the selected card's contents.
func contentsTitle(card cardData) string {
	total := 0
	for _, g := range card.contents {
		total += len(g.items)
	}
	return fmt.Sprintf("Contents (%d)", total)
}

// selectedCardContents returns the contents for the currently selected card.
// Returns nil groups if the grid is empty or the card has no contents.
func (m galleryModel) selectedCardContents() (string, []contentGroup) {
	if len(m.cards.cards) == 0 {
		return "Contents (0)", nil
	}
	card := m.cards.cards[m.cards.cursor]
	return contentsTitle(card), card.contents
}

// Update handles input for the gallery layout. Tab switches focus between
// the card grid and contents sidebar. Arrow keys navigate within the focused
// component. Card selection changes update the sidebar contents.
func (m galleryModel) Update(msg tea.Msg) (galleryModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Tab switches focus between grid and sidebar
		if key.Matches(msg, keys.Tab) && m.showSidebar() {
			m.focusGrid = !m.focusGrid
			return m, nil
		}

		// Route input to the focused component
		if m.focusGrid {
			prevCursor := m.cards.cursor
			var cmd tea.Cmd
			m.cards, cmd = m.cards.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			// If cursor moved, update sidebar contents
			if m.cards.cursor != prevCursor && len(m.cards.cards) > 0 {
				card := m.cards.cards[m.cards.cursor]
				m.contents = newContentsModel(
					contentsTitle(card),
					card.contents,
				)
				_, sidebarW := gallerySplit(m.width)
				m.contents.width = sidebarW
				m.contents.height = m.height
			}
		} else {
			var cmd tea.Cmd
			m.contents, cmd = m.contents.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case tea.MouseMsg:
		// Route mouse events to the focused component
		if m.focusGrid {
			var cmd tea.Cmd
			m.cards, cmd = m.cards.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			var cmd tea.Cmd
			m.contents, cmd = m.contents.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	default:
		// Route other messages to both components
		var cmd tea.Cmd
		m.cards, cmd = m.cards.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.contents, cmd = m.contents.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the gallery layout with the card grid on the left and the
// contents sidebar on the right, separated by a vertical line.
func (m galleryModel) View() string {
	gridView := m.cards.View()

	if !m.showSidebar() {
		// At narrow widths, only show the card grid
		return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(gridView)
	}

	sidebarView := m.contents.View()

	// Separator column: vertical bar for each line of height
	sepStyle := lipgloss.NewStyle().Foreground(borderColor)
	var sepLines []string
	for i := 0; i < m.height; i++ {
		sepLines = append(sepLines, sepStyle.Render("\u2502"))
	}
	separator := strings.Join(sepLines, "\n")

	// Join grid | separator | sidebar horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, gridView, separator, sidebarView)
}
