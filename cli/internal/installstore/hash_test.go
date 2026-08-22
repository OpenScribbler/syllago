package installstore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

var installStoreHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func TestHashContent(t *testing.T) {
	t.Run("directory matches moat content hash", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "SKILL.md"), []byte("# Writer\n"))
		writeFile(t, filepath.Join(dir, "lib", "helper.py"), []byte("print('ok')\n"))

		want, err := moat.ContentHash(dir)
		if err != nil {
			t.Fatalf("moat.ContentHash: %v", err)
		}
		got, err := HashContent(dir)
		if err != nil {
			t.Fatalf("HashContent(dir): %v", err)
		}
		if got != want {
			t.Fatalf("HashContent(dir) = %s, want %s", got, want)
		}
	})

	t.Run("text files are canonicalized and prefixed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		lfPath := filepath.Join(dir, "lf.txt")
		crlfPath := filepath.Join(dir, "crlf.txt")
		writeFile(t, lfPath, []byte("alpha\nbeta\n"))
		writeFile(t, crlfPath, []byte("alpha\r\nbeta\r\n"))

		lfHash, err := HashContent(lfPath)
		if err != nil {
			t.Fatalf("HashContent(lf): %v", err)
		}
		crlfHash, err := HashContent(crlfPath)
		if err != nil {
			t.Fatalf("HashContent(crlf): %v", err)
		}
		if lfHash != crlfHash {
			t.Fatalf("LF hash %s != CRLF hash %s", lfHash, crlfHash)
		}
		if !installStoreHashPattern.MatchString(lfHash) {
			t.Fatalf("hash %q does not match sha256:<64 lowercase hex>", lfHash)
		}
	})

	t.Run("symlink argument returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target.txt")
		link := filepath.Join(dir, "link.txt")
		writeFile(t, target, []byte("target\n"))
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("Symlink not available: %v", err)
		}

		_, err := HashContent(link)
		if err == nil {
			t.Fatal("HashContent(symlink) returned nil error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "symlink") {
			t.Fatalf("HashContent(symlink) error = %q, want symlink mention", err.Error())
		}
	})

	t.Run("missing path returns error", func(t *testing.T) {
		t.Parallel()
		_, err := HashContent(filepath.Join(t.TempDir(), "missing.txt"))
		if err == nil {
			t.Fatal("HashContent(missing) returned nil error")
		}
	})
}
