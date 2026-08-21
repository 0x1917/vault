package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ruaan-deysel/vault/internal/db"
	"github.com/ruaan-deysel/vault/internal/engine"
)

// TestRestorePointContents_HappyPath places a valid JSON tar-index sidecar
// next to the (fictional) archive and verifies RestorePointContents reads it
// and returns the parsed TarIndex.
func TestRestorePointContents_HappyPath(t *testing.T) {
	t.Parallel()
	h, d := newJobHandlerDB(t)

	// Build a local storage dest so we know the on-disk root.
	storageRoot := t.TempDir()
	cfg, _ := json.Marshal(map[string]string{"path": storageRoot})
	destID, err := d.CreateStorageDestination(db.StorageDestination{
		Name: "rpc-happy-" + nextUnique(), Type: "local", Config: string(cfg),
	})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}
	jobID, err := d.CreateJob(db.Job{
		Name: "rpc-happy-job-" + nextUnique(), StorageDestID: destID,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	runID, err := d.CreateJobRun(db.JobRun{JobID: jobID, Status: "success"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	rpID, err := d.CreateRestorePoint(db.RestorePoint{
		JobRunID:    runID,
		JobID:       jobID,
		BackupType:  "full",
		StoragePath: "rp-happy",
		Metadata:    "{}",
	})
	if err != nil {
		t.Fatalf("create restore point: %v", err)
	}

	// Write a tar-index sidecar at the expected per-item subdir.
	itemDir := filepath.Join(storageRoot, "rp-happy", "fooitem")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	idx := engine.TarIndex{
		Version: 1,
		Archive: "backup.tar",
		Files: []engine.TarIndexEntry{
			{Path: "/etc/conf", Size: 42, Mode: "0644", ModTime: "2026-01-01T00:00:00Z"},
		},
	}
	idxBytes, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal idx: %v", err)
	}
	sidecarPath := filepath.Join(itemDir, "backup.tar"+engine.IndexSuffix)
	if err := os.WriteFile(sidecarPath, idxBytes, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	url := fmt.Sprintf("/api/v1/jobs/%d/restore-points/%d/contents?item=fooitem", jobID, rpID)
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
	if got.Archive != "backup.tar" {
		t.Errorf("archive = %q, want backup.tar", got.Archive)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "/etc/conf" {
		t.Errorf("files = %+v, want one entry at /etc/conf", got.Files)
	}
}

// TestRestorePointContents_AgeSidecarWithoutPassphrase covers the
// 424-failed-dependency branch where the sidecar exists with the .age
// extension but no passphrase is configured on the runner.
func TestRestorePointContents_AgeSidecarWithoutPassphrase(t *testing.T) {
	t.Parallel()
	h, d := newJobHandlerDB(t)

	storageRoot := t.TempDir()
	cfg, _ := json.Marshal(map[string]string{"path": storageRoot})
	destID, err := d.CreateStorageDestination(db.StorageDestination{
		Name: "rpc-age-" + nextUnique(), Type: "local", Config: string(cfg),
	})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}
	jobID, err := d.CreateJob(db.Job{
		Name: "rpc-age-job-" + nextUnique(), StorageDestID: destID,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	runID, err := d.CreateJobRun(db.JobRun{JobID: jobID, Status: "success"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	rpID, err := d.CreateRestorePoint(db.RestorePoint{
		JobRunID:    runID,
		JobID:       jobID,
		BackupType:  "full",
		StoragePath: "rp-age",
		Metadata:    "{}",
	})
	if err != nil {
		t.Fatalf("create restore point: %v", err)
	}

	itemDir := filepath.Join(storageRoot, "rp-age", "encitem")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Use a dummy non-empty file content; the handler bails before
	// trying to decrypt because no passphrase is configured.
	sidecar := filepath.Join(itemDir, "enc.tar"+engine.IndexSuffix+".age")
	if err := os.WriteFile(sidecar, []byte("encrypted-blob"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	url := fmt.Sprintf("/api/v1/jobs/%d/restore-points/%d/contents?item=encitem&file=enc.tar.age",
		jobID, rpID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = withURLParams(req, "id", strconv.FormatInt(jobID, 10), "rpid", strconv.FormatInt(rpID, 10))
	w := httptest.NewRecorder()
	h.RestorePointContents(w, req)

	if w.Code != http.StatusFailedDependency {
		t.Fatalf("status = %d, want 424; body: %s", w.Code, w.Body.String())
	}
}

func TestRestorePointContents_DirParamReturnsChildren(t *testing.T) {
	t.Parallel()
	h, d := newJobHandlerDB(t)

	storageRoot := t.TempDir()
	cfg, _ := json.Marshal(map[string]string{"path": storageRoot})
	destID, err := d.CreateStorageDestination(db.StorageDestination{Name: "rpc-dir-" + nextUnique(), Type: "local", Config: string(cfg)})
	if err != nil {
		t.Fatalf("create dest: %v", err)
	}
	jobID, err := d.CreateJob(db.Job{Name: "rpc-dir-job-" + nextUnique(), StorageDestID: destID})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	runID, err := d.CreateJobRun(db.JobRun{JobID: jobID, Status: "success"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	rpID, err := d.CreateRestorePoint(db.RestorePoint{JobRunID: runID, JobID: jobID, BackupType: "full", StoragePath: "rp-dir", Metadata: "{}"})
	if err != nil {
		t.Fatalf("create rp: %v", err)
	}

	itemDir := filepath.Join(storageRoot, "rp-dir", "fooitem")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	idx := engine.TarIndex{Version: 1, Archive: "backup.tar", Files: []engine.TarIndexEntry{
		{Path: "app", IsDir: true},
		{Path: "app/main.py", Size: 10, Mode: "0644", ModTime: "2026-01-01T00:00:00Z"},
		{Path: "app/lib", IsDir: true},
		{Path: "app/lib/util.py", Size: 20, Mode: "0644", ModTime: "2026-01-01T00:00:00Z"},
		{Path: "README.md", Size: 5, Mode: "0644", ModTime: "2026-01-01T00:00:00Z"},
	}}
	b, _ := json.Marshal(idx)
	if err := os.WriteFile(filepath.Join(itemDir, "backup.tar"+engine.IndexSuffix), b, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	url := fmt.Sprintf("/api/v1/jobs/%d/restore-points/%d/contents?item=fooitem&dir=app", jobID, rpID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = withURLParams(req, "id", strconv.FormatInt(jobID, 10), "rpid", strconv.FormatInt(rpID, 10))
	w := httptest.NewRecorder()
	h.RestorePointContents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var got engine.TarIndexDir
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if got.Dir != "app" {
		t.Errorf("dir = %q, want app", got.Dir)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %+v, want 2 children of app", got.Entries)
	}
	paths := map[string]bool{}
	for _, e := range got.Entries {
		paths[e.Path] = true
	}
	if !paths["app/main.py"] || !paths["app/lib"] {
		t.Errorf("entries = %+v, want app/main.py and app/lib", got.Entries)
	}
	if got.TotalFiles != 3 || got.TotalDirs != 2 {
		t.Errorf("totals = (%d files, %d dirs), want (3, 2)", got.TotalFiles, got.TotalDirs)
	}
}
