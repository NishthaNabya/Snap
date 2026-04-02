package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestSealAndVerify(t *testing.T) {
	m := New("7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b")
	m.AddEntry("dotenv", ".env", "abc123hash", 1024, map[string]interface{}{"key_count": 5})
	m.AddEntry("sqlite", "./data/app.db", "def456hash", 524288, nil)

	// Seal should set the checksum.
	if err := m.Seal(); err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	if m.Checksum == "" {
		t.Fatal("Checksum should not be empty after Seal")
	}

	// Verify should pass.
	ok, err := m.VerifyChecksum()
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !ok {
		t.Error("VerifyChecksum returned false for sealed manifest")
	}

	// Tamper and verify should fail.
	m.Hostname = "tampered"
	ok, err = m.VerifyChecksum()
	if err != nil {
		t.Fatalf("VerifyChecksum error after tamper: %v", err)
	}
	if ok {
		t.Error("VerifyChecksum should return false after tampering")
	}
}

func TestManagerWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "manifests")
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(manifestDir, 0o755)
	os.MkdirAll(tmpDir, 0o755)

	mgr := NewManager(manifestDir, tmpDir)

	gitHash := "7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b"
	m := New(gitHash)
	m.AddEntry("dotenv", ".env", "abc123hash", 256, nil)
	m.Seal()

	// Write.
	if err := mgr.Write(m); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// File should exist.
	if !mgr.Exists(gitHash) {
		t.Fatal("Exists returned false after Write")
	}

	// Load should return identical data.
	loaded, err := mgr.Load(gitHash)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.GitHash != gitHash {
		t.Errorf("GitHash = %s, want %s", loaded.GitHash, gitHash)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("Entries count = %d, want 1", len(loaded.Entries))
	}
	if loaded.Entries[0].Driver != "dotenv" {
		t.Errorf("Entry driver = %s, want dotenv", loaded.Entries[0].Driver)
	}
	if loaded.Entries[0].BlobHash != "abc123hash" {
		t.Errorf("Entry blob_hash = %s, want abc123hash", loaded.Entries[0].BlobHash)
	}
}

func TestManagerLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "manifests")
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(manifestDir, 0o755)
	os.MkdirAll(tmpDir, 0o755)

	mgr := NewManager(manifestDir, tmpDir)

	_, err := mgr.Load("0000000000000000000000000000000000000000")
	if err == nil {
		t.Error("Load should fail for non-existent manifest")
	}
}

func TestManagerExists(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "manifests")
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(manifestDir, 0o755)
	os.MkdirAll(tmpDir, 0o755)

	mgr := NewManager(manifestDir, tmpDir)

	if mgr.Exists("nonexistent") {
		t.Error("Exists returned true for non-existent manifest")
	}
}

func TestManifestNew(t *testing.T) {
	m := New("abcdef1234567890abcdef1234567890abcdef12")
	if m.Schema != "snap/manifest/v1" {
		t.Errorf("Schema = %s, want snap/manifest/v1", m.Schema)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if m.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if len(m.Entries) != 0 {
		t.Errorf("Entries should be empty, got %d", len(m.Entries))
	}
}
