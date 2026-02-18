package tui

// provCheckModel groups the provider checkbox state for the Install tab.
type provCheckModel struct {
	checks []bool
	cursor int
}
