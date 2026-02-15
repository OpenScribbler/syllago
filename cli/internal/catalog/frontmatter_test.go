package catalog

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantDesc  string
		wantProvs []string
		wantErr   bool
	}{
		{
			name:     "valid name and description",
			input:    "---\nname: my-skill\ndescription: A useful skill\n---\n",
			wantName: "my-skill",
			wantDesc: "A useful skill",
		},
		{
			name:      "with providers list",
			input:     "---\nname: my-app\ndescription: An app\nproviders:\n  - claude-code\n  - gemini-cli\n---\n",
			wantName:  "my-app",
			wantDesc:  "An app",
			wantProvs: []string{"claude-code", "gemini-cli"},
		},
		{
			name:    "no opening delimiter",
			input:   "name: my-skill\ndescription: oops\n---\n",
			wantErr: true,
		},
		{
			name:    "no closing delimiter",
			input:   "---\nname: my-skill\ndescription: oops\n",
			wantErr: true,
		},
		{
			name:     "CRLF line endings normalized",
			input:    "---\r\nname: crlf-skill\r\ndescription: Windows style\r\n---\r\n",
			wantName: "crlf-skill",
			wantDesc: "Windows style",
		},
		{
			name:     "closing delimiter at EOF without trailing newline",
			input:    "---\nname: eof-skill\ndescription: No trailing newline\n---",
			wantName: "eof-skill",
			wantDesc: "No trailing newline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := ParseFrontmatter([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", fm.Name, tt.wantName)
			}
			if fm.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", fm.Description, tt.wantDesc)
			}
			if tt.wantProvs != nil {
				if len(fm.Providers) != len(tt.wantProvs) {
					t.Fatalf("Providers length = %d, want %d", len(fm.Providers), len(tt.wantProvs))
				}
				for i, p := range fm.Providers {
					if p != tt.wantProvs[i] {
						t.Errorf("Providers[%d] = %q, want %q", i, p, tt.wantProvs[i])
					}
				}
			}
		})
	}
}

func TestParseFrontmatterWithBody(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantDesc string
		wantBody string
		wantErr  bool
	}{
		{
			name:     "body text after frontmatter",
			input:    "---\nname: my-skill\ndescription: A skill\n---\nThis is the body content.\n\nIt has multiple paragraphs.",
			wantName: "my-skill",
			wantDesc: "A skill",
			wantBody: "This is the body content.\n\nIt has multiple paragraphs.",
		},
		{
			name:     "empty body after frontmatter",
			input:    "---\nname: empty-body\ndescription: No body here\n---\n",
			wantName: "empty-body",
			wantDesc: "No body here",
			wantBody: "",
		},
		{
			name:    "no opening delimiter returns error",
			input:   "name: bad\n---\n",
			wantErr: true,
		},
		{
			name:    "no closing delimiter returns error",
			input:   "---\nname: bad\n",
			wantErr: true,
		},
		{
			name:     "CRLF normalized with body",
			input:    "---\r\nname: crlf\r\ndescription: Win\r\n---\r\nBody here.",
			wantName: "crlf",
			wantDesc: "Win",
			wantBody: "Body here.",
		},
		{
			name:     "closing delimiter at EOF without trailing newline",
			input:    "---\nname: eof\ndescription: At end\n---",
			wantName: "eof",
			wantDesc: "At end",
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := ParseFrontmatterWithBody([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", fm.Name, tt.wantName)
			}
			if fm.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", fm.Description, tt.wantDesc)
			}
			if body != tt.wantBody {
				t.Errorf("Body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
