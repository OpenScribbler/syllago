package registry

import "testing"

func TestNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:acme/nesco-tools.git", "nesco-tools"},
		{"https://github.com/acme/nesco-tools.git", "nesco-tools"},
		{"https://github.com/acme/nesco-tools", "nesco-tools"},
		{"https://github.com/acme/nesco-tools/", "nesco-tools"},
		{"git@github.com:acme/my_tools.git", "my_tools"},
	}
	for _, tt := range tests {
		got := NameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("NameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
