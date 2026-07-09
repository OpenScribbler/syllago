package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// overlay is the shared surface for the App's modal-like components. The App
// hand-wires each modal in four places (key routing, mouse routing, window
// sizing, and view compositing); this interface lets those call sites iterate
// one ordered slice instead of repeating an if-chain per modal.
//
// Contract:
//   - Active reports whether the overlay is currently shown. Only active
//     overlays receive routed input or are composited over the content.
//   - SetSize passes the available content area (full width x contentHeight).
//     Fixed-size overlays (editModal) implement it as a no-op.
//   - View renders the overlay; the App centers it via overlayModal().
//   - routeUpdate adapts each component's concrete-typed
//     `Update(msg) (T, tea.Cmd)` signature: it applies the update in place
//     through the pointer receiver and returns only the command, so message
//     flow is unchanged.
type overlay interface {
	Active() bool
	SetSize(w, h int)
	View() string
	routeUpdate(msg tea.Msg) tea.Cmd
}

// overlays returns the App's modal-like components in priority order. The
// order is load-bearing in three ways:
//   - key/mouse routing: the first active overlay captures the input;
//   - view compositing: later entries render on top of earlier ones.
//
// The telemetry consent modal is intentionally NOT in this slice — it blocks
// every other routing path (including wizards) and renders above everything,
// so App.Update/App.View special-case it before/after this list. The toast is
// also separate: it is not exclusive-capture (it only consumes specific
// keys/clicks and composites bottom-right, not centered).
func (a *App) overlays() []overlay {
	return []overlay{
		&a.modal,
		&a.confirm,
		&a.remove,
		&a.registryAdd,
		&a.tofu,
		&a.trustInspector,
		&a.hint,
		&a.help,
	}
}

// --- editModal ---

func (m editModal) Active() bool { return m.active }

// SetSize is a no-op: the edit modal owns its dimensions (fixed 56-wide by
// default; inline placements clamp via SetWidth at open time). The App has
// never resized it on WindowSizeMsg and must not start doing so.
func (m *editModal) SetSize(int, int) {}

func (m *editModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- confirmModal ---

func (m confirmModal) Active() bool { return m.active }

func (m *confirmModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *confirmModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- removeModal ---

func (m removeModal) Active() bool { return m.active }

func (m *removeModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *removeModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- registryAddModal ---

func (m registryAddModal) Active() bool { return m.active }

func (m *registryAddModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *registryAddModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- tofuModal ---

func (m tofuModal) Active() bool { return m.active }

func (m *tofuModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *tofuModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- trustInspectorModel ---

func (m trustInspectorModel) Active() bool { return m.active }

func (m *trustInspectorModel) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- hintModal ---

func (m hintModal) Active() bool { return m.active }

func (m *hintModal) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	return cmd
}

// --- helpOverlay ---

func (h helpOverlay) Active() bool { return h.active }

func (h *helpOverlay) routeUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*h, cmd = h.Update(msg)
	return cmd
}
