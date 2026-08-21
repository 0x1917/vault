package engine

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTarEntries(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar: %v", err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	for name, body := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
}

func TestUntarDirectoryFilteredEx_ExcludesDirAndDescendants(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "data.tar")
	writeTarEntries(t, archive, map[string]string{
		"keep.txt":    "keep",
		"config/a":    "a",
		"config/b/c":  "c",
		"other/d.txt": "d",
	})
	dest := t.TempDir()
	if err := untarDirectoryFilteredEx(context.Background(), archive, dest, nil, []string{"config"}); err != nil {
		t.Fatalf("untarDirectoryFilteredEx: %v", err)
	}
	for _, p := range []string{"config/a", "config/b/c"} {
		if _, err := os.Stat(filepath.Join(dest, p)); !os.IsNotExist(err) {
			t.Errorf("excluded %s must not be extracted", p)
		}
	}
	for _, p := range []string{"keep.txt", "other/d.txt"} {
		if _, err := os.Stat(filepath.Join(dest, p)); err != nil {
			t.Errorf("sibling %s must be extracted: %v", p, err)
		}
	}
}

func TestUntarDirectoryFilteredEx_NilExcludeExtractsEverything(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "data.tar")
	writeTarEntries(t, archive, map[string]string{"a.txt": "a", "b.txt": "b"})
	dest := t.TempDir()
	if err := untarDirectoryFilteredEx(context.Background(), archive, dest, nil, nil); err != nil {
		t.Fatalf("untarDirectoryFilteredEx: %v", err)
	}
	for _, p := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dest, p)); err != nil {
			t.Errorf("%s must be extracted: %v", p, err)
		}
	}
}
