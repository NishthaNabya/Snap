// Package dotenv implements the StateDriver for .env flat files.
// It has PriorityEnvironment, ensuring it executes before any
// database driver during both save and restore.
package dotenv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/NishthaNabya/Snap-CLI/snap"
)

type driver struct{}

func init() {
	snap.Registry.Register(&driver{})
}

func (d *driver) Name() string                  { return "dotenv" }
func (d *driver) Priority() snap.DriverPriority { return snap.PriorityEnvironment }

// Capture opens the .env file and returns it as a stream.
func (d *driver) Capture(_ context.Context, source string) (io.ReadCloser, snap.CaptureMetadata, error) {
	f, err := os.Open(source)
	if err != nil {
		return nil, nil, fmt.Errorf("dotenv: open: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("dotenv: stat: %w", err)
	}

	meta := snap.CaptureMetadata{
		"size_bytes": info.Size(),
	}
	return f, meta, nil
}

// Restore writes the blob to the source path atomically
// (temp file + rename).
func (d *driver) Restore(_ context.Context, source string, blob io.Reader) error {
	dir := filepath.Dir(source)
	tmpFile, err := os.CreateTemp(dir, ".snap-dotenv-*")
	if err != nil {
		return fmt.Errorf("dotenv: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmpFile, blob); err != nil {
		return fmt.Errorf("dotenv: write: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("dotenv: fsync: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, source); err != nil {
		return fmt.Errorf("dotenv: rename: %w", err)
	}

	success = true
	return nil
}

// Verify hashes the current file and compares to expectedHash.
func (d *driver) Verify(_ context.Context, source string, expectedHash string) (bool, error) {
	f, err := os.Open(source)
	if err != nil {
		return false, fmt.Errorf("dotenv: open for verify: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false, fmt.Errorf("dotenv: hash: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	return actual == expectedHash, nil
}
