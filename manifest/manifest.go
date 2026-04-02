// Package manifest manages the State Ledger — JSON documents that
// map a Git commit hash to captured state blob hashes.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry represents a single captured state source within a manifest.
type Entry struct {
	Driver   string                 `json:"driver"`
	Source   string                 `json:"source"`
	BlobHash string                `json:"blob_hash"`
	BlobSize int64                  `json:"blob_size"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Manifest is the State Ledger for a single Git commit.
type Manifest struct {
	Schema    string  `json:"$schema"`
	Version   int     `json:"version"`
	GitHash   string  `json:"git_hash"`
	CreatedAt string  `json:"created_at"`
	Hostname  string  `json:"hostname"`
	Entries   []Entry `json:"entries"`
	Checksum  string  `json:"checksum"`
}

// New creates a Manifest for the given git hash with the current timestamp.
func New(gitHash string) *Manifest {
	hostname, _ := os.Hostname()
	return &Manifest{
		Schema:    "snap/manifest/v1",
		Version:   1,
		GitHash:   gitHash,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Hostname:  hostname,
		Entries:   make([]Entry, 0),
	}
}

// AddEntry appends a captured state entry to the manifest.
func (m *Manifest) AddEntry(driver, source, blobHash string, blobSize int64, meta map[string]interface{}) {
	m.Entries = append(m.Entries, Entry{
		Driver:   driver,
		Source:   source,
		BlobHash: blobHash,
		BlobSize: blobSize,
		Metadata: meta,
	})
}

// computeChecksum calculates the SHA-256 checksum of the manifest
// with the Checksum field set to empty string.
func (m *Manifest) computeChecksum() (string, error) {
	original := m.Checksum
	m.Checksum = ""
	defer func() { m.Checksum = original }()

	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("manifest: marshal for checksum: %w", err)
	}
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// Seal computes and sets the checksum field. Must be called before Write.
func (m *Manifest) Seal() error {
	cs, err := m.computeChecksum()
	if err != nil {
		return err
	}
	m.Checksum = cs
	return nil
}

// VerifyChecksum re-computes the checksum and compares it to the
// stored value. Returns false if the manifest has been tampered with.
func (m *Manifest) VerifyChecksum() (bool, error) {
	cs, err := m.computeChecksum()
	if err != nil {
		return false, err
	}
	return cs == m.Checksum, nil
}

// Manager handles reading and writing manifests to disk.
type Manager struct {
	manifestDir string
	tmpDir      string
}

// NewManager creates a Manager that stores manifests in manifestDir
// and uses tmpDir for atomic staging.
func NewManager(manifestDir, tmpDir string) *Manager {
	return &Manager{manifestDir: manifestDir, tmpDir: tmpDir}
}

// Write atomically writes a manifest to disk.
// The manifest must be Seal()'d before calling Write.
func (mgr *Manager) Write(m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest: marshal: %w", err)
	}

	tmpFile, err := os.CreateTemp(mgr.tmpDir, "snap-manifest-*")
	if err != nil {
		return fmt.Errorf("manifest: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup on failure.
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("manifest: write: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("manifest: fsync: %w", err)
	}
	tmpFile.Close()

	target := filepath.Join(mgr.manifestDir, m.GitHash+".json")
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("manifest: rename: %w", err)
	}

	success = true
	return nil
}

// Load reads and deserializes a manifest for the given git hash.
// It verifies the checksum before returning.
func (mgr *Manager) Load(gitHash string) (*Manifest, error) {
	path := filepath.Join(mgr.manifestDir, gitHash+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: unmarshal: %w", err)
	}

	ok, err := m.VerifyChecksum()
	if err != nil {
		return nil, fmt.Errorf("manifest: verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("manifest: checksum mismatch for %s (possible corruption)", gitHash)
	}

	return &m, nil
}

// Exists returns true if a manifest for the given git hash exists.
func (mgr *Manager) Exists(gitHash string) bool {
	path := filepath.Join(mgr.manifestDir, gitHash+".json")
	_, err := os.Stat(path)
	return err == nil
}
