package installer

import "testing"

func TestStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{"installed renders with text label", StatusInstalled, "[ok]"},
		{"not installed renders with text label", StatusNotInstalled, "[--]"},
		{"not available renders with text label", StatusNotAvailable, "[-]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("Status.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
