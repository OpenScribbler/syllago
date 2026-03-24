package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	// Global
	Quit   key.Binding
	Help   key.Binding
	Search key.Binding

	// Navigation
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Escape key.Binding
	Tab    key.Binding
	Home   key.Binding
	End    key.Binding
	PgUp   key.Binding
	PgDown key.Binding

	// Dropdowns (Phase 2)
	Dropdown1 key.Binding // Content
	Dropdown2 key.Binding // Collection
	Dropdown3 key.Binding // Config

	// Actions (Phase 4+)
	Install      key.Binding
	Uninstall    key.Binding
	Copy         key.Binding
	Share        key.Binding
	Add          key.Binding
	Remove       key.Binding
	Sync         key.Binding
	ToggleHidden key.Binding
}

var keys = keyMap{
	Quit:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),

	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("j/k", "navigate")),
	Down:   key.NewBinding(key.WithKeys("down", "j")),
	Left:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("h/l", "pane")),
	Right:  key.NewBinding(key.WithKeys("right", "l")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
	Home:   key.NewBinding(key.WithKeys("home", "g")),
	End:    key.NewBinding(key.WithKeys("end", "G")),
	PgUp:   key.NewBinding(key.WithKeys("pgup")),
	PgDown: key.NewBinding(key.WithKeys("pgdown")),

	Dropdown1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "content")),
	Dropdown2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "collection")),
	Dropdown3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "config")),

	Install:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install")),
	Uninstall:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "uninstall")),
	Copy:         key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	Share:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "share")),
	Add:          key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Remove:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "remove")),
	Sync:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	ToggleHidden: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "hidden")),
}
