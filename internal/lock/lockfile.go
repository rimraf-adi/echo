package lock

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	ErrLockTimeout = errors.New("timed out waiting for workspace lock")
)

// Lock represents an acquired exclusive lock
type Lock struct {
	path string
	file *os.File
}

// Acquire attempts to acquire an exclusive lockfile with timeout and stale PID detection
func Acquire(lockPath string, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	backoff := 10 * time.Millisecond

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Lock acquired! Write our PID
			pid := os.Getpid()
			_, _ = f.WriteString(fmt.Sprintf("%d\n", pid))
			_ = f.Sync()
			return &Lock{path: lockPath, file: f}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("opening lockfile: %w", err)
		}

		// Check if existing lock is stale
		if isLockStale(lockPath) {
			_ = os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			return nil, ErrLockTimeout
		}

		time.Sleep(backoff)
		if backoff < 200*time.Millisecond {
			backoff *= 2
		}
	}
}

// Release releases and removes the lockfile
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isLockStale(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		// Empty lockfile older than 2 seconds is stale
		info, serr := os.Stat(lockPath)
		if serr == nil && time.Since(info.ModTime()) > 2*time.Second {
			return true
		}
		return false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return true
	}

	// Check if process with pid is still alive
	process, err := os.FindProcess(pid)
	if err != nil {
		return true
	}

	// On Unix, sending signal 0 checks if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process is dead
		return true
	}

	return false
}
