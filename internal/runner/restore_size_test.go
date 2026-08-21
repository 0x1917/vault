package runner

import (
	"fmt"
	"testing"

	"github.com/ruaan-deysel/vault/internal/db"
)

func TestRestoredItemSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		metadata string
		item     string
		want     int64
	}{
		{"empty metadata", "", "plex", 0},
		{"malformed json", `not-json`, "plex", 0},
		{"no item_sizes key", `{"items":1}`, "plex", 0},
		{"item present", `{"item_sizes":{"plex":10737418240,"sonarr":5}}`, "plex", 10737418240},
		{"item missing", `{"item_sizes":{"sonarr":5}}`, "plex", 0},
		{"zero-size item", `{"item_sizes":{"plex":0}}`, "plex", 0},
		{"non-integer value", `{"item_sizes":{"plex":"huge"}}`, "plex", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := restoredItemSize(tc.metadata, tc.item); got != tc.want {
				t.Fatalf("restoredItemSize(%q, %q) = %d, want %d", tc.metadata, tc.item, got, tc.want)
			}
		})
	}
}

func TestRestoredRunSizeBytes(t *testing.T) {
	t.Parallel()
	plexSize := int64(10 << 30)
	sonarrSize := int64(5 << 30)
	metadata := fmt.Sprintf(`{"item_sizes":{"plex":%d,"sonarr":%d}}`, plexSize, sonarrSize)

	t.Run("sums only the selected items", func(t *testing.T) {
		rp := db.RestorePoint{SizeBytes: 450 << 30, Metadata: metadata}
		got := restoredRunSizeBytes(rp, []RestoreTarget{{Name: "plex", Type: "container"}})
		if got != plexSize {
			t.Fatalf("subset size = %d, want %d", got, plexSize)
		}
	})

	t.Run("sums multiple selected items", func(t *testing.T) {
		rp := db.RestorePoint{SizeBytes: 450 << 30, Metadata: metadata}
		got := restoredRunSizeBytes(rp, []RestoreTarget{
			{Name: "plex", Type: "container"},
			{Name: "sonarr", Type: "container"},
		})
		if got != plexSize+sonarrSize {
			t.Fatalf("multi-subset size = %d, want %d", got, plexSize+sonarrSize)
		}
	})

	t.Run("falls back to full size without per-item sizes", func(t *testing.T) {
		rp := db.RestorePoint{SizeBytes: 450 << 30, Metadata: `{"items":2}`}
		got := restoredRunSizeBytes(rp, []RestoreTarget{{Name: "plex", Type: "container"}})
		if got != 450<<30 {
			t.Fatalf("legacy fallback size = %d, want %d", got, 450<<30)
		}
	})
}

// TestRunRestore_SubsetSizeReflectsRestoredItems is the regression test for
// issue #334: a partial restore must record the size of the restored subset,
// not the whole restore point.
func TestRunRestore_SubsetSizeReflectsRestoredItems(t *testing.T) {
	t.Parallel()
	r, d := newTestRunner(t)

	destID := seedRunnerStorageDest(t, d)
	jobID, err := d.CreateJob(db.Job{
		Name:          "rr-subset-" + nextUniqueRunner(t),
		Enabled:       true,
		StorageDestID: destID,
		Compression:   "none",
		Encryption:    "none",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	fullSize := int64(450 << 30) // whole backup (~450 GiB)
	plexSize := int64(10 << 30)  // one container (~10 GiB)
	rp := db.RestorePoint{
		JobID:       jobID,
		BackupType:  "full",
		StoragePath: "no-such-path", // restore errors cleanly, but the run size is still recorded
		SizeBytes:   fullSize,
		Metadata:    fmt.Sprintf(`{"item_sizes":{"plex":%d,"sonarr":%d}}`, plexSize, int64(5<<30)),
	}

	r.RunRestore(rp, []RestoreTarget{{Name: "plex", Type: "container"}}, "/tmp/restore-dest", "")

	runs, err := d.GetJobRuns(jobID, 10)
	if err != nil {
		t.Fatalf("get runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("no run recorded")
	}
	if got := runs[0].SizeBytes; got != plexSize {
		t.Fatalf("restore run size_bytes = %d, want %d (the restored subset, not the whole restore point %d)", got, plexSize, fullSize)
	}
}
