// Package lock provides filesystem-based advisory locking using flock(2).
// This prevents concurrent Snap operations from corrupting the store.
package lock

import (
	"fmt"
	"os"
	"syscall"
)

// Lock represents an acquired file-system advisory lock.
type Lock struct {
	file *os.File
	path string
}

// Acquire attempts to take an exclusive, non-blocking advisory lock
// on the file at path. Returns ErrLocked if the lock is already held
// by another process.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lock: open: %w", err)
	}

	// LOCK_EX = exclusive, LOCK_NB = non-blocking.
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, fmt.Errorf("lock: another snap operation is in progress (%s)", path)
		}
		return nil, fmt.Errorf("lock: flock: %w", err)
	}

	return &Lock{file: f, path: path}, nil
}

// Release releases the advisory lock and closes the file.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	// Unlock then close. Close alone would release, but being explicit
	// avoids relying on GC behavior.
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	err := l.file.Close()
	l.file = nil
	return err
}
