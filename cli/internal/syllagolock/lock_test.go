package syllagolock

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func useTempGlobalDir(t *testing.T) string {
	t.Helper()
	orig := config.GlobalDirOverride
	dir := filepath.Join(t.TempDir(), ".syllago")
	config.GlobalDirOverride = dir
	t.Cleanup(func() { config.GlobalDirOverride = orig })
	return dir
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var se output.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v (%T), want StructuredError %s", err, err, code)
	}
	if se.Code != code {
		t.Fatalf("code = %s, want %s", se.Code, code)
	}
}

func TestAcquireCreatesLockFileInGlobalDir(t *testing.T) {
	dir := useTempGlobalDir(t)

	release, err := Acquire(time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Fatalf("lock file missing: %v", err)
	}
}

func TestAcquireSerializesGoroutines(t *testing.T) {
	useTempGlobalDir(t)

	var inside, maxInside atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := Acquire(5 * time.Second)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			n := inside.Add(1)
			for {
				m := maxInside.Load()
				if n <= m || maxInside.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			inside.Add(-1)
			release()
		}()
	}
	wg.Wait()

	if got := maxInside.Load(); got != 1 {
		t.Fatalf("max concurrent holders = %d, want 1", got)
	}
}

func TestAcquireTimesOutWhileGoroutineHolds(t *testing.T) {
	useTempGlobalDir(t)

	release, err := Acquire(time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	_, err = Acquire(100 * time.Millisecond)
	requireCode(t, err, output.ErrSystemLocked)
}

func TestReleaseTwiceIsSafeAndFreesLock(t *testing.T) {
	useTempGlobalDir(t)

	release, err := Acquire(time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	release()

	release2, err := Acquire(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	release2()
}

func TestAcquireUnwritableDirReturnsIOError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	_, err := acquireAt(filepath.Join(parent, "sub", fileName), 100*time.Millisecond)
	requireCode(t, err, output.ErrSystemIO)

	// The failed attempt must not leave the in-process slot taken.
	useTempGlobalDir(t)
	release, err := Acquire(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire after failed attempt: %v", err)
	}
	release()
}

// TestHelperHoldLock runs only as a child process. It takes the lock, prints
// "held", and holds it until its stdin closes.
func TestHelperHoldLock(t *testing.T) {
	path := os.Getenv("SYLLAGOLOCK_HELPER_PATH")
	if path == "" {
		t.Skip("helper process only")
	}
	release, err := acquireAt(path, 5*time.Second)
	if err != nil {
		os.Exit(2)
	}
	os.Stdout.WriteString("held\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	release()
	os.Exit(0)
}

func TestAcquireTimesOutWhileOtherProcessHolds(t *testing.T) {
	dir := useTempGlobalDir(t)
	path := filepath.Join(dir, fileName)

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	cmd.Env = append(os.Environ(), "SYLLAGOLOCK_HELPER_PATH="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "held\n" {
		t.Fatalf("helper did not take the lock: %q, %v", line, err)
	}

	_, err = Acquire(200 * time.Millisecond)
	requireCode(t, err, output.ErrSystemLocked)

	// Once the other process lets go, Acquire succeeds.
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper exit: %v", err)
	}
	release, err := Acquire(time.Second)
	if err != nil {
		t.Fatalf("Acquire after helper exit: %v", err)
	}
	release()
}
