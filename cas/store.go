// Package cas implements a Content-Addressable Store backed by the
// local filesystem. Blobs are stored in a sharded directory structure
// under .snap/objects/ and named by their SHA-256 hex digest.
package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Store manages the Content-Addressable Store.
type Store struct {
	root string // e.g., ".snap/objects"
	tmp  string // e.g., ".snap/tmp"
}

// NewStore creates a Store rooted at objectsDir with tmpDir for staging.
// Both directories must already exist.
func NewStore(objectsDir, tmpDir string) *Store {
	return &Store{root: objectsDir, tmp: tmpDir}
}

// Put writes a blob to the CAS atomically.
// Returns the SHA-256 hex hash and size of the stored blob.
// If the blob already exists (dedup hit), the temp file is discarded.
func (s *Store) Put(r io.Reader) (hash string, size int64, err error) {
	// Step 1: Create a temp file in the staging area.
	tmpFile, err := os.CreateTemp(s.tmp, "snap-blob-*")
	if err != nil {
		return "", 0, fmt.Errorf("cas: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on any error path.
	success := false
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Step 2: Stream content to disk while computing SHA-256.
	// Memory usage is bounded by io.Copy's internal 32KB buffer.
	hasher := sha256.New()
	tee := io.TeeReader(r, hasher)
	size, err = io.Copy(tmpFile, tee)
	if err != nil {
		return "", 0, fmt.Errorf("cas: write blob: %w", err)
	}

	// Step 3: Sync to disk. Ensures data survives power loss.
	if err := tmpFile.Sync(); err != nil {
		return "", 0, fmt.Errorf("cas: fsync: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", 0, fmt.Errorf("cas: close: %w", err)
	}

	// Step 4: Compute the final hash and derive the object path.
	hash = hex.EncodeToString(hasher.Sum(nil))
	shard := hash[:2]
	objDir := filepath.Join(s.root, shard)
	objPath := filepath.Join(objDir, hash)

	// Step 5: Check for deduplication.
	if _, statErr := os.Stat(objPath); statErr == nil {
		// Blob already exists. Discard the temp file.
		os.Remove(tmpPath)
		success = true
		return hash, size, nil
	}

	// Step 6: Ensure the shard directory exists.
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("cas: mkdir shard: %w", err)
	}

	// Step 7: Atomic rename.
	// On POSIX, rename(2) is atomic within the same filesystem.
	if err := os.Rename(tmpPath, objPath); err != nil {
		return "", 0, fmt.Errorf("cas: rename: %w", err)
	}

	// Step 8: Set read-only permissions. Objects are immutable.
	os.Chmod(objPath, 0o444)

	success = true
	return hash, size, nil
}

// Get opens a blob by its hash for streaming reads.
// The caller is responsible for closing the returned ReadCloser.
// Returns an error if the blob does not exist or the integrity
// check fails.
func (s *Store) Get(hash string) (io.ReadCloser, error) {
	if len(hash) < 2 {
		return nil, fmt.Errorf("cas: invalid hash: %q", hash)
	}
	shard := hash[:2]
	objPath := filepath.Join(s.root, shard, hash)

	f, err := os.Open(objPath)
	if err != nil {
		return nil, fmt.Errorf("cas: open blob: %w", err)
	}
	return f, nil
}

// Verify opens a blob and re-hashes it to confirm integrity.
// Returns true if sha256(blob) == hash, false otherwise.
func (s *Store) Verify(hash string) (bool, error) {
	rc, err := s.Get(hash)
	if err != nil {
		return false, err
	}
	defer rc.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, rc); err != nil {
		return false, fmt.Errorf("cas: verify read: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	return actual == hash, nil
}

// Has returns true if a blob with the given hash exists in the store.
func (s *Store) Has(hash string) bool {
	if len(hash) < 2 {
		return false
	}
	shard := hash[:2]
	objPath := filepath.Join(s.root, shard, hash)
	_, err := os.Stat(objPath)
	return err == nil
}

// CleanupOrphans removes temp files older than maxAge from the
// staging directory. This handles orphans left behind by crashes.
func (s *Store) CleanupOrphans(maxAge time.Duration) error {
	entries, err := os.ReadDir(s.tmp)
	if err != nil {
		return fmt.Errorf("cas: read tmp dir: %w", err)
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(s.tmp, e.Name()))
		}
	}
	return nil
}
