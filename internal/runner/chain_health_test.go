package runner

import (
	"testing"
	"time"

	"github.com/ruaan-deysel/vault/internal/db"
)

func TestAnnotateRestorePointsMarksBrokenChain(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	job := db.Job{Name: "broken", RetentionCount: 1}
	points := []db.RestorePoint{{
		ID:                   2,
		BackupType:           "incremental",
		ParentRestorePointID: 99,
		CreatedAt:            now,
	}}

	annotated := annotateRestorePoints(job, points, now)
	if len(annotated) != 1 {
		t.Fatalf("got %d points, want 1", len(annotated))
	}
	if annotated[0].ChainStatus != "broken" {
		t.Fatalf("ChainStatus = %q, want broken", annotated[0].ChainStatus)
	}
	if annotated[0].MissingParentRestorePointID != 99 {
		t.Fatalf("MissingParentRestorePointID = %d, want 99", annotated[0].MissingParentRestorePointID)
	}
	if annotated[0].ChainWarning == "" {
		t.Fatal("expected non-empty chain warning")
	}
}

func TestAnnotateRestorePointsMarksRetentionPreservedParent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	job := db.Job{Name: "retention", RetentionCount: 1}
	points := []db.RestorePoint{
		{
			ID:         10,
			BackupType: "full",
			CreatedAt:  now.Add(-2 * time.Hour),
		},
		{
			ID:                   11,
			BackupType:           "differential",
			ParentRestorePointID: 10,
			CreatedAt:            now,
		},
	}

	annotated := annotateRestorePoints(job, points, now)
	byID := make(map[int64]AnnotatedRestorePoint, len(annotated))
	for _, rp := range annotated {
		byID[rp.ID] = rp
	}

	child := byID[11]
	if child.ChainStatus != "healthy" {
		t.Fatalf("child ChainStatus = %q, want healthy", child.ChainStatus)
	}
	if child.ChainDepth != 2 {
		t.Fatalf("child ChainDepth = %d, want 2", child.ChainDepth)
	}

	parent := byID[10]
	if parent.ChainStatus != "standalone" {
		t.Fatalf("parent ChainStatus = %q, want standalone", parent.ChainStatus)
	}
	if !parent.RetentionPreserved {
		t.Fatal("expected parent restore point to be marked retention-preserved")
	}
	if parent.RetentionPreservedFor != 1 {
		t.Fatalf("RetentionPreservedFor = %d, want 1", parent.RetentionPreservedFor)
	}
}

func TestAnnotateRestorePointsUsesNewestOrderForRetention(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	job := db.Job{Name: "unsorted", RetentionCount: 1}
	points := []db.RestorePoint{
		{
			ID:         20,
			BackupType: "full",
			CreatedAt:  now.Add(-2 * time.Hour),
		},
		{
			ID:                   22,
			BackupType:           "incremental",
			ParentRestorePointID: 21,
			CreatedAt:            now,
		},
		{
			ID:                   21,
			BackupType:           "incremental",
			ParentRestorePointID: 20,
			CreatedAt:            now.Add(-time.Hour),
		},
	}

	annotated := annotateRestorePoints(job, points, now)
	byID := make(map[int64]AnnotatedRestorePoint, len(annotated))
	for _, rp := range annotated {
		byID[rp.ID] = rp
	}

	if !byID[21].RetentionPreserved {
		t.Fatal("expected intermediate restore point to be preserved for the newest child")
	}
	if !byID[20].RetentionPreserved {
		t.Fatal("expected base restore point to be preserved for the newest child")
	}
	if byID[22].ChainDepth != 3 {
		t.Fatalf("latest ChainDepth = %d, want 3", byID[22].ChainDepth)
	}
}

func TestAnnotateRestorePointsMarksBaseFull(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	job := db.Job{Name: "base-full", RetentionCount: 1}
	fullAt := now.Add(-4 * time.Hour)
	points := []db.RestorePoint{
		{ID: 10, BackupType: "full", CreatedAt: fullAt},
		{ID: 11, BackupType: "differential", ParentRestorePointID: 10, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: 12, BackupType: "incremental", ParentRestorePointID: 11, CreatedAt: now},
	}

	annotated := annotateRestorePoints(job, points, now)
	byID := make(map[int64]AnnotatedRestorePoint, len(annotated))
	for _, rp := range annotated {
		byID[rp.ID] = rp
	}

	// The deepest point resolves its base full to the chain root (ID 10).
	deep := byID[12]
	if deep.BaseFullRestorePointID != 10 {
		t.Fatalf("BaseFullRestorePointID = %d, want 10", deep.BaseFullRestorePointID)
	}
	if deep.BaseFullRestorePointAt == nil || !deep.BaseFullRestorePointAt.Equal(fullAt) {
		t.Fatalf("BaseFullRestorePointAt = %v, want %v", deep.BaseFullRestorePointAt, fullAt)
	}

	// The differential point resolves to the same base full.
	mid := byID[11]
	if mid.BaseFullRestorePointID != 10 {
		t.Fatalf("mid BaseFullRestorePointID = %d, want 10", mid.BaseFullRestorePointID)
	}

	// A standalone full has no base-full reference (it is the reference).
	full := byID[10]
	if full.BaseFullRestorePointID != 0 {
		t.Fatalf("full BaseFullRestorePointID = %d, want 0", full.BaseFullRestorePointID)
	}
	if full.BaseFullRestorePointAt != nil {
		t.Fatalf("full BaseFullRestorePointAt = %v, want nil", full.BaseFullRestorePointAt)
	}
}
