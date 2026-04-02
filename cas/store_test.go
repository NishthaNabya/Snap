package cas

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	objDir := filepath.Join(dir, "objects")
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(objDir, 0o755)
	os.MkdirAll(tmpDir, 0o755)
	return NewStore(objDir, tmpDir)
}

func TestPutAndGet(t *testing.T) {
	store := setupTestStore(t)
	content := []byte("hello, snap!")

	// Put a blob.
	hash, size, err := store.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}

	// Verify the hash is correct.
	expected := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(expected[:])
	if hash != expectedHash {
		t.Errorf("hash = %s, want %s", hash, expectedHash)
	}

	// Get the blob back.
	rc, err := store.Get(hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	buf.ReadFrom(rc)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Errorf("content mismatch: got %q, want %q", buf.String(), string(content))
	}
}

func TestPutDeduplication(t *testing.T) {
	store := setupTestStore(t)
	content := []byte("dedup-test-content")

	hash1, _, err := store.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("first Put failed: %v", err)
	}

	hash2, _, err := store.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("second Put failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hashes differ: %s != %s", hash1, hash2)
	}

	// Verify the object file is read-only (set by first Put).
	shard := hash1[:2]
	objPath := filepath.Join(store.root, shard, hash1)
	info, err := os.Stat(objPath)
	if err != nil {
		t.Fatalf("stat object: %v", err)
	}
	if info.Mode().Perm()&0o200 != 0 {
		t.Errorf("object should be read-only, got permissions: %o", info.Mode().Perm())
	}
}

func TestVerify(t *testing.T) {
	store := setupTestStore(t)
	content := []byte("integrity-check")

	hash, _, err := store.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	ok, err := store.Verify(hash)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !ok {
		t.Error("Verify returned false for valid blob")
	}

	// Verify with wrong hash.
	ok, err = store.Verify("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil && ok {
		t.Error("Verify should fail for non-existent hash")
	}
}

func TestHas(t *testing.T) {
	store := setupTestStore(t)
	content := []byte("has-test")

	hash, _, _ := store.Put(bytes.NewReader(content))

	if !store.Has(hash) {
		t.Error("Has returned false for existing blob")
	}
	if store.Has("aaaa000000000000000000000000000000000000000000000000000000000000") {
		t.Error("Has returned true for non-existent blob")
	}
}

func TestCleanupOrphans(t *testing.T) {
	store := setupTestStore(t)

	// Create a fake orphan.
	orphan := filepath.Join(store.tmp, "snap-blob-orphan")
	os.WriteFile(orphan, []byte("orphan"), 0o644)

	// Set its mtime to 2 hours ago.
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(orphan, old, old)

	if err := store.CleanupOrphans(1 * time.Hour); err != nil {
		t.Fatalf("CleanupOrphans failed: %v", err)
	}

	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan file should have been removed")
	}
}

func TestGetInvalidHash(t *testing.T) {
	store := setupTestStore(t)
	_, err := store.Get("x")
	if err == nil {
		t.Error("expected error for short hash")
	}
}
