package installer

import "testing"

func TestIsWindowsMount(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/mnt/c/Users/foo", true},
		{"/mnt/d/some/path", true},
		{"/home/user/.claude", false},
		{"/mnt", false},
		{"/mnt/", false},
		{"", false},
		{"/mnt/c", false},    // no trailing slash after drive letter
		{"/mnt/cc/foo", false}, // two-char mount name — not a Windows drive
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsWindowsMount(tt.path)
			if got != tt.want {
				t.Errorf("IsWindowsMount(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
