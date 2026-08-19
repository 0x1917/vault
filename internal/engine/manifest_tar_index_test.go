package engine

import (
	"errors"
	"testing"

	"github.com/ruaan-deysel/vault/internal/dedup"
)

func testID(b byte) dedup.ID {
	var id dedup.ID
	id[0] = b
	return id
}

func TestContainerVolumeDest(t *testing.T) {
	tests := []struct {
		key      string
		wantDest string
		wantOK   bool
	}{
		{"__vol__/data", "/data", true},
		{"__vol__/etc/myapp", "/etc/myapp", true},
		{"__vol__/", "/", true},
		{"__inspect", "", false},
		{"data/foo", "", false},
	}
	for _, tt := range tests {
		gotDest, gotOK := ContainerVolumeDest(tt.key)
		if gotDest != tt.wantDest || gotOK != tt.wantOK {
			t.Errorf("ContainerVolumeDest(%q) = (%q, %v), want (%q, %v)", tt.key, gotDest, gotOK, tt.wantDest, tt.wantOK)
		}
	}
}

func TestIsSkippedVolumeSize(t *testing.T) {
	if !IsSkippedVolumeSize(-1) {
		t.Error("IsSkippedVolumeSize(-1) = false, want true")
	}
	if IsSkippedVolumeSize(0) {
		t.Error("IsSkippedVolumeSize(0) = true, want false (backed-up volumes store Size 0)")
	}
	if IsSkippedVolumeSize(1234) {
		t.Error("IsSkippedVolumeSize(1234) = true, want false")
	}
}

func TestIsSyntheticContainerKey(t *testing.T) {
	for _, k := range []string{"__inspect", "__image_meta", "__dbdump__"} {
		if !IsSyntheticContainerKey(k) {
			t.Errorf("IsSyntheticContainerKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"__vol__/data", "data/foo", "__vol__", ""} {
		if IsSyntheticContainerKey(k) {
			t.Errorf("IsSyntheticContainerKey(%q) = true, want false", k)
		}
	}
}

func TestManifestToTarIndex_ContainerFlattensVolumes(t *testing.T) {
	sub := dedup.Manifest{
		Version: 1,
		Item:    "data",
		Files: map[string]dedup.ManifestEntry{
			"app/config.yml": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 128, IsDir: false},
			"app":            {Mode: 0o755, ModTime: "2026-01-01T00:00:00Z", Size: 0, IsDir: true},
		},
	}
	getSub := func(dedup.ID) (dedup.Manifest, error) { return sub, nil }

	m := dedup.Manifest{
		Version: 1,
		Item:    "plex",
		Files: map[string]dedup.ManifestEntry{
			"__inspect":      {Size: 9000},
			"__image_meta":   {Size: 512},
			"__vol__/data":   {Size: 0, Chunks: []dedup.ID{testID(0x01)}},
			"__vol__/movies": {Size: -1}, // skipped/excluded volume
		},
	}

	idx, err := ManifestToTarIndex("plex", m, getSub)
	if err != nil {
		t.Fatalf("ManifestToTarIndex() error = %v", err)
	}
	if idx.Archive != "plex" {
		t.Errorf("archive = %q, want plex", idx.Archive)
	}
	if len(idx.Files) != 2 {
		t.Fatalf("files len = %d, want 2; got %+v", len(idx.Files), idx.Files)
	}
	paths := map[string]TarIndexEntry{}
	for _, f := range idx.Files {
		paths[f.Path] = f
	}
	cfg, ok := paths["/data/app/config.yml"]
	if !ok {
		t.Fatalf("missing /data/app/config.yml in %+v", idx.Files)
	}
	if cfg.Size != 128 {
		t.Errorf("/data/app/config.yml size = %d, want 128", cfg.Size)
	}
	if cfg.IsDir {
		t.Error("/data/app/config.yml IsDir = true, want false")
	}
	dir, ok := paths["/data/app"]
	if !ok {
		t.Fatalf("missing /data/app dir in %+v", idx.Files)
	}
	if !dir.IsDir {
		t.Error("/data/app IsDir = false, want true")
	}
}

func TestManifestToTarIndex_FolderMapsOneToOne(t *testing.T) {
	m := dedup.Manifest{
		Version: 1,
		Item:    "appdata",
		Files: map[string]dedup.ManifestEntry{
			"etc/hosts": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 256, IsDir: false},
		},
	}
	idx, err := ManifestToTarIndex("appdata", m, func(dedup.ID) (dedup.Manifest, error) {
		return dedup.Manifest{}, errors.New("should not be called")
	})
	if err != nil {
		t.Fatalf("ManifestToTarIndex() error = %v", err)
	}
	if len(idx.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(idx.Files))
	}
	if idx.Files[0].Path != "etc/hosts" || idx.Files[0].Size != 256 || idx.Files[0].Mode != "0644" {
		t.Errorf("file = %+v, want path etc/hosts size 256 mode 0644", idx.Files[0])
	}
}

func TestManifestToTarIndex_SubManifestLookupError(t *testing.T) {
	m := dedup.Manifest{
		Version: 1,
		Item:    "plex",
		Files: map[string]dedup.ManifestEntry{
			"__vol__/data": {Size: 0, Chunks: []dedup.ID{testID(0x07)}},
		},
	}
	_, err := ManifestToTarIndex("plex", m, func(dedup.ID) (dedup.Manifest, error) {
		return dedup.Manifest{}, errors.New("boom")
	})
	if err == nil {
		t.Fatal("ManifestToTarIndex() error = nil, want sub-manifest error")
	}
}
