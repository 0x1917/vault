package runner

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ruaan-deysel/vault/internal/crypto"
	"github.com/ruaan-deysel/vault/internal/db"
	"github.com/ruaan-deysel/vault/internal/dedup"
	"github.com/ruaan-deysel/vault/internal/engine"
	"github.com/ruaan-deysel/vault/internal/storage"
)

// testServerKey returns a fixed 32-byte AES key suitable for dedup InitRepo.
func testServerKey() []byte {
	k := make([]byte, crypto.ServerKeySize)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

// makeDedupDest creates a local destination with dedup_enabled=true. The
// dedup repo is NOT initialised on disk so callers can exercise the
// "repo not yet written" branch.
func makeDedupDest(t *testing.T, database *db.DB, storageDir string) db.StorageDestination {
	t.Helper()
	cfg := `{"path":"` + strings.ReplaceAll(storageDir, `\`, `\\`) + `"}`
	id, err := database.CreateStorageDestination(db.StorageDestination{
		Name:         "dedup-local",
		Type:         "local",
		Config:       cfg,
		DedupEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateStorageDestination: %v", err)
	}
	dest, _ := database.GetStorageDestination(id)
	return dest
}

// TestGetDedupStatsNonDedupRefuses covers the early-return when the
// destination has dedup disabled.
func TestGetDedupStatsNonDedupRefuses(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir) // dedup disabled by default

	_, err := r.GetDedupStats(dest)
	if err == nil {
		t.Fatal("GetDedupStats on non-dedup destination should return an error")
	}
}

// TestGetDedupStatsUninitialised covers the "repo.json doesn't exist yet"
// branch — returns zero stats with no error so the UI doesn't render a
// broken card immediately after Add Storage.
func TestGetDedupStatsUninitialised(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := makeDedupDest(t, database, storageDir)

	stats, err := r.GetDedupStats(dest)
	if err != nil {
		t.Fatalf("GetDedupStats (uninitialised): %v", err)
	}
	if stats.TotalChunks != 0 || stats.TotalPacks != 0 {
		t.Errorf("expected zero stats from uninitialised repo, got %+v", stats)
	}
}

// TestGetDedupStatsInitialisedEmpty covers the open-existing-repo branch by
// pre-initialising the repo and then querying stats.
func TestGetDedupStatsInitialisedEmpty(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	// Initialise the repo on disk so the next GetDedupStats opens it.
	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if _, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	storage.CloseAdapter(adapter)

	stats, err := r.GetDedupStats(dest)
	if err != nil {
		t.Fatalf("GetDedupStats (initialised): %v", err)
	}
	if stats.TotalChunks != 0 {
		t.Errorf("fresh repo TotalChunks = %d, want 0", stats.TotalChunks)
	}
}

// TestGetDedupManifestNonDedupRefuses covers the early-return when the
// destination has dedup disabled.
func TestGetDedupManifestNonDedupRefuses(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir)

	_, err := r.GetDedupManifest(dest, dedup.ID{})
	if err == nil {
		t.Fatal("GetDedupManifest on non-dedup destination should error")
	}
}

// TestGetDedupManifestNotFound exercises the open-repo + lookup-missing
// branch. Initialise the repo then query a manifest ID that doesn't exist.
func TestGetDedupManifestNotFound(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if _, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	storage.CloseAdapter(adapter)

	var bogusID dedup.ID
	for i := range bogusID {
		bogusID[i] = 0xAB
	}
	_, err = r.GetDedupManifest(dest, bogusID)
	if err == nil {
		t.Fatal("GetDedupManifest for unknown ID should error")
	}
}

// TestResolveItemManifestIDFromMetadata covers the multi-item dedup path
// where the item-manifest mapping lives in metadata.
func TestResolveItemManifestIDFromMetadata(t *testing.T) {
	t.Parallel()
	const hexID = "ababababababababababababababababababababababababababababababcdef"
	rp := db.RestorePoint{
		Metadata: `{"item_manifests":{"plex":"` + hexID + `","sonarr":"deadbeef"}}`,
	}

	id, ok := ResolveItemManifestID(rp, "plex")
	if !ok {
		t.Fatal("ResolveItemManifestID(plex) returned ok=false")
	}
	want, _ := hex.DecodeString(hexID)
	for i := range id {
		if id[i] != want[i] {
			t.Errorf("byte %d: got %02x want %02x", i, id[i], want[i])
		}
	}
}

func TestResolveItemManifestIDFromRowFallback(t *testing.T) {
	t.Parallel()
	raw, _ := hex.DecodeString("1212121212121212121212121212121212121212121212121212121212121212")
	rp := db.RestorePoint{ManifestID: raw}
	id, ok := ResolveItemManifestID(rp, "any-item")
	if !ok {
		t.Fatal("ResolveItemManifestID with raw ManifestID returned ok=false")
	}
	if id[0] != 0x12 {
		t.Errorf("first byte = %02x, want 12", id[0])
	}
}

func TestResolveItemManifestIDNotDedup(t *testing.T) {
	t.Parallel()
	rp := db.RestorePoint{Metadata: `{}`}
	if _, ok := ResolveItemManifestID(rp, "plex"); ok {
		t.Error("ResolveItemManifestID on non-dedup restore point returned ok=true")
	}
}

func TestResolveItemManifestIDBadHex(t *testing.T) {
	t.Parallel()
	// Hex too short / bad chars are silently skipped.
	rp := db.RestorePoint{Metadata: `{"item_manifests":{"plex":"not-hex"}}`}
	if _, ok := ResolveItemManifestID(rp, "plex"); ok {
		t.Error("bad hex item_manifest should not resolve")
	}
}

// TestRunDedupGCNonDedup covers the refuse branch; broadcasting to a nil
// hub is also exercised here.
func TestRunDedupGCNonDedup(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	dest := createLocalDest(t, database, storageDir) // dedup disabled

	// Should return cleanly after broadcasting a failed event.
	r.RunDedupGC(dest, "test-run-1")
}

// TestRunDedupGCEmptyRepo runs GC against a freshly initialised repo with
// no live manifests; the GC should complete without error and broadcast a
// dedup_gc_complete event.
func TestRunDedupGCEmptyRepo(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if _, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	storage.CloseAdapter(adapter)

	r.RunDedupGC(dest, "gc-run-empty")
}

// TestCollectLiveManifestIDsEmptyDB exercises the empty-result path —
// no restore points means no live IDs.
func TestCollectLiveManifestIDsEmptyDB(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	defer storage.CloseAdapter(adapter)
	repo, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey)
	if err != nil {
		t.Fatalf("InitRepo: %v", err)
	}

	ids, err := r.collectLiveManifestIDs(repo, dest.ID)
	if err != nil {
		t.Fatalf("collectLiveManifestIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("got %d live IDs, want 0", len(ids))
	}
}

// TestCollectLiveManifestIDsWithUnreadableManifest exercises the
// "manifest can't be read" branch: we insert a restore point with a
// manifest ID that doesn't exist on disk; collectLiveManifestIDs should
// still include the top-level ID rather than failing.
func TestCollectLiveManifestIDsWithUnreadableManifest(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	// Create a job + run + restore point referencing a synthetic manifest_id
	// that doesn't exist in the dedup repo (32 bytes of 0xAA).
	jobID, err := database.CreateJob(db.Job{
		Name: "dedup-job", BackupTypeChain: "full", StorageDestID: dest.ID,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	runID, err := database.CreateJobRun(db.JobRun{
		JobID: jobID, Status: "completed", BackupType: "full",
	})
	if err != nil {
		t.Fatalf("CreateJobRun: %v", err)
	}
	bogus := make([]byte, 32)
	for i := range bogus {
		bogus[i] = 0xAA
	}
	rpID, err := database.CreateRestorePoint(db.RestorePoint{
		JobRunID: runID, JobID: jobID, BackupType: "full",
		StoragePath: "dedup-job/1_run", Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("CreateRestorePoint: %v", err)
	}
	if err := database.SetRestorePointManifestID(rpID, bogus); err != nil {
		t.Fatalf("SetRestorePointManifestID: %v", err)
	}

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	defer storage.CloseAdapter(adapter)
	repo, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey)
	if err != nil {
		t.Fatalf("InitRepo: %v", err)
	}

	ids, err := r.collectLiveManifestIDs(repo, dest.ID)
	if err != nil {
		t.Fatalf("collectLiveManifestIDs: %v", err)
	}
	// The walk fails, but the function falls back to treating the top-level
	// manifest as reachable.
	if len(ids) != 1 {
		t.Errorf("got %d live IDs, want 1 (top-level kept on walk failure)", len(ids))
	}
	if len(ids) == 1 && ids[0][0] != 0xAA {
		t.Errorf("first live ID byte = %02x, want AA", ids[0][0])
	}
}

// TestCollectLiveManifestIDsFromMetadataItemManifests confirms multi-item
// dedup metadata (item_manifests map) is also extracted.
func TestCollectLiveManifestIDsFromMetadataItemManifests(t *testing.T) {
	t.Parallel()
	r, database, storageDir := setupTestRunner(t)
	r.serverKey = testServerKey()
	dest := makeDedupDest(t, database, storageDir)

	jobID, err := database.CreateJob(db.Job{
		Name: "multi-dedup-job", BackupTypeChain: "full", StorageDestID: dest.ID,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	runID, _ := database.CreateJobRun(db.JobRun{
		JobID: jobID, Status: "completed", BackupType: "full",
	})
	const hexA = "1111111111111111111111111111111111111111111111111111111111111111"
	const hexB = "2222222222222222222222222222222222222222222222222222222222222222"
	meta := `{"item_manifests":{"appdata":"` + hexA + `","config":"` + hexB + `"}}`
	if _, err := database.CreateRestorePoint(db.RestorePoint{
		JobRunID: runID, JobID: jobID, BackupType: "full",
		StoragePath: "multi-dedup-job/1_run", Metadata: meta,
	}); err != nil {
		t.Fatalf("CreateRestorePoint: %v", err)
	}

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	defer storage.CloseAdapter(adapter)
	repo, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey)
	if err != nil {
		t.Fatalf("InitRepo: %v", err)
	}

	ids, err := r.collectLiveManifestIDs(repo, dest.ID)
	if err != nil {
		t.Fatalf("collectLiveManifestIDs: %v", err)
	}
	// Two top-level IDs; each fails to walk (missing from repo) but is
	// retained as a fallback.
	if len(ids) != 2 {
		t.Errorf("got %d live IDs, want 2: %x", len(ids), ids)
	}
}

// TestGetDedupTarIndex covers the restore file-picker flattening end-to-end:
// container manifests are flattened to real per-file entries (synthetic keys
// and skipped volumes dropped), and non-dedup destinations refuse (issue #333).
func TestGetDedupTarIndex(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, r *Runner, database *db.DB, storageDir string) (db.StorageDestination, dedup.ID, string)
		check func(t *testing.T, idx engine.TarIndex, err error)
	}{
		{
			name: "flattens container manifest",
			setup: func(t *testing.T, r *Runner, database *db.DB, storageDir string) (db.StorageDestination, dedup.ID, string) {
				r.serverKey = testServerKey()
				dest := makeDedupDest(t, database, storageDir)

				adapter, err := storage.NewAdapter(dest.Type, dest.Config)
				if err != nil {
					t.Fatalf("NewAdapter: %v", err)
				}
				repo, err := dedup.InitRepo(database, adapter, dest.ID, r.serverKey)
				if err != nil {
					t.Fatalf("InitRepo: %v", err)
				}

				subID, err := repo.PutManifest("data", dedup.Manifest{
					Version: 1,
					Item:    "data",
					Files: map[string]dedup.ManifestEntry{
						"app/config.yml": {Mode: 0o644, ModTime: "2026-01-01T00:00:00Z", Size: 128},
					},
				})
				if err != nil {
					t.Fatalf("PutManifest (sub): %v", err)
				}
				topID, err := repo.PutManifest("plex", dedup.Manifest{
					Version: 1,
					Item:    "plex",
					Files: map[string]dedup.ManifestEntry{
						"__inspect":      {Size: 9000},
						"__vol__/data":   {Size: 0, Chunks: []dedup.ID{subID}},
						"__vol__/movies": {Size: -1},
					},
				})
				if err != nil {
					t.Fatalf("PutManifest (top): %v", err)
				}
				if err := repo.Flush(); err != nil {
					t.Fatalf("Flush: %v", err)
				}
				storage.CloseAdapter(adapter)

				return dest, topID, "plex"
			},
			check: func(t *testing.T, idx engine.TarIndex, err error) {
				if err != nil {
					t.Fatalf("GetDedupTarIndex: %v", err)
				}
				if idx.Archive != "plex" {
					t.Errorf("archive = %q, want plex", idx.Archive)
				}
				if len(idx.Files) != 1 {
					t.Fatalf("files len = %d, want 1; got %+v", len(idx.Files), idx.Files)
				}
				if idx.Files[0].Path != "/data/app/config.yml" || idx.Files[0].Size != 128 {
					t.Errorf("file = %+v, want path /data/app/config.yml size 128", idx.Files[0])
				}
			},
		},
		{
			name: "non-dedup destination refuses",
			setup: func(t *testing.T, r *Runner, database *db.DB, storageDir string) (db.StorageDestination, dedup.ID, string) {
				return createLocalDest(t, database, storageDir), dedup.ID{}, "item"
			},
			check: func(t *testing.T, idx engine.TarIndex, err error) {
				if err == nil {
					t.Fatal("GetDedupTarIndex on non-dedup destination should error")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, database, storageDir := setupTestRunner(t)
			dest, manifestID, itemName := tt.setup(t, r, database, storageDir)
			idx, err := r.GetDedupTarIndex(dest, manifestID, itemName)
			tt.check(t, idx, err)
		})
	}
}
