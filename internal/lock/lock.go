// Package lock provides a small PID-file lock so only one shoal engine runs
// against the persisted queue at a time.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// HeldError reports that a lock is owned by a process that appears alive.
type HeldError struct {
	PID int
}

func (e HeldError) Error() string { return fmt.Sprintf("lock held by pid %d", e.PID) }

// Lock is an acquired PID-file lock.
type Lock struct {
	path string
	file *os.File
}

// Acquire creates path exclusively, writing the current PID into it. If an
// existing lock points at a dead process, it is removed and acquisition is
// retried once.
func Acquire(path string) (*Lock, error) {
	l, err := acquire(path)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	pid, ok := readPID(path)
	if ok && processAlive(pid) {
		return nil, HeldError{PID: pid}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return acquire(path)
}

func acquire(path string) (*Lock, error) {
	if path == "" {
		return nil, fmt.Errorf("empty lock path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(f, os.Getpid()); err != nil {
		f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return &Lock{path: path, file: f}, nil
}

func readPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil || errors.Is(err, syscall.EPERM) {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	// Some platforms don't support signal 0 through os.Process.Signal. When the
	// answer is ambiguous, treat the process as alive to avoid stealing a lock.
	if runtime.GOOS == "windows" {
		return !strings.Contains(strings.ToLower(err.Error()), "no such process")
	}
	return false
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	if l.path == "" {
		return nil
	}
	err := os.Remove(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
