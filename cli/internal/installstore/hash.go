package installstore

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moathash"
)

// HashContent returns the canonical content hash for an installed item's
// library source, formatted "sha256:<64 lowercase hex>".
func HashContent(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symlink rejected: %s", path)
	}
	if info.IsDir() {
		return moat.ContentHash(path)
	}
	if info.Mode().IsRegular() {
		hash, err := moathash.FileHash(path)
		if err != nil {
			return "", err
		}
		return "sha256:" + hash, nil
	}
	return "", fmt.Errorf("unsupported content path type: %s", path)
}
