package parse

import (
	"strings"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		wantYAML string
		wantBody string
		wantOK   bool
	}{
		{
			name:     "well-formed",
			in:       "---\nname: x\n---\n\nBody text.\n",
			wantYAML: "name: x\n",
			wantBody: "Body text.",
			wantOK:   true,
		},
		{
			name:     "crlf normalized",
			in:       "---\r\nname: x\r\n---\r\nBody.\r\n",
			wantYAML: "name: x\n",
			wantBody: "Body.",
			wantOK:   true,
		},
		{
			name:     "no frontmatter",
			in:       "# Just markdown\n",
			wantBody: "# Just markdown",
			wantOK:   false,
		},
		{
			name:     "unclosed fence falls back to whole content",
			in:       "---\nname: x\nno closing fence",
			wantBody: "---\nname: x\nno closing fence",
			wantOK:   false,
		},
		{
			name:     "bare closing fence at EOF not recognized",
			in:       "---\nname: x\n---",
			wantBody: "---\nname: x\n---",
			wantOK:   false,
		},
		{
			name:     "empty frontmatter block",
			in:       "---\n---\nBody.",
			wantYAML: "",
			wantBody: "Body.",
			wantOK:   true,
		},
		{
			name:     "empty body",
			in:       "---\nname: x\n---\n",
			wantYAML: "name: x\n",
			wantBody: "",
			wantOK:   true,
		},
		{
			name:     "fence not at byte zero",
			in:       "\n---\nkey: v\n---\nbody\n",
			wantBody: "---\nkey: v\n---\nbody",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			yamlBytes, body, ok := SplitFrontmatter([]byte(tt.in))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && string(yamlBytes) != tt.wantYAML {
				t.Errorf("yaml = %q, want %q", yamlBytes, tt.wantYAML)
			}
			if !ok && yamlBytes != nil {
				t.Errorf("yaml must be nil when ok is false, got %q", yamlBytes)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if strings.Contains(body, "\r") {
				t.Error("body must not contain CR after normalization")
			}
		})
	}
}
