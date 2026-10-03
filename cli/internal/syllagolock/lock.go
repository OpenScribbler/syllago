// Package syllagolock serializes every syllago operation that changes install
// state: the install store, installed.json, and syllago's writes to provider
// settings files. One lock covers all three, across goroutines and processes,
// so a CLI command and a GUI or TUI session cannot lose each other's updates.
//
// The lock is not reentrant. Only a top-level entry point acquires it, and
// code holding it never calls another entry point that acquires it.
package syllagolock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// DefaultTimeout is how long Acquire waits for another holder to finish.
// It is a variable so tests can shorten the wait.
var DefaultTimeout = 10 * time.Second

const (
	fileName     = "syllago.lock"
	pollInterval = 50 * time.Millisecond
)

// inProcess serializes goroutines, so each process opens the lock file once
// and the file lock only has to exclude other processes.
var inProcess = make(chan struct{}, 1)

// errWouldBlock reports that another process holds the file lock.
var errWouldBlock = errors.New("lock held by another process")

// Acquire takes the syllago-wide install lock, waiting up to timeout. The
// returned release function is safe to call more than once.
func Acquire(timeout time.Duration) (release func(), err error) {
	dir, err := config.GlobalDirPath()
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrSystemHomedir, "could not locate the syllago directory for the install lock", "Check that your home directory is set", err.Error())
	}
	return acquireAt(filepath.Join(dir, fileName), timeout)
}

func acquireAt(path string, timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case inProcess <- struct{}{}:
	case <-timer.C:
		return nil, busyError()
	}

	f, err := openLockFile(path)
	if err != nil {
		<-inProcess
		return nil, output.NewStructuredErrorDetail(output.ErrSystemIO, "could not open the install lock", "Check permissions on the syllago directory", err.Error())
	}

	for {
		err := tryLock(f)
		if err == nil {
			break
		}
		if !errors.Is(err, errWouldBlock) {
			_ = f.Close()
			<-inProcess
			return nil, output.NewStructuredErrorDetail(output.ErrSystemIO, "could not take the install lock", "Check permissions on the syllago directory", err.Error())
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			<-inProcess
			return nil, busyError()
		}
		time.Sleep(pollInterval)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unlock(f)
			_ = f.Close()
			<-inProcess
		})
	}, nil
}

func openLockFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
}

func busyError() error {
	return output.NewStructuredError(output.ErrSystemLocked, "another syllago process is changing installs", "Wait for it to finish, then run the command again")
}
