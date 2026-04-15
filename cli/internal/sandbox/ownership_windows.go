//go:build windows

package sandbox

import "os"

func isOwnedByCurrentUser(_ os.FileInfo) bool {
	return true
}
