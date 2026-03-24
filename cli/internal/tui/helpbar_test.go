package tui

import (
	"strings"
	"testing"
)

func TestHelpBarView(t *testing.T) {
	hb := helpBarModel{version: "v1.0.0", width: 80}
	hints := explorerHints()
	result := hb.View(hints)

	if !strings.Contains(result, "syllago v1.0.0") {
		t.Error("help bar should contain version string")
	}
	if !strings.Contains(result, "navigate") {
		t.Error("help bar should contain navigation hint")
	}
}

func TestHelpBarOverflow(t *testing.T) {
	hb := helpBarModel{version: "v1.0.0", width: 30}
	hints := explorerHints()
	result := hb.View(hints)

	// Should still render even at narrow width, just with fewer hints
	if !strings.Contains(result, "syllago v1.0.0") {
		t.Error("help bar should always show version")
	}
}

func TestGalleryHints(t *testing.T) {
	hints := galleryHints()
	if len(hints) == 0 {
		t.Error("galleryHints should return hints")
	}
}
