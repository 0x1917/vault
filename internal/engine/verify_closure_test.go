package engine

import (
	"context"
	"os"
	"testing"

	"github.com/ruaan-deysel/vault/internal/dedup"
)

// TestVerifyDedupClosure_HappyPath backs up a folder into a test repo and
// verifies that VerifyDedupClosure re-reads the whole closure without error.
func TestVerifyDedupClosure_HappyPath(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "folder_backup"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _, cleanup := dedup.NewTestRepoForEngine(t)
			defer cleanup()

			fh := &FolderHandler{}
			src := t.TempDir()
			if err := osWriteFileForTest(src); err != nil {
				t.Fatal(err)
			}
			id, err := fh.BackupChunked(context.Background(), BackupItem{
				Name: "verify-folder", Type: "folder", Settings: map[string]any{"path": src},
			}, r, nil, nil)
			if err != nil {
				t.Fatalf("BackupChunked() error = %v", err)
			}
			if err := r.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}

			if err := VerifyDedupClosure(context.Background(), r, id); err != nil {
				t.Fatalf("VerifyDedupClosure() error = %v", err)
			}
		})
	}
}

// TestVerifyDedupClosure_MissingChunk drives the corruption-detection path: a
// manifest that references a chunk ID not present in the repo must fail the
// verify, mirroring the classic path's checksum-mismatch failure.
func TestVerifyDedupClosure_MissingChunk(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "missing_chunk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _, cleanup := dedup.NewTestRepoForEngine(t)
			defer cleanup()

			var bogus dedup.ID
			for i := range bogus {
				bogus[i] = 0xAB
			}
			m := dedup.Manifest{
				Files: map[string]dedup.ManifestEntry{
					"missing.bin": {Size: 4, Chunks: []dedup.ID{bogus}},
				},
			}
			id, err := r.PutManifest("verify-missing", m)
			if err != nil {
				t.Fatalf("PutManifest() error = %v", err)
			}
			if err := r.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}

			if err := VerifyDedupClosure(context.Background(), r, id); err == nil {
				t.Fatal("VerifyDedupClosure() expected error for a missing chunk, got nil")
			}
		})
	}
}

// osWriteFileForTest is a tiny local helper: add to the same file.
func osWriteFileForTest(src string) error {
	return os.WriteFile(src+"/a.txt", []byte("hello"), 0o644)
}

// TestVerifyDedupClosure_ContainerManifest verifies VerifyDedupClosure walks a
// container-shaped manifest: the __vol__ entry's chunk is a sub-manifest
// (exercising recursion into volume data) and the Installer payload is read as
// data, so a missing chunk in either fails the verify.
func TestVerifyDedupClosure_ContainerManifest(t *testing.T) {
	cases := []struct {
		name          string
		corruptVolume bool
		corruptInst   bool
		wantErr       bool
	}{
		{name: "recursion_and_installer_happy_path"},
		{name: "missing_volume_data_chunk", corruptVolume: true, wantErr: true},
		{name: "missing_installer_chunk", corruptInst: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bogus := func(b byte) dedup.ID {
				var id dedup.ID
				for i := range id {
					id[i] = b
				}
				return id
			}

			r, _, cleanup := dedup.NewTestRepoForEngine(t)
			defer cleanup()

			// A sub-manifest whose own entry references a data chunk, so the
			// walk must recurse one level to reach the volume data.
			subData := []byte("volume-file-content")
			subDataID, err := r.Put(subData)
			if err != nil {
				t.Fatal(err)
			}
			subChunks := []dedup.ID{subDataID}
			if tc.corruptVolume {
				subChunks = []dedup.ID{bogus(0x11)}
			}
			subManifest := dedup.Manifest{Files: map[string]dedup.ManifestEntry{
				"data.bin": {Size: int64(len(subData)), Chunks: subChunks},
			}}
			subID, err := r.PutManifest("vol", subManifest)
			if err != nil {
				t.Fatal(err)
			}

			// Installer payload carried out-of-tree on the top manifest.
			installer := []byte("installer-payload")
			installerID, err := r.Put(installer)
			if err != nil {
				t.Fatal(err)
			}
			instChunks := []dedup.ID{installerID}
			if tc.corruptInst {
				instChunks = []dedup.ID{bogus(0x22)}
			}

			top := dedup.Manifest{
				Files: map[string]dedup.ManifestEntry{
					containerVolPrefix + "/data": {Chunks: []dedup.ID{subID}},
				},
				Installer: &dedup.ManifestEntry{Size: int64(len(installer)), Chunks: instChunks},
			}
			topID, err := r.PutManifest("container", top)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}

			err = VerifyDedupClosure(context.Background(), r, topID)
			if tc.wantErr {
				if err == nil {
					t.Fatal("VerifyDedupClosure() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerifyDedupClosure() error = %v", err)
			}
		})
	}
}

// TestVerifyDedupClosure_Errors drives the defensive and corruption error
// paths: a nil repo and a missing top-level manifest must both fail fast.
func TestVerifyDedupClosure_Errors(t *testing.T) {
	tests := []struct {
		name    string
		useRepo bool
	}{
		{name: "nil_repo", useRepo: false},
		{name: "missing_top_manifest", useRepo: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var repo *dedup.Repo
			if tc.useRepo {
				r, _, cleanup := dedup.NewTestRepoForEngine(t)
				defer cleanup()
				repo = r
			}
			if err := VerifyDedupClosure(context.Background(), repo, dedup.ID{}); err == nil {
				t.Fatal("VerifyDedupClosure() expected error, got nil")
			}
		})
	}
}

// TestVerifyDedupClosure_ContextCancelled verifies the walk short-circuits on
// a cancelled context before re-reading each data chunk.
func TestVerifyDedupClosure_ContextCancelled(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "cancelled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _, cleanup := dedup.NewTestRepoForEngine(t)
			defer cleanup()

			fh := &FolderHandler{}
			src := t.TempDir()
			if err := osWriteFileForTest(src); err != nil {
				t.Fatal(err)
			}
			id, err := fh.BackupChunked(context.Background(), BackupItem{
				Name: "verify-folder", Type: "folder", Settings: map[string]any{"path": src},
			}, r, nil, nil)
			if err != nil {
				t.Fatalf("BackupChunked() error = %v", err)
			}
			if err := r.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := VerifyDedupClosure(ctx, r, id); err == nil {
				t.Fatal("VerifyDedupClosure() with cancelled context expected error, got nil")
			}
		})
	}
}
