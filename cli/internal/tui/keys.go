package tui

// Key constants for the TUI. Used in app.go Update() via msg.String() comparisons.
// A structured keyMap with bubbles/key.Binding will be added when the help overlay
// (Phase 7) needs to enumerate all bindings programmatically.

// Key names used in Update() handlers. Defined here to avoid magic strings.
const (
	keyQuit      = "q"
	keySearch    = "/"
	keyGroup1    = "1"
	keyGroup2    = "2"
	keyGroup3    = "3"
	keyLeft      = "h"
	keyRight     = "l"
	keyUp        = "k"
	keyDown      = "j"
	keyAdd       = "a"
	keyCreate    = "n"
	keyEdit      = "e"
	keyRemove    = "d"
	keyUninstall = "x"
	keyInstall   = "i"
	keyRefresh   = "R"
	keySync      = "S"
	keyHelp      = "?"
	keyTrust     = "t"
	keyFilter    = "f"
)
