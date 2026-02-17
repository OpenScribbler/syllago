package tui

import "testing"

func TestMessagePrefixes(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		isError    bool
		wantPrefix string
	}{
		{"error message has Error: prefix", "installation failed", true, "Error:"},
		{"success message has Done: prefix", "installed successfully", false, "Done:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp(t)
			app = pressN(app, keyEnter, 1) // → items
			app = pressN(app, keyEnter, 1) // → detail

			app.detail.message = tt.message
			app.detail.messageIsErr = tt.isError

			view := app.detail.View()
			assertContains(t, view, tt.wantPrefix+" "+tt.message)
		})
	}
}
