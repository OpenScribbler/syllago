package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Enter        key.Binding
	Back         key.Binding
	Quit         key.Binding
	Search       key.Binding
	Install      key.Binding
	Uninstall    key.Binding
	Copy         key.Binding
	Share        key.Binding
	Tab          key.Binding
	ShiftTab     key.Binding
	Help         key.Binding
	Home         key.Binding
	End          key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Space        key.Binding
	ConfirmYes   key.Binding
	ConfirmNo    key.Binding
	Dropdown1    key.Binding // Content dropdown
	Dropdown2    key.Binding // Collection dropdown
	Dropdown3    key.Binding // Config dropdown
	EnvSetup     key.Binding
	Save         key.Binding
	Add          key.Binding
	Delete       key.Binding
	Refresh      key.Binding
	ToggleHidden key.Binding
	ToggleAll    key.Binding
	ToggleCompat key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("j/k", "navigate"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/k", "navigate"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("h/l", "pane"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("h/l", "pane"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Install: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "install"),
	),
	Uninstall: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "uninstall"),
	),
	Copy: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "copy"),
	),
	Share: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "share"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Home: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("home", "top"),
	),
	End: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("end", "bottom"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),
	ConfirmYes: key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "confirm"),
	),
	ConfirmNo: key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n", "cancel"),
	),
	Dropdown1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "content"),
	),
	Dropdown2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "collection"),
	),
	Dropdown3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "config"),
	),
	EnvSetup: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "env setup"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "save"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	),
	Delete: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "remove"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sync"),
	),
	ToggleHidden: key.NewBinding(
		key.WithKeys("H"),
		key.WithHelp("H", "show/hide hidden"),
	),
	ToggleAll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "toggle all"),
	),
	ToggleCompat: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle filter"),
	),
}
