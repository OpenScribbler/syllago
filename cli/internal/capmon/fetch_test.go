package capmon_test

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestValidateSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{"valid https", "https://docs.anthropic.com/llms-full.txt", false, ""},
		{"http rejected", "http://example.com", true, "only https scheme allowed"},
		{"raw IPv4", "https://127.0.0.1/path", true, "raw IP literal not allowed"},
		{"raw IPv6", "https://[::1]/path", true, "raw IP literal not allowed"},
		{"loopback hostname", "https://localhost/path", true, "reserved IP"},
		{"link-local", "https://169.254.169.254/latest/meta-data", true, "raw IP literal not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := capmon.ValidateSourceURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSourceURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
