package engine

import (
	"fmt"
	"path"
	"strings"

	"github.com/ruaan-deysel/vault/internal/dedup"
)

// IsSyntheticContainerKey reports whether key is one of the synthetic
// bookkeeping keys the dedup/chunked container format stores alongside its
// __vol__ volume entries (__inspect, __image_meta, __dbdump__). These are
// never real files, so anything that lists a container manifest for display
// (the restore file picker) must drop them.
func IsSyntheticContainerKey(key string) bool {
	switch key {
	case containerInspectKey, containerImageMetaKey, ContainerDBDumpKey:
		return true
	}
	return false
}

// ContainerVolumeDest reports whether key is a __vol__<destination> volume
// entry and, if so, returns the volume's destination path (the part after the
// "__vol__" prefix). Container manifests key their volume entries by mount
// destination, so this is the path the picker should prefix onto the volume's
// file paths.
func ContainerVolumeDest(key string) (string, bool) {
	if !strings.HasPrefix(key, containerVolPrefix) {
		return "", false
	}
	return strings.TrimPrefix(key, containerVolPrefix), true
}

// IsSkippedVolumeSize reports whether size is the sentinel recorded on a
// __vol__ entry whose volume was skipped/excluded at backup time. Such entries
// carry no sub-manifest (empty Chunks) and must not be offered for restore.
func IsSkippedVolumeSize(size int64) bool {
	return size == volumeSkippedSize
}

// ManifestToTarIndex flattens a dedup manifest into a TarIndex for the restore
// file picker. Folder and plugin manifests map 1:1 (their Files keys are real
// relative paths). Container manifests are identified by their synthetic keys:
// bookkeeping entries (__inspect, __image_meta, __dbdump__) are dropped,
// skipped/excluded volumes (Size == -1) are dropped, and each backed-up
// __vol__<dest> entry is recursed into via getSub so its real per-file paths
// and sizes are listed, prefixed with the mount destination.
func ManifestToTarIndex(itemName string, m dedup.Manifest, getSub func(dedup.ID) (dedup.Manifest, error)) (TarIndex, error) {
	idx := TarIndex{
		Version: 1,
		Archive: itemName,
		Files:   make([]TarIndexEntry, 0, len(m.Files)),
	}
	if err := appendManifestFiles(&idx, m, "", getSub); err != nil {
		return TarIndex{}, err
	}
	return idx, nil
}

// appendManifestFiles appends the flattened entries of m to idx, prefixing
// each path with prefix (non-empty only when recursing into a volume
// sub-manifest). Synthetic keys and skipped volumes are omitted.
func appendManifestFiles(idx *TarIndex, m dedup.Manifest, prefix string, getSub func(dedup.ID) (dedup.Manifest, error)) error {
	for key, entry := range m.Files {
		if IsSyntheticContainerKey(key) {
			continue
		}
		if dest, ok := ContainerVolumeDest(key); ok {
			if IsSkippedVolumeSize(entry.Size) {
				continue
			}
			if len(entry.Chunks) == 0 {
				continue
			}
			sub, err := getSub(entry.Chunks[0])
			if err != nil {
				return fmt.Errorf("read volume %q sub-manifest: %w", key, err)
			}
			if err := appendManifestFiles(idx, sub, dest, getSub); err != nil {
				return err
			}
			continue
		}
		idx.Files = append(idx.Files, tarIndexEntryFromManifest(key, entry, prefix))
	}
	return nil
}

// tarIndexEntryFromManifest builds a TarIndexEntry from a manifest entry,
// joining prefix onto key so volume sub-manifest files render under their
// mount destination (e.g. __vol__/data + "app/config.yml" -> "/data/app/config.yml").
func tarIndexEntryFromManifest(key string, entry dedup.ManifestEntry, prefix string) TarIndexEntry {
	full := key
	if prefix != "" {
		full = path.Join(prefix, key)
	}
	return TarIndexEntry{
		Path:    full,
		Size:    entry.Size,
		Mode:    fmt.Sprintf("%04o", entry.Mode&0o7777),
		ModTime: entry.ModTime,
		IsDir:   entry.IsDir,
	}
}
