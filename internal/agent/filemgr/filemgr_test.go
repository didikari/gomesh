package filemgr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileList(t *testing.T) {
	tempDir := t.TempDir()

	subDir := filepath.Join(tempDir, "subfolder")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed creating subfolder: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed creating test file: %v", err)
	}

	// Test listing directory
	resolved, entries, err := List(tempDir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Directories should come first
	if !entries[0].IsDir || entries[0].Name != "subfolder" {
		t.Errorf("expected first entry to be subfolder, got %+v", entries[0])
	}
	if entries[1].IsDir || entries[1].Name != "test.txt" {
		t.Errorf("expected second entry to be test.txt, got %+v", entries[1])
	}

	t.Logf("Resolved path: %s", resolved)
	for i, e := range entries {
		t.Logf("Entry [%d]: Name=%s, Path=%s, IsDir=%v", i, e.Name, e.Path, e.IsDir)
	}

	// Test listing subfolder
	subResolved, subEntries, err := List(entries[0].Path)
	if err != nil {
		t.Fatalf("List subfolder failed: %v", err)
	}
	t.Logf("Subfolder resolved: %s, entries: %d", subResolved, len(subEntries))
}
