package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireReleaseRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shoal.lock")
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := strings.TrimSpace(string(b)), fmt.Sprint(os.Getpid()); got != want {
		t.Fatalf("lock pid = %q, want %q", got, want)
	}

	if err := l.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file still exists after Release: %v", err)
	}
}

func TestAcquireHeldLockReportsPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shoal.lock")
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release()

	_, err = Acquire(path)
	if err == nil {
		t.Fatal("second Acquire succeeded; want held-lock error")
	}
	var held HeldError
	if !errors.As(err, &held) {
		t.Fatalf("second Acquire error = %T %v, want HeldError", err, err)
	}
	if held.PID != os.Getpid() {
		t.Fatalf("held PID = %d, want %d", held.PID, os.Getpid())
	}
	if !strings.Contains(err.Error(), fmt.Sprint(os.Getpid())) {
		t.Fatalf("held error %q does not include pid", err.Error())
	}
}

func TestAcquireStaleLockSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shoal.lock")
	if err := os.WriteFile(path, []byte("99999999\n"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire stale lock: %v", err)
	}
	defer l.Release()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := strings.TrimSpace(string(b)), fmt.Sprint(os.Getpid()); got != want {
		t.Fatalf("lock pid after stale recovery = %q, want %q", got, want)
	}
}
