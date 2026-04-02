package telemetry

import (
	"os"
	"strings"
)

// isDNTSet returns true if the DO_NOT_TRACK environment variable is set to any
// truthy value: 1, true, yes, on (case-insensitive). Any other value is falsy.
func isDNTSet() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DO_NOT_TRACK")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
