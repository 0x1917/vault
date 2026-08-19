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
		name     string
		key      string
		wantDest string
		wantOK   bool
	}{
		{name: "simple dest", key: "__vol__/data", wantDest: "/data", wantOK: true},
		{name: "nested dest", key: "__vol__/etc/myapp", wantDest: "/etc/myapp", wantOK: true},
		{name: "root dest", key: "__vol__/", wantDest: "/", wantOK: true},
		{name: "inspect key is not a volume", key: "__inspect", wantDest: "", wantOK: false},
		{name: "plain path is not a volume", key: "data/foo", wantDest: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDest, gotOK := ContainerVolumeDest(tt.key)
			if gotDest != tt.wantDest || gotOK != tt.wantOK {
				t.Errorf("ContainerVolumeDest(%q) = (%q, %v), want (%q, %v)", tt.key, gotDest, gotOK, tt.wantDest, tt.wantOK)
			}
		})
	}
}

func TestIsSkippedVolumeSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want bool
	}{
		{name: "sentinel -1", size: -1, want: true},
		{name: "backed-up volume size 0", size: 0, want: false},
		{name: "ordinary size", size: 1234, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSkippedVolumeSize(tt.size); got != tt.want {
				t.Errorf("IsSkippedVolumeSize(%d) = %v, want %v", tt.size, got, tt.want)
			}
		})
	}
}

func TestIsSyntheticContainerKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "inspect", key: "__inspect", want: true},
		{name: "image meta", key: "__image_meta", want: true},
		{name: "db dump", key: "__dbdump__", want: true},
		{name: "db replay marker", key: "__dbdump_replay__", want: true},
		{name: "volume key", key: "__vol__/data", want: false},
		{name: "plain path", key: "data/foo", want: false},
		{name: "bare volume prefix", key: "__vol__", want: false},
		{name: "empty", key: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSyntheticContainerKey(tt.key); got != tt.want {
				t.Errorf("IsSyntheticContainerKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestManifestToTarIndex(t *testing.T) {
	tests := []struct {
		name   string
		m      dedup.Manifest
		getSub func(dedup.ID) (dedup.Manifest, error)
		check  func(t *testing.T, idx TarIndex, err error)
	}{
		{
			name: "container flattens volumes and drops synthetic/skipped keys",
			m: dedup.Manifest{
				Version: 1,
				Item:    "plex",
				Files: map[string]dedup.ManifestEntry{
					"__inspect":      {Size: 9000},
					"__image_meta":   {Size: 512},
					"__vol__/data":   {Size: 0, Chunks: []dedup.ID{testID(0x01)}},
					"__vol__/movies": {Size: -1}, // skipped/excluded volume
				},
			},
			getSub: func(dedup.ID) (dedup.Manifest, error) {
				return dedup.Manifest{
					Version: 1,
					Item:    "data",
					Files: map[string]dedup.ManifestEntry{
						"app/config.yml": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 128, IsDir: false},
						"app":            {Mode: 0o755, ModTime: "2026-01-01T00:00:00Z", Size: 0, IsDir: true},
					},
				}, nil
			},
			check: func(t *testing.T, idx TarIndex, err error) {
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
			},
		},
		{
			name: "folder manifest maps one-to-one",
			m: dedup.Manifest{
				Version: 1,
				Item:    "appdata",
				Files: map[string]dedup.ManifestEntry{
					"etc/hosts": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 256, IsDir: false},
				},
			},
			getSub: func(dedup.ID) (dedup.Manifest, error) {
				return dedup.Manifest{}, errors.New("should not be called")
			},
			check: func(t *testing.T, idx TarIndex, err error) {
				if err != nil {
					t.Fatalf("ManifestToTarIndex() error = %v", err)
				}
				if len(idx.Files) != 1 {
					t.Fatalf("files len = %d, want 1", len(idx.Files))
				}
				if idx.Files[0].Path != "etc/hosts" || idx.Files[0].Size != 256 || idx.Files[0].Mode != "0644" {
					t.Errorf("file = %+v, want path etc/hosts size 256 mode 0644", idx.Files[0])
				}
			},
		},
		{
			name: "sub-manifest lookup error propagates",
			m: dedup.Manifest{
				Version: 1,
				Item:    "plex",
				Files: map[string]dedup.ManifestEntry{
					"__vol__/data": {Size: 0, Chunks: []dedup.ID{testID(0x07)}},
				},
			},
			getSub: func(dedup.ID) (dedup.Manifest, error) {
				return dedup.Manifest{}, errors.New("boom")
			},
			check: func(t *testing.T, idx TarIndex, err error) {
				if err == nil {
					t.Fatal("ManifestToTarIndex() error = nil, want sub-manifest error")
				}
			},
		},
		{
			name: "folder manifest entries are sorted by path",
			m: dedup.Manifest{
				Version: 1,
				Item:    "appdata",
				Files: map[string]dedup.ManifestEntry{
					"z.txt":   {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 1, IsDir: false},
					"a/b.txt": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 2, IsDir: false},
					"a.txt":   {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 3, IsDir: false},
				},
			},
			getSub: func(dedup.ID) (dedup.Manifest, error) {
				return dedup.Manifest{}, errors.New("should not be called")
			},
			check: func(t *testing.T, idx TarIndex, err error) {
				if err != nil {
					t.Fatalf("ManifestToTarIndex() error = %v", err)
				}
				want := []string{"a.txt", "a/b.txt", "z.txt"}
				if len(idx.Files) != len(want) {
					t.Fatalf("files len = %d, want %d", len(idx.Files), len(want))
				}
				for i, w := range want {
					if idx.Files[i].Path != w {
						t.Errorf("files[%d].Path = %q, want %q (got order %+v)", i, idx.Files[i].Path, w, idx.Files)
					}
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := ManifestToTarIndex(tt.m.Item, tt.m, tt.getSub)
			tt.check(t, idx, err)
		})
	}
}
