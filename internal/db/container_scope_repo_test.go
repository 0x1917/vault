package db

import (
	"path/filepath"
	"testing"
)

// TestContainerScopeRoundTrip proves the new container_scope column survives
// Create/Get/Update. Without it, an "all"-scoped job would silently save as
// the default "custom" and the feature would never engage.
func TestContainerScopeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = d.Close() }()

	id, err := d.CreateJob(Job{Name: "all-scope", ContainerScope: "all", BackupTypeChain: "full"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	got, err := d.GetJob(id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.ContainerScope != "all" {
		t.Errorf("ContainerScope = %q, want %q", got.ContainerScope, "all")
	}

	got.ContainerScope = "custom"
	if err := d.UpdateJob(got); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	re, err := d.GetJob(id)
	if err != nil {
		t.Fatalf("GetJob after update: %v", err)
	}
	if re.ContainerScope != "custom" {
		t.Errorf("after update ContainerScope = %q, want %q", re.ContainerScope, "custom")
	}

	// ListJobs shares the same SELECT shape as GetJob.
	jobs, err := d.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.ID == id && j.ContainerScope == "custom" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListJobs did not return the job with ContainerScope %q", "custom")
	}
}

// TestContainerScopeColumnDefaultForLegacyRows proves the ALTER's DEFAULT fills
// the column for rows written before it existed, so existing jobs keep their
// current explicit-selection behaviour (custom) after upgrade.
func TestContainerScopeColumnDefaultForLegacyRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = d.Close() }()

	// Insert a row without naming container_scope — simulates a pre-migration row.
	if _, err := d.Exec("INSERT INTO jobs (name, backup_type_chain) VALUES ('legacy', 'full')"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := d.GetJobByName("legacy")
	if err != nil {
		t.Fatalf("GetJobByName: %v", err)
	}
	if got.ContainerScope != "custom" {
		t.Errorf("legacy row ContainerScope = %q, want %q (column default)", got.ContainerScope, "custom")
	}
}
