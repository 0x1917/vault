package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// buildFolderArchive writes a single-file "data.tar" into a temp dir and
// returns the dir path, mirroring the layout FolderHandler.Restore expects.
func buildFolderArchive(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	archive := filepath.Join(dir, "data.tar")
	writeTarEntries(t, archive, entries)
	return dir
}

// TestFolderRestore_HonorsRestoreExcludePaths verifies FolderHandler.Restore
// extracts everything except the directory named in restore_exclude_paths.
func TestFolderRestore_HonorsRestoreExcludePaths(t *testing.T) {
	sourceDir := buildFolderArchive(t, map[string]string{
		"keep.txt":    "keep",
		"cache/a":     "a",
		"cache/b/c":   "c",
		"other/d.txt": "d",
	})
	dest := t.TempDir()

	h, err := NewFolderHandler()
	if err != nil {
		t.Fatalf("NewFolderHandler: %v", err)
	}
	item := BackupItem{
		Name: "Test Folder",
		Type: "folder",
		Settings: map[string]any{
			"restore_destination":   dest,
			"restore_exclude_paths": []string{"cache"},
		},
	}
	if err := h.Restore(context.Background(), item, sourceDir, func(name string, pct int, msg string) {}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	for _, p := range []string{"cache/a", "cache/b/c"} {
		if _, err := os.Stat(filepath.Join(dest, p)); !os.IsNotExist(err) {
			t.Errorf("excluded %s must not be restored", p)
		}
	}
	for _, p := range []string{"keep.txt", "other/d.txt"} {
		if _, err := os.Stat(filepath.Join(dest, p)); err != nil {
			t.Errorf("sibling %s must be restored: %v", p, err)
		}
	}
}
