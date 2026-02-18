package tui

// fileViewerModel groups the file viewer state for the Files tab.
type fileViewerModel struct {
	cursor       int
	content      string
	scrollOffset int
	viewing      bool // true when viewing file content (not file list)
}
