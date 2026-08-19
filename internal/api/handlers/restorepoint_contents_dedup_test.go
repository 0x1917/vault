package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ruaan-deysel/vault/internal/db"
	"github.com/ruaan-deysel/vault/internal/dedup"
	"github.com/ruaan-deysel/vault/internal/engine"
	"github.com/ruaan-deysel/vault/internal/storage"
)

// dedupTestServerKey matches the serverKey newJobHandlerDB wires into the
// runner (bytes.Repeat([]byte{0xab}, 32)), so the repo inited here unseals
// with the runner's key.
var dedupTestServerKey = bytes.Repeat([]byte{0xab}, 32)

// TestRestorePointContents_DedupContainerFlattensVolumes is the #333
// regression: browsing a container dedup restore point returns real per-file
// entries (with the mount destination prefix) and omits synthetic keys and
// skipped/excluded volumes.
func TestRestorePointContents_DedupContainerFlattensVolumes(t *testing.T) {
	t.Parallel()
	h, d := newJobHandlerDB(t)

	storageRoot := t.TempDir()
	cfg, _ := json.Marshal(map[string]string{"path": storageRoot})
	destID, err := d.CreateStorageDestination(db.StorageDestination{
		Name: "rpc-dedup-" + nextUnique(), Type: "local", Config: string(cfg), DedupEnabled: true,
	})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}
	dest, _ := d.GetStorageDestination(destID)

	adapter, err := storage.NewAdapter(dest.Type, dest.Config)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	repo, err := dedup.InitRepo(d, adapter, dest.ID, dedupTestServerKey)
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

	jobID, err := d.CreateJob(db.Job{Name: "rpc-dedup-job-" + nextUnique(), StorageDestID: destID})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	runID, err := d.CreateJobRun(db.JobRun{JobID: jobID, Status: "success"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	rpID, err := d.CreateRestorePoint(db.RestorePoint{
		JobRunID: runID, JobID: jobID, BackupType: "full", StoragePath: "rp-dedup",
	})
	if err != nil {
		t.Fatalf("create rp: %v", err)
	}
	// CreateRestorePoint does not persist manifest_id; the dedup engine sets
	// it separately after the row exists.
	if err := d.SetRestorePointManifestID(rpID, topID[:]); err != nil {
		t.Fatalf("set manifest id: %v", err)
	}

	url := fmt.Sprintf("/api/v1/jobs/%d/restore-points/%d/contents?item=plex", jobID, rpID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = withURLParams(req, "id", strconv.FormatInt(jobID, 10), "rpid", strconv.FormatInt(rpID, 10))
	w := httptest.NewRecorder()
	h.RestorePointContents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var got engine.TarIndex
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if got.Archive != "plex" {
		t.Errorf("archive = %q, want plex", got.Archive)
	}
	if len(got.Files) != 1 {
		t.Fatalf("files = %+v, want 1 flattened entry", got.Files)
	}
	if got.Files[0].Path != "/data/app/config.yml" || got.Files[0].Size != 128 {
		t.Errorf("file = %+v, want /data/app/config.yml size 128", got.Files[0])
	}
}
