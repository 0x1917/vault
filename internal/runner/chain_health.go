package runner

import (
	"fmt"
	"sort"
	"time"

	"github.com/ruaan-deysel/vault/internal/db"
)

// AnnotatedRestorePoint extends a restore point with chain-health and
// retention-preservation information for the restore UI.
type AnnotatedRestorePoint struct {
	db.RestorePoint
	ChainStatus                 string `json:"chain_status"`
	ChainDepth                  int    `json:"chain_depth"`
	ChainWarning                string `json:"chain_warning,omitempty"`
	MissingParentRestorePointID int64  `json:"missing_parent_restore_point_id,omitempty"`
	RetentionPreserved          bool   `json:"retention_preserved,omitempty"`
	RetentionPreservedFor       int    `json:"retention_preserved_for,omitempty"`
	// BaseFullRestorePointID / BaseFullRestorePointAt identify the full
	// backup at the root of this point's chain — the reference a
	// differential/incremental restore is built on. Both stay zero/nil for
	// a standalone full and for broken chains.
	BaseFullRestorePointID int64      `json:"base_full_restore_point_id,omitempty"`
	BaseFullRestorePointAt *time.Time `json:"base_full_restore_point_at,omitempty"`
}

// AnnotateRestorePoints enriches restore points with chain-health information
// and retention hints without changing their existing API shape.
func AnnotateRestorePoints(job db.Job, points []db.RestorePoint) []AnnotatedRestorePoint {
	return annotateRestorePoints(job, points, time.Now())
}

func annotateRestorePoints(job db.Job, points []db.RestorePoint, now time.Time) []AnnotatedRestorePoint {
	sorted := sortRestorePointsNewest(points)
	var (
		directKeep map[int64]struct{}
		protected  map[int64]struct{}
	)
	if ltr := ltrPolicyFromJob(job); ltr.IsActive() {
		directKeep = ltrDirectlyKept(sorted, ltr, time.Local)
		protected = ltrProtectedRestorePointIDs(sorted, ltr, time.Local)
	} else {
		directKeep = directlyKeptRestorePointIDs(sorted, job.RetentionCount, job.RetentionDays, now)
		protected = protectedRestorePointIDs(sorted, job.RetentionCount, job.RetentionDays, now)
	}
	dependencyCounts := retainedDependencyCounts(sorted, directKeep)
	byID := make(map[int64]db.RestorePoint, len(sorted))
	for _, rp := range sorted {
		byID[rp.ID] = rp
	}

	annotated := make([]AnnotatedRestorePoint, 0, len(points))
	for _, rp := range points {
		status, depth, missingParentID, warning, baseFull := restorePointChainState(rp, byID)
		entry := AnnotatedRestorePoint{
			RestorePoint:                rp,
			ChainStatus:                 status,
			ChainDepth:                  depth,
			ChainWarning:                warning,
			MissingParentRestorePointID: missingParentID,
		}
		if depth > 1 && baseFull.ID > 0 {
			entry.BaseFullRestorePointID = baseFull.ID
			at := baseFull.CreatedAt
			entry.BaseFullRestorePointAt = &at
		}
		if _, isProtected := protected[rp.ID]; isProtected {
			if _, isDirectKeep := directKeep[rp.ID]; !isDirectKeep && dependencyCounts[rp.ID] > 0 {
				entry.RetentionPreserved = true
				entry.RetentionPreservedFor = dependencyCounts[rp.ID]
			}
		}
		annotated = append(annotated, entry)
	}

	return annotated
}

func sortRestorePointsNewest(points []db.RestorePoint) []db.RestorePoint {
	sorted := make([]db.RestorePoint, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].ID > sorted[j].ID
		}
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted
}

func directlyKeptRestorePointIDs(points []db.RestorePoint, keepCount, keepDays int, now time.Time) map[int64]struct{} {
	candidates := points
	if keepCount > 0 && keepCount < len(candidates) {
		candidates = candidates[:keepCount]
	}
	if keepDays > 0 {
		cutoff := now.AddDate(0, 0, -keepDays)
		filtered := make([]db.RestorePoint, 0, len(candidates))
		for _, rp := range candidates {
			if !rp.CreatedAt.Before(cutoff) {
				filtered = append(filtered, rp)
			}
		}
		candidates = filtered
	}

	kept := make(map[int64]struct{}, len(candidates))
	for _, rp := range candidates {
		kept[rp.ID] = struct{}{}
	}
	return kept
}

func retainedDependencyCounts(points []db.RestorePoint, directKeep map[int64]struct{}) map[int64]int {
	byID := make(map[int64]db.RestorePoint, len(points))
	for _, rp := range points {
		byID[rp.ID] = rp
	}

	counts := make(map[int64]int)
	for _, rp := range points {
		if _, keep := directKeep[rp.ID]; !keep {
			continue
		}
		current := rp
		seen := map[int64]struct{}{rp.ID: {}}
		for current.ParentRestorePointID > 0 {
			parentID := current.ParentRestorePointID
			if _, ok := seen[parentID]; ok {
				break
			}
			parent, ok := byID[parentID]
			if !ok {
				break
			}
			counts[parent.ID]++
			seen[parent.ID] = struct{}{}
			current = parent
		}
	}

	return counts
}

// restorePointChainState walks a restore point's parent chain and returns
// (status, depth, missingParentID, warning, baseFull). baseFull is the
// restore point at the root of the chain (the full backup a
// differential/incremental restore is built on). For a standalone point it
// is rp itself; for a broken chain it is the zero value.
func restorePointChainState(rp db.RestorePoint, byID map[int64]db.RestorePoint) (string, int, int64, string, db.RestorePoint) {
	if rp.ParentRestorePointID <= 0 {
		return "standalone", 1, 0, "", rp
	}

	depth := 1
	current := rp
	seen := map[int64]struct{}{rp.ID: {}}
	for current.ParentRestorePointID > 0 {
		depth++
		parentID := current.ParentRestorePointID
		if _, ok := seen[parentID]; ok {
			return "broken", depth, parentID, fmt.Sprintf("Restore chain loops back to restore point #%d.", parentID), db.RestorePoint{}
		}
		parent, ok := byID[parentID]
		if !ok {
			return "broken", depth, parentID, fmt.Sprintf("Parent restore point #%d is missing. Restore from this point will fail until the chain is repaired.", parentID), db.RestorePoint{}
		}
		seen[parentID] = struct{}{}
		current = parent
	}

	return "healthy", depth, 0, "", current
}
